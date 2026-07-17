# forgejo api

Make a raw Forgejo API request.

METHOD is optional and must be exactly one of GET, POST, PUT, PATCH, DELETE,
or HEAD in uppercase. If omitted, GET is used. Lowercase method words are
treated as the path and therefore fail the path rules.

path may be an absolute http(s):// URL, an /api/v1/... path, or any other
/path. /api/v1/... is used as-is on the configured instance; other /path
values are prefixed with /api/v1. Paths starting with // are rejected.

Absolute URLs are allowed only when their scheme, host, and port match the
configured FORGEJO_URL. The command refuses cross-host absolute URLs rather
than sending credentials to them.

-f key=value adds a JSON string field. -F key=value adds a typed field:
true/false become JSON booleans and integer-looking values become JSON
numbers; floats remain strings. -F key=@file reads the file contents as the
value. key[]=value accumulates array entries. For GET and HEAD, -f/-F pairs
become URL-encoded query parameters instead of a JSON body. --input - reads
the full request body from stdin and overrides any -f/-F body fields.

Argument forms:
  -f key=value       add key as a JSON string field, or query param on GET/HEAD
  -F key=value       add key as a typed field: bools and integers become JSON
                     values; floats remain strings
  -F key=@file       read file contents as the value
  key[]=value        accumulate repeated values into a JSON array
  --input -          read the raw request body from stdin
  -                  stdin sentinel accepted only after --input

## Global Flags

These inherited flags apply to commands in this group unless a command defines a local flag with the same name.

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| `--dry-run` | `bool` | `false` | print mutating requests instead of sending them |
| `--jq` | `string` | `""` | filter JSON output through a jq expression (implies --json) |
| `--json` | `bool` | `false` | output raw JSON from the server |
| `--limit` | `int` | `-1` | max items for list verbs (0 = fetch all pages; default: per-verb) |
| `--verbose` | `bool` | `false` | log requests to stderr (tokens are never logged) |

## forgejo api

Use: `forgejo api [METHOD] <path> [-f key=val]... [-F key=val]... [--input -]`

| Name | Type | Default | Help |
| :--- | :--- | :--- | :--- |
| _None_ |  |  |  |

