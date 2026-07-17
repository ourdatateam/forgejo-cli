# forgejo org

Manage organizations through the same endpoints as the bash CLI.

Organization members are conferred by team membership in Forgejo; org member add is therefore an explicit usage error. Team commands accept a numeric team id or resolve an exact team name within the organization before calling teams/{id}.

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

## forgejo org create

Use: `forgejo org create <name> [--desc=X] [--visibility=public|private]`

Create an organization through orgs. The name positional is required. --desc defaults to an empty string and --visibility defaults to public, matching the bash request body.

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| `--desc` | `string` | `""` | organization description |
| `--visibility` | `string` | `public` | organization visibility to send (public or private) |

## forgejo org delete

Use: `forgejo org delete <name> [--yes]`

Delete an organization through orgs/{name}. This destructive command requires --yes or an interactive typed-name confirmation.

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| `--yes` | `bool` | `false` | skip the delete confirmation prompt |

## forgejo org edit

Use: `forgejo org edit <name> [--desc=X] [--visibility=public|private]`

Edit an organization through orgs/{name}. --desc and --visibility are optional; when neither is supplied the bash command still sends an empty JSON object.

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| `--desc` | `string` | `""` | set the organization description |
| `--visibility` | `string` | `""` | set organization visibility (public or private) |

## forgejo org list

Use: `forgejo org list`

List organizations through admin/orgs. The bash command fetched admin/orgs?limit=50; the global --limit flag overrides that page size, and --limit=0 fetches all pages.

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| _None_ |  |  |  |

## forgejo org member

Use: `forgejo org member <list|add|remove>`

List or remove organization members. Forgejo grants organization membership through teams, so member add is intentionally rejected and points to org team member add.

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| _None_ |  |  |  |

## forgejo org member add

Use: `forgejo org member add [<org>] [--user=<user>]`

Organization members are conferred by team membership in Forgejo. This command always returns a usage error; use forgejo org team member add <org> <team> --user=<user> instead.

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| `--user` | `string` | `""` | username attempted for direct org membership; use org team member add instead |

## forgejo org member list

Use: `forgejo org member list <org>`

List organization members through orgs/{org}/members. The bash command fetched ?limit=50; the global --limit flag overrides that page size, and --limit=0 fetches all pages.

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| _None_ |  |  |  |

## forgejo org member remove

Use: `forgejo org member remove <org> --user=<user> [--yes]`

Remove a user from an organization through orgs/{org}/members/{user}. This destructive remove command requires --yes or an interactive typed-name confirmation.

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| `--user` | `string` | `""` | username to remove from the organization (required) |
| `--yes` | `bool` | `false` | skip the delete confirmation prompt |

## forgejo org team

Use: `forgejo org team <list|view|create|edit|delete|member|repo>`

Manage teams. Team arguments can be numeric ids or exact team names; names are resolved by searching orgs/{org}/teams/search?q=<name> and matching .name case-sensitively.

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| _None_ |  |  |  |

## forgejo org team create

Use: `forgejo org team create <org> --name=X [--description=X] [--permission=read|write|admin]`

Create a team in an organization. --name is required. --description defaults to an empty string, --permission defaults to read, includes_all_repositories and can_create_org_repo are sent false, and the bash unit list is sent unchanged.

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| `--description` | `string` | `""` | team description |
| `--name` | `string` | `""` | team name to create (required) |
| `--permission` | `string` | `read` | team permission to send (read, write, or admin) |

## forgejo org team delete

Use: `forgejo org team delete <org> <team> [--yes]`

Delete a team. <team> may be a numeric id or exact team name resolved within <org>. This destructive command requires --yes or an interactive typed-name confirmation.

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| `--yes` | `bool` | `false` | skip the delete confirmation prompt |

## forgejo org team edit

Use: `forgejo org team edit <org> <team> [--name=X] [--description=X] [--permission=X]`

Edit a team. <team> may be a numeric id or exact team name. The command first fetches teams/{id} because Forgejo PATCH requires name and permission; unchanged fields are copied into the request body, matching bash.

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| `--description` | `string` | `""` | set the team description |
| `--name` | `string` | `""` | set the team name |
| `--permission` | `string` | `""` | set team permission |

## forgejo org team list

Use: `forgejo org team list <org>`

List teams in an organization through orgs/{org}/teams. The bash command fetched ?limit=50; the global --limit flag overrides that page size, and --limit=0 fetches all pages.

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| _None_ |  |  |  |

## forgejo org team member

Use: `forgejo org team member <list|add|remove>`

Manage team members. <team> may be a numeric id or exact team name resolved within <org> before teams/{id}/members calls.

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| _None_ |  |  |  |

## forgejo org team member add

Use: `forgejo org team member add <org> <team> --user=<user>`

Add a user to a team through teams/{id}/members/{user}. <team> may be a numeric id or exact team name resolved within <org>. --user is required.

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| `--user` | `string` | `""` | username to add to the team (required) |

## forgejo org team member list

Use: `forgejo org team member list <org> <team>`

List team members. <team> may be a numeric id or exact team name. The bash command fetched ?limit=50; the global --limit flag overrides that page size, and --limit=0 fetches all pages.

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| _None_ |  |  |  |

## forgejo org team member remove

Use: `forgejo org team member remove <org> <team> --user=<user> [--yes]`

Remove a user from a team through teams/{id}/members/{user}. <team> may be a numeric id or exact team name resolved within <org>. This destructive remove command requires --yes or an interactive typed-name confirmation.

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| `--user` | `string` | `""` | username to remove from the team (required) |
| `--yes` | `bool` | `false` | skip the delete confirmation prompt |

## forgejo org team repo

Use: `forgejo org team repo <list|add|remove>`

Manage repositories assigned to a team. <team> may be a numeric id or exact team name resolved within <org> before teams/{id}/repos calls.

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| _None_ |  |  |  |

## forgejo org team repo add

Use: `forgejo org team repo add <org> <team> --repo=<owner/repo>`

Add a repository to a team through teams/{id}/repos/{owner}/{repo}. <team> may be a numeric id or exact team name resolved within <org>. --repo is required and must be owner/repo.

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| `--repo` | `string` | `""` | repository full name to add, in owner/repo form (required) |

## forgejo org team repo list

Use: `forgejo org team repo list <org> <team>`

List repositories assigned to a team. <team> may be a numeric id or exact team name. The bash command fetched ?limit=50; the global --limit flag overrides that page size, and --limit=0 fetches all pages.

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| _None_ |  |  |  |

## forgejo org team repo remove

Use: `forgejo org team repo remove <org> <team> --repo=<owner/repo> [--yes]`

Remove a repository from a team through teams/{id}/repos/{owner}/{repo}. <team> may be a numeric id or exact team name resolved within <org>. --repo is required and must be owner/repo. This destructive remove command requires --yes or an interactive typed-name confirmation.

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| `--repo` | `string` | `""` | repository full name to remove, in owner/repo form (required) |
| `--yes` | `bool` | `false` | skip the delete confirmation prompt |

## forgejo org team view

Use: `forgejo org team view <org> <team>`

View a team. <team> may be a numeric id or an exact team name resolved within <org> before GET teams/{id}.

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| _None_ |  |  |  |

## forgejo org view

Use: `forgejo org view <name>`

View an organization through orgs/{name}. Text mode also fetches orgs/{name}/members?limit=50 and prints the member section; JSON mode emits only the raw organization response, matching bash.

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| _None_ |  |  |  |

