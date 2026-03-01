# CLI Overview

Use the CLI to manage worktrees non-interactively or in scripts.

## Available Commands

- `lazyworktree list`
- `lazyworktree create`
- `lazyworktree delete`
- `lazyworktree rename`
- `lazyworktree exec`

Global config overrides:

```bash
lazyworktree --worktree-dir ~/worktrees
lazyworktree --config lw.theme=nord --config lw.sort_mode=active
```

## Command Pages

- [`list`](list.md)
- [`create`](create.md)
- [`delete`](delete.md)
- [`rename`](rename.md)
- [`exec`](exec.md)
- [`commands` reference](commands.md)
- [`flags` reference](flags.md)

For complete generated references, use:

- [CLI Commands Reference](commands.md)
- [CLI Flags Reference](flags.md)
- `lazyworktree --help`
- `man lazyworktree`
