package cmd

// Ported from the bash cmd_release family (forgejo:4632-4960).
// Asset download is a Go-port addition from the release group brief; uploads
// and downloads use custom HTTP because the shared client is JSON-only.

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	pathpkg "path"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/ourdatateam/forgejo-cli/internal/api"
	"github.com/ourdatateam/forgejo-cli/internal/cmdutil"
	"github.com/spf13/cobra"
)

func init() { Register(newReleaseCmd) }

func newReleaseCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "release <list|create|view|edit|delete|download|upload-asset|asset>",
		Short: "Manage releases and release assets",
	}
	cmd.AddCommand(
		newReleaseListCmd(ctx),
		newReleaseCreateCmd(ctx),
		newReleaseViewCmd(ctx),
		newReleaseEditCmd(ctx),
		newReleaseDeleteCmd(ctx),
		newReleaseDownloadCmd(ctx),
		newReleaseUploadAssetCmd(ctx),
		newReleaseAssetCmd(ctx),
	)
	return cmd
}

func newReleaseListCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list <owner/repo>",
		Short: "List releases",
		Long:  "List releases for a repository. The bash version fetched repos/{repo}/releases?limit=20; --limit overrides that default and --limit=0 fetches all pages.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			repo, err := ctx.RepoArg(args[0])
			if err != nil {
				return err
			}
			n := ctx.ListLimit(20)
			lr, err := ctx.Client.DoList("repos/"+repoAPIPath(repo)+"/releases", n)
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
				fmt.Fprintln(ctx.Out, "No releases found.")
				return nil
			}
			rows := make([][]string, 0, len(items))
			for _, m := range items {
				published := cmdutil.Str(m, "published_at")
				if published == "" {
					published = cmdutil.Str(m, "created_at")
				}
				rows = append(rows, []string{
					cmdutil.Str(m, "tag_name"),
					cmdutil.Trunc(cmdutil.Str(m, "name"), 40),
					yesNo(cmdutil.Str(m, "draft")),
					yesNo(cmdutil.Str(m, "prerelease")),
					dateOnly(published),
				})
			}
			ctx.Table([]string{"TAG", "TITLE", "DRAFT", "PRERELEASE", "PUBLISHED"}, rows)
			ctx.Trailer(len(items), lr.Total, n)
			return nil
		},
	}
	return cmd
}

func newReleaseCreateCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create <owner/repo> --tag=X --title=X [--body=X|--body-file=path] [--draft] [--prerelease]",
		Short: "Create a release",
		Long:  "Create a release. --tag and --title are required. --body supplies inline release notes; --body=- or --body-file=- reads notes from stdin.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			repo, err := ctx.RepoArg(args[0])
			if err != nil {
				return err
			}
			tag, _ := cmd.Flags().GetString("tag")
			title, _ := cmd.Flags().GetString("title")
			if tag == "" {
				return cmdutil.Usagef("Missing --tag")
			}
			if title == "" {
				return cmdutil.Usagef("Missing --title")
			}
			bodyText, _, err := ctx.Body(cmd)
			if err != nil {
				return err
			}
			draft, _ := cmd.Flags().GetBool("draft")
			prerelease, _ := cmd.Flags().GetBool("prerelease")
			body, err := cmdutil.BuildBody(map[string]any{
				"tag_name":   tag,
				"name":       title,
				"body":       bodyText,
				"draft":      draft,
				"prerelease": prerelease,
			})
			if err != nil {
				return err
			}
			raw, err := ctx.Client.Do("POST", "repos/"+repoAPIPath(repo)+"/releases", body)
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
			fmt.Fprintf(ctx.Out, "Created release: %s (%s)\nURL: %s\n",
				cmdutil.Str(obj, "name"), cmdutil.Str(obj, "tag_name"), dash(cmdutil.Str(obj, "html_url")))
			return nil
		},
	}
	cmd.Flags().String("tag", "", "release tag name (required)")
	cmd.Flags().String("title", "", "release title (required)")
	cmdutil.AddBodyFlags(cmd)
	cmd.Flags().Bool("draft", false, "create the release as a draft")
	cmd.Flags().Bool("prerelease", false, "mark the release as a prerelease")
	return cmd
}

func newReleaseViewCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "view <owner/repo> <tag>",
		Short: "View release detail",
		Long:  "View a release by tag. The tag is resolved to a release id with GET repos/{repo}/releases/tags/{tag}; if that 404s, releases are scanned as a fallback.",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			repo, err := ctx.RepoArg(args[0])
			if err != nil {
				return err
			}
			id, err := resolveReleaseID(ctx, repo, args[1])
			if err != nil {
				return err
			}
			raw, err := ctx.Client.Do("GET", "repos/"+repoAPIPath(repo)+"/releases/"+id, nil)
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
			published := cmdutil.Str(obj, "published_at")
			if published == "" {
				published = cmdutil.Str(obj, "created_at")
			}
			body := cmdutil.Str(obj, "body")
			if body == "" {
				body = "(no body)"
			}
			fmt.Fprintf(ctx.Out, "Tag:        %s\n", cmdutil.Str(obj, "tag_name"))
			fmt.Fprintf(ctx.Out, "Title:      %s\n", cmdutil.Str(obj, "name"))
			fmt.Fprintf(ctx.Out, "Draft:      %s\n", yesNo(cmdutil.Str(obj, "draft")))
			fmt.Fprintf(ctx.Out, "Prerelease: %s\n", yesNo(cmdutil.Str(obj, "prerelease")))
			fmt.Fprintf(ctx.Out, "Target:     %s\n", cmdutil.Str(obj, "target_commitish"))
			fmt.Fprintf(ctx.Out, "Published:  %s\n", published)
			fmt.Fprintf(ctx.Out, "Author:     %s\n", dash(cmdutil.Str(obj, "author.login")))
			fmt.Fprintf(ctx.Out, "URL:        %s\n\n", dash(cmdutil.Str(obj, "html_url")))
			fmt.Fprintln(ctx.Out, body)
			return nil
		},
	}
	return cmd
}

func newReleaseEditCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "edit <owner/repo> <tag> [--title=X] [--body=X|--body-file=path] [--draft|--no-draft] [--prerelease|--no-prerelease]",
		Short: "Edit a release",
		Long:  "Edit a release by tag. Supply at least one of --title, --body, --body-file, --draft/--no-draft, or --prerelease/--no-prerelease. --draft and --no-draft are mutually exclusive, as are --prerelease and --no-prerelease.",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			repo, err := ctx.RepoArg(args[0])
			if err != nil {
				return err
			}
			tag := args[1]
			title, _ := cmd.Flags().GetString("title")
			bodyText, haveBody, err := ctx.Body(cmd)
			if err != nil {
				return err
			}
			draftChanged := cmd.Flags().Changed("draft")
			noDraftChanged := cmd.Flags().Changed("no-draft")
			if draftChanged && noDraftChanged {
				return cmdutil.Usagef("Provide --draft OR --no-draft, not both")
			}
			prereleaseChanged := cmd.Flags().Changed("prerelease")
			noPrereleaseChanged := cmd.Flags().Changed("no-prerelease")
			if prereleaseChanged && noPrereleaseChanged {
				return cmdutil.Usagef("Provide --prerelease OR --no-prerelease, not both")
			}
			// Bash only sends body when the resolved text is non-empty.
			sendBody := haveBody && bodyText != ""
			if title == "" && !sendBody && !draftChanged && !noDraftChanged && !prereleaseChanged && !noPrereleaseChanged {
				return cmdutil.Usagef("Nothing to edit (supply --title, --body, --draft/--no-draft, or --prerelease/--no-prerelease)")
			}
			id, err := resolveReleaseID(ctx, repo, tag)
			if err != nil {
				return err
			}
			fields := map[string]any{}
			if title != "" {
				fields["name"] = title
			}
			if sendBody {
				fields["body"] = bodyText
			}
			if draftChanged || noDraftChanged {
				fields["draft"] = draftChanged
			}
			if prereleaseChanged || noPrereleaseChanged {
				fields["prerelease"] = prereleaseChanged
			}
			body, err := cmdutil.BuildBody(fields)
			if err != nil {
				return err
			}
			raw, err := ctx.Client.Do("PATCH", "repos/"+repoAPIPath(repo)+"/releases/"+id, body)
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
			fmt.Fprintf(ctx.Out, "Updated release: %s (%s)\nURL: %s\n",
				cmdutil.Str(obj, "name"), cmdutil.Str(obj, "tag_name"), dash(cmdutil.Str(obj, "html_url")))
			return nil
		},
	}
	cmd.Flags().String("title", "", "new release title")
	cmdutil.AddBodyFlags(cmd)
	cmd.Flags().Bool("draft", false, "mark the release as a draft")
	cmd.Flags().Bool("no-draft", false, "mark the release as non-draft")
	cmd.Flags().Bool("prerelease", false, "mark the release as a prerelease")
	cmd.Flags().Bool("no-prerelease", false, "mark the release as a normal release")
	return cmd
}

func newReleaseDeleteCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete <owner/repo> <tag> [--yes]",
		Short: "Delete a release",
		Long:  "Delete a release by tag. This is destructive and requires --yes or a typed tag confirmation.",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			repo, err := ctx.RepoArg(args[0])
			if err != nil {
				return err
			}
			tag := args[1]
			if err := ctx.ConfirmDelete(cmd, "release", tag); err != nil {
				return err
			}
			if _, err := ctx.Client.Do("DELETE", "repos/"+repoAPIPath(repo)+"/releases/tags/"+cmdutil.NameSeg(tag), nil); err != nil {
				return err
			}
			fmt.Fprintf(ctx.Out, "Deleted release: %s\n", tag)
			return nil
		},
	}
	cmdutil.AddYesFlag(cmd)
	return cmd
}

func newReleaseDownloadCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "download <owner/repo> <tag> [--pattern=GLOB] [--output=DIR]",
		Short: "Download release assets",
		Long:  "Download assets attached to a release by tag. --pattern filters asset names with path.Match; --output is a directory (default: .).",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			repo, err := ctx.RepoArg(args[0])
			if err != nil {
				return err
			}
			tag := args[1]
			pattern, _ := cmd.Flags().GetString("pattern")
			outdir, _ := cmd.Flags().GetString("output")
			id, err := resolveReleaseID(ctx, repo, tag)
			if err != nil {
				return err
			}
			raw, err := ctx.Client.Do("GET", "repos/"+repoAPIPath(repo)+"/releases/"+id+"/assets", nil)
			if err != nil {
				return err
			}
			assets, err := parseReleaseAssets(raw)
			if err != nil {
				return err
			}
			selected, err := filterReleaseAssets(assets, pattern)
			if err != nil {
				return err
			}
			if pattern != "" && len(selected) == 0 {
				return fmt.Errorf("no release assets match pattern %q; available assets: %s", pattern, availableReleaseAssetNames(assets))
			}

			downloaded := make([]json.RawMessage, 0, len(selected))
			for _, asset := range selected {
				fallback := releaseAssetFallback(asset.obj)
				name := safeBasename(asset.name, fallback)
				downloadURL := cmdutil.Str(asset.obj, "browser_download_url")
				if downloadURL == "" {
					return fmt.Errorf("asset %q has no browser_download_url", name)
				}
				dest, err := containedPath(outdir, name)
				if err != nil {
					return err
				}
				if releaseDryRun(ctx) {
					if err := downloadReleaseAssetDryRun(ctx, downloadURL); err != nil {
						return err
					}
					if !ctx.WantsJSON() {
						fmt.Fprintf(ctx.Out, "would write %s\n", dest)
					}
				} else {
					if err := downloadReleaseAsset(ctx, downloadURL, dest); err != nil {
						return err
					}
					if !ctx.WantsJSON() {
						fmt.Fprintf(ctx.Out, "wrote %s\n", dest)
					}
				}
				downloaded = append(downloaded, asset.raw)
			}
			if ctx.WantsJSON() {
				raw, err := json.Marshal(downloaded)
				if err != nil {
					return err
				}
				return ctx.EmitJSON(raw)
			}
			if releaseDryRun(ctx) {
				fmt.Fprintf(ctx.Out, "would write %d %s\n", len(selected), fileWord(len(selected)))
			} else {
				fmt.Fprintf(ctx.Out, "wrote %d %s\n", len(selected), fileWord(len(selected)))
			}
			return nil
		},
	}
	cmd.Flags().String("pattern", "", "glob pattern for asset names")
	cmd.Flags().String("output", ".", "directory to write downloaded assets into")
	return cmd
}

func newReleaseUploadAssetCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "upload-asset <owner/repo> <tag> <file>...",
		Short: "Upload release assets",
		Long:  "Upload one or more asset files to a release. Every path is validated as a regular file before the first upload, so a typo does not leave a partial upload set.",
		Args:  cobra.MinimumNArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			repo, err := ctx.RepoArg(args[0])
			if err != nil {
				return err
			}
			tag := args[1]
			files := args[2:]
			for _, f := range files {
				st, err := os.Stat(f)
				if err != nil || !st.Mode().IsRegular() {
					return cmdutil.Usagef("Not a regular file: %s", f)
				}
			}
			id, err := resolveReleaseID(ctx, repo, tag)
			if err != nil {
				return err
			}
			var raws []json.RawMessage
			var uploaded []string
			for _, f := range files {
				raw, err := uploadReleaseAsset(ctx, repo, id, f)
				if err != nil {
					if len(uploaded) > 0 {
						return fmt.Errorf("%w\nFiles uploaded before failure: %s", err, strings.Join(uploaded, " "))
					}
					return err
				}
				if ctx.WantsJSON() {
					raws = append(raws, json.RawMessage(raw))
				} else {
					obj, err := cmdutil.ParseObject(raw)
					if err != nil {
						return err
					}
					fmt.Fprintf(ctx.Out, "Uploaded: %s (id=%s)\n", filepath.Base(f), cmdutil.Str(obj, "id"))
				}
				uploaded = append(uploaded, filepath.Base(f))
			}
			if ctx.WantsJSON() {
				if len(raws) == 1 {
					return ctx.EmitJSON(raws[0])
				}
				raw, err := json.Marshal(raws)
				if err != nil {
					return err
				}
				return ctx.EmitJSON(raw)
			}
			return nil
		},
	}
	return cmd
}

func newReleaseAssetCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "asset <list|download|delete|upload>",
		Short: "Manage release assets",
	}
	cmd.AddCommand(newReleaseAssetListCmd(ctx), newReleaseAssetDownloadCmd(ctx), newReleaseAssetDeleteCmd(ctx), newReleaseAssetUploadCmd(ctx))
	return cmd
}

func newReleaseAssetListCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list <owner/repo> <tag>",
		Short: "List release assets",
		Long:  "List assets attached to a release. The tag is resolved to a release id before fetching repos/{repo}/releases/{id}/assets.",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			repo, err := ctx.RepoArg(args[0])
			if err != nil {
				return err
			}
			id, err := resolveReleaseID(ctx, repo, args[1])
			if err != nil {
				return err
			}
			raw, err := ctx.Client.Do("GET", "repos/"+repoAPIPath(repo)+"/releases/"+id+"/assets", nil)
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
				fmt.Fprintln(ctx.Out, "No assets found.")
				return nil
			}
			rows := make([][]string, 0, len(items))
			for _, m := range items {
				rows = append(rows, []string{
					cmdutil.Str(m, "id"),
					cmdutil.Str(m, "name"),
					cmdutil.Str(m, "size"),
					cmdutil.Str(m, "download_count"),
					dateOnly(cmdutil.Str(m, "created_at")),
				})
			}
			ctx.Table([]string{"ID", "NAME", "SIZE", "DOWNLOADS", "CREATED"}, rows)
			return nil
		},
	}
	return cmd
}

func newReleaseAssetDownloadCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "download <owner/repo> <tag> <asset_id> [--output=DIR]",
		Short: "Download a release asset",
		Long:  "Download a release asset by id. --output is a directory (default: .); the remote filename is reduced to a safe basename and the destination is kept inside that directory. The token is attached to the asset URL only when its origin matches FORGEJO_URL.",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			repo, err := ctx.RepoArg(args[0])
			if err != nil {
				return err
			}
			assetID, err := cmdutil.IDArg(args[2], "asset id")
			if err != nil {
				return err
			}
			outdir, _ := cmd.Flags().GetString("output")
			id, err := resolveReleaseID(ctx, repo, args[1])
			if err != nil {
				return err
			}
			raw, err := ctx.Client.Do("GET", "repos/"+repoAPIPath(repo)+"/releases/"+id+"/assets/"+assetID, nil)
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
			name := safeBasename(cmdutil.Str(obj, "name"), "asset-"+assetID)
			downloadURL := cmdutil.Str(obj, "browser_download_url")
			if downloadURL == "" {
				return fmt.Errorf("asset #%s has no browser_download_url", assetID)
			}
			dest, err := containedPath(outdir, name)
			if err != nil {
				return err
			}
			if err := downloadReleaseAsset(ctx, downloadURL, dest); err != nil {
				return err
			}
			fmt.Fprintf(ctx.Out, "%s (%s bytes) -> %s\n", name, dash(cmdutil.Str(obj, "size")), dest)
			return nil
		},
	}
	cmd.Flags().String("output", ".", "directory to write the downloaded asset into")
	return cmd
}

func newReleaseAssetDeleteCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete <owner/repo> <tag> <asset_id> [--yes]",
		Short: "Delete a release asset",
		Long:  "Delete a release asset by numeric id. This is destructive and requires --yes or typed confirmation of #<asset_id>.",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			repo, err := ctx.RepoArg(args[0])
			if err != nil {
				return err
			}
			assetID, err := cmdutil.IDArg(args[2], "asset id")
			if err != nil {
				return err
			}
			if err := ctx.ConfirmDelete(cmd, "asset", "#"+assetID); err != nil {
				return err
			}
			id, err := resolveReleaseID(ctx, repo, args[1])
			if err != nil {
				return err
			}
			if _, err := ctx.Client.Do("DELETE", "repos/"+repoAPIPath(repo)+"/releases/"+id+"/assets/"+assetID, nil); err != nil {
				return err
			}
			fmt.Fprintf(ctx.Out, "Deleted asset #%s\n", assetID)
			return nil
		},
	}
	cmdutil.AddYesFlag(cmd)
	return cmd
}

func newReleaseAssetUploadCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "upload <owner/repo> <tag> --file=PATH [--name=NAME]",
		Short: "Upload a release asset",
		Long:  "Upload one asset file to a release. --file is required; --name defaults to the file basename.",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			repo, err := ctx.RepoArg(args[0])
			if err != nil {
				return err
			}
			file, _ := cmd.Flags().GetString("file")
			if file == "" {
				return cmdutil.Usagef("Missing --file")
			}
			if err := validateReleaseAssetFile(file); err != nil {
				return err
			}
			name, _ := cmd.Flags().GetString("name")
			if name == "" {
				name = filepath.Base(file)
			}
			id, err := resolveReleaseID(ctx, repo, args[1])
			if err != nil {
				return err
			}
			raw, err := uploadReleaseAssetWithName(ctx, repo, id, file, name, false)
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
			fmt.Fprintf(ctx.Out, "uploaded %s (id %s)\n", name, cmdutil.Str(obj, "id"))
			return nil
		},
	}
	cmd.Flags().String("file", "", "file to upload")
	cmd.Flags().String("name", "", "asset name (default: file basename)")
	return cmd
}

func resolveReleaseID(ctx *cmdutil.Ctx, repo, tag string) (string, error) {
	repoPath := repoAPIPath(repo)
	raw, err := ctx.Client.Do("GET", "repos/"+repoPath+"/releases/tags/"+cmdutil.NameSeg(tag), nil)
	if err == nil {
		obj, err := cmdutil.ParseObject(raw)
		if err != nil {
			return "", err
		}
		if id := cmdutil.Str(obj, "id"); id != "" {
			return id, nil
		}
		return "", fmt.Errorf("release %q has no id", tag)
	}
	var apiErr *api.Error
	if !errors.As(err, &apiErr) || apiErr.Status != http.StatusNotFound {
		return "", err
	}
	raw, err = ctx.Client.DoPaged("repos/" + repoPath + "/releases")
	if err != nil {
		return "", err
	}
	items, err := cmdutil.ParseArray(raw)
	if err != nil {
		return "", err
	}
	for _, m := range items {
		if cmdutil.Str(m, "tag_name") == tag {
			if id := cmdutil.Str(m, "id"); id != "" {
				return id, nil
			}
			return "", fmt.Errorf("release %q has no id", tag)
		}
	}
	return "", fmt.Errorf("release not found for tag %q", tag)
}

type releaseAsset struct {
	raw  json.RawMessage
	obj  map[string]any
	name string
}

func parseReleaseAssets(raw []byte) ([]releaseAsset, error) {
	var raws []json.RawMessage
	if err := json.Unmarshal(raw, &raws); err != nil {
		return nil, fmt.Errorf("unexpected response shape: %w", err)
	}
	assets := make([]releaseAsset, 0, len(raws))
	for _, item := range raws {
		obj, err := cmdutil.ParseObject(item)
		if err != nil {
			return nil, err
		}
		assets = append(assets, releaseAsset{
			raw:  item,
			obj:  obj,
			name: cmdutil.Str(obj, "name"),
		})
	}
	return assets, nil
}

func filterReleaseAssets(assets []releaseAsset, pattern string) ([]releaseAsset, error) {
	if pattern == "" {
		return assets, nil
	}
	if _, err := pathpkg.Match(pattern, ""); err != nil {
		return nil, cmdutil.Usagef("invalid --pattern %q: %v", pattern, err)
	}
	selected := make([]releaseAsset, 0, len(assets))
	for _, asset := range assets {
		ok, err := pathpkg.Match(pattern, asset.name)
		if err != nil {
			return nil, cmdutil.Usagef("invalid --pattern %q: %v", pattern, err)
		}
		if ok {
			selected = append(selected, asset)
		}
	}
	return selected, nil
}

func availableReleaseAssetNames(assets []releaseAsset) string {
	if len(assets) == 0 {
		return "(none)"
	}
	names := make([]string, 0, len(assets))
	for _, asset := range assets {
		name := asset.name
		if name == "" {
			name = releaseAssetFallback(asset.obj)
		}
		names = append(names, name)
	}
	return strings.Join(names, ", ")
}

func releaseAssetFallback(obj map[string]any) string {
	if id := cmdutil.Str(obj, "id"); id != "" {
		return "asset-" + id
	}
	return "asset"
}

func releaseDryRun(ctx *cmdutil.Ctx) bool {
	return ctx.Client != nil && ctx.Client.DryRun
}

func validateReleaseAssetFile(path string) error {
	st, err := os.Stat(path)
	if err != nil {
		return err
	}
	if !st.Mode().IsRegular() {
		return fmt.Errorf("not a regular file: %s", path)
	}
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	_ = f.Close()
	return nil
}

func uploadReleaseAsset(ctx *cmdutil.Ctx, repo, releaseID, path string) ([]byte, error) {
	base := safeBasename(filepath.Base(path), "asset")
	return uploadReleaseAssetWithName(ctx, repo, releaseID, path, base, true)
}

func uploadReleaseAssetWithName(ctx *cmdutil.Ctx, repo, releaseID, path, name string, includeNameField bool) ([]byte, error) {
	if name == "" {
		name = safeBasename(filepath.Base(path), "asset")
	}
	endpoint := fmt.Sprintf("repos/%s/releases/%s/assets?name=%s", repoAPIPath(repo), releaseID, cmdutil.QueryEscape(name))
	fullURL := strings.TrimRight(ctx.Client.BaseURL, "/") + "/api/v1/" + endpoint
	if ctx.Client.DryRun {
		errw := ctx.Err
		if ctx.Client.Stderr != nil {
			errw = ctx.Client.Stderr
		}
		fmt.Fprintf(errw, "DRY-RUN: POST %s\nmultipart: name=%s attachment=%s\n", fullURL, name, path)
		return nil, api.ErrDryRun
	}

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	if includeNameField {
		if err := mw.WriteField("name", name); err != nil {
			return nil, err
		}
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	part, err := mw.CreateFormFile("attachment", name)
	if err != nil {
		return nil, err
	}
	if _, err := io.Copy(part, f); err != nil {
		return nil, err
	}
	if err := mw.Close(); err != nil {
		return nil, err
	}

	base, err := url.Parse(ctx.Client.BaseURL)
	if err != nil {
		return nil, err
	}
	target, err := url.Parse(fullURL)
	if err != nil {
		return nil, err
	}
	client := cloneHTTPClient(releaseHTTPClient(ctx))
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if !sameOrigin(req.URL, base) {
			req.Header.Del("Authorization")
		}
		return nil
	}
	req, err := http.NewRequest("POST", fullURL, &buf)
	if err != nil {
		return nil, err
	}
	if sameOrigin(target, base) && ctx.Client.Token != "" {
		req.Header.Set("Authorization", "token "+ctx.Client.Token)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", mw.FormDataContentType())
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("forgejo: network error: %w", err)
	}
	defer resp.Body.Close()
	out, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("forgejo: network error: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("upload failed for %s (HTTP %d): %s", path, resp.StatusCode, extractAPIMessage(out))
	}
	return out, nil
}

func downloadReleaseAsset(ctx *cmdutil.Ctx, rawURL, dest string) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	return readReleaseAsset(ctx, rawURL, dest, func(r io.Reader) error {
		tmp := dest + ".tmp"
		f, err := os.Create(tmp)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(f, r)
		closeErr := f.Close()
		if copyErr != nil {
			_ = os.Remove(tmp)
			return copyErr
		}
		if closeErr != nil {
			_ = os.Remove(tmp)
			return closeErr
		}
		return os.Rename(tmp, dest)
	})
}

func downloadReleaseAssetDryRun(ctx *cmdutil.Ctx, rawURL string) error {
	return readReleaseAsset(ctx, rawURL, "", func(r io.Reader) error {
		_, err := io.Copy(io.Discard, r)
		return err
	})
}

func readReleaseAsset(ctx *cmdutil.Ctx, rawURL, dest string, consume func(io.Reader) error) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return err
	}
	base, err := url.Parse(ctx.Client.BaseURL)
	if err != nil {
		return err
	}
	client := cloneHTTPClient(releaseHTTPClient(ctx))
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if !sameOrigin(req.URL, base) {
			req.Header.Del("Authorization")
		}
		return nil
	}
	req, err := http.NewRequest("GET", u.String(), nil)
	if err != nil {
		return err
	}
	if sameOrigin(u, base) {
		req.Header.Set("Authorization", "token "+ctx.Client.Token)
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("forgejo: network error: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		out, _ := io.ReadAll(resp.Body)
		if dest != "" {
			_ = os.Remove(dest)
		}
		return fmt.Errorf("download failed from %s (HTTP %d): %s", rawURL, resp.StatusCode, strings.TrimSpace(string(out)))
	}
	return consume(resp.Body)
}

func releaseHTTPClient(ctx *cmdutil.Ctx) *http.Client {
	if ctx.Client != nil && ctx.Client.HTTP != nil {
		return ctx.Client.HTTP
	}
	return http.DefaultClient
}

func cloneHTTPClient(c *http.Client) *http.Client {
	cp := *c
	return &cp
}

func safeBasename(name, fallback string) string {
	name = strings.ReplaceAll(name, "\\", "/")
	base := filepath.Base(name)
	if base == "." || base == ".." || base == string(filepath.Separator) || base == "" {
		return fallback
	}
	return base
}

func containedPath(dir, name string) (string, error) {
	if dir == "" {
		dir = "."
	}
	dest := filepath.Clean(filepath.Join(dir, safeBasename(name, "asset")))
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	absDest, err := filepath.Abs(dest)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(absDir, absDest)
	if err != nil {
		return "", err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return "", fmt.Errorf("refusing to write outside output directory")
	}
	return dest, nil
}

func extractAPIMessage(raw []byte) string {
	var parsed struct {
		Message string `json:"message"`
		Err     string `json:"error"`
	}
	if err := json.Unmarshal(raw, &parsed); err == nil {
		switch {
		case parsed.Message != "":
			return parsed.Message
		case parsed.Err != "":
			return parsed.Err
		}
	}
	return strings.TrimSpace(string(raw))
}

func dateOnly(s string) string {
	if len(s) >= 10 {
		return s[:10]
	}
	return s
}

func intString(v string) string {
	if v == "" {
		return "0"
	}
	if _, err := strconv.Atoi(v); err != nil {
		return v
	}
	return v
}

func fileWord(n int) string {
	if n == 1 {
		return "file"
	}
	return "files"
}
