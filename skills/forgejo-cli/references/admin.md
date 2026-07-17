# forgejo admin

Admin commands for Forgejo instance operations.

The Forgejo API exposes cron task listing/runs and selected server settings.
Queue, stats, and notices are not exposed by the Forgejo API on the probed
server versions; those commands exit 2 with a web UI pointer.

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

## forgejo admin config

Use: `forgejo admin config <view>`

View selected Forgejo server settings.

The view command aggregates the settings/api, settings/attachment,
settings/repository, and settings/ui endpoints into api, attachment,
repository, and ui sections.

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| _None_ |  |  |  |

## forgejo admin config view

Use: `forgejo admin config view`

Show selected Forgejo server settings.

This command GETs settings/api, settings/attachment, settings/repository, and
settings/ui, then merges them under api, attachment, repository, and ui. With
--json, the merged JSON object is printed. Text mode prints each section with
keys sorted alphabetically.

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| _None_ |  |  |  |

## forgejo admin cron

Use: `forgejo admin cron <list|run>`

List Forgejo admin cron tasks or trigger one task by name.

The list command preserves the bash default limit=50; --limit overrides that
default and --limit=0 fetches all pages. The run command posts to the cron task
name as a single escaped path segment.

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| _None_ |  |  |  |

## forgejo admin cron list

Use: `forgejo admin cron list`

List Forgejo admin cron tasks.

The bash command used limit=50; --limit overrides that default and --limit=0
fetches all pages. Text output includes name, schedule, next run, previous run,
and execution count.

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| _None_ |  |  |  |

## forgejo admin cron run

Use: `forgejo admin cron run <name>`

Trigger one Forgejo admin cron task by name.

The task name is required and is escaped as a single path segment before the
POST request.

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| _None_ |  |  |  |

## forgejo admin notice

Use: `forgejo admin notice <list|delete>`

Admin notices are not exposed by the Forgejo API.

The list and delete subcommands preserve the bash behavior: each exits with
code 2 and points to the notices page in the admin web UI.

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| _None_ |  |  |  |

## forgejo admin notice delete

Use: `forgejo admin notice delete [id] [--yes]`

Admin notice deletion is not exposed by the Forgejo API.

The command accepts --yes for consistency with destructive verbs, but no API
call is attempted; it exits with code 2 and points to the notices page in the
admin web UI.

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| `--yes` | `bool` | `false` | skip the delete confirmation prompt |

## forgejo admin notice list

Use: `forgejo admin notice list`

Admin notice listing is not exposed by the Forgejo API.

This command exits with code 2 and points to the notices page in the admin web
UI.

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| _None_ |  |  |  |

## forgejo admin queue

Use: `forgejo admin queue <list|view|pause|resume>`

Admin queue operations are not exposed by the Forgejo API.

The list, view, pause, and resume subcommands preserve the bash behavior: each
exits with code 2 and points to the queue page in the admin web UI.

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| _None_ |  |  |  |

## forgejo admin queue list

Use: `forgejo admin queue list`

This admin queue operation is not exposed by the
Forgejo API and exits with code 2. Use the admin web UI queue page instead.

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| _None_ |  |  |  |

## forgejo admin queue pause

Use: `forgejo admin queue pause`

This admin queue operation is not exposed by the
Forgejo API and exits with code 2. Use the admin web UI queue page instead.

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| _None_ |  |  |  |

## forgejo admin queue resume

Use: `forgejo admin queue resume`

This admin queue operation is not exposed by the
Forgejo API and exits with code 2. Use the admin web UI queue page instead.

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| _None_ |  |  |  |

## forgejo admin queue view

Use: `forgejo admin queue view`

This admin queue operation is not exposed by the
Forgejo API and exits with code 2. Use the admin web UI queue page instead.

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| _None_ |  |  |  |

## forgejo admin stats

Use: `forgejo admin stats`

Admin stats are not exposed by the Forgejo API.

This command preserves the bash behavior: it exits with code 2 and points to
the admin web UI instead of attempting an API call.

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| _None_ |  |  |  |

