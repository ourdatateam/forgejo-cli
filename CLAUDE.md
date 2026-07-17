# Working in this repo

## Workflow rules

- **Always work in a git worktree on a feature branch.** Never edit files
  while the primary checkout sits on `main`. Use
  `git worktree add ../forgejo-cli-<topic> -b feature/<topic>` (or use the
  `superpowers:using-git-worktrees` skill).
- **No commits directly to `main`.** `main` is reachable only by merging a
  PR. Server-side branch protection (if configured) enforces this; client-
  side, never push to `main` and never commit while checked out on `main`.
- **All changes ship via PR.** Even single-line doc tweaks. Open a PR from
  the feature branch, squash-merge it, then delete the branch (local and
  remote).
- **Never `git commit --no-verify`.** The pre-commit hook
  (`hooks/pre-commit`) shellchecks the shell files and, when Go files are
  staged, gates on gofmt, `go vet`, `go test`, and `make docs-check`
  (generated-references drift). Fix findings rather than bypassing the gate.

## Repo facts

- **The Go CLI is the product.** The installed `forgejo` binary comes from
  a release tarball (`make release`, tagged; v0.1.0 was the first — see
  `docs/RELEASING.md`), NOT from a symlink into this repo. Layout:
  `cmd/forgejo` + `internal/{api,cmd,cmdutil,config}`, one file per command
  group in `internal/cmd/`. Shared plumbing lives in `internal/cmdutil`
  (output, repo resolution, confirmation) and `internal/api` (client,
  pagination, retries, dry-run). Read `docs/PORT.md` before changing output
  or exit-code behavior — both are contracts. `make build test docs-check`
  before any commit touching Go. Skill references under
  `skills/forgejo-cli/references/` are GENERATED (`make docs`) — never
  hand-edit them; edit command Long/Short text instead.
- **Bash CLI (`./forgejo`) is frozen** — behavioral reference only, kept
  for `scripts/parity.sh`. Never add features to it; when porting or
  comparing, bash semantics win over API docs. On hosts where it's still
  wanted it installs as `forgejo-bash`. Helpers (`api_call`,
  `api_call_status`, `api_call_raw`, `api_call_basic`, `get_flag`,
  `has_flag`, `confirm_delete`) live near the top.
- Token + URL in `~/.config/forgejo-cli/config` (mode 600), read at script
  startup. Do not log token contents.
- Branch protection on `main` is project policy: a single-maintainer merge
  whitelist plus a required-PR gate is the recommended default. See
  `CONTRIBUTING.md` for the exact `forgejo branch protect` invocation.
- **Throwaway live-acceptance repo:** for verbs that mutate repo state
  (issues, PRs, releases, hooks), create a disposable repo in an org you
  control — e.g. `forgejo api POST /orgs/<your-org>/repos -f
  name=forgejo-cli-acceptance -F auto_init=true` — and tear it down at the
  end of each spec. Admin- and instance-scoped verbs do not need one.

## Style

- `--flag=value` for all flags (cobra also accepts `--flag value`). No
  gh-style `-f`/`-F` except inside `forgejo api`, where the gh-style flags
  are the documented surface; `-R/--repo` is the one gh-style shorthand on
  the general surface.
- Idempotent verbs where the API allows. GET-first then POST/PATCH is the
  pattern for "create or update" flows (see `branch protect`).
- Test surface: `go test ./...` (httptest unit tests), `scripts/parity.sh`
  (Go vs frozen bash on read verbs), and `scripts/acceptance.sh` (live,
  creates and deletes its own throwaway repo). Run all three for client or
  verb-behavior changes; acceptance needs a token with full repo scopes.
