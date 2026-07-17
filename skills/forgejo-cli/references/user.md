# forgejo user

Manage users through the same admin and user endpoints as the bash CLI.

User create/delete are admin endpoints. SSH keys can target either --self or another user; GPG add/delete are self-only because Forgejo has no admin-on-behalf endpoint. Access token verbs authenticate with HTTP Basic auth using FORGEJO_PASSWORD from the config.

## Global Flags

These inherited flags apply to commands in this group unless a command defines a local flag with the same name.

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| `--dry-run` | `bool` | `false` | print mutating requests instead of sending them |
| `--jq` | `string` | `""` | filter JSON output through a jq expression (implies --json) |
| `--json` | `bool` | `false` | output raw JSON from the server |
| `--limit` | `int` | `-1` | max items for list verbs (0 = fetch all pages; default: per-verb) |
| `--verbose` | `bool` | `false` | log requests to stderr (tokens are never logged) |

## forgejo user create

Use: `forgejo user create --login=X --email=X --password=X [--fullname=X] [--admin]`

Create a user through admin/users. --login, --email, and --password are required.

Forgejo's create-user option cannot set site admin directly, so --admin first creates the user and then sends the same follow-up PATCH used by user edit --admin=true. If that promotion fails, the user still exists and the command reports the gap.

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| `--admin` | `bool` | `false` | promote the new user to site admin after creation |
| `--email` | `string` | `""` | email address for the new user (required) |
| `--fullname` | `string` | `""` | full name to store on the user |
| `--login` | `string` | `""` | username to create (required) |
| `--password` | `string` | `""` | initial password for the new user (required) |

## forgejo user delete

Use: `forgejo user delete <username> [--yes]`

Delete a user through admin/users/{username}. This destructive command requires --yes or an interactive typed-name confirmation.

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| `--yes` | `bool` | `false` | skip the delete confirmation prompt |

## forgejo user edit

Use: `forgejo user edit <username> [--email=X] [--admin=true|false] [--active=true|false]`

Edit a user through admin/users/{username}. The command first fetches users/{username} to preserve the required login_name and source_id fields, matching the bash GET-first-then-PATCH flow.

--admin and --active accept true or false; any value other than true is sent as false, matching the bash implementation.

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| `--active` | `string` | `""` | set active status (true or false) |
| `--admin` | `string` | `""` | set site admin status (true or false) |
| `--email` | `string` | `""` | set the user's email address |

## forgejo user gpg

Use: `forgejo user gpg <list|add|delete>`

Manage GPG keys. Listing can target --self or a user, but add/delete are self-only because Forgejo has no admin-on-behalf endpoint for GPG keys.

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| _None_ |  |  |  |

## forgejo user gpg add

Use: `forgejo user gpg add --self --armored=<key-or-@file>`

Add a GPG key to the authenticated account. This is self-only; passing a user is an error because Forgejo has no admin-on-behalf endpoint. --armored is a literal armored public key unless it starts with @, in which case the rest is read as a file path and trailing newlines are stripped.

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| `--armored` | `string` | `""` | literal armored GPG public key, or @path to read a key file (required) |
| `--self` | `bool` | `false` | add the GPG key to the authenticated user (required) |

## forgejo user gpg delete

Use: `forgejo user gpg delete --self <id> [--yes]`

Delete a GPG key from the authenticated account by numeric id. This is self-only; passing a user is an error because Forgejo has no admin-on-behalf endpoint. This destructive command requires --yes or an interactive typed-name confirmation.

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| `--self` | `bool` | `false` | delete the GPG key from the authenticated user (required) |
| `--yes` | `bool` | `false` | skip the delete confirmation prompt |

## forgejo user gpg list

Use: `forgejo user gpg list <user> | --self`

List GPG keys for a user or for the authenticated account with --self. The bash command fetched ?limit=50; the global --limit flag overrides that page size, and --limit=0 fetches all pages.

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| `--self` | `bool` | `false` | list GPG keys for the authenticated user |

## forgejo user key

Use: `forgejo user key <list|add|delete>`

Manage SSH keys for --self or for another user. For another user, add/delete use admin/users/{user}/keys while list uses the public users/{user}/keys endpoint.

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| _None_ |  |  |  |

## forgejo user key add

Use: `forgejo user key add <user> --title=X --key=<key-or-@file> | --self --title=X --key=<key-or-@file>`

Add an SSH key for a user or for the authenticated account with --self. --title and --key are required. --key is a literal key unless it starts with @, in which case the rest is read as a file path and trailing newlines are stripped.

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| `--key` | `string` | `""` | literal SSH public key, or @path to read a key file (required) |
| `--self` | `bool` | `false` | add the SSH key to the authenticated user |
| `--title` | `string` | `""` | title for the SSH key (required) |

## forgejo user key delete

Use: `forgejo user key delete <user> <id> | --self <id> [--yes]`

Delete an SSH key by numeric id. For --self the command uses user/keys/{id}; for another user it uses admin/users/{user}/keys/{id}. This destructive command requires --yes or an interactive typed-name confirmation.

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| `--self` | `bool` | `false` | delete the SSH key from the authenticated user |
| `--yes` | `bool` | `false` | skip the delete confirmation prompt |

## forgejo user key list

Use: `forgejo user key list <user> | --self`

List SSH keys for a user or for the authenticated account with --self. The bash command fetched ?limit=50; the global --limit flag overrides that page size, and --limit=0 fetches all pages.

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| `--self` | `bool` | `false` | list SSH keys for the authenticated user |

## forgejo user list

Use: `forgejo user list`

List instance users through the admin/users endpoint. The bash command fetched admin/users?limit=50; the global --limit flag overrides that page size, and --limit=0 fetches all pages.

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| _None_ |  |  |  |

## forgejo user token

Use: `forgejo user token <list|create|delete>`

Manage a user's access tokens. Forgejo requires HTTP Basic auth for users/{user}/tokens, so these verbs use the username argument plus FORGEJO_PASSWORD from the config instead of bearer-token auth. --otp sends X-Forgejo-OTP for accounts with TOTP enabled.

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| _None_ |  |  |  |

## forgejo user token create

Use: `forgejo user token create <user> --name=X [--scopes=read:repo,...] [--otp=X]`

Create an access token for a user using HTTP Basic auth. FORGEJO_PASSWORD must be set in the config. --otp is passed as X-Forgejo-OTP for accounts with TOTP enabled.

--name is required. --scopes is optional; when omitted the request body contains only the token name. In text mode the token sha1 is printed once with the bash warning because it cannot be retrieved again.

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| `--name` | `string` | `""` | access token name (required) |
| `--otp` | `string` | `""` | one-time password for TOTP accounts, sent as X-Forgejo-OTP |
| `--scopes` | `string` | `""` | comma-separated token scopes; omitted lets the server choose its default |

## forgejo user token delete

Use: `forgejo user token delete <user> <name> [--otp=X] [--yes]`

Delete an access token by name using HTTP Basic auth. FORGEJO_PASSWORD must be set in the config. --otp is passed as X-Forgejo-OTP for accounts with TOTP enabled. This destructive command requires --yes or an interactive typed-name confirmation.

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| `--otp` | `string` | `""` | one-time password for TOTP accounts, sent as X-Forgejo-OTP |
| `--yes` | `bool` | `false` | skip the delete confirmation prompt |

## forgejo user token list

Use: `forgejo user token list <user> [--otp=X]`

List access tokens for a user using HTTP Basic auth. FORGEJO_PASSWORD must be set in the config. --otp is passed as X-Forgejo-OTP for accounts with TOTP enabled. The bash command fetched ?limit=50; the global --limit flag overrides that page size, and --limit=0 fetches all pages.

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| `--otp` | `string` | `""` | one-time password for TOTP accounts, sent as X-Forgejo-OTP |

## forgejo user view

Use: `forgejo user view <username>`

View a user by username through users/{username}. Text output prints the bash fields: login, full name, email, admin status, and creation date.

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| _None_ |  |  |  |

