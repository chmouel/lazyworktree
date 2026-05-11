# worktrees

Discover, resolve, and inspect worktrees with stable machine-readable output.

## Subcommands

### `worktrees list`

List worktrees for the current repository.

Useful flags:

| Flag | Type | Usage |
| --- | --- | --- |
| `--json` | `bool` | Output result as JSON |
| `--main` | `bool` | Show only the main worktree |
| `--limit` | `int` | Limit the number of returned worktrees |
| `--no-agent` | `bool` | Skip agent session data |

### `worktrees resolve`

Resolve a worktree name, branch, path, or cwd into a canonical worktree path.

Useful flags:

| Flag | Type | Usage |
| --- | --- | --- |
| `--json` | `bool` | Output result as JSON |
| `--name` | `string` | Resolve by name, basename, or branch |
| `--path` | `string` | Resolve by path or a path inside the worktree |
| `--cwd` | `string` | Resolve by working directory |
| `--no-agent` | `bool` | Skip agent session data |

### `worktrees get <worktree>`

Read one exact worktree by canonical path, name, or branch.

### `worktrees context <worktree>`

Read note and agent-session context for one worktree.

Useful flags:

| Flag | Type | Usage |
| --- | --- | --- |
| `--json` | `bool` | Output result as JSON |
| `--include` | `string` | Comma-separated sections: `notes`, `agents` |

## Examples

```bash
lazyworktree worktrees list --json
lazyworktree worktrees list --main --json
lazyworktree worktrees resolve --name my-feature --json
lazyworktree worktrees resolve --path ~/worktrees/repo/my-feature --json
lazyworktree worktrees get my-feature --json
lazyworktree worktrees context my-feature --json
lazyworktree worktrees context my-feature --include notes --json
```

## Agent workflow

For coding agents, the usual order is:

1. `lazyworktree worktrees list --json`
2. `lazyworktree worktrees resolve --name <name> --json`
3. `lazyworktree worktrees get <path-or-name> --json`
4. `lazyworktree worktrees context <path-or-name> --json`
