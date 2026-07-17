// Package cmd wires the cobra command tree. Each top-level resource group
// lives in its own file (repo.go, issue.go, pr.go, …) and registers itself
// via Register(newXxxCmd).
package cmd

import (
	"errors"
	"fmt"
	"os"

	"github.com/ourdatateam/forgejo-cli/internal/api"
	"github.com/ourdatateam/forgejo-cli/internal/cmdutil"
	"github.com/ourdatateam/forgejo-cli/internal/config"
	"github.com/spf13/cobra"
)

// Exit codes: 0 success, 1 API/runtime error, 2 usage error, 3 auth/scope
// error (401/403). `forgejo api` keeps its historical 22 on HTTP errors.
const (
	ExitOK    = 0
	ExitError = 1
	ExitUsage = 2
	ExitAuth  = 3
)

var groups []func(*cmdutil.Ctx) *cobra.Command

// Register adds a command-group constructor; called from init() in each
// group file so files stay independent.
func Register(g func(*cmdutil.Ctx) *cobra.Command) {
	groups = append(groups, g)
}

// Version is stamped by the release build (-ldflags "-X ...cmd.Version=v1.2.3").
var Version = "dev"

func NewRoot(ctx *cmdutil.Ctx) *cobra.Command {
	root := &cobra.Command{
		Use:           "forgejo <resource> <verb> [args]",
		Short:         "CLI for the Forgejo REST API",
		Version:       Version,
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			// auth login must work before any config exists.
			if cmd.Name() == "login" || cmd.CalledAs() == "login" {
				return nil
			}
			return initClient(cmd, ctx)
		},
	}
	pf := root.PersistentFlags()
	pf.BoolVar(&ctx.JSON, "json", false, "output raw JSON from the server")
	pf.StringVar(&ctx.JQ, "jq", "", "filter JSON output through a jq expression (implies --json)")
	pf.IntVar(&ctx.Limit, "limit", -1, "max items for list verbs (0 = fetch all pages; default: per-verb)")
	pf.Bool("dry-run", false, "print mutating requests instead of sending them")
	pf.Bool("verbose", false, "log requests to stderr (tokens are never logged)")
	pf.StringP("repo", "R", "", "target repository as owner/repo (gh-style alternative to the repo positional; '.' infers from the cwd git remote)")

	for _, g := range groups {
		root.AddCommand(g(ctx))
	}
	return root
}

func initClient(cmd *cobra.Command, ctx *cmdutil.Ctx) error {
	if ctx.Client != nil { // tests may pre-inject a client
		return nil
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	ctx.Config = cfg
	cl := api.New(cfg.URL, cfg.Token)
	cl.ReviewToken = cfg.ReviewToken
	cl.DryRun, _ = cmd.Flags().GetBool("dry-run")
	cl.Verbose, _ = cmd.Flags().GetBool("verbose")
	cl.Stderr = ctx.Err
	ctx.Client = cl
	return nil
}

// Main runs the CLI and returns the process exit code.
func Main() int {
	ctx := &cmdutil.Ctx{Out: os.Stdout, Err: os.Stderr, In: os.Stdin}
	root := NewRoot(ctx)
	args, err := PreprocessRepoFlag(root, os.Args[1:])
	if err == nil {
		root.SetArgs(args)
		err = root.Execute()
	}
	if err == nil {
		return ExitOK
	}
	if errors.Is(err, api.ErrDryRun) {
		return ExitOK
	}
	var exitErr *cmdutil.ExitError
	if errors.As(err, &exitErr) {
		if exitErr.Err != nil {
			fmt.Fprintln(os.Stderr, exitErr.Err.Error())
		}
		return exitErr.Code
	}
	var usageErr *cmdutil.UsageError
	if errors.As(err, &usageErr) {
		fmt.Fprintln(os.Stderr, "forgejo:", usageErr.Msg)
		return ExitUsage
	}
	var apiErr *api.Error
	if errors.As(err, &apiErr) {
		fmt.Fprintln(os.Stderr, apiErr.Error())
		if apiErr.Status == 401 || apiErr.Status == 403 {
			return ExitAuth
		}
		return ExitError
	}
	fmt.Fprintln(os.Stderr, "forgejo:", err.Error())
	// cobra reports its own parse errors here; treat unknown flags/commands as usage.
	if isCobraParseError(err) {
		return ExitUsage
	}
	return ExitError
}

func isCobraParseError(err error) bool {
	msg := err.Error()
	for _, s := range []string{"unknown command", "unknown flag", "unknown shorthand", "invalid argument", "requires at least", "accepts "} {
		if len(msg) >= len(s) && msg[:len(s)] == s {
			return true
		}
	}
	return false
}
