# forgejo wiki

Manage repository wiki pages.

Content sources for create/edit are, in precedence order:
  --content=TEXT    inline content
  --file=PATH       read content from a file
  --file=-          read content from stdin

There is no implicit stdin; piping content requires --file=-. Page names may contain spaces and are escaped as one path segment. wiki edit requires --title and/or a content source; --message sets the commit message and omitted uses the server default.

## Global Flags

These inherited flags apply to commands in this group unless a command defines a local flag with the same name.

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| `--dry-run` | `bool` | `false` | print mutating requests instead of sending them |
| `--jq` | `string` | `""` | filter JSON output through a jq expression (implies --json) |
| `--json` | `bool` | `false` | output raw JSON from the server |
| `--limit` | `int` | `-1` | max items for list verbs (0 = fetch all pages; default: per-verb) |
| `--verbose` | `bool` | `false` | log requests to stderr (tokens are never logged) |

## forgejo wiki create

Use: `forgejo wiki create <owner/repo> --title=X [--content=X|--file=path|--file=-] [--message=X]`

Create a wiki page. --title is required. Content must be supplied with --content, --file=PATH, or --file=-; --content takes precedence over --file, and --content='' creates an empty page.

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| `--content` | `string` | `""` | inline wiki content; takes precedence over --file and may be empty |
| `--file` | `string` | `""` | read wiki content from a file; '-' reads stdin |
| `--message` | `string` | `""` | commit message for the wiki change |
| `--title` | `string` | `""` | wiki page title (required) |

## forgejo wiki delete

Use: `forgejo wiki delete <owner/repo> <page> [--yes]`

Delete a wiki page. This is destructive and requires --yes or a typed page-title confirmation.

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| `--yes` | `bool` | `false` | skip the delete confirmation prompt |

## forgejo wiki edit

Use: `forgejo wiki edit <owner/repo> <page> [--title=X] [--content=X|--file=path|--file=-] [--message=X]`

Edit or rename a wiki page. Supply --title and/or a content source. If --title is omitted, the title is sent as the current page name so Forgejo does not rename it to unnamed. If content is omitted, the current page is fetched and content_base64 is re-sent because Forgejo otherwise blanks the page on PATCH.

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| `--content` | `string` | `""` | inline wiki content; takes precedence over --file and may be empty |
| `--file` | `string` | `""` | read wiki content from a file; '-' reads stdin |
| `--message` | `string` | `""` | commit message for the wiki change |
| `--title` | `string` | `""` | new wiki page title |

## forgejo wiki list

Use: `forgejo wiki list <owner/repo>`

List wiki pages for a repository. The bash version fetched repos/{repo}/wiki/pages?limit=50; --limit overrides that default and --limit=0 fetches all pages.

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| _None_ |  |  |  |

## forgejo wiki revisions

Use: `forgejo wiki revisions <owner/repo> <page>`

Show a wiki page's commit history. --json emits the raw server object; text output renders the commits array.

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| _None_ |  |  |  |

## forgejo wiki view

Use: `forgejo wiki view <owner/repo> <page>`

View a wiki page. Text output prints page metadata followed by decoded content_base64 verbatim; --json emits the raw server object.

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| _None_ |  |  |  |

