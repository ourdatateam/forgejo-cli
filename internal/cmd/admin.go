package cmd

// Ported from the bash cmd_admin family (forgejo:5673-5822).

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/ourdatateam/forgejo-cli/internal/cmdutil"
	"github.com/spf13/cobra"
)

func init() { Register(newAdminCmd) }

func newAdminCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "admin <cron|config|queue|stats|notice>",
		Short: "Admin operations",
		Long: strings.TrimSpace(`Admin commands for Forgejo instance operations.

The Forgejo API exposes cron task listing/runs and selected server settings.
Queue, stats, and notices are not exposed by the Forgejo API on the probed
server versions; those commands exit 2 with a web UI pointer.`),
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmdutil.Usagef("Usage: forgejo admin <cron|config|queue|stats|notice> [args]")
		},
	}
	cmd.AddCommand(
		newAdminStatsCmd(ctx),
		newAdminQueueCmd(ctx),
		newAdminNoticeCmd(ctx),
		newAdminCronCmd(ctx),
		newAdminConfigCmd(ctx),
	)
	return cmd
}

func newAdminStatsCmd(ctx *cmdutil.Ctx) *cobra.Command {
	return &cobra.Command{
		Use:   "stats",
		Short: "Show admin stats (not exposed by API)",
		Long: strings.TrimSpace(`Admin stats are not exposed by the Forgejo API.

This command preserves the bash behavior: it exits with code 2 and points to
the admin web UI instead of attempting an API call.`),
		Args: cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return adminUnsupported(ctx, "stats", "/-/admin")
		},
	}
}

func newAdminQueueCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "queue <list|view|pause|resume>",
		Short: "Admin queue operations (not exposed by API)",
		Long: strings.TrimSpace(`Admin queue operations are not exposed by the Forgejo API.

The list, view, pause, and resume subcommands preserve the bash behavior: each
exits with code 2 and points to the queue page in the admin web UI.`),
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmdutil.Usagef("Usage: forgejo admin queue <list|view|pause|resume> [args]")
		},
	}
	for _, sub := range []string{"list", "view", "pause", "resume"} {
		name := sub
		cmd.AddCommand(&cobra.Command{
			Use:   name,
			Short: "Not exposed by the Forgejo API",
			Long: strings.TrimSpace(`This admin queue operation is not exposed by the
Forgejo API and exits with code 2. Use the admin web UI queue page instead.`),
			Args: cobra.ArbitraryArgs,
			RunE: func(cmd *cobra.Command, args []string) error {
				return adminUnsupported(ctx, "queue "+name, "/-/admin/monitor/queue")
			},
		})
	}
	return cmd
}

func newAdminNoticeCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "notice <list|delete>",
		Short: "Admin notices (not exposed by API)",
		Long: strings.TrimSpace(`Admin notices are not exposed by the Forgejo API.

The list and delete subcommands preserve the bash behavior: each exits with
code 2 and points to the notices page in the admin web UI.`),
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmdutil.Usagef("Usage: forgejo admin notice <list|delete> [args]")
		},
	}
	list := &cobra.Command{
		Use:   "list",
		Short: "List admin notices (not exposed by API)",
		Long: strings.TrimSpace(`Admin notice listing is not exposed by the Forgejo API.

This command exits with code 2 and points to the notices page in the admin web
UI.`),
		Args: cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return adminUnsupported(ctx, "notice list", "/-/admin/notices")
		},
	}
	del := &cobra.Command{
		Use:   "delete [id] [--yes]",
		Short: "Delete an admin notice (not exposed by API)",
		Long: strings.TrimSpace(`Admin notice deletion is not exposed by the Forgejo API.

The command accepts --yes for consistency with destructive verbs, but no API
call is attempted; it exits with code 2 and points to the notices page in the
admin web UI.`),
		Args: cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return adminUnsupported(ctx, "notice delete", "/-/admin/notices")
		},
	}
	cmdutil.AddYesFlag(del)
	cmd.AddCommand(list, del)
	return cmd
}

func newAdminCronCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cron <list|run>",
		Short: "List and run admin cron tasks",
		Long: strings.TrimSpace(`List Forgejo admin cron tasks or trigger one task by name.

The list command preserves the bash default limit=50; --limit overrides that
default and --limit=0 fetches all pages. The run command posts to the cron task
name as a single escaped path segment.`),
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmdutil.Usagef("Usage: forgejo admin cron <list|run> [args]")
		},
	}
	cmd.AddCommand(newAdminCronListCmd(ctx), newAdminCronRunCmd(ctx))
	return cmd
}

func newAdminCronListCmd(ctx *cmdutil.Ctx) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List cron tasks",
		Long: strings.TrimSpace(`List Forgejo admin cron tasks.

The bash command used limit=50; --limit overrides that default and --limit=0
fetches all pages. Text output includes name, schedule, next run, previous run,
and execution count.`),
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			n := ctx.ListLimit(50)
			lr, err := ctx.Client.DoList("admin/cron", n)
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
			rows := make([][]string, 0, len(items))
			for _, item := range items {
				next := dash(cmdutil.Str(item, "next"))
				if next != "-" {
					next = cmdutil.Trunc(next, 19) + "Z"
				}
				prev := dash(cmdutil.Str(item, "prev"))
				if prev != "-" {
					prev = cmdutil.Trunc(prev, 19) + "Z"
				}
				execTimes := cmdutil.Str(item, "exec_times")
				if execTimes == "" {
					execTimes = "0"
				}
				rows = append(rows, []string{
					dash(cmdutil.Str(item, "name")),
					dash(cmdutil.Str(item, "schedule")),
					next,
					prev,
					execTimes,
				})
			}
			ctx.Table([]string{"NAME", "SCHEDULE", "NEXT", "PREV", "EXEC_TIMES"}, rows)
			ctx.Trailer(len(rows), lr.Total, n)
			return nil
		},
	}
}

func newAdminCronRunCmd(ctx *cmdutil.Ctx) *cobra.Command {
	return &cobra.Command{
		Use:   "run <name>",
		Short: "Trigger a cron task",
		Long: strings.TrimSpace(`Trigger one Forgejo admin cron task by name.

The task name is required and is escaped as a single path segment before the
POST request.`),
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			if _, err := ctx.Client.Do("POST", "admin/cron/"+cmdutil.NameSeg(name), nil); err != nil {
				return err
			}
			fmt.Fprintf(ctx.Out, "Triggered cron task: %s\n", name)
			return nil
		},
	}
}

func newAdminConfigCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config <view>",
		Short: "View server settings",
		Long: strings.TrimSpace(`View selected Forgejo server settings.

The view command aggregates the settings/api, settings/attachment,
settings/repository, and settings/ui endpoints into api, attachment,
repository, and ui sections.`),
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmdutil.Usagef("Usage: forgejo admin config <view> [args]")
		},
	}
	cmd.AddCommand(newAdminConfigViewCmd(ctx))
	return cmd
}

func newAdminConfigViewCmd(ctx *cmdutil.Ctx) *cobra.Command {
	return &cobra.Command{
		Use:   "view",
		Short: "Show server settings",
		Long: strings.TrimSpace(`Show selected Forgejo server settings.

This command GETs settings/api, settings/attachment, settings/repository, and
settings/ui, then merges them under api, attachment, repository, and ui. With
--json, the merged JSON object is printed. Text mode prints each section with
keys sorted alphabetically.`),
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			merged, sections, err := adminConfig(ctx)
			if err != nil {
				return err
			}
			if ctx.WantsJSON() {
				return ctx.EmitJSON(merged)
			}
			for _, name := range []string{"api", "attachment", "repository", "ui"} {
				fmt.Fprintf(ctx.Out, "[%s]\n", name)
				printSettingsSection(ctx, sections[name])
				fmt.Fprintln(ctx.Out)
			}
			return nil
		},
	}
}

func adminUnsupported(ctx *cmdutil.Ctx, verb, webPath string) error {
	return &cmdutil.ExitError{
		Code: 2,
		Err:  errors.New("forgejo: admin " + verb + " is not exposed by the Forgejo API.\nUse the web UI: " + forgejoBaseURL(ctx) + webPath),
	}
}

type adminConfigJSON struct {
	API        json.RawMessage `json:"api"`
	Attachment json.RawMessage `json:"attachment"`
	Repository json.RawMessage `json:"repository"`
	UI         json.RawMessage `json:"ui"`
}

func adminConfig(ctx *cmdutil.Ctx) ([]byte, map[string]map[string]any, error) {
	endpoints := map[string]string{
		"api":        "settings/api",
		"attachment": "settings/attachment",
		"repository": "settings/repository",
		"ui":         "settings/ui",
	}
	raw := map[string][]byte{}
	sections := map[string]map[string]any{}
	for _, name := range []string{"api", "attachment", "repository", "ui"} {
		body, err := ctx.Client.Do("GET", endpoints[name], nil)
		if err != nil {
			return nil, nil, err
		}
		raw[name] = body
		obj, err := cmdutil.ParseObject(body)
		if err != nil {
			return nil, nil, err
		}
		sections[name] = obj
	}
	merged, err := json.Marshal(adminConfigJSON{
		API:        json.RawMessage(raw["api"]),
		Attachment: json.RawMessage(raw["attachment"]),
		Repository: json.RawMessage(raw["repository"]),
		UI:         json.RawMessage(raw["ui"]),
	})
	if err != nil {
		return nil, nil, err
	}
	return merged, sections, nil
}

func printSettingsSection(ctx *cmdutil.Ctx, section map[string]any) {
	keys := make([]string, 0, len(section))
	for k := range section {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Fprintf(ctx.Out, "%s: %s\n", k, settingValue(section[k]))
	}
}

func settingValue(v any) string {
	switch t := v.(type) {
	case nil:
		return "null"
	case string:
		return t
	case json.Number:
		return t.String()
	case bool:
		if t {
			return "true"
		}
		return "false"
	case []any, map[string]any:
		raw, err := json.Marshal(t)
		if err != nil {
			return fmt.Sprintf("%v", t)
		}
		return string(raw)
	default:
		return fmt.Sprintf("%v", t)
	}
}
