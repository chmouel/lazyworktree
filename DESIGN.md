# LazyWorktree Architecture & Design

**Last updated**: 2026-02-02
**Status**: Living document

## Overview

LazyWorktree is a TUI for Git worktree management built with [BubbleTea](https://github.com/charmbracelet/bubbletea). The architecture follows the Elm-inspired Model-Update-View pattern, with clear separation between UI logic, Git operations, and configuration.

## Component Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                         CLI Layer                                │
│                   cmd/lazyworktree/                              │
│             (cobra commands, flags, subcommands)                 │
└───────────────────────────┬─────────────────────────────────────┘
                            │
┌───────────────────────────▼─────────────────────────────────────┐
│                         TUI Layer                                │
│                     internal/app/                                │
│  ┌──────────────────────────────────────────────────────────┐   │
│  │  Model (app.go)                                          │   │
│  │  - State management (state/)                             │   │
│  │  - Screen manager (screen/)                              │   │
│  │  - Services (services/)                                  │   │
│  └──────────────┬───────────────────────────────────────────┘   │
│                 │                                                │
│  ┌──────────────▼───────────────────────────────────────────┐   │
│  │  Update (handlers.go)                                    │   │
│  │  - Key bindings                                          │   │
│  │  - Message routing                                       │   │
│  │  - Async command dispatch                                │   │
│  └──────────────┬───────────────────────────────────────────┘   │
│                 │                                                │
│  ┌──────────────▼───────────────────────────────────────────┐   │
│  │  View (render_*.go)                                      │   │
│  │  - Lipgloss styling                                      │   │
│  │  - Theme integration                                     │   │
│  │  - Layout rendering                                      │   │
│  └──────────────────────────────────────────────────────────┘   │
└───────────────────────────┬─────────────────────────────────────┘
                            │
┌───────────────────────────▼─────────────────────────────────────┐
│                      Services Layer                              │
│                    internal/git/                                 │
│  ┌──────────────────────────────────────────────────────────┐   │
│  │  Git Service (service.go)                                │   │
│  │  - Semaphore-based concurrency (54-97)                   │   │
│  │  - Git CLI wrapper                                       │   │
│  │  - PR/MR integration (GitHub/GitLab API)                 │   │
│  │  - CI status polling                                     │   │
│  └──────────────────────────────────────────────────────────┘   │
└───────────────────────────┬─────────────────────────────────────┘
                            │
┌───────────────────────────▼─────────────────────────────────────┐
│                   Configuration Layer                            │
│                    internal/config/                              │
│             (5-level cascade, theme management)                  │
└─────────────────────────────────────────────────────────────────┘
```

## Directory Structure

```
.
├── cmd/lazyworktree/          # CLI entry point (cobra commands)
├── internal/
│   ├── app/                   # TUI application (BubbleTea)
│   │   ├── screen/            # Modal screen management (stack-based)
│   │   ├── services/          # UI services (debounce, cache, etc.)
│   │   ├── state/             # Application state
│   │   ├── handlers/          # Message handlers
│   │   ├── app.go             # Main Model implementation
│   │   ├── handlers.go        # Update function
│   │   ├── render_*.go        # View functions
│   │   └── messages.go        # BubbleTea messages
│   ├── git/                   # Git operations and API integration
│   │   ├── service.go         # Git operations, GitHub and GitLab API (PRs/MRs, CI, issues)
│   │   └── worktree.go        # Worktree operations
│   ├── config/                # Configuration cascade
│   │   ├── config.go          # Main config struct
│   │   └── load.go            # YAML loading + precedence
│   ├── theme/                 # Theme system (21 built-in themes)
│   ├── models/                # Data structures (WorktreeInfo, PRInfo)
│   ├── security/              # TOFU security for .wt files
│   ├── commands/              # Custom command execution
│   ├── log/                   # Logging utilities
│   └── utils/                 # Shared utilities
└── .claude/                   # LLM agent configuration
    ├── rules/                 # Coding rules
    ├── agents/                # Custom agents
    └── skills/                # Custom skills
```

## Key Abstractions

### 1. BubbleTea Model-Update-View Pattern

**Model** (internal/app/app.go:77-210)
```go
type Model struct {
    state         *state.State      // Application state
    screenManager *screen.Manager   // Modal screen stack
    table         table.Model       // Worktree table
    statusFiles   []StatusFile      // Git status
    gitService    *git.Service      // Git operations
    config        *config.AppConfig // Configuration
    theme         *theme.Theme      // Active theme
    // ... more fields
}
```

**Update** (internal/app/handlers.go:30-622)
- Receives key events and custom messages
- Routes to specialized handlers (handlers/*.go)
- Dispatches async commands via tea.Cmd
- Returns updated Model and commands

**View** (internal/app/render_panes.go, render_components.go)
- Pure functions: Model → string
- Uses lipgloss for styling
- No business logic, only presentation

### 2. Git Service Abstraction

**Design**: Wrapper around git CLI (not go-git library)

**Location**: internal/git/service.go

**Concurrency model** (54-97):
```go
// Semaphore-based concurrency control
semaphore := make(chan struct{}, limit)  // limit = NumCPU * 2 (4-32)
for i := 0; i < limit; i++ {
    semaphore <- struct{}{}  // Pre-fill tokens
}

// Operations acquire/release tokens
func (s *Service) operation() {
    <-s.semaphore              // Acquire
    defer func() { s.semaphore <- struct{}{} }()  // Release
    // ... run git command
}
```

**Why git CLI wrapper?**
- Direct control over git flags and behavior
- Easier to match user's git configuration
- Simpler error handling (parse stderr)
- Avoids go-git API surface complexity

**Trade-off**: Requires git binary installed (acceptable for TUI tool)

### 3. Screen Management System

**Design**: Stack-based modal overlay system

**Location**: internal/app/screen/manager.go

**Stack behavior**:
```go
type Manager struct {
    current Screen        // Active screen (top of stack)
    stack   []Screen      // Previous screens
}

// Push new screen (e.g., help screen over main view)
manager.Push(helpScreen)  // Pushes current to stack, sets new current

// Pop screen (e.g., close help, restore main view)
manager.Pop()  // Removes current, restores previous
```

**Screen types** (internal/app/screen/types.go):
- TypeNone: No modal active (main view visible)
- TypeHelp: Help overlay
- TypeWorktreeCreate: Worktree creation menu
- TypeCustomCommand: Command selection
- TypeConfirm: Confirmation dialog
- TypeInput: Text input prompt
- TypeFileList: File selection
- TypeCommandHistory: Command history

**Usage pattern**:
```go
// Show help screen
m.screenManager.Push(NewHelpScreen(m.config))

// User presses 'q' to close help
if key == "q" && m.screenManager.Type() == TypeHelp {
    m.screenManager.Pop()  // Returns to main view
}
```

### 4. Theme System

**Design**: 11 color fields, 21 built-in themes, custom theme support

**Location**: internal/theme/theme.go

**Theme struct** (28-40):
```go
type Theme struct {
    Accent    lipgloss.Color  // Primary accent (selections, highlights)
    AccentFg  lipgloss.Color  // Text on accent background
    AccentDim lipgloss.Color  // Dimmed accent
    Border    lipgloss.Color  // Active borders
    BorderDim lipgloss.Color  // Inactive borders
    MutedFg   lipgloss.Color  // Muted text (comments, hints)
    TextFg    lipgloss.Color  // Primary text
    SuccessFg lipgloss.Color  // Success indicators
    WarnFg    lipgloss.Color  // Warnings
    ErrorFg   lipgloss.Color  // Errors
    Cyan      lipgloss.Color  // Special highlight
}
```

**Built-in themes** (42-66):
- dracula, dracula-light
- catppuccin-mocha, catppuccin-latte
- solarized-dark, solarized-light
- gruvbox-dark, gruvbox-light
- nord, monokai
- tokyo-night, one-dark, one-light
- rose-pine, rose-pine-dawn
- everforest-dark, everforest-light
- ayu-mirage, kanagawa
- narna, clean-light, modern

**Custom themes** (internal/config/config.go:56-70):
- Defined in YAML config
- Can inherit from base theme
- Override individual fields

**Critical rule**: All UI rendering MUST use theme fields, never hardcoded colors.

### 5. Configuration Cascade

**Design**: 5-level precedence (highest to lowest)

**Location**: internal/config/load.go

**Precedence order**:
1. **CLI flags** (--worktree-dir, --theme, etc.)
2. **Environment variables** (LAZYWORKTREE_*)
3. **Repo-local config** (.lazyworktree.yaml in repo root)
4. **Global config** (~/.config/lazyworktree/config.yaml)
5. **Built-in defaults** (hardcoded in AppConfig)

**Implementation**:
```go
// Load in reverse order (defaults → global → repo → env → flags)
config := defaultConfig()
mergeGlobalConfig(config)
mergeRepoConfig(config)
mergeEnvVars(config)
mergeFlags(config)
```

**Key configuration options** (internal/config/config.go:72-116):
- WorktreeDir: Where to create worktrees
- Theme: Active theme name
- CustomCommands: User-defined keybindings
- CustomCreateMenu: Custom worktree creation entries
- AutoRefresh: Periodic worktree refresh
- TrustMode: Security for .wt init/terminate scripts
- InitCommands/TerminateCommands: Hooks for worktree lifecycle

## Architecture Trade-offs

### Why BubbleTea over alternatives?

**Chosen**: BubbleTea (Elm architecture)

**Alternatives considered**:
- tcell + custom event loop
- tview (higher-level widgets)
- termbox-go (low-level)

**Rationale**:
- ✅ Declarative Model-Update-View pattern (easier to reason about)
- ✅ Built-in message passing (async commands via tea.Cmd)
- ✅ Strong ecosystem (bubbles, lipgloss for styling)
- ✅ Testable (pure functions, no global state)
- ❌ Steeper learning curve (Elm concepts)
- ❌ More verbose than imperative approaches

**Trade-off**: Verbosity acceptable for better testability and maintainability.

### Why git CLI wrapper over go-git?

**Chosen**: Wrapper around git CLI (internal/git/service.go)

**Alternatives considered**:
- go-git (pure Go implementation)
- libgit2 bindings

**Rationale**:
- ✅ User's git config respected (aliases, hooks, credentials)
- ✅ Simpler error handling (parse stderr strings)
- ✅ Exact control over flags (--worktree, --branch, etc.)
- ✅ Easier to match git CLI behavior
- ❌ Requires git binary installed
- ❌ Cross-platform command differences (Windows)

**Trade-off**: Dependency on git binary acceptable for a git-focused TUI.

### Why semaphore-based concurrency?

**Chosen**: Buffered channel semaphore (internal/git/service.go:77-83)

**Alternatives considered**:
- sync.WaitGroup + goroutine pool
- Worker pool pattern
- errgroup.Group

**Rationale**:
- ✅ Simple token-based limiting (NumCPU * 2, capped 4-32)
- ✅ No goroutine leaks (tokens always released via defer)
- ✅ Backpressure (blocks if limit reached)
- ✅ Easy to reason about (acquire token → run → release token)
- ❌ No priority queuing (all operations equal)

**Trade-off**: Simplicity over advanced scheduling features.

### Why 5-level config cascade?

**Chosen**: CLI > Env > Repo > Global > Defaults

**Alternatives considered**:
- Viper (auto-merge all sources)
- Single config file
- CLI flags only

**Rationale**:
- ✅ Flexibility (per-repo overrides, global defaults)
- ✅ Explicit precedence (no surprises)
- ✅ Matches user expectations (git-like config hierarchy)
- ❌ More complex loading logic
- ❌ Debugging "which config applied?" can be tricky

**Trade-off**: Complexity acceptable for per-repo customization needs.

## Import Dependency Graph

```
cmd/lazyworktree
    ↓
internal/app
    ├→ internal/config (config loading)
    ├→ internal/theme (theme system)
    ├→ internal/git (git service)
    ├→ internal/models (data structures)
    ├→ internal/security (TOFU for .wt files)
    ├→ internal/commands (custom command execution)
    └→ internal/log (logging)

internal/git
    ├→ internal/config (git service config)
    ├→ internal/models (WorktreeInfo, PRInfo)
    ├→ internal/commands (command execution)
    └→ internal/log (logging)

internal/config
    ├→ internal/theme (theme definitions)
    └→ internal/utils (filesystem utilities)

internal/theme
    └→ (no internal dependencies, uses lipgloss)
```

**Critical rule**: Avoid circular dependencies (theme avoids importing config, uses CustomThemeData struct instead).

## Critical Files Reference

### Core TUI Implementation
- `internal/app/app.go:77-210` - Model struct definition
- `internal/app/handlers.go:30-622` - Update function (message routing)
- `internal/app/render_panes.go:18-284` - Main view rendering
- `internal/app/messages.go:1-215` - BubbleTea message types

### Git Service
- `internal/git/service.go` - Service struct, semaphore, GitHub and GitLab API (PRs/MRs, CI, issues)
- `internal/git/worktree.go:1-450` - Worktree operations (list, create, remove)

### Screen Management
- `internal/app/screen/manager.go:1-74` - Screen stack implementation
- `internal/app/screen/types.go:1-45` - Screen type constants
- `internal/app/screen/help.go:1-120` - Help screen implementation
- `internal/app/screen/worktree_create.go:1-180` - Worktree creation menu

### Configuration
- `internal/config/config.go:72-116` - AppConfig struct
- `internal/config/load.go:1-350` - YAML loading + cascade logic
- `internal/config/custom_theme.go:1-80` - Custom theme loading

### Theme System
- `internal/theme/theme.go:28-40` - Theme struct
- `internal/theme/theme.go:68-850` - Built-in theme definitions
- `internal/theme/apply.go:1-120` - Theme application + custom theme merging

### Key Handlers
- `internal/app/handlers.go:100-250` - Main key event routing
- `internal/app/worktree_operations.go:1-450` - Worktree CRUD handlers
- `internal/app/base_selection.go:1-650` - Table selection + navigation
- `internal/app/app_git.go:1-380` - Git command handlers (stage, commit, diff)

## Development Workflow

### Adding a New Screen Type

1. Define screen type constant in `internal/app/screen/types.go`
2. Create screen implementation file `internal/app/screen/<name>.go`
3. Implement Screen interface:
   ```go
   type Screen interface {
       Type() Type
       Update(tea.Msg) (Screen, tea.Cmd)
       View(width, height int, theme *theme.Theme) string
   }
   ```
4. Add handler in `internal/app/handlers.go` to Push/Pop screen
5. Add rendering logic in View() method using theme fields

### Adding a New Configuration Option

1. Add field to AppConfig struct in `internal/config/config.go`
2. Add default value in `defaultConfig()` in `internal/config/load.go`
3. Add CLI flag in `cmd/lazyworktree/flags.go`
4. Add environment variable mapping in `internal/config/load.go`
5. Update README.md, lazyworktree.1 man page, and help screen

### Adding a New Theme

1. Add theme constant in `internal/theme/theme.go`
2. Define theme function returning *Theme struct
3. Add to AvailableThemes map
4. Test all 11 color fields render correctly
5. Ensure sufficient contrast for accessibility

### Adding a Git Operation

1. Add method to `internal/git/service.go`
2. Use semaphore pattern:
   ```go
   func (s *Service) Operation(ctx context.Context) error {
       <-s.semaphore  // Acquire
       defer func() { s.semaphore <- struct{}{} }()  // Release
       // ... git command logic
   }
   ```
3. Add message type in `internal/app/messages.go`
4. Add handler in `internal/app/handlers.go`
5. Add tests in `internal/git/service_test.go`

## Testing Strategy

### Unit Tests
- Pure functions (theme calculations, config merging)
- Git command parsing (status, worktree list)
- Screen state transitions

### Integration Tests
- Full Model-Update-View cycle (`internal/app/integration_test.go`)
- Git service with mocked git commands
- Config cascade with temp files

### Test Coverage
- Target: 55%+ coverage (current baseline)
- Focus: Critical paths (git operations, config loading, key handlers)
- Skip: Devicons, file type detection (low risk)

## Performance Considerations

### Debouncing
- Details pane updates debounced (200ms) to avoid excessive git calls
- File search input debounced (150ms)

### Caching
- PR data cached (30s TTL) to avoid API rate limits
- Worktree details cached (2s TTL) for rapid navigation
- CI check results cached (30s TTL)

### Concurrency
- Semaphore limits concurrent git operations (NumCPU * 2, max 32)
- Async BubbleTea commands for non-blocking UI
- Goroutine-safe notification system (notifyOnce deduplication)

### Optimization Targets
- Worktree list refresh: <200ms for 50 worktrees
- PR status update: <500ms (network-dependent)
- Screen transitions: <16ms (60 FPS)

## Security Model

### .wt File Execution (TOFU)

**Design**: Trust On First Use for init/terminate scripts

**Location**: internal/security/security.go

**Flow**:
1. First run: Hash .wt file, store in ~/.config/lazyworktree/trusted_scripts
2. Subsequent runs: Compare hash, warn if changed
3. User must re-approve if hash mismatch

**Trust modes** (internal/config/config.go:88):
- `ask`: Prompt every time (safe, annoying)
- `first-use`: Trust on first use, warn on changes (default)
- `always`: Always trust (dangerous)

### Command Injection Prevention
- All custom commands validated for shell metacharacters
- User confirmation required for commands with backticks, pipes, etc.

## Future Architecture Considerations

### Potential Enhancements
- **Plugin system**: Custom screens, commands, themes
- **Remote worktrees**: SSH-based worktree management
- **Diff viewer**: Built-in diff rendering (not relying on external pager)
- **Graph view**: Commit graph visualization
- **Multi-repo**: Manage multiple repos simultaneously

### Scalability Concerns
- **Large repos**: Worktree list >1000 entries (pagination needed)
- **PR polling**: GitHub API rate limits (need smarter caching)
- **CI logs**: Large log files (streaming/pagination)

---

## Quick Reference: Where to Find Things

| Need to... | Look in... |
|-----------|-----------|
| Add a keybinding | `internal/app/handlers.go` (Update function) |
| Add a screen type | `internal/app/screen/<name>.go` + `types.go` |
| Modify git operations | `internal/git/service.go` |
| Add config option | `internal/config/config.go` + `load.go` |
| Add theme | `internal/theme/theme.go` |
| Change UI rendering | `internal/app/render_*.go` |
| Add custom command | User's config YAML (CustomCommands section) |
| Understand message flow | `internal/app/messages.go` → `handlers.go` |
| Debug concurrency | `internal/git/service.go:77-97` (semaphore) |

## Maintenance Notes

- **Update this document** when:
  - New major component added (e.g., plugin system)
  - Architectural decision reversed (e.g., switch to go-git)
  - Directory structure reorganized
  - New abstraction pattern introduced

- **Review quarterly**:
  - Verify line number references still accurate
  - Check if new components need documentation
  - Update dependency graph if imports changed

- **Keep concise**: This document is for LLM agents and developers to quickly understand architecture, not comprehensive API documentation.
