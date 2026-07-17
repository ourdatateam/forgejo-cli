package cmd

// Ported from bash cmd_api/build_api_body (forgejo:5293-5385).
// This command deliberately bypasses EmitJSON: successful responses are
// written verbatim, and HTTP failures keep the historical exit code 22.

import (
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"regexp"
	"strings"

	"github.com/ourdatateam/forgejo-cli/internal/cmdutil"
	"github.com/spf13/cobra"
)

func init() { Register(newApiCmd) }

var apiIntRe = regexp.MustCompile(`^-?[0-9]+$`)

func newApiCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "api [METHOD] <path> [-f key=val]... [-F key=val]... [--input -]",
		Short: "Make a raw Forgejo API request",
		Long: `Make a raw Forgejo API request.

METHOD is optional and must be exactly one of GET, POST, PUT, PATCH, DELETE,
or HEAD in uppercase. If omitted, GET is used. Lowercase method words are
treated as the path and therefore fail the path rules.

path may be an absolute http(s):// URL, an /api/v1/... path, or any other
/path. /api/v1/... is used as-is on the configured instance; other /path
values are prefixed with /api/v1. Paths starting with // are rejected.

Absolute URLs are allowed only when their scheme, host, and port match the
configured FORGEJO_URL. The command refuses cross-host absolute URLs rather
than sending credentials to them.

-f key=value adds a JSON string field. -F key=value adds a typed field:
true/false become JSON booleans and integer-looking values become JSON
numbers; floats remain strings. -F key=@file reads the file contents as the
value. key[]=value accumulates array entries. For GET and HEAD, -f/-F pairs
become URL-encoded query parameters instead of a JSON body. --input - reads
the full request body from stdin and overrides any -f/-F body fields.

Argument forms:
  -f key=value       add key as a JSON string field, or query param on GET/HEAD
  -F key=value       add key as a typed field: bools and integers become JSON
                     values; floats remain strings
  -F key=@file       read file contents as the value
  key[]=value        accumulate repeated values into a JSON array
  --input -          read the raw request body from stdin
  -                  stdin sentinel accepted only after --input`,
		DisableFlagParsing: true,
		Args:               cobra.ArbitraryArgs,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if apiHelpRequested(args) {
				return cmd.Help()
			}
			return runAPI(cmd, ctx, args)
		},
	}
	cmd.SetOut(ctx.Out)
	cmd.SetErr(ctx.Err)
	return cmd
}

func runAPI(cmd *cobra.Command, ctx *cmdutil.Ctx, args []string) error {
	if len(args) == 0 {
		return cmdutil.Usagef("Usage: forgejo api [METHOD] <path> [-f key=val]... [-F key=val]... [--input -]")
	}

	method := "GET"
	if isAPIMethod(args[0]) {
		method = args[0]
		args = args[1:]
	}
	if len(args) == 0 {
		return cmdutil.Usagef("forgejo api: missing path")
	}
	path := args[0]
	args = args[1:]

	if err := validateAPIPath(path); err != nil {
		return err
	}
	if err := initClient(cmd, ctx); err != nil {
		return err
	}
	fullURL, absolute, err := resolveAPIURL(ctx, path)
	if err != nil {
		return err
	}
	if absolute {
		if err := enforceSameOrigin(ctx, fullURL); err != nil {
			return err
		}
	}

	built, err := buildAPIBody(ctx.In, method, args)
	if err != nil {
		return err
	}
	if built.Query != "" {
		fullURL = appendQuery(fullURL, built.Query)
	}

	status, raw, err := ctx.Client.DoAbsolute(method, fullURL, built.Body, built.ContentType)
	if err != nil {
		return err
	}
	if status < 200 || status >= 300 {
		fmt.Fprintf(ctx.Err, "forgejo api: HTTP %d\n", status)
		if len(raw) > 0 {
			_, _ = ctx.Err.Write(raw)
		}
		return &cmdutil.ExitError{Code: 22}
	}
	_, err = ctx.Out.Write(raw)
	return err
}

type apiBody struct {
	Body        []byte
	Query       string
	ContentType string
}

func buildAPIBody(in io.Reader, method string, args []string) (*apiBody, error) {
	forQuery := method == "GET" || method == "HEAD"
	var queryPairs []string
	bodyFields := map[string]any{}
	inputSeen := false
	var inputBody []byte

	for i := 0; i < len(args); {
		switch args[i] {
		case "--input":
			if i+1 >= len(args) || args[i+1] != "-" {
				return nil, cmdutil.Usagef("forgejo api: --input requires '-' (stdin) in v1")
			}
			data, err := io.ReadAll(in)
			if err != nil {
				return nil, err
			}
			inputSeen = true
			inputBody = data
			i += 2
		case "-f", "-F":
			typed := args[i] == "-F"
			if i+1 >= len(args) || !strings.Contains(args[i+1], "=") {
				return nil, cmdutil.Usagef("forgejo api: -f/-F requires key=value")
			}
			key, rawVal, _ := strings.Cut(args[i+1], "=")
			isArray := false
			if strings.HasSuffix(key, "[]") {
				isArray = true
				key = strings.TrimSuffix(key, "[]")
			}
			val := rawVal
			if typed && strings.HasPrefix(val, "@") {
				data, err := os.ReadFile(strings.TrimPrefix(val, "@"))
				if err != nil {
					return nil, err
				}
				val = string(data)
			}
			if forQuery {
				queryPairs = append(queryPairs, apiQueryEscape(key)+"="+apiQueryEscape(val))
				i += 2
				continue
			}
			converted := convertAPIValue(val, typed)
			if isArray {
				existing, ok := bodyFields[key]
				if !ok {
					bodyFields[key] = []any{converted}
				} else if arr, ok := existing.([]any); ok {
					bodyFields[key] = append(arr, converted)
				} else {
					return nil, cmdutil.Usagef("forgejo api: cannot append array value to non-array field %q", key)
				}
			} else {
				bodyFields[key] = converted
			}
			i += 2
		default:
			return nil, cmdutil.Usagef("forgejo api: unknown arg: %s", args[i])
		}
	}

	out := &apiBody{}
	if len(queryPairs) > 0 {
		out.Query = strings.Join(queryPairs, "&")
	}
	if inputSeen {
		if len(inputBody) > 0 {
			out.Body = inputBody
			out.ContentType = "application/json"
		}
		return out, nil
	}
	if !forQuery && len(bodyFields) > 0 {
		body, err := json.Marshal(bodyFields)
		if err != nil {
			return nil, err
		}
		out.Body = body
		out.ContentType = "application/json"
	}
	return out, nil
}

func convertAPIValue(val string, typed bool) any {
	if !typed {
		return val
	}
	switch val {
	case "true":
		return true
	case "false":
		return false
	default:
		if apiIntRe.MatchString(val) {
			return json.Number(val)
		}
		return val
	}
}

func isAPIMethod(s string) bool {
	switch s {
	case "GET", "POST", "PUT", "PATCH", "DELETE", "HEAD":
		return true
	default:
		return false
	}
}

func apiHelpRequested(args []string) bool {
	for _, arg := range args {
		if arg == "-h" || arg == "--help" {
			return true
		}
	}
	return false
}

func resolveAPIURL(ctx *cmdutil.Ctx, path string) (string, bool, error) {
	base := apiBaseURL(ctx)
	if base == "" {
		return "", false, fmt.Errorf("missing FORGEJO_URL")
	}
	switch {
	case strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://"):
		u, err := url.Parse(path)
		if err != nil || u.Scheme == "" || u.Host == "" {
			return "", false, cmdutil.Usagef("forgejo api: invalid URL %q", path)
		}
		return path, true, nil
	case strings.HasPrefix(path, "//"):
		return "", false, cmdutil.Usagef("forgejo api: path must not start with //")
	case strings.HasPrefix(path, "/api/v1/"):
		return base + path, false, nil
	case strings.HasPrefix(path, "/"):
		return base + "/api/v1" + path, false, nil
	default:
		return "", false, cmdutil.Usagef("forgejo api: path must start with / or http(s)://")
	}
}

func validateAPIPath(path string) error {
	switch {
	case strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://"):
		u, err := url.Parse(path)
		if err != nil || u.Scheme == "" || u.Host == "" {
			return cmdutil.Usagef("forgejo api: invalid URL %q", path)
		}
		return nil
	case strings.HasPrefix(path, "//"):
		return cmdutil.Usagef("forgejo api: path must not start with //")
	case strings.HasPrefix(path, "/"):
		return nil
	default:
		return cmdutil.Usagef("forgejo api: path must start with / or http(s)://")
	}
}

func apiBaseURL(ctx *cmdutil.Ctx) string {
	if ctx.Client != nil && ctx.Client.BaseURL != "" {
		return strings.TrimRight(ctx.Client.BaseURL, "/")
	}
	if ctx.Config != nil {
		return strings.TrimRight(ctx.Config.URL, "/")
	}
	return ""
}

func enforceSameOrigin(ctx *cmdutil.Ctx, fullURL string) error {
	base, err := url.Parse(apiBaseURL(ctx))
	if err != nil {
		return fmt.Errorf("invalid FORGEJO_URL: %w", err)
	}
	target, err := url.Parse(fullURL)
	if err != nil {
		return err
	}
	if sameOrigin(base, target) {
		return nil
	}
	host := target.Host
	if host == "" {
		host = fullURL
	}
	return fmt.Errorf("refusing to send credentials to %s", host)
}

func appendQuery(fullURL, query string) string {
	if query == "" {
		return fullURL
	}
	if strings.Contains(fullURL, "?") {
		if strings.HasSuffix(fullURL, "?") || strings.HasSuffix(fullURL, "&") {
			return fullURL + query
		}
		return fullURL + "&" + query
	}
	return fullURL + "?" + query
}

func apiQueryEscape(s string) string {
	return strings.ReplaceAll(url.QueryEscape(s), "+", "%20")
}
