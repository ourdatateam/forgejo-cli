package cmd

// Ported from the bash cmd_pr family (forgejo:3850-4650). The PR view and
// review lookup commands intentionally assemble data from several endpoints so
// inline review threads and general issue comments are visible in one result.

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"

	"github.com/ourdatateam/forgejo-cli/internal/api"
	"github.com/ourdatateam/forgejo-cli/internal/cmdutil"
	"github.com/spf13/cobra"
)

func init() { Register(newPrCmd) }

var prWIPPrefixRe = regexp.MustCompile(`(?i)^[[:space:]]*(\[WIP\]|WIP:)[[:space:]]*`)

func newPrCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pr <list|create|view|edit|close|reopen|merge|diff|patch|checks|ready|comment|review|files>",
		Short: "Work with pull requests",
		Long: `Pull request commands.

The repo positional is always required. Pass "." explicitly to infer the repo
from a git remote on the configured Forgejo host. Use --json for raw JSON where
the verb returns JSON; pr diff and pr patch stream raw bytes instead.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmdutil.Usagef("Usage: forgejo pr <list|create|view|edit|close|reopen|merge|diff|patch|checks|ready|comment|review|files> [args]")
		},
	}
	cmd.AddCommand(
		newPrListCmd(ctx),
		newPrCreateCmd(ctx),
		newPrViewCmd(ctx),
		newPrEditCmd(ctx),
		newPrCloseCmd(ctx),
		newPrReopenCmd(ctx),
		newPrMergeCmd(ctx),
		newPrDiffCmd(ctx),
		newPrPatchCmd(ctx),
		newPrChecksCmd(ctx),
		newPrReadyCmd(ctx),
		newPrCommentCmd(ctx),
		newPrReviewCmd(ctx),
		newPrFilesCmd(ctx),
		newPrRequestedReviewersCmd(ctx),
	)
	return cmd
}

func newPrListCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list <owner/repo> [--state=open|closed|all]",
		Short: "List pull requests",
		Long: `List pull requests for a repository.

The repo positional is required; "." triggers git-remote inference. The state
filter defaults to open and accepts the same values as the bash command:
open, closed, or all.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return cmdutil.Usagef("Usage: forgejo pr list <owner/repo> [--state=open|closed|all]")
			}
			repo, err := ctx.RepoArg(args[0])
			if err != nil {
				return err
			}
			state, _ := cmd.Flags().GetString("state")
			endpoint := fmt.Sprintf("repos/%s/pulls?state=%s", repo, cmdutil.QueryEscape(state))
			limit := ctx.ListLimit(50)
			lr, err := ctx.Client.DoList(endpoint, limit)
			if err != nil {
				return err
			}
			if ctx.WantsJSON() {
				return ctx.EmitJSON(lr.Body)
			}
			items, err := cmdutil.ParseArray(lr.Body)
			if err != nil {
				return err
			}
			if len(items) == 0 {
				fmt.Fprintln(ctx.Out, "No pull requests found.")
				return nil
			}
			rows := make([][]string, 0, len(items))
			for _, m := range items {
				rows = append(rows, []string{
					cmdutil.Str(m, "number"),
					cmdutil.Trunc(cmdutil.Str(m, "title"), 50),
					cmdutil.Str(m, "state"),
					cmdutil.Str(m, "user.login"),
					cmdutil.Str(m, "base.ref"),
					cmdutil.Str(m, "head.ref"),
					prDate(cmdutil.Str(m, "updated_at")),
				})
			}
			ctx.Table([]string{"#", "TITLE", "STATE", "AUTHOR", "BASE", "HEAD", "UPDATED"}, rows)
			ctx.Trailer(len(items), lr.Total, limit)
			return nil
		},
	}
	cmd.Flags().String("state", "open", "PR state filter: open, closed, or all")
	return cmd
}

func newPrCreateCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create <owner/repo> --title=TEXT --head=BRANCH [--base=main] [--body=TEXT]",
		Short: "Create a pull request",
		Long: `Create a new pull request.

The repo positional, --title, and --head are required. --base defaults to main.
The PR body may be supplied with --body, --body=-, --body-file=PATH, or
--body-file=-. If no body flag is supplied, the body is empty.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return cmdutil.Usagef("Usage: forgejo pr create <owner/repo> --title=X --head=X [--base=main] [--body=X]")
			}
			repo, err := ctx.RepoArg(args[0])
			if err != nil {
				return err
			}
			title, _ := cmd.Flags().GetString("title")
			head, _ := cmd.Flags().GetString("head")
			base, _ := cmd.Flags().GetString("base")
			if title == "" {
				return cmdutil.Usagef("Missing --title")
			}
			if head == "" {
				return cmdutil.Usagef("Missing --head")
			}
			bodyText, _, err := ctx.Body(cmd)
			if err != nil {
				return err
			}
			body, err := cmdutil.BuildBody(map[string]any{
				"title": title,
				"head":  head,
				"base":  base,
				"body":  bodyText,
			})
			if err != nil {
				return err
			}
			raw, err := ctx.Client.Do("POST", "repos/"+repo+"/pulls", body)
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
			fmt.Fprintf(ctx.Out, "Created PR #%s: %s\nURL: %s\n",
				cmdutil.Str(obj, "number"), cmdutil.Str(obj, "title"), cmdutil.Str(obj, "html_url"))
			return nil
		},
	}
	cmd.Flags().String("title", "", "PR title (required)")
	cmd.Flags().String("head", "", "head branch for the PR (required)")
	cmd.Flags().String("base", "main", "base branch to merge into")
	cmdutil.AddBodyFlags(cmd)
	return cmd
}

func newPrViewCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "view <owner/repo> <number>",
		Short: "View PR details and conversation",
		Long: `View a PR with its complete conversation.

The output includes the pull request body, all reviews, inline review-thread
comments nested under their review, and general issue comments in
issue_comments. The review and comment lists are fetched with paged API calls.
Use --json for the complete machine-readable object.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			repo, number, err := prRepoNumber(ctx, args, "Usage: forgejo pr view <owner/repo> <number>")
			if err != nil {
				return err
			}
			conv, err := prConversation(ctx, repo, number)
			if err != nil {
				return err
			}
			raw, err := json.Marshal(conv.pull)
			if err != nil {
				return err
			}
			if ctx.WantsJSON() {
				return ctx.EmitJSON(raw)
			}
			prPrintConversation(ctx, conv)
			return nil
		},
	}
	return cmd
}

func newPrEditCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "edit <owner/repo> <number> [--title=TEXT] [--body=TEXT] [--base=BRANCH]",
		Short: "Edit a pull request",
		Long: `Edit a PR's title, body, or base branch.

At least one of --title, --body/--body-file, or --base is required. --body=-
and --body-file=- read the replacement body from stdin.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			repo, number, err := prRepoNumber(ctx, args, "Usage: forgejo pr edit <owner/repo> <number> [--title=X] [--body=X] [--base=X]")
			if err != nil {
				return err
			}
			fields := map[string]any{}
			if cmd.Flags().Changed("title") {
				title, _ := cmd.Flags().GetString("title")
				fields["title"] = title
			}
			bodyText, hasBody, err := ctx.Body(cmd)
			if err != nil {
				return err
			}
			if hasBody {
				fields["body"] = bodyText
			}
			if cmd.Flags().Changed("base") {
				base, _ := cmd.Flags().GetString("base")
				fields["base"] = base
			}
			if len(fields) == 0 {
				return cmdutil.Usagef("No fields to update. Provide --title, --body, or --base.")
			}
			body, err := cmdutil.BuildBody(fields)
			if err != nil {
				return err
			}
			raw, err := ctx.Client.Do("PATCH", "repos/"+repo+"/pulls/"+number, body)
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
			fmt.Fprintf(ctx.Out, "Updated PR #%s: %s\n", cmdutil.Str(obj, "number"), cmdutil.Str(obj, "title"))
			return nil
		},
	}
	cmd.Flags().String("title", "", "replacement PR title")
	cmd.Flags().String("base", "", "replacement base branch")
	cmdutil.AddBodyFlags(cmd)
	return cmd
}

func newPrCloseCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "close <owner/repo> <number>",
		Short: "Close a PR without merging",
		Long:  `Close a pull request without merging by PATCHing its state to closed.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			repo, number, err := prRepoNumber(ctx, args, "Usage: forgejo pr close <owner/repo> <number>")
			if err != nil {
				return err
			}
			raw, err := prPatchState(ctx, repo, number, "closed")
			if err != nil {
				return err
			}
			if ctx.WantsJSON() {
				return ctx.EmitJSON(raw)
			}
			fmt.Fprintf(ctx.Out, "Closed PR #%s\n", number)
			return nil
		},
	}
	return cmd
}

func newPrReopenCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "reopen <owner/repo> <number>",
		Short: "Reopen a PR",
		Long:  `Reopen a pull request by PATCHing its state to open.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			repo, number, err := prRepoNumber(ctx, args, "Usage: forgejo pr reopen <owner/repo> <number>")
			if err != nil {
				return err
			}
			raw, err := prPatchState(ctx, repo, number, "open")
			if err != nil {
				return err
			}
			if ctx.WantsJSON() {
				return ctx.EmitJSON(raw)
			}
			fmt.Fprintf(ctx.Out, "Reopened PR #%s\n", number)
			return nil
		},
	}
	return cmd
}

func newPrMergeCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "merge <owner/repo> <number> [--method=merge|rebase|squash]",
		Short: "Merge a pull request",
		Long: `Merge a pull request.

--method defaults to merge and is passed to the server as the Do field. The
bash command accepted merge, rebase, or squash and otherwise let the server
reject invalid methods.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			repo, number, err := prRepoNumber(ctx, args, "Usage: forgejo pr merge <owner/repo> <number> [--method=merge|rebase|squash]")
			if err != nil {
				return err
			}
			method, _ := cmd.Flags().GetString("method")
			body, err := cmdutil.BuildBody(map[string]any{"Do": method})
			if err != nil {
				return err
			}
			raw, err := ctx.Client.Do("POST", "repos/"+repo+"/pulls/"+number+"/merge", body)
			if err != nil {
				return err
			}
			if ctx.WantsJSON() {
				return ctx.EmitJSON(raw)
			}
			fmt.Fprintf(ctx.Out, "Merged PR #%s (method: %s)\n", number, method)
			return nil
		},
	}
	cmd.Flags().String("method", "merge", "merge method to request: merge, rebase, or squash")
	return cmd
}

func newPrDiffCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "diff <owner/repo> <number>",
		Short: "Show the raw unified diff",
		Long:  `Show the raw unified diff for a PR. This streams bytes with Accept: */* and does not apply JSON formatting.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return prRawPull(ctx, args, "diff", "Usage: forgejo pr diff <owner/repo> <number>")
		},
	}
	return cmd
}

func newPrPatchCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "patch <owner/repo> <number>",
		Short: "Show the raw patch",
		Long:  `Show the raw patch for a PR. This streams bytes with Accept: */* and does not apply JSON formatting.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return prRawPull(ctx, args, "patch", "Usage: forgejo pr patch <owner/repo> <number>")
		},
	}
	return cmd
}

func newPrChecksCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "checks <owner/repo> <number>",
		Short: "Show commit statuses and Actions checks",
		Long: `Show commit statuses and Actions CI runs for the PR head commit.

The PR head SHA is resolved first. Actions runs are looked up by head_sha, then
joined to jobs from repos/{repo}/actions/tasks?limit=200 by matching each run's
index_in_repo to each task's run_number. That tasks limit is a fixed internal
join limit and is not affected by --limit.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			repo, number, err := prRepoNumber(ctx, args, "Usage: forgejo pr checks <owner/repo> <number>")
			if err != nil {
				return err
			}
			result, err := prChecks(ctx, repo, number)
			if err != nil {
				return err
			}
			raw, err := json.Marshal(map[string]any{
				"statuses": result.statuses,
				"actions":  result.runs,
			})
			if err != nil {
				return err
			}
			if ctx.WantsJSON() {
				return ctx.EmitJSON(raw)
			}
			prPrintChecks(ctx, number, result)
			return nil
		},
	}
	return cmd
}

func newPrReadyCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ready <owner/repo> <number>",
		Short: "Mark a draft PR ready",
		Long: `Mark a draft PR as ready for review.

Forgejo silently drops PATCH {draft:false}; the bash workaround is to GET the
pull request and strip a leading WIP title prefix instead. This recognizes the
default Forgejo prefixes "WIP:" and "[WIP]" case-insensitively and then PATCHes
the title.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			repo, number, err := prRepoNumber(ctx, args, "Usage: forgejo pr ready <owner/repo> <number>")
			if err != nil {
				return err
			}
			raw, err := ctx.Client.Do("GET", "repos/"+repo+"/pulls/"+number, nil)
			if err != nil {
				return err
			}
			obj, err := cmdutil.ParseObject(raw)
			if err != nil {
				return err
			}
			if cmdutil.Str(obj, "draft") != "true" {
				return fmt.Errorf("PR #%s is not a draft; nothing to do", number)
			}
			title := cmdutil.Str(obj, "title")
			newTitle := prWIPPrefixRe.ReplaceAllString(title, "")
			if newTitle == title {
				return fmt.Errorf("PR #%s is a draft but its title has no recognised WIP prefix\n(this instance may use a custom prefix). Remove it manually:\n  forgejo pr edit %s %s --title=\"<title without WIP prefix>\"", number, repo, number)
			}
			body, err := cmdutil.BuildBody(map[string]any{"title": newTitle})
			if err != nil {
				return err
			}
			raw, err = ctx.Client.Do("PATCH", "repos/"+repo+"/pulls/"+number, body)
			if err != nil {
				return err
			}
			if ctx.WantsJSON() {
				return ctx.EmitJSON(raw)
			}
			fmt.Fprintf(ctx.Out, "Marked PR #%s ready for review\n", number)
			return nil
		},
	}
	return cmd
}

func newPrFilesCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "files <owner/repo> <number>",
		Short: "List changed files",
		Long:  `List files changed by a pull request, including status, additions, and deletions.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			repo, number, err := prRepoNumber(ctx, args, "Usage: forgejo pr files <owner/repo> <number>")
			if err != nil {
				return err
			}
			limit := ctx.ListLimit(50)
			lr, err := ctx.Client.DoList("repos/"+repo+"/pulls/"+number+"/files", limit)
			if err != nil {
				return err
			}
			if ctx.WantsJSON() {
				return ctx.EmitJSON(lr.Body)
			}
			items, err := cmdutil.ParseArray(lr.Body)
			if err != nil {
				return err
			}
			if len(items) == 0 {
				fmt.Fprintln(ctx.Out, "No files changed.")
				return nil
			}
			rows := make([][]string, 0, len(items))
			for _, m := range items {
				rows = append(rows, []string{
					cmdutil.Str(m, "status"),
					cmdutil.Str(m, "filename"),
					cmdutil.Str(m, "additions"),
					cmdutil.Str(m, "deletions"),
				})
			}
			ctx.Table([]string{"STATUS", "FILE", "+ADD", "-DEL"}, rows)
			ctx.Trailer(len(items), lr.Total, limit)
			return nil
		},
	}
	return cmd
}

func newPrCommentCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "comment <owner/repo> <number> --body=TEXT",
		Short: "Work with PR comments",
		Long: `PR comment commands.

With no subcommand, pr comment <owner/repo> <number> --body=TEXT creates a
general PR thread comment. The <number> is the PR number. Comment subcommands
that take <comment_id> use the numeric comment ID, which is globally unique
within a repo across issues and PRs. --body=- and --body-file=- read from stdin.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return prCommentCreate(ctx, cmd, args)
		},
	}
	cmdutil.AddBodyFlags(cmd)
	cmd.AddCommand(
		newPrCommentCreateCmd(ctx),
		newPrCommentListCmd(ctx),
		newPrCommentViewCmd(ctx),
		newPrCommentEditCmd(ctx),
		newPrCommentDeleteCmd(ctx),
	)
	return cmd
}

func newPrCommentCreateCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create <owner/repo> <number> --body=TEXT",
		Short: "Add a comment to a PR thread",
		Long: `Add a general comment to a PR thread.

The repo positional and PR number are required. The comment body is required
and may be supplied with --body, --body=-, --body-file=PATH, or --body-file=-.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return prCommentCreate(ctx, cmd, args)
		},
	}
	cmdutil.AddBodyFlags(cmd)
	return cmd
}

func newPrCommentListCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list <owner/repo> <number>",
		Short: "List PR comments",
		Long:  `List all general comments on a PR. Shows ID, author, date, and body preview.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			repo, number, err := prRepoNumber(ctx, args, "Usage: forgejo pr comment list <owner/repo> <number>")
			if err != nil {
				return err
			}
			raw, err := ctx.Client.Do("GET", "repos/"+repo+"/issues/"+number+"/comments", nil)
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
			if len(items) == 0 {
				fmt.Fprintln(ctx.Out, "No comments.")
				return nil
			}
			rows := make([][]string, 0, len(items))
			for _, m := range items {
				rows = append(rows, []string{
					cmdutil.Str(m, "id"),
					cmdutil.Str(m, "user.login"),
					prDate(cmdutil.Str(m, "created_at")),
					prDate(cmdutil.Str(m, "updated_at")),
					cmdutil.Trunc(prFirstLine(cmdutil.Str(m, "body")), 60),
				})
			}
			ctx.Table([]string{"ID", "AUTHOR", "CREATED", "UPDATED", "BODY"}, rows)
			return nil
		},
	}
	return cmd
}

func newPrCommentViewCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "view <owner/repo> <comment_id>",
		Short: "View a PR comment",
		Long:  `View one PR comment by numeric comment ID.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			repo, id, err := prRepoID(ctx, args, "comment id", "Usage: forgejo pr comment view <owner/repo> <comment_id>")
			if err != nil {
				return err
			}
			raw, err := ctx.Client.Do("GET", "repos/"+repo+"/issues/comments/"+id, nil)
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
			fmt.Fprintf(ctx.Out, "ID:       %s\n", cmdutil.Str(obj, "id"))
			fmt.Fprintf(ctx.Out, "Author:   %s\n", cmdutil.Str(obj, "user.login"))
			fmt.Fprintf(ctx.Out, "Created:  %s\n", cmdutil.Str(obj, "created_at"))
			fmt.Fprintf(ctx.Out, "Updated:  %s\n", cmdutil.Str(obj, "updated_at"))
			fmt.Fprintln(ctx.Out)
			fmt.Fprintln(ctx.Out, cmdutil.Str(obj, "body"))
			return nil
		},
	}
	return cmd
}

func newPrCommentEditCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "edit <owner/repo> <comment_id> --body=TEXT",
		Short: "Edit a PR comment",
		Long: `Edit one PR comment by numeric comment ID.

The replacement body is required and may be supplied with --body, --body=-,
--body-file=PATH, or --body-file=-.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			repo, id, err := prRepoID(ctx, args, "comment id", "Usage: forgejo pr comment edit <owner/repo> <comment_id> --body=X")
			if err != nil {
				return err
			}
			bodyText, hasBody, err := ctx.Body(cmd)
			if err != nil {
				return err
			}
			if !hasBody {
				return cmdutil.Usagef("Missing --body")
			}
			body, err := cmdutil.BuildBody(map[string]any{"body": bodyText})
			if err != nil {
				return err
			}
			raw, err := ctx.Client.Do("PATCH", "repos/"+repo+"/issues/comments/"+id, body)
			if err != nil {
				return err
			}
			if ctx.WantsJSON() {
				return ctx.EmitJSON(raw)
			}
			fmt.Fprintf(ctx.Out, "Updated comment #%s\n", id)
			return nil
		},
	}
	cmdutil.AddBodyFlags(cmd)
	return cmd
}

func newPrCommentDeleteCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete <owner/repo> <comment_id> [--yes]",
		Short: "Delete a PR comment",
		Long: `Delete one PR comment by numeric comment ID.

This is destructive. Pass --yes to skip the confirmation prompt; otherwise the
comment ID must be typed interactively.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			repo, id, err := prRepoID(ctx, args, "comment id", "Usage: forgejo pr comment delete <owner/repo> <comment_id>")
			if err != nil {
				return err
			}
			if err := ctx.ConfirmDelete(cmd, "comment", id); err != nil {
				return err
			}
			raw, err := ctx.Client.Do("DELETE", "repos/"+repo+"/issues/comments/"+id, nil)
			if err != nil {
				return err
			}
			if ctx.WantsJSON() {
				return ctx.EmitJSON(raw)
			}
			fmt.Fprintf(ctx.Out, "Deleted comment #%s\n", id)
			return nil
		},
	}
	cmdutil.AddYesFlag(cmd)
	return cmd
}

func newPrReviewCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "review <owner/repo> <number> --approve|--request-changes|--comment [--body=TEXT] [--comments=FILE|-]",
		Short: "Work with PR reviews",
		Long: `PR review commands.

With no subcommand, pr review <owner/repo> <number> submits a review. Exactly
one of --approve, --request-changes, or --comment is required. The flags are
mutually exclusive. --body is optional for approvals, required with
--request-changes, and required with --comment unless --comments is supplied.

--comments=FILE|- attaches inline line-level comments. The file or stdin must
be a non-empty JSON array of {"path":"f","line":N,"body":"text"} objects.
The line field is sent to Forgejo as new_position, matching the bash command.

The submit POST uses FORGEJO_REVIEW_TOKEN when configured; all surrounding GETs
use the primary token. If no review token is configured, submission silently
falls back to the primary token.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return prReviewCreate(ctx, cmd, args)
		},
	}
	addPrReviewCreateFlags(cmd)
	cmd.AddCommand(
		newPrReviewCreateCmd(ctx),
		newPrReviewListCmd(ctx),
		newPrReviewLookupCmd(ctx),
		newPrReviewDismissCmd(ctx),
	)
	return cmd
}

func newPrReviewCreateCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create <owner/repo> <number> --approve|--request-changes|--comment [--body=TEXT] [--comments=FILE|-]",
		Short: "Submit a PR review",
		Long: `Submit a PR review.

Exactly one of --approve, --request-changes, or --comment is required.
--body is optional for approvals, required with --request-changes, and required
with --comment unless --comments is supplied. --comments=FILE|- reads a
non-empty JSON array of inline comments and maps each line field to
new_position in the API payload. The submit POST uses FORGEJO_REVIEW_TOKEN when
configured and otherwise falls back to the primary token.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return prReviewCreate(ctx, cmd, args)
		},
	}
	addPrReviewCreateFlags(cmd)
	return cmd
}

func newPrReviewListCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list <owner/repo> <number>",
		Short: "List reviews on a PR",
		Long:  `List all reviews on a PR. Shows ID, user, state, and submitted date.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			repo, number, err := prRepoNumber(ctx, args, "Usage: forgejo pr review list <owner/repo> <number>")
			if err != nil {
				return err
			}
			raw, err := ctx.Client.Do("GET", "repos/"+repo+"/pulls/"+number+"/reviews", nil)
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
			if len(items) == 0 {
				fmt.Fprintln(ctx.Out, "No reviews.")
				return nil
			}
			rows := make([][]string, 0, len(items))
			for _, m := range items {
				rows = append(rows, []string{
					cmdutil.Str(m, "id"),
					cmdutil.Str(m, "user.login"),
					dash(cmdutil.Str(m, "state")),
					prDateOrDash(cmdutil.Str(m, "submitted_at")),
				})
			}
			ctx.Table([]string{"ID", "USER", "STATE", "SUBMITTED"}, rows)
			return nil
		},
	}
	return cmd
}

func newPrReviewLookupCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "lookup <owner/repo> <number>",
		Short: "Fetch unified review/comment JSON",
		Long: `Fetch all reviews, review comments, and PR comments merged into one JSON array.

Each item has a type field: "review", "review_comment", or "comment". Output
is always JSON; --json is implied. Review comment lists are fetched with paged
API calls through the same complete-conversation path as pr view.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			repo, number, err := prRepoNumber(ctx, args, "Usage: forgejo pr review lookup <owner/repo> <number>")
			if err != nil {
				return err
			}
			conv, err := prConversation(ctx, repo, number)
			if err != nil {
				return err
			}
			items := make([]map[string]any, 0)
			for _, r := range conv.reviews {
				review := prCloneMap(r)
				delete(review, "comments")
				review["type"] = "review"
				items = append(items, review)
			}
			for _, r := range conv.reviews {
				for _, c := range prReviewComments(r) {
					comment := prCloneMap(c)
					comment["type"] = "review_comment"
					items = append(items, comment)
				}
			}
			for _, c := range conv.issueComments {
				comment := prCloneMap(c)
				comment["type"] = "comment"
				items = append(items, comment)
			}
			raw, err := json.Marshal(items)
			if err != nil {
				return err
			}
			return ctx.EmitJSON(raw)
		},
	}
	return cmd
}

func newPrReviewDismissCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dismiss <owner/repo> <number> <review_id> --message=TEXT",
		Short: "Dismiss a review",
		Long:  `Dismiss a previous review by numeric review ID. --message is required and is sent as the dismissal message.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 3 {
				return cmdutil.Usagef("Usage: forgejo pr review dismiss <owner/repo> <number> <review_id> --message=X")
			}
			repo, err := ctx.RepoArg(args[0])
			if err != nil {
				return err
			}
			number, err := cmdutil.IDArg(args[1], "PR number")
			if err != nil {
				return err
			}
			rid, err := cmdutil.IDArg(args[2], "review id")
			if err != nil {
				return err
			}
			msg, _ := cmd.Flags().GetString("message")
			if msg == "" {
				return cmdutil.Usagef("Missing --message")
			}
			body, err := cmdutil.BuildBody(map[string]any{"message": msg})
			if err != nil {
				return err
			}
			raw, err := ctx.Client.Do("POST", "repos/"+repo+"/pulls/"+number+"/reviews/"+rid+"/dismissals", body)
			if err != nil {
				return err
			}
			if ctx.WantsJSON() {
				return ctx.EmitJSON(raw)
			}
			fmt.Fprintf(ctx.Out, "Dismissed review #%s on PR #%s\n", rid, number)
			return nil
		},
	}
	cmd.Flags().String("message", "", "dismissal message (required)")
	return cmd
}

func newPrRequestedReviewersCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "requested-reviewers <add|remove> <owner/repo> <number> --users=u1,u2",
		Short: "Manage requested PR reviewers",
		Long:  `Request or withdraw specific user review requests for a PR. --users is a comma-separated login list.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmdutil.Usagef("Usage: forgejo pr requested-reviewers <add|remove> <owner/repo> <number> --users=u1,u2")
		},
	}
	cmd.AddCommand(newPrRequestedReviewersAddCmd(ctx), newPrRequestedReviewersRemoveCmd(ctx))
	return cmd
}

func newPrRequestedReviewersAddCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add <owner/repo> <number> --users=u1,u2",
		Short: "Request PR reviewers",
		Long:  `Request specific users to review a PR. --users is a comma-separated login list.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return prRequestedReviewers(ctx, cmd, args, "add")
		},
	}
	cmd.Flags().String("users", "", "comma-separated user logins to request (required)")
	return cmd
}

func newPrRequestedReviewersRemoveCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "remove <owner/repo> <number> --users=u1,u2 [--yes]",
		Short: "Withdraw requested PR reviewers",
		Long: `Withdraw requested reviewers from a PR.

This is a destructive remove operation. Pass --yes to skip the confirmation
prompt; otherwise the comma-separated --users value must be typed interactively.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return prRequestedReviewers(ctx, cmd, args, "remove")
		},
	}
	cmd.Flags().String("users", "", "comma-separated user logins to withdraw (required)")
	cmdutil.AddYesFlag(cmd)
	return cmd
}

func addPrReviewCreateFlags(cmd *cobra.Command) {
	cmd.Flags().Bool("approve", false, "approve the PR")
	cmd.Flags().Bool("request-changes", false, "request changes on the PR; requires --body")
	cmd.Flags().Bool("comment", false, "leave a general review comment; requires --body unless --comments is supplied")
	cmd.Flags().String("comments", "", "inline review comments JSON file, or '-' to read JSON from stdin")
	cmdutil.AddBodyFlags(cmd)
}

func prRepoNumber(ctx *cmdutil.Ctx, args []string, usage string) (string, string, error) {
	if len(args) != 2 {
		return "", "", cmdutil.Usagef("%s", usage)
	}
	repo, err := ctx.RepoArg(args[0])
	if err != nil {
		return "", "", err
	}
	number, err := cmdutil.IDArg(args[1], "PR number")
	if err != nil {
		return "", "", err
	}
	return repo, number, nil
}

func prRepoID(ctx *cmdutil.Ctx, args []string, what, usage string) (string, string, error) {
	if len(args) != 2 {
		return "", "", cmdutil.Usagef("%s", usage)
	}
	repo, err := ctx.RepoArg(args[0])
	if err != nil {
		return "", "", err
	}
	id, err := cmdutil.IDArg(args[1], what)
	if err != nil {
		return "", "", err
	}
	return repo, id, nil
}

func prPatchState(ctx *cmdutil.Ctx, repo, number, state string) ([]byte, error) {
	body, err := cmdutil.BuildBody(map[string]any{"state": state})
	if err != nil {
		return nil, err
	}
	return ctx.Client.Do("PATCH", "repos/"+repo+"/pulls/"+number, body)
}

func prRawPull(ctx *cmdutil.Ctx, args []string, ext, usage string) error {
	repo, number, err := prRepoNumber(ctx, args, usage)
	if err != nil {
		return err
	}
	raw, err := ctx.Client.DoRaw("GET", "repos/"+repo+"/pulls/"+number+"."+ext, "*/*")
	if err != nil {
		return err
	}
	_, err = ctx.Out.Write(raw)
	return err
}

type prConversationData struct {
	pull          map[string]any
	reviews       []map[string]any
	issueComments []map[string]any
}

func prConversation(ctx *cmdutil.Ctx, repo, number string) (*prConversationData, error) {
	pullRaw, err := ctx.Client.Do("GET", "repos/"+repo+"/pulls/"+number, nil)
	if err != nil {
		return nil, err
	}
	pull, err := cmdutil.ParseObject(pullRaw)
	if err != nil {
		return nil, err
	}
	reviewsRaw, err := ctx.Client.DoPaged("repos/" + repo + "/pulls/" + number + "/reviews")
	if err != nil {
		return nil, err
	}
	reviews, err := cmdutil.ParseArray(reviewsRaw)
	if err != nil {
		return nil, err
	}
	for i := range reviews {
		rid := cmdutil.Str(reviews[i], "id")
		if rid == "" {
			reviews[i]["comments"] = []map[string]any{}
			continue
		}
		rcRaw, err := ctx.Client.DoPaged("repos/" + repo + "/pulls/" + number + "/reviews/" + cmdutil.PathEscape(rid) + "/comments")
		if err != nil {
			return nil, err
		}
		rc, err := cmdutil.ParseArray(rcRaw)
		if err != nil {
			return nil, err
		}
		reviews[i]["comments"] = rc
	}
	commentsRaw, err := ctx.Client.DoPaged("repos/" + repo + "/issues/" + number + "/comments")
	if err != nil {
		return nil, err
	}
	issueComments, err := cmdutil.ParseArray(commentsRaw)
	if err != nil {
		return nil, err
	}
	pull["reviews"] = reviews
	pull["issue_comments"] = issueComments
	return &prConversationData{pull: pull, reviews: reviews, issueComments: issueComments}, nil
}

func prPrintConversation(ctx *cmdutil.Ctx, conv *prConversationData) {
	pull := conv.pull
	fmt.Fprintf(ctx.Out, "#%s: %s\n", cmdutil.Str(pull, "number"), cmdutil.Str(pull, "title"))
	fmt.Fprintf(ctx.Out, "State:      %s\n", cmdutil.Str(pull, "state"))
	fmt.Fprintf(ctx.Out, "Author:     %s\n", cmdutil.Str(pull, "user.login"))
	fmt.Fprintf(ctx.Out, "Base:       %s ← Head: %s\n", cmdutil.Str(pull, "base.ref"), cmdutil.Str(pull, "head.ref"))
	fmt.Fprintf(ctx.Out, "Mergeable:  %s\n", dash(cmdutil.Str(pull, "mergeable")))
	fmt.Fprintf(ctx.Out, "Created:    %s\n", prDate(cmdutil.Str(pull, "created_at")))
	fmt.Fprintf(ctx.Out, "Updated:    %s\n", prDate(cmdutil.Str(pull, "updated_at")))
	fmt.Fprintf(ctx.Out, "Labels:     %s\n", prLabels(pull))
	fmt.Fprintln(ctx.Out)
	body := cmdutil.Str(pull, "body")
	if body == "" {
		body = "(no body)"
	}
	fmt.Fprintln(ctx.Out, body)

	fmt.Fprintln(ctx.Out)
	fmt.Fprintln(ctx.Out, "--- Reviews ---")
	if len(conv.reviews) == 0 {
		fmt.Fprintln(ctx.Out, "No reviews.")
	} else {
		for _, r := range conv.reviews {
			fmt.Fprintf(ctx.Out, "  %s: %s (%s)\n",
				cmdutil.Str(r, "user.login"), dash(cmdutil.Str(r, "state")), prDateOrDash(cmdutil.Str(r, "submitted_at")))
			for _, c := range prReviewComments(r) {
				pos := cmdutil.Str(c, "position")
				if pos == "" {
					pos = cmdutil.Str(c, "original_position")
				}
				if pos == "" {
					pos = "?"
				}
				fmt.Fprintf(ctx.Out, "      %s:%s — %s\n", cmdutil.Str(c, "path"), pos, cmdutil.Str(c, "body"))
			}
		}
	}

	fmt.Fprintln(ctx.Out)
	fmt.Fprintln(ctx.Out, "--- Comments ---")
	if len(conv.issueComments) == 0 {
		fmt.Fprintln(ctx.Out, "No comments.")
		return
	}
	for _, c := range conv.issueComments {
		fmt.Fprintf(ctx.Out, "  %s (%s): %s\n", cmdutil.Str(c, "user.login"), prDate(cmdutil.Str(c, "created_at")), cmdutil.Str(c, "body"))
	}
}

func prCommentCreate(ctx *cmdutil.Ctx, cmd *cobra.Command, args []string) error {
	repo, number, err := prRepoNumber(ctx, args, "Usage: forgejo pr comment <owner/repo> <number> --body=X")
	if err != nil {
		return err
	}
	bodyText, hasBody, err := ctx.Body(cmd)
	if err != nil {
		return err
	}
	if !hasBody {
		return cmdutil.Usagef("Missing --body")
	}
	body, err := cmdutil.BuildBody(map[string]any{"body": bodyText})
	if err != nil {
		return err
	}
	raw, err := ctx.Client.Do("POST", "repos/"+repo+"/issues/"+number+"/comments", body)
	if err != nil {
		return err
	}
	if ctx.WantsJSON() {
		return ctx.EmitJSON(raw)
	}
	fmt.Fprintf(ctx.Out, "Comment added to PR #%s\n", number)
	return nil
}

func prReviewCreate(ctx *cmdutil.Ctx, cmd *cobra.Command, args []string) error {
	repo, number, err := prRepoNumber(ctx, args, "Usage: forgejo pr review <owner/repo> <number> --approve|--request-changes|--comment [--body=X]")
	if err != nil {
		return err
	}
	approve, _ := cmd.Flags().GetBool("approve")
	requestChanges, _ := cmd.Flags().GetBool("request-changes")
	comment, _ := cmd.Flags().GetBool("comment")
	count := 0
	event := ""
	if approve {
		event = "APPROVED"
		count++
	}
	if requestChanges {
		event = "REQUEST_CHANGES"
		count++
	}
	if comment {
		event = "COMMENT"
		count++
	}
	if count == 0 {
		return cmdutil.Usagef("Specify exactly one of --approve|--request-changes|--comment")
	}
	if count > 1 {
		return cmdutil.Usagef("--approve, --request-changes, --comment are mutually exclusive")
	}
	bodyText, _, err := ctx.Body(cmd)
	if err != nil {
		return err
	}
	inlineComments, hasInline, err := prInlineComments(ctx, cmd)
	if err != nil {
		return err
	}
	if event == "REQUEST_CHANGES" && bodyText == "" {
		return cmdutil.Usagef("--body required with --request-changes")
	}
	if event == "COMMENT" && bodyText == "" && !hasInline {
		return cmdutil.Usagef("--body required with --comment (or supply --comments=FILE)")
	}
	fields := map[string]any{"event": event}
	if bodyText != "" {
		fields["body"] = bodyText
	}
	if hasInline {
		fields["comments"] = inlineComments
	}
	payload, err := cmdutil.BuildBody(fields)
	if err != nil {
		return err
	}

	submit := ctx.Client
	usingReview := false
	if rc, err := ctx.Client.WithReviewToken(); err == nil {
		submit = rc
		usingReview = true
	}
	endpoint := "repos/" + repo + "/pulls/" + number + "/reviews"
	var raw []byte
	if usingReview {
		status, out, err := submit.DoStatus("POST", endpoint, payload)
		if err != nil {
			return err
		}
		if status < 200 || status >= 300 {
			return prReviewSubmitError(status, out, repo)
		}
		raw = out
	} else {
		raw, err = submit.Do("POST", endpoint, payload)
		if err != nil {
			return err
		}
	}
	if ctx.WantsJSON() {
		return ctx.EmitJSON(raw)
	}
	obj, err := cmdutil.ParseObject(raw)
	if err != nil {
		return err
	}
	fmt.Fprintf(ctx.Out, "Submitted review #%s on PR #%s (state: %s)\n",
		cmdutil.Str(obj, "id"), number, cmdutil.Str(obj, "state"))
	return nil
}

func prInlineComments(ctx *cmdutil.Ctx, cmd *cobra.Command) ([]map[string]any, bool, error) {
	src, _ := cmd.Flags().GetString("comments")
	if src == "" {
		return nil, false, nil
	}
	var data []byte
	var err error
	if src == "-" {
		data, err = io.ReadAll(ctx.In)
	} else {
		data, err = os.ReadFile(src)
		if errors.Is(err, os.ErrNotExist) {
			return nil, false, cmdutil.Usagef("--comments: file not found: %s", src)
		}
	}
	if err != nil {
		return nil, false, err
	}
	dec := json.NewDecoder(strings.NewReader(string(data)))
	dec.UseNumber()
	var input []map[string]any
	if err := dec.Decode(&input); err != nil {
		return nil, false, prInvalidComments()
	}
	if len(input) == 0 {
		return nil, false, prInvalidComments()
	}
	out := make([]map[string]any, 0, len(input))
	for _, c := range input {
		path, ok := c["path"].(string)
		if !ok || path == "" {
			return nil, false, prInvalidComments()
		}
		line, ok := c["line"].(json.Number)
		if !ok {
			return nil, false, prInvalidComments()
		}
		body, ok := c["body"].(string)
		if !ok || body == "" {
			return nil, false, prInvalidComments()
		}
		out = append(out, map[string]any{
			"path":         path,
			"new_position": line,
			"body":         body,
		})
	}
	return out, true, nil
}

func prInvalidComments() error {
	return cmdutil.Usagef("--comments: invalid JSON. Expected a non-empty array of {\"path\":\"file\",\"line\":N,\"body\":\"text\"} objects.")
}

func prReviewSubmitError(status int, raw []byte, repo string) error {
	hint := ""
	if status == 401 || status == 403 || status == 404 {
		hint = "this review was submitted as the FORGEJO_REVIEW_TOKEN identity, not FORGEJO_TOKEN. " +
			fmt.Sprintf("A %d here usually means that account cannot access %s (Forgejo returns 404 for repos a user can't see), or its token is missing scopes. Verify the review account is a collaborator/org member on the repo and that its token has read:user + write:repository scopes.", status, repo)
	}
	return &api.Error{Status: status, Message: prErrorMessage(raw), Hint: hint}
}

func prErrorMessage(raw []byte) string {
	msg := strings.TrimSpace(string(raw))
	var parsed struct {
		Message string `json:"message"`
		Err     string `json:"error"`
	}
	if err := json.Unmarshal(raw, &parsed); err == nil {
		if parsed.Message != "" {
			msg = parsed.Message
		} else if parsed.Err != "" {
			msg = parsed.Err
		}
	}
	if msg == "" {
		msg = "empty response"
	}
	return msg
}

type prChecksResult struct {
	sha      string
	statuses []map[string]any
	runs     []map[string]any
}

func prChecks(ctx *cmdutil.Ctx, repo, number string) (*prChecksResult, error) {
	prRaw, err := ctx.Client.Do("GET", "repos/"+repo+"/pulls/"+number, nil)
	if err != nil {
		return nil, err
	}
	prObj, err := cmdutil.ParseObject(prRaw)
	if err != nil {
		return nil, err
	}
	sha := cmdutil.Str(prObj, "head.sha")
	if sha == "" || sha == "null" {
		return nil, fmt.Errorf("Could not resolve head SHA for PR #%s", number)
	}
	statusesRaw, err := ctx.Client.Do("GET", "repos/"+repo+"/commits/"+cmdutil.PathEscape(sha)+"/statuses", nil)
	if err != nil {
		return nil, err
	}
	statuses, err := cmdutil.ParseArray(statusesRaw)
	if err != nil {
		return nil, err
	}

	runs, err := prOptionalObjectList(ctx, "repos/"+repo+"/actions/runs?head_sha="+cmdutil.QueryEscape(sha), "workflow_runs")
	if err != nil {
		return nil, err
	}
	if len(runs) > 0 {
		tasks, err := prOptionalObjectList(ctx, "repos/"+repo+"/actions/tasks?limit=200", "workflow_runs")
		if err != nil {
			return nil, err
		}
		prJoinRunTasks(runs, tasks)
	}
	return &prChecksResult{sha: sha, statuses: statuses, runs: runs}, nil
}

func prOptionalObjectList(ctx *cmdutil.Ctx, endpoint, key string) ([]map[string]any, error) {
	status, raw, err := ctx.Client.DoStatus("GET", endpoint, nil)
	if err != nil {
		return nil, err
	}
	if status < 200 || status >= 300 {
		return []map[string]any{}, nil
	}
	obj, err := cmdutil.ParseObject(raw)
	if err != nil {
		items, aerr := cmdutil.ParseArray(raw)
		if aerr == nil {
			return items, nil
		}
		return nil, err
	}
	items := prMapsFromAny(obj[key])
	if items == nil {
		items = []map[string]any{}
	}
	return items, nil
}

func prJoinRunTasks(runs, tasks []map[string]any) {
	byRun := map[string][]map[string]any{}
	for _, task := range tasks {
		runNumber := cmdutil.Str(task, "run_number")
		if runNumber == "" {
			continue
		}
		byRun[runNumber] = append(byRun[runNumber], task)
	}
	for _, run := range runs {
		idx := cmdutil.Str(run, "index_in_repo")
		if idx == "" {
			idx = cmdutil.Str(run, "run_number")
		}
		if joined := byRun[idx]; joined != nil {
			run["tasks"] = joined
		} else {
			run["tasks"] = []map[string]any{}
		}
	}
}

func prPrintChecks(ctx *cmdutil.Ctx, number string, result *prChecksResult) {
	rows := make([][]string, 0)
	for _, s := range result.statuses {
		rows = append(rows, []string{
			"status",
			dash(cmdutil.Str(s, "context")),
			dash(cmdutil.Str(s, "status")),
			dash(cmdutil.Str(s, "target_url")),
		})
	}
	for _, run := range result.runs {
		runURL := dash(cmdutil.Str(run, "html_url"))
		rows = append(rows, []string{
			"action",
			prRunName(run),
			prRunStatus(run),
			runURL,
		})
		for _, task := range prRunTasks(run) {
			rows = append(rows, []string{
				"task",
				prRunName(task),
				prRunStatus(task),
				runURL,
			})
		}
	}
	if len(rows) == 0 {
		fmt.Fprintf(ctx.Out, "No checks found for PR #%s (head %s).\n", number, result.sha)
		return
	}
	ctx.Table([]string{"SOURCE", "NAME", "STATUS", "URL"}, rows)
}

func prRequestedReviewers(ctx *cmdutil.Ctx, cmd *cobra.Command, args []string, op string) error {
	repo, number, err := prRepoNumber(ctx, args, fmt.Sprintf("Usage: forgejo pr requested-reviewers %s <owner/repo> <number> --users=u1,u2", op))
	if err != nil {
		return err
	}
	users, _ := cmd.Flags().GetString("users")
	if users == "" {
		return cmdutil.Usagef("Missing --users")
	}
	reviewers := splitComma(users)
	body, err := cmdutil.BuildBody(map[string]any{"reviewers": reviewers})
	if err != nil {
		return err
	}
	method := "POST"
	success := "Requested reviewers added to PR #%s: %s\n"
	if op == "remove" {
		if err := ctx.ConfirmDelete(cmd, "requested reviewers", users); err != nil {
			return err
		}
		method = "DELETE"
		success = "Requested reviewers removed from PR #%s: %s\n"
	}
	raw, err := ctx.Client.Do(method, "repos/"+repo+"/pulls/"+number+"/requested_reviewers", body)
	if err != nil {
		return err
	}
	if ctx.WantsJSON() {
		return ctx.EmitJSON(raw)
	}
	fmt.Fprintf(ctx.Out, success, number, users)
	return nil
}

func prDate(s string) string {
	if len(s) > 10 {
		return s[:10]
	}
	return s
}

func prDateOrDash(s string) string {
	if s == "" {
		return "-"
	}
	return prDate(s)
}

func prFirstLine(s string) string {
	if before, _, ok := strings.Cut(s, "\n"); ok {
		return before
	}
	return s
}

func prLabels(m map[string]any) string {
	labels := cmdutil.Str(m, "labels")
	if strings.TrimSpace(labels) == "" {
		return "-"
	}
	return labels
}

func prCloneMap(m map[string]any) map[string]any {
	out := make(map[string]any, len(m)+1)
	for k, v := range m {
		out[k] = v
	}
	return out
}

func prReviewComments(review map[string]any) []map[string]any {
	return prMapsFromAny(review["comments"])
}

func prRunTasks(run map[string]any) []map[string]any {
	return prMapsFromAny(run["tasks"])
}

func prMapsFromAny(v any) []map[string]any {
	switch t := v.(type) {
	case []map[string]any:
		return t
	case []any:
		out := make([]map[string]any, 0, len(t))
		for _, it := range t {
			if m, ok := it.(map[string]any); ok {
				out = append(out, m)
			}
		}
		return out
	default:
		return nil
	}
}

func prRunName(m map[string]any) string {
	if name := cmdutil.Str(m, "name"); name != "" {
		return name
	}
	if title := cmdutil.Str(m, "title"); title != "" {
		return title
	}
	return "-"
}

func prRunStatus(m map[string]any) string {
	if conclusion := cmdutil.Str(m, "conclusion"); conclusion != "" {
		return conclusion
	}
	if status := cmdutil.Str(m, "status"); status != "" {
		return status
	}
	return "-"
}
