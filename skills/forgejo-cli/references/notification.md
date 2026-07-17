# forgejo notification

List and mark notifications

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

## forgejo notification list

Use: `forgejo notification list [--all] [--status=unread,read,pinned]`

List notifications (unread by default)

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| `--all` | `bool` | `false` | include read notifications |
| `--limit` | `int` | `-1` | server-side result limit |
| `--status` | `string` | `""` | comma-separated status filter (unread,read,pinned) |

## forgejo notification read

Use: `forgejo notification read <id> | --all`

Mark a notification thread (or everything) as read

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| `--all` | `bool` | `false` | mark every unread notification as read |

