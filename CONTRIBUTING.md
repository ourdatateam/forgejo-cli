# Contributing to forgejo-cli

Solo project, but the workflow is real — `main` is the installed CLI for
everyone using it, so it has to stay working.

## Branching

- `main` — always installable. Server-protected (see below).
- `feature/<short-kebab-name>` — all new work, including trivial fixes.
  Examples: `feature/pr-review-cmd`, `feature/labels-bulk`,
  `feature/fix-readme-typo`.

No `develop` branch, no hotfix lane. Every change is a feature branch.

## Workflow

```bash
# Start
git checkout main && git pull
git checkout -b feature/my-thing

# Work, test live (the installed CLI runs whatever's checked out)
$EDITOR forgejo
forgejo --help            # runs your feature-branch code via the symlink

# Push
git push -u origin feature/my-thing

# Open and merge the PR (dogfood this CLI)
forgejo pr create OWNER/REPO \
  --head=feature/my-thing --base=main \
  --title="..." --body="..."

forgejo pr merge OWNER/REPO <number> --method=squash

# Back to stable
git checkout main && git pull
# -D (not -d): squash merges leave the feature branch tip unreachable from main,
# so git's "is it fully merged" check refuses -d. The work IS merged — just squashed.
git branch -D feature/my-thing
git push origin --delete feature/my-thing
```

If you need the CLI for unrelated real work while a feature branch is
checked out, `git checkout main`, do the work, then check the branch back
out. Or escalate to a git worktree if it gets annoying.

## Branch protection on `main`

These can be applied via the `forgejo` CLI (recommended for repeatability):

```bash
forgejo branch protect OWNER/REPO main \
  --no-push \
  --merge-whitelist=<your-username> \
  --required-approvals=0 \
  --dismiss-stale-approvals
```

Or manually in the Forgejo web UI at `Settings → Branches → Branch Protection
Rules`. The CLI form is idempotent — re-running it converges to the stated
state. Documented here so settings can be re-applied if the repo is
recreated/migrated, or audited against drift.

| Setting | Value |
|---|---|
| Branch name pattern | `main` |
| Enable Push | off |
| Whitelist Restricted Push | (empty) |
| Enable Merge Whitelist | on, contains only the repo owner's Forgejo username |
| Require Pull Request | on |
| Approvals required | 0 |
| Dismiss stale approvals on new commits | on |

Effect: `git push origin main` is rejected by the server from any branch.
PRs are the only path to `main`. Only the whitelisted user can merge.
New commits to a PR branch dismiss any prior approvals.

The "dismiss stale approvals" setting is defensive — with approvals required
at 0, nothing strictly needs dismissing, but if you ever self-approve a PR
in the UI before merging, this ensures that approval doesn't survive a
follow-up push.

## Hooks

A `shellcheck` pre-commit hook lives in `hooks/`. To activate it after
clone:

```bash
sudo apt install shellcheck
git config core.hooksPath hooks
```

The hook lints `forgejo` and `hooks/pre-commit` at default severity.
Currently both are clean — keep them that way.

If you genuinely need to commit despite a shellcheck failure (e.g. mid-
refactor, fixing in next commit), `git commit --no-verify` bypasses the
hook. Use sparingly.
