# forgejo pr

Pull request commands.

The repo positional is always required. Pass "." explicitly to infer the repo
from a git remote on the configured Forgejo host. Use --json for raw JSON where
the verb returns JSON; pr diff and pr patch stream raw bytes instead.

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

## forgejo pr checkout

Use: `forgejo pr checkout <owner/repo> <number> [--branch=NAME] [--detach]`

Check out a pull request's head ref using git.

The command must run inside a clone with a git remote on the configured
Forgejo host. It resolves the PR first, then fetches refs/pull/<number>/head
from the matching remote and switches to the requested local branch. With
--detach it checks out FETCH_HEAD detached instead.

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| `--branch` | `string` | `""` | local branch name to create or update |
| `--detach` | `bool` | `false` | check out the PR head detached at FETCH_HEAD |

## forgejo pr checks

Use: `forgejo pr checks <owner/repo> <number>`

Show commit statuses and Actions CI runs for the PR head commit.

The PR head SHA is resolved first. Actions runs are looked up by head_sha, then
joined to jobs from repos/{repo}/actions/tasks?limit=200 by matching each run's
index_in_repo to each task's run_number. That tasks limit is a fixed internal
join limit and is not affected by --limit.

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| _None_ |  |  |  |

## forgejo pr close

Use: `forgejo pr close <owner/repo> <number>`

Close a pull request without merging by PATCHing its state to closed.

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| _None_ |  |  |  |

## forgejo pr comment

Use: `forgejo pr comment <owner/repo> <number> --body=TEXT`

PR comment commands.

With no subcommand, pr comment <owner/repo> <number> --body=TEXT creates a
general PR thread comment. The <number> is the PR number. Comment subcommands
that take <comment_id> use the numeric comment ID, which is globally unique
within a repo across issues and PRs. --body=- and --body-file=- read from stdin.

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| `--body` | `string` | `""` | body text ('-' reads stdin) |
| `--body-file` | `string` | `""` | read body from a file |

## forgejo pr comment create

Use: `forgejo pr comment create <owner/repo> <number> --body=TEXT`

Add a general comment to a PR thread.

The repo positional and PR number are required. The comment body is required
and may be supplied with --body, --body=-, --body-file=PATH, or --body-file=-.

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| `--body` | `string` | `""` | body text ('-' reads stdin) |
| `--body-file` | `string` | `""` | read body from a file |

## forgejo pr comment delete

Use: `forgejo pr comment delete <owner/repo> <comment_id> [--yes]`

Delete one PR comment by numeric comment ID.

This is destructive. Pass --yes to skip the confirmation prompt; otherwise the
comment ID must be typed interactively.

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| `--yes` | `bool` | `false` | skip the delete confirmation prompt |

## forgejo pr comment edit

Use: `forgejo pr comment edit <owner/repo> <comment_id> --body=TEXT`

Edit one PR comment by numeric comment ID.

The replacement body is required and may be supplied with --body, --body=-,
--body-file=PATH, or --body-file=-.

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| `--body` | `string` | `""` | body text ('-' reads stdin) |
| `--body-file` | `string` | `""` | read body from a file |

## forgejo pr comment list

Use: `forgejo pr comment list <owner/repo> <number>`

List all general comments on a PR. Shows ID, author, date, and body preview.

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| _None_ |  |  |  |

## forgejo pr comment view

Use: `forgejo pr comment view <owner/repo> <comment_id>`

View one PR comment by numeric comment ID.

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| _None_ |  |  |  |

## forgejo pr create

Use: `forgejo pr create <owner/repo> --title=TEXT --head=BRANCH [--base=main] [--body=TEXT]`

Create a new pull request.

The repo positional, --title, and --head are required. --base defaults to main.
The PR body may be supplied with --body, --body=-, --body-file=PATH, or
--body-file=-. If no body flag is supplied, the body is empty.

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| `--base` | `string` | `main` | base branch to merge into |
| `--body` | `string` | `""` | body text ('-' reads stdin) |
| `--body-file` | `string` | `""` | read body from a file |
| `--head` | `string` | `""` | head branch for the PR (required) |
| `--title` | `string` | `""` | PR title (required) |

## forgejo pr diff

Use: `forgejo pr diff <owner/repo> <number>`

Show the raw unified diff for a PR. This streams bytes with Accept: */* and does not apply JSON formatting.

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| _None_ |  |  |  |

## forgejo pr edit

Use: `forgejo pr edit <owner/repo> <number> [--title=TEXT] [--body=TEXT] [--base=BRANCH]`

Edit a PR's title, body, base branch, labels, assignees, or requested reviewers.

At least one edit flag is required. --body=- and --body-file=- read the
replacement body from stdin. Label and assignee edits use the issue endpoints
because Forgejo models PRs as issues for that metadata.

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| `--add-assignees` | `string` | `""` | comma-separated assignees to add |
| `--add-labels` | `string` | `""` | comma-separated label names to add |
| `--add-reviewers` | `string` | `""` | comma-separated reviewers to request |
| `--base` | `string` | `""` | replacement base branch |
| `--body` | `string` | `""` | body text ('-' reads stdin) |
| `--body-file` | `string` | `""` | read body from a file |
| `--remove-assignees` | `string` | `""` | comma-separated assignees to remove |
| `--remove-labels` | `string` | `""` | comma-separated label names to remove |
| `--remove-reviewers` | `string` | `""` | comma-separated reviewer requests to withdraw |
| `--title` | `string` | `""` | replacement PR title |

## forgejo pr files

Use: `forgejo pr files <owner/repo> <number>`

List files changed by a pull request, including status, additions, and deletions.

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| _None_ |  |  |  |

## forgejo pr list

Use: `forgejo pr list <owner/repo> [--state=open|closed|all]`

List pull requests for a repository.

The repo positional is required; "." triggers git-remote inference. The state
filter defaults to open and accepts the same values as the bash command:
open, closed, or all.

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| `--state` | `string` | `open` | PR state filter: open, closed, or all |

## forgejo pr merge

Use: `forgejo pr merge <owner/repo> <number> [--method=merge|rebase|squash]`

Merge a pull request.

--method defaults to merge and is passed to the server as the Do field. The
bash command accepted merge, rebase, or squash and otherwise let the server
reject invalid methods.

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| `--method` | `string` | `merge` | merge method to request: merge, rebase, or squash |

## forgejo pr patch

Use: `forgejo pr patch <owner/repo> <number>`

Show the raw patch for a PR. This streams bytes with Accept: */* and does not apply JSON formatting.

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| _None_ |  |  |  |

## forgejo pr ready

Use: `forgejo pr ready <owner/repo> <number>`

Mark a draft PR as ready for review.

Forgejo silently drops PATCH {draft:false}; the bash workaround is to GET the
pull request and strip a leading WIP title prefix instead. This recognizes the
default Forgejo prefixes "WIP:" and "[WIP]" case-insensitively and then PATCHes
the title.

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| _None_ |  |  |  |

## forgejo pr reopen

Use: `forgejo pr reopen <owner/repo> <number>`

Reopen a pull request by PATCHing its state to open.

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| _None_ |  |  |  |

## forgejo pr requested-reviewers

Use: `forgejo pr requested-reviewers <add|remove> <owner/repo> <number> --users=u1,u2`

Request or withdraw specific user review requests for a PR. --users is a comma-separated login list.

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| _None_ |  |  |  |

## forgejo pr requested-reviewers add

Use: `forgejo pr requested-reviewers add <owner/repo> <number> --users=u1,u2`

Request specific users to review a PR. --users is a comma-separated login list.

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| `--users` | `string` | `""` | comma-separated user logins to request (required) |

## forgejo pr requested-reviewers remove

Use: `forgejo pr requested-reviewers remove <owner/repo> <number> --users=u1,u2 [--yes]`

Withdraw requested reviewers from a PR.

This is a destructive remove operation. Pass --yes to skip the confirmation
prompt; otherwise the comma-separated --users value must be typed interactively.

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| `--users` | `string` | `""` | comma-separated user logins to withdraw (required) |
| `--yes` | `bool` | `false` | skip the delete confirmation prompt |

## forgejo pr review

Use: `forgejo pr review <owner/repo> <number> --approve|--request-changes|--comment [--body=TEXT] [--comments=FILE|-]`

PR review commands.

With no subcommand, pr review <owner/repo> <number> submits a review. Exactly
one of --approve, --request-changes, or --comment is required. The flags are
mutually exclusive. --body is optional for approvals, required with
--request-changes, and required with --comment unless --comments is supplied.

--comments=FILE|- attaches inline line-level comments. The file or stdin must
be a non-empty JSON array of {"path":"f","line":N,"body":"text"} objects.
The line field is sent to Forgejo as new_position, matching the bash command.

The submit POST uses FORGEJO_REVIEW_TOKEN when configured; all surrounding GETs
use the primary token. If no review token is configured, submission silently
falls back to the primary token.

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| `--approve` | `bool` | `false` | approve the PR |
| `--body` | `string` | `""` | body text ('-' reads stdin) |
| `--body-file` | `string` | `""` | read body from a file |
| `--comment` | `bool` | `false` | leave a general review comment; requires --body unless --comments is supplied |
| `--comments` | `string` | `""` | inline review comments JSON file, or '-' to read JSON from stdin |
| `--request-changes` | `bool` | `false` | request changes on the PR; requires --body |

## forgejo pr review create

Use: `forgejo pr review create <owner/repo> <number> --approve|--request-changes|--comment [--body=TEXT] [--comments=FILE|-]`

Submit a PR review.

Exactly one of --approve, --request-changes, or --comment is required.
--body is optional for approvals, required with --request-changes, and required
with --comment unless --comments is supplied. --comments=FILE|- reads a
non-empty JSON array of inline comments and maps each line field to
new_position in the API payload. The submit POST uses FORGEJO_REVIEW_TOKEN when
configured and otherwise falls back to the primary token.

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| `--approve` | `bool` | `false` | approve the PR |
| `--body` | `string` | `""` | body text ('-' reads stdin) |
| `--body-file` | `string` | `""` | read body from a file |
| `--comment` | `bool` | `false` | leave a general review comment; requires --body unless --comments is supplied |
| `--comments` | `string` | `""` | inline review comments JSON file, or '-' to read JSON from stdin |
| `--request-changes` | `bool` | `false` | request changes on the PR; requires --body |

## forgejo pr review dismiss

Use: `forgejo pr review dismiss <owner/repo> <number> <review_id> --message=TEXT`

Dismiss a previous review by numeric review ID. --message is required and is sent as the dismissal message.

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| `--message` | `string` | `""` | dismissal message (required) |

## forgejo pr review list

Use: `forgejo pr review list <owner/repo> <number>`

List all reviews on a PR. Shows ID, user, state, and submitted date.

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| _None_ |  |  |  |

## forgejo pr review lookup

Use: `forgejo pr review lookup <owner/repo> <number>`

Fetch all reviews, review comments, and PR comments merged into one JSON array.

Each item has a type field: "review", "review_comment", or "comment". Output
is always JSON; --json is implied. Review comment lists are fetched with paged
API calls through the same complete-conversation path as pr view.

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| _None_ |  |  |  |

## forgejo pr view

Use: `forgejo pr view <owner/repo> <number>`

View a PR with its complete conversation.

The output includes the pull request body, all reviews, inline review-thread
comments nested under their review, and general issue comments in
issue_comments. The review and comment lists are fetched with paged API calls.
Use --json for the complete machine-readable object.

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| _None_ |  |  |  |

