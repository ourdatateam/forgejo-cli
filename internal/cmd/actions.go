package cmd

// Ported from the bash cmd_actions family (forgejo:3315-3848).
// The task join and on-host log layout are discovered Forgejo behavior, not
// documented API surface.

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/klauspost/compress/zstd"
	"github.com/ourdatateam/forgejo-cli/internal/cmdutil"
	"github.com/spf13/cobra"
)

func init() { Register(newActionsCmd) }

func newActionsCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "actions <list|view|watch|logs|dispatch|runner|workflow|secret|variable>",
		Short: "List and operate on Forgejo Actions",
		Long: strings.TrimSpace(`Actions commands inspect workflow runs, watch a run until it
finishes, read task logs from the Forgejo host when FORGEJO_LOG_PATH is set,
dispatch workflows, and manage scoped runners, secrets, and variables.

Scopes for runners, secrets, and variables are one of <owner/repo>, <org>, or
--admin for the instance scope.`),
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmdutil.Usagef("Usage: forgejo actions <list|view|watch|logs|dispatch|runner|workflow|secret|variable> [args]")
		},
	}
	cmd.AddCommand(
		newActionsListCmd(ctx),
		newActionsViewCmd(ctx),
		newActionsWatchCmd(ctx),
		newActionsLogsCmd(ctx),
		newActionsDispatchCmd(ctx),
		newActionsRunnerCmd(ctx),
		newActionsWorkflowCmd(ctx),
		newActionsSecretCmd(ctx),
		newActionsVariableCmd(ctx),
	)
	return cmd
}

func newActionsListCmd(ctx *cmdutil.Ctx) *cobra.Command {
	return &cobra.Command{
		Use:   "list <owner/repo>",
		Short: "List recent workflow runs",
		Long: strings.TrimSpace(`List recent workflow runs for a required repository.

The repository positional is required; pass "." to infer owner/repo from the
current git remote. The bash command used limit=20; --limit overrides that
default and --limit=0 requests all pages where the API supports it.`),
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			repo, err := ctx.RepoArg(args[0])
			if err != nil {
				return err
			}
			n := ctx.ListLimit(20)
			lr, err := ctx.Client.DoList("repos/"+repo+"/actions/runs", n)
			if err != nil {
				return err
			}
			if ctx.WantsJSON() {
				return ctx.EmitJSON(lr.Body)
			}
			items, err := workflowRuns(lr.Body)
			if err != nil {
				return err
			}
			rows := make([][]string, 0, len(items))
			for _, m := range items {
				started := firstNonEmpty(cmdutil.Str(m, "started"), cmdutil.Str(m, "created"), "-")
				if started != "-" {
					started = cmdutil.Trunc(started, 19)
				}
				rows = append(rows, []string{
					cmdutil.Str(m, "id"),
					dash(cmdutil.Str(m, "status")),
					dash(cmdutil.Str(m, "event")),
					dash(cmdutil.Str(m, "prettyref")),
					cmdutil.Trunc(dash(cmdutil.Str(m, "commit_sha")), 12),
					started,
					cmdutil.Trunc(dash(cmdutil.Str(m, "title")), 50),
				})
			}
			ctx.Table([]string{"ID", "STATUS", "EVENT", "BRANCH", "COMMIT", "STARTED", "TITLE"}, rows)
			ctx.Trailer(len(rows), lr.Total, n)
			return nil
		},
	}
}

func newActionsViewCmd(ctx *cmdutil.Ctx) *cobra.Command {
	return &cobra.Command{
		Use:   "view <owner/repo> <run_id>",
		Short: "View run details and jobs",
		Long: strings.TrimSpace(`View a workflow run and, in text mode, the jobs that belong to
that run.

The repository positional is required; pass "." to infer owner/repo from the
current git remote. Jobs are joined by comparing the run's index_in_repo to
task run_number from actions/tasks?limit=200; this hardcoded join limit is not
affected by --limit. With --json, only the raw run JSON is printed.`),
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			repo, err := ctx.RepoArg(args[0])
			if err != nil {
				return err
			}
			runID, err := cmdutil.IDArg(args[1], "run id")
			if err != nil {
				return err
			}
			raw, err := ctx.Client.Do("GET", "repos/"+repo+"/actions/runs/"+runID, nil)
			if err != nil {
				return err
			}
			if ctx.WantsJSON() {
				return ctx.EmitJSON(raw)
			}
			run, err := cmdutil.ParseObject(raw)
			if err != nil {
				return err
			}
			printRun(ctx, run)

			fmt.Fprintln(ctx.Out)
			fmt.Fprintln(ctx.Out, "--- Jobs ---")
			tasks, err := tasksForRun(ctx, repo, cmdutil.Str(run, "index_in_repo"))
			if err != nil {
				return err
			}
			if len(tasks) == 0 {
				fmt.Fprintln(ctx.Out, "No jobs found.")
				return nil
			}
			for _, task := range tasks {
				fmt.Fprintf(ctx.Out, "  [%s] %s (task_id=%s)\n",
					dash(cmdutil.Str(task, "status")),
					dash(cmdutil.Str(task, "name")),
					cmdutil.Str(task, "id"))
			}
			return nil
		},
	}
}

func newActionsWatchCmd(ctx *cmdutil.Ctx) *cobra.Command {
	return &cobra.Command{
		Use:   "watch <owner/repo> <run_id>",
		Short: "Poll until a run completes",
		Long: strings.TrimSpace(`Poll a workflow run every five seconds until its status is
success, failure, or cancelled.

The repository positional is required; pass "." to infer owner/repo from the
current git remote. On a terminal, the status line is rewritten with carriage
returns like the bash command; when stdout is not a terminal, one status line is
printed for each poll. A successful run exits 0; failure or cancellation prints
the final status and returns a non-zero error.`),
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			repo, err := ctx.RepoArg(args[0])
			if err != nil {
				return err
			}
			runID, err := cmdutil.IDArg(args[1], "run id")
			if err != nil {
				return err
			}
			tty := writerIsTerminal(ctx.Out)
			fmt.Fprintf(ctx.Out, "Watching run #%s...\n", runID)
			for {
				raw, err := ctx.Client.Do("GET", "repos/"+repo+"/actions/runs/"+runID, nil)
				if err != nil {
					return err
				}
				run, err := cmdutil.ParseObject(raw)
				if err != nil {
					return err
				}
				status := cmdutil.Str(run, "status")
				title := dash(cmdutil.Str(run, "title"))
				line := fmt.Sprintf("[%s] %s", status, title)
				if tty {
					fmt.Fprintf(ctx.Out, "\r%-80s", line)
				} else {
					fmt.Fprintln(ctx.Out, line)
				}
				switch status {
				case "success":
					if tty {
						fmt.Fprintln(ctx.Out)
					}
					fmt.Fprintln(ctx.Out, "Run completed successfully.")
					return nil
				case "failure", "cancelled":
					if tty {
						fmt.Fprintln(ctx.Out)
					}
					fmt.Fprintf(ctx.Out, "Run finished: %s\n", status)
					return fmt.Errorf("run finished: %s", status)
				}
				time.Sleep(5 * time.Second)
			}
		},
	}
}

func newActionsLogsCmd(ctx *cmdutil.Ctx) *cobra.Command {
	return &cobra.Command{
		Use:   "logs <owner/repo> <run_id>",
		Short: "View run logs",
		Long: strings.TrimSpace(`View workflow run logs.

Forgejo's API does not expose job log contents for the probed server versions.
When FORGEJO_LOG_PATH is configured and this CLI runs on the Forgejo host, logs
are read directly from disk as zstd files at
FORGEJO_LOG_PATH/<owner>/<repo>/<last-byte-hex>/<task_id>.log.zst. Without
FORGEJO_LOG_PATH, this prints the task list, browser URL, and setup hint.`),
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			repo, err := ctx.RepoArg(args[0])
			if err != nil {
				return err
			}
			runID, err := cmdutil.IDArg(args[1], "run id")
			if err != nil {
				return err
			}
			raw, err := ctx.Client.Do("GET", "repos/"+repo+"/actions/runs/"+runID, nil)
			if err != nil {
				return err
			}
			run, err := cmdutil.ParseObject(raw)
			if err != nil {
				return err
			}
			runIndex := cmdutil.Str(run, "index_in_repo")
			tasks, err := tasksForRun(ctx, repo, runIndex)
			if err != nil {
				return err
			}
			if len(tasks) == 0 {
				return &cmdutil.ExitError{Code: 1, Err: fmt.Errorf("No jobs found for run #%s", runID)}
			}

			if logPath := configLogPath(ctx); logPath != "" {
				return printDiskLogs(ctx, logPath, repo, tasks)
			}
			for _, task := range tasks {
				fmt.Fprintf(ctx.Out, "  [%s] %s (task_id=%s)\n",
					dash(cmdutil.Str(task, "status")),
					dash(cmdutil.Str(task, "name")),
					cmdutil.Str(task, "id"))
			}
			fmt.Fprintln(ctx.Out)
			fmt.Fprintln(ctx.Out, "Note: Forgejo's API does not expose job logs (probed on Gitea 1.22 / Forgejo 15.0.2).")
			fmt.Fprintf(ctx.Out, "  - View in browser: %s/%s/actions/runs/%s\n", forgejoBaseURL(ctx), repo, runIndex)
			fmt.Fprintln(ctx.Out, "  - On the Forgejo host, set FORGEJO_LOG_PATH in ~/.config/forgejo-cli/config")
			fmt.Fprintln(ctx.Out, "    (e.g. FORGEJO_LOG_PATH=/srv/forgejo/data/gitea/actions_log) to read from disk.")
			return nil
		},
	}
}

func newActionsDispatchCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dispatch <owner/repo> --workflow=X --ref=Y [--input=k=v]...",
		Short: "Trigger a workflow",
		Long: strings.TrimSpace(`Trigger a workflow dispatch event for a required repository.

The repository positional is required; pass "." to infer owner/repo from the
current git remote. --workflow names the workflow file or workflow id, --ref is
the branch or tag to run, and each --input must be k=v. Repeat --input for
multiple workflow inputs.`),
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runWorkflowDispatch(ctx, cmd, args)
		},
	}
	addWorkflowDispatchFlags(cmd)
	return cmd
}

func newActionsRunnerCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "runner <list|register|delete>",
		Short: "Manage Actions runners",
		Long: strings.TrimSpace(`Manage Actions runners scoped to a repository, an organization,
or the whole instance.

Pass <owner/repo> for repository scope, <org> for organization scope, or
--admin for instance scope.`),
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmdutil.Usagef("Usage: forgejo actions runner <list|register|delete> [args]")
		},
	}
	cmd.AddCommand(newActionsRunnerListCmd(ctx), newActionsRunnerRegisterCmd(ctx), newActionsRunnerDeleteCmd(ctx))
	return cmd
}

func newActionsRunnerListCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list [<owner/repo|org>|--admin]",
		Short: "List Actions runners",
		Long: strings.TrimSpace(`List Actions runners for one scope.

Pass <owner/repo> for repository scope, <org> for organization scope, or
--admin for instance scope. Exactly one scope is required.`),
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			scope, _, err := actionsScope(ctx, cmd, args, 0)
			if err != nil {
				return err
			}
			raw, err := ctx.Client.Do("GET", scope+"/runners", nil)
			if err != nil {
				return err
			}
			if ctx.WantsJSON() {
				return ctx.EmitJSON(raw)
			}
			runners, err := runnerItems(raw)
			if err != nil {
				return err
			}
			rows := make([][]string, 0, len(runners))
			for _, r := range runners {
				rows = append(rows, []string{
					cmdutil.Str(r, "id"),
					dash(cmdutil.Str(r, "name")),
					dash(cmdutil.Str(r, "status")),
					renderLabels(r["labels"]),
					dash(cmdutil.Str(r, "version")),
				})
			}
			ctx.Table([]string{"ID", "NAME", "STATUS", "LABELS", "VERSION"}, rows)
			return nil
		},
	}
	addAdminScopeFlag(cmd)
	return cmd
}

func newActionsRunnerRegisterCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "register [<owner/repo|org>|--admin]",
		Short: "Print runner registration token",
		Long: strings.TrimSpace(`Print the Actions runner registration token for one scope.

Pass <owner/repo> for repository scope, <org> for organization scope, or
--admin for instance scope. Exactly one scope is required.`),
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			scope, _, err := actionsScope(ctx, cmd, args, 0)
			if err != nil {
				return err
			}
			raw, err := ctx.Client.Do("GET", scope+"/runners/registration-token", nil)
			if err != nil {
				return err
			}
			if ctx.WantsJSON() {
				return ctx.EmitJSON(raw)
			}
			obj, err := cmdutil.ParseObject(raw)
			if err != nil {
				return err
			}
			fmt.Fprintln(ctx.Out, cmdutil.Str(obj, "token"))
			return nil
		},
	}
	addAdminScopeFlag(cmd)
	return cmd
}

func newActionsRunnerDeleteCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete [<owner/repo|org>|--admin] <runner_id> [--yes]",
		Short: "Remove an Actions runner",
		Long: strings.TrimSpace(`Delete an Actions runner from one scope.

Pass <owner/repo> for repository scope, <org> for organization scope, or
--admin for instance scope. The runner id is required. Deletion requires --yes
or an interactive typed-name confirmation.`),
		Args: cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			scope, rest, err := actionsScope(ctx, cmd, args, 1)
			if err != nil {
				return err
			}
			if len(rest) == 0 {
				return cmdutil.Usagef("Usage: forgejo actions runner delete <owner/repo|org|--admin> <runner_id> [--yes]")
			}
			runnerID, err := cmdutil.IDArg(rest[0], "runner id")
			if err != nil {
				return err
			}
			if err := ctx.ConfirmDelete(cmd, "runner", runnerID); err != nil {
				return err
			}
			if _, err := ctx.Client.Do("DELETE", scope+"/runners/"+runnerID, nil); err != nil {
				return err
			}
			fmt.Fprintf(ctx.Out, "Deleted runner %s\n", runnerID)
			return nil
		},
	}
	addAdminScopeFlag(cmd)
	cmdutil.AddYesFlag(cmd)
	return cmd
}

func newActionsWorkflowCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "workflow <list|enable|disable|dispatch>",
		Short: "List, toggle, and dispatch workflows",
		Long: strings.TrimSpace(`List workflow files from a repository, enable or disable a
workflow, or trigger a workflow dispatch.

Workflow listing uses the contents API because Forgejo has no direct workflow
list endpoint. It checks .forgejo/workflows, then .gitea/workflows, then
.github/workflows, and returns the first directory containing YAML files.`),
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmdutil.Usagef("Usage: forgejo actions workflow <list|enable|disable|dispatch> [args]")
		},
	}
	cmd.AddCommand(
		newActionsWorkflowListCmd(ctx),
		newActionsWorkflowEnableCmd(ctx),
		newActionsWorkflowDisableCmd(ctx),
		newActionsWorkflowDispatchCmd(ctx),
	)
	return cmd
}

func newActionsWorkflowListCmd(ctx *cmdutil.Ctx) *cobra.Command {
	return &cobra.Command{
		Use:   "list <owner/repo>",
		Short: "List workflow files in a repository",
		Long: strings.TrimSpace(`List workflow YAML files in a required repository.

The repository positional is required; pass "." to infer owner/repo from the
current git remote. Forgejo has no direct workflow-list endpoint, so this reads
.forgejo/workflows, .gitea/workflows, then .github/workflows through the
contents API and prints the first directory containing .yml or .yaml files.`),
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			repo, err := ctx.RepoArg(args[0])
			if err != nil {
				return err
			}
			return listWorkflowFiles(ctx, repo)
		},
	}
}

func newActionsWorkflowEnableCmd(ctx *cmdutil.Ctx) *cobra.Command {
	return &cobra.Command{
		Use:   "enable <owner/repo> <workflow>",
		Short: "Enable a workflow",
		Long: strings.TrimSpace(`Enable a workflow for a required repository.

The repository positional is required; pass "." to infer owner/repo from the
current git remote. The workflow is sent as a single path segment, so names,
ids, and workflow file paths are percent-escaped before calling the API.`),
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runWorkflowToggle(ctx, args, true)
		},
	}
}

func newActionsWorkflowDisableCmd(ctx *cmdutil.Ctx) *cobra.Command {
	return &cobra.Command{
		Use:   "disable <owner/repo> <workflow>",
		Short: "Disable a workflow",
		Long: strings.TrimSpace(`Disable a workflow for a required repository.

The repository positional is required; pass "." to infer owner/repo from the
current git remote. The workflow is sent as a single path segment, so names,
ids, and workflow file paths are percent-escaped before calling the API.`),
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runWorkflowToggle(ctx, args, false)
		},
	}
}

func newActionsWorkflowDispatchCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dispatch <owner/repo> --workflow=X --ref=Y [--input=k=v]...",
		Short: "Trigger a workflow",
		Long: strings.TrimSpace(`Trigger a workflow dispatch event for a required repository.

The repository positional is required; pass "." to infer owner/repo from the
current git remote. --workflow names the workflow file or workflow id, --ref is
the branch or tag to run, and each --input must be k=v. Repeat --input for
multiple workflow inputs.`),
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runWorkflowDispatch(ctx, cmd, args)
		},
	}
	addWorkflowDispatchFlags(cmd)
	return cmd
}

func newActionsSecretCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "secret <list|set|delete>",
		Short: "Manage Actions secrets",
		Long: strings.TrimSpace(`Manage Actions secrets scoped to a repository, an
organization, or the whole instance.

Pass <owner/repo> for repository scope, <org> for organization scope, or
--admin for instance scope. Secret values are write-only; list shows names and
creation times.`),
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmdutil.Usagef("Usage: forgejo actions secret <list|set|delete> [args]")
		},
	}
	cmd.AddCommand(newActionsSecretListCmd(ctx), newActionsSecretSetCmd(ctx), newActionsSecretDeleteCmd(ctx))
	return cmd
}

func newActionsSecretListCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list [<owner/repo|org>|--admin]",
		Short: "List secret names",
		Long: strings.TrimSpace(`List Actions secrets for one scope.

Pass <owner/repo> for repository scope, <org> for organization scope, or
--admin for instance scope. Exactly one scope is required. Secret values are
not returned by Forgejo.`),
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			scope, _, err := actionsScope(ctx, cmd, args, 0)
			if err != nil {
				return err
			}
			raw, err := ctx.Client.Do("GET", scope+"/secrets", nil)
			if err != nil {
				return err
			}
			if ctx.WantsJSON() {
				return ctx.EmitJSON(raw)
			}
			items, err := cmdutil.ParseArray(raw)
			if err != nil {
				return err
			}
			rows := make([][]string, 0, len(items))
			for _, item := range items {
				created := dash(cmdutil.Str(item, "created_at"))
				if created != "-" {
					created = cmdutil.Trunc(created, 19)
				}
				rows = append(rows, []string{dash(cmdutil.Str(item, "name")), created})
			}
			ctx.Table([]string{"NAME", "CREATED"}, rows)
			return nil
		},
	}
	addAdminScopeFlag(cmd)
	return cmd
}

func newActionsSecretSetCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "set [<owner/repo|org>|--admin] --name=X --value=X",
		Short: "Create or update a secret",
		Long: strings.TrimSpace(`Create or update an Actions secret for one scope.

Pass <owner/repo> for repository scope, <org> for organization scope, or
--admin for instance scope. --name and --value are required and must be
non-empty. The request body uses Forgejo's discovered {data: value} field.`),
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			scope, _, err := actionsScope(ctx, cmd, args, 0)
			if err != nil {
				return err
			}
			name, _ := cmd.Flags().GetString("name")
			value, _ := cmd.Flags().GetString("value")
			if name == "" || value == "" {
				return cmdutil.Usagef("Usage: forgejo actions secret set <owner/repo|org> --name=X --value=X")
			}
			body, err := cmdutil.BuildBody(map[string]any{"data": value})
			if err != nil {
				return err
			}
			status, out, err := ctx.Client.DoStatus("PUT", scope+"/secrets/"+cmdutil.NameSeg(name), body)
			if err != nil {
				return err
			}
			if status < 200 || status >= 300 {
				return statusFailure("Failed to set secret", status, out)
			}
			fmt.Fprintf(ctx.Out, "Set secret %s\n", name)
			return nil
		},
	}
	addAdminScopeFlag(cmd)
	cmd.Flags().String("name", "", "secret name (required)")
	cmd.Flags().String("value", "", "secret value (required; write-only)")
	return cmd
}

func newActionsSecretDeleteCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete [<owner/repo|org>|--admin] <name> [--yes]",
		Short: "Delete a secret",
		Long: strings.TrimSpace(`Delete an Actions secret from one scope.

Pass <owner/repo> for repository scope, <org> for organization scope, or
--admin for instance scope. The secret name is required and is escaped as a
single path segment. Deletion requires --yes or an interactive typed-name
confirmation.`),
		Args: cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			scope, rest, err := actionsScope(ctx, cmd, args, 1)
			if err != nil {
				return err
			}
			if len(rest) == 0 {
				return cmdutil.Usagef("Usage: forgejo actions secret delete <owner/repo|org> <name> [--yes]")
			}
			name := rest[0]
			if err := ctx.ConfirmDelete(cmd, "secret", name); err != nil {
				return err
			}
			if _, err := ctx.Client.Do("DELETE", scope+"/secrets/"+cmdutil.NameSeg(name), nil); err != nil {
				return err
			}
			fmt.Fprintf(ctx.Out, "Deleted secret %s\n", name)
			return nil
		},
	}
	addAdminScopeFlag(cmd)
	cmdutil.AddYesFlag(cmd)
	return cmd
}

func newActionsVariableCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "variable <list|get|set|delete>",
		Short: "Manage Actions variables",
		Long: strings.TrimSpace(`Manage Actions variables scoped to a repository, an
organization, or the whole instance.

Pass <owner/repo> for repository scope, <org> for organization scope, or
--admin for instance scope. Variable values are readable. Set is idempotent:
the command GETs the variable first and then PUTs an existing variable or POSTs
a new one.`),
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmdutil.Usagef("Usage: forgejo actions variable <list|get|set|delete> [args]")
		},
	}
	cmd.AddCommand(
		newActionsVariableListCmd(ctx),
		newActionsVariableGetCmd(ctx),
		newActionsVariableSetCmd(ctx),
		newActionsVariableDeleteCmd(ctx),
	)
	return cmd
}

func newActionsVariableListCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list [<owner/repo|org>|--admin]",
		Short: "List variables",
		Long: strings.TrimSpace(`List Actions variables for one scope.

Pass <owner/repo> for repository scope, <org> for organization scope, or
--admin for instance scope. Exactly one scope is required. Values are printed
in text mode because Forgejo returns them as the data field.`),
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			scope, _, err := actionsScope(ctx, cmd, args, 0)
			if err != nil {
				return err
			}
			raw, err := ctx.Client.Do("GET", scope+"/variables", nil)
			if err != nil {
				return err
			}
			if ctx.WantsJSON() {
				return ctx.EmitJSON(raw)
			}
			items, err := cmdutil.ParseArray(raw)
			if err != nil {
				return err
			}
			rows := make([][]string, 0, len(items))
			for _, item := range items {
				rows = append(rows, []string{dash(cmdutil.Str(item, "name")), dash(cmdutil.Str(item, "data"))})
			}
			ctx.Table([]string{"NAME", "VALUE"}, rows)
			return nil
		},
	}
	addAdminScopeFlag(cmd)
	return cmd
}

func newActionsVariableGetCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get [<owner/repo|org>|--admin] <name>",
		Short: "Get a variable",
		Long: strings.TrimSpace(`Get one Actions variable from one scope.

Pass <owner/repo> for repository scope, <org> for organization scope, or
--admin for instance scope. The variable name is required and is escaped as a
single path segment. With --json, the raw variable JSON is printed.`),
		Args: cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			scope, rest, err := actionsScope(ctx, cmd, args, 1)
			if err != nil {
				return err
			}
			if len(rest) == 0 {
				return cmdutil.Usagef("Usage: forgejo actions variable get <owner/repo|org> <name>")
			}
			name := rest[0]
			raw, err := ctx.Client.Do("GET", scope+"/variables/"+cmdutil.NameSeg(name), nil)
			if err != nil {
				return err
			}
			if ctx.WantsJSON() {
				return ctx.EmitJSON(raw)
			}
			obj, err := cmdutil.ParseObject(raw)
			if err != nil {
				return err
			}
			ctx.Table([]string{"NAME", "VALUE"}, [][]string{{dash(cmdutil.Str(obj, "name")), dash(cmdutil.Str(obj, "data"))}})
			return nil
		},
	}
	addAdminScopeFlag(cmd)
	return cmd
}

func newActionsVariableSetCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "set [<owner/repo|org>|--admin] --name=X --value=X",
		Short: "Create or update a variable",
		Long: strings.TrimSpace(`Create or update an Actions variable for one scope.

Pass <owner/repo> for repository scope, <org> for organization scope, or
--admin for instance scope. --name and --value are required and must be
non-empty. This preserves the bash command's idempotent GET-first flow: a
successful GET is followed by PUT, and any non-2xx probe is followed by POST.
Both create and update bodies use Forgejo's discovered {value: value} field.`),
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			scope, _, err := actionsScope(ctx, cmd, args, 0)
			if err != nil {
				return err
			}
			name, _ := cmd.Flags().GetString("name")
			value, _ := cmd.Flags().GetString("value")
			if name == "" || value == "" {
				return cmdutil.Usagef("Usage: forgejo actions variable set <owner/repo|org> --name=X --value=X")
			}
			body, err := cmdutil.BuildBody(map[string]any{"value": value})
			if err != nil {
				return err
			}
			endpoint := scope + "/variables/" + cmdutil.NameSeg(name)
			probeStatus, _, err := ctx.Client.DoStatus("GET", endpoint, nil)
			if err != nil {
				return err
			}
			if probeStatus >= 200 && probeStatus < 300 {
				status, out, err := ctx.Client.DoStatus("PUT", endpoint, body)
				if err != nil {
					return err
				}
				if status < 200 || status >= 300 {
					return statusFailure("Failed to update variable", status, out)
				}
				fmt.Fprintf(ctx.Out, "Updated variable %s\n", name)
				return nil
			}
			status, out, err := ctx.Client.DoStatus("POST", endpoint, body)
			if err != nil {
				return err
			}
			if status < 200 || status >= 300 {
				return statusFailure("Failed to create variable", status, out)
			}
			fmt.Fprintf(ctx.Out, "Created variable %s\n", name)
			return nil
		},
	}
	addAdminScopeFlag(cmd)
	cmd.Flags().String("name", "", "variable name (required)")
	cmd.Flags().String("value", "", "variable value (required)")
	return cmd
}

func newActionsVariableDeleteCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete [<owner/repo|org>|--admin] <name> [--yes]",
		Short: "Delete a variable",
		Long: strings.TrimSpace(`Delete an Actions variable from one scope.

Pass <owner/repo> for repository scope, <org> for organization scope, or
--admin for instance scope. The variable name is required and is escaped as a
single path segment. Deletion requires --yes or an interactive typed-name
confirmation.`),
		Args: cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			scope, rest, err := actionsScope(ctx, cmd, args, 1)
			if err != nil {
				return err
			}
			if len(rest) == 0 {
				return cmdutil.Usagef("Usage: forgejo actions variable delete <owner/repo|org> <name> [--yes]")
			}
			name := rest[0]
			if err := ctx.ConfirmDelete(cmd, "variable", name); err != nil {
				return err
			}
			if _, err := ctx.Client.Do("DELETE", scope+"/variables/"+cmdutil.NameSeg(name), nil); err != nil {
				return err
			}
			fmt.Fprintf(ctx.Out, "Deleted variable %s\n", name)
			return nil
		},
	}
	addAdminScopeFlag(cmd)
	cmdutil.AddYesFlag(cmd)
	return cmd
}

func addAdminScopeFlag(cmd *cobra.Command) {
	cmd.Flags().Bool("admin", false, "use instance-wide Actions scope instead of repo/org scope")
}

func addWorkflowDispatchFlags(cmd *cobra.Command) {
	cmd.Flags().String("workflow", "", "workflow file name or workflow id (required)")
	cmd.Flags().String("ref", "", "branch or tag ref to dispatch (required)")
	cmd.Flags().StringArray("input", nil, "workflow input as k=v (repeatable)")
}

func actionsScope(ctx *cmdutil.Ctx, cmd *cobra.Command, args []string, trailing int) (string, []string, error) {
	admin, _ := cmd.Flags().GetBool("admin")
	scopeArgs := len(args) - trailing
	if scopeArgs < 0 {
		scopeArgs = 0
	}
	if admin {
		if scopeArgs > 0 {
			return "", nil, cmdutil.Usagef("Ambiguous scope: expected exactly one of <owner/repo>, <org>, or --admin")
		}
		return "admin/actions", args, nil
	}
	if scopeArgs == 0 {
		return "", nil, cmdutil.Usagef("Missing scope: expected <owner/repo>, <org>, or --admin")
	}
	if scopeArgs > 1 {
		return "", nil, cmdutil.Usagef("Ambiguous scope: expected exactly one of <owner/repo>, <org>, or --admin")
	}
	scope := args[0]
	rest := args[1:]
	if scope == "." || strings.Contains(scope, "/") {
		repo, err := ctx.RepoArg(scope)
		if err != nil {
			return "", nil, err
		}
		return "repos/" + repo + "/actions", rest, nil
	}
	return "orgs/" + cmdutil.NameSeg(scope) + "/actions", rest, nil
}

func printRun(ctx *cmdutil.Ctx, run map[string]any) {
	duration := "-"
	if d := cmdutil.Str(run, "duration"); d != "" {
		duration = durationSeconds(d)
	}
	fmt.Fprintf(ctx.Out, "Run #%s\n", cmdutil.Str(run, "id"))
	fmt.Fprintf(ctx.Out, "Title:      %s\n", dash(cmdutil.Str(run, "title")))
	fmt.Fprintf(ctx.Out, "Status:     %s\n", dash(cmdutil.Str(run, "status")))
	fmt.Fprintf(ctx.Out, "Event:      %s\n", dash(cmdutil.Str(run, "event")))
	fmt.Fprintf(ctx.Out, "Branch:     %s\n", dash(cmdutil.Str(run, "prettyref")))
	fmt.Fprintf(ctx.Out, "Commit:     %s\n", cmdutil.Trunc(dash(cmdutil.Str(run, "commit_sha")), 12))
	fmt.Fprintf(ctx.Out, "Started:    %s\n", firstNonEmpty(cmdutil.Str(run, "started"), cmdutil.Str(run, "created"), "-"))
	fmt.Fprintf(ctx.Out, "Stopped:    %s\n", dash(cmdutil.Str(run, "stopped")))
	fmt.Fprintf(ctx.Out, "Duration:   %s\n", duration)
	fmt.Fprintf(ctx.Out, "Trigger:    %s\n", dash(cmdutil.Str(run, "trigger_user.login")))
}

func durationSeconds(s string) string {
	if n, err := strconv.ParseInt(s, 10, 64); err == nil {
		return fmt.Sprintf("%ds", n/1_000_000_000)
	}
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		return fmt.Sprintf("%ds", int64(f/1_000_000_000))
	}
	return "-"
}

func tasksForRun(ctx *cmdutil.Ctx, repo, runIndex string) ([]map[string]any, error) {
	raw, err := ctx.Client.Do("GET", "repos/"+repo+"/actions/tasks?limit=200", nil)
	if err != nil {
		return nil, err
	}
	tasks, err := workflowRuns(raw)
	if err != nil {
		return nil, err
	}
	var out []map[string]any
	for _, task := range tasks {
		if cmdutil.Str(task, "run_number") == runIndex {
			out = append(out, task)
		}
	}
	return out, nil
}

func workflowRuns(raw []byte) ([]map[string]any, error) {
	obj, err := cmdutil.ParseObject(raw)
	if err != nil {
		return nil, err
	}
	return mapsFromAny(obj["workflow_runs"]), nil
}

func runnerItems(raw []byte) ([]map[string]any, error) {
	var v any
	if err := decodeJSON(raw, &v); err != nil {
		return nil, err
	}
	if obj, ok := v.(map[string]any); ok {
		return mapsFromAny(obj["runners"]), nil
	}
	return mapsFromAny(v), nil
}

func mapsFromAny(v any) []map[string]any {
	items, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		if m, ok := item.(map[string]any); ok {
			out = append(out, m)
		}
	}
	return out
}

func decodeJSON(raw []byte, v any) error {
	dec := json.NewDecoder(strings.NewReader(string(raw)))
	dec.UseNumber()
	if err := dec.Decode(v); err != nil {
		return fmt.Errorf("unexpected response shape: %w", err)
	}
	return nil
}

func printDiskLogs(ctx *cmdutil.Ctx, logRoot, repo string, tasks []map[string]any) error {
	for _, task := range tasks {
		taskID := cmdutil.Str(task, "id")
		taskName := cmdutil.Str(task, "name")
		taskStatus := cmdutil.Str(task, "status")
		shard, err := taskShard(taskID)
		if err != nil {
			return err
		}
		logPath := filepath.Join(logRoot, filepath.FromSlash(repo), shard, taskID+".log.zst")
		fmt.Fprintf(ctx.Out, "=== Job: %s (task_id=%s, status=%s) ===\n", taskName, taskID, taskStatus)
		if err := decompressLog(ctx.Out, logPath); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				fmt.Fprintf(ctx.Out, "(log not on disk: %s)\n", logPath)
			} else {
				return err
			}
		}
		fmt.Fprintln(ctx.Out)
	}
	return nil
}

func taskShard(taskID string) (string, error) {
	n, err := strconv.ParseUint(taskID, 10, 64)
	if err != nil {
		return "", fmt.Errorf("invalid task id %q: %w", taskID, err)
	}
	return fmt.Sprintf("%02x", n&0xff), nil
}

func decompressLog(out io.Writer, path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	dec, err := zstd.NewReader(f)
	if err != nil {
		return err
	}
	defer dec.Close()
	_, err = io.Copy(out, dec)
	return err
}

type workflowFile struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

func listWorkflowFiles(ctx *cmdutil.Ctx, repo string) error {
	for _, path := range []string{".forgejo/workflows", ".gitea/workflows", ".github/workflows"} {
		raw, err := ctx.Client.Do("GET", "repos/"+repo+"/contents/"+escapePath(path), nil)
		if err != nil {
			continue
		}
		var contents []map[string]any
		if err := decodeJSON(raw, &contents); err != nil {
			return err
		}
		var files []workflowFile
		for _, item := range contents {
			name := cmdutil.Str(item, "name")
			if cmdutil.Str(item, "type") == "file" && isWorkflowYAML(name) {
				files = append(files, workflowFile{Name: name, Path: path + "/" + name})
			}
		}
		if len(files) == 0 {
			continue
		}
		if ctx.WantsJSON() {
			out, err := json.Marshal(files)
			if err != nil {
				return err
			}
			return ctx.EmitJSON(out)
		}
		rows := make([][]string, 0, len(files))
		for _, f := range files {
			rows = append(rows, []string{f.Name, f.Path})
		}
		ctx.Table([]string{"NAME", "PATH"}, rows)
		return nil
	}
	return &cmdutil.ExitError{Code: 1, Err: fmt.Errorf("No workflows found in .forgejo/workflows, .gitea/workflows or .github/workflows of %s", repo)}
}

func isWorkflowYAML(name string) bool {
	lower := strings.ToLower(name)
	return strings.HasSuffix(lower, ".yml") || strings.HasSuffix(lower, ".yaml")
}

func escapePath(path string) string {
	parts := strings.Split(path, "/")
	for i, part := range parts {
		parts[i] = cmdutil.PathEscape(part)
	}
	return strings.Join(parts, "/")
}

func runWorkflowToggle(ctx *cmdutil.Ctx, args []string, enable bool) error {
	repo, err := ctx.RepoArg(args[0])
	if err != nil {
		return err
	}
	workflow := args[1]
	action := "disable"
	past := "Disabled"
	if enable {
		action = "enable"
		past = "Enabled"
	}
	endpoint := "repos/" + repo + "/actions/workflows/" + cmdutil.NameSeg(workflow) + "/" + action
	if _, err := ctx.Client.Do("PUT", endpoint, nil); err != nil {
		return err
	}
	fmt.Fprintf(ctx.Out, "%s workflow %s\n", past, workflow)
	return nil
}

func runWorkflowDispatch(ctx *cmdutil.Ctx, cmd *cobra.Command, args []string) error {
	repo, err := ctx.RepoArg(args[0])
	if err != nil {
		return err
	}
	workflow, _ := cmd.Flags().GetString("workflow")
	ref, _ := cmd.Flags().GetString("ref")
	inputs, _ := cmd.Flags().GetStringArray("input")
	if workflow == "" || ref == "" {
		usage := "Usage: forgejo actions workflow dispatch <owner/repo> --workflow=X --ref=Y [--input=k=v]..."
		if cmd.Parent() != nil && cmd.Parent().Name() == "actions" {
			usage = "Usage: forgejo actions dispatch <owner/repo> --workflow=X --ref=Y [--input=k=v]..."
		}
		return cmdutil.Usagef("%s", usage)
	}
	parsedInputs := map[string]string{}
	for _, input := range inputs {
		k, v, ok := strings.Cut(input, "=")
		if !ok {
			return cmdutil.Usagef("Invalid --input (expected k=v): --input=%s", input)
		}
		parsedInputs[k] = v
	}
	body, err := cmdutil.BuildBody(map[string]any{"ref": ref, "inputs": parsedInputs})
	if err != nil {
		return err
	}
	if _, err := ctx.Client.Do("POST", "repos/"+repo+"/actions/workflows/"+cmdutil.NameSeg(workflow)+"/dispatches", body); err != nil {
		return err
	}
	fmt.Fprintf(ctx.Out, "Dispatched %s on %s\n", workflow, ref)
	return nil
}

func renderLabels(v any) string {
	items, ok := v.([]any)
	if !ok || len(items) == 0 {
		return "-"
	}
	parts := make([]string, 0, len(items))
	for _, item := range items {
		switch t := item.(type) {
		case string:
			parts = append(parts, t)
		default:
			parts = append(parts, fmt.Sprintf("%v", t))
		}
	}
	if len(parts) == 0 {
		return "-"
	}
	return strings.Join(parts, ",")
}

func statusFailure(prefix string, status int, body []byte) error {
	msg := strings.TrimSpace(string(body))
	if msg == "" {
		msg = "<empty response>"
	}
	return &cmdutil.ExitError{Code: 1, Err: fmt.Errorf("%s (HTTP %d):\n%s", prefix, status, msg)}
}

func writerIsTerminal(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	st, err := f.Stat()
	return err == nil && st.Mode()&os.ModeCharDevice != 0
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

func configLogPath(ctx *cmdutil.Ctx) string {
	if ctx.Config == nil {
		return ""
	}
	return ctx.Config.LogPath
}

func forgejoBaseURL(ctx *cmdutil.Ctx) string {
	if ctx.Config != nil && ctx.Config.URL != "" {
		return ctx.Config.URL
	}
	if ctx.Client != nil {
		return strings.TrimRight(ctx.Client.BaseURL, "/")
	}
	return ""
}
