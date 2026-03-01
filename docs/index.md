# LazyWorktree

<div class="mint-hero">
  <p class="mint-kicker">LazyWorktree</p>
  <p>Easy Git worktree management for the terminal. LazyWorktree helps you create isolated development environments, keep context per branch, and move quickly across parallel tasks.</p>
</div>

<div class="mint-card-grid">
  <a class="mint-card" href="getting-started.md">
    <strong>Quickstart</strong>
    <span>Get running in minutes and jump directly to selected worktrees from your shell.</span>
  </a>
  <a class="mint-card" href="installation.md">
    <strong>Installation</strong>
    <span>Install with Homebrew, Arch Linux AUR, or from source.</span>
  </a>
</div>

## Key Features

### Worktree Management

- Create, rename, remove, absorb, and prune worktrees.
- Switch quickly without branch checkout churn.
- Keep each task isolated with its own filesystem context.

### Development Workflow

- Stage, unstage, commit, and inspect diffs in-terminal.
- View CI checks and logs from GitHub/GitLab workflows.
- Open PR/MR links and worktree-aware tooling from the same view.

### Customisation

- Configure globally, per repository, or via CLI overrides.
- Add custom command bindings for shell/tmux/zellij actions.
- Run `.wt` initialisation and termination workflows with trust controls.

## What are Git Worktrees?

Git worktrees allow multiple checkouts of the same repository simultaneously.
This is useful when you need to:

- Work on several branches in parallel.
- Review PR/MR changes without stashing local work.
- Keep long-running features isolated from urgent fixes.

## Why LazyWorktree?

- Keyboard-first terminal UX.
- PR/MR and CI-aware status surface.
- Built-in notes and taskboard per worktree.
- Strong scripting and multiplexer integration.

## Next Steps

<div class="mint-card-grid">
  <a class="mint-card" href="configuration.md">
    <strong>Configuration</strong>
    <span>Theme, refresh, pager/editor, lifecycle, and naming settings.</span>
  </a>
  <a class="mint-card" href="keybindings.md">
    <strong>Key Bindings</strong>
    <span>Global shortcuts and pane-specific controls.</span>
  </a>
  <a class="mint-card" href="custom-commands.md">
    <strong>Custom Commands</strong>
    <span>Bind your own shell and multiplexer actions.</span>
  </a>
  <a class="mint-card" href="cli.md">
    <strong>CLI Usage</strong>
    <span>Create, list, rename, and execute commands from scripts.</span>
  </a>
</div>
