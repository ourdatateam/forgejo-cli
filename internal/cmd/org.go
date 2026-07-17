package cmd

// Ported from the bash cmd_org family (forgejo:2914-3311). Team commands
// keep the bash name-to-id resolver because the discovered behavior depends
// on exact .name matching from orgs/{org}/teams/search.

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/ourdatateam/forgejo-cli/internal/cmdutil"
	"github.com/spf13/cobra"
)

func init() { Register(newOrgCmd) }

func newOrgCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "org <list|create|view|edit|delete|member|team>",
		Short: "Manage organizations, members, and teams",
		Long: "Manage organizations through the same endpoints as the bash CLI.\n\n" +
			"Organization members are conferred by team membership in Forgejo; org member add is therefore an explicit usage error. Team commands accept a numeric team id or resolve an exact team name within the organization before calling teams/{id}.",
	}
	cmd.AddCommand(
		newOrgListCmd(ctx),
		newOrgCreateCmd(ctx),
		newOrgViewCmd(ctx),
		newOrgEditCmd(ctx),
		newOrgDeleteCmd(ctx),
		newOrgMemberCmd(ctx),
		newOrgTeamCmd(ctx),
	)
	return cmd
}

func newOrgListCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List organizations",
		Long:  "List organizations through admin/orgs. The bash command fetched admin/orgs?limit=50; the global --limit flag overrides that page size, and --limit=0 fetches all pages.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			n := ctx.ListLimit(50)
			lr, err := ctx.Client.DoList("admin/orgs", n)
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
			for _, m := range items {
				rows = append(rows, []string{
					cmdutil.Str(m, "username"),
					cmdutil.Trunc(strDefault(m, "description", "-"), 40),
					cmdutil.Str(m, "visibility"),
				})
			}
			ctx.Table([]string{"NAME", "DESCRIPTION", "VISIBILITY"}, rows)
			ctx.Trailer(len(items), lr.Total, n)
			return nil
		},
	}
	return cmd
}

func newOrgCreateCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create <name> [--desc=X] [--visibility=public|private]",
		Short: "Create an organization",
		Long:  "Create an organization through orgs. The name positional is required. --desc defaults to an empty string and --visibility defaults to public, matching the bash request body.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			desc, _ := cmd.Flags().GetString("desc")
			visibility, _ := cmd.Flags().GetString("visibility")
			body, err := json.Marshal(map[string]any{
				"username":    name,
				"description": desc,
				"visibility":  visibility,
			})
			if err != nil {
				return err
			}
			raw, err := ctx.Client.Do("POST", "orgs", body)
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
			fmt.Fprintf(ctx.Out, "Created org: %s\n", cmdutil.Str(obj, "username"))
			return nil
		},
	}
	cmd.Flags().String("desc", "", "organization description")
	cmd.Flags().String("visibility", "public", "organization visibility to send (public or private)")
	return cmd
}

func newOrgViewCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "view <name>",
		Short: "View an organization",
		Long:  "View an organization through orgs/{name}. Text mode also fetches orgs/{name}/members?limit=50 and prints the member section; JSON mode emits only the raw organization response, matching bash.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			orgPath := cmdutil.NameSeg(name)
			raw, err := ctx.Client.Do("GET", "orgs/"+orgPath, nil)
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
			fmt.Fprintf(ctx.Out, "Name:        %s\n", cmdutil.Str(obj, "username"))
			fmt.Fprintf(ctx.Out, "Description: %s\n", strDefault(obj, "description", "-"))
			fmt.Fprintf(ctx.Out, "Visibility:  %s\n", cmdutil.Str(obj, "visibility"))
			fmt.Fprintf(ctx.Out, "Website:     %s\n", strDefault(obj, "website", "-"))
			fmt.Fprintln(ctx.Out)
			fmt.Fprintln(ctx.Out, "--- Members ---")

			membersRaw, err := ctx.Client.Do("GET", "orgs/"+orgPath+"/members?limit=50", nil)
			if err != nil {
				return err
			}
			members, err := cmdutil.ParseArray(membersRaw)
			if err != nil {
				return err
			}
			if len(members) == 0 {
				fmt.Fprintln(ctx.Out, "No members.")
				return nil
			}
			for _, m := range members {
				fmt.Fprintf(ctx.Out, "  %s\n", cmdutil.Str(m, "login"))
			}
			return nil
		},
	}
	return cmd
}

func newOrgEditCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "edit <name> [--desc=X] [--visibility=public|private]",
		Short: "Edit an organization",
		Long:  "Edit an organization through orgs/{name}. --desc and --visibility are optional; when neither is supplied the bash command still sends an empty JSON object.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			bodyMap := map[string]any{}
			if cmd.Flags().Changed("desc") {
				desc, _ := cmd.Flags().GetString("desc")
				bodyMap["description"] = desc
			}
			if cmd.Flags().Changed("visibility") {
				visibility, _ := cmd.Flags().GetString("visibility")
				bodyMap["visibility"] = visibility
			}
			body, err := json.Marshal(bodyMap)
			if err != nil {
				return err
			}
			raw, err := ctx.Client.Do("PATCH", "orgs/"+cmdutil.NameSeg(name), body)
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
			fmt.Fprintf(ctx.Out, "Updated org: %s\n", cmdutil.Str(obj, "username"))
			return nil
		},
	}
	cmd.Flags().String("desc", "", "set the organization description")
	cmd.Flags().String("visibility", "", "set organization visibility (public or private)")
	return cmd
}

func newOrgDeleteCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete <name> [--yes]",
		Short: "Delete an organization",
		Long:  "Delete an organization through orgs/{name}. This destructive command requires --yes or an interactive typed-name confirmation.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			org := args[0]
			if err := ctx.ConfirmDelete(cmd, "org", org); err != nil {
				return err
			}
			if _, err := ctx.Client.Do("DELETE", "orgs/"+cmdutil.NameSeg(org), nil); err != nil {
				return err
			}
			fmt.Fprintf(ctx.Out, "Deleted org: %s\n", org)
			return nil
		},
	}
	cmdutil.AddYesFlag(cmd)
	return cmd
}

func newOrgMemberCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "member <list|add|remove>",
		Short: "Manage organization members",
		Long:  "List or remove organization members. Forgejo grants organization membership through teams, so member add is intentionally rejected and points to org team member add.",
	}
	cmd.AddCommand(newOrgMemberListCmd(ctx), newOrgMemberAddCmd(ctx), newOrgMemberRemoveCmd(ctx))
	return cmd
}

func newOrgMemberListCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list <org>",
		Short: "List organization members",
		Long:  "List organization members through orgs/{org}/members. The bash command fetched ?limit=50; the global --limit flag overrides that page size, and --limit=0 fetches all pages.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			org := args[0]
			n := ctx.ListLimit(50)
			lr, err := ctx.Client.DoList("orgs/"+cmdutil.NameSeg(org)+"/members", n)
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
			for _, m := range items {
				rows = append(rows, []string{
					cmdutil.Str(m, "login"),
					strDefault(m, "full_name", "-"),
					strDefault(m, "email", "-"),
				})
			}
			ctx.Table([]string{"LOGIN", "NAME", "EMAIL"}, rows)
			ctx.Trailer(len(items), lr.Total, n)
			return nil
		},
	}
	return cmd
}

func newOrgMemberAddCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add [<org>] [--user=<user>]",
		Short: "Explain how to add organization members",
		Long:  "Organization members are conferred by team membership in Forgejo. This command always returns a usage error; use forgejo org team member add <org> <team> --user=<user> instead.",
		Args:  cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmdutil.Usagef("org members are conferred by team membership in Forgejo.\n        Use: forgejo org team member add <org> <team> --user=<user>")
		},
	}
	cmd.Flags().String("user", "", "username attempted for direct org membership; use org team member add instead")
	return cmd
}

func newOrgMemberRemoveCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "remove <org> --user=<user> [--yes]",
		Short: "Remove a user from an organization",
		Long:  "Remove a user from an organization through orgs/{org}/members/{user}. This destructive remove command requires --yes or an interactive typed-name confirmation.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			org := args[0]
			user, _ := cmd.Flags().GetString("user")
			if user == "" {
				return cmdutil.Usagef("Missing --user")
			}
			if err := ctx.ConfirmDelete(cmd, "org member", user); err != nil {
				return err
			}
			if _, err := ctx.Client.Do("DELETE", "orgs/"+cmdutil.NameSeg(org)+"/members/"+cmdutil.NameSeg(user), nil); err != nil {
				return err
			}
			fmt.Fprintf(ctx.Out, "Removed %s from org %s\n", user, org)
			return nil
		},
	}
	cmd.Flags().String("user", "", "username to remove from the organization (required)")
	cmdutil.AddYesFlag(cmd)
	return cmd
}

func newOrgTeamCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "team <list|view|create|edit|delete|member|repo>",
		Short: "Manage organization teams",
		Long:  "Manage teams. Team arguments can be numeric ids or exact team names; names are resolved by searching orgs/{org}/teams/search?q=<name> and matching .name case-sensitively.",
	}
	cmd.AddCommand(
		newOrgTeamListCmd(ctx),
		newOrgTeamViewCmd(ctx),
		newOrgTeamCreateCmd(ctx),
		newOrgTeamEditCmd(ctx),
		newOrgTeamDeleteCmd(ctx),
		newOrgTeamMemberCmd(ctx),
		newOrgTeamRepoCmd(ctx),
	)
	return cmd
}

func newOrgTeamListCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list <org>",
		Short: "List teams",
		Long:  "List teams in an organization through orgs/{org}/teams. The bash command fetched ?limit=50; the global --limit flag overrides that page size, and --limit=0 fetches all pages.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			org := args[0]
			n := ctx.ListLimit(50)
			lr, err := ctx.Client.DoList("orgs/"+cmdutil.NameSeg(org)+"/teams", n)
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
			for _, m := range items {
				rows = append(rows, []string{
					cmdutil.Str(m, "id"),
					cmdutil.Str(m, "name"),
					cmdutil.Str(m, "permission"),
					cmdutil.Str(m, "includes_all_repositories"),
					cmdutil.Trunc(strDefault(m, "description", "-"), 40),
				})
			}
			ctx.Table([]string{"ID", "NAME", "PERMISSION", "ALL_REPOS", "DESCRIPTION"}, rows)
			ctx.Trailer(len(items), lr.Total, n)
			return nil
		},
	}
	return cmd
}

func newOrgTeamViewCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "view <org> <team>",
		Short: "View a team",
		Long:  "View a team. <team> may be a numeric id or an exact team name resolved within <org> before GET teams/{id}.",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			org, teamArg := args[0], args[1]
			teamID, err := resolveTeamID(ctx, org, teamArg)
			if err != nil {
				return err
			}
			raw, err := ctx.Client.Do("GET", "teams/"+teamID, nil)
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
			fmt.Fprintf(ctx.Out, "ID:           %s\n", cmdutil.Str(obj, "id"))
			fmt.Fprintf(ctx.Out, "Name:         %s\n", cmdutil.Str(obj, "name"))
			fmt.Fprintf(ctx.Out, "Description:  %s\n", strDefault(obj, "description", "-"))
			fmt.Fprintf(ctx.Out, "Permission:   %s\n", cmdutil.Str(obj, "permission"))
			fmt.Fprintf(ctx.Out, "Includes all: %s\n", cmdutil.Str(obj, "includes_all_repositories"))
			fmt.Fprintf(ctx.Out, "Can create:   %s\n", cmdutil.Str(obj, "can_create_org_repo"))
			return nil
		},
	}
	return cmd
}

func newOrgTeamCreateCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create <org> --name=X [--description=X] [--permission=read|write|admin]",
		Short: "Create a team",
		Long:  "Create a team in an organization. --name is required. --description defaults to an empty string, --permission defaults to read, includes_all_repositories and can_create_org_repo are sent false, and the bash unit list is sent unchanged.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			org := args[0]
			name, _ := cmd.Flags().GetString("name")
			desc, _ := cmd.Flags().GetString("description")
			permission, _ := cmd.Flags().GetString("permission")
			if name == "" {
				return cmdutil.Usagef("Missing --name")
			}
			body, err := json.Marshal(map[string]any{
				"name":                      name,
				"description":               desc,
				"permission":                permission,
				"includes_all_repositories": false,
				"can_create_org_repo":       false,
				"units": []string{
					"repo.code",
					"repo.issues",
					"repo.pulls",
					"repo.releases",
					"repo.wiki",
					"repo.ext_wiki",
					"repo.ext_issues",
					"repo.projects",
				},
			})
			if err != nil {
				return err
			}
			raw, err := ctx.Client.Do("POST", "orgs/"+cmdutil.NameSeg(org)+"/teams", body)
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
			fmt.Fprintf(ctx.Out, "Created team: %s (id=%s)\n", cmdutil.Str(obj, "name"), cmdutil.Str(obj, "id"))
			return nil
		},
	}
	cmd.Flags().String("name", "", "team name to create (required)")
	cmd.Flags().String("description", "", "team description")
	cmd.Flags().String("permission", "read", "team permission to send (read, write, or admin)")
	return cmd
}

func newOrgTeamEditCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "edit <org> <team> [--name=X] [--description=X] [--permission=X]",
		Short: "Edit a team",
		Long:  "Edit a team. <team> may be a numeric id or exact team name. The command first fetches teams/{id} because Forgejo PATCH requires name and permission; unchanged fields are copied into the request body, matching bash.",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			org, teamArg := args[0], args[1]
			teamID, err := resolveTeamID(ctx, org, teamArg)
			if err != nil {
				return err
			}
			currentRaw, err := ctx.Client.Do("GET", "teams/"+teamID, nil)
			if err != nil {
				return err
			}
			current, err := cmdutil.ParseObject(currentRaw)
			if err != nil {
				return err
			}
			bodyMap := map[string]any{
				"name":                      rawField(current, "name"),
				"description":               rawField(current, "description"),
				"permission":                rawField(current, "permission"),
				"includes_all_repositories": rawField(current, "includes_all_repositories"),
				"can_create_org_repo":       rawField(current, "can_create_org_repo"),
				"units":                     rawFieldDefault(current, "units", []string{"repo.code"}),
			}
			if cmd.Flags().Changed("name") {
				name, _ := cmd.Flags().GetString("name")
				bodyMap["name"] = name
			}
			if cmd.Flags().Changed("description") {
				desc, _ := cmd.Flags().GetString("description")
				bodyMap["description"] = desc
			}
			if cmd.Flags().Changed("permission") {
				permission, _ := cmd.Flags().GetString("permission")
				bodyMap["permission"] = permission
			}
			body, err := json.Marshal(bodyMap)
			if err != nil {
				return err
			}
			raw, err := ctx.Client.Do("PATCH", "teams/"+teamID, body)
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
			fmt.Fprintf(ctx.Out, "Updated team: %s (id=%s)\n", cmdutil.Str(obj, "name"), cmdutil.Str(obj, "id"))
			return nil
		},
	}
	cmd.Flags().String("name", "", "set the team name")
	cmd.Flags().String("description", "", "set the team description")
	cmd.Flags().String("permission", "", "set team permission")
	return cmd
}

func newOrgTeamDeleteCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete <org> <team> [--yes]",
		Short: "Delete a team",
		Long:  "Delete a team. <team> may be a numeric id or exact team name resolved within <org>. This destructive command requires --yes or an interactive typed-name confirmation.",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			org, teamArg := args[0], args[1]
			teamID, err := resolveTeamID(ctx, org, teamArg)
			if err != nil {
				return err
			}
			if err := ctx.ConfirmDelete(cmd, "team", teamArg); err != nil {
				return err
			}
			if _, err := ctx.Client.Do("DELETE", "teams/"+teamID, nil); err != nil {
				return err
			}
			fmt.Fprintf(ctx.Out, "Deleted team: %s (id=%s)\n", teamArg, teamID)
			return nil
		},
	}
	cmdutil.AddYesFlag(cmd)
	return cmd
}

func newOrgTeamMemberCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "member <list|add|remove>",
		Short: "Manage team members",
		Long:  "Manage team members. <team> may be a numeric id or exact team name resolved within <org> before teams/{id}/members calls.",
	}
	cmd.AddCommand(newOrgTeamMemberListCmd(ctx), newOrgTeamMemberAddCmd(ctx), newOrgTeamMemberRemoveCmd(ctx))
	return cmd
}

func newOrgTeamMemberListCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list <org> <team>",
		Short: "List team members",
		Long:  "List team members. <team> may be a numeric id or exact team name. The bash command fetched ?limit=50; the global --limit flag overrides that page size, and --limit=0 fetches all pages.",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			teamID, err := resolveTeamID(ctx, args[0], args[1])
			if err != nil {
				return err
			}
			n := ctx.ListLimit(50)
			lr, err := ctx.Client.DoList("teams/"+teamID+"/members", n)
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
			for _, m := range items {
				rows = append(rows, []string{
					cmdutil.Str(m, "login"),
					strDefault(m, "full_name", "-"),
					strDefault(m, "email", "-"),
				})
			}
			ctx.Table([]string{"LOGIN", "NAME", "EMAIL"}, rows)
			ctx.Trailer(len(items), lr.Total, n)
			return nil
		},
	}
	return cmd
}

func newOrgTeamMemberAddCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add <org> <team> --user=<user>",
		Short: "Add a user to a team",
		Long:  "Add a user to a team through teams/{id}/members/{user}. <team> may be a numeric id or exact team name resolved within <org>. --user is required.",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			org, teamArg := args[0], args[1]
			user, _ := cmd.Flags().GetString("user")
			if user == "" {
				return cmdutil.Usagef("Missing --user")
			}
			teamID, err := resolveTeamID(ctx, org, teamArg)
			if err != nil {
				return err
			}
			if _, err := ctx.Client.Do("PUT", "teams/"+teamID+"/members/"+cmdutil.NameSeg(user), nil); err != nil {
				return err
			}
			fmt.Fprintf(ctx.Out, "Added %s to team %s (id=%s)\n", user, teamArg, teamID)
			return nil
		},
	}
	cmd.Flags().String("user", "", "username to add to the team (required)")
	return cmd
}

func newOrgTeamMemberRemoveCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "remove <org> <team> --user=<user> [--yes]",
		Short: "Remove a user from a team",
		Long:  "Remove a user from a team through teams/{id}/members/{user}. <team> may be a numeric id or exact team name resolved within <org>. This destructive remove command requires --yes or an interactive typed-name confirmation.",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			org, teamArg := args[0], args[1]
			user, _ := cmd.Flags().GetString("user")
			if user == "" {
				return cmdutil.Usagef("Missing --user")
			}
			teamID, err := resolveTeamID(ctx, org, teamArg)
			if err != nil {
				return err
			}
			if err := ctx.ConfirmDelete(cmd, "team member", user); err != nil {
				return err
			}
			if _, err := ctx.Client.Do("DELETE", "teams/"+teamID+"/members/"+cmdutil.NameSeg(user), nil); err != nil {
				return err
			}
			fmt.Fprintf(ctx.Out, "Removed %s from team %s (id=%s)\n", user, teamArg, teamID)
			return nil
		},
	}
	cmd.Flags().String("user", "", "username to remove from the team (required)")
	cmdutil.AddYesFlag(cmd)
	return cmd
}

func newOrgTeamRepoCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "repo <list|add|remove>",
		Short: "Manage team repositories",
		Long:  "Manage repositories assigned to a team. <team> may be a numeric id or exact team name resolved within <org> before teams/{id}/repos calls.",
	}
	cmd.AddCommand(newOrgTeamRepoListCmd(ctx), newOrgTeamRepoAddCmd(ctx), newOrgTeamRepoRemoveCmd(ctx))
	return cmd
}

func newOrgTeamRepoListCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list <org> <team>",
		Short: "List team repositories",
		Long:  "List repositories assigned to a team. <team> may be a numeric id or exact team name. The bash command fetched ?limit=50; the global --limit flag overrides that page size, and --limit=0 fetches all pages.",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			teamID, err := resolveTeamID(ctx, args[0], args[1])
			if err != nil {
				return err
			}
			n := ctx.ListLimit(50)
			lr, err := ctx.Client.DoList("teams/"+teamID+"/repos", n)
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
			for _, m := range items {
				rows = append(rows, []string{
					cmdutil.Str(m, "full_name"),
					cmdutil.Str(m, "private"),
					cmdutil.Str(m, "archived"),
				})
			}
			ctx.Table([]string{"FULL_NAME", "PRIVATE", "ARCHIVED"}, rows)
			ctx.Trailer(len(items), lr.Total, n)
			return nil
		},
	}
	return cmd
}

func newOrgTeamRepoAddCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add <org> <team> --repo=<owner/repo>",
		Short: "Add a repository to a team",
		Long:  "Add a repository to a team through teams/{id}/repos/{owner}/{repo}. <team> may be a numeric id or exact team name resolved within <org>. --repo is required and must be owner/repo.",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			org, teamArg := args[0], args[1]
			fullname, _ := cmd.Flags().GetString("repo")
			if fullname == "" {
				return cmdutil.Usagef("Missing --repo (expected owner/repo)")
			}
			repoPath, err := repoFlagPath(fullname)
			if err != nil {
				return err
			}
			teamID, err := resolveTeamID(ctx, org, teamArg)
			if err != nil {
				return err
			}
			if _, err := ctx.Client.Do("PUT", "teams/"+teamID+"/repos/"+repoPath, nil); err != nil {
				return err
			}
			fmt.Fprintf(ctx.Out, "Added %s to team %s (id=%s)\n", fullname, teamArg, teamID)
			return nil
		},
	}
	cmd.Flags().String("repo", "", "repository full name to add, in owner/repo form (required)")
	return cmd
}

func newOrgTeamRepoRemoveCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "remove <org> <team> --repo=<owner/repo> [--yes]",
		Short: "Remove a repository from a team",
		Long:  "Remove a repository from a team through teams/{id}/repos/{owner}/{repo}. <team> may be a numeric id or exact team name resolved within <org>. --repo is required and must be owner/repo. This destructive remove command requires --yes or an interactive typed-name confirmation.",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			org, teamArg := args[0], args[1]
			fullname, _ := cmd.Flags().GetString("repo")
			if fullname == "" {
				return cmdutil.Usagef("Missing --repo")
			}
			repoPath, err := repoFlagPath(fullname)
			if err != nil {
				return err
			}
			teamID, err := resolveTeamID(ctx, org, teamArg)
			if err != nil {
				return err
			}
			if err := ctx.ConfirmDelete(cmd, "team repo", fullname); err != nil {
				return err
			}
			if _, err := ctx.Client.Do("DELETE", "teams/"+teamID+"/repos/"+repoPath, nil); err != nil {
				return err
			}
			fmt.Fprintf(ctx.Out, "Removed %s from team %s (id=%s)\n", fullname, teamArg, teamID)
			return nil
		},
	}
	cmd.Flags().String("repo", "", "repository full name to remove, in owner/repo form (required)")
	cmdutil.AddYesFlag(cmd)
	return cmd
}

func resolveTeamID(ctx *cmdutil.Ctx, org, needle string) (string, error) {
	if isDecimalID(needle) {
		return cmdutil.IDArg(needle, "team id")
	}
	raw, err := ctx.Client.Do("GET", "orgs/"+cmdutil.NameSeg(org)+"/teams/search?q="+cmdutil.QueryEscape(needle), nil)
	if err != nil {
		return "", err
	}
	obj, err := cmdutil.ParseObject(raw)
	if err != nil {
		return "", err
	}
	data, _ := obj["data"].([]any)
	var hits []map[string]any
	var available []string
	for _, item := range data {
		m, _ := item.(map[string]any)
		name := cmdutil.Str(m, "name")
		if name != "" {
			available = append(available, "  "+name)
		}
		if name == needle {
			hits = append(hits, m)
		}
	}
	switch len(hits) {
	case 0:
		msg := fmt.Sprintf("No team named '%s' in org '%s'\nAvailable:", needle, org)
		if len(available) > 0 {
			msg += "\n" + strings.Join(available, "\n")
		}
		return "", fmt.Errorf("%s", msg)
	case 1:
		id := cmdutil.Str(hits[0], "id")
		if id == "" {
			return "", fmt.Errorf("team '%s' in org '%s' has no id", needle, org)
		}
		return cmdutil.IDArg(id, "team id")
	default:
		return "", fmt.Errorf("Ambiguous team name '%s' in org '%s' (matched %d)", needle, org, len(hits))
	}
}

func repoFlagPath(fullname string) (string, error) {
	if !cmdutil.ValidRepo(fullname) {
		return "", cmdutil.Usagef("invalid --repo %q (expected owner/repo)", fullname)
	}
	owner, repo, _ := strings.Cut(fullname, "/")
	return cmdutil.NameSeg(owner) + "/" + cmdutil.NameSeg(repo), nil
}

func rawField(m map[string]any, path string) any {
	v, ok := lookupPath(m, path)
	if !ok {
		return nil
	}
	return v
}

func rawFieldDefault(m map[string]any, path string, def any) any {
	v, ok := lookupPath(m, path)
	if !ok || v == nil {
		return def
	}
	return v
}

func isDecimalID(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
