package cmdutil

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestEmitJSONPrettyPrintsPreservingKeyOrderAndBigInteger(t *testing.T) {
	var out bytes.Buffer
	ctx := &Ctx{Out: &out}

	raw := []byte(`{"b":1,"id":9007199254740993,"a":2}`)
	if err := ctx.EmitJSON(raw); err != nil {
		t.Fatalf("EmitJSON returned error: %v", err)
	}

	want := "{\n  \"b\": 1,\n  \"id\": 9007199254740993,\n  \"a\": 2\n}\n"
	if out.String() != want {
		t.Fatalf("EmitJSON output = %q, want %q", out.String(), want)
	}
}

func TestEmitJSONJQBareStringPrintsRaw(t *testing.T) {
	var out bytes.Buffer
	ctx := &Ctx{Out: &out, JQ: ".name"}

	if err := ctx.EmitJSON([]byte(`{"name":"octo"}`)); err != nil {
		t.Fatalf("EmitJSON returned error: %v", err)
	}
	if got, want := out.String(), "octo\n"; got != want {
		t.Fatalf("jq output = %q, want %q", got, want)
	}
}

func TestStrDottedPathsAndMissing(t *testing.T) {
	obj, err := ParseObject([]byte(`{"user":{"login":"alice"},"nil":null}`))
	if err != nil {
		t.Fatalf("ParseObject returned error: %v", err)
	}

	if got, want := Str(obj, "user.login"), "alice"; got != want {
		t.Fatalf("Str(user.login) = %q, want %q", got, want)
	}
	if got := Str(obj, "user.missing"); got != "" {
		t.Fatalf("Str(user.missing) = %q, want empty string", got)
	}
	if got := Str(obj, "missing.path"); got != "" {
		t.Fatalf("Str(missing.path) = %q, want empty string", got)
	}
	if got := Str(obj, "nil"); got != "" {
		t.Fatalf("Str(nil) = %q, want empty string", got)
	}
}

func TestValidRepoRejectsDotDotAndExtraSlash(t *testing.T) {
	for _, repo := range []string{"../repo", "owner/..", "a/b/c"} {
		if ValidRepo(repo) {
			t.Fatalf("ValidRepo(%q) = true, want false", repo)
		}
	}
	if !ValidRepo("owner/repo") {
		t.Fatal("ValidRepo(owner/repo) = false, want true")
	}
}

func TestIDArgRejectsNonNumeric(t *testing.T) {
	if _, err := IDArg("12x", "issue"); err == nil {
		t.Fatal("IDArg(12x) returned nil error, want usage error")
	}
}

func TestTruncIsRuneSafe(t *testing.T) {
	if got, want := Trunc("ab猫犬", 3), "ab猫"; got != want {
		t.Fatalf("Trunc returned %q, want %q", got, want)
	}
}

func TestMatchRemoteMatchesHTTPSAndSCPFormsOnRightHostOnly(t *testing.T) {
	tests := []struct {
		remote string
		host   string
		repo   string
		ok     bool
	}{
		{remote: "https://forgejo.example/owner/repo.git", host: "forgejo.example", repo: "owner/repo", ok: true},
		{remote: "git@forgejo.example:owner/repo.git", host: "forgejo.example", repo: "owner/repo", ok: true},
		{remote: "https://evil.example/owner/repo.git", host: "forgejo.example", ok: false},
		{remote: "git@evil.example:owner/repo.git", host: "forgejo.example", ok: false},
	}

	for _, tt := range tests {
		t.Run(tt.remote, func(t *testing.T) {
			repo, ok := matchRemote(tt.remote, tt.host)
			if ok != tt.ok || repo != tt.repo {
				t.Fatalf("matchRemote(%q, %q) = %q, %v; want %q, %v", tt.remote, tt.host, repo, ok, tt.repo, tt.ok)
			}
		})
	}
}

func TestConfirmDeleteWithPipedStdinNoYesErrorsWithoutReading(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	defer r.Close()
	if _, err := w.WriteString("target\n"); err != nil {
		t.Fatalf("write pipe: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close pipe writer: %v", err)
	}

	cmd := &cobra.Command{}
	AddYesFlag(cmd)
	ctx := &Ctx{In: r, Out: io.Discard}

	err = ctx.ConfirmDelete(cmd, "repo", "target")
	if err == nil {
		t.Fatal("ConfirmDelete returned nil error, want non-TTY confirmation error")
	}
	if !strings.Contains(err.Error(), "pass --yes") {
		t.Fatalf("error = %q, want --yes guidance", err.Error())
	}

	remaining, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read remaining pipe data: %v", err)
	}
	if got, want := string(remaining), "target\n"; got != want {
		t.Fatalf("pipe contents after ConfirmDelete = %q, want %q", got, want)
	}
}
