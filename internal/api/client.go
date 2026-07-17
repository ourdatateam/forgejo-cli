// Package api is a thin client for the Forgejo REST API mirroring the bash
// script's api_call family: token auth, JSON error extraction, raw
// passthrough for diffs, and dedupe-by-id pagination.
package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)

// ErrDryRun is returned instead of performing a mutating request when
// DryRun is set. The command runner treats it as success (exit 0).
var ErrDryRun = errors.New("dry run")

// Error is a non-2xx API response.
type Error struct {
	Status  int
	Message string
	Hint    string
}

func (e *Error) Error() string {
	msg := fmt.Sprintf("API error (%d): %s", e.Status, e.Message)
	if e.Hint != "" {
		msg += "\nhint: " + e.Hint
	}
	return msg
}

type Client struct {
	BaseURL     string
	Token       string
	ReviewToken string
	DryRun      bool
	Verbose     bool
	HTTP        *http.Client
	Stderr      io.Writer

	// asReview switches auth to ReviewToken (see WithReviewToken).
	asReview bool
	// retryWait overrides backoff in tests.
	retryWait func(attempt int, resp *http.Response) time.Duration
}

func New(baseURL, token string) *Client {
	return &Client{
		BaseURL: baseURL,
		Token:   token,
		HTTP:    &http.Client{Timeout: 120 * time.Second},
		Stderr:  os.Stderr,
	}
}

// WithReviewToken returns a copy of the client that authenticates with the
// configured FORGEJO_REVIEW_TOKEN (second identity for `pr review`).
// Returns an error if no review token is configured.
func (c *Client) WithReviewToken() (*Client, error) {
	if c.ReviewToken == "" {
		return nil, fmt.Errorf("FORGEJO_REVIEW_TOKEN is not configured")
	}
	cp := *c
	cp.asReview = true
	return &cp, nil
}

func (c *Client) authToken() string {
	if c.asReview {
		return c.ReviewToken
	}
	return c.Token
}

// apiURL joins an endpoint like "repos/o/r/issues" onto BaseURL/api/v1/.
func (c *Client) apiURL(endpoint string) string {
	return c.BaseURL + "/api/v1/" + strings.TrimPrefix(endpoint, "/")
}

// Do performs a JSON API call and returns the raw response body on 2xx.
// A non-2xx response becomes *Error with the server's message extracted.
func (c *Client) Do(method, endpoint string, body []byte) ([]byte, error) {
	status, out, err := c.DoStatus(method, endpoint, body)
	if err != nil {
		return nil, err
	}
	if status < 200 || status >= 300 {
		return nil, c.apiError(status, out)
	}
	return out, nil
}

// DoStatus is Do without the non-2xx → error mapping: it returns the HTTP
// status and body for the caller to interpret (idempotent GET-then-write
// flows). Transport failures and dry-run still return an error.
func (c *Client) DoStatus(method, endpoint string, body []byte) (int, []byte, error) {
	return c.roundTrip(method, c.apiURL(endpoint), body, "application/json", nil)
}

// DoRaw performs a call with a caller-chosen Accept header and returns the
// body verbatim (diffs, patches, file contents).
func (c *Client) DoRaw(method, endpoint, accept string) ([]byte, error) {
	status, out, err := c.roundTrip(method, c.apiURL(endpoint), nil, accept, nil)
	if err != nil {
		return nil, err
	}
	if status < 200 || status >= 300 {
		return nil, c.apiError(status, out)
	}
	return out, nil
}

// DoAbsolute performs a call against a full URL (api passthrough). Extra
// headers may be supplied. Returns status and body without error mapping.
func (c *Client) DoAbsolute(method, fullURL string, body []byte, contentType string) (int, []byte, error) {
	return c.roundTrip(method, fullURL, body, "application/json", map[string]string{"Content-Type": contentType})
}

// DoBasic performs a call authenticated with HTTP basic auth (token
// endpoints require it). otp, when non-empty, is sent as X-Forgejo-OTP for
// accounts with TOTP enabled.
func (c *Client) DoBasic(method, endpoint, username, password, otp string, body []byte) (int, []byte, error) {
	hdrs := map[string]string{"basic-user": username, "basic-pass": password}
	if otp != "" {
		hdrs["X-Forgejo-OTP"] = otp
	}
	return c.roundTrip(method, c.apiURL(endpoint), body, "application/json", hdrs)
}

// DoPaged fetches every page of a list endpoint (limit=50, page=1,2,…),
// merges the results deduplicated by .id, and returns one JSON array.
// Termination mirrors the bash api_call_paged: stop when a page contributes
// no new items (covers both a short/empty tail page and a server that
// ignores ?page= and re-returns the same set), or on a short page, capped
// at 200 pages.
func (c *Client) DoPaged(endpoint string) ([]byte, error) {
	sep := "?"
	if strings.Contains(endpoint, "?") {
		sep = "&"
	}
	seen := map[string]bool{}
	var acc []json.RawMessage
	for page := 1; ; page++ {
		raw, err := c.Do("GET", fmt.Sprintf("%s%slimit=50&page=%d", endpoint, sep, page), nil)
		if err != nil {
			return nil, err
		}
		var items []json.RawMessage
		if err := json.Unmarshal(raw, &items); err != nil {
			return nil, fmt.Errorf("expected JSON array from %s: %w", endpoint, err)
		}
		added := 0
		for _, it := range items {
			var probe struct {
				ID json.Number `json:"id"`
			}
			key := string(it)
			if err := json.Unmarshal(it, &probe); err == nil && probe.ID != "" {
				key = probe.ID.String()
			}
			if !seen[key] {
				seen[key] = true
				acc = append(acc, it)
				added++
			}
		}
		if added == 0 || len(items) < 50 {
			break
		}
		if page >= 200 {
			fmt.Fprintf(c.Stderr, "forgejo: warning: pagination cap (200 pages) hit at %s\n", endpoint)
			break
		}
	}
	if acc == nil {
		return []byte("[]"), nil
	}
	// unique_by(.id) in the bash version sorts the merged array ascending
	// by id; downstream consumers depend on that ordering.
	sort.SliceStable(acc, func(i, j int) bool {
		return pagedSortKey(acc[i]) < pagedSortKey(acc[j])
	})
	return json.Marshal(acc)
}

func pagedSortKey(item json.RawMessage) float64 {
	var probe struct {
		ID json.Number `json:"id"`
	}
	if err := json.Unmarshal(item, &probe); err == nil && probe.ID != "" {
		if f, err := probe.ID.Float64(); err == nil {
			return f
		}
	}
	return 0
}

// ListTotal extracts X-Total-Count style totals for truncation trailers.
type ListResult struct {
	Body  []byte
	Total int // -1 when the server did not report a total
}

// DoList performs a GET with server-side paging (?limit=&page=1) and
// captures X-Total-Count so callers can render a visible-truncation
// trailer. limit<=0 falls back to DoPaged (fetch everything).
func (c *Client) DoList(endpoint string, limit int) (*ListResult, error) {
	if limit <= 0 {
		body, err := c.DoPaged(endpoint)
		if err != nil {
			return nil, err
		}
		return &ListResult{Body: body, Total: -1}, nil
	}
	sep := "?"
	if strings.Contains(endpoint, "?") {
		sep = "&"
	}
	var hdr http.Header
	status, out, err := c.roundTripHdr("GET", c.apiURL(fmt.Sprintf("%s%slimit=%d&page=1", endpoint, sep, limit)), nil, "application/json", nil, &hdr)
	if err != nil {
		return nil, err
	}
	if status < 200 || status >= 300 {
		return nil, c.apiError(status, out)
	}
	total := -1
	if t := hdr.Get("X-Total-Count"); t != "" {
		if n, err := strconv.Atoi(t); err == nil {
			total = n
		}
	}
	return &ListResult{Body: out, Total: total}, nil
}

func (c *Client) roundTrip(method, fullURL string, body []byte, accept string, extra map[string]string) (int, []byte, error) {
	return c.roundTripHdr(method, fullURL, body, accept, extra, nil)
}

func (c *Client) roundTripHdr(method, fullURL string, body []byte, accept string, extra map[string]string, hdrOut *http.Header) (int, []byte, error) {
	mutating := method != "GET" && method != "HEAD"
	if c.DryRun && mutating {
		fmt.Fprintf(c.Stderr, "DRY-RUN: %s %s\n", method, redactURL(fullURL))
		if len(body) > 0 {
			fmt.Fprintf(c.Stderr, "%s\n", body)
		}
		return 0, nil, ErrDryRun
	}

	const maxAttempts = 3
	var lastStatus int
	var lastBody []byte
	for attempt := 1; ; attempt++ {
		req, err := http.NewRequest(method, fullURL, bytes.NewReader(body))
		if err != nil {
			return 0, nil, err
		}
		req.Header.Set("Accept", accept)
		basicUser, basicPass := "", ""
		for k, v := range extra {
			switch k {
			case "basic-user":
				basicUser = v
			case "basic-pass":
				basicPass = v
			default:
				req.Header.Set(k, v)
			}
		}
		if basicUser != "" {
			req.SetBasicAuth(basicUser, basicPass)
		} else {
			req.Header.Set("Authorization", "token "+c.authToken())
		}
		if len(body) > 0 && req.Header.Get("Content-Type") == "" {
			req.Header.Set("Content-Type", "application/json")
		}

		start := time.Now()
		resp, err := c.httpClient().Do(req)
		if err != nil {
			return 0, nil, fmt.Errorf("forgejo: network error: %w", err)
		}
		out, rerr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if rerr != nil {
			return 0, nil, fmt.Errorf("forgejo: network error: %w", rerr)
		}
		if c.Verbose {
			fmt.Fprintf(c.Stderr, "> %s %s → %d (%.2fs)\n", method, redactURL(fullURL), resp.StatusCode, time.Since(start).Seconds())
		}
		lastStatus, lastBody = resp.StatusCode, out
		if hdrOut != nil {
			*hdrOut = resp.Header
		}

		if attempt < maxAttempts && shouldRetry(method, resp.StatusCode) {
			wait := c.backoff(attempt, resp)
			if c.Verbose {
				fmt.Fprintf(c.Stderr, "> retrying in %s (attempt %d/%d)\n", wait, attempt+1, maxAttempts)
			}
			time.Sleep(wait)
			continue
		}
		return lastStatus, lastBody, nil
	}
}

func (c *Client) httpClient() *http.Client {
	if c.HTTP != nil {
		return c.HTTP
	}
	return http.DefaultClient
}

// shouldRetry: GET/HEAD retry on 429 and transient 5xx. PUT/PATCH/DELETE
// retry only on 429 (rejected before processing). POST is never retried —
// a timed-out POST may have committed server-side, and a retry would
// duplicate the comment/review/release.
func shouldRetry(method string, status int) bool {
	switch method {
	case "GET", "HEAD":
		return status == 429 || status == 500 || status == 502 || status == 503 || status == 504
	case "PUT", "PATCH", "DELETE":
		return status == 429
	default:
		return false
	}
}

func (c *Client) backoff(attempt int, resp *http.Response) time.Duration {
	if c.retryWait != nil {
		return c.retryWait(attempt, resp)
	}
	if ra := resp.Header.Get("Retry-After"); ra != "" {
		if secs, err := strconv.Atoi(ra); err == nil && secs >= 0 && secs <= 30 {
			return time.Duration(secs) * time.Second
		}
	}
	return time.Duration(500*(1<<(attempt-1))) * time.Millisecond
}

// apiError extracts .message // .error from the body like the bash version.
func (c *Client) apiError(status int, body []byte) *Error {
	msg := strings.TrimSpace(string(body))
	var parsed struct {
		Message string `json:"message"`
		Err     string `json:"error"`
	}
	if err := json.Unmarshal(body, &parsed); err == nil {
		if parsed.Message != "" {
			msg = parsed.Message
		} else if parsed.Err != "" {
			msg = parsed.Err
		}
	}
	e := &Error{Status: status, Message: msg}
	switch status {
	case 401:
		e.Hint = "token rejected — check FORGEJO_TOKEN (forgejo auth status)"
	case 403:
		e.Hint = c.scopeHint()
	}
	return e
}

// scopeHint fetches the current token's scopes so a 403 tells the caller
// what the token can actually do. Best-effort: failures yield a generic hint.
func (c *Client) scopeHint() string {
	// A second 403 here would recurse; call roundTrip directly.
	status, body, err := c.roundTrip("GET", c.apiURL("user/tokens/current"), nil, "application/json", nil)
	if err != nil || status < 200 || status >= 300 {
		return "token may lack the required scope for this operation"
	}
	var tok struct {
		Name   string   `json:"name"`
		Scopes []string `json:"scopes"`
	}
	if json.Unmarshal(body, &tok) != nil || len(tok.Scopes) == 0 {
		return "token may lack the required scope for this operation"
	}
	return fmt.Sprintf("token %q has scopes: %s — this operation needs more", tok.Name, strings.Join(tok.Scopes, ", "))
}

// PathEscape escapes a single path segment for endpoint construction.
func PathEscape(seg string) string {
	return url.PathEscape(seg)
}

// redactURL strips credentials that might appear in a caller-supplied URL.
func redactURL(s string) string {
	u, err := url.Parse(s)
	if err != nil || u.User == nil {
		return s
	}
	u.User = url.User("REDACTED")
	return u.String()
}
