# forgejo-cli

CLI for the [Forgejo](https://forgejo.org/) REST API. Manages repos, issues,
PRs, releases, users, orgs, actions, and admin operations on a Forgejo
instance. Built because Forgejo doesn't expose GraphQL, so `gh` doesn't work
against it.

Two implementations live in this repo during the transition:

- **Go CLI** (`cmd/forgejo`, `internal/`) — the current implementation.
  Single static binary, no runtime dependencies. Build with `make build`
  (binary lands in `bin/forgejo`). Design record and deliberate behavior
  changes: [docs/PORT.md](docs/PORT.md). Agent-facing usage docs ship as a
  skill in [skills/forgejo-cli/](skills/forgejo-cli/) (`make docs`
  regenerates the per-group references from the command tree).
- **Bash script** (`./forgejo`) — the original single-file implementation,
  kept as the behavioral reference. `scripts/parity.sh` diffs the two on
  read verbs; `scripts/acceptance.sh` exercises the Go binary against a
  throwaway repo it creates and deletes.

The Go CLI needs `jq` for nothing — `--jq` is built in (gojq). Filter any
output: `forgejo issue list o/r --jq '.[].number'`.

## Install

### Go binary (recommended)

Grab the tarball for your platform from the repo's releases page
(`darwin`/`linux`, `amd64`/`arm64` — static binaries, no runtime
dependencies), verify against `SHA256SUMS`, and drop it on your `PATH`:

```bash
tar -xzf forgejo_<version>_<os>_<arch>.tar.gz
install -m 755 forgejo ~/.local/bin/forgejo
forgejo --version
```

Or build from source (Go ≥ 1.24):

```bash
git clone https://github.com/ourdatateam/forgejo-cli.git && cd forgejo-cli
make build                    # → bin/forgejo for this machine
make release                  # → dist/ tarballs for darwin/linux, amd64/arm64
```

Cross-compiling needs nothing beyond the Go toolchain: the dependency tree
is pure Go, so `make release` produces all four platforms from any host
(`CGO_ENABLED=0`, version stamped from `git describe`). See
[docs/RELEASING.md](docs/RELEASING.md) for cutting a release.

### Bash script (reference implementation)

The original single-file script still works and stays in-tree during the
transition. It installs as a **symlink** into the working copy — this lets
you test feature branches by checking them out (see
[CONTRIBUTING.md](CONTRIBUTING.md)):

```bash
REPO="$HOME/projects/forgejo-cli"   # adjust to your preferred clone location
mkdir -p ~/.local/bin "$(dirname "$REPO")"
git clone https://github.com/ourdatateam/forgejo-cli.git "$REPO"
ln -sf "$REPO/forgejo" ~/.local/bin/forgejo
```

Updating the symlink install: `cd "$REPO" && git checkout main && git pull`.
Plain `git pull` on a feature branch updates the feature, not the installed
stable — be deliberate.

## Configure

The script reads credentials from `~/.config/forgejo-cli/config`:

```bash
mkdir -p ~/.config/forgejo-cli
cat > ~/.config/forgejo-cli/config <<'EOF'
FORGEJO_URL=https://forgejo.example.com
FORGEJO_TOKEN=your-token-here
EOF
chmod 600 ~/.config/forgejo-cli/config
```

Generate the token at `<FORGEJO_URL>/user/settings/applications`. Scopes
needed depend on usage — `repo`, `issue`, `user`, `organization` cover the
implemented commands.

## Usage

Run without arguments to see the full command list. Common operations:

```bash
forgejo repo list OWNER
forgejo repo view OWNER/REPO
forgejo issue list OWNER/REPO
forgejo issue view OWNER/REPO <number>
forgejo issue create OWNER/REPO --title="..." --body="..."
forgejo issue comment OWNER/REPO <number> --body="..."
forgejo issue label OWNER/REPO add <number> --labels=ready-to-test
forgejo branch list OWNER/REPO
forgejo branch view OWNER/REPO <branch>
forgejo branch protect OWNER/REPO <branch> --no-push --merge-whitelist=user1
```

**Issue verbs (close/assign/search/milestones):**

```bash
# Close / reopen / assign
forgejo issue close OWNER/REPO 42
forgejo issue reopen OWNER/REPO 42
forgejo issue assign OWNER/REPO 42 --users=alice,bob
forgejo issue unassign OWNER/REPO 42 --users=alice

# Search across an org
forgejo issue search --owner=OWNER --state=open --labels=bug --query=auth

# Milestones
forgejo issue milestone list OWNER/REPO
forgejo issue milestone create OWNER/REPO --title=v2 --due=2026-06-30
forgejo issue milestone edit OWNER/REPO 3 --state=closed
forgejo issue milestone delete OWNER/REPO 3
forgejo issue milestone set OWNER/REPO 42 --milestone=v2
forgejo issue milestone set OWNER/REPO 42 --milestone=0    # clear
```

**PR verbs (close/edit/review/checks/diff/files/comment/ready/requested-reviewers):**

```bash
# PR lifecycle
forgejo pr close OWNER/REPO 17
forgejo pr edit OWNER/REPO 17 --title="New title" --base=develop
forgejo pr ready OWNER/REPO 17
forgejo pr comment OWNER/REPO 17 --body="LGTM"

# PR review
forgejo pr review OWNER/REPO 17 --approve
forgejo pr review OWNER/REPO 17 --request-changes --body="See comments"
forgejo pr review OWNER/REPO 17 --comment --body="One nit"
forgejo pr review list OWNER/REPO 17
forgejo pr review dismiss OWNER/REPO 17 42 --message="Stale review"

# PR inspection
forgejo pr diff OWNER/REPO 17 | less
forgejo pr files OWNER/REPO 17
forgejo pr checks OWNER/REPO 17

# Request reviewers
forgejo pr requested-reviewers add OWNER/REPO 17 --users=alice,bob
forgejo pr requested-reviewers remove OWNER/REPO 17 --users=bob
```

**Repo verbs (fork/transfer/archive/tags/collaborators/webhooks/keys/topics/mirror):**

```bash
# Fork / transfer / archive
forgejo repo fork OWNER/REPO [--org=X]          # idempotent (ignores 409)
forgejo repo transfer OWNER/REPO --new-owner=X [--new-name=X]   # confirms by typing repo name
forgejo repo archive OWNER/REPO
forgejo repo unarchive OWNER/REPO

# Tags
forgejo repo tags list OWNER/REPO
forgejo repo tags create OWNER/REPO --tag=v1.0 [--message="..."] [--target=main]
forgejo repo tags delete OWNER/REPO <tag>

# Collaborators
forgejo repo collaborator list OWNER/REPO
forgejo repo collaborator add OWNER/REPO --user=alice [--permission=write]
forgejo repo collaborator remove OWNER/REPO --user=alice

# Webhooks (Forgejo-type)
forgejo repo webhook list OWNER/REPO
forgejo repo webhook view OWNER/REPO <id>
forgejo repo webhook create OWNER/REPO --url=https://... --events=push,pull_request [--secret=X] [--content-type=json|form] [--inactive]
forgejo repo webhook edit OWNER/REPO <id> [--url=...] [--events=...] [--active|--inactive]
forgejo repo webhook delete OWNER/REPO <id>

# Deploy keys
forgejo repo key list OWNER/REPO
forgejo repo key add OWNER/REPO --title=ci --key-file=~/.ssh/id_ed25519.pub [--read-only]
forgejo repo key delete OWNER/REPO <id>

# Topics
forgejo repo topic list OWNER/REPO
forgejo repo topic add OWNER/REPO --topics=cli,bash
forgejo repo topic remove OWNER/REPO --topics=cli,bash

# Mirror
forgejo repo mirror sync OWNER/REPO             # triggers push-mirror sync
```

**Release verbs (view/edit/delete/upload-asset, asset list/delete):**

```bash
forgejo release list OWNER/REPO
forgejo release create OWNER/REPO --tag=v1.0 --title="v1.0" [--body=...] [--draft] [--prerelease]
forgejo release view OWNER/REPO v1.0
forgejo release edit OWNER/REPO v1.0 [--title=...] [--body=...] [--draft|--no-draft] [--prerelease|--no-prerelease]
forgejo release delete OWNER/REPO v1.0 [--yes]

# Assets
forgejo release upload-asset OWNER/REPO v1.0 ./dist/app.tar.gz ./dist/app.tar.gz.sig
forgejo release asset list OWNER/REPO v1.0
forgejo release asset delete OWNER/REPO v1.0 <asset_id> [--yes]
```

**Actions verbs (runners, workflow list/dispatch, secrets, variables):**

```bash
# Workflow runs (already shown above): list / view / watch / logs

# Logs from disk — useful only when the CLI is run on the Forgejo host.
# Set FORGEJO_LOG_PATH in ~/.config/forgejo-cli/config, e.g.:
#   FORGEJO_LOG_PATH=/srv/forgejo/data/gitea/actions_log
# When set, `actions logs` zstd-decompresses each task's stored log instead
# of just printing a browser link. Requires the `zstd` binary.

# Runners (scope inferred: OWNER/REPO = repo, OWNER = org, --admin = instance)
forgejo actions runner list OWNER/REPO
forgejo actions runner list OWNER
forgejo actions runner list --admin
forgejo actions runner register OWNER              # prints registration token
forgejo actions runner delete OWNER 7 --yes

# Workflows
forgejo actions workflow list OWNER/REPO         # reads .forgejo/.gitea/.github workflows dir
forgejo actions workflow dispatch OWNER/REPO --workflow=ci.yml --ref=main --input=env=prod

# Secrets (repo or org; values write-only)
forgejo actions secret list OWNER/REPO
forgejo actions secret set OWNER/REPO --name=DEPLOY_KEY --value="..."
forgejo actions secret delete OWNER/REPO DEPLOY_KEY --yes

# Variables (repo or org; values readable)
forgejo actions variable list OWNER/REPO
forgejo actions variable set OWNER/REPO --name=REGION --value=ap-south-1
forgejo actions variable delete OWNER/REPO REGION --yes
```

Not yet implemented: `actions rerun` / `actions cancel`. Forgejo 15.0.2 /
Gitea 1.22 does not expose these — only `GET` on `/actions/runs/{run_id}`.
Use the web UI's Re-run / Cancel buttons.

**User + Org verbs (members, teams, keys, GPG, tokens):**

```bash
# Org membership (add is via team membership in Forgejo)
forgejo org member list OWNER
forgejo org member remove OWNER --user=alice

# Org teams (team identifier accepts name OR numeric id)
forgejo org team list OWNER
forgejo org team view OWNER Owners
forgejo org team create OWNER --name=Reviewers --description="PR review crew" --permission=write
forgejo org team edit OWNER Reviewers --permission=admin
forgejo org team delete OWNER Reviewers --yes
forgejo org team member list OWNER Reviewers
forgejo org team member add OWNER Reviewers --user=alice
forgejo org team member remove OWNER Reviewers --user=alice
forgejo org team repo list OWNER Reviewers
forgejo org team repo add OWNER Reviewers --repo=OWNER/some-repo
forgejo org team repo remove OWNER Reviewers --repo=OWNER/some-repo

# SSH keys (--self for the configured token's user; <username> for admin-on-behalf)
forgejo user key list --self
forgejo user key list alice
forgejo user key add --self --title=laptop --key=@~/.ssh/id_ed25519.pub
forgejo user key add alice --title=ci --key="ssh-ed25519 AAAA..."   # admin only
forgejo user key delete --self 42 --yes

# GPG keys (self-only writes; reads work for any user)
forgejo user gpg list --self
forgejo user gpg add --self --armored=@./mykey.asc
forgejo user gpg delete --self 7 --yes

# Access tokens (username always required; SHA1 shown ONCE at creation)
forgejo user token list alice
forgejo user token create alice --name=ci --scopes=read:repo,write:issue
forgejo user token delete alice ci --yes
```

Notes:
- `org member add` is not exposed by the Forgejo API. Add users to an org
  by adding them to one of its teams.
- `user gpg add/delete` for another user is not exposed by the Forgejo
  API. Self-add only via `--self`.
- `user token` verbs require **HTTP Basic Auth** (Forgejo refuses bearer
  tokens on these endpoints). Set `FORGEJO_PASSWORD=your-account-password`
  in `~/.config/forgejo-cli/config` to enable them. You can only manage
  your own tokens — the `<user>` argument must match the configured
  account.

**Admin verbs (cron, config + gap notes):**

```bash
# Cron tasks
forgejo admin cron list
forgejo admin cron run synchronize_repo_milestones

# Server config (aggregates 4 /settings endpoints)
forgejo admin config view
forgejo admin config view --json | jq .repository
```

Not exposed by the Forgejo 15.0.2 / Gitea 1.22 API — these exit with a
pointer to the web UI:

- `admin queue list|view|pause|resume` — use `<FORGEJO_URL>/-/admin/monitor/queue`
- `admin stats` — use `<FORGEJO_URL>/-/admin`
- `admin notice list|delete` — use `<FORGEJO_URL>/-/admin/notices`

**Misc verbs (notifications, search, auth status):**

```bash
# Whoami
forgejo auth status

# Notifications
forgejo notification list                                # default: unread + pinned
forgejo notification list --all --status=read            # include already-read
forgejo notification list --status=unread,pinned         # multi-value status filter
forgejo notification read 5299                           # mark single thread as read
forgejo notification read --all                          # mark all unread as read

# Search (instance-wide)
forgejo search repos --query=forgejo-cli [--owner=OWNER]
forgejo search users --query=alice
forgejo search issues --query=hook --type=issue --state=closed [--owner=OWNER]
forgejo search issues --query=migration --type=pr
```

Notes:
- `auth status` shows `Scopes: not exposed by Forgejo API` because the
  Forgejo REST API does not surface scopes on `/user`.
- `search repos --owner=X` resolves `X` to its numeric uid via
  `/users/{owner}`; an unknown owner exits with `unknown owner '...'`.

**Generic API passthrough (escape hatch for anything not yet wrapped):**

```bash
forgejo api /repos/OWNER/REPO                          # GET (default)
forgejo api POST /repos/OWNER/REPO/issues -f title=...
echo '{"...":"..."}' | forgejo api POST /... --input -     # body from stdin
```

All commands support `--json` for raw API output.

## Dependencies

`bash`, `curl`, `jq`. The script checks for these at startup and exits with
an install hint if missing. `zstd` is additionally needed only if you set
`FORGEJO_LOG_PATH` to read action logs from disk.

## License

MIT. See [LICENSE](LICENSE).

## Contributing

PRs welcome. See [CONTRIBUTING.md](CONTRIBUTING.md) for the branching and
release workflow. `shellcheck` (dev only) is required for the pre-commit
hook; setup is in [CONTRIBUTING.md#hooks](CONTRIBUTING.md#hooks).
