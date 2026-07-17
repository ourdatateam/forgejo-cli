package cmd

// Ported from the bash cmd_repo family (forgejo:805-1579). The repo slot is
// always a required positional where bash had one; "." still means infer from
// the current git remote through cmdutil.RepoArg.

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/ourdatateam/forgejo-cli/internal/api"
	"github.com/ourdatateam/forgejo-cli/internal/cmdutil"
	"github.com/spf13/cobra"
)

func init() { Register(newRepoCmd) }

func newRepoCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "repo <list|create|view|edit|delete|fork|transfer|archive|unarchive|tags|collaborator|webhook|key|topic|mirror>",
		Short: "Manage repositories",
	}
	cmd.AddCommand(
		newRepoListCmd(ctx),
		newRepoCreateCmd(ctx),
		newRepoViewCmd(ctx),
		newRepoEditCmd(ctx),
		newRepoDeleteCmd(ctx),
		newRepoForkCmd(ctx),
		newRepoTransferCmd(ctx),
		newRepoArchiveCmd(ctx),
		newRepoUnarchiveCmd(ctx),
		newRepoTagsCmd(ctx),
		newRepoCollaboratorCmd(ctx),
		newRepoWebhookCmd(ctx),
		newRepoKeyCmd(ctx),
		newRepoTopicCmd(ctx),
		newRepoMirrorCmd(ctx),
	)
	return cmd
}

func newRepoListCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list [org]",
		Short: "List repositories",
		Long: "List repositories visible to the authenticated user.\n\n" +
			"With no org, this lists user repositories. Pass an org either as the optional positional or with --org; if both are supplied, --org wins like the bash CLI. The bash endpoint used limit=50, so the global --limit overrides that default and --limit=0 fetches all pages.",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			org, _ := cmd.Flags().GetString("org")
			if org == "" && len(args) > 0 {
				org = args[0]
			}

			endpoint := "user/repos"
			if org != "" {
				endpoint = "orgs/" + cmdutil.PathEscape(org) + "/repos"
			}
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
			rows := make([][]string, 0, len(items))
			for _, m := range items {
				rows = append(rows, []string{
					cmdutil.Str(m, "full_name"),
					yesNo(cmdutil.Str(m, "private")),
					cmdutil.Str(m, "stars_count"),
					prefixRunes(cmdutil.Str(m, "updated_at"), 10),
				})
			}
			ctx.Table([]string{"NAME", "PRIVATE", "STARS", "UPDATED"}, rows)
			ctx.Trailer(len(items), lr.Total, limit)
			return nil
		},
	}
	cmd.Flags().String("org", "", "list repositories in this organization instead of user repositories")
	return cmd
}

func newRepoCreateCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create <name> [--org=X] [--private] [--desc=X]",
		Short: "Create a repository",
		Long: "Create a repository named <name>.\n\n" +
			"By default the repository is created for the authenticated user. Use --org to create it under an organization. --private makes the repository private; --desc sets the description, including an empty description when passed as --desc=.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			org, _ := cmd.Flags().GetString("org")
			desc, _ := cmd.Flags().GetString("desc")
			private, _ := cmd.Flags().GetBool("private")

			body, err := cmdutil.BuildBody(map[string]any{
				"name":        args[0],
				"description": desc,
				"private":     private,
			})
			if err != nil {
				return err
			}
			endpoint := "user/repos"
			if org != "" {
				endpoint = "orgs/" + cmdutil.PathEscape(org) + "/repos"
			}
			raw, err := ctx.Client.Do("POST", endpoint, body)
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
			fmt.Fprintf(ctx.Out, "Created: %s\nURL: %s\nSSH: %s\n",
				cmdutil.Str(obj, "full_name"),
				cmdutil.Str(obj, "html_url"),
				cmdutil.Str(obj, "ssh_url"),
			)
			return nil
		},
	}
	cmd.Flags().String("org", "", "organization owner for the new repository")
	cmd.Flags().Bool("private", false, "create a private repository")
	cmd.Flags().String("desc", "", "repository description")
	return cmd
}

func newRepoViewCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "view <owner/repo>",
		Short: "View repository details",
		Long:  "View details for a repository. The repo positional is required; pass '.' to infer owner/repo from the current git remote.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			repo, err := ctx.RepoArg(args[0])
			if err != nil {
				return err
			}
			raw, err := ctx.Client.Do("GET", "repos/"+repoAPIPath(repo), nil)
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
			fmt.Fprintf(ctx.Out, "Name:        %s\n", cmdutil.Str(obj, "full_name"))
			fmt.Fprintf(ctx.Out, "Description: %s\n", dash(cmdutil.Str(obj, "description")))
			fmt.Fprintf(ctx.Out, "Private:     %s\n", yesNo(cmdutil.Str(obj, "private")))
			fmt.Fprintf(ctx.Out, "Stars:       %s\n", cmdutil.Str(obj, "stars_count"))
			fmt.Fprintf(ctx.Out, "Forks:       %s\n", cmdutil.Str(obj, "forks_count"))
			fmt.Fprintf(ctx.Out, "HTTP URL:    %s\n", cmdutil.Str(obj, "clone_url"))
			fmt.Fprintf(ctx.Out, "SSH URL:     %s\n", cmdutil.Str(obj, "ssh_url"))
			fmt.Fprintf(ctx.Out, "Created:     %s\n", prefixRunes(cmdutil.Str(obj, "created_at"), 10))
			fmt.Fprintf(ctx.Out, "Updated:     %s\n", prefixRunes(cmdutil.Str(obj, "updated_at"), 10))
			return nil
		},
	}
	return cmd
}

func newRepoEditCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "edit <owner/repo> [--name=X] [--desc=X] [--private|--public]",
		Short: "Edit or rename a repository",
		Long: "Edit repository metadata.\n\n" +
			"The repo positional is required; pass '.' to infer owner/repo from the current git remote. Provide at least one of --name, --desc, --private, or --public. --private and --public are mutually exclusive.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			repo, err := ctx.RepoArg(args[0])
			if err != nil {
				return err
			}
			private, _ := cmd.Flags().GetBool("private")
			public, _ := cmd.Flags().GetBool("public")
			if private && public {
				return cmdutil.Usagef("--private and --public are mutually exclusive")
			}

			fields := map[string]any{}
			if cmd.Flags().Changed("name") {
				name, _ := cmd.Flags().GetString("name")
				fields["name"] = name
			}
			if cmd.Flags().Changed("desc") {
				desc, _ := cmd.Flags().GetString("desc")
				fields["description"] = desc
			}
			if private {
				fields["private"] = true
			}
			if public {
				fields["private"] = false
			}
			if len(fields) == 0 {
				return cmdutil.Usagef("No fields to update. Provide --name, --desc, --private, or --public.")
			}
			body, err := cmdutil.BuildBody(fields)
			if err != nil {
				return err
			}
			raw, err := ctx.Client.Do("PATCH", "repos/"+repoAPIPath(repo), body)
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
			fmt.Fprintf(ctx.Out, "Updated: %s\n", cmdutil.Str(obj, "full_name"))
			return nil
		},
	}
	cmd.Flags().String("name", "", "new repository name")
	cmd.Flags().String("desc", "", "new repository description")
	cmd.Flags().Bool("private", false, "make the repository private")
	cmd.Flags().Bool("public", false, "make the repository public")
	return cmd
}

func newRepoDeleteCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete <owner/repo>",
		Short: "Delete a repository",
		Long:  "Delete a repository. The repo positional is required; pass '.' to infer owner/repo from the current git remote. This destructive command requires --yes or an interactive typed-name confirmation.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			repo, err := ctx.RepoArg(args[0])
			if err != nil {
				return err
			}
			if err := ctx.ConfirmDelete(cmd, "repo", repo); err != nil {
				return err
			}
			if _, err := ctx.Client.Do("DELETE", "repos/"+repoAPIPath(repo), nil); err != nil {
				return err
			}
			fmt.Fprintf(ctx.Out, "Deleted: %s\n", repo)
			return nil
		},
	}
	cmdutil.AddYesFlag(cmd)
	return cmd
}

func newRepoForkCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "fork <owner/repo> [--org=X]",
		Short: "Fork a repository",
		Long: "Fork a repository.\n\n" +
			"The repo positional is required; pass '.' to infer owner/repo from the current git remote. Use --org to fork into an organization. A server 409 means the fork already exists; the command treats that as success and fetches the existing fork.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			repo, err := ctx.RepoArg(args[0])
			if err != nil {
				return err
			}
			org, _ := cmd.Flags().GetString("org")

			fields := map[string]any{}
			if org != "" {
				fields["organization"] = org
			}
			body, err := cmdutil.BuildBody(fields)
			if err != nil {
				return err
			}
			status, raw, err := ctx.Client.DoStatus("POST", "repos/"+repoAPIPath(repo)+"/forks", body)
			if err != nil {
				return err
			}
			if status >= 200 && status < 300 {
				if ctx.WantsJSON() {
					return ctx.EmitJSON(raw)
				}
				obj, err := cmdutil.ParseObject(raw)
				if err != nil {
					return err
				}
				fmt.Fprintf(ctx.Out, "Forked: %s\nURL: %s\n", cmdutil.Str(obj, "full_name"), cmdutil.Str(obj, "html_url"))
				return nil
			}
			if status != 409 {
				return apiStatusError(ctx.Client, status, raw)
			}

			targetOwner := org
			if targetOwner == "" {
				userRaw, err := ctx.Client.Do("GET", "user", nil)
				if err != nil {
					return err
				}
				userObj, err := cmdutil.ParseObject(userRaw)
				if err != nil {
					return err
				}
				targetOwner = cmdutil.Str(userObj, "login")
			}
			_, repoName, _ := strings.Cut(repo, "/")
			existing, err := ctx.Client.Do("GET", "repos/"+cmdutil.PathEscape(targetOwner)+"/"+cmdutil.PathEscape(repoName), nil)
			if err != nil {
				return err
			}
			if ctx.WantsJSON() {
				return ctx.EmitJSON(existing)
			}
			obj, err := cmdutil.ParseObject(existing)
			if err != nil {
				return err
			}
			fmt.Fprintf(ctx.Out, "Fork already exists: %s\nURL: %s\n", cmdutil.Str(obj, "full_name"), cmdutil.Str(obj, "html_url"))
			return nil
		},
	}
	cmd.Flags().String("org", "", "organization to receive the fork")
	return cmd
}

func newRepoTransferCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "transfer <owner/repo> --new-owner=X",
		Short: "Transfer repository ownership",
		Long: "Transfer repository ownership.\n\n" +
			"The repo positional is required; pass '.' to infer owner/repo from the current git remote. --new-owner is required. The API cannot rename during transfer; --new-name is rejected and the repository must be renamed later with repo edit. This destructive command requires --yes or an interactive typed-name confirmation.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			repo, err := ctx.RepoArg(args[0])
			if err != nil {
				return err
			}
			if !cmd.Flags().Changed("new-owner") {
				return cmdutil.Usagef("Missing --new-owner")
			}
			newOwner, _ := cmd.Flags().GetString("new-owner")
			if cmd.Flags().Changed("new-name") {
				_, repoName, _ := strings.Cut(repo, "/")
				return cmdutil.Usagef("Transfer cannot rename a repo (the API ignores it).\nTransfer first, then rename the moved repo:\n  forgejo repo edit %s/%s --name=<new-name>", newOwner, repoName)
			}
			if err := ctx.ConfirmDelete(cmd, "repo (transfer)", repo); err != nil {
				return err
			}
			body, err := cmdutil.BuildBody(map[string]any{"new_owner": newOwner})
			if err != nil {
				return err
			}
			status, raw, err := ctx.Client.DoStatus("POST", "repos/"+repoAPIPath(repo)+"/transfer", body)
			if err != nil {
				return err
			}
			if status < 200 || status >= 300 {
				return apiStatusError(ctx.Client, status, raw)
			}
			if ctx.WantsJSON() {
				return ctx.EmitJSON(raw)
			}
			if status == 202 {
				fmt.Fprintf(ctx.Out, "Transfer initiated. Recipient (%s) must accept before it completes.\n", newOwner)
				return nil
			}
			obj, err := cmdutil.ParseObject(raw)
			if err != nil {
				return err
			}
			fmt.Fprintf(ctx.Out, "Transferred: %s\n", cmdutil.Str(obj, "full_name"))
			return nil
		},
	}
	cmd.Flags().String("new-owner", "", "new user or organization owner (required)")
	cmd.Flags().String("new-name", "", "unsupported; transfer cannot rename a repository")
	_ = cmd.Flags().MarkHidden("new-name")
	cmdutil.AddYesFlag(cmd)
	return cmd
}

func newRepoArchiveCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "archive <owner/repo>",
		Short: "Archive a repository",
		Long:  "Archive a repository by PATCHing archived=true. The repo positional is required; pass '.' to infer owner/repo from the current git remote. This destructive command requires --yes or an interactive typed-name confirmation.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			repo, err := ctx.RepoArg(args[0])
			if err != nil {
				return err
			}
			if err := ctx.ConfirmDelete(cmd, "repo (archive)", repo); err != nil {
				return err
			}
			body, err := cmdutil.BuildBody(map[string]any{"archived": true})
			if err != nil {
				return err
			}
			if _, err := ctx.Client.Do("PATCH", "repos/"+repoAPIPath(repo), body); err != nil {
				return err
			}
			fmt.Fprintf(ctx.Out, "Archived: %s\n", repo)
			return nil
		},
	}
	cmdutil.AddYesFlag(cmd)
	return cmd
}

func newRepoUnarchiveCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "unarchive <owner/repo>",
		Short: "Unarchive a repository",
		Long:  "Unarchive a repository by PATCHing archived=false. The repo positional is required; pass '.' to infer owner/repo from the current git remote.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			repo, err := ctx.RepoArg(args[0])
			if err != nil {
				return err
			}
			body, err := cmdutil.BuildBody(map[string]any{"archived": false})
			if err != nil {
				return err
			}
			if _, err := ctx.Client.Do("PATCH", "repos/"+repoAPIPath(repo), body); err != nil {
				return err
			}
			fmt.Fprintf(ctx.Out, "Unarchived: %s\n", repo)
			return nil
		},
	}
	return cmd
}

func newRepoTagsCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tags <list|create|delete>",
		Short: "Manage repository tags",
	}
	cmd.AddCommand(newRepoTagsListCmd(ctx), newRepoTagsCreateCmd(ctx), newRepoTagsDeleteCmd(ctx))
	return cmd
}

func newRepoTagsListCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list <owner/repo>",
		Short: "List tags",
		Long:  "List repository tags. The repo positional is required; pass '.' to infer owner/repo from the current git remote.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			repo, err := ctx.RepoArg(args[0])
			if err != nil {
				return err
			}
			raw, err := ctx.Client.Do("GET", "repos/"+repoAPIPath(repo)+"/tags", nil)
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
			for _, m := range items {
				rows = append(rows, []string{
					cmdutil.Str(m, "name"),
					dash(prefixRunes(cmdutil.Str(m, "commit.sha"), 7)),
					firstLine(cmdutil.Str(m, "message")),
				})
			}
			ctx.Table([]string{"NAME", "COMMIT", "MESSAGE"}, rows)
			return nil
		},
	}
	return cmd
}

func newRepoTagsCreateCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create <owner/repo> --tag=X [--message=X] [--target=<sha|branch>]",
		Short: "Create a tag",
		Long: "Create a repository tag.\n\n" +
			"The repo positional is required; pass '.' to infer owner/repo from the current git remote. --tag is required. --message and --target are included only when non-empty, matching the bash jq body.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			repo, err := ctx.RepoArg(args[0])
			if err != nil {
				return err
			}
			if !cmd.Flags().Changed("tag") {
				return cmdutil.Usagef("Missing --tag")
			}
			tag, _ := cmd.Flags().GetString("tag")
			message, _ := cmd.Flags().GetString("message")
			target, _ := cmd.Flags().GetString("target")
			fields := map[string]any{"tag_name": tag}
			if message != "" {
				fields["message"] = message
			}
			if target != "" {
				fields["target"] = target
			}
			body, err := cmdutil.BuildBody(fields)
			if err != nil {
				return err
			}
			raw, err := ctx.Client.Do("POST", "repos/"+repoAPIPath(repo)+"/tags", body)
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
			fmt.Fprintf(ctx.Out, "Created tag: %s at %s\n", cmdutil.Str(obj, "name"), prefixRunes(cmdutil.Str(obj, "commit.sha"), 7))
			return nil
		},
	}
	cmd.Flags().String("tag", "", "tag name to create (required)")
	cmd.Flags().String("message", "", "annotated tag message")
	cmd.Flags().String("target", "", "target commit SHA or branch")
	return cmd
}

func newRepoTagsDeleteCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete <owner/repo> <tag>",
		Short: "Delete a tag",
		Long:  "Delete a repository tag. The repo positional is required; pass '.' to infer owner/repo from the current git remote. This destructive command requires --yes or an interactive typed-name confirmation.",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			repo, err := ctx.RepoArg(args[0])
			if err != nil {
				return err
			}
			tag := args[1]
			if err := ctx.ConfirmDelete(cmd, "tag", tag); err != nil {
				return err
			}
			if _, err := ctx.Client.Do("DELETE", "repos/"+repoAPIPath(repo)+"/tags/"+cmdutil.NameSeg(tag), nil); err != nil {
				return err
			}
			fmt.Fprintf(ctx.Out, "Deleted tag: %s from %s\n", tag, repo)
			return nil
		},
	}
	cmdutil.AddYesFlag(cmd)
	return cmd
}

func newRepoCollaboratorCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "collaborator <list|add|remove>",
		Short: "Manage repository collaborators",
	}
	cmd.AddCommand(newRepoCollaboratorListCmd(ctx), newRepoCollaboratorAddCmd(ctx), newRepoCollaboratorRemoveCmd(ctx))
	return cmd
}

func newRepoCollaboratorListCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list <owner/repo>",
		Short: "List collaborators",
		Long:  "List repository collaborators. The repo positional is required; pass '.' to infer owner/repo from the current git remote.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			repo, err := ctx.RepoArg(args[0])
			if err != nil {
				return err
			}
			raw, err := ctx.Client.Do("GET", "repos/"+repoAPIPath(repo)+"/collaborators", nil)
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
			for _, m := range items {
				rows = append(rows, []string{
					cmdutil.Str(m, "login"),
					dash(cmdutil.Str(m, "full_name")),
					dash(cmdutil.Str(m, "email")),
				})
			}
			ctx.Table([]string{"LOGIN", "FULL_NAME", "EMAIL"}, rows)
			return nil
		},
	}
	return cmd
}

func newRepoCollaboratorAddCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add <owner/repo> --user=X [--permission=read|write|admin]",
		Short: "Add a collaborator",
		Long: "Add a repository collaborator.\n\n" +
			"The repo positional is required; pass '.' to infer owner/repo from the current git remote. --user is required. --permission defaults to write and is sent to the server exactly as provided.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			repo, err := ctx.RepoArg(args[0])
			if err != nil {
				return err
			}
			if !cmd.Flags().Changed("user") {
				return cmdutil.Usagef("Missing --user")
			}
			user, _ := cmd.Flags().GetString("user")
			permission, _ := cmd.Flags().GetString("permission")
			body, err := cmdutil.BuildBody(map[string]any{"permission": permission})
			if err != nil {
				return err
			}
			if _, err := ctx.Client.Do("PUT", "repos/"+repoAPIPath(repo)+"/collaborators/"+cmdutil.NameSeg(user), body); err != nil {
				return err
			}
			fmt.Fprintf(ctx.Out, "Added collaborator %s to %s with permission=%s\n", user, repo, permission)
			return nil
		},
	}
	cmd.Flags().String("user", "", "collaborator username (required)")
	cmd.Flags().String("permission", "write", "permission to grant (read, write, admin)")
	return cmd
}

func newRepoCollaboratorRemoveCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "remove <owner/repo> --user=X",
		Short: "Remove a collaborator",
		Long:  "Remove a repository collaborator. The repo positional is required; pass '.' to infer owner/repo from the current git remote. --user is required. This destructive command requires --yes or an interactive typed-name confirmation.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			repo, err := ctx.RepoArg(args[0])
			if err != nil {
				return err
			}
			if !cmd.Flags().Changed("user") {
				return cmdutil.Usagef("Missing --user")
			}
			user, _ := cmd.Flags().GetString("user")
			if err := ctx.ConfirmDelete(cmd, "collaborator", user); err != nil {
				return err
			}
			if _, err := ctx.Client.Do("DELETE", "repos/"+repoAPIPath(repo)+"/collaborators/"+cmdutil.NameSeg(user), nil); err != nil {
				return err
			}
			fmt.Fprintf(ctx.Out, "Removed collaborator %s from %s\n", user, repo)
			return nil
		},
	}
	cmd.Flags().String("user", "", "collaborator username to remove (required)")
	cmdutil.AddYesFlag(cmd)
	return cmd
}

func newRepoWebhookCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "webhook <list|view|create|edit|delete>",
		Short: "Manage repository webhooks",
	}
	cmd.AddCommand(
		newRepoWebhookListCmd(ctx),
		newRepoWebhookViewCmd(ctx),
		newRepoWebhookCreateCmd(ctx),
		newRepoWebhookEditCmd(ctx),
		newRepoWebhookDeleteCmd(ctx),
	)
	return cmd
}

func newRepoWebhookListCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list <owner/repo>",
		Short: "List webhooks",
		Long:  "List repository webhooks. The repo positional is required; pass '.' to infer owner/repo from the current git remote.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			repo, err := ctx.RepoArg(args[0])
			if err != nil {
				return err
			}
			raw, err := ctx.Client.Do("GET", "repos/"+repoAPIPath(repo)+"/hooks", nil)
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
			for _, m := range items {
				rows = append(rows, []string{
					cmdutil.Str(m, "id"),
					dash(cmdutil.Str(m, "config.url")),
					yesNo(cmdutil.Str(m, "active")),
					cmdutil.Str(m, "events"),
				})
			}
			ctx.Table([]string{"ID", "URL", "ACTIVE", "EVENTS"}, rows)
			return nil
		},
	}
	return cmd
}

func newRepoWebhookViewCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "view <owner/repo> <id>",
		Short: "View webhook details",
		Long:  "View a repository webhook by numeric id. The repo positional is required; pass '.' to infer owner/repo from the current git remote.",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			repo, err := ctx.RepoArg(args[0])
			if err != nil {
				return err
			}
			id, err := cmdutil.IDArg(args[1], "webhook id")
			if err != nil {
				return err
			}
			raw, err := ctx.Client.Do("GET", "repos/"+repoAPIPath(repo)+"/hooks/"+id, nil)
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
			fmt.Fprintf(ctx.Out, "URL:          %s\n", dash(cmdutil.Str(obj, "config.url")))
			fmt.Fprintf(ctx.Out, "ContentType:  %s\n", dash(cmdutil.Str(obj, "config.content_type")))
			fmt.Fprintf(ctx.Out, "Active:       %s\n", yesNo(cmdutil.Str(obj, "active")))
			fmt.Fprintf(ctx.Out, "Events:       %s\n", cmdutil.Str(obj, "events"))
			fmt.Fprintf(ctx.Out, "Created:      %s\n", prefixRunes(cmdutil.Str(obj, "created_at"), 19))
			fmt.Fprintf(ctx.Out, "Updated:      %s\n", prefixRunes(cmdutil.Str(obj, "updated_at"), 19))
			return nil
		},
	}
	return cmd
}

func newRepoWebhookCreateCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create <owner/repo> --url=X --events=a,b [--secret=X] [--content-type=json|form] [--inactive]",
		Short: "Create a webhook",
		Long: "Create a forgejo-type repository webhook.\n\n" +
			"The repo positional is required; pass '.' to infer owner/repo from the current git remote. --url and --events are required. --events is a comma-separated list trimmed like the bash jq expression. --content-type defaults to json. --inactive sends active=false.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			repo, err := ctx.RepoArg(args[0])
			if err != nil {
				return err
			}
			if !cmd.Flags().Changed("url") {
				return cmdutil.Usagef("Missing --url")
			}
			if !cmd.Flags().Changed("events") {
				return cmdutil.Usagef("Missing --events")
			}
			hookURL, _ := cmd.Flags().GetString("url")
			events, _ := cmd.Flags().GetString("events")
			secret, _ := cmd.Flags().GetString("secret")
			contentType, _ := cmd.Flags().GetString("content-type")
			inactive, _ := cmd.Flags().GetBool("inactive")

			cfg := map[string]any{"url": hookURL, "content_type": contentType}
			if secret != "" {
				cfg["secret"] = secret
			}
			body, err := cmdutil.BuildBody(map[string]any{
				"type":   "forgejo",
				"config": cfg,
				"events": splitCSVTrimKeepEmpty(events),
				"active": !inactive,
			})
			if err != nil {
				return err
			}
			raw, err := ctx.Client.Do("POST", "repos/"+repoAPIPath(repo)+"/hooks", body)
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
			fmt.Fprintf(ctx.Out, "Created webhook #%s on %s\n", cmdutil.Str(obj, "id"), cmdutil.Str(obj, "config.url"))
			return nil
		},
	}
	cmd.Flags().String("url", "", "webhook target URL (required)")
	cmd.Flags().String("events", "", "comma-separated webhook events (required)")
	cmd.Flags().String("secret", "", "webhook secret")
	cmd.Flags().String("content-type", "json", "payload content type (json or form)")
	cmd.Flags().Bool("inactive", false, "create the webhook with active=false")
	return cmd
}

func newRepoWebhookEditCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "edit <owner/repo> <id> [--url=X] [--events=a,b] [--secret=X] [--content-type=X] [--active|--inactive]",
		Short: "Edit a webhook",
		Long: "Edit a repository webhook by numeric id.\n\n" +
			"The repo positional is required; pass '.' to infer owner/repo from the current git remote. Config keys --url, --content-type, and --secret are sent as a partial config object only when at least one is supplied. --events replaces the event list. --active and --inactive are mutually exclusive. At least one editable field is required.",
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			repo, err := ctx.RepoArg(args[0])
			if err != nil {
				return err
			}
			id, err := cmdutil.IDArg(args[1], "webhook id")
			if err != nil {
				return err
			}
			active, _ := cmd.Flags().GetBool("active")
			inactive, _ := cmd.Flags().GetBool("inactive")
			if active && inactive {
				return cmdutil.Usagef("--active and --inactive are mutually exclusive")
			}

			fields := map[string]any{}
			haveCfg := cmd.Flags().Changed("url") || cmd.Flags().Changed("content-type") || cmd.Flags().Changed("secret")
			if haveCfg {
				cfg := map[string]any{}
				if v, _ := cmd.Flags().GetString("url"); v != "" {
					cfg["url"] = v
				}
				if v, _ := cmd.Flags().GetString("content-type"); v != "" {
					cfg["content_type"] = v
				}
				if v, _ := cmd.Flags().GetString("secret"); v != "" {
					cfg["secret"] = v
				}
				fields["config"] = cfg
			}
			if cmd.Flags().Changed("events") {
				events, _ := cmd.Flags().GetString("events")
				fields["events"] = splitCSVTrimKeepEmpty(events)
			}
			if active {
				fields["active"] = true
			} else if inactive {
				fields["active"] = false
			}
			if len(fields) == 0 {
				return cmdutil.Usagef("No webhook fields supplied to edit")
			}

			body, err := cmdutil.BuildBody(fields)
			if err != nil {
				return err
			}
			raw, err := ctx.Client.Do("PATCH", "repos/"+repoAPIPath(repo)+"/hooks/"+id, body)
			if err != nil {
				return err
			}
			if ctx.WantsJSON() {
				return ctx.EmitJSON(raw)
			}
			fmt.Fprintf(ctx.Out, "Updated webhook #%s on %s\n", id, repo)
			return nil
		},
	}
	cmd.Flags().String("url", "", "new webhook target URL")
	cmd.Flags().String("events", "", "comma-separated webhook events")
	cmd.Flags().String("secret", "", "new webhook secret")
	cmd.Flags().String("content-type", "", "new payload content type")
	cmd.Flags().Bool("active", false, "set active=true")
	cmd.Flags().Bool("inactive", false, "set active=false")
	return cmd
}

func newRepoWebhookDeleteCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete <owner/repo> <id>",
		Short: "Delete a webhook",
		Long:  "Delete a repository webhook by numeric id. The repo positional is required; pass '.' to infer owner/repo from the current git remote. This destructive command requires --yes or an interactive typed-name confirmation.",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			repo, err := ctx.RepoArg(args[0])
			if err != nil {
				return err
			}
			id, err := cmdutil.IDArg(args[1], "webhook id")
			if err != nil {
				return err
			}
			if err := ctx.ConfirmDelete(cmd, "webhook", id); err != nil {
				return err
			}
			if _, err := ctx.Client.Do("DELETE", "repos/"+repoAPIPath(repo)+"/hooks/"+id, nil); err != nil {
				return err
			}
			fmt.Fprintf(ctx.Out, "Deleted webhook #%s from %s\n", id, repo)
			return nil
		},
	}
	cmdutil.AddYesFlag(cmd)
	return cmd
}

func newRepoKeyCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "key <list|add|delete>",
		Short: "Manage deploy keys",
	}
	cmd.AddCommand(newRepoKeyListCmd(ctx), newRepoKeyAddCmd(ctx), newRepoKeyDeleteCmd(ctx))
	return cmd
}

func newRepoKeyListCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list <owner/repo>",
		Short: "List deploy keys",
		Long:  "List repository deploy keys. The repo positional is required; pass '.' to infer owner/repo from the current git remote.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			repo, err := ctx.RepoArg(args[0])
			if err != nil {
				return err
			}
			raw, err := ctx.Client.Do("GET", "repos/"+repoAPIPath(repo)+"/keys", nil)
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
			for _, m := range items {
				rows = append(rows, []string{
					cmdutil.Str(m, "id"),
					dash(cmdutil.Str(m, "title")),
					dash(cmdutil.Str(m, "fingerprint")),
					yesNo(cmdutil.Str(m, "read_only")),
				})
			}
			ctx.Table([]string{"ID", "TITLE", "FINGERPRINT", "READONLY"}, rows)
			return nil
		},
	}
	return cmd
}

func newRepoKeyAddCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add <owner/repo> --title=X (--key=X | --key-file=path) [--read-only]",
		Short: "Add a deploy key",
		Long: "Add a repository deploy key.\n\n" +
			"The repo positional is required; pass '.' to infer owner/repo from the current git remote. --title is required. Provide exactly one non-empty key source: --key for the literal public key, or --key-file to read the key from a file. --read-only sends read_only=true.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			repo, err := ctx.RepoArg(args[0])
			if err != nil {
				return err
			}
			if !cmd.Flags().Changed("title") {
				return cmdutil.Usagef("Missing --title")
			}
			title, _ := cmd.Flags().GetString("title")
			key, err := readDeployKeyInput(cmd)
			if err != nil {
				return err
			}
			readOnly, _ := cmd.Flags().GetBool("read-only")
			body, err := cmdutil.BuildBody(map[string]any{
				"title":     title,
				"key":       key,
				"read_only": readOnly,
			})
			if err != nil {
				return err
			}
			raw, err := ctx.Client.Do("POST", "repos/"+repoAPIPath(repo)+"/keys", body)
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
			fmt.Fprintf(ctx.Out, "Added deploy key #%s: %s\n", cmdutil.Str(obj, "id"), cmdutil.Str(obj, "title"))
			return nil
		},
	}
	cmd.Flags().String("title", "", "deploy key title (required)")
	cmd.Flags().String("key", "", "literal public key; mutually exclusive with --key-file")
	cmd.Flags().String("key-file", "", "file containing the public key; mutually exclusive with --key")
	cmd.Flags().Bool("read-only", false, "add the key with read_only=true")
	return cmd
}

func newRepoKeyDeleteCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete <owner/repo> <id>",
		Short: "Delete a deploy key",
		Long:  "Delete a repository deploy key by numeric id. The repo positional is required; pass '.' to infer owner/repo from the current git remote. This destructive command requires --yes or an interactive typed-name confirmation.",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			repo, err := ctx.RepoArg(args[0])
			if err != nil {
				return err
			}
			id, err := cmdutil.IDArg(args[1], "deploy key id")
			if err != nil {
				return err
			}
			if err := ctx.ConfirmDelete(cmd, "deploy key", id); err != nil {
				return err
			}
			if _, err := ctx.Client.Do("DELETE", "repos/"+repoAPIPath(repo)+"/keys/"+id, nil); err != nil {
				return err
			}
			fmt.Fprintf(ctx.Out, "Deleted deploy key #%s from %s\n", id, repo)
			return nil
		},
	}
	cmdutil.AddYesFlag(cmd)
	return cmd
}

func newRepoTopicCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "topic <list|add|remove>",
		Short: "Manage repository topics",
	}
	cmd.AddCommand(newRepoTopicListCmd(ctx), newRepoTopicAddCmd(ctx), newRepoTopicRemoveCmd(ctx))
	return cmd
}

func newRepoTopicListCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list <owner/repo>",
		Short: "List topics",
		Long:  "List repository topics. The repo positional is required; pass '.' to infer owner/repo from the current git remote. Text output prints one topic per line with no header, matching bash.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			repo, err := ctx.RepoArg(args[0])
			if err != nil {
				return err
			}
			raw, err := ctx.Client.Do("GET", "repos/"+repoAPIPath(repo)+"/topics", nil)
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
			topics, _ := obj["topics"].([]any)
			for _, topic := range topics {
				fmt.Fprintln(ctx.Out, scalarString(topic))
			}
			return nil
		},
	}
	return cmd
}

func newRepoTopicAddCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add <owner/repo> --topics=a,b",
		Short: "Add topics",
		Long:  "Add repository topics. The repo positional is required; pass '.' to infer owner/repo from the current git remote. --topics is a comma-separated list; each topic is trimmed, empty entries are skipped, and each non-empty topic is added with PUT /repos/{repo}/topics/{topic}.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			repo, err := ctx.RepoArg(args[0])
			if err != nil {
				return err
			}
			if !cmd.Flags().Changed("topics") {
				return cmdutil.Usagef("Missing --topics")
			}
			rawTopics, _ := cmd.Flags().GetString("topics")
			topics := splitComma(rawTopics)
			for _, topic := range topics {
				if _, err := ctx.Client.Do("PUT", "repos/"+repoAPIPath(repo)+"/topics/"+cmdutil.NameSeg(topic), nil); err != nil {
					return err
				}
			}
			fmt.Fprintf(ctx.Out, "Added topics to %s: %s\n", repo, strings.Join(topics, " "))
			return nil
		},
	}
	cmd.Flags().String("topics", "", "comma-separated topics to add (required)")
	return cmd
}

func newRepoTopicRemoveCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "remove <owner/repo> --topics=a,b",
		Short: "Remove topics",
		Long:  "Remove repository topics. The repo positional is required; pass '.' to infer owner/repo from the current git remote. --topics is a comma-separated list; each topic is trimmed, empty entries are skipped, and each non-empty topic is removed with DELETE /repos/{repo}/topics/{topic}. This destructive command requires --yes or an interactive typed-name confirmation.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			repo, err := ctx.RepoArg(args[0])
			if err != nil {
				return err
			}
			if !cmd.Flags().Changed("topics") {
				return cmdutil.Usagef("Missing --topics")
			}
			rawTopics, _ := cmd.Flags().GetString("topics")
			topics := splitComma(rawTopics)
			if len(topics) > 0 {
				if err := ctx.ConfirmDelete(cmd, "topics", strings.Join(topics, " ")); err != nil {
					return err
				}
			}
			for _, topic := range topics {
				if _, err := ctx.Client.Do("DELETE", "repos/"+repoAPIPath(repo)+"/topics/"+cmdutil.NameSeg(topic), nil); err != nil {
					return err
				}
			}
			fmt.Fprintf(ctx.Out, "Removed topics from %s: %s\n", repo, strings.Join(topics, " "))
			return nil
		},
	}
	cmd.Flags().String("topics", "", "comma-separated topics to remove (required)")
	cmdutil.AddYesFlag(cmd)
	return cmd
}

func newRepoMirrorCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mirror <sync>",
		Short: "Manage repository mirrors",
	}
	cmd.AddCommand(newRepoMirrorSyncCmd(ctx))
	return cmd
}

func newRepoMirrorSyncCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sync <owner/repo>",
		Short: "Trigger mirror sync",
		Long:  "Trigger push-mirror sync for a repository. The repo positional is required; pass '.' to infer owner/repo from the current git remote.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			repo, err := ctx.RepoArg(args[0])
			if err != nil {
				return err
			}
			if _, err := ctx.Client.Do("POST", "repos/"+repoAPIPath(repo)+"/mirror-sync", nil); err != nil {
				return err
			}
			fmt.Fprintf(ctx.Out, "Triggered mirror sync for %s\n", repo)
			return nil
		},
	}
	return cmd
}

func prefixRunes(s string, n int) string {
	if s == "" {
		return ""
	}
	return cmdutil.Trunc(s, n)
}

func splitCSVTrimKeepEmpty(s string) []string {
	parts := strings.Split(s, ",")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}

func readDeployKeyInput(cmd *cobra.Command) (string, error) {
	key, _ := cmd.Flags().GetString("key")
	keyFile, _ := cmd.Flags().GetString("key-file")
	if key != "" && keyFile != "" {
		return "", cmdutil.Usagef("Provide --key OR --key-file, not both")
	}
	if key == "" && keyFile == "" {
		return "", cmdutil.Usagef("Missing --key or --key-file")
	}
	if keyFile == "" {
		return key, nil
	}
	f, err := os.Open(keyFile)
	if err != nil {
		return "", fmt.Errorf("Cannot read key file: %s", keyFile)
	}
	defer f.Close()
	data, err := io.ReadAll(f)
	if err != nil {
		return "", err
	}
	return strings.TrimRight(string(data), "\n"), nil
}

func apiStatusError(client *api.Client, status int, raw []byte) error {
	e := &api.Error{Status: status, Message: apiMessage(raw)}
	switch status {
	case 401:
		e.Hint = "token rejected - check FORGEJO_TOKEN (forgejo auth status)"
	case 403:
		e.Hint = scopeHint(client)
	}
	return e
}

func apiMessage(raw []byte) string {
	msg := strings.TrimSpace(string(raw))
	var parsed struct {
		Message string `json:"message"`
		Err     string `json:"error"`
	}
	if err := json.Unmarshal(raw, &parsed); err == nil {
		if parsed.Message != "" {
			return parsed.Message
		}
		if parsed.Err != "" {
			return parsed.Err
		}
	}
	return msg
}

func scopeHint(client *api.Client) string {
	status, raw, err := client.DoStatus("GET", "user/tokens/current", nil)
	if err != nil || status < 200 || status >= 300 {
		return "token may lack the required scope for this operation"
	}
	var tok struct {
		Name   string   `json:"name"`
		Scopes []string `json:"scopes"`
	}
	if json.Unmarshal(raw, &tok) != nil || len(tok.Scopes) == 0 {
		return "token may lack the required scope for this operation"
	}
	return fmt.Sprintf("token %q has scopes: %s - this operation needs more", tok.Name, strings.Join(tok.Scopes, ", "))
}
