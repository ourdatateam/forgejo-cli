package cmd

import (
	"errors"
	"reflect"
	"testing"

	"github.com/ourdatateam/forgejo-cli/internal/cmdutil"
)

func TestPreprocessRepoFlag(t *testing.T) {
	tests := []struct {
		name      string
		argv      []string
		want      []string
		wantUsage bool
	}{
		{
			name: "after verb",
			argv: []string{"pr", "list", "-R", "o/r"},
			want: []string{"pr", "list", "o/r"},
		},
		{
			name: "before verb",
			argv: []string{"-R", "o/r", "pr", "list"},
			want: []string{"pr", "list", "o/r"},
		},
		{
			name: "short equals before positional",
			argv: []string{"pr", "view", "-R=o/r", "42"},
			want: []string{"pr", "view", "o/r", "42"},
		},
		{
			name: "freeform label command",
			argv: []string{"issue", "label", "--repo", "o/r", "add", "1"},
			want: []string{"issue", "label", "o/r", "add", "1"},
		},
		{
			name: "dot repo",
			argv: []string{"pr", "list", "-R", "."},
			want: []string{"pr", "list", "."},
		},
		{
			name: "api passthrough",
			argv: []string{"api", "-R", "o/r", "/user"},
			want: []string{"api", "-R", "o/r", "/user"},
		},
		{
			name: "no flag",
			argv: []string{"pr", "list"},
			want: []string{"pr", "list"},
		},
		{
			name:      "missing value",
			argv:      []string{"pr", "list", "-R"},
			wantUsage: true,
		},
		{
			name:      "invalid repo",
			argv:      []string{"pr", "list", "-R", "not-a-repo"},
			wantUsage: true,
		},
		{
			name: "non repo command",
			argv: []string{"search", "repos", "--query=x", "-R", "o/r"},
			want: []string{"search", "repos", "o/r", "--query=x"},
		},
		{
			name: "existing local repo flag",
			argv: []string{"org", "team", "repo", "add", "acme", "maintainers", "--repo", "o/r"},
			want: []string{"org", "team", "repo", "add", "acme", "maintainers", "--repo", "o/r"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := PreprocessRepoFlag(NewRoot(&cmdutil.Ctx{}), tt.argv)
			if tt.wantUsage {
				var usageErr *cmdutil.UsageError
				if !errors.As(err, &usageErr) {
					t.Fatalf("PreprocessRepoFlag error = %T %[1]v, want *cmdutil.UsageError", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("PreprocessRepoFlag returned error: %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("PreprocessRepoFlag(%q) = %q, want %q", tt.argv, got, tt.want)
			}
		})
	}
}

func TestRootRepoFlagForHelp(t *testing.T) {
	flag := NewRoot(&cmdutil.Ctx{}).PersistentFlags().Lookup("repo")
	if flag == nil {
		t.Fatal("root persistent --repo flag is missing")
	}
	if flag.Shorthand != "R" {
		t.Fatalf("--repo shorthand = %q, want R", flag.Shorthand)
	}
	const wantUsage = "target repository as owner/repo (gh-style alternative to the repo positional; '.' infers from the cwd git remote)"
	if flag.Usage != wantUsage {
		t.Fatalf("--repo usage = %q, want %q", flag.Usage, wantUsage)
	}
}
