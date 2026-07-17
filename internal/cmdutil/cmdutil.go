// Package cmdutil holds the shared plumbing every command group uses:
// output rendering (--json / --jq / tables), repo argument resolution with
// git-remote inference, body input (--body / --body-file / stdin), delete
// confirmation, and JSON navigation helpers.
package cmdutil

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"text/tabwriter"

	"github.com/itchyny/gojq"
	"github.com/ourdatateam/forgejo-cli/internal/api"
	"github.com/ourdatateam/forgejo-cli/internal/config"
	"github.com/spf13/cobra"
)

// Ctx carries per-invocation state into verb implementations.
type Ctx struct {
	Client *api.Client
	Config *config.Config
	JSON   bool   // --json: print server JSON (pretty, key order preserved)
	JQ     string // --jq: filter output through a jq expression (implies JSON source)
	Limit  int    // --limit override; -1 = unset (verb default), 0 = all pages
	Out    io.Writer
	Err    io.Writer
	In     io.Reader
}

// ExitError carries an explicit process exit code (e.g. `api` keeps 22).
type ExitError struct {
	Code int
	Err  error
}

func (e *ExitError) Error() string {
	if e.Err == nil {
		return fmt.Sprintf("exit %d", e.Code)
	}
	return e.Err.Error()
}

func (e *ExitError) Unwrap() error { return e.Err }

// UsageError marks bad invocations (exit 2).
type UsageError struct{ Msg string }

func (e *UsageError) Error() string { return e.Msg }

func Usagef(format string, a ...any) error {
	return &UsageError{Msg: fmt.Sprintf(format, a...)}
}

// ---------- output ----------

// EmitJSON matches the bash `| jq .` contract: pretty-printed with 2-space
// indent, key order and number fidelity preserved (json.Indent operates on
// the byte stream — the body is never decoded into Go maps). With --jq set,
// each gojq result prints on its own line instead.
func (c *Ctx) EmitJSON(raw []byte) error {
	if c.JQ != "" {
		return c.emitJQ(raw)
	}
	var buf bytes.Buffer
	if err := json.Indent(&buf, orNull(raw), "", "  "); err != nil {
		// Non-JSON bodies (raw diffs already routed elsewhere) pass through.
		_, werr := c.Out.Write(append(orNull(raw), '\n'))
		return werr
	}
	_, err := fmt.Fprintln(c.Out, buf.String())
	return err
}

func orNull(raw []byte) []byte {
	if raw == nil {
		return []byte("null")
	}
	return raw
}

func (c *Ctx) emitJQ(raw []byte) error {
	q, err := gojq.Parse(c.JQ)
	if err != nil {
		return Usagef("--jq: %v", err)
	}
	var input any
	dec := json.NewDecoder(strings.NewReader(string(orNull(raw))))
	dec.UseNumber()
	if err := dec.Decode(&input); err != nil {
		return fmt.Errorf("--jq: response is not JSON: %w", err)
	}
	iter := q.Run(normalize(input))
	enc := json.NewEncoder(c.Out)
	for {
		v, ok := iter.Next()
		if !ok {
			break
		}
		if err, isErr := v.(error); isErr {
			return fmt.Errorf("--jq: %w", err)
		}
		if s, isStr := v.(string); isStr {
			fmt.Fprintln(c.Out, s) // raw strings, like jq -r; agents want unquoted values
			continue
		}
		if err := enc.Encode(v); err != nil {
			return err
		}
	}
	return nil
}

// normalize converts json.Number into gojq-acceptable values.
func normalize(v any) any {
	switch t := v.(type) {
	case json.Number:
		if i, err := t.Int64(); err == nil {
			return int(i)
		}
		f, _ := t.Float64()
		return f
	case map[string]any:
		for k, e := range t {
			t[k] = normalize(e)
		}
		return t
	case []any:
		for i, e := range t {
			t[i] = normalize(e)
		}
		return t
	default:
		return v
	}
}

// WantsJSON reports whether output should be the raw JSON path.
func (c *Ctx) WantsJSON() bool { return c.JSON || c.JQ != "" }

// Table renders rows through a tabwriter. Headers are uppercase by
// convention. No colors, one line per item.
func (c *Ctx) Table(headers []string, rows [][]string) {
	w := tabwriter.NewWriter(c.Out, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, strings.Join(headers, "\t"))
	for _, r := range rows {
		for i, cell := range r {
			r[i] = strings.ReplaceAll(strings.ReplaceAll(cell, "\t", " "), "\n", " ")
		}
		fmt.Fprintln(w, strings.Join(r, "\t"))
	}
	w.Flush()
}

// ListLimit resolves the effective page size for a list verb. Each verb
// passes the limit the bash version hardcoded (its compatibility default);
// an explicit --limit overrides it (0 = fetch all pages). Internal joins
// (e.g. actions tasks?limit=200) must NOT route through this.
func (c *Ctx) ListLimit(verbDefault int) int {
	if c.Limit >= 0 {
		return c.Limit
	}
	return verbDefault
}

// Trailer prints the visible-truncation line for text-mode lists — on
// stderr, so stdout stays strictly one line per item for parsers.
// total == -1 means the server reported no X-Total-Count.
func (c *Ctx) Trailer(shown, total, limit int) {
	if c.WantsJSON() || limit <= 0 {
		return
	}
	if total >= 0 && total > shown {
		fmt.Fprintf(c.Err, "(showing %d of %d; use --limit=N or --limit=0 for all)\n", shown, total)
	} else if total < 0 && shown == limit {
		fmt.Fprintf(c.Err, "(showing first %d; use --limit=N or --limit=0 for all)\n", shown)
	}
}

// ---------- JSON navigation ----------

// ParseArray decodes a JSON array of objects, preserving number fidelity.
func ParseArray(raw []byte) ([]map[string]any, error) {
	dec := json.NewDecoder(strings.NewReader(string(raw)))
	dec.UseNumber()
	var items []map[string]any
	if err := dec.Decode(&items); err != nil {
		return nil, fmt.Errorf("unexpected response shape: %w", err)
	}
	return items, nil
}

// ParseObject decodes a JSON object, preserving number fidelity.
func ParseObject(raw []byte) (map[string]any, error) {
	dec := json.NewDecoder(strings.NewReader(string(raw)))
	dec.UseNumber()
	var obj map[string]any
	if err := dec.Decode(&obj); err != nil {
		return nil, fmt.Errorf("unexpected response shape: %w", err)
	}
	return obj, nil
}

// Str walks a dotted path ("user.login") and renders the value as a string.
// Missing paths and nulls render as "". Arrays render comma-joined scalars.
func Str(m map[string]any, path string) string {
	var cur any = m
	for _, part := range strings.Split(path, ".") {
		obj, ok := cur.(map[string]any)
		if !ok {
			return ""
		}
		cur = obj[part]
	}
	return renderScalar(cur)
}

func renderScalar(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case json.Number:
		return t.String()
	case bool:
		if t {
			return "true"
		}
		return "false"
	case []any:
		parts := make([]string, 0, len(t))
		for _, e := range t {
			parts = append(parts, renderScalar(e))
		}
		return strings.Join(parts, ",")
	case map[string]any:
		if name, ok := t["name"].(string); ok {
			return name
		}
		return "{…}"
	default:
		return fmt.Sprintf("%v", t)
	}
}

// Trunc shortens s to at most n runes (the bash tables truncate long
// titles/descriptions the same way).
func Trunc(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}

// ---------- repo resolution ----------

var (
	repoRe = regexp.MustCompile(`^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$`)
	idRe   = regexp.MustCompile(`^[0-9]+$`)
	nameRe = regexp.MustCompile(`^[A-Za-z0-9_.\- ]+$`)
)

// ValidRepo reports whether s looks like owner/repo. "." and ".." segments
// are rejected so user input can never pivot the request path.
func ValidRepo(s string) bool {
	if !repoRe.MatchString(s) {
		return false
	}
	owner, name, _ := strings.Cut(s, "/")
	return owner != "." && owner != ".." && name != "." && name != ".."
}

// IDArg validates a numeric path segment (issue/PR numbers, ids) before it
// is interpolated into an endpoint.
func IDArg(s, what string) (string, error) {
	if !idRe.MatchString(s) {
		return "", Usagef("invalid %s %q (expected a number)", what, s)
	}
	return s, nil
}

// NameSeg percent-encodes a free-form name (tag, label, branch, secret …)
// for safe use as a single path segment.
func NameSeg(s string) string { return PathEscape(s) }

// SafeName reports whether s is a conservative identifier (no path or query
// metacharacters); used for extra validation where the API is strict anyway.
func SafeName(s string) bool { return nameRe.MatchString(s) }

// RepoArg resolves a required repo positional. The arity is identical to
// the bash CLI (the repo slot is never optional — silent wrong-repo
// inference is how a headless agent mutates the wrong project). Passing
// "." explicitly opts into inference from the cwd's git remotes; only
// remotes whose host matches FORGEJO_URL are considered.
func (c *Ctx) RepoArg(arg string) (string, error) {
	if arg == "." {
		return InferRepo(c.Config.URL)
	}
	if !ValidRepo(arg) {
		return "", Usagef("invalid repo %q (expected owner/repo, or '.' to use the current directory's remote)", arg)
	}
	return arg, nil
}

// InferRepo finds owner/repo from the current directory's git remotes,
// considering only remotes on the configured Forgejo host ("origin" wins).
func InferRepo(baseURL string) (string, error) {
	base, err := url.Parse(baseURL)
	if err != nil {
		return "", fmt.Errorf("invalid FORGEJO_URL: %w", err)
	}
	out, err := exec.Command("git", "remote", "-v").Output()
	if err != nil {
		return "", Usagef("no repo argument and not in a git repository; pass owner/repo")
	}
	found := map[string]string{}
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		name, remote := fields[0], fields[1]
		if repo, ok := matchRemote(remote, base.Host); ok {
			found[name] = repo
		}
	}
	if repo, ok := found["origin"]; ok {
		return repo, nil
	}
	for _, repo := range found {
		return repo, nil
	}
	return "", Usagef("no repo argument and no git remote on %s; pass owner/repo", base.Host)
}

func matchRemote(remote, host string) (string, bool) {
	var path string
	switch {
	case strings.HasPrefix(remote, "http://"), strings.HasPrefix(remote, "https://"):
		u, err := url.Parse(remote)
		if err != nil || u.Host != host {
			return "", false
		}
		path = strings.TrimPrefix(u.Path, "/")
	case strings.Contains(remote, "@") && strings.Contains(remote, ":"):
		// scp-like: git@host:owner/repo.git
		rest := remote[strings.Index(remote, "@")+1:]
		h, p, ok := strings.Cut(rest, ":")
		if !ok || h != host {
			return "", false
		}
		path = p
	default:
		return "", false
	}
	path = strings.TrimSuffix(strings.TrimSuffix(path, "/"), ".git")
	if !ValidRepo(path) {
		return "", false
	}
	return path, true
}

// ---------- input helpers ----------

// AddBodyFlags registers --body and --body-file on a command.
func AddBodyFlags(cmd *cobra.Command) {
	cmd.Flags().String("body", "", "body text ('-' reads stdin)")
	cmd.Flags().String("body-file", "", "read body from a file")
}

// Body resolves --body / --body-file, supporting '-' for stdin.
func (c *Ctx) Body(cmd *cobra.Command) (string, bool, error) {
	body, _ := cmd.Flags().GetString("body")
	file, _ := cmd.Flags().GetString("body-file")
	if body != "" && file != "" {
		return "", false, Usagef("provide --body or --body-file, not both")
	}
	switch {
	case body == "-" || file == "-":
		data, err := io.ReadAll(c.In)
		if err != nil {
			return "", false, err
		}
		return string(data), true, nil
	case file != "":
		data, err := os.ReadFile(file)
		if err != nil {
			return "", false, err
		}
		return string(data), true, nil
	case cmd.Flags().Changed("body"):
		return body, true, nil
	default:
		return "", false, nil
	}
}

// ConfirmDelete mirrors the bash confirm_delete: --yes bypasses, otherwise
// the user must type the resource name. Refuses when stdin is not a TTY
// and --yes was not given, so headless agents fail loudly instead of hanging.
func (c *Ctx) ConfirmDelete(cmd *cobra.Command, resourceType, resourceName string) error {
	if yes, _ := cmd.Flags().GetBool("yes"); yes {
		return nil
	}
	if f, ok := c.In.(*os.File); !ok || !isTTY(f) {
		return Usagef("refusing to delete %s %q without confirmation; pass --yes", resourceType, resourceName)
	}
	fmt.Fprintf(c.Out, "Are you sure you want to delete %s %q?\nType the name to confirm: ", resourceType, resourceName)
	sc := bufio.NewScanner(c.In)
	if !sc.Scan() || strings.TrimSpace(sc.Text()) != resourceName {
		return fmt.Errorf("aborted")
	}
	return nil
}

func isTTY(f *os.File) bool {
	st, err := f.Stat()
	if err != nil {
		return false
	}
	return st.Mode()&os.ModeCharDevice != 0
}

// AddYesFlag registers --yes on destructive commands.
func AddYesFlag(cmd *cobra.Command) {
	cmd.Flags().Bool("yes", false, "skip the delete confirmation prompt")
}

// BuildBody marshals a map into a JSON request body, dropping nil values.
func BuildBody(fields map[string]any) ([]byte, error) {
	clean := map[string]any{}
	for k, v := range fields {
		if v == nil {
			continue
		}
		clean[k] = v
	}
	return json.Marshal(clean)
}

// QueryEscape is url.QueryEscape re-exported for verb files.
func QueryEscape(s string) string { return url.QueryEscape(s) }

// PathEscape is api.PathEscape re-exported for verb files.
func PathEscape(s string) string { return api.PathEscape(s) }
