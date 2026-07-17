package cmd

// Ported from bash cmd_auth_login/cmd_auth_status (forgejo:5822-5971).
// Login intentionally diverges from the old password-storage flow: it writes
// only FORGEJO_URL and FORGEJO_TOKEN, mode 0600.

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ourdatateam/forgejo-cli/internal/api"
	"github.com/ourdatateam/forgejo-cli/internal/cmdutil"
	"github.com/ourdatateam/forgejo-cli/internal/config"
	"github.com/spf13/cobra"
)

func init() { Register(newAuthCmd) }

func newAuthCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth <login|status>",
		Short: "Authenticate and inspect the configured account",
		Long: `Authenticate and inspect the configured account.

login prompts for a Forgejo URL, username, and password, then mints a token
using HTTP Basic auth. The token is verified with GET /user before the config
file is written. Existing config files require an overwrite prompt showing the
current URL. The config write is atomic, mode 0600, and stores only
FORGEJO_URL and FORGEJO_TOKEN; account passwords are never stored.

status calls GET /user with the configured token. Text output prints the URL,
login, email, and token scopes. Forgejo does not always expose current-token
scopes; when GET /user/tokens/current does not return scopes, status prints
"not exposed by Forgejo API".`,
	}
	cmd.AddCommand(newAuthLoginCmd(ctx), newAuthStatusCmd(ctx))
	return cmd
}

func newAuthLoginCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "login [--scopes=all] [--otp=CODE]",
		Short: "Prompt for credentials and write a token config",
		Long: `Prompt for a Forgejo URL, username, and password, mint an API token,
verify that token with GET /user, and atomically write the CLI config.

--scopes is a comma-separated list sent to the token endpoint; it defaults to
"all", matching the bash implementation. --otp is sent as X-Forgejo-OTP for
accounts that require a one-time password.

Passwords are read from stdin with echo still enabled because this port stays
dependency-free. The password is used only for token creation and is never
written to the config file.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			scopesFlag, _ := cmd.Flags().GetString("scopes")
			otp, _ := cmd.Flags().GetString("otp")
			scopes := splitComma(scopesFlag)
			if len(scopes) == 0 {
				return cmdutil.Usagef("auth login requires at least one scope")
			}
			return runAuthLogin(ctx, scopes, otp)
		},
	}
	cmd.Flags().String("scopes", "all", "comma-separated token scopes to request")
	cmd.Flags().String("otp", "", "one-time password for accounts with 2FA enabled")
	return cmd
}

func runAuthLogin(ctx *cmdutil.Ctx, scopes []string, otp string) error {
	cfgPath := config.Path()
	if cfgPath == "" {
		return fmt.Errorf("could not determine config path")
	}
	reader := bufio.NewReader(ctx.In)

	if err := confirmConfigOverwrite(ctx, reader, cfgPath); err != nil {
		return err
	}

	urlText, err := promptLine(ctx.Err, reader, "Forgejo URL (e.g. https://forgejo.example.com): ")
	if err != nil {
		return err
	}
	urlText = strings.TrimRight(strings.TrimSpace(urlText), "/")
	if !strings.HasPrefix(urlText, "http://") && !strings.HasPrefix(urlText, "https://") {
		return cmdutil.Usagef("URL must start with http:// or https://")
	}

	user, err := promptLine(ctx.Err, reader, "Username: ")
	if err != nil {
		return err
	}
	user = strings.TrimSpace(user)
	if user == "" {
		return cmdutil.Usagef("Username is required.")
	}

	password, err := promptLine(ctx.Err, reader, "Password (input is visible): ")
	if err != nil {
		return err
	}
	if password == "" {
		return cmdutil.Usagef("Password is required.")
	}

	tokenName := authTokenName()
	body, err := json.Marshal(map[string]any{
		"name":   tokenName,
		"scopes": scopes,
	})
	if err != nil {
		return err
	}

	loginClient := api.New(urlText, "")
	loginClient.Stderr = ctx.Err
	status, raw, err := loginClient.DoBasic("POST", "users/"+cmdutil.PathEscape(user)+"/tokens", user, password, otp, body)
	if err != nil {
		return err
	}
	if status == 401 {
		fmt.Fprintln(ctx.Err, "Authentication failed.")
		fmt.Fprintln(ctx.Err, "If this account has 2FA enabled, pass --otp with a current one-time password.")
		fmt.Fprintf(ctx.Err, "Otherwise, check your username and password or create a token in the web UI and set FORGEJO_TOKEN manually in %s.\n", cfgPath)
		return &cmdutil.ExitError{Code: 1}
	}
	if status < 200 || status >= 300 {
		fmt.Fprintf(ctx.Err, "API error (%d):\n%s\n", status, apiErrorText(raw))
		return &cmdutil.ExitError{Code: 1}
	}

	obj, err := cmdutil.ParseObject(raw)
	if err != nil {
		return err
	}
	token := cmdutil.Str(obj, "sha1")
	if token == "" {
		return fmt.Errorf("forgejo: token endpoint returned no sha1 value")
	}

	verifyClient := api.New(urlText, token)
	verifyClient.Stderr = ctx.Err
	vstatus, vraw, err := verifyClient.DoStatus("GET", "user", nil)
	if err != nil {
		return fmt.Errorf("forgejo: network error verifying token: %w", err)
	}
	if vstatus < 200 || vstatus >= 300 {
		fmt.Fprintf(ctx.Err, "Token verification failed (status %d).\n", vstatus)
		return &cmdutil.ExitError{Code: 1}
	}
	login := user
	if vobj, err := cmdutil.ParseObject(vraw); err == nil {
		if got := cmdutil.Str(vobj, "login"); got != "" {
			login = got
		}
	}

	if err := writeConfigAtomic(cfgPath, urlText, token); err != nil {
		return err
	}
	fmt.Fprintf(ctx.Out, "Logged in as %s at %s.\n", login, urlText)
	return nil
}

func newAuthStatusCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show the configured authenticated account",
		Long: `Show the configured authenticated account.

status calls GET /user with the configured token. With --json, the raw user
JSON from the server is printed through the normal JSON path. Text output
prints URL, login, email, and scopes. Scopes are discovered with a best-effort
GET /user/tokens/current; if Forgejo does not expose them, the scopes line says
"not exposed by Forgejo API".`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			raw, err := ctx.Client.Do("GET", "user", nil)
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

			login := cmdutil.Str(obj, "login")
			if cmdutil.Str(obj, "is_admin") == "true" {
				login += " (admin)"
			}
			email := cmdutil.Str(obj, "email")
			if strings.TrimSpace(email) == "" {
				email = "-"
			}
			scopes := currentTokenScopes(ctx)
			if scopes == "" {
				scopes = "not exposed by Forgejo API"
			}

			fmt.Fprintf(ctx.Out, "URL:    %s\n", authConfiguredURL(ctx))
			fmt.Fprintf(ctx.Out, "Login:  %s\n", login)
			fmt.Fprintf(ctx.Out, "Email:  %s\n", email)
			fmt.Fprintf(ctx.Out, "Scopes: %s\n", scopes)
			return nil
		},
	}
	return cmd
}

func confirmConfigOverwrite(ctx *cmdutil.Ctx, reader *bufio.Reader, path string) error {
	st, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if st.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("config file %s is a symlink; refusing to overwrite it", path)
	}
	if !st.Mode().IsRegular() {
		return fmt.Errorf("config file %s is not a regular file", path)
	}

	fmt.Fprintf(ctx.Err, "A config already exists at %s\n", path)
	if existing := currentURLFromFile(path); existing != "" {
		fmt.Fprintf(ctx.Err, "  Current URL: %s\n", existing)
	}
	reply, err := promptLine(ctx.Err, reader, "Overwrite it? (y/N) ")
	if err != nil {
		return err
	}
	if reply != "y" && reply != "Y" {
		fmt.Fprintln(ctx.Err, "Aborted.")
		return &cmdutil.ExitError{Code: 1}
	}
	return nil
}

func promptLine(out io.Writer, reader *bufio.Reader, prompt string) (string, error) {
	fmt.Fprint(out, prompt)
	line, err := reader.ReadString('\n')
	if err != nil {
		if errors.Is(err, io.EOF) && line != "" {
			return strings.TrimRight(line, "\r\n"), nil
		}
		return "", err
	}
	return strings.TrimRight(line, "\r\n"), nil
}

func authTokenName() string {
	host, err := os.Hostname()
	if err != nil || host == "" {
		host = "host"
	}
	if short, _, ok := strings.Cut(host, "."); ok && short != "" {
		host = short
	}
	return "forgejo-cli-" + host + "-" + time.Now().Format("20060102150405")
}

func currentURLFromFile(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		s := strings.TrimSpace(line)
		s = strings.TrimPrefix(s, "export ")
		key, val, ok := strings.Cut(s, "=")
		if !ok || strings.TrimSpace(key) != "FORGEJO_URL" {
			continue
		}
		val = strings.TrimSpace(val)
		if len(val) >= 2 && (val[0] == '"' && val[len(val)-1] == '"' || val[0] == '\'' && val[len(val)-1] == '\'') {
			val = val[1 : len(val)-1]
		}
		return val
	}
	return ""
}

func writeConfigAtomic(path, forgejoURL, token string) error {
	if err := validateBareConfigValue("FORGEJO_URL", forgejoURL); err != nil {
		return err
	}
	if err := validateBareConfigValue("FORGEJO_TOKEN", token); err != nil {
		return err
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".config-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return err
	}
	content := fmt.Sprintf("FORGEJO_URL=%s\nFORGEJO_TOKEN=%s\n", forgejoURL, token)
	if _, err := tmp.WriteString(content); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		return err
	}
	return nil
}

func validateBareConfigValue(key, value string) error {
	if strings.ContainsAny(value, " \t\r\n\"'") {
		return fmt.Errorf("%s contains whitespace or quotes; refusing to write an unsafe config value", key)
	}
	return nil
}

func apiErrorText(raw []byte) string {
	var parsed struct {
		Message string `json:"message"`
		Err     string `json:"error"`
	}
	if json.Unmarshal(raw, &parsed) == nil {
		if parsed.Message != "" {
			return parsed.Message
		}
		if parsed.Err != "" {
			return parsed.Err
		}
	}
	return strings.TrimSpace(string(raw))
}

func authConfiguredURL(ctx *cmdutil.Ctx) string {
	if ctx.Config != nil {
		return ctx.Config.URL
	}
	if ctx.Client != nil {
		return strings.TrimRight(ctx.Client.BaseURL, "/")
	}
	return ""
}

func currentTokenScopes(ctx *cmdutil.Ctx) string {
	status, raw, err := ctx.Client.DoStatus("GET", "user/tokens/current", nil)
	if err != nil || status < 200 || status >= 300 {
		return ""
	}
	obj, err := cmdutil.ParseObject(raw)
	if err != nil {
		return ""
	}
	switch v := obj["scopes"].(type) {
	case []any:
		parts := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok && s != "" {
				parts = append(parts, s)
			}
		}
		return strings.Join(parts, ", ")
	case string:
		return v
	default:
		return cmdutil.Str(obj, "scopes")
	}
}
