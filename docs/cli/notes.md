# notes

Read worktree notes with machine-readable output.

## Subcommands

### `notes get <worktree>`

Return note metadata for one exact worktree.

| Flag | Type | Usage |
| --- | --- | --- |
| `--json` | `bool` | Output result as JSON including metadata |

Use this command when you want a stable JSON note payload. The older `note show` command remains available for plain-text note output and editor-driven note changes.

## Examples

```bash
lazyworktree notes get my-feature --json
lazyworktree worktrees resolve --name my-feature --json | jq -r '.worktree.path'
lazyworktree notes get /path/to/worktree --json
```
