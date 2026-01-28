# Refactoring Status

This document tracks the ongoing refactoring of the lazyworktree codebase.

## Current Status

**Branch:** `refactor`
**Base:** `main`
**Commits:** 5 completed + Phase 1 in progress

---

## Completed Refactoring (5 Commits)

### Commit 1: `5a3d781` - Extract application logic into separate files

Split the monolithic `app.go` (4000+ lines) into focused files:

| New File | Purpose | Lines |
|----------|---------|-------|
| `app_diff.go` | Diff routing and display | ~70 |
| `app_external.go` | Browser, editor, external commands | ~305 |
| `app_git.go` | PR data fetching, git operations | ~435 |
| `app_helpers.go` | Wrapper functions to services/util | ~330 |
| `app_nav.go` | Navigation labels and helpers | ~505 |
| `app_screens.go` | Screen key handling | ~1230 |
| `app_status.go` | Status pane management | ~485 |

### Commit 2: `b525740` - Migrate app state management to dedicated structures

Created `internal/app/state/` package:

| File | Contents |
|------|----------|
| `state/view.go` | `ViewState` struct with `ShowingFilter`, `ShowingSearch`, `FocusedPane`, `ZoomedPane`, `WindowWidth`, `WindowHeight`, `FilterTarget`, `SearchTarget` |
| `state/pending.go` | `PendingState` struct for deferred command execution |
| `state_aliases.go` | Type aliases (`searchTarget`, `filterTarget`) for backward compatibility |

**Pattern introduced:** `m.view.ShowingFilter` instead of `m.showingFilter`

### Commit 3: `aa54b94` - Move application logic to services and util packages

Created `internal/app/services/`:

| File | Functions |
|------|-----------|
| `services/environment.go` | `BuildCommandEnv()`, `ExpandWithEnv()`, `EnvMapToList()` |
| `services/executor.go` | `CommandExecutor` interface |
| `services/pager.go` | `PagerCommand()`, `EditorCommand()`, `PagerEnv()` |
| `services/persistence.go` | `LoadCache()`, `SaveCache()`, `LoadCommandHistory()`, `SaveCommandHistory()`, `LoadPaletteHistory()`, `SavePaletteHistory()` |

Created `internal/app/util/`:

| File | Functions |
|------|-----------|
| `util/git.go` | `SanitizePRURL()`, `GitURLToWebURL()` |
| `util/strings.go` | `CommitMeta`, `AuthorInitials()`, `ParseCommitMeta()` |

### Commit 4: `307c3e2` - Centralize diff handling logic

Created `internal/app/handlers/diff.go`:

| Type | Purpose |
|------|---------|
| `DiffRouter` | Consolidates diff routing with dependency injection |
| `WorktreeDiffParams` | Parameters for worktree-level diffs |
| `FileDiffParams` | Parameters for single file diffs |
| `CommitDiffParams` | Parameters for commit diffs |
| `CommitFileDiffParams` | Parameters for file-in-commit diffs |

Methods: `ShowDiff()`, `ShowFileDiff()`, `ShowCommitDiff()`, `ShowCommitFileDiff()`

### Commit 5: `f3b1dd8` - Refactor command palette logic

Created `internal/app/commands/`:

| File | Contents |
|------|----------|
| `commands/palette.go` | `PaletteItem`, `PaletteOptions`, `BuildPaletteItems()`, `buildMRUItems()` |
| `commands/registry.go` | `CommandAction`, `Registry`, handler structs (`WorktreeHandlers`, `GitHandlers`, `StatusHandlers`, `LogHandlers`, `NavigationHandlers`, `SettingsHandlers`), `Register*Actions()` functions |

---

## Phase 1: Screen Manager Foundation (COMPLETED)

**Goal:** Create a unified screen management system as a foundation for reducing `screens.go`.

### New files created:

| File | Purpose |
|------|---------|
| `screen/screen.go` | `Screen` interface, `Type` enum with 16 screen types |
| `screen/manager.go` | `Manager` struct with Push/Pop/Current/Clear/Set/StackDepth |
| `screen/confirm.go` | `ConfirmScreen` implementation of Screen interface |
| `screen/info.go` | `InfoScreen` implementation of Screen interface |
| `screen/loading.go` | `LoadingScreen` implementation of Screen interface |
| `screen/manager_test.go` | Comprehensive tests for manager and screen types |

---

## Phase 2: Migrate Complex Screens (IN PROGRESS)

**Goal:** Migrate screens to `screen.Manager`, reducing `handleScreenKey` from ~494 lines incrementally.

### Completed Wave 1 Migrations:

| Screen | New File | Status |
|--------|----------|--------|
| WelcomeScreen | `screen/welcome.go` | ✅ Migrated |
| TrustScreen | `screen/trust.go` | ✅ Migrated |
| CommitScreen | `screen/commit.go` | ✅ Migrated |
| PRSelectionScreen | `screen/pr_select.go` | ✅ Migrated |

### Changes Made:

1. **Added parallel path infrastructure** to `handleScreenKey`:
   - Screen manager is checked first
   - Migrated screens use `screenManager.Push()` and callbacks
   - Legacy screens continue to use the switch statement

2. **Updated renderer.go**:
   - Added screen manager rendering path with type-specific handling
   - WelcomeScreen and TrustScreen get full-screen centered layout
   - CommitScreen gets overlay popup with viewport resizing

3. **Updated handlers.go**:
   - Mouse handling for CommitScreen via screen manager

4. **Removed legacy fields**:
   - `welcomeScreen *WelcomeScreen` removed from Model
   - `commitScreen *CommitScreen` removed from Model
   - `trustScreen *TrustScreen` removed from Model
   - `prSelectionScreen *PRSelectionScreen` removed from Model
   - `prSelectionSubmit func(*models.PRInfo) tea.Cmd` removed from Model

5. **New screen files**:
   - `screen/welcome.go` - with `OnRefresh` and `OnQuit` callbacks
   - `screen/trust.go` - with `OnTrust`, `OnBlock`, and `OnCancel` callbacks
   - `screen/commit.go` - with `CommitMeta` type and viewport scrolling
   - `screen/pr_select.go` - with `OnSelect` and `OnCancel` callbacks

6. **Test updates**:
   - Updated tests to use `screenManager.IsActive()` and `screenManager.Type()` instead of legacy `currentScreen` field
   - Fixed `TestHandleOpenPRsLoaded` in `app_git_test.go`
   - Fixed `TestIntegrationCreateFromPRValidationErrors` in `integration_error_flow_test.go`
   - Fixed `TestHandleOpenPRsLoadedAsyncCreation` in `pr_worktree_test.go`
   - Tests now directly call screen callbacks instead of legacy methods like `prSelectionSubmit()`

7. **Theme switching**:
   - Updated theme switching to work with screen manager for PRSelectionScreen
   - Added alias `appscreen` to avoid naming conflict with `screen` parameter in functions

### Remaining Screens (Wave 2-4):

| Screen | Complexity | Status |
|--------|-----------|--------|
| HelpScreen | Medium | Pending |
| CommandPaletteScreen | Medium | Pending |
| InputScreen | High (history, checkbox, validation) | Pending |
| IssueSelectionScreen | Medium | Pending (similar to PRSelectionScreen) |
| ListSelectionScreen | High (CI check handling) | Pending |
| ChecklistScreen | Medium | Pending |
| CommitFilesScreen | High (tree-based) | Pending |
| ConfirmScreen | Low (uses channels) | Pending |
| InfoScreen | Low (uses channels) | Pending |

### Screen interface:
```go
type Screen interface {
    Update(msg tea.KeyMsg) (Screen, tea.Cmd)
    View() string
    Type() Type
}
```

### Manager struct:
```go
type Manager struct {
    current Screen
    stack   []Screen  // For nested screens
}

func NewManager() *Manager
func (m *Manager) Push(s Screen)
func (m *Manager) Pop() Screen
func (m *Manager) Current() Screen
func (m *Manager) IsActive() bool
func (m *Manager) Type() Type
func (m *Manager) Clear()
func (m *Manager) Set(s Screen)
func (m *Manager) StackDepth() int
```

### Integration:
- Added `screenManager *screen.Manager` to Model struct in `app.go`
- Manager is initialized in `NewModel()`
- Legacy screen pointers remain for backward compatibility during gradual migration

### Notes:
- The screen package types use exported fields (e.g., `Message`, `ResultChan`, `Thm`)
- Legacy types in `screens.go` use unexported fields (e.g., `message`, `result`, `thm`)
- This allows both systems to coexist during gradual migration
- The loading tips are now also available in `screen.LoadingTips`

---

## Current Architecture

```
internal/app/
├── app.go                    # Core Model struct (~935 lines)
├── app_diff.go               # Diff routing
├── app_external.go           # External commands
├── app_git.go                # Git/PR operations
├── app_helpers.go            # Service wrappers
├── app_nav.go                # Navigation helpers
├── app_screens.go            # Screen key handling
├── app_status.go             # Status pane
├── handlers.go               # Main key handling
├── screens.go                # Screen definitions (3524 lines - to be reduced)
├── ci.go                     # CI check logic
├── worktree_operations.go    # Worktree CRUD
├── worktree_sync.go          # Worktree synchronization
│
├── commands/
│   ├── palette.go            # Palette building
│   └── registry.go           # Command registry
│
├── handlers/
│   └── diff.go               # DiffRouter
│
├── screen/                   # NEW - Screen manager package
│   ├── screen.go             # Screen interface and Type enum
│   ├── manager.go            # Manager implementation
│   ├── manager_test.go       # Tests
│   ├── confirm.go            # ConfirmScreen
│   ├── info.go               # InfoScreen
│   └── loading.go            # LoadingScreen
│
├── services/
│   ├── environment.go        # Environment utilities
│   ├── executor.go           # Command execution interface
│   ├── pager.go              # Pager/editor utilities
│   └── persistence.go        # Cache and history
│
├── state/
│   ├── pending.go            # PendingState
│   └── view.go               # ViewState
│
├── util/
│   ├── git.go                # Git URL utilities
│   └── strings.go            # String parsing
│
└── state_aliases.go          # Type aliases
```

---

## Planned Refactoring

### Phase 2: Migrate Complex Screens (Next)

**Goal:** Migrate all remaining screens to `screen.Manager`, reducing `handleScreenKey` from ~494 lines to ~8 lines.

#### Migration Waves

**Wave 1: Simple Screens** (Establish patterns)

| Screen | New File | Lines | Callbacks |
|--------|----------|-------|-----------|
| WelcomeScreen | `screen/welcome.go` | ~50 | None |
| TrustScreen | `screen/trust.go` | ~120 | ResultChan |
| CommitScreen | `screen/commit.go` | ~140 | None |

**Wave 2: Selection Screens** (Similar patterns)

| Screen | New File | Lines | Callbacks |
|--------|----------|-------|-----------|
| ListSelectionScreen | `screen/list_select.go` | ~200 | `OnSelect func(selectionItem) tea.Cmd` |
| PRSelectionScreen | `screen/pr_select.go` | ~180 | `OnSelect func(*models.PRInfo) tea.Cmd` |
| IssueSelectionScreen | `screen/issue_select.go` | ~180 | `OnSelect func(*models.IssueInfo) tea.Cmd` |
| ChecklistScreen | `screen/checklist.go` | ~230 | `OnSubmit func([]ChecklistItem) tea.Cmd` |

**Wave 3: Complex Screens**

| Screen | New File | Lines | Callbacks |
|--------|----------|-------|-----------|
| CommandPaletteScreen | `screen/palette.go` | ~350 | `OnSelect func(string) tea.Cmd` |
| HelpScreen | `screen/help.go` | ~460 | None |
| InputScreen | `screen/input.go` | ~400+ | `OnSubmit func(string, bool) (tea.Cmd, bool)` |

**Wave 4: Tree-Based Screen**

| Screen | New File | Lines | Callbacks |
|--------|----------|-------|-----------|
| CommitFilesScreen | `screen/commit_files.go` | ~400 | Action callbacks for diff/navigation |

#### Implementation Strategy

**Step 1:** Add parallel path to `handleScreenKey`:
```go
func (m *Model) handleScreenKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
    // New path: delegate to screen manager
    if m.screenManager.IsActive() {
        screen, cmd := m.screenManager.Current().Update(msg)
        if screen == nil {
            m.screenManager.Pop()
        } else {
            m.screenManager.Set(screen)
        }
        return m, cmd
    }

    // Legacy path: existing switch (removed incrementally)
    switch m.currentScreen { ... }
}
```

**Step 2:** For each screen:
1. Create new file in `screen/` implementing `Screen` interface
2. Embed callbacks directly in screen struct
3. Update creation sites to use `m.screenManager.Push()`
4. Remove corresponding `case` from legacy switch
5. Run tests, verify functionality

**Step 3:** Final cleanup:
1. Remove legacy screen pointers from Model struct
2. Remove `currentScreen screenType` field
3. Remove entire legacy switch statement

#### Screen Template

```go
type ExampleScreen struct {
    Message    string
    Thm        *theme.Theme
    OnComplete func(result T) tea.Cmd  // Embedded callback
}

func (s *ExampleScreen) Type() Type { return TypeExample }

func (s *ExampleScreen) Update(msg tea.KeyMsg) (Screen, tea.Cmd) {
    switch msg.String() {
    case "enter":
        if s.OnComplete != nil {
            return nil, s.OnComplete(result)
        }
        return nil, nil
    case "esc", "q":
        return nil, nil
    }
    return s, nil
}

func (s *ExampleScreen) View() string { /* ... */ }
```

#### Target State

```go
func (m *Model) handleScreenKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
    if !m.screenManager.IsActive() {
        return m, nil
    }
    screen, cmd := m.screenManager.Current().Update(msg)
    if screen == nil {
        m.screenManager.Pop()
    }
    return m, cmd
}
```

### Phase 3: Service Extraction (Future)

**CI Service** - Extract from `ci.go`:
- CI cache management
- CI check fetching
- CI check sorting

**Worktree Operations Service** - Extract from `worktree_operations.go` (864 lines):
- Create/delete/rename operations
- Worktree lifecycle management

### Phase 4: Model Cleanup (Future)

Group the 50+ fields in Model struct:
```go
type Model struct {
    config *config.AppConfig
    git    *git.Service
    theme  *theme.Theme

    ui struct { ... }       // UI components
    screenManager *screen.Manager
    view    *state.ViewState
    pending *state.PendingState
    data struct { ... }     // Worktrees, status, logs
    cache struct { ... }    // Details, CI, divergence
    services struct { ... } // Trust, CI, executor
}
```

---

## Design Decisions

### Decision 1: Keep Nested State (`m.view.*`)

The nested state pattern (`m.view.ShowingFilter`, `m.view.FocusedPane`) is **kept** because:
- Already integrated with 70+ usages across the codebase
- Provides clear semantic grouping (UI state vs pending ops vs data)
- Allows `ViewState` and `PendingState` to be tested independently
- Reverting would require 145+ changes across 18 files

### Decision 2: Implement Screen Manager

The screen manager pattern is **implemented** because:
- `screenType` enum has 16 values and growing
- `currentScreen` checked in 145 places across 18 files
- Giant switch statements in `handleScreenKey` (500+ lines)
- Each screen has its own pointer in Model struct

### Decision 3: Gradual Migration with Coexistence

The screen package types coexist with legacy types because:
- Allows incremental migration without breaking existing code
- Each screen can be migrated independently
- Tests continue to pass throughout the migration
- Reduces risk of introducing bugs

---

## Verification

After each refactoring phase:
1. `make build` - Compilation check
2. `make sanity` - golangci-lint, gofumpt, go test
3. `go test -race ./internal/app/...` - Race detection
4. Manual testing of affected features
5. Verify test coverage stays above 67%

**Phase 1 verification:**
- `make sanity` passes with 0 issues
- All existing tests pass
- New screen package tests pass
- No breaking changes to existing functionality

**Wave 1 Extended verification (after PRSelectionScreen migration):**
- All tests updated to work with screen manager
- Legacy `prSelectionScreen` field and `prSelectionSubmit` callback removed
- Theme switching updated for screen manager
- Import alias `appscreen` used to avoid naming conflicts
- Test pattern established: use `screenManager.IsActive()` and `screenManager.Type()` instead of `currentScreen`
- Test pattern established: call screen callbacks directly instead of legacy helper methods

---

## File Statistics

| File | Lines | Status |
|------|-------|--------|
| `app.go` | 935 | Refactored (screenManager added, legacy fields removed) |
| `screens.go` | 3524 | Needs splitting (Phase 2) |
| `app_screens.go` | 1150 | Partially simplified (3 screens migrated) |
| `handlers.go` | 1043 | Updated for screen manager |
| `ci.go` | 330 | Could become service (Phase 3) |
| `worktree_operations.go` | 864 | Could become service (Phase 3) |
| `screen/screen.go` | 83 | Core interface and types |
| `screen/manager.go` | 74 | Manager implementation |
| `screen/confirm.go` | 143 | ConfirmScreen (key constants) |
| `screen/info.go` | 81 | InfoScreen |
| `screen/loading.go` | 169 | LoadingScreen |
| `screen/welcome.go` | 102 | WelcomeScreen migrated |
| `screen/trust.go` | 114 | TrustScreen migrated |
| `screen/commit.go` | 141 | CommitScreen migrated |
| `screen/pr_select.go` | 316 | PRSelectionScreen migrated |
| `screen/pr_select_test.go` | NEW | Tests for PRSelectionScreen |
| `screen/ui_helpers.go` | NEW | Shared UI helper functions |
| `screen/manager_test.go` | 195 | Tests |

---

## Test Coverage

Current coverage: **67.8%**

Test files for refactored code:
- `app_diff_test.go`
- `app_external_test.go`
- `app_git_test.go`
- `app_helpers_test.go`
- `app_nav_test.go`
- `app_screens_test.go`
- `app_status_test.go`
- `screen/manager_test.go` (NEW)
