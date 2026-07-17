package cmd

// Shared private helpers for the command group files. These were
// independently (re)invented by several groups during the port and are
// deduplicated here — add to this file rather than redeclaring per group.

import (
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"strings"

	"github.com/ourdatateam/forgejo-cli/internal/cmdutil"
)

// sameOrigin reports whether two URLs share scheme+host+port. Credentials
// are only ever attached to same-origin requests, so this comparison is
// strict: default ports normalized, hostnames case-folded, IPs canonical.
func sameOrigin(a, b *url.URL) bool {
	if a == nil || b == nil {
		return false
	}
	as, ah, ap := originParts(a)
	bs, bh, bp := originParts(b)
	return as == bs && ah == bh && ap == bp
}

func originParts(u *url.URL) (string, string, string) {
	scheme := strings.ToLower(u.Scheme)
	host := strings.ToLower(u.Hostname())
	port := u.Port()
	if port == "" {
		switch scheme {
		case "http":
			port = "80"
		case "https":
			port = "443"
		}
	}
	if ip := net.ParseIP(host); ip != nil {
		host = ip.String()
	}
	return scheme, host, port
}

// repoAPIPath renders owner/repo with each segment path-escaped.
func repoAPIPath(repo string) string {
	owner, name, _ := strings.Cut(repo, "/")
	return cmdutil.PathEscape(owner) + "/" + cmdutil.PathEscape(name)
}

func firstLine(s string) string {
	line, _, _ := strings.Cut(s, "\n")
	return line
}

// scalarString renders a decoded JSON value for table cells.
func scalarString(v any) string {
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
			parts = append(parts, scalarString(e))
		}
		return strings.Join(parts, ",")
	case map[string]any:
		if name, ok := t["name"]; ok {
			return scalarString(name)
		}
		return "{...}"
	default:
		return fmt.Sprintf("%v", t)
	}
}
