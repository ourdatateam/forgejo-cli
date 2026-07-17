package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

const testBaseURL = "https://forgejo.test"

type handlerTransport struct {
	h http.Handler
}

func (t handlerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	rec := httptest.NewRecorder()
	t.h.ServeHTTP(rec, req)
	resp := rec.Result()
	resp.Request = req
	return resp, nil
}

func newHandlerClient(token string, h http.Handler) *Client {
	c := New(testBaseURL, token)
	c.HTTP = &http.Client{Transport: handlerTransport{h: h}}
	return c
}

func TestAuthHeaderSetFromToken(t *testing.T) {
	const token = "secret-token"
	c := newHandlerClient(token, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got, want := r.Header.Get("Authorization"), "token "+token; got != want {
			t.Fatalf("Authorization header = %q, want %q", got, want)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))

	if _, err := c.Do("GET", "user", nil); err != nil {
		t.Fatalf("Do returned error: %v", err)
	}
}

func TestErrorMappingExtractsMessageThenError(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{name: "message wins", body: `{"message":"from message","error":"from error"}`, want: "from message"},
		{name: "error fallback", body: `{"error":"from error"}`, want: "from error"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := newHandlerClient("tok", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte(tt.body))
			}))

			_, err := c.Do("GET", "boom", nil)
			var apiErr *Error
			if !errors.As(err, &apiErr) {
				t.Fatalf("Do error = %T %[1]v, want *Error", err)
			}
			if apiErr.Message != tt.want {
				t.Fatalf("Message = %q, want %q", apiErr.Message, tt.want)
			}
		})
	}
}

func TestForbiddenHintIncludesCurrentTokenScopes(t *testing.T) {
	c := newHandlerClient("tok", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/protected":
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"message":"forbidden"}`))
		case "/api/v1/user/tokens/current":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"name":"ci-token","scopes":["read:repository","write:issue"]}`))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))

	_, err := c.Do("GET", "protected", nil)
	var apiErr *Error
	if !errors.As(err, &apiErr) {
		t.Fatalf("Do error = %T %[1]v, want *Error", err)
	}
	for _, want := range []string{"ci-token", "read:repository", "write:issue"} {
		if !strings.Contains(apiErr.Hint, want) {
			t.Fatalf("hint %q does not contain %q", apiErr.Hint, want)
		}
	}
}

func TestDoPagedDedupesTerminatesAndSortsByID(t *testing.T) {
	var requests int32
	page := fullIgnoredPageBody()
	c := newHandlerClient("tok", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requests, 1)
		if got, want := r.URL.Query().Get("limit"), "50"; got != want {
			t.Fatalf("limit = %q, want %q", got, want)
		}
		_, _ = w.Write([]byte(page))
	}))

	raw, err := c.DoPaged("items")
	if err != nil {
		t.Fatalf("DoPaged returned error: %v", err)
	}
	var items []struct {
		ID int `json:"id"`
	}
	if err := json.Unmarshal(raw, &items); err != nil {
		t.Fatalf("DoPaged body is not JSON array: %v", err)
	}
	if len(items) != 50 {
		t.Fatalf("DoPaged returned %d items, want 50 deduped items", len(items))
	}
	for i, item := range items {
		if want := i + 1; item.ID != want {
			t.Fatalf("item %d id = %d, want ascending id %d", i, item.ID, want)
		}
	}
	if got := atomic.LoadInt32(&requests); got != 2 {
		t.Fatalf("requests = %d, want 2 (initial page plus duplicate terminator)", got)
	}
}

func fullIgnoredPageBody() string {
	var b strings.Builder
	b.WriteByte('[')
	for id := 50; id >= 1; id-- {
		if id != 50 {
			b.WriteByte(',')
		}
		_, _ = fmt.Fprintf(&b, `{"id":%d}`, id)
	}
	b.WriteByte(']')
	return b.String()
}

func TestRetryPolicyGETRetriedOn500ThenSucceeds(t *testing.T) {
	var requests int32
	c := newHandlerClient("tok", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if n := atomic.AddInt32(&requests, 1); n == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))

	c.retryWait = func(int, *http.Response) time.Duration { return 0 }
	if _, err := c.Do("GET", "flaky", nil); err != nil {
		t.Fatalf("Do returned error: %v", err)
	}
	if got := atomic.LoadInt32(&requests); got != 2 {
		t.Fatalf("requests = %d, want 2", got)
	}
}

func TestRetryPolicyPOSTNeverRetriedOn429Or500(t *testing.T) {
	tests := []int{http.StatusTooManyRequests, http.StatusInternalServerError}
	for _, status := range tests {
		t.Run(http.StatusText(status), func(t *testing.T) {
			var requests int32
			c := newHandlerClient("tok", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				atomic.AddInt32(&requests, 1)
				w.WriteHeader(status)
				_, _ = w.Write([]byte(`{"message":"try later"}`))
			}))

			c.retryWait = func(int, *http.Response) time.Duration { return 0 }
			if _, err := c.Do("POST", "mutate", []byte(`{"x":1}`)); err == nil {
				t.Fatal("Do returned nil error, want non-2xx error")
			}
			if got := atomic.LoadInt32(&requests); got != 1 {
				t.Fatalf("requests = %d, want 1", got)
			}
		})
	}
}

func TestRetryPolicyPUTRetriedOn429(t *testing.T) {
	var requests int32
	c := newHandlerClient("tok", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if n := atomic.AddInt32(&requests, 1); n == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))

	c.retryWait = func(int, *http.Response) time.Duration { return 0 }
	if _, err := c.Do("PUT", "mutate", []byte(`{"x":1}`)); err != nil {
		t.Fatalf("Do returned error: %v", err)
	}
	if got := atomic.LoadInt32(&requests); got != 2 {
		t.Fatalf("requests = %d, want 2", got)
	}
}

func TestRetryAfterHonored(t *testing.T) {
	c := New("https://forgejo.example", "tok")
	resp := &http.Response{Header: http.Header{"Retry-After": []string{"0"}}}
	if got := c.backoff(1, resp); got != 0 {
		t.Fatalf("backoff with Retry-After: 0 = %s, want 0", got)
	}
}

func TestDryRunBlocksPOSTButLetsGETThrough(t *testing.T) {
	var requests int32
	c := newHandlerClient("tok", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requests, 1)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))

	var stderr bytes.Buffer
	c.DryRun = true
	c.Stderr = &stderr

	_, err := c.Do("POST", "mutate", []byte(`{"x":1}`))
	if !errors.Is(err, ErrDryRun) {
		t.Fatalf("POST error = %v, want ErrDryRun", err)
	}
	if got := atomic.LoadInt32(&requests); got != 0 {
		t.Fatalf("requests after dry-run POST = %d, want 0", got)
	}

	if _, err := c.Do("GET", "read", nil); err != nil {
		t.Fatalf("GET returned error: %v", err)
	}
	if got := atomic.LoadInt32(&requests); got != 1 {
		t.Fatalf("requests after GET = %d, want 1", got)
	}
}

func TestVerboseOutputNeverContainsTokenString(t *testing.T) {
	const token = "top-secret-token"
	c := newHandlerClient(token, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))

	var stderr bytes.Buffer
	c.Verbose = true
	c.Stderr = &stderr
	if _, err := c.Do("GET", "user", nil); err != nil {
		t.Fatalf("Do returned error: %v", err)
	}
	if strings.Contains(stderr.String(), token) {
		t.Fatalf("verbose output leaked token: %q", stderr.String())
	}
}
