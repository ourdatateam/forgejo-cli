# forgejo issue

Issue commands:

  issue list <owner/repo> [--state=open|closed]
      List issues in a repository.

  issue create <owner/repo> --title=TEXT [--body=TEXT]
      Create a new issue.

  issue view <owner/repo> <number>
      Show issue details and all comments inline.

  issue edit <owner/repo> <number> [--title=TEXT] [--state=open|closed] [--body=TEXT] [--labels=X,Y]
      Edit an issue's title, state, body, or labels.

  issue comment <owner/repo> <number> --body=TEXT
      Add a comment to an issue.
      Comment IDs are shown in issue view and in the API response (--json).

  issue comment delete <owner/repo> <comment_id>
      Delete a specific comment by its numeric ID.

  There is NO separate issue comment list or issue comment view.
  Use forgejo issue view <owner/repo> <number>, which shows all comments inline.

  issue close <owner/repo> <number>
      Close an issue.

  issue reopen <owner/repo> <number>
      Reopen a closed issue.

  issue assign <owner/repo> <number> --users=u1,u2
      Add assignees to an issue (union with existing).

  issue unassign <owner/repo> <number> --users=u1,u2
      Remove assignees from an issue.

  issue search [--owner=ORG] [--state=open|closed|all] [--labels=a,b] [--query=TEXT] [--limit=N]
      Search issues across repositories.

  issue milestone list <owner/repo> [--state=open|closed|all]
  issue milestone create <owner/repo> --title=TEXT [--description=TEXT] [--due=YYYY-MM-DD]
  issue milestone edit <owner/repo> <id> [--title=TEXT] [--description=TEXT] [--due=YYYY-MM-DD] [--state=open|closed]
  issue milestone delete <owner/repo> <id>
  issue milestone set <owner/repo> <number> --milestone=<id|title>
      Manage milestones. Pass --milestone=0 to clear.

  issue label <owner/repo> list [--scope=org|repo]
      List labels (scope defaults to org).

  issue label <owner/repo> create --name=TEXT [--color=HEX] [--desc=TEXT] [--scope=org|repo]
      Create a label.

  issue label <owner/repo> add <number> --labels=X,Y
      Add labels to an issue.

  issue label <owner/repo> remove <number> --label=TEXT
      Remove a single label from an issue.

  issue images <owner/repo> <number> [--output=DIR]
      Download all image attachments (body + comments) to DIR.
      DIR defaults to ./issue-<number>-images/. Non-image attachments
      (extension not in .png .jpg .jpeg .gif .webp .svg .bmp) are skipped.

## Global Flags

These inherited flags apply to commands in this group unless a command defines a local flag with the same name.

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| `--dry-run` | `bool` | `false` | print mutating requests instead of sending them |
| `--jq` | `string` | `""` | filter JSON output through a jq expression (implies --json) |
| `--json` | `bool` | `false` | output raw JSON from the server |
| `--limit` | `int` | `-1` | max items for list verbs (0 = fetch all pages; default: per-verb) |
| `--verbose` | `bool` | `false` | log requests to stderr (tokens are never logged) |

## forgejo issue assign

Use: `forgejo issue assign <owner/repo> <number> --users=u1,u2`

Add assignees to an issue, unioned with the current assignees. The command GETs the issue first, computes the new set, and PATCHes the complete assignee list back even if unchanged.

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| `--users` | `string` | `""` | comma-separated usernames to add |

## forgejo issue close

Use: `forgejo issue close <owner/repo> <number>`

Close an issue by PATCHing its state to closed. The repository argument is required; pass . to infer it from the current git remote.

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| _None_ |  |  |  |

## forgejo issue comment

Use: `forgejo issue comment <owner/repo> <number> --body=TEXT`

Issue comment commands:
  forgejo issue comment <owner/repo> <number> --body=TEXT
      Add a comment to an issue or PR.
      The <number> is the issue number (for example, #42).
      --body=- reads stdin and --body-file=PATH reads from a file.

  forgejo issue comment delete <owner/repo> <comment_id>
      Delete a comment by its numeric comment ID.
      Find the comment ID from forgejo issue view <owner/repo> <number>
      (shown as [comment #123]) or use --json for raw API output.

IMPORTANT NOTES:
  - There is NO issue comment list or issue comment view subcommand.
    To see all comments on an issue, use: forgejo issue view <owner/repo> <number>
  - Issue and PR comments share the same API backend; comment IDs are globally
    unique within a repo across both issues and PRs.
  - issue comment on a PR number also works (comments on the PR thread).

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| `--body` | `string` | `""` | body text ('-' reads stdin) |
| `--body-file` | `string` | `""` | read body from a file |

## forgejo issue comment delete

Use: `forgejo issue comment delete <owner/repo> <comment_id>`

Delete a comment by its numeric comment ID. Comment IDs are shown by issue view as [comment #123] and in raw JSON output. Requires --yes or an interactive typed confirmation.

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| `--yes` | `bool` | `false` | skip the delete confirmation prompt |

## forgejo issue create

Use: `forgejo issue create <owner/repo> --title=TEXT [--body=TEXT] [--labels=X,Y]`

Create a new issue. --title is required. --body may be text, --body=- to read stdin, or --body-file=PATH. --labels resolves comma-separated label names to IDs before creating the issue.

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| `--body` | `string` | `""` | body text ('-' reads stdin) |
| `--body-file` | `string` | `""` | read body from a file |
| `--labels` | `string` | `""` | comma-separated label names to apply |
| `--title` | `string` | `""` | issue title (required) |

## forgejo issue edit

Use: `forgejo issue edit <owner/repo> <number> [--title=TEXT] [--state=open|closed] [--body=TEXT] [--labels=X,Y]`

Edit an issue's title, state, body, or labels. Forgejo's EditIssueOption has no labels field, so --labels replaces all labels with a separate PUT to the labels endpoint after the issue PATCH. Passing --labels= with an empty value clears all labels.

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| `--body` | `string` | `""` | body text ('-' reads stdin) |
| `--body-file` | `string` | `""` | read body from a file |
| `--labels` | `string` | `""` | comma-separated label names to replace all labels; empty clears all labels |
| `--state` | `string` | `""` | new issue state (open or closed) |
| `--title` | `string` | `""` | new issue title |

## forgejo issue images

Use: `forgejo issue images <owner/repo> <number> [--output=DIR]`

Download all image attachments from an issue body and its comments. DIR defaults to ./issue-<number>-images/. Non-image attachments (extension not in .png .jpg .jpeg .gif .webp .svg .bmp) are skipped. JSON output prints the filtered attachment objects instead of downloading.

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| `--output` | `string` | `""` | output directory (default ./issue-<number>-images) |

## forgejo issue label

Use: `forgejo issue label <owner/repo> <list|create|add|remove> [args]`

Manage labels using the bash-compatible argument order:

  issue label <owner/repo> list [--scope=org|repo]
      List labels. Scope defaults to org; any scope other than repo uses the org endpoint.

  issue label <owner/repo> create --name=TEXT [--color=HEX] [--desc=TEXT] [--scope=org|repo]
      Create a label. Color defaults to #0075ca; a leading # is accepted.

  issue label <owner/repo> add <number> --labels=X,Y
      Add labels to an issue. Label names are resolved by checking org labels before repo labels.

  issue label <owner/repo> remove <number> --label=TEXT
      Remove a single label from an issue. Requires --yes or an interactive typed confirmation.

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| `--color` | `string` | `#0075ca` | label color hex for create; leading # is accepted |
| `--desc` | `string` | `""` | label description for create |
| `--label` | `string` | `""` | single label name for remove |
| `--labels` | `string` | `""` | comma-separated label names for add |
| `--name` | `string` | `""` | label name for create |
| `--scope` | `string` | `org` | label scope for list/create (org or repo; default org) |
| `--yes` | `bool` | `false` | skip the delete confirmation prompt |

## forgejo issue list

Use: `forgejo issue list <owner/repo> [--state=open|closed]`

List issues in a repository. The repository argument is required; pass . to infer it from the current git remote. State defaults to open and is passed through to the server.

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| `--state` | `string` | `open` | issue state filter (open, closed, or all) |

## forgejo issue milestone

Use: `forgejo issue milestone <list|create|edit|delete|set>`

Milestone commands:
  forgejo issue milestone list <owner/repo> [--state=open|closed|all]
      List milestones in a repository.

  forgejo issue milestone create <owner/repo> --title=TEXT [--description=TEXT] [--due=YYYY-MM-DD]
      Create a new milestone.

  forgejo issue milestone edit <owner/repo> <id> [--title=TEXT] [--description=TEXT] [--due=YYYY-MM-DD] [--state=open|closed]
      Edit a milestone. <id> can be the numeric milestone ID.

  forgejo issue milestone delete <owner/repo> <id>
      Delete a milestone by its numeric ID. Requires --yes or an interactive typed confirmation.

  forgejo issue milestone set <owner/repo> <number> --milestone=<id|title>
      Set a milestone on an issue. Pass --milestone=0 to clear the milestone.
      <id|title> accepts either the numeric ID or exact title of the milestone.

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| _None_ |  |  |  |

## forgejo issue milestone create

Use: `forgejo issue milestone create <owner/repo> --title=TEXT [--description=TEXT] [--due=YYYY-MM-DD]`

Create a new milestone. --title is required. --description defaults to empty. --due, when provided, must be YYYY-MM-DD and is sent as midnight UTC.

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| `--description` | `string` | `""` | milestone description |
| `--due` | `string` | `""` | due date in YYYY-MM-DD |
| `--title` | `string` | `""` | milestone title (required) |

## forgejo issue milestone delete

Use: `forgejo issue milestone delete <owner/repo> <id>`

Delete a milestone by numeric ID. Requires --yes or an interactive typed confirmation.

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| `--yes` | `bool` | `false` | skip the delete confirmation prompt |

## forgejo issue milestone edit

Use: `forgejo issue milestone edit <owner/repo> <id> [--title=TEXT] [--description=TEXT] [--due=YYYY-MM-DD] [--state=open|closed]`

Edit a milestone by numeric ID. At least one of --title, --description, --due, or --state is required. --due must be YYYY-MM-DD and is sent as midnight UTC.

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| `--description` | `string` | `""` | new milestone description |
| `--due` | `string` | `""` | new due date in YYYY-MM-DD |
| `--state` | `string` | `""` | new milestone state (open or closed) |
| `--title` | `string` | `""` | new milestone title |

## forgejo issue milestone list

Use: `forgejo issue milestone list <owner/repo> [--state=open|closed|all]`

List milestones in a repository. State defaults to open and is passed through to the server.

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| `--state` | `string` | `open` | milestone state filter (open, closed, or all) |

## forgejo issue milestone set

Use: `forgejo issue milestone set <owner/repo> <number> --milestone=<id|title>`

Set a milestone on an issue. Pass --milestone=0 to clear the milestone. Otherwise --milestone accepts either a numeric milestone ID or the exact title of a milestone in the repository.

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| `--milestone` | `string` | `""` | milestone numeric ID or exact title; 0 clears |

## forgejo issue reopen

Use: `forgejo issue reopen <owner/repo> <number>`

Reopen a closed issue by PATCHing its state to open. The repository argument is required; pass . to infer it from the current git remote.

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| _None_ |  |  |  |

## forgejo issue search

Use: `forgejo issue search [--owner=ORG] [--state=open|closed|all] [--labels=a,b] [--query=TEXT] [--limit=N]`

Search issues across repositories. The search is issue-only (type=issues). State defaults to open. --owner, --labels, and --query are optional filters. --limit is a local server-side query parameter and must be non-negative when provided.

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| `--labels` | `string` | `""` | comma-separated label filter |
| `--limit` | `int` | `-1` | server-side result limit |
| `--owner` | `string` | `""` | restrict to an owner/org |
| `--query` | `string` | `""` | full-text search query |
| `--state` | `string` | `open` | issue state filter (open, closed, or all) |

## forgejo issue unassign

Use: `forgejo issue unassign <owner/repo> <number> --users=u1,u2`

Remove assignees from an issue. The command GETs the issue first, computes the new set, and PATCHes the complete assignee list back even if unchanged.

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| `--users` | `string` | `""` | comma-separated usernames to remove |

## forgejo issue view

Use: `forgejo issue view <owner/repo> <number>`

Show issue details and all comments inline. JSON output is the raw issue object, matching the bash command; comments are fetched only for text output.

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| _None_ |  |  |  |

