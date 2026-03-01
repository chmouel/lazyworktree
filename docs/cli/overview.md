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

## Next Steps

<div class="mint-card-grid">
  <a class="mint-card" href="create.md">
    <strong>CLI create</strong>
    <span>Generate worktrees from branch, issue, or PR contexts.</span>
  </a>
  <a class="mint-card" href="exec.md">
    <strong>CLI exec</strong>
    <span>Run shell/custom commands in selected worktrees.</span>
  </a>
  <a class="mint-card" href="flags.md">
    <strong>CLI flags reference</strong>
    <span>Global and command-level flags generated from source.</span>
  </a>
  <a class="mint-card" href="commands.md">
    <strong>CLI commands reference</strong>
    <span>Usage, args, aliases, and generated per-command flags.</span>
  </a>
</div>
