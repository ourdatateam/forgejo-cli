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
- **Never `git commit --no-verify`.** The shellcheck pre-commit hook
  (`hooks/pre-commit`) lints `forgejo` and the hook itself; fix findings
  rather than bypassing the gate.

## Repo facts

- Single-file bash CLI at `./forgejo`. Helpers (`api_call`,
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

- `--flag=value` for all flags. No gh-style `-f`/`-F` (except inside
  `forgejo api`, where the gh-style flags are the documented surface).
- Idempotent verbs where the API allows. GET-first then POST/PATCH is the
  pattern for "create or update" flows (see `branch_protect`).
- Live acceptance against a throwaway repo is the only test surface today.
  There is no test framework.
