---
name: forgejo-cli
description: >
  Drive a Forgejo instance (repos, issues, PRs, releases, wikis, orgs, users,
  actions, admin) with the `forgejo` CLI. Use whenever a task touches the
  Forgejo server — creating or triaging issues, opening/reviewing/merging PRs,
  managing repos, releases, webhooks, or CI. Load references/<group>.md for
  full per-verb docs; this file covers auth, invocation grammar, and the
  everyday verbs.
---

# forgejo CLI — core usage

Config: `~/.config/forgejo-cli/config` (mode 600) with `FORGEJO_URL` and
`FORGEJO_TOKEN` (or `FORGEJO_TOKEN_COMMAND=op read "op://..."`). Env vars
override the file. First-time setup: `forgejo auth login`. Sanity check:
`forgejo auth status`.

## Invocation grammar

```
forgejo <resource> <verb> [<owner/repo>] [args] [--flags]
```

- The repo positional is **required** where a verb targets a repo. Pass `.`
  to use the current directory's git remote (only remotes on the configured
  Forgejo host match). gh-style `-R owner/repo` / `--repo owner/repo` also
  works anywhere on the line and fills the repo slot (`forgejo pr list -R
  owner/repo`); `-R .` infers like the positional dot.
- Flags are `--flag=value` or `--flag value`. Unknown flags are errors.
- `--json` on any verb prints pretty JSON — this is the stable output
  contract; parse it, not the text tables. `--jq '<expr>'` filters it
  server-side of your pipe: `forgejo issue list o/r --jq '.[].number'`.
- List verbs: `--limit=N` overrides the per-verb default; `--limit=0`
  fetches everything. Truncation notices go to stderr, never stdout.
- Long text: `--body=-` reads stdin; `--body-file=path` reads a file.
  Prefer these over inline quoting for anything multi-line.
- Destructive verbs (delete/remove/transfer) require `--yes` when running
  non-interactively — without it they error instead of prompting.
- `--dry-run` executes reads but prints-and-skips any write (to stderr).

Exit codes: 0 ok · 1 API error · 2 usage · 3 auth/scope (the 403 message
includes the token's actual scopes) · `forgejo api` returns 22 on HTTP errors.

## Everyday verbs

```
forgejo issue list <o/r> [--state=open|closed|all] [--labels=a,b]
forgejo issue create <o/r> --title=T [--body=-] [--labels=x] [--assignees=u]
forgejo issue view <o/r> <n>            # + comments
forgejo issue comment <o/r> <n> --body=-
forgejo issue close <o/r> <n>

forgejo pr list <o/r> [--state=...]
forgejo pr create <o/r> --title=T --head=branch --base=main [--body=-]
forgejo pr view <o/r> <n>               # FULL conversation: reviews+comments
forgejo pr diff <o/r> <n>               # raw diff
forgejo pr checkout <o/r> <n>           # fetch + switch to the PR head branch
forgejo pr edit <o/r> <n> --add-labels=x --add-reviewers=user
forgejo pr review <o/r> <n> --approve|--request-changes|--comment [--body=-]
forgejo pr merge <o/r> <n> [--method=merge|rebase|squash]

forgejo repo list <owner> | view <o/r> | create --name=N [--org=O] [--private]
forgejo release create <o/r> --tag=v1 [--title=T] [--notes=-]
forgejo release download <o/r> v1 [--pattern='*.tar.gz'] [--output=DIR]
forgejo issue edit <o/r> <n> --add-labels=bug --remove-assignees=user
forgejo actions list <o/r> | view <o/r> <run-id> | watch <o/r> <run-id>
forgejo search issues --query=X [--type=issue|pr]
```

## Escape hatch

Any endpoint the verbs don't cover:

```
forgejo api /repos/{owner}/{repo}/... [-f key=val] [-F typed=1] [--input -]
forgejo api POST /orgs/myorg/repos -f name=scratch -F auto_init=true
```

`-f` sends strings, `-F` types booleans/integers, `key[]=v` builds arrays;
on GET they become query params. The token is only ever sent to the
configured instance.

## Deep references (load on demand)

One file per group in `references/`: repo, issue, pr, release, wiki, user,
org, actions, admin, branch, auth, notification, search — plus
`recipes.md` for multi-step workflows (branch→PR→review→merge, triage,
release publishing). These are generated from the CLI itself (`make docs`)
and are always current for the checked-out version.
