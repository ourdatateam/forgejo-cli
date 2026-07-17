package cmd

// Ported from the bash cmd_issue family (forgejo:1582-2481). The issue
// command has several API-shape workarounds that are intentionally preserved:
// issue edit replaces labels via the dedicated labels endpoint, label
// resolution checks org labels before repo labels, assignee updates are
// GET-then-PATCH, and attachment downloads only send credentials to the
// configured Forgejo origin.

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/ourdatateam/forgejo-cli/internal/cmdutil"
	"github.com/spf13/cobra"
)

var milestoneDueRe = regexp.MustCompile(`^[0-9]{4}-[0-9]{2}-[0-9]{2}$`)

func init() { Register(newIssueCmd) }

func newIssueCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "issue <list|create|view|edit|comment|label|close|reopen|assign|unassign|search|milestone|images>",
		Short: "Manage issues, issue labels, milestones, comments, and attachments",
		Long: `Issue commands:

  issue list <owner/repo> [--state=open|closed]
      List issues in a repository.

  issue create <owner/repo> --title=TEXT [--body=TEXT]
      Create a new issue.

  issue view <owner/repo> <number>
      Show issue details and all comments inline.

  issue edit <owner/repo> <number> [--title=TEXT] [--state=open|closed] [--body=TEXT] [--labels=X,Y]
      Edit an issue's title, state, body, labels, or assignees.

  issue comment <owner/repo> <number> --body=TEXT
      Add a comment to an issue.
      Comment IDs are shown in issue view and in the API response (--json).

  issue comment delete <owner/repo> <comment_id>
      Delete a specific comment by its numeric ID.

  There is NO separate issue comment list or issue comment view.
  Use forgejo issue view <owner/repo> <number>, which shows all comments inline.

  issue close <owner/repo> <number>
      Close an issue.

  issue reopen <owner/repo> <number>
      Reopen a closed issue.

  issue assign <owner/repo> <number> --users=u1,u2
      Add assignees to an issue (union with existing).

  issue unassign <owner/repo> <number> --users=u1,u2
      Remove assignees from an issue.

  issue search [--owner=ORG] [--state=open|closed|all] [--labels=a,b] [--query=TEXT] [--limit=N]
      Search issues across repositories.

  issue milestone list <owner/repo> [--state=open|closed|all]
  issue milestone create <owner/repo> --title=TEXT [--description=TEXT] [--due=YYYY-MM-DD]
  issue milestone edit <owner/repo> <id> [--title=TEXT] [--description=TEXT] [--due=YYYY-MM-DD] [--state=open|closed]
  issue milestone delete <owner/repo> <id>
  issue milestone set <owner/repo> <number> --milestone=<id|title>
      Manage milestones. Pass --milestone=0 to clear.

  issue label <owner/repo> list [--scope=org|repo]
      List labels (scope defaults to org).

  issue label <owner/repo> create --name=TEXT [--color=HEX] [--desc=TEXT] [--scope=org|repo]
      Create a label.

  issue label <owner/repo> add <number> --labels=X,Y
      Add labels to an issue.

  issue label <owner/repo> remove <number> --label=TEXT
      Remove a single label from an issue.

  issue images <owner/repo> <number> [--output=DIR]
      Download all image attachments (body + comments) to DIR.
      DIR defaults to ./issue-<number>-images/. Non-image attachments
      (extension not in .png .jpg .jpeg .gif .webp .svg .bmp) are skipped.`,
	}
	cmd.AddCommand(
		newIssueListCmd(ctx),
		newIssueCreateCmd(ctx),
		newIssueViewCmd(ctx),
		newIssueEditCmd(ctx),
		newIssueCommentCmd(ctx),
		newIssueLabelCmd(ctx),
		newIssueCloseCmd(ctx),
		newIssueReopenCmd(ctx),
		newIssueAssignCmd(ctx),
		newIssueUnassignCmd(ctx),
		newIssueSearchCmd(ctx),
		newIssueMilestoneCmd(ctx),
		newIssueImagesCmd(ctx),
	)
	return cmd
}

func newIssueListCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list <owner/repo> [--state=open|closed]",
		Short: "List issues in a repository",
		Long:  "List issues in a repository. The repository argument is required; pass . to infer it from the current git remote. State defaults to open and is passed through to the server.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			repo, err := ctx.RepoArg(args[0])
			if err != nil {
				return err
			}
			state, _ := cmd.Flags().GetString("state")
			endpoint := "repos/" + repoAPIPath(repo) + "/issues?state=" + cmdutil.QueryEscape(state) + "&type=issues"
			n := ctx.ListLimit(50)
			lr, err := ctx.Client.DoList(endpoint, n)
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
				fmt.Fprintln(ctx.Out, "No issues found.")
				return nil
			}
			rows := make([][]string, 0, len(items))
			for _, m := range items {
				rows = append(rows, []string{
					cmdutil.Str(m, "number"),
					cmdutil.Trunc(cmdutil.Str(m, "title"), 60),
					cmdutil.Str(m, "state"),
					cmdutil.Str(m, "user.login"),
					firstN(cmdutil.Str(m, "updated_at"), 10),
				})
			}
			ctx.Table([]string{"#", "TITLE", "STATE", "AUTHOR", "UPDATED"}, rows)
			ctx.Trailer(len(items), lr.Total, n)
			return nil
		},
	}
	cmd.Flags().String("state", "open", "issue state filter (open, closed, or all)")
	return cmd
}

func newIssueCreateCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create <owner/repo> --title=TEXT [--body=TEXT] [--labels=X,Y]",
		Short: "Create an issue",
		Long:  "Create a new issue. --title is required. --body may be text, --body=- to read stdin, or --body-file=PATH. --labels resolves comma-separated label names to IDs before creating the issue.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			repo, err := ctx.RepoArg(args[0])
			if err != nil {
				return err
			}
			if !cmd.Flags().Changed("title") {
				return cmdutil.Usagef("Missing --title")
			}
			title, _ := cmd.Flags().GetString("title")
			bodyText, present, err := ctx.Body(cmd)
			if err != nil {
				return err
			}
			if !present {
				bodyText = ""
			}
			fields := map[string]any{
				"title": title,
				"body":  bodyText,
			}
			if cmd.Flags().Changed("labels") {
				labels, _ := cmd.Flags().GetString("labels")
				ids, err := resolveLabelIDs(ctx, repo, labels)
				if err != nil {
					return err
				}
				fields["labels"] = ids
			}
			req, err := cmdutil.BuildBody(fields)
			if err != nil {
				return err
			}
			raw, err := ctx.Client.Do("POST", "repos/"+repoAPIPath(repo)+"/issues", req)
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
			fmt.Fprintf(ctx.Out, "Created issue #%s: %s\nURL: %s\n",
				cmdutil.Str(obj, "number"), cmdutil.Str(obj, "title"), cmdutil.Str(obj, "html_url"))
			return nil
		},
	}
	cmd.Flags().String("title", "", "issue title (required)")
	cmd.Flags().String("labels", "", "comma-separated label names to apply")
	cmdutil.AddBodyFlags(cmd)
	return cmd
}

func newIssueViewCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "view <owner/repo> <number>",
		Short: "View an issue and its comments",
		Long:  "Show issue details and all comments inline. JSON output is the raw issue object, matching the bash command; comments are fetched only for text output.",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			repo, err := ctx.RepoArg(args[0])
			if err != nil {
				return err
			}
			number, err := cmdutil.IDArg(args[1], "issue number")
			if err != nil {
				return err
			}
			raw, err := ctx.Client.Do("GET", "repos/"+repoAPIPath(repo)+"/issues/"+number, nil)
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
			fmt.Fprintf(ctx.Out, "#%s: %s\n", cmdutil.Str(obj, "number"), cmdutil.Str(obj, "title"))
			fmt.Fprintf(ctx.Out, "State:   %s\n", cmdutil.Str(obj, "state"))
			fmt.Fprintf(ctx.Out, "Author:  %s\n", cmdutil.Str(obj, "user.login"))
			fmt.Fprintf(ctx.Out, "Created: %s\n", firstN(cmdutil.Str(obj, "created_at"), 10))
			fmt.Fprintf(ctx.Out, "Labels:  %s\n", issueLabelNames(obj))
			fmt.Fprintln(ctx.Out)
			fmt.Fprintln(ctx.Out, issueBody(obj))
			fmt.Fprintln(ctx.Out)
			fmt.Fprintln(ctx.Out, "--- Comments ---")

			commentsRaw, err := ctx.Client.DoPaged("repos/" + repoAPIPath(repo) + "/issues/" + number + "/comments")
			if err != nil {
				return err
			}
			comments, err := cmdutil.ParseArray(commentsRaw)
			if err != nil {
				return err
			}
			if len(comments) == 0 {
				fmt.Fprintln(ctx.Out, "No comments.")
				return nil
			}
			for _, c := range comments {
				fmt.Fprintf(ctx.Out, "[comment #%s] %s (%s):\n%s\n\n",
					cmdutil.Str(c, "id"),
					cmdutil.Str(c, "user.login"),
					firstN(cmdutil.Str(c, "created_at"), 10),
					cmdutil.Str(c, "body"))
			}
			return nil
		},
	}
	return cmd
}

func newIssueEditCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "edit <owner/repo> <number> [--title=TEXT] [--state=open|closed] [--body=TEXT] [--labels=X,Y]",
		Short: "Edit an issue",
		Long:  "Edit an issue's title, state, body, labels, or assignees. Forgejo's EditIssueOption has no labels field, so --labels replaces all labels with a separate PUT to the labels endpoint after the issue PATCH. Passing --labels= with an empty value clears all labels. --add-labels and --remove-labels incrementally append or remove labels. --add-assignees and --remove-assignees incrementally update the assignee list.",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			repo, err := ctx.RepoArg(args[0])
			if err != nil {
				return err
			}
			number, err := cmdutil.IDArg(args[1], "issue number")
			if err != nil {
				return err
			}

			issueEndpoint := "repos/" + repoAPIPath(repo) + "/issues/" + number
			labelsChanged := cmd.Flags().Changed("labels")
			addLabelsChanged := cmd.Flags().Changed("add-labels")
			removeLabelsChanged := cmd.Flags().Changed("remove-labels")
			assigneesChanged := cmd.Flags().Changed("assignees")
			addAssigneesChanged := cmd.Flags().Changed("add-assignees")
			removeAssigneesChanged := cmd.Flags().Changed("remove-assignees")
			incrementalLabelsChanged := addLabelsChanged || removeLabelsChanged
			incrementalAssigneesChanged := addAssigneesChanged || removeAssigneesChanged
			incrementalChanged := incrementalLabelsChanged || incrementalAssigneesChanged

			if labelsChanged && incrementalLabelsChanged {
				return cmdutil.Usagef("cannot combine --labels with --add-labels or --remove-labels")
			}
			if assigneesChanged && incrementalAssigneesChanged {
				return cmdutil.Usagef("cannot combine --assignees with --add-assignees or --remove-assignees")
			}

			fields := map[string]any{}
			if cmd.Flags().Changed("title") {
				v, _ := cmd.Flags().GetString("title")
				fields["title"] = v
			}
			if cmd.Flags().Changed("state") {
				v, _ := cmd.Flags().GetString("state")
				fields["state"] = v
			}
			bodyText, bodyPresent, err := ctx.Body(cmd)
			if err != nil {
				return err
			}
			if bodyPresent {
				fields["body"] = bodyText
			}
			if assigneesChanged {
				v, _ := cmd.Flags().GetString("assignees")
				fields["assignees"] = splitIssueCSV(v, true)
			}

			var labelIDs []json.Number
			if labelsChanged {
				labels, _ := cmd.Flags().GetString("labels")
				labelIDs, err = resolveLabelIDs(ctx, repo, labels)
				if err != nil {
					return err
				}
			}

			var addLabelIDs []json.Number
			if addLabelsChanged {
				labels, _ := cmd.Flags().GetString("add-labels")
				addLabelIDs, err = resolveLabelIDs(ctx, repo, labels)
				if err != nil {
					return err
				}
			}
			var removeLabelIDs []json.Number
			if removeLabelsChanged {
				labels, _ := cmd.Flags().GetString("remove-labels")
				removeLabelIDs, err = resolveLabelIDs(ctx, repo, labels)
				if err != nil {
					return err
				}
			}
			if incrementalAssigneesChanged {
				raw, err := ctx.Client.Do("GET", issueEndpoint, nil)
				if err != nil {
					return err
				}
				obj, err := cmdutil.ParseObject(raw)
				if err != nil {
					return err
				}
				addAssignees, _ := cmd.Flags().GetString("add-assignees")
				removeAssignees, _ := cmd.Flags().GetString("remove-assignees")
				next, err := mergeIssueEditAssignees(issueAssignees(obj), addAssignees, removeAssignees, addAssigneesChanged, removeAssigneesChanged)
				if err != nil {
					return err
				}
				fields["assignees"] = next
			}
			if len(fields) == 0 && !labelsChanged && !incrementalLabelsChanged {
				return cmdutil.Usagef("No fields to update. Provide --title, --state, --body, or --labels.")
			}

			var result []byte
			if incrementalChanged {
				if labelsChanged {
					req, err := cmdutil.BuildBody(map[string]any{"labels": labelIDs})
					if err != nil {
						return err
					}
					if _, err := ctx.Client.Do("PUT", issueEndpoint+"/labels", req); err != nil {
						return err
					}
				}
				if addLabelsChanged {
					req, err := cmdutil.BuildBody(map[string]any{"labels": addLabelIDs})
					if err != nil {
						return err
					}
					if _, err := ctx.Client.Do("POST", issueEndpoint+"/labels", req); err != nil {
						return err
					}
				}
				if removeLabelsChanged {
					for _, id := range removeLabelIDs {
						if _, err := ctx.Client.Do("DELETE", issueEndpoint+"/labels/"+id.String(), nil); err != nil {
							return err
						}
					}
				}
				if len(fields) != 0 {
					req, err := cmdutil.BuildBody(fields)
					if err != nil {
						return err
					}
					result, err = ctx.Client.Do("PATCH", issueEndpoint, req)
					if err != nil {
						return err
					}
				}
			} else {
				if len(fields) != 0 {
					req, err := cmdutil.BuildBody(fields)
					if err != nil {
						return err
					}
					result, err = ctx.Client.Do("PATCH", issueEndpoint, req)
					if err != nil {
						return err
					}
				}
				if labelsChanged {
					req, err := cmdutil.BuildBody(map[string]any{"labels": labelIDs})
					if err != nil {
						return err
					}
					if _, err := ctx.Client.Do("PUT", issueEndpoint+"/labels", req); err != nil {
						return err
					}
				}
			}
			if len(result) == 0 {
				result, err = ctx.Client.Do("GET", issueEndpoint, nil)
				if err != nil {
					return err
				}
			}
			if ctx.WantsJSON() {
				return ctx.EmitJSON(result)
			}
			obj, err := cmdutil.ParseObject(result)
			if err != nil {
				return err
			}
			fmt.Fprintf(ctx.Out, "Updated issue #%s: %s\n", cmdutil.Str(obj, "number"), cmdutil.Str(obj, "title"))
			return nil
		},
	}
	cmd.Flags().String("title", "", "new issue title")
	cmd.Flags().String("state", "", "new issue state (open or closed)")
	cmd.Flags().String("labels", "", "comma-separated label names to replace all labels; empty clears all labels")
	cmd.Flags().String("add-labels", "", "comma-separated label names to add")
	cmd.Flags().String("remove-labels", "", "comma-separated label names to remove")
	cmd.Flags().String("assignees", "", "comma-separated assignee logins to replace all assignees; empty clears all assignees")
	cmd.Flags().String("add-assignees", "", "comma-separated assignee logins to add")
	cmd.Flags().String("remove-assignees", "", "comma-separated assignee logins to remove")
	cmdutil.AddBodyFlags(cmd)
	return cmd
}

func newIssueCommentCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "comment <owner/repo> <number> --body=TEXT",
		Short: "Add or delete issue comments",
		Long: `Issue comment commands:
  forgejo issue comment <owner/repo> <number> --body=TEXT
      Add a comment to an issue or PR.
      The <number> is the issue number (for example, #42).
      --body=- reads stdin and --body-file=PATH reads from a file.

  forgejo issue comment delete <owner/repo> <comment_id>
      Delete a comment by its numeric comment ID.
      Find the comment ID from forgejo issue view <owner/repo> <number>
      (shown as [comment #123]) or use --json for raw API output.

IMPORTANT NOTES:
  - There is NO issue comment list or issue comment view subcommand.
    To see all comments on an issue, use: forgejo issue view <owner/repo> <number>
  - Issue and PR comments share the same API backend; comment IDs are globally
    unique within a repo across both issues and PRs.
  - issue comment on a PR number also works (comments on the PR thread).`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			repo, err := ctx.RepoArg(args[0])
			if err != nil {
				return err
			}
			number, err := cmdutil.IDArg(args[1], "issue number")
			if err != nil {
				return err
			}
			bodyText, present, err := ctx.Body(cmd)
			if err != nil {
				return err
			}
			if !present {
				return cmdutil.Usagef("Missing --body")
			}
			req, err := cmdutil.BuildBody(map[string]any{"body": bodyText})
			if err != nil {
				return err
			}
			raw, err := ctx.Client.Do("POST", "repos/"+repoAPIPath(repo)+"/issues/"+number+"/comments", req)
			if err != nil {
				return err
			}
			if ctx.WantsJSON() {
				return ctx.EmitJSON(raw)
			}
			fmt.Fprintf(ctx.Out, "Comment added to issue #%s\n", number)
			return nil
		},
	}
	cmdutil.AddBodyFlags(cmd)
	cmd.AddCommand(newIssueCommentDeleteCmd(ctx))
	return cmd
}

func newIssueCommentDeleteCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete <owner/repo> <comment_id>",
		Short: "Delete an issue comment",
		Long:  "Delete a comment by its numeric comment ID. Comment IDs are shown by issue view as [comment #123] and in raw JSON output. Requires --yes or an interactive typed confirmation.",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			repo, err := ctx.RepoArg(args[0])
			if err != nil {
				return err
			}
			commentID, err := cmdutil.IDArg(args[1], "comment id")
			if err != nil {
				return err
			}
			if err := ctx.ConfirmDelete(cmd, "comment", "#"+commentID); err != nil {
				return err
			}
			if _, err := ctx.Client.Do("DELETE", "repos/"+repoAPIPath(repo)+"/issues/comments/"+commentID, nil); err != nil {
				return err
			}
			fmt.Fprintf(ctx.Out, "Deleted comment #%s\n", commentID)
			return nil
		},
	}
	cmdutil.AddYesFlag(cmd)
	return cmd
}

func newIssueLabelCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "label <owner/repo> <list|create|add|remove> [args]",
		Short: "Manage issue labels",
		Long: `Manage labels using the bash-compatible argument order:

  issue label <owner/repo> list [--scope=org|repo]
      List labels. Scope defaults to org; any scope other than repo uses the org endpoint.

  issue label <owner/repo> create --name=TEXT [--color=HEX] [--desc=TEXT] [--scope=org|repo]
      Create a label. Color defaults to #0075ca; a leading # is accepted.

  issue label <owner/repo> add <number> --labels=X,Y
      Add labels to an issue. Label names are resolved by checking org labels before repo labels.

  issue label <owner/repo> remove <number> --label=TEXT
      Remove a single label from an issue. Requires --yes or an interactive typed confirmation.`,
		Args: cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 2 {
				return cmdutil.Usagef("Usage: forgejo issue label <owner/repo> <action> [args]")
			}
			repo, err := ctx.RepoArg(args[0])
			if err != nil {
				return err
			}
			switch args[1] {
			case "list":
				if len(args) != 2 {
					return cmdutil.Usagef("Usage: forgejo issue label <owner/repo> list [--scope=org|repo]")
				}
				return labelList(ctx, cmd, repo)
			case "create":
				if len(args) != 2 {
					return cmdutil.Usagef("Usage: forgejo issue label <owner/repo> create --name=X [--color=X] [--desc=X] [--scope=org|repo]")
				}
				return labelCreate(ctx, cmd, repo)
			case "add":
				if len(args) != 3 {
					return cmdutil.Usagef("Usage: forgejo issue label <owner/repo> add <number> --labels=X,Y")
				}
				return labelAdd(ctx, cmd, repo, args[2])
			case "remove":
				if len(args) != 3 {
					return cmdutil.Usagef("Usage: forgejo issue label <owner/repo> remove <number> --label=X")
				}
				return labelRemove(ctx, cmd, repo, args[2])
			default:
				return cmdutil.Usagef("Unknown label action: %s", args[1])
			}
		},
	}
	cmd.Flags().String("scope", "org", "label scope for list/create (org or repo; default org)")
	cmd.Flags().String("name", "", "label name for create")
	cmd.Flags().String("color", "#0075ca", "label color hex for create; leading # is accepted")
	cmd.Flags().String("desc", "", "label description for create")
	cmd.Flags().String("labels", "", "comma-separated label names for add")
	cmd.Flags().String("label", "", "single label name for remove")
	cmdutil.AddYesFlag(cmd)
	return cmd
}

func newIssueCloseCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "close <owner/repo> <number>",
		Short: "Close an issue",
		Long:  "Close an issue by PATCHing its state to closed. The repository argument is required; pass . to infer it from the current git remote.",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return setIssueState(ctx, args, "closed", "Closed")
		},
	}
	return cmd
}

func newIssueReopenCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "reopen <owner/repo> <number>",
		Short: "Reopen an issue",
		Long:  "Reopen a closed issue by PATCHing its state to open. The repository argument is required; pass . to infer it from the current git remote.",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return setIssueState(ctx, args, "open", "Reopened")
		},
	}
	return cmd
}

func newIssueAssignCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "assign <owner/repo> <number> --users=u1,u2",
		Short: "Add assignees to an issue",
		Long:  "Add assignees to an issue, unioned with the current assignees. The command GETs the issue first, computes the new set, and PATCHes the complete assignee list back even if unchanged.",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			repo, err := ctx.RepoArg(args[0])
			if err != nil {
				return err
			}
			number, err := cmdutil.IDArg(args[1], "issue number")
			if err != nil {
				return err
			}
			if !cmd.Flags().Changed("users") {
				return cmdutil.Usagef("Missing --users")
			}
			users, _ := cmd.Flags().GetString("users")
			return updateIssueAssignees(ctx, repo, number, "add", users)
		},
	}
	cmd.Flags().String("users", "", "comma-separated usernames to add")
	return cmd
}

func newIssueUnassignCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "unassign <owner/repo> <number> --users=u1,u2",
		Short: "Remove assignees from an issue",
		Long:  "Remove assignees from an issue. The command GETs the issue first, computes the new set, and PATCHes the complete assignee list back even if unchanged.",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			repo, err := ctx.RepoArg(args[0])
			if err != nil {
				return err
			}
			number, err := cmdutil.IDArg(args[1], "issue number")
			if err != nil {
				return err
			}
			if !cmd.Flags().Changed("users") {
				return cmdutil.Usagef("Missing --users")
			}
			users, _ := cmd.Flags().GetString("users")
			return updateIssueAssignees(ctx, repo, number, "remove", users)
		},
	}
	cmd.Flags().String("users", "", "comma-separated usernames to remove")
	return cmd
}

func newIssueSearchCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "search [--owner=ORG] [--state=open|closed|all] [--labels=a,b] [--query=TEXT] [--limit=N]",
		Short: "Search issues across repositories",
		Long:  "Search issues across repositories. The search is issue-only (type=issues). State defaults to open. --owner, --labels, and --query are optional filters. --limit is a local server-side query parameter and must be non-negative when provided.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			owner, _ := cmd.Flags().GetString("owner")
			state, _ := cmd.Flags().GetString("state")
			labels, _ := cmd.Flags().GetString("labels")
			query, _ := cmd.Flags().GetString("query")
			limit, _ := cmd.Flags().GetInt("limit")
			if cmd.Flags().Changed("limit") && limit < 0 {
				return cmdutil.Usagef("--limit must be a non-negative integer")
			}

			qs := []string{"type=issues", "state=" + cmdutil.QueryEscape(state)}
			if limit >= 0 {
				qs = append(qs, fmt.Sprintf("limit=%d", limit))
			}
			if owner != "" {
				qs = append(qs, "owner="+cmdutil.QueryEscape(owner))
			}
			if labels != "" {
				qs = append(qs, "labels="+cmdutil.QueryEscape(labels))
			}
			if query != "" {
				qs = append(qs, "q="+cmdutil.QueryEscape(query))
			}
			raw, err := ctx.Client.Do("GET", "repos/issues/search?"+strings.Join(qs, "&"), nil)
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
				rows = append(rows, []string{
					orDash(cmdutil.Str(m, "repository.full_name")),
					cmdutil.Str(m, "number"),
					cmdutil.Trunc(cmdutil.Str(m, "title"), 50),
					cmdutil.Str(m, "state"),
					cmdutil.Str(m, "user.login"),
					firstN(cmdutil.Str(m, "updated_at"), 10),
				})
			}
			ctx.Table([]string{"REPO", "#", "TITLE", "STATE", "AUTHOR", "UPDATED"}, rows)
			return nil
		},
	}
	cmd.Flags().String("owner", "", "restrict to an owner/org")
	cmd.Flags().String("state", "open", "issue state filter (open, closed, or all)")
	cmd.Flags().String("labels", "", "comma-separated label filter")
	cmd.Flags().String("query", "", "full-text search query")
	cmd.Flags().Int("limit", -1, "server-side result limit")
	return cmd
}

func newIssueMilestoneCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "milestone <list|create|edit|delete|set>",
		Short: "Manage milestones and set issue milestones",
		Long: `Milestone commands:
  forgejo issue milestone list <owner/repo> [--state=open|closed|all]
      List milestones in a repository.

  forgejo issue milestone create <owner/repo> --title=TEXT [--description=TEXT] [--due=YYYY-MM-DD]
      Create a new milestone.

  forgejo issue milestone edit <owner/repo> <id> [--title=TEXT] [--description=TEXT] [--due=YYYY-MM-DD] [--state=open|closed]
      Edit a milestone. <id> can be the numeric milestone ID.

  forgejo issue milestone delete <owner/repo> <id>
      Delete a milestone by its numeric ID. Requires --yes or an interactive typed confirmation.

  forgejo issue milestone set <owner/repo> <number> --milestone=<id|title>
      Set a milestone on an issue. Pass --milestone=0 to clear the milestone.
      <id|title> accepts either the numeric ID or exact title of the milestone.`,
	}
	cmd.AddCommand(
		newMilestoneListCmd(ctx),
		newMilestoneCreateCmd(ctx),
		newMilestoneEditCmd(ctx),
		newMilestoneDeleteCmd(ctx),
		newMilestoneSetCmd(ctx),
	)
	return cmd
}

func newMilestoneListCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list <owner/repo> [--state=open|closed|all]",
		Short: "List milestones",
		Long:  "List milestones in a repository. State defaults to open and is passed through to the server.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			repo, err := ctx.RepoArg(args[0])
			if err != nil {
				return err
			}
			state, _ := cmd.Flags().GetString("state")
			endpoint := "repos/" + repoAPIPath(repo) + "/milestones?state=" + cmdutil.QueryEscape(state)
			n := ctx.ListLimit(50)
			lr, err := ctx.Client.DoList(endpoint, n)
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
				fmt.Fprintln(ctx.Out, "No milestones found.")
				return nil
			}
			rows := make([][]string, 0, len(items))
			for _, m := range items {
				due := firstN(cmdutil.Str(m, "due_on"), 10)
				if due == "" {
					due = "-"
				}
				rows = append(rows, []string{
					cmdutil.Str(m, "id"),
					cmdutil.Trunc(cmdutil.Str(m, "title"), 40),
					cmdutil.Str(m, "state"),
					due,
					cmdutil.Str(m, "open_issues"),
					cmdutil.Str(m, "closed_issues"),
				})
			}
			ctx.Table([]string{"ID", "TITLE", "STATE", "DUE", "OPEN", "CLOSED"}, rows)
			ctx.Trailer(len(items), lr.Total, n)
			return nil
		},
	}
	cmd.Flags().String("state", "open", "milestone state filter (open, closed, or all)")
	return cmd
}

func newMilestoneCreateCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create <owner/repo> --title=TEXT [--description=TEXT] [--due=YYYY-MM-DD]",
		Short: "Create a milestone",
		Long:  "Create a new milestone. --title is required. --description defaults to empty. --due, when provided, must be YYYY-MM-DD and is sent as midnight UTC.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			repo, err := ctx.RepoArg(args[0])
			if err != nil {
				return err
			}
			if !cmd.Flags().Changed("title") {
				return cmdutil.Usagef("Missing --title")
			}
			title, _ := cmd.Flags().GetString("title")
			desc, _ := cmd.Flags().GetString("description")
			due, _ := cmd.Flags().GetString("due")
			fields := map[string]any{"title": title, "description": desc}
			if due != "" {
				if !milestoneDueRe.MatchString(due) {
					return cmdutil.Usagef("--due must be YYYY-MM-DD")
				}
				fields["due_on"] = due + "T00:00:00Z"
			}
			req, err := cmdutil.BuildBody(fields)
			if err != nil {
				return err
			}
			raw, err := ctx.Client.Do("POST", "repos/"+repoAPIPath(repo)+"/milestones", req)
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
			fmt.Fprintf(ctx.Out, "Created milestone #%s: %s\n", cmdutil.Str(obj, "id"), cmdutil.Str(obj, "title"))
			return nil
		},
	}
	cmd.Flags().String("title", "", "milestone title (required)")
	cmd.Flags().String("description", "", "milestone description")
	cmd.Flags().String("due", "", "due date in YYYY-MM-DD")
	return cmd
}

func newMilestoneEditCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "edit <owner/repo> <id> [--title=TEXT] [--description=TEXT] [--due=YYYY-MM-DD] [--state=open|closed]",
		Short: "Edit a milestone",
		Long:  "Edit a milestone by numeric ID. At least one of --title, --description, --due, or --state is required. --due must be YYYY-MM-DD and is sent as midnight UTC.",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			repo, err := ctx.RepoArg(args[0])
			if err != nil {
				return err
			}
			id, err := cmdutil.IDArg(args[1], "milestone id")
			if err != nil {
				return err
			}
			fields := map[string]any{}
			if cmd.Flags().Changed("title") {
				v, _ := cmd.Flags().GetString("title")
				fields["title"] = v
			}
			if cmd.Flags().Changed("description") {
				v, _ := cmd.Flags().GetString("description")
				fields["description"] = v
			}
			if cmd.Flags().Changed("state") {
				v, _ := cmd.Flags().GetString("state")
				fields["state"] = v
			}
			if cmd.Flags().Changed("due") {
				v, _ := cmd.Flags().GetString("due")
				if !milestoneDueRe.MatchString(v) {
					return cmdutil.Usagef("--due must be YYYY-MM-DD")
				}
				fields["due_on"] = v + "T00:00:00Z"
			}
			if len(fields) == 0 {
				return cmdutil.Usagef("No fields to update. Provide --title, --description, --due, or --state.")
			}
			req, err := cmdutil.BuildBody(fields)
			if err != nil {
				return err
			}
			raw, err := ctx.Client.Do("PATCH", "repos/"+repoAPIPath(repo)+"/milestones/"+id, req)
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
			fmt.Fprintf(ctx.Out, "Updated milestone #%s: %s (state=%s)\n",
				cmdutil.Str(obj, "id"), cmdutil.Str(obj, "title"), cmdutil.Str(obj, "state"))
			return nil
		},
	}
	cmd.Flags().String("title", "", "new milestone title")
	cmd.Flags().String("description", "", "new milestone description")
	cmd.Flags().String("due", "", "new due date in YYYY-MM-DD")
	cmd.Flags().String("state", "", "new milestone state (open or closed)")
	return cmd
}

func newMilestoneDeleteCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete <owner/repo> <id>",
		Short: "Delete a milestone",
		Long:  "Delete a milestone by numeric ID. Requires --yes or an interactive typed confirmation.",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			repo, err := ctx.RepoArg(args[0])
			if err != nil {
				return err
			}
			id, err := cmdutil.IDArg(args[1], "milestone id")
			if err != nil {
				return err
			}
			if err := ctx.ConfirmDelete(cmd, "milestone", "#"+id); err != nil {
				return err
			}
			if _, err := ctx.Client.Do("DELETE", "repos/"+repoAPIPath(repo)+"/milestones/"+id, nil); err != nil {
				return err
			}
			fmt.Fprintf(ctx.Out, "Deleted milestone #%s\n", id)
			return nil
		},
	}
	cmdutil.AddYesFlag(cmd)
	return cmd
}

func newMilestoneSetCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "set <owner/repo> <number> --milestone=<id|title>",
		Short: "Set an issue milestone",
		Long:  "Set a milestone on an issue. Pass --milestone=0 to clear the milestone. Otherwise --milestone accepts either a numeric milestone ID or the exact title of a milestone in the repository.",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			repo, err := ctx.RepoArg(args[0])
			if err != nil {
				return err
			}
			number, err := cmdutil.IDArg(args[1], "issue number")
			if err != nil {
				return err
			}
			if !cmd.Flags().Changed("milestone") {
				return cmdutil.Usagef("Missing --milestone")
			}
			rawMilestone, _ := cmd.Flags().GetString("milestone")
			var id json.Number
			if rawMilestone == "0" {
				id = json.Number("0")
			} else {
				resolved, err := resolveMilestoneID(ctx, repo, rawMilestone)
				if err != nil {
					return err
				}
				id = json.Number(resolved)
			}
			req, err := cmdutil.BuildBody(map[string]any{"milestone": id})
			if err != nil {
				return err
			}
			result, err := ctx.Client.Do("PATCH", "repos/"+repoAPIPath(repo)+"/issues/"+number, req)
			if err != nil {
				return err
			}
			if ctx.WantsJSON() {
				return ctx.EmitJSON(result)
			}
			obj, err := cmdutil.ParseObject(result)
			if err != nil {
				return err
			}
			title := cmdutil.Str(obj, "milestone.title")
			if title == "" {
				title = "(none)"
			}
			fmt.Fprintf(ctx.Out, "Set milestone on issue #%s to: %s\n", number, title)
			return nil
		},
	}
	cmd.Flags().String("milestone", "", "milestone numeric ID or exact title; 0 clears")
	return cmd
}

func newIssueImagesCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "images <owner/repo> <number> [--output=DIR]",
		Short: "Download issue image attachments",
		Long:  "Download all image attachments from an issue body and its comments. DIR defaults to ./issue-<number>-images/. Non-image attachments (extension not in .png .jpg .jpeg .gif .webp .svg .bmp) are skipped. JSON output prints the filtered attachment objects instead of downloading.",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			repo, err := ctx.RepoArg(args[0])
			if err != nil {
				return err
			}
			number, err := cmdutil.IDArg(args[1], "issue number")
			if err != nil {
				return err
			}
			outdir, _ := cmd.Flags().GetString("output")
			if outdir == "" {
				outdir = "issue-" + number + "-images"
			}

			images, err := collectIssueImages(ctx, repo, number)
			if err != nil {
				return err
			}
			rawImages, err := json.Marshal(images)
			if err != nil {
				return err
			}
			if ctx.WantsJSON() {
				return ctx.EmitJSON(rawImages)
			}
			if len(images) == 0 {
				fmt.Fprintln(ctx.Out, "No image attachments found.")
				return nil
			}
			if err := os.MkdirAll(outdir, 0o755); err != nil {
				return err
			}
			outAbs, err := filepath.Abs(outdir)
			if err != nil {
				return err
			}

			seenNames := map[string]bool{}
			failed := false
			for _, raw := range images {
				att, err := decodeAttachment(raw)
				if err != nil {
					return err
				}
				name := filepath.Base(att.Name)
				if name == "." || name == ".." || name == "" {
					return cmdutil.Usagef("unsafe attachment filename: %q", att.Name)
				}
				destName := name
				if seenNames[name] {
					destName = firstN(att.UUID, 8) + "-" + name
				}
				seenNames[name] = true
				dest := filepath.Join(outdir, destName)
				if err := ensureInsideDir(outAbs, dest); err != nil {
					return err
				}
				if err := downloadIssueAttachment(ctx, att.URL, dest); err != nil {
					_ = os.Remove(dest)
					fmt.Fprintf(ctx.Err, "forgejo: failed to download %s from %s\n", name, att.URL)
					failed = true
					continue
				}
				fmt.Fprintf(ctx.Out, "%s (%s bytes) -> %s\n", name, att.Size, dest)
			}
			if failed {
				return fmt.Errorf("one or more image downloads failed")
			}
			return nil
		},
	}
	cmd.Flags().String("output", "", "output directory (default ./issue-<number>-images)")
	return cmd
}

func labelList(ctx *cmdutil.Ctx, cmd *cobra.Command, repo string) error {
	scope, _ := cmd.Flags().GetString("scope")
	org := repoOwner(repo)
	endpoint := "orgs/" + cmdutil.PathEscape(org) + "/labels"
	if scope == "repo" {
		endpoint = "repos/" + repoAPIPath(repo) + "/labels"
	}
	n := ctx.ListLimit(50)
	lr, err := ctx.Client.DoList(endpoint, n)
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
		fmt.Fprintln(ctx.Out, "No labels found.")
		return nil
	}
	rows := make([][]string, 0, len(items))
	for _, m := range items {
		desc := cmdutil.Str(m, "description")
		if desc == "" {
			desc = "-"
		}
		rows = append(rows, []string{
			cmdutil.Str(m, "id"),
			cmdutil.Str(m, "name"),
			cmdutil.Str(m, "color"),
			cmdutil.Trunc(desc, 40),
		})
	}
	ctx.Table([]string{"ID", "NAME", "COLOR", "DESCRIPTION"}, rows)
	ctx.Trailer(len(items), lr.Total, n)
	return nil
}

func labelCreate(ctx *cmdutil.Ctx, cmd *cobra.Command, repo string) error {
	if !cmd.Flags().Changed("name") {
		return cmdutil.Usagef("Missing --name")
	}
	name, _ := cmd.Flags().GetString("name")
	color, _ := cmd.Flags().GetString("color")
	desc, _ := cmd.Flags().GetString("desc")
	scope, _ := cmd.Flags().GetString("scope")
	color = "#" + strings.TrimPrefix(color, "#")

	req, err := cmdutil.BuildBody(map[string]any{
		"name":        name,
		"color":       color,
		"description": desc,
	})
	if err != nil {
		return err
	}
	org := repoOwner(repo)
	endpoint := "orgs/" + cmdutil.PathEscape(org) + "/labels"
	if scope == "repo" {
		endpoint = "repos/" + repoAPIPath(repo) + "/labels"
	}
	raw, err := ctx.Client.Do("POST", endpoint, req)
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
	fmt.Fprintf(ctx.Out, "Created label: %s (id=%s, scope=%s)\n",
		cmdutil.Str(obj, "name"), cmdutil.Str(obj, "id"), scope)
	return nil
}

func labelAdd(ctx *cmdutil.Ctx, cmd *cobra.Command, repo, numberArg string) error {
	number, err := cmdutil.IDArg(numberArg, "issue number")
	if err != nil {
		return err
	}
	if !cmd.Flags().Changed("labels") {
		return cmdutil.Usagef("Missing --labels")
	}
	labels, _ := cmd.Flags().GetString("labels")
	ids, err := resolveLabelIDs(ctx, repo, labels)
	if err != nil {
		return err
	}
	req, err := cmdutil.BuildBody(map[string]any{"labels": ids})
	if err != nil {
		return err
	}
	raw, err := ctx.Client.Do("POST", "repos/"+repoAPIPath(repo)+"/issues/"+number+"/labels", req)
	if err != nil {
		return err
	}
	if ctx.WantsJSON() {
		return ctx.EmitJSON(raw)
	}
	fmt.Fprintf(ctx.Out, "Added labels to issue #%s: %s\n", number, labels)
	return nil
}

func labelRemove(ctx *cmdutil.Ctx, cmd *cobra.Command, repo, numberArg string) error {
	number, err := cmdutil.IDArg(numberArg, "issue number")
	if err != nil {
		return err
	}
	if !cmd.Flags().Changed("label") {
		return cmdutil.Usagef("Missing --label")
	}
	labelName, _ := cmd.Flags().GetString("label")
	id, err := resolveLabelID(ctx, repo, labelName)
	if err != nil {
		return err
	}
	if err := ctx.ConfirmDelete(cmd, "label", labelName); err != nil {
		return err
	}
	if _, err := ctx.Client.Do("DELETE", "repos/"+repoAPIPath(repo)+"/issues/"+number+"/labels/"+id, nil); err != nil {
		return err
	}
	fmt.Fprintf(ctx.Out, "Removed label from issue #%s: %s\n", number, labelName)
	return nil
}

func setIssueState(ctx *cmdutil.Ctx, args []string, state, verb string) error {
	repo, err := ctx.RepoArg(args[0])
	if err != nil {
		return err
	}
	number, err := cmdutil.IDArg(args[1], "issue number")
	if err != nil {
		return err
	}
	req, err := cmdutil.BuildBody(map[string]any{"state": state})
	if err != nil {
		return err
	}
	raw, err := ctx.Client.Do("PATCH", "repos/"+repoAPIPath(repo)+"/issues/"+number, req)
	if err != nil {
		return err
	}
	if ctx.WantsJSON() {
		return ctx.EmitJSON(raw)
	}
	fmt.Fprintf(ctx.Out, "%s issue #%s\n", verb, number)
	return nil
}

func resolveLabelIDs(ctx *cmdutil.Ctx, repo, csv string) ([]json.Number, error) {
	var ids []json.Number
	for _, name := range splitIssueCSV(csv, true) {
		id, err := resolveLabelID(ctx, repo, name)
		if err != nil {
			return nil, err
		}
		ids = append(ids, json.Number(id))
	}
	if ids == nil {
		return []json.Number{}, nil
	}
	return ids, nil
}

func resolveLabelID(ctx *cmdutil.Ctx, repo, name string) (string, error) {
	org := repoOwner(repo)
	orgRaw, err := ctx.Client.Do("GET", "orgs/"+cmdutil.PathEscape(org)+"/labels?limit=50", nil)
	if err == nil && len(orgRaw) != 0 {
		if id := findLabelID(orgRaw, name); id != "" {
			return id, nil
		}
	}
	repoRaw, err := ctx.Client.Do("GET", "repos/"+repoAPIPath(repo)+"/labels?limit=50", nil)
	if err != nil {
		return "", err
	}
	if id := findLabelID(repoRaw, name); id != "" {
		return id, nil
	}
	return "", cmdutil.Usagef("Label not found: %s (in org %s or repo %s)", name, org, repo)
}

func findLabelID(raw []byte, name string) string {
	items, err := cmdutil.ParseArray(raw)
	if err != nil {
		return ""
	}
	for _, m := range items {
		if cmdutil.Str(m, "name") == name {
			id := cmdutil.Str(m, "id")
			if valid, err := cmdutil.IDArg(id, "label id"); err == nil {
				return valid
			}
		}
	}
	return ""
}

func resolveMilestoneID(ctx *cmdutil.Ctx, repo, input string) (string, error) {
	if id, err := cmdutil.IDArg(input, "milestone id"); err == nil {
		return id, nil
	}
	raw, err := ctx.Client.Do("GET", "repos/"+repoAPIPath(repo)+"/milestones?state=all&limit=50", nil)
	if err != nil {
		return "", err
	}
	items, err := cmdutil.ParseArray(raw)
	if err != nil {
		return "", err
	}
	for _, m := range items {
		if cmdutil.Str(m, "title") == input {
			id := cmdutil.Str(m, "id")
			if valid, err := cmdutil.IDArg(id, "milestone id"); err == nil {
				return valid, nil
			}
		}
	}
	return "", cmdutil.Usagef("Milestone not found: %s (in %s)", input, repo)
}

func mergeIssueEditAssignees(current []string, addCSV, removeCSV string, addChanged, removeChanged bool) ([]string, error) {
	add := splitIssueCSV(addCSV, true)
	remove := splitIssueCSV(removeCSV, true)
	if addChanged && len(add) == 0 {
		return nil, cmdutil.Usagef("No users provided")
	}
	if removeChanged && len(remove) == 0 {
		return nil, cmdutil.Usagef("No users provided")
	}

	seen := map[string]bool{}
	next := make([]string, 0, len(current)+len(add))
	for _, u := range current {
		if u == "" || seen[u] {
			continue
		}
		seen[u] = true
		next = append(next, u)
	}
	for _, u := range add {
		if u == "" || seen[u] {
			continue
		}
		seen[u] = true
		next = append(next, u)
	}
	if len(remove) != 0 {
		removeSet := map[string]bool{}
		for _, u := range remove {
			removeSet[u] = true
		}
		kept := next[:0]
		for _, u := range next {
			if !removeSet[u] {
				kept = append(kept, u)
			}
		}
		next = kept
	}
	if next == nil {
		return []string{}, nil
	}
	return next, nil
}

func updateIssueAssignees(ctx *cmdutil.Ctx, repo, number, op, usersCSV string) error {
	if usersCSV == "" {
		return cmdutil.Usagef("No users provided")
	}
	users := splitIssueCSV(usersCSV, false)
	raw, err := ctx.Client.Do("GET", "repos/"+repoAPIPath(repo)+"/issues/"+number, nil)
	if err != nil {
		return err
	}
	obj, err := cmdutil.ParseObject(raw)
	if err != nil {
		return err
	}
	current := issueAssignees(obj)
	var next []string
	if op == "add" {
		seen := map[string]bool{}
		for _, u := range current {
			seen[u] = true
		}
		for _, u := range users {
			seen[u] = true
		}
		for u := range seen {
			next = append(next, u)
		}
		sort.Strings(next)
	} else {
		remove := map[string]bool{}
		for _, u := range users {
			remove[u] = true
		}
		for _, u := range current {
			if !remove[u] {
				next = append(next, u)
			}
		}
	}
	req, err := cmdutil.BuildBody(map[string]any{"assignees": next})
	if err != nil {
		return err
	}
	result, err := ctx.Client.Do("PATCH", "repos/"+repoAPIPath(repo)+"/issues/"+number, req)
	if err != nil {
		return err
	}
	if ctx.WantsJSON() {
		return ctx.EmitJSON(result)
	}
	obj, err = cmdutil.ParseObject(result)
	if err != nil {
		return err
	}
	final := strings.Join(issueAssignees(obj), ", ")
	if final == "" {
		final = "(none)"
	}
	verb := "Assigned"
	if op != "add" {
		verb = "Unassigned"
	}
	fmt.Fprintf(ctx.Out, "%s %s on issue #%s. Current assignees: %s\n", verb, usersCSV, number, final)
	return nil
}

func collectIssueImages(ctx *cmdutil.Ctx, repo, number string) ([]json.RawMessage, error) {
	var assets []json.RawMessage
	issueAssets, err := ctx.Client.DoPaged("repos/" + repoAPIPath(repo) + "/issues/" + number + "/assets")
	if err != nil {
		return nil, err
	}
	parsed, err := parseRawArray(issueAssets)
	if err != nil {
		return nil, err
	}
	assets = append(assets, parsed...)

	commentsRaw, err := ctx.Client.DoPaged("repos/" + repoAPIPath(repo) + "/issues/" + number + "/comments")
	if err != nil {
		return nil, err
	}
	comments, err := parseRawArray(commentsRaw)
	if err != nil {
		return nil, err
	}
	for _, c := range comments {
		var probe struct {
			ID json.Number `json:"id"`
		}
		if err := json.Unmarshal(c, &probe); err != nil || probe.ID == "" {
			continue
		}
		commentAssets, err := ctx.Client.DoPaged("repos/" + repoAPIPath(repo) + "/issues/comments/" + probe.ID.String() + "/assets")
		if err != nil {
			return nil, err
		}
		parsed, err := parseRawArray(commentAssets)
		if err != nil {
			return nil, err
		}
		assets = append(assets, parsed...)
	}

	var images []json.RawMessage
	for _, raw := range dedupeSortRawByID(assets) {
		att, err := decodeAttachment(raw)
		if err != nil {
			return nil, err
		}
		if isImageAttachment(att.Name) {
			images = append(images, raw)
		}
	}
	if images == nil {
		return []json.RawMessage{}, nil
	}
	return images, nil
}

type issueAttachment struct {
	ID   json.Number
	Name string
	URL  string
	UUID string
	Size string
}

func decodeAttachment(raw json.RawMessage) (*issueAttachment, error) {
	dec := json.NewDecoder(strings.NewReader(string(raw)))
	dec.UseNumber()
	var obj map[string]any
	if err := dec.Decode(&obj); err != nil {
		return nil, err
	}
	return &issueAttachment{
		ID:   json.Number(scalarString(obj["id"])),
		Name: scalarString(obj["name"]),
		URL:  scalarString(obj["browser_download_url"]),
		UUID: scalarString(obj["uuid"]),
		Size: scalarString(obj["size"]),
	}, nil
}

func downloadIssueAttachment(ctx *cmdutil.Ctx, rawURL, dest string) error {
	if strings.TrimSpace(rawURL) == "" {
		return fmt.Errorf("empty attachment download URL")
	}
	baseURL := ""
	if ctx.Config != nil {
		baseURL = ctx.Config.URL
	}
	if baseURL == "" && ctx.Client != nil {
		baseURL = ctx.Client.BaseURL
	}
	base, err := url.Parse(baseURL)
	if err != nil {
		return err
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return err
	}
	if !u.IsAbs() {
		u = base.ResolveReference(u)
	}

	baseClient := http.DefaultClient
	if ctx.Client != nil && ctx.Client.HTTP != nil {
		baseClient = ctx.Client.HTTP
	}
	httpClient := *baseClient
	previous := httpClient.CheckRedirect
	httpClient.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if previous != nil {
			if err := previous(req, via); err != nil {
				return err
			}
		}
		if sameOrigin(req.URL, base) && ctx.Client != nil && ctx.Client.Token != "" {
			req.Header.Set("Authorization", "token "+ctx.Client.Token)
		} else {
			req.Header.Del("Authorization")
		}
		return nil
	}

	req, err := http.NewRequest("GET", u.String(), nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "*/*")
	if ctx.Client != nil && ctx.Client.Token != "" && sameOrigin(u, base) {
		req.Header.Set("Authorization", "token "+ctx.Client.Token)
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("download status %d", resp.StatusCode)
	}
	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, resp.Body)
	return err
}

func parseRawArray(raw []byte) ([]json.RawMessage, error) {
	var items []json.RawMessage
	if err := json.Unmarshal(raw, &items); err != nil {
		return nil, fmt.Errorf("unexpected response shape: %w", err)
	}
	if items == nil {
		return []json.RawMessage{}, nil
	}
	return items, nil
}

func dedupeSortRawByID(items []json.RawMessage) []json.RawMessage {
	type keyed struct {
		raw json.RawMessage
		key string
		num float64
	}
	seen := map[string]bool{}
	var out []keyed
	for _, raw := range items {
		var probe struct {
			ID json.Number `json:"id"`
		}
		key := string(raw)
		num := 0.0
		if err := json.Unmarshal(raw, &probe); err == nil && probe.ID != "" {
			key = probe.ID.String()
			if f, err := probe.ID.Float64(); err == nil {
				num = f
			}
		}
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, keyed{raw: raw, key: key, num: num})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].num < out[j].num })
	raw := make([]json.RawMessage, 0, len(out))
	for _, item := range out {
		raw = append(raw, item.raw)
	}
	return raw
}

func ensureInsideDir(outAbs, dest string) error {
	destAbs, err := filepath.Abs(dest)
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(outAbs, destAbs)
	if err != nil {
		return err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) || filepath.IsAbs(rel) {
		return cmdutil.Usagef("unsafe attachment destination outside output dir: %s", dest)
	}
	return nil
}

func issueLabelNames(obj map[string]any) string {
	raw, _ := obj["labels"].([]any)
	var names []string
	for _, it := range raw {
		if m, ok := it.(map[string]any); ok {
			if name := scalarString(m["name"]); name != "" {
				names = append(names, name)
			}
		}
	}
	if len(names) == 0 {
		return "-"
	}
	return strings.Join(names, ", ")
}

func issueBody(obj map[string]any) string {
	v, ok := obj["body"]
	if !ok || v == nil {
		return "(no body)"
	}
	return scalarString(v)
}

func issueAssignees(obj map[string]any) []string {
	raw, _ := obj["assignees"].([]any)
	var names []string
	for _, it := range raw {
		if m, ok := it.(map[string]any); ok {
			names = append(names, scalarString(m["login"]))
		}
	}
	return names
}

func splitIssueCSV(s string, dropEmpty bool) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if dropEmpty && p == "" {
			continue
		}
		out = append(out, p)
	}
	return out
}

func isImageAttachment(name string) bool {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".png", ".jpg", ".jpeg", ".gif", ".webp", ".svg", ".bmp":
		return true
	default:
		return false
	}
}

func repoOwner(repo string) string {
	owner, _, _ := strings.Cut(repo, "/")
	return owner
}

func firstN(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}

func orDash(s string) string {
	if strings.TrimSpace(s) == "" {
		return "-"
	}
	return s
}
