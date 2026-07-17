# forgejo branch

Manage branches and branch protection

## Global Flags

These inherited flags apply to commands in this group unless a command defines a local flag with the same name.

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| `--dry-run` | `bool` | `false` | print mutating requests instead of sending them |
| `--jq` | `string` | `""` | filter JSON output through a jq expression (implies --json) |
| `--json` | `bool` | `false` | output raw JSON from the server |
| `--limit` | `int` | `-1` | max items for list verbs (0 = fetch all pages; default: per-verb) |
| `--verbose` | `bool` | `false` | log requests to stderr (tokens are never logged) |
| `-R, --repo` | `string` | `""` | target repository as owner/repo (gh-style alternative to the repo positional; '.' infers from the cwd git remote) |

## forgejo branch create

Use: `forgejo branch create <owner/repo> <branch> [--from=X]`

Create a branch with POST repos/{repo}/branches. --from sets old_branch_name; omitted lets the server use the repository default branch.

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| `--from` | `string` | `""` | source branch or commit-ish (old_branch_name) |

## forgejo branch delete

Use: `forgejo branch delete <owner/repo> <branch>`

Delete a branch. Deliberately no --yes prompt: bash kept branch deletion scriptable because branches are recoverable and protected branches are rejected by the server.

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| _None_ |  |  |  |

## forgejo branch list

Use: `forgejo branch list <owner/repo>`

List branches in a repository. This matches the bash endpoint repos/{repo}/branches without adding pagination parameters.

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| _None_ |  |  |  |

## forgejo branch protect

Use: `forgejo branch protect <owner/repo> <branch> [flags]`

Apply or update branch protection idempotently. The command first GETs repos/{repo}/branch_protections/{branch}; 404 creates a protection with POST, otherwise PATCH updates only the fields requested. With no flags on create, the safe default body locks pushes and requires zero approvals. --no-push and --push-whitelist are mutually exclusive.

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| `--block-on-outdated` | `bool` | `false` | block merging outdated branches |
| `--dismiss-stale-approvals` | `bool` | `false` | dismiss stale approvals when new commits are pushed |
| `--merge-whitelist` | `string` | `""` | comma-separated users allowed to merge |
| `--no-push` | `bool` | `false` | disable direct pushes and push whitelist |
| `--push-whitelist` | `string` | `""` | comma-separated users allowed to push; mutually exclusive with --no-push |
| `--require-signed` | `bool` | `false` | require signed commits |
| `--required-approvals` | `int` | `-1` | required approval count (non-negative) |

## forgejo branch protection

Use: `forgejo branch protection <list>`

Manage branch protection records

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| _None_ |  |  |  |

## forgejo branch protection list

Use: `forgejo branch protection list <owner/repo>`

List branch protection records for a repository. This Go-port verb fetches repos/{repo}/branch_protections with a default list limit of 50; --limit=0 fetches all pages.

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| _None_ |  |  |  |

## forgejo branch unprotect

Use: `forgejo branch unprotect <owner/repo> <branch>`

Remove branch protection. The command is idempotent: 404 prints that the branch was not protected and exits successfully.

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| _None_ |  |  |  |

## forgejo branch view

Use: `forgejo branch view <owner/repo> <branch>`

View branch details plus branch protection. Protection is checked with DoStatus; 404 renders as not protected instead of an error.

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| _None_ |  |  |  |

