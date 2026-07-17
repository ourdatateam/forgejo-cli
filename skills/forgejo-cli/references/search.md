# forgejo search

Search repos, users, and issues instance-wide

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

## forgejo search issues

Use: `forgejo search issues --query=X`

Search issues and pull requests instance-wide

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| `--limit` | `int` | `-1` | server-side result limit |
| `--owner` | `string` | `""` | restrict to an owner |
| `--query` | `string` | `""` | search query (required) |
| `--state` | `string` | `""` | open\|closed\|all |
| `--type` | `string` | `""` | issue\|pr |

## forgejo search repos

Use: `forgejo search repos --query=X`

Search repositories

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| `--limit` | `int` | `-1` | server-side result limit |
| `--owner` | `string` | `""` | restrict to a user/org (resolved to uid) |
| `--query` | `string` | `""` | search query (required) |

## forgejo search users

Use: `forgejo search users --query=X`

Search users

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| `--limit` | `int` | `-1` | server-side result limit |
| `--query` | `string` | `""` | search query (required) |

