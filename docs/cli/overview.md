# CLI Overview

Use the CLI to manage worktrees non-interactively, in scripts, and from coding agents.

## Available Commands

- `lazyworktree list`
- `lazyworktree create`
- `lazyworktree delete`
- `lazyworktree cleanup`
- `lazyworktree rename`
- `lazyworktree doctor`
- `lazyworktree worktrees ...`
- `lazyworktree notes get`
- `lazyworktree exec`
- `lazyworktree describe`

Global config overrides:

```bash
lazyworktree --worktree-dir ~/worktrees
lazyworktree --config lw.theme=nord --config lw.sort_mode=active
```

## Command Pages

- [`doctor`](doctor.md)
- [`worktrees`](worktrees.md)
- [`notes`](notes.md)
- [`list`](list.md)
- [`create`](create.md)
- [`delete`](delete.md)
- [`cleanup`](cleanup.md)
- [`rename`](rename.md)
- [`exec`](exec.md)
- [`commands` reference](commands.md)
- [`flags` reference](flags.md)

For complete generated references, use:

- [CLI Commands Reference](commands.md)
- [CLI Flags Reference](flags.md)
- `lazyworktree --help`
- `man lazyworktree`

## Machine-first workflow

For Codex or script automation, prefer this order:

1. `lazyworktree describe`
2. `lazyworktree doctor --json`
3. `lazyworktree worktrees list --json`
4. `lazyworktree worktrees resolve --name <name> --json`
5. `lazyworktree worktrees get <path-or-name> --json`
6. `lazyworktree worktrees context <path-or-name> --json`
7. `lazyworktree notes get <path-or-name> --json`
