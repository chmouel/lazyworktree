![Go](https://img.shields.io/badge/go-1.25%2B-blue) ![Coverage](https://img.shields.io/badge/Coverage-57.0%25-yellow)

# LazyWorktree - Easy Git worktree management for the terminal

<img align="right" width="180" height="180" alt="lw-logo" src="https://github.com/user-attachments/assets/77b63679-40b8-494c-a62d-19ccc39ac38e" />

LazyWorktree is a TUI for Git worktrees. It provides a keyboard-driven workflow
for creating, inspecting, and navigating worktrees within a repository.

Built with [BubbleTea](https://github.com/charmbracelet/bubbletea), it focuses on fast iteration, clear state visibility, and tight Git tooling integration.

## Screenshot

![lazyworktree screenshot](https://github.com/user-attachments/assets/229a4d6d-f26f-4b85-b909-fc28b9c524c2)

_See other [Screenshots below](#screenshots)_

## Features

* Worktree management: create, rename, remove, absorb, and prune merged worktrees.
* Powerful creation options:
  * From current branch, optionally with uncommitted changes.
  * Checkout existing branch or create a new branch from it.
  * From GitHub/GitLab issue with automatic branch naming.
  * From open GitHub/GitLab PR or MR.
* VIM style keybinding and a VSCode-like command palette (and as configurable
as emacs!).
* View CI logs from GitHub Actions.
* Display linked PR/MR, CI status, and checks.
* Stage, unstage, commit, edit, and diff files.
* View diffs in a pager with optional delta integration.
* Manage per-worktree tmux or zellij sessions.
* Cherry-pick commits between worktrees.
* Command palette with MRU-based navigation.
* Custom commands: define keybindings, tmux/zellij layouts, and per-repo workflows.
* Init/terminate hooks via `.wt` files with TOFU security.
* Shell integration: jump into worktrees and return to the last-used one.
* Automatic branch naming via scripts (e.g., LLM tools).

## Getting Started

* Install lazyworktree using one of the methods below
* Navigate to a Git repository in your terminal
* Run lazyworktree to launch the TUI
* Press ? to view the interactive help screen

## Requirements

* **Git**: 2.31+
* **Forge CLI**: `gh` or `glab` for PR/MR status

**Optional:**

* Nerd Font: Icons default to Nerd Font glyphs.

> [!IMPORTANT]
> If you see weird characters when starting lazyworktree, set `icon_set: text` or install a font patched with Nerd Font.

* delta: Syntax-highlighted diffs (recommended)
* lazygit: Full TUI git control
* tmux / zellij: Session management

**Build-time only:**

* Go 1.25+

## Installation

### Pre-built Binaries

Pre-built binaries are available in the [Releases](https://github.com/chmouel/lazyworktree/releases).

### 🍺 Homebrew (macOS)

```shell
brew tap chmouel/lazyworktree https://github.com/chmouel/lazyworktree
brew install lazyworktree --cask
```

#### macOS Gatekeeper

If macOS shows "Apple could not verify lazyworktree":

**Option 1:** System Settings > Privacy & Security > "Open Anyway"

**Option 2:** Remove quarantine attribute:

```bash
xattr -d com.apple.quarantine /opt/homebrew/bin/lazyworktree
```

### [Arch](https://aur.archlinux.org/packages/lazyworktree-bin)

```shell
yay -S lazyworktree-bin
```

### From Source

Direct installation:

```bash
go install github.com/chmouel/lazyworktree/cmd/lazyworktree@latest
```

Via cloning and building:

```bash
git clone https://github.com/chmouel/lazyworktree.git
cd lazyworktree
go build -o lazyworktree ./cmd/lazyworktree
```

## Shell Integration

Shell helpers change directory to the selected worktree on exit. Optional but recommended.

Zsh helpers are in `shell/functions.zsh`. See [./shell/README.md](./shell/README.md) for details.

## Key Bindings

| Key | Action |
| --- | --- |
| `Enter` | Jump to worktree (exit and cd) |
| `j`, `k` | Move selection up/down in lists and menus |
| `c` | Create new worktree (from branch, commit, PR/MR, or issue) |
| `m` | Rename selected worktree |
| `D` | Delete selected worktree |
| `d` | View diff in pager (worktree or commit, depending on pane) |
| `A` | Absorb worktree into main |
| `X` | Prune merged worktrees (refreshes PR data, checks merge status) |
| `!` | Run arbitrary command in selected worktree (with command history) |
| `v` | View CI checks (Enter opens in browser, Ctrl+v views logs in pager) |
| `o` | Open PR/MR in browser (or root repo in editor if main branch with merged/closed/no PR) |
| `ctrl+p`, `:` | Command palette |
| `g` | Open LazyGit |
| `r` | Refresh list (also refreshes PR/MR/CI for current worktree on GitHub/GitLab) |
| `R` | Fetch all remotes |
| `S` | Synchronise with upstream (pull + push, requires clean worktree) |
| `P` | Push to upstream (prompts to set upstream if missing) |
| `f` | Filter focused pane (worktrees, files, commits) |
| `/` | Search focused pane (incremental) |
| `alt+n`, `alt+p` | Move selection and fill filter input |
| `↑`, `↓` | Move selection (filter active, no fill) |
| `s` | Cycle sort mode (Path / Last Active / Last Switched) |
| `Home` | Go to first item in focused pane |
| `End` | Go to last item in focused pane |
| `?` | Show help |
| `q` | Quit |
| `1` | Focus Worktree pane (toggle zoom if focused) |
| `2` | Focus Status pane (toggle zoom if focused) |
| `3` | Focus Log pane (toggle zoom if focused) |
| `h`, `l` | Navigate left/right (h=worktree pane, l=cycle right panes) |
| `Tab`, `]` | Cycle to next pane |
| `[` | Cycle to previous pane |
| `=` | Toggle zoom for focused pane (full screen) |

**Log Pane** (when focused on commit log):

| Key | Action |
| --- | --- |
| `Enter` | Open commit file tree (browse files changed in commit) |
| `d` | Show full commit diff in pager |
| `C` | Cherry-pick commit to another worktree |
| `j/k` | Navigate commits |
| `ctrl+j` | Next commit and open file tree |
| `/` | Search commit titles (incremental) |

**Commit File Tree** (when viewing files in a commit):

| Key | Action |
| --- | --- |
| `j/k` | Navigate files and directories |
| `Enter` | Toggle directory collapse/expand, or show file diff |
| `d` | Show full commit diff in pager |
| `f` | Filter files by name |
| `/` | Search files (incremental) |
| `n/N` | Next/previous search match |
| `ctrl+d`, `Space` | Half page down |
| `ctrl+u` | Half page up |
| `g`, `G` | Jump to top/bottom |
| `q`, `Esc` | Return to commit log |

**Status Pane** (when focused on status):

Displays changed files in a collapsible tree view, grouped by directory (similar to lazygit).

| Key | Action |
| --- | --- |
| `j/k` | Navigate between files and directories |
| `Enter` | Toggle directory expand/collapse, or show diff for files |
| `e` | Open selected file in editor |
| `d` | Show full diff of all files in pager |
| `s` | Stage/unstage selected file or directory |
| `D` | Delete selected file or directory (with confirmation) |
| `c` | Commit staged changes |
| `C` | Stage all changes and commit |
| `g` | Open LazyGit |
| `ctrl+←`, `ctrl+→` | Jump to previous/next folder |
| `/` | Search file/directory names (incremental) |
| `ctrl+d`, `Space` | Half page down |
| `ctrl+u` | Half page up |
| `PageUp`, `PageDown` | Half page up/down |

**CI Status Pane** (when viewing CI checks):

| Key | Action |
|--- | --- |
| `Enter` | Open CI job in browser |
| `Ctrl+v` | View CI logs in pager |
| `Ctrl+r` | Restart CI job (GitHub Actions only) |

**Filter Mode:**

Applies to focused pane (worktrees, files, commits). Active filter shows `[Esc] Clear` hint.

Selection menus: press `f` to show the filter input; `Esc` returns to the list and keeps the current filter.

* `alt+n`, `alt+p`: Navigate and update filter input
* `↑`, `↓`, `ctrl+j`, `ctrl+k`: Navigate without changing input
* `Enter`: Exit filter mode (filter remains)
* `Esc`, `Ctrl+C`: Clear filter

**Search Mode:**

* Type to jump to the first matching item
* `n`, `N`: Next / previous match
* `Enter`: Close search
* `Esc`, `Ctrl+C`: Clear search

**Command History (`!`):**

Saved per repository (100 max). Use `↑`/`↓` to navigate.

**Command Palette Actions:**

* **Select theme**: Change theme with live preview (see [Themes](#themes)).
* **Create from current branch**: Copy current branch to a new worktree. Tick "Include current file changes" to carry over uncommitted changes. Uses `branch_name_script` if configured.

### Mouse Controls

* **Click**: Select and focus panes or items
* **Scroll**: Navigate lists in any pane

## Configuration

Default worktree location: `~/.local/share/worktrees/<organization>-<repo_name>`.

### Global Configuration (YAML)

Reads `~/.config/lazyworktree/config.yaml`. Example (also in [config.example.yaml](./config.example.yaml)):

```yaml
worktree_dir: ~/.local/share/worktrees
sort_mode: switched  # Options: "path", "active" (commit date), "switched" (last accessed)
auto_refresh: true
refresh_interval: 10  # Seconds
disable_pr: false     # Disable all PR/MR fetching and display (default: false)
icon_set: nerd-font-v3
search_auto_select: false
fuzzy_finder_input: false
palette_mru: true         # Enable MRU (Most Recently Used) sorting for command palette
palette_mru_limit: 5      # Number of recent commands to show (default: 5)
max_untracked_diffs: 10
max_diff_chars: 200000
max_name_length: 95       # Maximum length for worktree names in table display (0 disables truncation)
theme: ""       # Leave empty to auto-detect based on terminal background colour
                # (defaults to "rose-pine" for dark, "dracula-light" for light).
                # Options: see the Themes section below.
git_pager: delta
pager: "less --use-color --wordwrap -qcR -P 'Press q to exit..'"
editor: nvim
git_pager_args:
  - --syntax-theme
  - Dracula
trust_mode: "tofu" # Options: "tofu" (default), "never", "always"
merge_method: "rebase" # Options: "rebase" (default), "merge"
session_prefix: "wt-" # Prefix for tmux/zellij session names (default: "wt-")
# Branch name generation for issues and PRs
issue_branch_name_template: "issue-{number}-{title}" # Placeholders: {number}, {title}, {generated}
pr_branch_name_template: "pr-{number}-{title}" # Placeholders: {number}, {title}, {generated}
# Automatic branch name generation (see "Automatically Generated Branch Names")
branch_name_script: "" # Script to generate names from diff/issue/PR content
init_commands:
  - link_topsymlinks
terminate_commands:
  - echo "Cleaning up $WORKTREE_NAME"
custom_commands:
  t:
    command: make test
    description: Run tests
    show_help: true
    wait: true
# Custom worktree creation menu items
custom_create_menus:
  - label: "From JIRA ticket"
    description: "Create from JIRA issue"
    command: "jayrah browse 'SRVKP' --choose"
    interactive: true  # TUI-based commands need this to suspend lazyworktree
    post_command: "git commit --allow-empty -m 'Initial commit for ${WORKTREE_BRANCH}'"
    post_interactive: false  # Run post-command in background
  - label: "From clipboard"
    description: "Use clipboard as branch name"
    command: "pbpaste"
```

### Configuration Precedence

Highest to lowest priority:

1. **CLI overrides** (`--config` flag)
2. **Git local configuration** (`git config --local`)
3. **Git global configuration** (`git config --global`)
4. **YAML configuration file** (`~/.config/lazyworktree/config.yaml`)
5. **Built-in defaults**

### Git Configuration

Use the `lw.` prefix:

```bash
# Set globally
git config --global lw.theme nord
git config --global lw.worktree_dir ~/.local/share/worktrees

# Set per-repository
git config --local lw.theme dracula
git config --local lw.init_commands "link_topsymlinks"
git config --local lw.init_commands "npm install"  # Multi-values supported
```

To view configured values:

```bash
git config --global --get-regexp "^lw\."
git config --local --get-regexp "^lw\."
```

### Settings

**Themes**

* `theme`: colour theme (auto-detected: `dracula` dark, `dracula-light` light). See [Themes](#themes).
* `lazyworktree --show-syntax-themes`: show delta syntax-theme defaults.
* `lazyworktree --theme <name>`: select UI theme.

**Worktree list and refresh**

* `sort_mode`: `"switched"` (last accessed, default), `"active"` (commit date), or `"path"` (alphabetical).
* `auto_refresh`: background refresh of git metadata (default: true).
* `ci_auto_refresh`: periodically refresh CI status for GitHub repositories (default: false).
* `refresh_interval`: refresh frequency in seconds (default: 10).
* `icon_set`: choose icon set ("nerd-font-v3", "text").
* `max_untracked_diffs`, `max_diff_chars`: limits for diff display (0 disables).
* `max_name_length`: maximum display length for worktree names (default: 95, 0 disables truncation).

**Search and palette**

* `search_auto_select`: start with filter focused (or use `--search-auto-select`).
* `fuzzy_finder_input`: show fuzzy suggestions in input dialogues.
* `palette_mru`: enable MRU sorting in command palette (default: true). Control count with `palette_mru_limit` (default: 5).

**Diff, pager, and editor**

* `git_pager`: diff formatter (default: `delta`). Empty string disables formatting.
* `git_pager_args`: arguments for git_pager. Auto-selects syntax theme for delta.
* `git_pager_interactive`: set `true` for interactive viewers like `diffnav` or `tig`.
* `pager`: pager for output display (default: `$PAGER`, fallback to `less`).
* `ci_script_pager`: pager for CI logs with direct terminal control. Falls back to `pager`. Example to strip GitHub Actions timestamps:

```yaml
ci_script_pager: |
  sed -E '
  s/.*[0-9]{4}-[0-9]{2}-[0-9]{2}T([0-9]{2}:[0-9]{2}:[0-9]{2})\.[0-9]+Z[[:space:]]*/\1 /;
  t;
  s/.*UNKNOWN STEP[[:space:]]+//' | \
   tee /tmp/.ci.${LW_CI_JOB_NAME_CLEAN}-${LW_CI_STARTED_AT}.md |
  less --use-color -q --wordwrap -qcR -P 'Press q to exit..'
```

CI environment variables: `LW_CI_JOB_NAME`, `LW_CI_JOB_NAME_CLEAN`, `LW_CI_RUN_ID`, `LW_CI_STARTED_AT`.

* `editor`: editor for Status pane `e` key (default: `$EDITOR`, fallback to `nvim`).

**Worktree lifecycle**

* `init_commands`, `terminate_commands`: run before repository `.wt` commands.

**Sync and multiplexers**

* `merge_method`: `"rebase"` (default) or `"merge"`. Controls Absorb and Sync (`S`) behaviour.
* `session_prefix`: prefix for tmux/zellij sessions (default: `wt-`). Palette filters by this prefix.

**Branch naming**

* `branch_name_script`: script for automatic branch suggestions. See [Automatically generated branch names](#automatically-generated-branch-names).
* `issue_branch_name_template`, `pr_branch_name_template`: templates with placeholders `{number}`, `{title}`, `{generated}`.

**Custom create menu**

* `custom_create_menus`: add custom items to the creation menu (`c` key). Supports `interactive` and `post_command`.

## Themes

Built-in themes:

| Theme | Notes | Best For |
|-------|-------|----------|
| **dracula** | Dark (#282A36) | Dark terminals, vibrant colours, default fallback |
| **dracula-light** | White (#FFFFFF) | Light terminals, Dracula colours, default light theme |
| **narna** | Charcoal (#0D1117) | Dark terminals, blue highlights |
| **clean-light** | White (#FFFFFF) | Light terminals, cyan accent |
| **catppuccin-latte** | Soft white (#EFF1F5) | Catppuccin Latte light palette |
| **rose-pine-dawn** | Warm white (#FAF4ED) | Rosé Pine Dawn warm palette |
| **one-light** | Light grey (#FAFAFA) | Atom One Light |
| **everforest-light** | Beige (#F3EFDA) | Everforest nature light |
| **solarized-dark** | Deep teal (#002B36) | Classic Solarized dark palette |
| **solarized-light** | Cream (#FDF6E3) | Classic Solarized light palette |
| **gruvbox-dark** | Dark grey (#282828) | Gruvbox dark, warm accents |
| **gruvbox-light** | Sand (#FBF1C7) | Gruvbox light, earthy tones |
| **nord** | Midnight blue (#2E3440) | Nord calm cyan accents |
| **monokai** | Olive black (#272822) | Monokai bright neon accents |
| **catppuccin-mocha** | Mocha (#1E1E2E) | Catppuccin Mocha pastels |
| **modern** | Zinc (#18181B) | Sleek modern dark theme with violet accents |
| **tokyo-night** | Storm (#24283B) | Tokyo Night Storm with blue highlights |
| **one-dark** | Dark (#282C34) | Atom One Dark classic palette |
| **rose-pine** | Midnight (#191724) | Rosé Pine dark and moody |
| **ayu-mirage** | Mirage (#212733) | Ayu Mirage modern look |
| **everforest-dark** | Dark (#2D353B) | Everforest nature dark |
| **kanagawa** | Wave (#1F1F28) | Kanagawa Wave inspired by Japanese art |

Set in config: `theme: dracula`

### Custom Themes

Define custom themes that inherit from built-in themes or define new colour schemes.

**Inherit from built-in:**

```yaml
custom_themes:
  my-dark:
    base: dracula
    accent: "#FF6B9D"
    text_fg: "#E8E8E8"

  my-light:
    base: dracula-light
    accent: "#0066CC"
```

**Defining a complete theme (all 11 colour fields required):**

```yaml
custom_themes:
  completely-custom:
    accent: "#00FF00"
    accent_fg: "#000000"
    accent_dim: "#2A2A2A"
    border: "#3A3A3A"
    border_dim: "#2A2A2A"
    muted_fg: "#888888"
    text_fg: "#FFFFFF"
    success_fg: "#00FF00"
    warn_fg: "#FFFF00"
    error_fg: "#FF0000"
    cyan: "#00FFFF"
```

**Inherit from other custom themes:**

```yaml
custom_themes:
  base-custom:
    base: dracula
    accent: "#FF0000"
  derived:
    base: base-custom
    accent: "#00FF00"
```

**Colour fields:** `accent`, `accent_fg`, `accent_dim`, `border`, `border_dim`, `muted_fg`, `text_fg`, `success_fg`, `warn_fg`, `error_fg`, `cyan`.

Values must be hex (`#RRGGBB` or `#RGB`). With `base`, only override what you need. Without `base`, all 11 fields are required. Custom themes appear alongside built-in themes.

## CI Status Display

Shows CI check statuses for worktrees with associated PR/MR:

* `✓` Green - Passed | `✗` Red - Failed | `●` Yellow - Pending | `○` Grey - Skipped | `⊘` Grey - Cancelled

Status is fetched lazily and cached for 30 seconds. Press `p` to refresh.

## Custom Commands

Define keybindings in config. Commands run interactively (TUI suspends) and appear in the palette. Use `show_output` to pipe through pager.

Defaults: `t` = tmux, `Z` = zellij. Override via `custom_commands.t` or `custom_commands.Z`. Palette lists sessions matching `session_prefix` (default: `wt-`).

### Configuration Format

```yaml
custom_commands:
  e:
    command: nvim
    description: Editor
    show_help: true
  s:
    command: zsh
    description: Shell
    show_help: true
  T: # Run tests and wait for keypress
    command: make test
    description: Run tests
    show_help: false
    wait: true
  o: # Show output in the pager
    command: git status -sb
    description: Status
    show_help: true
    show_output: true
  c: # Open Claude CLI in a new terminal tab (Kitty, WezTerm, or iTerm)
    command: claude
    description: Claude Code
    new_tab: true
    show_help: true
  t: # Open a tmux session with multiple windows
    description: Tmux
    show_help: true
    tmux: # If you specify zellij instead of tmux this would manage zellij sessions
      session_name: "wt:$WORKTREE_NAME"
      attach: true
      on_exists: switch
      windows:
        - name: claude
          command: claude
        - name: shell
          command: zsh
        - name: lazygit
          command: lazygit
```

### Field Reference

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `command` | string | **required** | Command to execute |
| `description` | string | `""` | Shown in help and palette |
| `show_help` | bool | `false` | Show in help screen (`?`) and footer |
| `wait` | bool | `false` | Wait for keypress after completion |
| `show_output` | bool | `false` | Show stdout/stderr in pager (ignores `wait`) |
| `new_tab` | bool | `false` | Launch in new terminal tab (Kitty with remote control enabled, WezTerm, or iTerm) |
| `tmux` | object | `null` | Configure tmux session |
| `zellij` | object | `null` | Configure zellij session |

#### tmux fields

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `session_name` | string | `wt:$WORKTREE_NAME` | Session name (env vars supported, special chars replaced) |
| `attach` | bool | `true` | Attach immediately; if false, show modal with instructions |
| `on_exists` | string | `switch` | Behaviour if session exists: `switch`, `attach`, `kill`, `new` |
| `windows` | list | `[ { name: "shell" } ]` | Window definitions for the session |

If `windows` is empty, creates a single `shell` window.

#### tmux window fields

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `name` | string | `window-N` | Window name (supports env vars) |
| `command` | string | `""` | Command to run in the window (empty uses your default shell) |
| `cwd` | string | `$WORKTREE_PATH` | Working directory for the window (supports env vars) |

#### zellij fields

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `session_name` | string | `wt:$WORKTREE_NAME` | Session name (env vars supported, special chars replaced) |
| `attach` | bool | `true` | Attach immediately; if false, show modal with instructions |
| `on_exists` | string | `switch` | Behaviour if session exists: `switch`, `attach`, `kill`, `new` |
| `windows` | list | `[ { name: "shell" } ]` | Tab definitions for the session |

If `windows` is empty, creates a single `shell` tab. Session names with `/`, `\`, `:` are replaced with `-`.

#### zellij window fields

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `name` | string | `window-N` | Tab name (supports env vars) |
| `command` | string | `""` | Command to run in the tab (empty uses your default shell) |
| `cwd` | string | `$WORKTREE_PATH` | Working directory for the tab (supports env vars) |

### Environment Variables

Available: `WORKTREE_BRANCH`, `MAIN_WORKTREE_PATH`, `WORKTREE_PATH`, `WORKTREE_NAME`, `REPO_NAME`.

### Supported Key Formats

Single keys (`e`, `s`), modifiers (`ctrl+e`, `alt+t`), special keys (`enter`, `esc`, `tab`, `space`).

**Examples:**

```yaml
custom_commands:
  "ctrl+e":
    command: nvim
    description: Open editor with Ctrl+E
  "alt+t":
    command: make test
    description: Run tests with Alt+T
    wait: true
```

### Key Precedence

Custom commands override built-in keys.

## Custom Initialisation and Termination

Create a `.wt` file in your repository to run commands when creating/removing worktrees. Format inspired by [wt](https://github.com/taecontrol/wt).

### Example `.wt` configuration

```yaml
init_commands:
    - link_topsymlinks
    - cp $MAIN_WORKTREE_PATH/.env $WORKTREE_PATH/.env
    - npm install
    - code .

terminate_commands:
    - echo "Cleaning up $WORKTREE_NAME"
```

Environment variables: `WORKTREE_BRANCH`, `MAIN_WORKTREE_PATH`, `WORKTREE_PATH`, `WORKTREE_NAME`.

### Security: Trust on First Use (TOFU)

Since `.wt` files execute arbitrary commands, lazyworktree uses TOFU. On first encounter or modification, select **Trust**, **Block**, or **Cancel**. Hashes stored in `~/.local/share/lazyworktree/trusted.json`.

Configure `trust_mode`: `tofu` (default, prompt), `never` (skip all), `always` (no prompts).

### Special Commands

* `link_topsymlinks`: Built-in command that symlinks untracked/ignored root files, editor configs (`.vscode`, `.idea`, `.cursor`, `.claude/settings.local.json`), creates `tmp/`, and runs `direnv allow` if `.envrc` exists.

## Branch Naming Conventions

Special characters are converted to hyphens for Git compatibility. Leading/trailing hyphens are removed, consecutive hyphens collapsed. Length capped at 50 (manual) or 100 (auto) characters.

| Input | Converted |
|-------|-----------|
| `feature.new` | `feature-new` |
| `bug fix here` | `bug-fix-here` |
| `feature:test` | `feature-test` |

## Automatically Generated Branch Names

Configure `branch_name_script` to generate names via tools like [aichat](https://github.com/sigoden/aichat/) or [claude code](https://claude.com/product/claude-code). PRs/issues output to `{generated}` placeholder; diffs output complete names.

> [!NOTE]
> Smaller, faster models suffice for branch names.

### Configuration

```yaml
# For PRs/issues: generate a title (available via {generated} placeholder)
branch_name_script: "aichat -m gemini:gemini-2.5-flash-lite 'Generate a short title for this PR or issue. Output only the title (like feat-ai-session-manager), nothing else.'"

# Choose which template to use:
pr_branch_name_template: "pr-{number}-{generated}"  # Use generated title
# pr_branch_name_template: "pr-{number}-{title}"    # Use original PR title
# pr_branch_name_template: "pr-{number}-{generated}-{title}"  # Use both!

# For diffs: generate a complete branch name
# branch_name_script: "aichat -m gemini:gemini-2.5-flash-lite 'Generate a short git branch name (no spaces, use hyphens) for this diff. Output only the branch name, nothing else.'"
```

### Template Placeholders

* `{number}` - PR/issue number
* `{title}` - Original sanitised title
* `{generated}` - Generated title (falls back to `{title}`)
* `{pr_author}` - PR author username

**Examples:**

| Template | Result (PR #2 by @alice: "Add AI session management") | Generated: `feat-ai-session-manager` |
|----------|-------------------------------------------------------|----------------------------------------|
| `pr-{number}-{title}` | `pr-2-add-ai-session-management` | Not used |
| `pr-{number}-{generated}` | `pr-2-feat-ai-session-manager` | Used |
| `pr-{number}-{pr_author}-{title}` | `pr-2-alice-add-ai-session-management` | Not used |
| `pr-{number}-{pr_author}-{generated}` | `pr-2-alice-feat-ai-session-manager` | Used |

If script fails, `{generated}` falls back to `{title}`.

### Script Requirements

Receives content on stdin, outputs branch name on stdout (first line). Timeout: 30s.

### Environment Variables

`LAZYWORKTREE_TYPE` (pr/issue/diff), `LAZYWORKTREE_NUMBER`, `LAZYWORKTREE_TEMPLATE`, `LAZYWORKTREE_SUGGESTED_NAME`.

**Example:**

```yaml
# Different prompts for different types
branch_name_script: |
  if [ "$LAZYWORKTREE_TYPE" = "diff" ]; then
    aichat -m gemini:gemini-2.5-flash-lite 'Generate a complete branch name for this diff'
  else
    aichat -m gemini:gemini-2.5-flash-lite 'Generate a short title (no pr- prefix) for this PR/issue'
  fi

# Use PR/issue number in the prompt
branch_name_script: |
  aichat -m gemini:gemini-2.5-flash-lite "Generate a title for PR #$LAZYWORKTREE_NUMBER. Output only the title."
```

## CLI Usage

### Config overrides

```bash
lazyworktree --worktree-dir ~/worktrees

# Override config values via command line
lazyworktree --config lw.theme=nord --config lw.sort_mode=active
```

Create, delete, and list worktrees from the command line. Legacy aliases `wt-create` and `wt-delete` still work.

### Listing Worktrees

```bash
lazyworktree list              # Table output (default)
lazyworktree list --pristine   # Paths only (scripting)
lazyworktree list --json       # JSON output
lazyworktree ls                # Alias
```

Note: `--pristine` and `--json` are mutually exclusive.

### Creating Worktrees

```bash
lazyworktree create                          # Auto-generated from current branch
lazyworktree create my-feature               # Explicit name
lazyworktree create my-feature --with-change # With uncommitted changes
lazyworktree create --from-branch main my-feature
lazyworktree create --from-pr 123
lazyworktree create --from-issue 42          # From issue (base: current branch)
lazyworktree create --from-issue 42 --from-branch main  # From issue with explicit base
lazyworktree create -I                       # Interactively select issue (fzf or list)
lazyworktree create -I --from-branch main    # Interactive issue with explicit base
lazyworktree create -P                       # Interactively select PR (fzf or list)
lazyworktree create --from-pr 123 --no-workspace        # Branch only, no worktree
lazyworktree create --from-issue 42 --no-workspace      # Branch only, no worktree
lazyworktree create -I --no-workspace                    # Interactively select issue, branch only
lazyworktree create -P --no-workspace        # Interactively select PR, branch only
```

For complete CLI documentation, see `man lazyworktree` or `lazyworktree --help`.

### Deleting Worktrees

```bash
lazyworktree delete                # Delete worktree and branch
lazyworktree delete --no-branch    # Delete worktree only
```

## Screenshots

### Light Theme (dracula-light theme)

<img width="3730" height="2484" alt="image" src="https://github.com/user-attachments/assets/ab19a23a-852c-46c3-a6f4-a27d8519f89a" />

### Command Palette (nord)

<img width="3730" height="2484" alt="image" src="https://github.com/user-attachments/assets/8a722eea-5d00-47b2-8e59-6019cfd6336f" />

### Github Actions Logs viewer (rose-pine)

<img width="3730" height="2484" alt="image" src="./.github/screenshots/ci-runs.png" />

### Branch creation (tokyo-night)

<img width="3730" height="2484" alt="image" src="https://github.com/user-attachments/assets/92888a4f-c3aa-4b39-b78b-d3c62897b69a" />

### Files in commit view (kanagawa)

<img width="3730" height="2484" alt="image" src="https://github.com/user-attachments/assets/735458b2-a3a9-451c-ac51-b43452d5e421" />

### Create a branch from a Issue (clean-light)

<https://github.com/user-attachments/assets/a733b95f-cd11-48a9-be58-810866aff1a2>

## How does it compare?

See [COMPARISON.md](./COMPARISON.md) for a detailed comparison with other worktree managers.

## Trivia

Originally a Python [Textual](https://textual.textualize.io/) app, migrated to Go ([BubbleTea](https://github.com/charmbracelet/bubbletea)) for faster startup. Python version: <https://github.com/chmouel/lazyworktree/tree/python>

## Copyright

[Apache-2.0](./LICENSE)

## Authors

### Chmouel Boudjnah

* 🐘 Fediverse - <[@chmouel@chmouel.com](https://fosstodon.org/@chmouel)>
* 🐦 Twitter - <[@chmouel](https://twitter.com/chmouel)>
* 📝 Blog  - <[https://blog.chmouel.com](https://blog.chmouel.com)>
