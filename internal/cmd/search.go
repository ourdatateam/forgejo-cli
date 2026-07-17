package cmd

// Ported from the bash cmd_search family (forgejo:6085-6270). Query
// parameters are always URL-encoded; --limit passes through to the server
// exactly as given (these verbs had no default limit in bash).

import (
	"fmt"
	"strings"

	"github.com/ourdatateam/forgejo-cli/internal/cmdutil"
	"github.com/spf13/cobra"
)

func init() { Register(newSearchCmd) }

func newSearchCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "search <repos|users|issues>",
		Short: "Search repos, users, and issues instance-wide",
	}
	cmd.AddCommand(newSearchReposCmd(ctx), newSearchUsersCmd(ctx), newSearchIssuesCmd(ctx))
	return cmd
}

func newSearchReposCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "repos --query=X",
		Short: "Search repositories",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			query, _ := cmd.Flags().GetString("query")
			owner, _ := cmd.Flags().GetString("owner")
			limit, _ := cmd.Flags().GetInt("limit")
			if query == "" {
				return cmdutil.Usagef("search repos requires --query")
			}
			qs := "q=" + cmdutil.QueryEscape(query)
			if owner != "" {
				// bash resolves --owner to a uid via users/{owner}.
				raw, err := ctx.Client.Do("GET", "users/"+cmdutil.PathEscape(owner), nil)
				if err != nil {
					return cmdutil.Usagef("unknown owner %q", owner)
				}
				obj, err := cmdutil.ParseObject(raw)
				if err != nil {
					return err
				}
				qs += "&uid=" + cmdutil.Str(obj, "id")
			}
			if limit >= 0 {
				qs += fmt.Sprintf("&limit=%d", limit)
			}
			raw, err := ctx.Client.Do("GET", "repos/search?"+qs, nil)
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
			items, _ := obj["data"].([]any)
			if len(items) == 0 {
				fmt.Fprintln(ctx.Out, "No repos found.")
				return nil
			}
			rows := make([][]string, 0, len(items))
			for _, it := range items {
				m, _ := it.(map[string]any)
				rows = append(rows, []string{
					cmdutil.Str(m, "full_name"),
					yesNo(cmdutil.Str(m, "private")),
					yesNo(cmdutil.Str(m, "archived")),
					cmdutil.Str(m, "stars_count"),
					cmdutil.Trunc(cmdutil.Str(m, "description"), 50),
				})
			}
			ctx.Table([]string{"FULL_NAME", "PRIVATE", "ARCHIVED", "STARS", "DESCRIPTION"}, rows)
			return nil
		},
	}
	cmd.Flags().String("query", "", "search query (required)")
	cmd.Flags().String("owner", "", "restrict to a user/org (resolved to uid)")
	cmd.Flags().Int("limit", -1, "server-side result limit")
	return cmd
}

func newSearchUsersCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "users --query=X",
		Short: "Search users",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			query, _ := cmd.Flags().GetString("query")
			limit, _ := cmd.Flags().GetInt("limit")
			if query == "" {
				return cmdutil.Usagef("search users requires --query")
			}
			qs := "q=" + cmdutil.QueryEscape(query)
			if limit >= 0 {
				qs += fmt.Sprintf("&limit=%d", limit)
			}
			raw, err := ctx.Client.Do("GET", "users/search?"+qs, nil)
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
			items, _ := obj["data"].([]any)
			if len(items) == 0 {
				fmt.Fprintln(ctx.Out, "No users found.")
				return nil
			}
			rows := make([][]string, 0, len(items))
			for _, it := range items {
				m, _ := it.(map[string]any)
				rows = append(rows, []string{
					cmdutil.Str(m, "login"),
					dash(cmdutil.Str(m, "full_name")),
					dash(cmdutil.Str(m, "email")),
					yesNo(cmdutil.Str(m, "active")),
				})
			}
			ctx.Table([]string{"LOGIN", "FULL_NAME", "EMAIL", "ACTIVE"}, rows)
			return nil
		},
	}
	cmd.Flags().String("query", "", "search query (required)")
	cmd.Flags().Int("limit", -1, "server-side result limit")
	return cmd
}

func newSearchIssuesCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "issues --query=X",
		Short: "Search issues and pull requests instance-wide",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			query, _ := cmd.Flags().GetString("query")
			typ, _ := cmd.Flags().GetString("type")
			state, _ := cmd.Flags().GetString("state")
			owner, _ := cmd.Flags().GetString("owner")
			limit, _ := cmd.Flags().GetInt("limit")
			if query == "" {
				return cmdutil.Usagef("search issues requires --query")
			}
			qs := "q=" + cmdutil.QueryEscape(query)
			switch typ {
			case "":
			case "issue":
				qs += "&type=issues"
			case "pr":
				qs += "&type=pulls"
			default:
				return cmdutil.Usagef("--type must be 'issue' or 'pr' (got: %s)", typ)
			}
			if state != "" {
				qs += "&state=" + cmdutil.QueryEscape(state)
			}
			if owner != "" {
				qs += "&owner=" + cmdutil.QueryEscape(owner)
			}
			if limit >= 0 {
				qs += fmt.Sprintf("&limit=%d", limit)
			}
			raw, err := ctx.Client.Do("GET", "repos/issues/search?"+qs, nil)
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
				fmt.Fprintln(ctx.Out, "No issues found.")
				return nil
			}
			rows := make([][]string, 0, len(items))
			for _, m := range items {
				kind := "issue"
				if m["pull_request"] != nil {
					kind = "pr"
				}
				rows = append(rows, []string{
					dash(cmdutil.Str(m, "repository.full_name")),
					cmdutil.Str(m, "number"),
					kind,
					cmdutil.Str(m, "state"),
					cmdutil.Trunc(cmdutil.Str(m, "title"), 50),
				})
			}
			ctx.Table([]string{"REPO", "#", "TYPE", "STATE", "TITLE"}, rows)
			return nil
		},
	}
	cmd.Flags().String("query", "", "search query (required)")
	cmd.Flags().String("type", "", "issue|pr")
	cmd.Flags().String("state", "", "open|closed|all")
	cmd.Flags().String("owner", "", "restrict to an owner")
	cmd.Flags().Int("limit", -1, "server-side result limit")
	return cmd
}

func yesNo(v string) string {
	if v == "true" {
		return "yes"
	}
	return "no"
}

func dash(v string) string {
	if strings.TrimSpace(v) == "" {
		return "-"
	}
	return v
}
