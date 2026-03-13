# CLI `describe`

Emit the CLI command structure as machine-readable JSON. Designed for coding agents and scripts that need to discover flags and subcommands without parsing `--help` output.

## Synopsis

```
lazyworktree describe [command] [subcommand]
lazyworktree describe --all
```

## Examples

```bash
# Describe the full CLI (root + all subcommands)
lazyworktree describe
lazyworktree describe --all

# Describe a specific command
lazyworktree describe create
lazyworktree describe list

# Describe a nested subcommand
lazyworktree describe note show

# Pipe to jq for targeted queries
lazyworktree describe create | jq '.flags[].name'
lazyworktree describe create | jq '.flags[] | select(.name == "json")'
lazyworktree describe note show | jq '.flags[].name'
```

## Output Format

```json
{
  "name": "create",
  "usage": "Create a new worktree",
  "args_usage": "[worktree-name]",
  "flags": [
    {
      "name": "from-branch",
      "aliases": ["branch"],
      "usage": "Create worktree from branch (defaults to current branch)",
      "type": "string"
    },
    {
      "name": "json",
      "usage": "Output result as JSON",
      "type": "bool"
    }
  ],
  "subcommands": []
}
```

### Fields

| Field | Description |
|---|---|
| `name` | Command name |
| `usage` | Short description |
| `args_usage` | Positional argument description |
| `flags` | Array of flag descriptors |
| `subcommands` | Nested subcommands (recursive) |

### Flag Fields

| Field | Description |
|---|---|
| `name` | Primary flag name (without `--`) |
| `aliases` | Alternative names |
| `usage` | Flag description |
| `type` | `string`, `bool`, `int`, `string-slice`, or `unknown` |
| `default` | Default value (omitted when zero/false/empty) |

## Flags

| Flag | Description |
|---|---|
| `--all` | Describe all commands (same as no args) |

## Behaviour Notes

- Without arguments, describes the full command tree from the root.
- With one argument, describes that top-level command.
- With two arguments, describes the nested subcommand (e.g. `note show`).
- `--all` is equivalent to no arguments.
- Unknown command names produce an error on stderr and exit non-zero.
- Output is always valid JSON — suitable for `jq`, Python `json.loads`, etc.

## For Coding Agents

The recommended introspection hierarchy is:

1. **`describe`** — authoritative, structured JSON
2. **`--json` flags** — machine-readable output from mutating commands
3. **`--help`** — human-readable fallback (do not parse programmatically)

```bash
# Discover all flags for create before invoking it
lazyworktree describe create | jq '.flags[] | {name, type, usage}'

# Check if a flag exists
lazyworktree describe list | jq '.flags[] | select(.name == "no-agent")'
```
