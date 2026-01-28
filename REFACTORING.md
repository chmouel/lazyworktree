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

## Phase 2: Migrate Complex Screens (NEAR COMPLETE)

**Goal:** Migrate screens to `screen.Manager`, reducing `handleScreenKey` from ~494 lines incrementally.

### Completed Wave 1 Migrations:

| Screen | New File | Status |
|--------|----------|--------|
| WelcomeScreen | `screen/welcome.go` | ✅ Migrated |
| TrustScreen | `screen/trust.go` | ✅ Migrated |
| CommitScreen | `screen/commit.go` | ✅ Migrated |
| PRSelectionScreen | `screen/pr_select.go` | ✅ Migrated |
| IssueSelectionScreen | `screen/issue_select.go` | ✅ Migrated |

### Completed Wave 2A Migrations:

| Screen | New File | Status |
|--------|----------|--------|
| ChecklistScreen | `screen/checklist.go` | ✅ Migrated |
| ListSelectionScreen | `screen/list_select.go` | ⚠️ Partially Migrated (CI checks only) |

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
   - `issueSelectionScreen *IssueSelectionScreen` removed from Model
   - `issueSelectionSubmit func(*models.IssueInfo) tea.Cmd` removed from Model
   - `checklistScreen *ChecklistScreen` removed from Model
   - `checklistSubmit func([]ChecklistItem) tea.Cmd` removed from Model
   - `listScreen *ListSelectionScreen` removed from Model
   - `listSubmit func(selectionItem) tea.Cmd` removed from Model
   - `listScreenCIChecks []*models.CICheck` removed from Model
   - `inputScreen *InputScreen` removed from Model (Wave 2C)
   - `inputSubmit func(string, bool) (tea.Cmd, bool)` removed from Model (Wave 2C)

5. **New screen files**:
   - `screen/welcome.go` - with `OnRefresh` and `OnQuit` callbacks
   - `screen/trust.go` - with `OnTrust`, `OnBlock`, and `OnCancel` callbacks
   - `screen/commit.go` - with `CommitMeta` type and viewport scrolling
   - `screen/pr_select.go` - with `OnSelect` and `OnCancel` callbacks
   - `screen/issue_select.go` - with `OnSelect` and `OnCancel` callbacks
   - `screen/checklist.go` - with `OnSubmit` and `OnCancel` callbacks, multi-select with checkboxes
   - `screen/list_select.go` - with `OnSelect`, `OnEnter`, `OnCtrlV`, `OnCtrlR`, `OnCursorChange` callbacks
   - `screen/input.go` - with `OnSubmit`, `OnCancel`, `OnCheckboxToggle`, `Validate` callbacks, history navigation (Wave 2C)

6. **Test updates**:
   - Updated tests to use `screenManager.IsActive()` and `screenManager.Type()` instead of legacy `currentScreen` field
   - Fixed `TestHandleOpenPRsLoaded` in `app_git_test.go`
   - Fixed `TestIntegrationCreateFromPRValidationErrors` in `integration_error_flow_test.go`
   - Fixed `TestHandleOpenPRsLoadedAsyncCreation` in `pr_worktree_test.go`
   - Tests now directly call screen callbacks instead of legacy methods like `prSelectionSubmit()`

7. **Theme switching**:
   - Updated theme switching to work with screen manager for PRSelectionScreen, IssueSelectionScreen, and ChecklistScreen
   - Added alias `appscreen` to avoid naming conflict with `screen` parameter in functions

8. **ChecklistScreen migration** (Wave 2A):
   - Migrated prune merged worktrees to use screen manager with callbacks
   - Removed `case screenChecklist` from `app_screens.go` and `renderer.go`
   - Added helper function `convertToScreenChecklistItems()` for type conversion
   - Updated tests to use `screenManager.IsActive()` and `screenManager.Type()`

9. **ListSelectionScreen partial migration** (Wave 2A):
   - CI checks usage fully migrated to screen manager with special callbacks (`OnEnter`, `OnCtrlV`, `OnCtrlR`)
   - **Remaining usage sites still use legacy pattern:**
     - Theme selection (app_screens.go:651-672) - uses `onCursorChange` for live preview
     - Cherry-pick target selection (app_screens.go:857-883)
     - Base branch selection (base_selection.go:81-130)
     - Base selection options (base_selection.go:156-160)
     - Commit selection (base_selection.go:195-238)
   - Legacy fields (`listScreen`, `listSubmit`, `listScreenCIChecks`) remain for these usage sites

### Wave 2B: Complete ListSelectionScreen Migration (COMPLETED ✅)

**Goal:** Finish migrating all ListSelectionScreen usage sites to screen manager.

**Completed:**
1. ✅ Theme selection (`app_screens.go:643-685`) - uses `OnCursorChange` for live preview
2. ✅ Cherry-pick target selection (`app_screens.go:867-918`) - converts worktree list to items
3. ✅ Base selection screens (`base_selection.go`):
   - `showBaseSelection()` (lines 36-149)
   - `showBranchSelection()` (lines 176-208)
   - `showCommitSelection()` (lines 218-305)
   - `showCheckoutOrCreatePrompt()` (lines 451-488)
4. ✅ Removed legacy fields from Model:
   - `listScreen *ListSelectionScreen`
   - `listSubmit func(selectionItem) tea.Cmd`
   - `listScreenCIChecks []*models.CICheck`
5. ✅ Removed legacy case blocks:
   - `case screenListSelect` in `app_screens.go` (65 lines removed)
   - `case screenListSelect` in `renderer.go` (6 lines removed)
   - Theme update code for `m.listScreen` removed
6. ✅ Removed `screenListSelect` constant from `screens.go`
7. ✅ Updated all tests to use screen manager pattern:
   - `app_screens_test.go` (3 tests)
   - `base_selection_test.go` (12 tests)
   - `command_runner_test.go` (4 tests)
   - `worktree_operations_test.go` (1 test)

**Pattern established:**
```go
// Creating a list selection screen
items := make([]appscreen.SelectionItem, len(data))
for i, d := range data {
    items[i] = appscreen.SelectionItem{
        ID:          d.id,
        Label:       d.label,
        Description: d.desc,
    }
}
listScreen := appscreen.NewListSelectionScreen(items, title, placeholder, noResults, width, height, initialID, theme)
listScreen.OnSelect = func(item appscreen.SelectionItem) tea.Cmd {
    // Handle selection
    return someCommand(item.ID)
}
listScreen.OnCancel = func() tea.Cmd {
    return nil
}
m.screenManager.Push(listScreen)
```

**Special callbacks supported:**
- `OnCursorChange(SelectionItem)` - For live preview (e.g., theme selection)
- `OnEnter(SelectionItem) tea.Cmd` - Override default selection behavior (e.g., CI checks)
- `OnCtrlV(SelectionItem) tea.Cmd` - View logs (CI checks)
- `OnCtrlR(SelectionItem) tea.Cmd` - Restart job (CI checks)

**Stats:**
- **Lines removed:** ~140 (legacy code + case blocks)
- **Files updated:** 10 (source + test files)
- **Tests passing:** All (`make sanity` ✅)
- **Coverage:** ListSelectionScreen now 100% migrated

### Wave 2C: InputScreen Migration (COMPLETED ✅)

**Goal:** Migrate InputScreen from legacy pattern to screen manager with callback-based validation.

**New file created:**
- `screen/input.go` - InputScreen with callback-based pattern (~200 lines)

**Key design changes:**
- Legacy pattern: `inputSubmit func(value string, checked bool) (tea.Cmd, bool)` where bool indicates close
- New pattern: `OnSubmit func(value string, checked bool) tea.Cmd` with `ErrorMsg` field for validation
- Screen stays open if `ErrorMsg` is set after `OnSubmit` is called

**Migrated 11 usage sites:**

| # | File | Function | Features |
|---|------|----------|----------|
| 1 | base_selection.go | showFreeformBaseInput | Validation |
| 2 | base_selection.go | showBranchNameInput | Validation |
| 3 | base_selection.go | showWorktreeNameForExistingBranch | Validation |
| 4 | worktree_sync.go | showUpstreamInput | Validation |
| 5 | messages.go | PR input (in showInfo callback) | Validation |
| 6 | messages.go | PR input (direct) | Validation |
| 7 | messages.go | Issue input | Validation |
| 8 | worktree_operations.go | showCreateFromChangesInput | Validation |
| 9 | worktree_operations.go | handleCreateFromCurrentReady | Checkbox + AI name toggle |
| 10 | worktree_operations.go | showRenameWorktree | Validation |
| 11 | app_screens.go | showRunCommand | History navigation |

**Removed legacy code:**
- `InputScreen` struct from `screens.go` (24 lines)
- `NewInputScreen`, `SetValidation`, `SetFuzzyFinder`, `SetHistory`, `Init`, `Update`, `View` methods (~254 lines)
- `SetCheckbox` method (7 lines)
- `inputScreen` and `inputSubmit` fields from Model
- `case screenInput:` blocks from `app_screens.go` and `renderer.go`
- Legacy tests `TestInputScreenInit` and `TestInputScreenUpdate` from `screens_test.go`

**Test files updated:**
- `app_screens_test.go` (2 tests)
- `integration_error_flow_test.go` (3 test sections)
- `base_selection_test.go` (multiple tests - in previous session)
- `worktree_operations_test.go` (multiple tests - in previous session)
- `worktree_sync_test.go` (multiple tests - in previous session)

**Special handling:**
- Added `createFromCurrentInputScreen *screen.InputScreen` field to Model for checkbox toggle callback
- Updated `aiBranchNameGeneratedMsg` handler to use `createFromCurrentInputScreen`

**Stats:**
- **Lines removed:** ~285 (legacy struct + methods + case blocks)
- **Files updated:** 12 (source + test files)
- **Tests passing:** All (`make sanity` ✅)

### Wave 2D: CommandPaletteScreen Migration (COMPLETED ✅)

**Goal:** Migrate CommandPaletteScreen from legacy pattern to screen manager with callback-based action execution.

**Completed:**
1. ✅ Created `screen/palette.go` (317 lines) implementing Screen interface
2. ✅ Migrated single usage site `showCommandPalette()` (app_screens.go:259-315)
3. ✅ Changed pattern: `paletteSubmit func(action) Cmd` → `OnSelect func(action) Cmd` callbacks
4. ✅ Removed legacy fields from Model:
   - `paletteScreen *CommandPaletteScreen`
   - `paletteSubmit func(string) tea.Cmd`
5. ✅ Removed legacy code from screens.go (~260 lines):
   - `type CommandPaletteScreen struct`
   - `type paletteItem struct`
   - All associated methods (NewCommandPaletteScreen, Init, Update, View, etc.)
6. ✅ Removed `screenPalette` constant from screenType enum
7. ✅ Removed helper functions from helpers.go (filterPaletteItems, paletteMatchScore)
8. ✅ Removed legacy case blocks:
   - `case screenPalette` in app_screens.go (keyboard handling)
   - Theme update code for `m.paletteScreen`
   - Legacy escape key handling
9. ✅ Updated 16 tests across 4 test files:
   - app_screens_test.go (9 tests updated, 3 obsolete tests removed)
   - screens_test.go (6 obsolete tests removed, 1 rendering test updated)
   - integration_flow_test.go (2 tests updated)
   - integration_test.go (1 test updated)
   - helpers_test.go (5 obsolete tests removed)
10. ✅ Added `appscreen` import alias to integration test files

**Features preserved:**
- Section navigation (cursor skips non-selectable section headers)
- MRU (Most Recently Used) tracking with deduplication
- Fuzzy filtering with live updates
- Tmux/Zellij session integration
- Custom command execution
- Action registry integration

**Pattern established:**
```go
// Creating command palette
items := make([]appscreen.PaletteItem, len(paletteItems))
for i, src := range paletteItems {
    items[i] = appscreen.PaletteItem{
        ID:          src.ID,
        Label:       src.Label,
        Description: src.Description,
        IsSection:   src.IsSection,
        IsMRU:       src.IsMRU,
    }
}
paletteScreen := appscreen.NewCommandPaletteScreen(items, width, height, theme)
paletteScreen.OnSelect = func(action string) tea.Cmd {
    m.addToPaletteHistory(action)  // CRITICAL for MRU tracking
    // ... handle action types (tmux-attach, zellij-attach, custom, registry)
}
paletteScreen.OnCancel = func() tea.Cmd { return nil }
m.screenManager.Push(paletteScreen)
```

**Stats:**
- **Commit:** `85e5ed9` - "refactor: Migrate CommandPaletteScreen to screen manager"
- **Lines removed:** ~417 (net reduction after adding new implementation)
- **Files modified:** 12 (1 new, 11 changed)
- **Tests updated:** 16 functions
- **Tests passing:** All (`make sanity` ✅)

### Wave 2E: HelpScreen Migration (COMPLETED ✅)

**Goal:** Migrate HelpScreen from legacy pattern to screen manager with search functionality.

**Completed:**
1. ✅ Created `screen/help.go` (~513 lines) implementing Screen interface
2. ✅ Migrated single usage site `handlers.go:525-528` (? key)
3. ✅ Added command palette Help action
4. ✅ No legacy case blocks to remove (already removed)
5. ✅ No legacy fields to remove (already removed)
6. ✅ No screenHelp constant to remove (already removed)
7. ✅ Theme switching already implemented via screen manager iteration
8. ✅ Test `TestHelpScreen` already uses screen manager pattern
9. ✅ All tests passing (`make sanity` ✅)

**Features preserved:**
- Full help text with keybindings and tips
- Search functionality with live filtering and highlighting
- Viewport scrolling (j/k, Ctrl+D/U, g/G)
- Custom commands integration
- Icon support (conditional)
- Theme-aware rendering

**Pattern established:**
```go
// Creating help screen
helpScreen := appscreen.NewHelpScreen(m.view.WindowWidth, m.view.WindowHeight, m.config.CustomCommands, m.theme, m.config.IconsEnabled())
m.screenManager.Push(helpScreen)
// Screen closes on 'q' or 'esc' (returns nil from Update())
```

**Stats:**
- **Lines added:** ~513 (new help.go implementation)
- **Lines removed:** ~0 (legacy code already removed in previous session)
- **Files modified:** 11 (1 new, 10 changed)
- **Tests passing:** All (`make sanity` ✅)

### Wave 2F: ConfirmScreen and InfoScreen Migration (COMPLETED ✅)

**Goal:** Migrate ConfirmScreen and InfoScreen from ResultChan pattern to callback pattern, completing the simple screen migrations.

**Completed:**

#### ConfirmScreen Migration (100% Complete ✅)
1. ✅ Updated `screen/confirm.go` - Converted from `ResultChan` to callbacks
   - Removed `ResultChan chan bool` field
   - Added `OnConfirm func() tea.Cmd` callback
   - Added `OnCancel func() tea.Cmd` callback
   - Updated `Update()` method to invoke callbacks instead of channel sends
2. ✅ Migrated 7 usage sites to screen manager with callbacks:
   - `worktree_sync.go:128` - Sync choice confirmation (uses both OnConfirm and OnCancel)
   - `worktree_operations.go:497` - Delete worktree confirmation
   - `worktree_operations.go:782` - Absorb worktree confirmation
   - `app_screens.go:473` - Theme save confirmation
   - `app_git.go:91-95` - File deletion confirmation
   - `app.go:596` - Branch deletion confirmation
3. ✅ Removed legacy code from `screens.go`:
   - Removed `ConfirmScreen` struct and all methods (~160 lines)
   - Removed `screenConfirm` constant
4. ✅ Removed legacy Model fields:
   - `confirmScreen *ConfirmScreen`
   - `confirmAction func() tea.Cmd`
   - `confirmCancel func() tea.Cmd`
5. ✅ Removed legacy case blocks:
   - `case screenConfirm` in `app_screens.go` (40 lines removed)
   - `case screenConfirm` in `renderer.go` (4 lines removed)
6. ✅ Updated tests:
   - `screen/manager_test.go` - Tests callback invocation instead of channels
   - `worktree_sync_test.go` - Uses screen manager checks
   - `worktree_operations_test.go` - Uses screen manager checks
   - `app_git_test.go` - Uses screen manager checks
   - `handlers_test.go` - Uses screen manager checks

**Pattern established:**
```go
// Old pattern (channel-based)
m.confirmScreen = NewConfirmScreen(message, m.theme)
m.confirmAction = func() tea.Cmd { /* ... */ }
m.confirmCancel = func() tea.Cmd { /* ... */ }
m.currentScreen = screenConfirm

// New pattern (callback-based with screen manager)
confirmScreen := appscreen.NewConfirmScreen(message, m.theme)
confirmScreen.OnConfirm = func() tea.Cmd { /* ... */ }
confirmScreen.OnCancel = func() tea.Cmd { /* ... */ }
m.screenManager.Push(confirmScreen)
```

#### InfoScreen Migration (100% Complete ✅)
1. ✅ Updated `screen/info.go` - Converted from `ResultChan` to callback
   - Removed `ResultChan chan bool` field
   - Added `OnClose func() tea.Cmd` callback
   - Updated `Update()` method to invoke callback instead of channel send
2. ✅ Updated `showInfo()` helper in `app_screens.go`:
   - Now uses screen manager internally
   - All existing call sites work without changes
3. ✅ Converted 7 direct InfoScreen instantiations to use helper:
   - `app.go:766, 774` - Tmux/Zellij session info
   - `messages.go:172` - Absorb failure message
   - `worktree_operations.go:735, 744, 761, 768` - Error messages
4. ✅ Removed legacy code from `screens.go`:
   - Removed `InfoScreen` struct and all methods (~155 lines)
   - Removed `screenInfo` constant
5. ✅ Removed legacy Model fields:
   - `infoScreen *InfoScreen`
   - `infoAction tea.Cmd`
6. ✅ Removed legacy case blocks:
   - `case screenInfo` in `app_screens.go` (17 lines removed)
   - `case screenInfo` in `renderer.go` (4 lines removed)
7. ✅ **All test files updated:**
   - `screens_test.go` - Uses `appscreen.NewConfirmScreen*` with capitalized fields
   - `pr_worktree_test.go` - Uses screen manager checks
   - `app_screens_test.go` - Uses screen manager and `m.View()` for rendering
   - `worktree_sync_test.go` - Uses screen manager checks
   - `app_external_test.go` - Uses screen manager checks
   - `app_git_test.go` - Uses screen manager checks
   - `handlers_test.go` - Uses screen manager checks
   - `integration_error_flow_test.go` - Uses screen manager checks
   - `integration_flow_test.go` - Uses screen manager checks
   - `integration_test.go` - Uses screen manager checks
   - `worktree_operations_test.go` - Uses screen manager checks
   - `base_selection_test.go` - Uses screen manager checks
   - `command_runner_test.go` - Uses screen manager checks

**Helper pattern (preferred):**
```go
// All code should use the helper (no direct instantiation)
m.showInfo(message, actionCmd)

// Helper internally does:
func (m *Model) showInfo(message string, action tea.Cmd) {
    infoScreen := appscreen.NewInfoScreen(message, m.theme)
    infoScreen.OnClose = func() tea.Cmd { return action }
    m.screenManager.Push(infoScreen)
}
```

**Migration pattern for tests:**
```go
// Old pattern
if m.currentScreen != screenInfo { ... }
if m.infoScreen == nil { ... }
msg := m.infoScreen.message

// New pattern
if !m.screenManager.IsActive() || m.screenManager.Type() != appscreen.TypeInfo { ... }
infoScr := m.screenManager.Current().(*appscreen.InfoScreen)
msg := infoScr.Message  // Capitalized field
```

**Stats:**
- **Lines removed:** ~332 (legacy code + case blocks for both screens)
- **Files modified:** 15 (source + test files)
- **Tests passing:** ✅ All tests pass (`make sanity` ✅)

#### CommitFilesScreen (Deferred)

| Screen | Complexity | Status | Notes |
|--------|-----------|--------|-------|
| CommitFilesScreen | High (tree-based) | Deferred | ~660 lines, tree navigation, requires separate task |

**Benefits achieved:**
- Consistent callback pattern across all simple screens
- Eliminated channel select statements (clearer control flow)
- Better testability with callbacks vs channels
- Unified screen management system

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

| Screen | New File | Lines | Callbacks | Status |
|--------|----------|-------|-----------|--------|
| CommandPaletteScreen | `screen/palette.go` | 317 | `OnSelect func(string) tea.Cmd` | ✅ Complete |
| HelpScreen | `screen/help.go` | ~460 | None | Pending |
| InputScreen | `screen/input.go` | ~400+ | `OnSubmit func(string, bool) (tea.Cmd, bool)` | ✅ Complete |

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
| `app.go` | ~910 | Refactored (confirm/info fields removed, screenManager used) |
| `screens.go` | ~2753 | Reduced (~487 lines removed: InputScreen + Wave 2F) |
| `app_screens.go` | ~980 | Simplified (confirm/info case blocks removed) |
| `handlers.go` | 1043 | Updated for screen manager |
| `ci.go` | 330 | Could become service (Phase 3) |
| `worktree_operations.go` | 864 | Could become service (Phase 3) |
| `screen/screen.go` | 83 | Core interface and types |
| `screen/manager.go` | 74 | Manager implementation |
| `screen/confirm.go` | 143 | ConfirmScreen (migrated to callbacks - Wave 2F) |
| `screen/info.go` | 81 | InfoScreen (migrated to callbacks - Wave 2F) |
| `screen/loading.go` | 169 | LoadingScreen |
| `screen/welcome.go` | 102 | WelcomeScreen migrated |
| `screen/trust.go` | 114 | TrustScreen migrated |
| `screen/commit.go` | 141 | CommitScreen migrated |
| `screen/pr_select.go` | 316 | PRSelectionScreen migrated |
| `screen/issue_select.go` | 269 | IssueSelectionScreen migrated |
| `screen/checklist.go` | 327 | ChecklistScreen migrated |
| `screen/list_select.go` | 305 | ListSelectionScreen migrated |
| `screen/input.go` | ~200 | InputScreen migrated (NEW) |
| `screen/help.go` | 513 | HelpScreen migrated (NEW) |
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
