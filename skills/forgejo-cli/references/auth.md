# forgejo auth

Authenticate and inspect the configured account.

login prompts for a Forgejo URL, username, and password, then mints a token
using HTTP Basic auth. The token is verified with GET /user before the config
file is written. Existing config files require an overwrite prompt showing the
current URL. The config write is atomic, mode 0600, and stores only
FORGEJO_URL and FORGEJO_TOKEN; account passwords are never stored.

status calls GET /user with the configured token. Text output prints the URL,
login, email, and token scopes. Forgejo does not always expose current-token
scopes; when GET /user/tokens/current does not return scopes, status prints
"not exposed by Forgejo API".

## Global Flags

These inherited flags apply to commands in this group unless a command defines a local flag with the same name.

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| `--dry-run` | `bool` | `false` | print mutating requests instead of sending them |
| `--jq` | `string` | `""` | filter JSON output through a jq expression (implies --json) |
| `--json` | `bool` | `false` | output raw JSON from the server |
| `--limit` | `int` | `-1` | max items for list verbs (0 = fetch all pages; default: per-verb) |
| `--verbose` | `bool` | `false` | log requests to stderr (tokens are never logged) |

## forgejo auth login

Use: `forgejo auth login [--scopes=all] [--otp=CODE]`

Prompt for a Forgejo URL, username, and password, mint an API token,
verify that token with GET /user, and atomically write the CLI config.

--scopes is a comma-separated list sent to the token endpoint; it defaults to
"all", matching the bash implementation. --otp is sent as X-Forgejo-OTP for
accounts that require a one-time password.

Passwords are read from stdin with echo still enabled because this port stays
dependency-free. The password is used only for token creation and is never
written to the config file.

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| `--otp` | `string` | `""` | one-time password for accounts with 2FA enabled |
| `--scopes` | `string` | `all` | comma-separated token scopes to request |

## forgejo auth status

Use: `forgejo auth status`

Show the configured authenticated account.

status calls GET /user with the configured token. With --json, the raw user
JSON from the server is printed through the normal JSON path. Text output
prints URL, login, email, and scopes. Scopes are discovered with a best-effort
GET /user/tokens/current; if Forgejo does not expose them, the scopes line says
"not exposed by Forgejo API".

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| _None_ |  |  |  |

