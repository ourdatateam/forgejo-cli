# Go port — design record

Port of the single-file bash CLI (`./forgejo`, kept in-tree as the reference
implementation during transition) to Go. Module
`github.com/ourdatateam/forgejo-cli`, cobra command tree, hand-rolled API
client (no Gitea/Forgejo SDK — the admin/actions/notification surface and the
raw-JSON output contract make a thin client the better fit).

## Output contract

- `--json` matches the bash `| jq .` behavior: pretty-printed 2-space JSON,
  key order and number fidelity preserved (`json.Indent` on the raw bytes —
  bodies are never decoded into structs/maps on the JSON path).
- Paged merges (`pr view`, `review lookup`, `issue images`) are deduplicated
  and sorted ascending by `.id`, matching `unique_by(.id)`.
- `--jq <expr>` (new, gojq) filters the same JSON; bare strings print raw
  (like `jq -r`).
- Text output is best-effort human/agent readable, one line per item, no
  color; it is NOT a stability contract — `--json` is.
- Truncation trailers for `--limit` go to **stderr** so stdout stays parseable.

## Semantics preserved from bash

- Repo positional is **required** everywhere, same arity as bash. `.` as the
  repo opts into inference from the cwd's git remote (only remotes on the
  configured Forgejo host are considered). No silent inference.
- Per-verb server limits are kept as defaults (`limit=50` etc.); `--limit` is
  a pure override, `--limit=0` = fetch all pages (dedupe pagination, 200-page
  cap). Internal joins (`actions tasks?limit=200`) are exempt.
- `forgejo api`: method sniffing (uppercase only), `/api/v1` prefix rules,
  `//` rejection, `-f` string / `-F` typed (`true|false`, integers) params,
  `-F key=@file`, `key[]=` accumulation, GET/HEAD params → query string,
  `--input -` stdin body override, exit **22** on HTTP error.
- `pr review` submits with `FORGEJO_REVIEW_TOKEN` when configured (only the
  review POST; surrounding GETs use the primary token), including the
  404-masquerade hint for invisible repos.
- Load-bearing server workarounds ported line-by-line from bash (issue edit
  label PUT + `--labels=` clear-all, pr ready WIP-prefix strip, wiki edit
  content_base64 re-send, org-then-repo label resolution, branch protect
  GET-then-PATCH/POST, review comment line→new_position, actions run→task
  join via `index_in_repo`, actions logs zstd shard math + API fallback).

## Deliberate changes (breaking, documented)

- Unknown flags are errors (bash silently ignored them). Exit 2.
- Exit codes: 0 ok, 1 API/runtime error, 2 usage, 3 auth (401/403);
  `api` keeps 22.
- **All** destructive verbs uniformly require `--yes` or an interactive
  typed-name confirmation (bash left 11 delete verbs unguarded and
  `repo transfer` un-scriptable). Non-TTY stdin without `--yes` errors
  immediately — never auto-confirms from piped data.
- `auth login` no longer offers to store the account password; supports
  `--scopes` (still defaults to `all`) and `X-Forgejo-OTP` for 2FA accounts.
- `403` responses include the token's actual scopes in the error hint.
- Retries: GET/HEAD on 429/5xx, PUT/PATCH/DELETE on 429 only, POST never.
  `Retry-After` honored (capped 30s).
- `forgejo api` refuses to send the token to absolute URLs on other hosts
  (bash exfiltrated it to any URL).

## Security requirements (from the audit of the bash version)

- Secrets never in argv: in-process HTTP client only.
- Config parsed as data (never sourced/executed); regular file, owner=uid,
  mode 0600, symlinks rejected; bash `$'…'` quoting detected with a
  migration error.
- Credentials origin-bound (incl. attachment downloads; Go strips
  Authorization on cross-host redirects).
- Every path segment escaped (`PathEscape`) or validated (`IDArg`,
  `ValidRepo` rejects `.`/`..`).
- `FORGEJO_TOKEN` env > `FORGEJO_TOKEN_COMMAND` > config file; token never
  written to logs (`--verbose` logs method/URL/status only).
- Response bodies stay in memory; no temp files.
- Plain `http://` warns on stderr.

## Testing

- httptest unit tests: client (auth header, error mapping, pagination
  dedupe+sort, retry policy incl. POST-never, dry-run blocking, redaction),
  config (perms, owner, quoting, token command), review-token routing.
- Live acceptance: throwaway repo only, allowlisted repo-scoped verbs;
  instance-scoped verbs (`notification read --all`, user/org/admin mutations,
  `repo transfer`, token deletes) are httptest-only.
- Parity harness (`scripts/parity.sh`): bash vs Go `--json` on read verbs,
  normalized with `jq -S`; exit codes asserted.

## gh-compatibility additions (Go-only, not in the bash reference)

Additive surface modeled on the gh CLI so gh-trained agents transfer; none
of it changes existing grammar, output, or exit codes:

- `-R`/`--repo owner/repo` accepted anywhere on the command line as an
  alternative to the repo positional (argv is rewritten before parsing; the
  `api` command's raw args are never rewritten). `-R .` infers like the
  positional dot.
- `pr checkout <repo> <n> [--branch=NAME] [--detach]` — fetches
  `refs/pull/<n>/head` from the git remote matching the configured host.
- `pr edit` / `issue edit`: `--add-labels/--remove-labels`,
  `--add-assignees/--remove-assignees` incremental flags (mutually
  exclusive with the wholesale flags); `pr edit
  --add-reviewers/--remove-reviewers` for review requests.
- `release download <repo> <tag> [--pattern=GLOB] [--output=DIR]`.

## Dry-run

`--dry-run` is enforced in the transport: GET/HEAD execute (resolution reads
still occur), any other method prints `DRY-RUN: METHOD URL` + body to stderr
and aborts the command successfully. Read-modify-write verbs therefore print
the real body they would have sent.
