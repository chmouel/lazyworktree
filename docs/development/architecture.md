# Architecture

An overview of LazyWorktree's internal design, adapted for contributors and anyone curious about how the application is structured.

<div class="mint-callout">
  <p><strong>Use this page when:</strong> you want to understand how the codebase is organised, where to find things, or how to add new features.</p>
</div>

!!! note
    This page is a curated summary. The full, authoritative design document lives at [`DESIGN.md`](https://github.com/chmouel/lazyworktree/blob/main/DESIGN.md) in the repository root.

## Overview

LazyWorktree is a terminal UI for Git worktree management, built with [BubbleTea](https://github.com/charmbracelet/bubbletea). The architecture follows the Elm-inspired **Model-Update-View** pattern:

- **Model** holds application state
- **Update** processes events and returns a new model
- **View** renders the model to a string (pure function, no side effects)

## Component Architecture

```
┌────────────────────────────────────────────────┐
│                  CLI Layer                      │
│            cmd/lazyworktree/                    │
│       (Cobra commands, flags, subcommands)      │
└────────────────────┬───────────────────────────┘
                     │
┌────────────────────▼───────────────────────────┐
│                  TUI Layer                      │
│              internal/app/                      │
│                                                 │
│   Model (app.go)                                │
│     ├── State management (state/)               │
│     ├── Screen manager (screen/)                │
│     └── Services (services/)                    │
│                                                 │
│   Update (handlers.go)                          │
│     └── Key bindings, message routing,          │
│         async command dispatch                  │
│                                                 │
│   View (render_*.go)                            │
│     └── Lipgloss styling, theme integration     │
└────────────────────┬───────────────────────────┘
                     │
┌────────────────────▼───────────────────────────┐
│               Services Layer                    │
│             internal/git/                       │
│   Git CLI wrapper, PR/MR integration,           │
│   CI status polling, semaphore concurrency      │
└────────────────────┬───────────────────────────┘
                     │
┌────────────────────▼───────────────────────────┐
│            Configuration Layer                  │
│             internal/config/                    │
│      5-level cascade, theme management          │
└─────────────────────────────────────────────────┘
```

## Directory Structure

| Directory | Purpose |
| --- | --- |
| `cmd/lazyworktree/` | CLI entry point (Cobra commands) |
| `internal/app/` | TUI application (BubbleTea model, handlers, views) |
| `internal/app/screen/` | Modal screen management (stack-based overlays) |
| `internal/app/services/` | UI services (debounce, cache) |
| `internal/app/state/` | Application state |
| `internal/app/handlers/` | Message handlers |
| `internal/git/` | Git CLI wrapper, GitHub/GitLab API integration |
| `internal/config/` | Configuration struct, YAML loading, cascade logic |
| `internal/theme/` | Theme system (21 built-in themes, custom theme support) |
| `internal/models/` | Data structures (`WorktreeInfo`, `PRInfo`) |
| `internal/security/` | TOFU trust model for `.wt` files |
| `internal/commands/` | Custom command execution |
| `internal/log/` | Logging utilities |

## Key Abstractions

### Model-Update-View

The **Model** struct (`internal/app/app.go`) holds all application state: the screen manager, worktree table, git service, configuration, and active theme.

The **Update** function (`internal/app/handlers.go`) receives key events and custom messages, routes them to specialised handlers, dispatches async commands via `tea.Cmd`, and returns the updated model.

**View** functions (`internal/app/render_*.go`) are pure: they take a model and return a string. No business logic lives in the view layer. All styling uses theme fields via Lipgloss.

### Git Service

The git service (`internal/git/service.go`) wraps the `git` CLI rather than using a Go library. This ensures the user's git configuration (aliases, hooks, credentials) is respected and gives precise control over flags.

Concurrency is managed with a buffered-channel semaphore (capacity: `NumCPU * 2`, capped between 4 and 32). Every git operation acquires a token before running and releases it on completion, providing simple backpressure without goroutine leaks.

### Screen Management

Screens use a stack-based modal system (`internal/app/screen/manager.go`). The main worktree view sits at the bottom of the stack; modals (help, worktree creation, command palette, confirmation dialogs) are pushed on top and popped when dismissed.

Screen types include: help overlay, worktree creation menu, custom command selection, confirmation dialogs, text input prompts, file selection, and command history.

### Theme System

Themes define 11 colour fields (accent, border, text, success, warning, error, and others). There are 21 built-in themes covering popular palettes (Dracula, Catppuccin, Solarized, Gruvbox, Nord, Tokyo Night, and more). Custom themes can inherit from a base theme and override individual fields via YAML configuration.

**Rule**: all UI rendering must use theme fields — never hardcoded colours.

### Configuration Cascade

Configuration follows a 5-level precedence (highest to lowest):

1. CLI flags (`--theme`, `--worktree-dir`, etc.)
2. Environment variables (`LAZYWORKTREE_*`)
3. Repository-local config (`.lazyworktree.yaml` in the repo root)
4. Global config (`~/.config/lazyworktree/config.yaml`)
5. Built-in defaults

This mirrors Git's own configuration hierarchy, allowing per-repository overrides whilst maintaining sensible global defaults.

## Architecture Trade-offs

| Decision | Rationale |
| --- | --- |
| **BubbleTea** over tcell/tview | Declarative Model-Update-View is easier to reason about and test, despite greater verbosity |
| **Git CLI wrapper** over go-git | Respects user git config, simpler error handling; requires git binary (acceptable for a git TUI) |
| **Semaphore concurrency** over worker pools | Simple token-based limiting with no goroutine leaks; trades away priority queuing |
| **5-level config cascade** over single file | Matches git-like expectations for per-repo customisation; adds loading complexity |

## Import Dependency Graph

```
cmd/lazyworktree
    ↓
internal/app
    ├→ internal/config
    ├→ internal/theme
    ├→ internal/git
    ├→ internal/models
    ├→ internal/security
    ├→ internal/commands
    └→ internal/log

internal/git
    ├→ internal/config
    ├→ internal/models
    ├→ internal/commands
    └→ internal/log

internal/config
    ├→ internal/theme
    └→ internal/utils
```

Circular dependencies are avoided by design — for example, `theme` does not import `config`.

## Performance Considerations

| Mechanism | Detail |
| --- | --- |
| **Debouncing** | Details pane: 200 ms, file search input: 150 ms |
| **Caching** | PR data: 30 s TTL, worktree details: 2 s TTL, CI checks: 30 s TTL |
| **Concurrency** | Semaphore limits concurrent git operations (`NumCPU * 2`, max 32) |
| **Targets** | Worktree list refresh < 200 ms, PR status < 500 ms, screen transitions < 16 ms |

## Security Model

`.wt` files can execute arbitrary commands, so LazyWorktree uses a **Trust On First Use** (TOFU) model:

1. On first encounter, the `.wt` file is hashed and the user is asked to **Trust**, **Block**, or **Cancel**
2. On subsequent runs, the hash is compared — a mismatch triggers re-evaluation
3. Trust decisions are stored in `~/.local/share/lazyworktree/trusted.json`

Three trust modes are available: `tofu` (default — prompt on first use, warn on changes), `never` (block all execution), and `always` (no prompts — use with caution).

## Testing Strategy

| Layer | Approach |
| --- | --- |
| Unit tests | Pure functions: theme calculations, config merging, git output parsing |
| Integration tests | Full Model-Update-View cycle, git service with mocked commands, config cascade with temp files |
| Coverage target | 55 %+ focused on critical paths (git operations, config loading, key handlers) |

## Adding New Features

### New screen type

1. Add a constant in `internal/app/screen/types.go`
2. Create `internal/app/screen/<name>.go` implementing the `Screen` interface (`Type()`, `Update()`, `View()`)
3. Add a handler in `internal/app/handlers.go` to push/pop the screen
4. Use theme fields for all rendering

### New configuration option

1. Add a field to `AppConfig` in `internal/config/config.go`
2. Set the default in `defaultConfig()` in `internal/config/load.go`
3. Add a CLI flag in `cmd/lazyworktree/flags.go`
4. Map the environment variable in `internal/config/load.go`
5. Update `README.md`, `lazyworktree.1`, and the help screen

### New theme

1. Add a theme constant in `internal/theme/theme.go`
2. Define the theme function returning a `*Theme` struct with all 11 colour fields
3. Register in `AvailableThemes`
4. Test contrast across light and dark terminal backgrounds

### New git operation

1. Add a method to `internal/git/service.go` using the semaphore pattern
2. Define a message type in `internal/app/messages.go`
3. Add a handler in `internal/app/handlers.go`
4. Add tests in `internal/git/service_test.go`
