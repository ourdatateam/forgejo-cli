# forgejo repo

Manage repositories

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

## forgejo repo archive

Use: `forgejo repo archive <owner/repo>`

Archive a repository by PATCHing archived=true. The repo positional is required; pass '.' to infer owner/repo from the current git remote. This destructive command requires --yes or an interactive typed-name confirmation.

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| `--yes` | `bool` | `false` | skip the delete confirmation prompt |

## forgejo repo collaborator

Use: `forgejo repo collaborator <list|add|remove>`

Manage repository collaborators

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| _None_ |  |  |  |

## forgejo repo collaborator add

Use: `forgejo repo collaborator add <owner/repo> --user=X [--permission=read|write|admin]`

Add a repository collaborator.

The repo positional is required; pass '.' to infer owner/repo from the current git remote. --user is required. --permission defaults to write and is sent to the server exactly as provided.

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| `--permission` | `string` | `write` | permission to grant (read, write, admin) |
| `--user` | `string` | `""` | collaborator username (required) |

## forgejo repo collaborator list

Use: `forgejo repo collaborator list <owner/repo>`

List repository collaborators. The repo positional is required; pass '.' to infer owner/repo from the current git remote.

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| _None_ |  |  |  |

## forgejo repo collaborator remove

Use: `forgejo repo collaborator remove <owner/repo> --user=X`

Remove a repository collaborator. The repo positional is required; pass '.' to infer owner/repo from the current git remote. --user is required. This destructive command requires --yes or an interactive typed-name confirmation.

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| `--user` | `string` | `""` | collaborator username to remove (required) |
| `--yes` | `bool` | `false` | skip the delete confirmation prompt |

## forgejo repo create

Use: `forgejo repo create <name> [--org=X] [--private] [--desc=X]`

Create a repository named <name>.

By default the repository is created for the authenticated user. Use --org to create it under an organization. --private makes the repository private; --desc sets the description, including an empty description when passed as --desc=.

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| `--desc` | `string` | `""` | repository description |
| `--org` | `string` | `""` | organization owner for the new repository |
| `--private` | `bool` | `false` | create a private repository |

## forgejo repo delete

Use: `forgejo repo delete <owner/repo>`

Delete a repository. The repo positional is required; pass '.' to infer owner/repo from the current git remote. This destructive command requires --yes or an interactive typed-name confirmation.

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| `--yes` | `bool` | `false` | skip the delete confirmation prompt |

## forgejo repo edit

Use: `forgejo repo edit <owner/repo> [--name=X] [--desc=X] [--private|--public]`

Edit repository metadata.

The repo positional is required; pass '.' to infer owner/repo from the current git remote. Provide at least one of --name, --desc, --private, or --public. --private and --public are mutually exclusive.

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| `--desc` | `string` | `""` | new repository description |
| `--name` | `string` | `""` | new repository name |
| `--private` | `bool` | `false` | make the repository private |
| `--public` | `bool` | `false` | make the repository public |

## forgejo repo fork

Use: `forgejo repo fork <owner/repo> [--org=X]`

Fork a repository.

The repo positional is required; pass '.' to infer owner/repo from the current git remote. Use --org to fork into an organization. A server 409 means the fork already exists; the command treats that as success and fetches the existing fork.

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| `--org` | `string` | `""` | organization to receive the fork |

## forgejo repo key

Use: `forgejo repo key <list|add|delete>`

Manage deploy keys

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| _None_ |  |  |  |

## forgejo repo key add

Use: `forgejo repo key add <owner/repo> --title=X (--key=X | --key-file=path) [--read-only]`

Add a repository deploy key.

The repo positional is required; pass '.' to infer owner/repo from the current git remote. --title is required. Provide exactly one non-empty key source: --key for the literal public key, or --key-file to read the key from a file. --read-only sends read_only=true.

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| `--key` | `string` | `""` | literal public key; mutually exclusive with --key-file |
| `--key-file` | `string` | `""` | file containing the public key; mutually exclusive with --key |
| `--read-only` | `bool` | `false` | add the key with read_only=true |
| `--title` | `string` | `""` | deploy key title (required) |

## forgejo repo key delete

Use: `forgejo repo key delete <owner/repo> <id>`

Delete a repository deploy key by numeric id. The repo positional is required; pass '.' to infer owner/repo from the current git remote. This destructive command requires --yes or an interactive typed-name confirmation.

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| `--yes` | `bool` | `false` | skip the delete confirmation prompt |

## forgejo repo key list

Use: `forgejo repo key list <owner/repo>`

List repository deploy keys. The repo positional is required; pass '.' to infer owner/repo from the current git remote.

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| _None_ |  |  |  |

## forgejo repo list

Use: `forgejo repo list [org]`

List repositories visible to the authenticated user.

With no org, this lists user repositories. Pass an org either as the optional positional or with --org; if both are supplied, --org wins like the bash CLI. The bash endpoint used limit=50, so the global --limit overrides that default and --limit=0 fetches all pages.

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| `--org` | `string` | `""` | list repositories in this organization instead of user repositories |

## forgejo repo mirror

Use: `forgejo repo mirror <sync>`

Manage repository mirrors

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| _None_ |  |  |  |

## forgejo repo mirror sync

Use: `forgejo repo mirror sync <owner/repo>`

Trigger push-mirror sync for a repository. The repo positional is required; pass '.' to infer owner/repo from the current git remote.

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| _None_ |  |  |  |

## forgejo repo tags

Use: `forgejo repo tags <list|create|delete>`

Manage repository tags

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| _None_ |  |  |  |

## forgejo repo tags create

Use: `forgejo repo tags create <owner/repo> --tag=X [--message=X] [--target=<sha|branch>]`

Create a repository tag.

The repo positional is required; pass '.' to infer owner/repo from the current git remote. --tag is required. --message and --target are included only when non-empty, matching the bash jq body.

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| `--message` | `string` | `""` | annotated tag message |
| `--tag` | `string` | `""` | tag name to create (required) |
| `--target` | `string` | `""` | target commit SHA or branch |

## forgejo repo tags delete

Use: `forgejo repo tags delete <owner/repo> <tag>`

Delete a repository tag. The repo positional is required; pass '.' to infer owner/repo from the current git remote. This destructive command requires --yes or an interactive typed-name confirmation.

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| `--yes` | `bool` | `false` | skip the delete confirmation prompt |

## forgejo repo tags list

Use: `forgejo repo tags list <owner/repo>`

List repository tags. The repo positional is required; pass '.' to infer owner/repo from the current git remote.

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| _None_ |  |  |  |

## forgejo repo topic

Use: `forgejo repo topic <list|add|remove>`

Manage repository topics

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| _None_ |  |  |  |

## forgejo repo topic add

Use: `forgejo repo topic add <owner/repo> --topics=a,b`

Add repository topics. The repo positional is required; pass '.' to infer owner/repo from the current git remote. --topics is a comma-separated list; each topic is trimmed, empty entries are skipped, and each non-empty topic is added with PUT /repos/{repo}/topics/{topic}.

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| `--topics` | `string` | `""` | comma-separated topics to add (required) |

## forgejo repo topic list

Use: `forgejo repo topic list <owner/repo>`

List repository topics. The repo positional is required; pass '.' to infer owner/repo from the current git remote. Text output prints one topic per line with no header, matching bash.

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| _None_ |  |  |  |

## forgejo repo topic remove

Use: `forgejo repo topic remove <owner/repo> --topics=a,b`

Remove repository topics. The repo positional is required; pass '.' to infer owner/repo from the current git remote. --topics is a comma-separated list; each topic is trimmed, empty entries are skipped, and each non-empty topic is removed with DELETE /repos/{repo}/topics/{topic}. This destructive command requires --yes or an interactive typed-name confirmation.

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| `--topics` | `string` | `""` | comma-separated topics to remove (required) |
| `--yes` | `bool` | `false` | skip the delete confirmation prompt |

## forgejo repo transfer

Use: `forgejo repo transfer <owner/repo> --new-owner=X`

Transfer repository ownership.

The repo positional is required; pass '.' to infer owner/repo from the current git remote. --new-owner is required. The API cannot rename during transfer; --new-name is rejected and the repository must be renamed later with repo edit. This destructive command requires --yes or an interactive typed-name confirmation.

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| `--new-owner` | `string` | `""` | new user or organization owner (required) |
| `--yes` | `bool` | `false` | skip the delete confirmation prompt |

## forgejo repo unarchive

Use: `forgejo repo unarchive <owner/repo>`

Unarchive a repository by PATCHing archived=false. The repo positional is required; pass '.' to infer owner/repo from the current git remote.

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| _None_ |  |  |  |

## forgejo repo view

Use: `forgejo repo view <owner/repo>`

View details for a repository. The repo positional is required; pass '.' to infer owner/repo from the current git remote.

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| _None_ |  |  |  |

## forgejo repo webhook

Use: `forgejo repo webhook <list|view|create|edit|delete>`

Manage repository webhooks

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| _None_ |  |  |  |

## forgejo repo webhook create

Use: `forgejo repo webhook create <owner/repo> --url=X --events=a,b [--secret=X] [--content-type=json|form] [--inactive]`

Create a forgejo-type repository webhook.

The repo positional is required; pass '.' to infer owner/repo from the current git remote. --url and --events are required. --events is a comma-separated list trimmed like the bash jq expression. --content-type defaults to json. --inactive sends active=false.

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| `--content-type` | `string` | `json` | payload content type (json or form) |
| `--events` | `string` | `""` | comma-separated webhook events (required) |
| `--inactive` | `bool` | `false` | create the webhook with active=false |
| `--secret` | `string` | `""` | webhook secret |
| `--url` | `string` | `""` | webhook target URL (required) |

## forgejo repo webhook delete

Use: `forgejo repo webhook delete <owner/repo> <id>`

Delete a repository webhook by numeric id. The repo positional is required; pass '.' to infer owner/repo from the current git remote. This destructive command requires --yes or an interactive typed-name confirmation.

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| `--yes` | `bool` | `false` | skip the delete confirmation prompt |

## forgejo repo webhook edit

Use: `forgejo repo webhook edit <owner/repo> <id> [--url=X] [--events=a,b] [--secret=X] [--content-type=X] [--active|--inactive]`

Edit a repository webhook by numeric id.

The repo positional is required; pass '.' to infer owner/repo from the current git remote. Config keys --url, --content-type, and --secret are sent as a partial config object only when at least one is supplied. --events replaces the event list. --active and --inactive are mutually exclusive. At least one editable field is required.

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| `--active` | `bool` | `false` | set active=true |
| `--content-type` | `string` | `""` | new payload content type |
| `--events` | `string` | `""` | comma-separated webhook events |
| `--inactive` | `bool` | `false` | set active=false |
| `--secret` | `string` | `""` | new webhook secret |
| `--url` | `string` | `""` | new webhook target URL |

## forgejo repo webhook list

Use: `forgejo repo webhook list <owner/repo>`

List repository webhooks. The repo positional is required; pass '.' to infer owner/repo from the current git remote.

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| _None_ |  |  |  |

## forgejo repo webhook view

Use: `forgejo repo webhook view <owner/repo> <id>`

View a repository webhook by numeric id. The repo positional is required; pass '.' to infer owner/repo from the current git remote.

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| _None_ |  |  |  |

