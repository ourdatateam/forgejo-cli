// Package config loads forgejo-cli configuration from the environment and
// ~/.config/forgejo-cli/config. The file is plain KEY=VALUE (optionally
// quoted, optional "export " prefix, # comments) and is parsed strictly —
// it is never executed, unlike the bash version which sourced it.
package config

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type Config struct {
	URL         string // FORGEJO_URL, no trailing slash
	Token       string // FORGEJO_TOKEN (or output of FORGEJO_TOKEN_COMMAND)
	ReviewToken string // FORGEJO_REVIEW_TOKEN: optional second identity for `pr review`
	Password    string // FORGEJO_PASSWORD: basic-auth verbs (token minting)
	LogPath     string // FORGEJO_LOG_PATH: on-host actions log directory
}

// Path returns the config file location, honoring FORGEJO_CONFIG for tests.
func Path() string {
	if p := os.Getenv("FORGEJO_CONFIG"); p != "" {
		return p
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config", "forgejo-cli", "config")
}

var knownKeys = map[string]bool{
	"FORGEJO_URL": true, "FORGEJO_TOKEN": true, "FORGEJO_TOKEN_COMMAND": true,
	"FORGEJO_REVIEW_TOKEN": true, "FORGEJO_REVIEW_TOKEN_COMMAND": true,
	"FORGEJO_PASSWORD": true, "FORGEJO_LOG_PATH": true,
}

// Load reads the config file (if present), applies environment overrides,
// and resolves *_COMMAND token sources. Environment variables always win
// over file values. Returns an error if neither source yields URL+token.
func Load() (*Config, error) {
	vals := map[string]string{}

	path := Path()
	if st, err := os.Lstat(path); err == nil {
		if st.Mode()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("config file %s is a symlink; refusing to read it", path)
		}
		if !st.Mode().IsRegular() {
			return nil, fmt.Errorf("config file %s is not a regular file", path)
		}
		if perm := st.Mode().Perm(); perm != 0o600 {
			return nil, fmt.Errorf("config file has insecure permissions: %o (expected 600)\nFix with: chmod 600 %s", perm, path)
		}
		if err := checkOwner(st); err != nil {
			return nil, fmt.Errorf("config file %s: %w", path, err)
		}
		fileVals, err := parseFile(path)
		if err != nil {
			return nil, err
		}
		vals = fileVals
	}

	for k := range knownKeys {
		if v, ok := os.LookupEnv(k); ok && v != "" {
			vals[k] = v
		}
	}

	c := &Config{
		URL:         strings.TrimRight(vals["FORGEJO_URL"], "/"),
		Token:       vals["FORGEJO_TOKEN"],
		ReviewToken: vals["FORGEJO_REVIEW_TOKEN"],
		Password:    vals["FORGEJO_PASSWORD"],
		LogPath:     vals["FORGEJO_LOG_PATH"],
	}

	if c.Token == "" && vals["FORGEJO_TOKEN_COMMAND"] != "" {
		tok, err := runTokenCommand(vals["FORGEJO_TOKEN_COMMAND"])
		if err != nil {
			return nil, fmt.Errorf("FORGEJO_TOKEN_COMMAND: %w", err)
		}
		c.Token = tok
	}
	if c.ReviewToken == "" && vals["FORGEJO_REVIEW_TOKEN_COMMAND"] != "" {
		tok, err := runTokenCommand(vals["FORGEJO_REVIEW_TOKEN_COMMAND"])
		if err != nil {
			return nil, fmt.Errorf("FORGEJO_REVIEW_TOKEN_COMMAND: %w", err)
		}
		c.ReviewToken = tok
	}

	if c.URL == "" || c.Token == "" {
		return nil, fmt.Errorf("missing FORGEJO_URL or FORGEJO_TOKEN\n\nCreate %s (mode 600) with:\n  FORGEJO_URL=https://forgejo.example.com\n  FORGEJO_TOKEN=your-token-here\nor run: forgejo auth login", Path())
	}
	if !strings.HasPrefix(c.URL, "http://") && !strings.HasPrefix(c.URL, "https://") {
		return nil, fmt.Errorf("FORGEJO_URL must start with http:// or https://")
	}
	if strings.HasPrefix(c.URL, "http://") {
		fmt.Fprintln(os.Stderr, "forgejo: warning: FORGEJO_URL uses plain http; the token is sent unencrypted")
	}
	return c, nil
}

func parseFile(path string) (map[string]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	vals := map[string]string{}
	for i, line := range strings.Split(string(data), "\n") {
		s := strings.TrimSpace(line)
		if s == "" || strings.HasPrefix(s, "#") {
			continue
		}
		s = strings.TrimPrefix(s, "export ")
		key, val, ok := strings.Cut(s, "=")
		key = strings.TrimSpace(key)
		if !ok || key == "" || strings.ContainsAny(key, " \t") {
			return nil, fmt.Errorf("%s:%d: not a KEY=VALUE line", path, i+1)
		}
		val = strings.TrimSpace(val)
		if strings.HasPrefix(val, "$'") {
			// bash `printf %q` output (old auth login could write this for
			// FORGEJO_PASSWORD). Parsing it literally would corrupt the value.
			return nil, fmt.Errorf("%s:%d: %s uses bash $'…' quoting, which this version does not evaluate — re-enter the value plainly (or quoted with \" \")", path, i+1, key)
		}
		if len(val) >= 2 && (val[0] == '"' && val[len(val)-1] == '"' || val[0] == '\'' && val[len(val)-1] == '\'') {
			val = val[1 : len(val)-1]
		}
		if !knownKeys[key] {
			continue // forward compatibility: ignore unknown keys
		}
		vals[key] = val
	}
	return vals, nil
}

func runTokenCommand(command string) (string, error) {
	cmd := exec.Command("sh", "-c", command)
	cmd.Stderr = os.Stderr
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("command failed: %w", err)
	}
	tok := strings.TrimSpace(string(out))
	if tok == "" {
		return "", fmt.Errorf("command produced no output")
	}
	return tok, nil
}
