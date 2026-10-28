# Architecture

An overview of LazyWorktree's internal design for contributors who need to
understand where behaviour lives and how the main subsystems fit together.

<div class="mint-callout">
  <p><strong>Refer to this page when:</strong> you want to understand package ownership, TUI versus CLI responsibilities, or where to make architectural changes.</p>
</div>

!!! note
    This page is a curated summary. The root [`DESIGN.md`](https://github.com/chmouel/lazyworktree/blob/main/DESIGN.md) remains the authoritative architecture note for the repository.

## Overview

LazyWorktree is a Git worktree manager with two entry paths built on shared
subsystems:

- an interactive Bubble Tea TUI for day-to-day workflow
- CLI subcommands for scripting and non-interactive operations

At a high level:

1. `cmd/lazyworktree` is the process entrypoint.
2. `internal/bootstrap` builds the command graph and selects TUI or CLI mode.
3. `internal/app` owns the Bubble Tea model, rendering, modal screens, and
   interactive workflows.
4. `internal/cli` owns non-interactive orchestration for commands such as
   create, rename, delete, list, exec, and note.
5. Shared packages such as `internal/git`, `internal/config`,
   `internal/security`, `internal/theme`, and `internal/multiplexer` support
   both modes.

## Runtime Shape

```text
cmd/lazyworktree/main.go
        |
        v
internal/bootstrap
  |-----------------------------|
  |                             |
  v                             v
TUI path                     CLI path
internal/app                 internal/cli
  |                             |
  |---- uses shared subsystems --|
                |
                v
  internal/config / internal/git / internal/security
  internal/theme  / internal/multiplexer / internal/models
                       / internal/app/services (agent session registry)
```

## Repository Map

| Path | Responsibility |
| --- | --- |
| `cmd/lazyworktree/` | Process entrypoint |
| `internal/bootstrap/` | CLI graph, flags, mode selection, TUI startup |
| `internal/app/` | Bubble Tea model, updates, rendering, worktree workflows |
| `internal/app/screen/` | Modal screens such as help, palette, trust, commit, note, checklist |
| `internal/app/services/` | Shared TUI helpers for notes, status tree, CI cache, env expansion, pager, and watch support |
| `internal/app/state/` | Small typed state containers for view and pending action state |
| `internal/cli/` | Non-interactive operations and interactive selectors for subcommands |
| `internal/git/` | Git, GitHub, and GitLab integration plus diff/pager helpers |
| `internal/config/` | App config loading, CLI override merging, `.wt` repo config loading |
| `internal/security/` | TOFU trust persistence for `.wt` files |
| `internal/theme/` | Built-in themes, custom theme support, terminal background detection |
| `internal/multiplexer/` | tmux, zellij, Herdr, shell, and container command assembly |
| `internal/models/` | Shared data types such as worktrees, PRs, issues, CI checks, and status files |
| `internal/commands/` | Helper commands such as symlink setup |
| `internal/testenv/` | Test-only helper; packages whose tests run git call it from `TestMain` so a hook's `GIT_DIR` cannot point them at the real repository |

## TUI Architecture

The default `lazyworktree` command launches the Bubble Tea program from
`internal/bootstrap`.

`internal/app` follows the Model-Update-View pattern:

- Model coordinates configuration, theme, caches, selection state, async
  work, and shared services.
- Update logic lives primarily in `handlers.go` and focused feature files
  such as `worktree_operations.go`, `app_git.go`, `ci.go`, and
  `worktree_sync.go`.
- View rendering is split across `render_*.go`, `layout.go`, `renderer.go`,
  and table style helpers.
- Modal screens are handled by `internal/app/screen` through a stack-based
  screen manager.

The model state is intentionally split by concern:

- UI widgets and active screen stack
- worktree, status, notes, and log data currently on screen
- view and layout state
- injected services such as git, trust, status tree, and watch support

## CLI Architecture

The CLI layer is not a thin wrapper over the TUI. `internal/cli` has its own
orchestration for:

- worktree creation and removal
- PR and issue selection
- trust checks for `.wt` lifecycle hooks
- interactive fallback with `fzf` or prompt-based selection
- editor, pager, and command execution helpers

This separation keeps the TUI responsive while still allowing automation
workflows to reuse the same domain services and config rules.

## Key Design Decisions

### Bootstrap owns command wiring

The root command graph is assembled in `internal/bootstrap` using
`urfave/cli/v3`, not in the entrypoint package. This keeps `main.go` trivial and
centralises startup behaviour:

- global flags are defined once
- subcommands share the same config-loading path
- TUI startup and CLI execution apply overrides consistently

### The TUI is stateful, but screens are modular

The main model is large because it coordinates panes, caches, selections, async
effects, and external integrations. To keep that manageable, modal behaviour is
pushed into `internal/app/screen`.

The screen manager is a simple stack:

- push a modal over the main view
- update only the active screen while it is open
- pop back to the previous screen on dismissal

Current screen types include confirm, info, input, textarea, note view, help,
trust, welcome, commit, palette, diff, PR selection, issue selection, generic
list selection, loading, commit files, checklist, taskboard, and commit
message.

### Git integration is intentionally CLI-first

`internal/git.Service` wraps external commands rather than embedding a pure Go
Git implementation.

Reasons:

- exact Git behaviour matters more than abstraction purity
- user credentials and forge tooling already live in the shell environment
- the same service can call `git`, `gh`, and `glab`
- errors and output are easier to align with real command-line behaviour

The service uses a buffered-channel semaphore to cap concurrent git work at
`NumCPU * 2`, clamped to the range `4..32`.

### Configuration is layered through files, git config, and runtime overrides

The current config model is:

1. built-in defaults from `DefaultConfig()`
2. YAML config from the standard config location or an explicit config file
3. Git global config (`git config --global lw.*`)
4. Git local config for the current repository (`git config --local lw.*`)
5. runtime CLI overrides applied by bootstrap flags and `--config`

Important details:

- terminal background detection chooses a default theme only when no theme was
  set by config sources
- repository lifecycle hooks are loaded separately from a `.wt` file, not from a
  repo-local YAML app config
- there is no `LAZYWORKTREE_*` application config cascade

### External tooling is a first-class capability

LazyWorktree is designed to cooperate with the surrounding development
environment instead of replacing it.

That shows up in several places:

- custom commands run in the shell with worktree-specific environment variables
- tmux, zellij, and Herdr session scripts or layouts are generated from
  config rather than hardcoded
- optional container execution wraps commands for Docker or Podman
- diff and CI output can flow through configured pager commands
- note and branch name generation can be delegated to external scripts

The command environment contract is shared across TUI and CLI paths so both
modes speak the same worktree-aware variable vocabulary.

### `.wt` lifecycle hooks use TOFU

Repository hooks are loaded from a `.wt` file in the repo root. Because these
commands can execute arbitrary shell code, LazyWorktree stores a hash-based
trust decision in:

- `$XDG_DATA_HOME/lazyworktree/trusted.json`, or
- `~/.local/share/lazyworktree/trusted.json`

Supported trust modes are:

- `tofu`: prompt on first use and when the file changes
- `never`: refuse to run `.wt` commands
- `always`: run without prompting

## Data and Control Flow

### TUI flow

1. Bootstrap loads config, applies runtime overrides, and starts Bubble Tea.
2. `app.NewModel` creates the git service, trust manager, status helpers,
   theme, and initial UI state.
3. User input becomes Bubble Tea messages.
4. The active screen handles input first when a modal is open.
5. Otherwise the main update loop routes the event to worktree, git, status,
   log, notes, CI, or palette handlers.
6. Long-running work runs via `tea.Cmd` and returns typed messages.
7. Render functions turn the current model state into the pane layout.

### CLI flow

1. Bootstrap loads config and applies CLI overrides.
2. A git service is created with the configured diff pager behaviour.
3. `internal/cli` performs validation, selection, trust checks, and worktree
   operations.
4. Shared helpers are reused for notes, command environments, repo config, and
   multiplexer integration where relevant.

## Working on the Codebase

When adding a new feature:

- prefer the most specific package rather than growing a catch-all helper file
- keep rendering theme-backed; avoid hardcoded colours
- keep command environment and escaping rules centralised so TUI and CLI do not
  drift apart
- update the root `DESIGN.md` when a top-level subsystem, precedence rule, or
  cross-package ownership boundary changes

## Testing Strategy

The test suite is broad and close to the implementation packages.

The current emphasis is:

- unit tests for config parsing, theme behaviour, trust logic, and helper
  functions
- service tests for git and multiplexer behaviour
- application tests for screen handling, layout, worktree operations, notes,
  diff flows, and command execution
- integration-style tests for end-to-end TUI and CLI flows

For new work, prefer targeted tests near the changed package before reaching for
broader integration coverage.
