package cmd

// Ported from the bash cmd_notification family (forgejo:5981-6082).
// `notification read --all` marks the whole account's unread queue read —
// it is deliberately excluded from the live acceptance suite.

import (
	"fmt"
	"strings"

	"github.com/ourdatateam/forgejo-cli/internal/cmdutil"
	"github.com/spf13/cobra"
)

func init() { Register(newNotificationCmd) }

func newNotificationCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "notification <list|read>",
		Short: "List and mark notifications",
	}
	cmd.AddCommand(newNotificationListCmd(ctx), newNotificationReadCmd(ctx))
	return cmd
}

func newNotificationListCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list [--all] [--status=unread,read,pinned]",
		Short: "List notifications (unread by default)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			status, _ := cmd.Flags().GetString("status")
			all, _ := cmd.Flags().GetBool("all")
			limit, _ := cmd.Flags().GetInt("limit")

			qs := ""
			appendQS := func(kv string) {
				if qs != "" {
					qs += "&"
				}
				qs += kv
			}
			if all {
				appendQS("all=true")
			}
			if status != "" {
				for _, s := range splitComma(status) {
					appendQS("status-types=" + cmdutil.QueryEscape(s))
				}
			}
			if limit >= 0 {
				appendQS(fmt.Sprintf("limit=%d", limit))
			}
			endpoint := "notifications"
			if qs != "" {
				endpoint += "?" + qs
			}
			raw, err := ctx.Client.Do("GET", endpoint, nil)
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
				fmt.Fprintln(ctx.Out, "No notifications.")
				return nil
			}
			rows := make([][]string, 0, len(items))
			for _, m := range items {
				updated := cmdutil.Str(m, "updated_at")
				if updated == "" {
					updated = "-"
				} else {
					updated = cmdutil.Trunc(updated, 19) + "Z"
				}
				rows = append(rows, []string{
					cmdutil.Str(m, "id"),
					dash(cmdutil.Str(m, "repository.full_name")),
					dash(cmdutil.Str(m, "subject.type")),
					cmdutil.Trunc(dash(cmdutil.Str(m, "subject.title")), 60),
					dash(cmdutil.Str(m, "subject.state")),
					updated,
				})
			}
			ctx.Table([]string{"ID", "REPO", "TYPE", "TITLE", "STATE", "UPDATED"}, rows)
			return nil
		},
	}
	cmd.Flags().Bool("all", false, "include read notifications")
	cmd.Flags().String("status", "", "comma-separated status filter (unread,read,pinned)")
	cmd.Flags().Int("limit", -1, "server-side result limit")
	return cmd
}

func newNotificationReadCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "read <id> | --all",
		Short: "Mark a notification thread (or everything) as read",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			all, _ := cmd.Flags().GetBool("all")
			if len(args) == 1 && all {
				return cmdutil.Usagef("notification read: pass either <id> or --all, not both")
			}
			if len(args) == 0 && !all {
				return cmdutil.Usagef("Usage: forgejo notification read <id> | --all")
			}
			if all {
				if _, err := ctx.Client.Do("PUT", "notifications?status-types=unread", nil); err != nil {
					return err
				}
				fmt.Fprintln(ctx.Out, "Marked all unread notifications as read")
				return nil
			}
			id, err := cmdutil.IDArg(args[0], "notification id")
			if err != nil {
				return err
			}
			if _, err := ctx.Client.Do("PATCH", "notifications/threads/"+id+"?to-status=read", nil); err != nil {
				return err
			}
			fmt.Fprintf(ctx.Out, "Marked thread %s as read\n", id)
			return nil
		},
	}
	cmd.Flags().Bool("all", false, "mark every unread notification as read")
	return cmd
}

func splitComma(s string) []string {
	var out []string
	for _, part := range strings.Split(s, ",") {
		if part = strings.TrimSpace(part); part != "" {
			out = append(out, part)
		}
	}
	return out
}
