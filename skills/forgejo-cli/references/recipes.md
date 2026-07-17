# Recipes — multi-step workflows

Hand-written; the per-verb files in this directory are generated. Everything
here assumes `--json`/`--jq` for parsing and `--yes` for non-interactive
destructive steps.

## Open a PR from the current branch

```bash
git push -u origin "$(git branch --show-current)"
forgejo pr create . --title="feat: thing" --base=main \
  --head="$(git branch --show-current)" --body=-  <<'EOF'
What changed and why.
EOF
```

## Review a PR end to end

```bash
forgejo pr view owner/repo 42            # full conversation, reviews inline
forgejo pr diff owner/repo 42            # raw diff to read or pipe
forgejo pr review owner/repo 42 --request-changes --body=- <<'EOF'
Blocking: X breaks Y.
EOF
# after fixes:
forgejo pr review owner/repo 42 --approve
forgejo pr merge owner/repo 42 --method=squash
```

## Issue triage sweep

```bash
forgejo issue list owner/repo --state=open --limit=0 --jq \
  '.[] | {n: .number, t: .title, l: [.labels[].name]}'
forgejo issue label owner/repo add 17 --labels=bug,p1
forgejo issue assign owner/repo 17 --assignees=someone
forgejo issue milestone owner/repo set 17 --milestone=3
```

## Cut a release

```bash
forgejo repo tags create owner/repo --tag=v1.4.0 --target=main
forgejo release create owner/repo --tag=v1.4.0 --title="v1.4.0" --notes=- <<'EOF'
## Changes
- ...
EOF
forgejo release asset upload owner/repo v1.4.0 --file=dist/thing_linux_amd64.tar.gz
```

## Throwaway acceptance repo (for testing anything mutating)

```bash
forgejo api POST /orgs/myorg/repos -f name=scratch-$$ -F auto_init=true
# ... exercise verbs against myorg/scratch-$$ ...
forgejo repo delete "myorg/scratch-$$" --yes
```

## Watch CI on a push

```bash
forgejo actions list owner/repo --limit=1 --jq '.[0].id'
forgejo actions watch owner/repo <run-id>     # exits non-zero if the run fails
```

## Protect a branch (idempotent — safe to re-run)

```bash
forgejo branch protect owner/repo main --require-pr --approvals=1
```
