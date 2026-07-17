package cmd

// Ported from the bash cmd_branch family (forgejo:5451-5669). branch create
// and protection list are Go-port additions from the group brief. Protection
// updates keep the bash GET-first idempotency contract.

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/ourdatateam/forgejo-cli/internal/api"
	"github.com/ourdatateam/forgejo-cli/internal/cmdutil"
	"github.com/spf13/cobra"
)

func init() { Register(newBranchCmd) }

func newBranchCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "branch <list|view|create|delete|protect|unprotect|protection>",
		Short: "Manage branches and branch protection",
	}
	cmd.AddCommand(
		newBranchListCmd(ctx),
		newBranchViewCmd(ctx),
		newBranchCreateCmd(ctx),
		newBranchDeleteCmd(ctx),
		newBranchProtectCmd(ctx),
		newBranchUnprotectCmd(ctx),
		newBranchProtectionCmd(ctx),
	)
	return cmd
}

func newBranchListCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list <owner/repo>",
		Short: "List branches",
		Long:  "List branches in a repository. This matches the bash endpoint repos/{repo}/branches without adding pagination parameters.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			repo, err := ctx.RepoArg(args[0])
			if err != nil {
				return err
			}
			raw, err := ctx.Client.Do("GET", "repos/"+repo+"/branches", nil)
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
					yesNo(cmdutil.Str(m, "protected")),
					cmdutil.Trunc(cmdutil.Str(m, "commit.id"), 7),
					dateOnly(cmdutil.Str(m, "commit.timestamp")),
				})
			}
			ctx.Table([]string{"NAME", "PROTECTED", "LAST_COMMIT", "DATE"}, rows)
			return nil
		},
	}
	return cmd
}

func newBranchViewCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "view <owner/repo> <branch>",
		Short: "View branch and protection detail",
		Long:  "View branch details plus branch protection. Protection is checked with DoStatus; 404 renders as not protected instead of an error.",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			repo, err := ctx.RepoArg(args[0])
			if err != nil {
				return err
			}
			branch := args[1]
			branchSeg := cmdutil.NameSeg(branch)
			branchData, err := ctx.Client.Do("GET", "repos/"+repo+"/branches/"+branchSeg, nil)
			if err != nil {
				return err
			}
			status, protBody, err := ctx.Client.DoStatus("GET", "repos/"+repo+"/branch_protections/"+branchSeg, nil)
			if err != nil {
				return err
			}
			if ctx.WantsJSON() {
				switch status {
				case http.StatusOK:
				case http.StatusNotFound:
					protBody = []byte("null")
				default:
					return apiErrorFromStatus(status, protBody)
				}
				raw, err := json.Marshal(struct {
					Branch     json.RawMessage `json:"branch"`
					Protection json.RawMessage `json:"protection"`
				}{Branch: branchData, Protection: protBody})
				if err != nil {
					return err
				}
				return ctx.EmitJSON(raw)
			}
			obj, err := cmdutil.ParseObject(branchData)
			if err != nil {
				return err
			}
			fmt.Fprintf(ctx.Out, "Branch:   %s\n", cmdutil.Str(obj, "name"))
			fmt.Fprintf(ctx.Out, "Commit:   %s\n", cmdutil.Str(obj, "commit.id"))
			fmt.Fprintf(ctx.Out, "Author:   %s (%s)\n", cmdutil.Str(obj, "commit.author.name"), cmdutil.Str(obj, "commit.author.email"))
			fmt.Fprintf(ctx.Out, "Date:     %s\n", branchDate(cmdutil.Str(obj, "commit.timestamp")))
			fmt.Fprintf(ctx.Out, "Message:  %s\n\n", firstLine(cmdutil.Str(obj, "commit.message")))
			switch status {
			case http.StatusOK:
				return printProtectionDetail(ctx, protBody)
			case http.StatusNotFound:
				fmt.Fprintln(ctx.Out, "Protection: not protected")
				return nil
			default:
				return apiErrorFromStatus(status, protBody)
			}
		},
	}
	return cmd
}

func newBranchCreateCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create <owner/repo> <branch> [--from=X]",
		Short: "Create a branch",
		Long:  "Create a branch with POST repos/{repo}/branches. --from sets old_branch_name; omitted lets the server use the repository default branch.",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			repo, err := ctx.RepoArg(args[0])
			if err != nil {
				return err
			}
			branch := args[1]
			if branch == "" {
				return cmdutil.Usagef("branch create: branch name is required")
			}
			from, _ := cmd.Flags().GetString("from")
			fields := map[string]any{"new_branch_name": branch}
			if from != "" {
				fields["old_branch_name"] = from
			}
			body, err := cmdutil.BuildBody(fields)
			if err != nil {
				return err
			}
			raw, err := ctx.Client.Do("POST", "repos/"+repo+"/branches", body)
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
			name := cmdutil.Str(obj, "name")
			if name == "" {
				name = branch
			}
			if from == "" {
				fmt.Fprintf(ctx.Out, "Created branch %s\n", name)
			} else {
				fmt.Fprintf(ctx.Out, "Created branch %s from %s\n", name, from)
			}
			return nil
		},
	}
	cmd.Flags().String("from", "", "source branch or commit-ish (old_branch_name)")
	return cmd
}

func newBranchDeleteCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete <owner/repo> <branch>",
		Short: "Delete a branch",
		Long:  "Delete a branch. Deliberately no --yes prompt: bash kept branch deletion scriptable because branches are recoverable and protected branches are rejected by the server.",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			repo, err := ctx.RepoArg(args[0])
			if err != nil {
				return err
			}
			branch := args[1]
			if _, err := ctx.Client.Do("DELETE", "repos/"+repo+"/branches/"+cmdutil.NameSeg(branch), nil); err != nil {
				return err
			}
			fmt.Fprintf(ctx.Out, "Deleted branch %s\n", branch)
			return nil
		},
	}
	return cmd
}

func newBranchProtectCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "protect <owner/repo> <branch> [flags]",
		Short: "Apply or update branch protection",
		Long:  "Apply or update branch protection idempotently. The command first GETs repos/{repo}/branch_protections/{branch}; 404 creates a protection with POST, otherwise PATCH updates only the fields requested. With no flags on create, the safe default body locks pushes and requires zero approvals. --no-push and --push-whitelist are mutually exclusive.",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			repo, err := ctx.RepoArg(args[0])
			if err != nil {
				return err
			}
			branch := args[1]
			bodyMap, err := branchProtectionBody(cmd)
			if err != nil {
				return err
			}
			body, err := cmdutil.BuildBody(bodyMap)
			if err != nil {
				return err
			}
			branchSeg := cmdutil.NameSeg(branch)
			status, checkBody, err := ctx.Client.DoStatus("GET", "repos/"+repo+"/branch_protections/"+branchSeg, nil)
			if err != nil {
				return err
			}
			var raw []byte
			switch status {
			case http.StatusOK:
				raw, err = ctx.Client.Do("PATCH", "repos/"+repo+"/branch_protections/"+branchSeg, body)
			case http.StatusNotFound:
				if len(bodyMap) == 0 {
					bodyMap["enable_push"] = false
					bodyMap["required_approvals"] = 0
				}
				bodyMap["branch_name"] = branch
				body, err = cmdutil.BuildBody(bodyMap)
				if err != nil {
					return err
				}
				raw, err = ctx.Client.Do("POST", "repos/"+repo+"/branch_protections", body)
			default:
				return apiErrorFromStatus(status, checkBody)
			}
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
			pushState := "disabled"
			if cmdutil.Str(obj, "enable_push") == "true" {
				pushState = "enabled"
			}
			mw := joinStringField(obj, "merge_whitelist_usernames", ",")
			if mw == "" {
				mw = "-"
			}
			fmt.Fprintf(ctx.Out, "Protected %s on %s (push: %s, merge whitelist: %s, approvals: %s)\n",
				branch, repo, pushState, mw, intString(cmdutil.Str(obj, "required_approvals")))
			return nil
		},
	}
	cmd.Flags().Bool("no-push", false, "disable direct pushes and push whitelist")
	cmd.Flags().String("push-whitelist", "", "comma-separated users allowed to push; mutually exclusive with --no-push")
	cmd.Flags().String("merge-whitelist", "", "comma-separated users allowed to merge")
	cmd.Flags().Int("required-approvals", -1, "required approval count (non-negative)")
	cmd.Flags().Bool("dismiss-stale-approvals", false, "dismiss stale approvals when new commits are pushed")
	cmd.Flags().Bool("require-signed", false, "require signed commits")
	cmd.Flags().Bool("block-on-outdated", false, "block merging outdated branches")
	return cmd
}

func newBranchUnprotectCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "unprotect <owner/repo> <branch>",
		Short: "Remove branch protection",
		Long:  "Remove branch protection. The command is idempotent: 404 prints that the branch was not protected and exits successfully.",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			repo, err := ctx.RepoArg(args[0])
			if err != nil {
				return err
			}
			branch := args[1]
			status, body, err := ctx.Client.DoStatus("DELETE", "repos/"+repo+"/branch_protections/"+cmdutil.NameSeg(branch), nil)
			if err != nil {
				return err
			}
			switch status {
			case http.StatusNoContent, http.StatusOK:
				if ctx.WantsJSON() {
					return ctx.EmitJSON(body)
				}
				fmt.Fprintf(ctx.Out, "Removed protection on %s\n", branch)
				return nil
			case http.StatusNotFound:
				if ctx.WantsJSON() {
					return ctx.EmitJSON([]byte("null"))
				}
				fmt.Fprintf(ctx.Out, "Branch %s was not protected\n", branch)
				return nil
			default:
				return apiErrorFromStatus(status, body)
			}
		},
	}
	return cmd
}

func newBranchProtectionCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "protection <list>",
		Short: "Manage branch protection records",
	}
	cmd.AddCommand(newBranchProtectionListCmd(ctx))
	return cmd
}

func newBranchProtectionListCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list <owner/repo>",
		Short: "List branch protection records",
		Long:  "List branch protection records for a repository. This Go-port verb fetches repos/{repo}/branch_protections with a default list limit of 50; --limit=0 fetches all pages.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			repo, err := ctx.RepoArg(args[0])
			if err != nil {
				return err
			}
			n := ctx.ListLimit(50)
			lr, err := ctx.Client.DoList("repos/"+repo+"/branch_protections", n)
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
				fmt.Fprintln(ctx.Out, "No branch protections found.")
				return nil
			}
			rows := make([][]string, 0, len(items))
			for _, m := range items {
				mw := joinStringField(m, "merge_whitelist_usernames", ",")
				if mw == "" {
					mw = "-"
				}
				rows = append(rows, []string{
					cmdutil.Str(m, "branch_name"),
					yesNo(cmdutil.Str(m, "enable_push")),
					mw,
					intString(cmdutil.Str(m, "required_approvals")),
					yesNo(cmdutil.Str(m, "require_signed_commits")),
				})
			}
			ctx.Table([]string{"BRANCH", "PUSH", "MERGE_WHITELIST", "APPROVALS", "SIGNED"}, rows)
			ctx.Trailer(len(items), lr.Total, n)
			return nil
		},
	}
	return cmd
}

func branchProtectionBody(cmd *cobra.Command) (map[string]any, error) {
	noPush, _ := cmd.Flags().GetBool("no-push")
	pushWhitelist, _ := cmd.Flags().GetString("push-whitelist")
	if noPush && pushWhitelist != "" {
		return nil, cmdutil.Usagef("forgejo branch protect: --no-push and --push-whitelist are mutually exclusive")
	}
	body := map[string]any{}
	if noPush {
		body["enable_push"] = false
		body["enable_push_whitelist"] = false
	} else if pushWhitelist != "" {
		body["enable_push"] = true
		body["enable_push_whitelist"] = true
		body["push_whitelist_usernames"] = splitCommaKeepSpace(pushWhitelist)
	}
	mergeWhitelist, _ := cmd.Flags().GetString("merge-whitelist")
	if mergeWhitelist != "" {
		body["enable_merge_whitelist"] = true
		body["merge_whitelist_usernames"] = splitCommaKeepSpace(mergeWhitelist)
	}
	if cmd.Flags().Changed("required-approvals") {
		n, _ := cmd.Flags().GetInt("required-approvals")
		if n < 0 {
			return nil, cmdutil.Usagef("forgejo branch protect: --required-approvals must be a non-negative integer")
		}
		body["required_approvals"] = n
	}
	if v, _ := cmd.Flags().GetBool("dismiss-stale-approvals"); v {
		body["dismiss_stale_approvals"] = true
	}
	if v, _ := cmd.Flags().GetBool("require-signed"); v {
		body["require_signed_commits"] = true
	}
	if v, _ := cmd.Flags().GetBool("block-on-outdated"); v {
		body["block_on_outdated_branch"] = true
	}
	return body, nil
}

func printProtectionDetail(ctx *cmdutil.Ctx, raw []byte) error {
	obj, err := cmdutil.ParseObject(raw)
	if err != nil {
		return err
	}
	push := "disabled"
	if cmdutil.Str(obj, "enable_push") == "true" {
		push = "enabled"
	}
	mw := joinStringField(obj, "merge_whitelist_usernames", ", ")
	if mw == "" {
		mw = "-"
	}
	fmt.Fprintln(ctx.Out, "Protection: enabled")
	fmt.Fprintf(ctx.Out, "  Push:               %s\n", push)
	fmt.Fprintf(ctx.Out, "  Merge whitelist:    %s\n", mw)
	fmt.Fprintf(ctx.Out, "  Required approvals: %s\n", intString(cmdutil.Str(obj, "required_approvals")))
	fmt.Fprintf(ctx.Out, "  Dismiss stale:      %s\n", yesNo(cmdutil.Str(obj, "dismiss_stale_approvals")))
	fmt.Fprintf(ctx.Out, "  Require signed:     %s\n", yesNo(cmdutil.Str(obj, "require_signed_commits")))
	return nil
}

func apiErrorFromStatus(status int, body []byte) error {
	return &api.Error{Status: status, Message: extractAPIMessage(body)}
}

func splitCommaKeepSpace(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func joinStringField(m map[string]any, path, sep string) string {
	var cur any = m
	for _, part := range strings.Split(path, ".") {
		obj, ok := cur.(map[string]any)
		if !ok {
			return ""
		}
		cur = obj[part]
	}
	arr, ok := cur.([]any)
	if !ok {
		return cmdutil.Str(m, path)
	}
	out := make([]string, 0, len(arr))
	for _, it := range arr {
		s, ok := it.(string)
		if ok && s != "" {
			out = append(out, s)
		}
	}
	return strings.Join(out, sep)
}

func branchDate(s string) string {
	if len(s) >= 16 {
		return strings.Replace(s[:16], "T", " ", 1)
	}
	return strings.Replace(s, "T", " ", 1)
}
