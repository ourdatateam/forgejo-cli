# forgejo release

Manage releases and release assets

## Global Flags

These inherited flags apply to commands in this group unless a command defines a local flag with the same name.

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| `--dry-run` | `bool` | `false` | print mutating requests instead of sending them |
| `--jq` | `string` | `""` | filter JSON output through a jq expression (implies --json) |
| `--json` | `bool` | `false` | output raw JSON from the server |
| `--limit` | `int` | `-1` | max items for list verbs (0 = fetch all pages; default: per-verb) |
| `--verbose` | `bool` | `false` | log requests to stderr (tokens are never logged) |

## forgejo release asset

Use: `forgejo release asset <list|download|delete>`

Manage release assets

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| _None_ |  |  |  |

## forgejo release asset delete

Use: `forgejo release asset delete <owner/repo> <tag> <asset_id> [--yes]`

Delete a release asset by numeric id. This is destructive and requires --yes or typed confirmation of #<asset_id>.

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| `--yes` | `bool` | `false` | skip the delete confirmation prompt |

## forgejo release asset download

Use: `forgejo release asset download <owner/repo> <tag> <asset_id> [--output=DIR]`

Download a release asset by id. --output is a directory (default: .); the remote filename is reduced to a safe basename and the destination is kept inside that directory. The token is attached to the asset URL only when its origin matches FORGEJO_URL.

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| `--output` | `string` | `.` | directory to write the downloaded asset into |

## forgejo release asset list

Use: `forgejo release asset list <owner/repo> <tag>`

List assets attached to a release. The tag is resolved to a release id before fetching repos/{repo}/releases/{id}/assets.

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| _None_ |  |  |  |

## forgejo release create

Use: `forgejo release create <owner/repo> --tag=X --title=X [--body=X|--body-file=path] [--draft] [--prerelease]`

Create a release. --tag and --title are required. --body supplies inline release notes; --body=- or --body-file=- reads notes from stdin.

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| `--body` | `string` | `""` | body text ('-' reads stdin) |
| `--body-file` | `string` | `""` | read body from a file |
| `--draft` | `bool` | `false` | create the release as a draft |
| `--prerelease` | `bool` | `false` | mark the release as a prerelease |
| `--tag` | `string` | `""` | release tag name (required) |
| `--title` | `string` | `""` | release title (required) |

## forgejo release delete

Use: `forgejo release delete <owner/repo> <tag> [--yes]`

Delete a release by tag. This is destructive and requires --yes or a typed tag confirmation.

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| `--yes` | `bool` | `false` | skip the delete confirmation prompt |

## forgejo release edit

Use: `forgejo release edit <owner/repo> <tag> [--title=X] [--body=X|--body-file=path] [--draft|--no-draft] [--prerelease|--no-prerelease]`

Edit a release by tag. Supply at least one of --title, --body, --body-file, --draft/--no-draft, or --prerelease/--no-prerelease. --draft and --no-draft are mutually exclusive, as are --prerelease and --no-prerelease.

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| `--body` | `string` | `""` | body text ('-' reads stdin) |
| `--body-file` | `string` | `""` | read body from a file |
| `--draft` | `bool` | `false` | mark the release as a draft |
| `--no-draft` | `bool` | `false` | mark the release as non-draft |
| `--no-prerelease` | `bool` | `false` | mark the release as a normal release |
| `--prerelease` | `bool` | `false` | mark the release as a prerelease |
| `--title` | `string` | `""` | new release title |

## forgejo release list

Use: `forgejo release list <owner/repo>`

List releases for a repository. The bash version fetched repos/{repo}/releases?limit=20; --limit overrides that default and --limit=0 fetches all pages.

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| _None_ |  |  |  |

## forgejo release upload-asset

Use: `forgejo release upload-asset <owner/repo> <tag> <file>...`

Upload one or more asset files to a release. Every path is validated as a regular file before the first upload, so a typo does not leave a partial upload set.

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| _None_ |  |  |  |

## forgejo release view

Use: `forgejo release view <owner/repo> <tag>`

View a release by tag. The tag is resolved to a release id with GET repos/{repo}/releases/tags/{tag}; if that 404s, releases are scanned as a fallback.

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| _None_ |  |  |  |

