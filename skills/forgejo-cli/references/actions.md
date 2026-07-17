# forgejo actions

Actions commands inspect workflow runs, watch a run until it
finishes, read task logs from the Forgejo host when FORGEJO_LOG_PATH is set,
dispatch workflows, and manage scoped runners, secrets, and variables.

Scopes for runners, secrets, and variables are one of <owner/repo>, <org>, or
--admin for the instance scope.

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

## forgejo actions dispatch

Use: `forgejo actions dispatch <owner/repo> --workflow=X --ref=Y [--input=k=v]...`

Trigger a workflow dispatch event for a required repository.

The repository positional is required; pass "." to infer owner/repo from the
current git remote. --workflow names the workflow file or workflow id, --ref is
the branch or tag to run, and each --input must be k=v. Repeat --input for
multiple workflow inputs.

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| `--input` | `stringArray` | `[]` | workflow input as k=v (repeatable) |
| `--ref` | `string` | `""` | branch or tag ref to dispatch (required) |
| `--workflow` | `string` | `""` | workflow file name or workflow id (required) |

## forgejo actions list

Use: `forgejo actions list <owner/repo>`

List recent workflow runs for a required repository.

The repository positional is required; pass "." to infer owner/repo from the
current git remote. The bash command used limit=20; --limit overrides that
default and --limit=0 requests all pages where the API supports it.

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| _None_ |  |  |  |

## forgejo actions logs

Use: `forgejo actions logs <owner/repo> <run_id>`

View workflow run logs.

Forgejo's API does not expose job log contents for the probed server versions.
When FORGEJO_LOG_PATH is configured and this CLI runs on the Forgejo host, logs
are read directly from disk as zstd files at
FORGEJO_LOG_PATH/<owner>/<repo>/<last-byte-hex>/<task_id>.log.zst. Without
FORGEJO_LOG_PATH, this prints the task list, browser URL, and setup hint.

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| _None_ |  |  |  |

## forgejo actions runner

Use: `forgejo actions runner <list|register|delete>`

Manage Actions runners scoped to a repository, an organization,
or the whole instance.

Pass <owner/repo> for repository scope, <org> for organization scope, or
--admin for instance scope.

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| _None_ |  |  |  |

## forgejo actions runner delete

Use: `forgejo actions runner delete [<owner/repo|org>|--admin] <runner_id> [--yes]`

Delete an Actions runner from one scope.

Pass <owner/repo> for repository scope, <org> for organization scope, or
--admin for instance scope. The runner id is required. Deletion requires --yes
or an interactive typed-name confirmation.

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| `--admin` | `bool` | `false` | use instance-wide Actions scope instead of repo/org scope |
| `--yes` | `bool` | `false` | skip the delete confirmation prompt |

## forgejo actions runner list

Use: `forgejo actions runner list [<owner/repo|org>|--admin]`

List Actions runners for one scope.

Pass <owner/repo> for repository scope, <org> for organization scope, or
--admin for instance scope. Exactly one scope is required.

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| `--admin` | `bool` | `false` | use instance-wide Actions scope instead of repo/org scope |

## forgejo actions runner register

Use: `forgejo actions runner register [<owner/repo|org>|--admin]`

Print the Actions runner registration token for one scope.

Pass <owner/repo> for repository scope, <org> for organization scope, or
--admin for instance scope. Exactly one scope is required.

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| `--admin` | `bool` | `false` | use instance-wide Actions scope instead of repo/org scope |

## forgejo actions secret

Use: `forgejo actions secret <list|set|delete>`

Manage Actions secrets scoped to a repository, an
organization, or the whole instance.

Pass <owner/repo> for repository scope, <org> for organization scope, or
--admin for instance scope. Secret values are write-only; list shows names and
creation times.

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| _None_ |  |  |  |

## forgejo actions secret delete

Use: `forgejo actions secret delete [<owner/repo|org>|--admin] <name> [--yes]`

Delete an Actions secret from one scope.

Pass <owner/repo> for repository scope, <org> for organization scope, or
--admin for instance scope. The secret name is required and is escaped as a
single path segment. Deletion requires --yes or an interactive typed-name
confirmation.

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| `--admin` | `bool` | `false` | use instance-wide Actions scope instead of repo/org scope |
| `--yes` | `bool` | `false` | skip the delete confirmation prompt |

## forgejo actions secret list

Use: `forgejo actions secret list [<owner/repo|org>|--admin]`

List Actions secrets for one scope.

Pass <owner/repo> for repository scope, <org> for organization scope, or
--admin for instance scope. Exactly one scope is required. Secret values are
not returned by Forgejo.

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| `--admin` | `bool` | `false` | use instance-wide Actions scope instead of repo/org scope |

## forgejo actions secret set

Use: `forgejo actions secret set [<owner/repo|org>|--admin] --name=X --value=X`

Create or update an Actions secret for one scope.

Pass <owner/repo> for repository scope, <org> for organization scope, or
--admin for instance scope. --name and --value are required and must be
non-empty. The request body uses Forgejo's discovered {data: value} field.

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| `--admin` | `bool` | `false` | use instance-wide Actions scope instead of repo/org scope |
| `--name` | `string` | `""` | secret name (required) |
| `--value` | `string` | `""` | secret value (required; write-only) |

## forgejo actions variable

Use: `forgejo actions variable <list|get|set|delete>`

Manage Actions variables scoped to a repository, an
organization, or the whole instance.

Pass <owner/repo> for repository scope, <org> for organization scope, or
--admin for instance scope. Variable values are readable. Set is idempotent:
the command GETs the variable first and then PUTs an existing variable or POSTs
a new one.

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| _None_ |  |  |  |

## forgejo actions variable delete

Use: `forgejo actions variable delete [<owner/repo|org>|--admin] <name> [--yes]`

Delete an Actions variable from one scope.

Pass <owner/repo> for repository scope, <org> for organization scope, or
--admin for instance scope. The variable name is required and is escaped as a
single path segment. Deletion requires --yes or an interactive typed-name
confirmation.

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| `--admin` | `bool` | `false` | use instance-wide Actions scope instead of repo/org scope |
| `--yes` | `bool` | `false` | skip the delete confirmation prompt |

## forgejo actions variable get

Use: `forgejo actions variable get [<owner/repo|org>|--admin] <name>`

Get one Actions variable from one scope.

Pass <owner/repo> for repository scope, <org> for organization scope, or
--admin for instance scope. The variable name is required and is escaped as a
single path segment. With --json, the raw variable JSON is printed.

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| `--admin` | `bool` | `false` | use instance-wide Actions scope instead of repo/org scope |

## forgejo actions variable list

Use: `forgejo actions variable list [<owner/repo|org>|--admin]`

List Actions variables for one scope.

Pass <owner/repo> for repository scope, <org> for organization scope, or
--admin for instance scope. Exactly one scope is required. Values are printed
in text mode because Forgejo returns them as the data field.

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| `--admin` | `bool` | `false` | use instance-wide Actions scope instead of repo/org scope |

## forgejo actions variable set

Use: `forgejo actions variable set [<owner/repo|org>|--admin] --name=X --value=X`

Create or update an Actions variable for one scope.

Pass <owner/repo> for repository scope, <org> for organization scope, or
--admin for instance scope. --name and --value are required and must be
non-empty. This preserves the bash command's idempotent GET-first flow: a
successful GET is followed by PUT, and any non-2xx probe is followed by POST.
Both create and update bodies use Forgejo's discovered {value: value} field.

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| `--admin` | `bool` | `false` | use instance-wide Actions scope instead of repo/org scope |
| `--name` | `string` | `""` | variable name (required) |
| `--value` | `string` | `""` | variable value (required) |

## forgejo actions view

Use: `forgejo actions view <owner/repo> <run_id>`

View a workflow run and, in text mode, the jobs that belong to
that run.

The repository positional is required; pass "." to infer owner/repo from the
current git remote. Jobs are joined by comparing the run's index_in_repo to
task run_number from actions/tasks?limit=200; this hardcoded join limit is not
affected by --limit. With --json, only the raw run JSON is printed.

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| _None_ |  |  |  |

## forgejo actions watch

Use: `forgejo actions watch <owner/repo> <run_id>`

Poll a workflow run every five seconds until its status is
success, failure, or cancelled.

The repository positional is required; pass "." to infer owner/repo from the
current git remote. On a terminal, the status line is rewritten with carriage
returns like the bash command; when stdout is not a terminal, one status line is
printed for each poll. A successful run exits 0; failure or cancellation prints
the final status and returns a non-zero error.

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| _None_ |  |  |  |

## forgejo actions workflow

Use: `forgejo actions workflow <list|enable|disable|dispatch>`

List workflow files from a repository, enable or disable a
workflow, or trigger a workflow dispatch.

Workflow listing uses the contents API because Forgejo has no direct workflow
list endpoint. It checks .forgejo/workflows, then .gitea/workflows, then
.github/workflows, and returns the first directory containing YAML files.

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| _None_ |  |  |  |

## forgejo actions workflow disable

Use: `forgejo actions workflow disable <owner/repo> <workflow>`

Disable a workflow for a required repository.

The repository positional is required; pass "." to infer owner/repo from the
current git remote. The workflow is sent as a single path segment, so names,
ids, and workflow file paths are percent-escaped before calling the API.

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| _None_ |  |  |  |

## forgejo actions workflow dispatch

Use: `forgejo actions workflow dispatch <owner/repo> --workflow=X --ref=Y [--input=k=v]...`

Trigger a workflow dispatch event for a required repository.

The repository positional is required; pass "." to infer owner/repo from the
current git remote. --workflow names the workflow file or workflow id, --ref is
the branch or tag to run, and each --input must be k=v. Repeat --input for
multiple workflow inputs.

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| `--input` | `stringArray` | `[]` | workflow input as k=v (repeatable) |
| `--ref` | `string` | `""` | branch or tag ref to dispatch (required) |
| `--workflow` | `string` | `""` | workflow file name or workflow id (required) |

## forgejo actions workflow enable

Use: `forgejo actions workflow enable <owner/repo> <workflow>`

Enable a workflow for a required repository.

The repository positional is required; pass "." to infer owner/repo from the
current git remote. The workflow is sent as a single path segment, so names,
ids, and workflow file paths are percent-escaped before calling the API.

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| _None_ |  |  |  |

## forgejo actions workflow list

Use: `forgejo actions workflow list <owner/repo>`

List workflow YAML files in a required repository.

The repository positional is required; pass "." to infer owner/repo from the
current git remote. Forgejo has no direct workflow-list endpoint, so this reads
.forgejo/workflows, .gitea/workflows, then .github/workflows through the
contents API and prints the first directory containing .yml or .yaml files.

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| _None_ |  |  |  |

