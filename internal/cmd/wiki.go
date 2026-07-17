package cmd

// Ported from the bash cmd_wiki family (forgejo:4964-5267). Wiki PATCH has
// two server quirks preserved here: title is always sent, and rename-only
// edits re-fetch and re-send content_base64 so the server does not blank it.

import (
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/ourdatateam/forgejo-cli/internal/cmdutil"
	"github.com/spf13/cobra"
)

func init() { Register(newWikiCmd) }

func newWikiCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "wiki <list|view|create|edit|delete|revisions>",
		Short: "Manage repository wiki pages",
		Long: `Manage repository wiki pages.

Content sources for create/edit are, in precedence order:
  --content=TEXT    inline content
  --file=PATH       read content from a file
  --file=-          read content from stdin

There is no implicit stdin; piping content requires --file=-. Page names may contain spaces and are escaped as one path segment. wiki edit requires --title and/or a content source; --message sets the commit message and omitted uses the server default.`,
	}
	cmd.AddCommand(
		newWikiListCmd(ctx),
		newWikiViewCmd(ctx),
		newWikiCreateCmd(ctx),
		newWikiEditCmd(ctx),
		newWikiDeleteCmd(ctx),
		newWikiRevisionsCmd(ctx),
	)
	return cmd
}

func newWikiListCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list <owner/repo>",
		Short: "List wiki pages",
		Long:  "List wiki pages for a repository. The bash version fetched repos/{repo}/wiki/pages?limit=50; --limit overrides that default and --limit=0 fetches all pages.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			repo, err := ctx.RepoArg(args[0])
			if err != nil {
				return err
			}
			n := ctx.ListLimit(50)
			lr, err := ctx.Client.DoList("repos/"+repo+"/wiki/pages", n)
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
				fmt.Fprintln(ctx.Out, "No wiki pages found.")
				return nil
			}
			rows := make([][]string, 0, len(items))
			for _, m := range items {
				rows = append(rows, []string{
					cmdutil.Str(m, "title"),
					dateOnly(cmdutil.Str(m, "last_commit.author.date")),
					dash(cmdutil.Str(m, "last_commit.author.name")),
				})
			}
			ctx.Table([]string{"TITLE", "UPDATED", "AUTHOR"}, rows)
			ctx.Trailer(len(items), lr.Total, n)
			return nil
		},
	}
	return cmd
}

func newWikiViewCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "view <owner/repo> <page>",
		Short: "View a wiki page",
		Long:  "View a wiki page. Text output prints page metadata followed by decoded content_base64 verbatim; --json emits the raw server object.",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			repo, err := ctx.RepoArg(args[0])
			if err != nil {
				return err
			}
			raw, err := ctx.Client.Do("GET", "repos/"+repo+"/wiki/page/"+wikiPageSeg(args[1]), nil)
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
			fmt.Fprintf(ctx.Out, "Title:    %s\n", cmdutil.Str(obj, "title"))
			fmt.Fprintf(ctx.Out, "Updated:  %s\n", dash(cmdutil.Str(obj, "last_commit.author.date")))
			fmt.Fprintf(ctx.Out, "Author:   %s\n", dash(cmdutil.Str(obj, "last_commit.author.name")))
			fmt.Fprintf(ctx.Out, "Commit:   %s\n\n", dash(cmdutil.Str(obj, "last_commit.sha")))
			content, err := base64.StdEncoding.DecodeString(cmdutil.Str(obj, "content_base64"))
			if err != nil {
				return err
			}
			_, err = ctx.Out.Write(content)
			return err
		},
	}
	return cmd
}

func newWikiCreateCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create <owner/repo> --title=X [--content=X|--file=path|--file=-] [--message=X]",
		Short: "Create a wiki page",
		Long:  "Create a wiki page. --title is required. Content must be supplied with --content, --file=PATH, or --file=-; --content takes precedence over --file, and --content='' creates an empty page.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			repo, err := ctx.RepoArg(args[0])
			if err != nil {
				return err
			}
			title, _ := cmd.Flags().GetString("title")
			if title == "" {
				return cmdutil.Usagef("Missing --title")
			}
			content, ok, err := wikiContent(ctx, cmd)
			if err != nil {
				return err
			}
			if !ok {
				return cmdutil.Usagef("No content supplied. Use --content=X, --file=path, or --file=- to read stdin (use --content='' for an empty page).")
			}
			message, _ := cmd.Flags().GetString("message")
			fields := map[string]any{
				"title":          title,
				"content_base64": base64.StdEncoding.EncodeToString([]byte(content)),
			}
			if message != "" {
				fields["message"] = message
			}
			body, err := cmdutil.BuildBody(fields)
			if err != nil {
				return err
			}
			raw, err := ctx.Client.Do("POST", "repos/"+repo+"/wiki/new", body)
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
			fmt.Fprintf(ctx.Out, "Created wiki page: %s\n", cmdutil.Str(obj, "title"))
			return nil
		},
	}
	addWikiContentFlags(cmd)
	cmd.Flags().String("title", "", "wiki page title (required)")
	cmd.Flags().String("message", "", "commit message for the wiki change")
	return cmd
}

func newWikiEditCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "edit <owner/repo> <page> [--title=X] [--content=X|--file=path|--file=-] [--message=X]",
		Short: "Edit or rename a wiki page",
		Long:  "Edit or rename a wiki page. Supply --title and/or a content source. If --title is omitted, the title is sent as the current page name so Forgejo does not rename it to unnamed. If content is omitted, the current page is fetched and content_base64 is re-sent because Forgejo otherwise blanks the page on PATCH.",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			repo, err := ctx.RepoArg(args[0])
			if err != nil {
				return err
			}
			page := args[1]
			title, _ := cmd.Flags().GetString("title")
			content, haveContent, err := wikiContent(ctx, cmd)
			if err != nil {
				return err
			}
			if title == "" && !haveContent {
				return cmdutil.Usagef("Nothing to edit (supply --title and/or content via --content/--file).")
			}
			if title == "" {
				title = page
			}
			pageSeg := wikiPageSeg(page)
			var contentB64 string
			if haveContent {
				contentB64 = base64.StdEncoding.EncodeToString([]byte(content))
			} else {
				current, err := ctx.Client.Do("GET", "repos/"+repo+"/wiki/page/"+pageSeg, nil)
				if err != nil {
					return err
				}
				obj, err := cmdutil.ParseObject(current)
				if err != nil {
					return err
				}
				contentB64 = cmdutil.Str(obj, "content_base64")
			}
			message, _ := cmd.Flags().GetString("message")
			fields := map[string]any{
				"title":          title,
				"content_base64": contentB64,
			}
			if message != "" {
				fields["message"] = message
			}
			body, err := cmdutil.BuildBody(fields)
			if err != nil {
				return err
			}
			raw, err := ctx.Client.Do("PATCH", "repos/"+repo+"/wiki/page/"+pageSeg, body)
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
			fmt.Fprintf(ctx.Out, "Updated wiki page: %s\n", cmdutil.Str(obj, "title"))
			return nil
		},
	}
	addWikiContentFlags(cmd)
	cmd.Flags().String("title", "", "new wiki page title")
	cmd.Flags().String("message", "", "commit message for the wiki change")
	return cmd
}

func newWikiDeleteCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete <owner/repo> <page> [--yes]",
		Short: "Delete a wiki page",
		Long:  "Delete a wiki page. This is destructive and requires --yes or a typed page-title confirmation.",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			repo, err := ctx.RepoArg(args[0])
			if err != nil {
				return err
			}
			page := args[1]
			if err := ctx.ConfirmDelete(cmd, "wiki page", page); err != nil {
				return err
			}
			if _, err := ctx.Client.Do("DELETE", "repos/"+repo+"/wiki/page/"+wikiPageSeg(page), nil); err != nil {
				return err
			}
			fmt.Fprintf(ctx.Out, "Deleted wiki page: %s\n", page)
			return nil
		},
	}
	cmdutil.AddYesFlag(cmd)
	return cmd
}

func newWikiRevisionsCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "revisions <owner/repo> <page>",
		Short: "Show a wiki page's commit history",
		Long:  "Show a wiki page's commit history. --json emits the raw server object; text output renders the commits array.",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			repo, err := ctx.RepoArg(args[0])
			if err != nil {
				return err
			}
			raw, err := ctx.Client.Do("GET", "repos/"+repo+"/wiki/revisions/"+wikiPageSeg(args[1]), nil)
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
			commits := objectArray(obj["commits"])
			if len(commits) == 0 {
				fmt.Fprintln(ctx.Out, "No revisions found.")
				return nil
			}
			rows := make([][]string, 0, len(commits))
			for _, m := range commits {
				rows = append(rows, []string{
					cmdutil.Trunc(cmdutil.Str(m, "sha"), 10),
					dateOnly(cmdutil.Str(m, "author.date")),
					dash(cmdutil.Str(m, "author.name")),
					cmdutil.Trunc(strings.ReplaceAll(cmdutil.Str(m, "message"), "\n", " "), 50),
				})
			}
			ctx.Table([]string{"SHA", "DATE", "AUTHOR", "MESSAGE"}, rows)
			return nil
		},
	}
	return cmd
}

func addWikiContentFlags(cmd *cobra.Command) {
	cmd.Flags().String("content", "", "inline wiki content; takes precedence over --file and may be empty")
	cmd.Flags().String("file", "", "read wiki content from a file; '-' reads stdin")
}

func wikiContent(ctx *cmdutil.Ctx, cmd *cobra.Command) (string, bool, error) {
	if cmd.Flags().Changed("content") {
		content, _ := cmd.Flags().GetString("content")
		return content, true, nil
	}
	file, _ := cmd.Flags().GetString("file")
	if file == "" {
		return "", false, nil
	}
	if file == "-" {
		data, err := io.ReadAll(ctx.In)
		if err != nil {
			return "", false, err
		}
		return string(data), true, nil
	}
	st, err := os.Stat(file)
	if err != nil || !st.Mode().IsRegular() {
		return "", false, cmdutil.Usagef("Not a regular file: %s", file)
	}
	data, err := os.ReadFile(file)
	if err != nil {
		return "", false, err
	}
	return string(data), true, nil
}

func wikiPageSeg(page string) string {
	return cmdutil.PathEscape(page)
}

func objectArray(v any) []map[string]any {
	arr, _ := v.([]any)
	out := make([]map[string]any, 0, len(arr))
	for _, it := range arr {
		if m, ok := it.(map[string]any); ok {
			out = append(out, m)
		}
	}
	return out
}
