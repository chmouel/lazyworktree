# LazyWorktree - Easy Git worktree management for the terminal

<img width="1865" height="1242" alt="Screenshot 2026-01-20 at 22 05 56" src="https://github.com/user-attachments/assets/3ac33b90-ea1a-4c21-84ac-d875bf3a444a" />

LazyWorktree is a terminal user interface for managing Git worktrees. It
provides a structured, keyboard-driven workflow for creating, inspecting, and
navigating multiple worktrees within a single repository.

Built with [BubbleTea](https://github.com/charmbracelet/bubbletea), it focuses on fast iteration, clear state visibility, and tight integration with common Git tooling.

![Go](https://img.shields.io/badge/go-1.25%2B-blue)
![Coverage](https://img.shields.io/badge/Coverage-68.3%25-yellow)

## Screenshot

<img width="3797" height="2110" alt="image" src="https://github.com/user-attachments/assets/cf5ba9c2-1f38-4865-8503-49ad0d8d5462" />

_See other [Screenshots below](#screenshots)_

## Features

* **Worktree lifecycle**: Create, rename, remove, absorb, and prune merged worktrees.
* **Worktree state**: Show dirty files, ahead/behind counts, and divergence from main.
* **From current branch**: Create from the current branch, optionally carrying over uncommitted changes.
* **From issue**: Create from a GitHub/GitLab issue with automatic branch naming.
* **From PR or MR**: Create from an open GitHub/GitLab pull or merge request.
* **Forge integration**: Show linked PR/MR, CI status, and checks via `gh` or `glab`.
* **Cherry-picking**: Apply commits from one worktree to another.
* **Commit inspection**: Browse commit logs with author initials and per-commit file trees.
* **Status management**: Stage, unstage, commit, edit, and diff files interactively.
* **Diff viewing**: View diffs in a pager, with optional delta integration.
* **Terminal multiplexers**: Manage per-worktree tmux or zellij sessions.
* **Shell integration**: Jump into selected worktrees and return to the last-used one.
* **Command palette**: Access actions, commands, and sessions with MRU-based navigation.
* **Custom commands**: Define keybindings, tmux/zellij layouts, and per-repo command workflows.
* **Automation and hooks**: Run init/terminate commands via `.wt` files with TOFU security.
* **Automatic branch naming**: Generate branch names from diffs, issues, or PRs via scripts.
* **LazyGit integration**: Launch lazygit for the selected worktree.

## Getting Started

1. Install lazyworktree using your preferred method below.
2. Run `lazyworktree` inside a Git repository.
3. Press `?` for help and key hints.

Common overrides:

```bash
lazyworktree --worktree-dir ~/worktrees

# Override config values via command line
lazyworktree --config lw.theme=nord --config lw.auto_fetch_prs=true
```

## Requirements

* **Git**: 2.31+ (recommended)
* **Forge CLI**: GitHub CLI (`gh`) or GitLab CLI (`glab`) for repo resolution and PR/MR status

**Optional:**

* **delta**: For syntax-highlighted diffs (highly recommended)
* **lazygit**: For full TUI git control
* **tmux**: For tmux integration support
* **zellij**: For zellij integration support

**Build-time only:**

* Go 1.25 or newer

## Installation

### Pre-built Binaries

Pre-built binaries for various platforms are provided in the [Releases](https://github.com/chmouel/lazyworktree/releases) section.

### 🍺 Homebrew (macOS)

```shell
brew tap chmouel/lazyworktree https://github.com/chmouel/lazyworktree
brew install lazyworktree --cask
```

### [Arch](https://aur.archlinux.org/packages/lazyworktree-bin)

```shell
yay -S lazyworktree-bin
```

### From Source

Clone the repository and build:

```bash
git clone https://github.com/chmouel/lazyworktree.git
cd lazyworktree
go build -o lazyworktree ./cmd/lazyworktree
```

Install to your PATH:

```bash
go install ./cmd/lazyworktree
```

Or build and run directly:

```bash
go run ./cmd/lazyworktree/main.go
```

## Shell Integration

LazyWorktree provides shell helpers to change the current directory to the
selected worktree on exit. These helpers are optional but recommended for
interactive use.

Zsh helpers are provided under shell/functions.shell and can be sourced
directly or downloaded.

See [./shell/README.md](./shell/README.md) for more detailed instructions.

## CLI Usage

LazyWorktree supports command-line operations for creating and deleting worktrees without launching the TUI:

### Creating Worktrees

**Create from current branch:**

```bash
# Auto-generated name from current branch
lazyworktree wt-create

# Explicit name
lazyworktree wt-create my-feature

# With uncommitted changes
lazyworktree wt-create --with-change

# Explicit name + changes
lazyworktree wt-create my-feature --with-change
```

**Create from a specific branch:**

```bash
# Explicit name
lazyworktree wt-create --from-branch main my-feature [--with-change] [--silent]

# Auto-generated name (sanitised from source branch)
lazyworktree wt-create --from-branch feature/new-feature [--with-change] [--silent]
```

The worktree/branch name can be specified explicitly or auto-generated:
- **Current branch + explicit name:** `lw wt-create my-feature`
- **Specific branch + explicit name:** `lw wt-create --from-branch main my-feature`
- **Current branch + auto-generated:** `lw wt-create` uses current branch name
- **Specific branch + auto-generated:** `lw wt-create --from-branch feature/cool-thing` creates "feature-cool-thing"
- Names are automatically sanitised to lowercase alphanumeric characters with hyphens

**Create from a PR:**

```bash
lazyworktree wt-create --from-pr 123 [--silent]
```

### Deleting Worktrees

```bash
lazyworktree wt-delete [--no-branch] [--silent]
```

Deletes the worktree and associated branch (only if worktree name matches branch name). Use `--no-branch` to skip branch deletion.

## Key Bindings

| Key | Action |
| --- | --- |
| `Enter` | Jump to worktree (exit and cd) |
| `c` | Create new worktree (from branch, commit, PR/MR, or issue) |
| `m` | Rename selected worktree |
| `D` | Delete selected worktree |
| `d` | View diff in pager (respects pager config) |
| `A` | Absorb worktree into main |
| `X` | Prune merged worktrees (refreshes PR data, checks merge status) |
| `!` | Run arbitrary command in selected worktree (with command history) |
| `p` | Fetch PR/MR status (also refreshes CI checks) |
| `o` | Open PR/MR in browser |
| `ctrl+p`, `:` | Command palette |
| `g` | Open LazyGit |
| `r` | Refresh list |
| `R` | Fetch all remotes |
| `S` | Sync with upstream (pull + push, requires clean worktree) |
| `P` | Push to upstream (prompts to set upstream if missing) |
| `f` | Filter focused pane (worktrees, files, commits) |
| `/` | Search focused pane (incremental) |
| `alt+n`, `alt+p` | Move selection and fill filter input |
| `↑`, `↓` | Move selection (filter active, no fill) |
| `s` | Cycle sort mode (Path / Last Active / Last Switched) |
| `Home` | Go to first item in focused pane |
| `End` | Go to last item in focused pane |
| `?` | Show help |
| `1` | Focus Worktree pane (toggle zoom if focused) |
| `2` | Focus Status pane (toggle zoom if focused) |
| `3` | Focus Log pane (toggle zoom if focused) |
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
| `q`, `Esc` | Return to commit log |

**Status Pane** (when focused on status):

The status pane displays changed files in a collapsible tree view, grouped by
directory (similar to lazygit). Directories can be expanded/collapsed, files
are sorted alphabetically within each directory level.

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

**Filter Mode:**

Filter mode applies to the focused pane (worktrees, file names, commit titles).

* `alt+n`, `alt+p`: Navigate and update filter input with selected item
* `↑`, `↓`, `ctrl+j`, `ctrl+k`: Navigate list without changing filter input
* `Enter`: Exit filter mode (filter remains active)
* `Esc`, `Ctrl+C`: Exit filter mode

When a filter is active, the pane title shows a filter indicator with `[Esc] Clear` hint. Press `Esc` to clear the filter.

**Search Mode:**

* Type to jump to the first matching item
* `n`, `N`: Next / previous match
* `Enter`: Close search
* `Esc`, `Ctrl+C`: Clear search

**Command History (! command):**

Commands run via `!` are saved per repository (100 entries max). Use `↑`/`↓` to navigate history.

**Command Palette Actions:**

* **Select theme**: Change the application theme with live preview (see [Themes](#themes)).
* **Create from current branch**: Copy your current branch to a new worktree. If uncommitted changes exist, tick "Include current file changes" to stash and reapply them in the new worktree. Any configured `branch_name_script` receives the diff for automatic naming.

### Mouse Controls

* **Click**: Select and focus panes or items
* **Scroll Wheel**: Scroll through lists and content
  * Worktree table (left pane)
  * Status pane (right top pane)
  * Log table (right bottom pane)

## Configuration

Worktrees are expected to be organised under `~/.local/share/worktrees/<organization>-<repo_name>` by default unless overridden via configuration.

### Global Configuration (YAML)

lazyworktree reads `~/.config/lazyworktree/config.yaml` (or `.yml`) for default settings. An example configuration is provided below (also available in [config.example.yaml](./config.example.yaml)):

```yaml
worktree_dir: ~/.local/share/worktrees
sort_mode: switched  # Options: "path", "active" (commit date), "switched" (last accessed)
auto_fetch_prs: false
auto_refresh: true
refresh_interval: 10  # Seconds
show_icons: true
search_auto_select: false
fuzzy_finder_input: false
palette_mru: true         # Enable MRU (Most Recently Used) sorting for command palette
palette_mru_limit: 5      # Number of recent commands to show (default: 5)
max_untracked_diffs: 10
max_diff_chars: 200000
max_name_length: 95       # Maximum length for worktree names in table display (0 disables truncation)
theme: ""       # Leave empty to auto-detect based on terminal background colour
                # (defaults to "dracula" for dark, "dracula-light" for light).
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

lazyworktree reads configuration from multiple sources with the following precedence (highest to lowest):

1. **CLI overrides** (using `--config` flag): Highest priority
2. **Git local configuration** (repository-specific via `git config --local`)
3. **Git global configuration** (user-wide via `git config --global`)
4. **YAML configuration file** (`~/.config/lazyworktree/config.yaml`)
5. **Built-in defaults**: Lowest priority

This allows flexible configuration at different levels. For example, you can set a default theme globally in your Git config, override it for a specific repository, and temporarily change it via the command line.

### Git Configuration

Settings can be stored in Git's configuration system with the `lw.` prefix. Examples:

```bash
# Set globally
git config --global lw.theme nord
git config --global lw.auto_fetch_prs true
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

* `theme` selects the colour theme. See [Themes](#themes). Default: auto-detected (`dracula` for dark, `dracula-light` for light).
* Execute `lazyworktree --show-syntax-themes` to display the default delta `--syntax-theme` values for each UI theme.
* Use `lazyworktree --theme <name>` to select a UI theme directly.

**Worktree list and refresh**

* `sort_mode`: `"switched"` (last accessed, default), `"active"` (commit date), or `"path"` (alphabetical).
* `auto_fetch_prs`: fetch PR data on startup.
* `auto_refresh`: background refresh of git metadata (default: true).
* `refresh_interval`: refresh frequency in seconds (default: 10).
* `show_icons`: display icons (default: true).
* `max_untracked_diffs`, `max_diff_chars`: limits for diff display (0 disables).
* `max_name_length`: maximum display length for worktree names (default: 95, 0 disables truncation).

**Search and palette**

* `search_auto_select`: start with filter focused (or use `--search-auto-select`).
* `fuzzy_finder_input`: show fuzzy suggestions in input dialogs.
* `palette_mru`: enable MRU sorting in command palette (default: true). Control count with `palette_mru_limit` (default: 5).

**Diff, pager, and editor**

* `git_pager`: diff formatter (default: `delta`). Empty string disables formatting.
* `git_pager_args`: arguments for git_pager. Auto-selects syntax theme for delta.
* `git_pager_interactive`: set `true` for interactive viewers like `diffnav` or `tig`.
* `pager`: pager for output display (default: `$PAGER`, fallback to `less`).
* `editor`: editor for Status pane `e` key (default: `$EDITOR`, fallback to `nvim`).

**Worktree lifecycle**

* `init_commands` and `terminate_commands` execute prior to any repository-specific `.wt` commands (if present).

**Sync and multiplexers**

* `merge_method`: `"rebase"` (default) or `"merge"`. Controls Absorb and Sync (`S`) behaviour.
* `session_prefix`: prefix for tmux/zellij sessions (default: `wt-`). Palette filters by this prefix.

**Branch naming**

* `branch_name_script`: script for automatic branch suggestions. See [Automatically generated branch names](#automatically-generated-branch-names).
* `issue_branch_name_template`, `pr_branch_name_template`: templates with placeholders `{number}`, `{title}`, `{generated}`.

**Custom create menu**

* `custom_create_menus`: add custom items to the creation menu (`c` key). Supports `interactive` and `post_command`.

## Themes

lazyworktree includes built-in themes:

| Theme | Background | Best For |
|-------|-----------|----------|
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

To select a theme, configure it in your configuration file:

```yaml
theme: dracula  # or any listed above
```

### Custom Themes

You can define custom themes in your configuration file that inherit from built-in themes or define completely new colour schemes.

**Inheriting from a built-in theme:**

```yaml
custom_themes:
  my-dark:
    base: dracula
    accent: "#FF6B9D"
    text_fg: "#E8E8E8"

  my-light:
    base: dracula-light
    background: "#FAFAFA"
    accent: "#0066CC"
```

**Defining a complete theme (all 14 colour fields required):**

```yaml
custom_themes:
  completely-custom:
    background: "#1A1A1A"
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
    pink: "#FF00FF"
    yellow: "#FFFF00"
```

**Custom themes can inherit from other custom themes:**

```yaml
custom_themes:
  base-custom:
    base: dracula
    accent: "#FF0000"
  derived:
    base: base-custom
    accent: "#00FF00"
```

**Available colour fields:**
- `background` - Main background colour
- `accent` - Primary accent colour (highlights, selected items)
- `accent_fg` - Foreground colour for text on accent background
- `accent_dim` - Dimmed accent colour (selected rows/panels)
- `border` - Border colour
- `border_dim` - Dimmed border colour
- `muted_fg` - Muted text colour
- `text_fg` - Primary text colour
- `success_fg` - Success indicator colour
- `warn_fg` - Warning indicator colour
- `error_fg` - Error indicator colour
- `cyan` - Cyan accent colour
- `pink` - Pink accent colour
- `yellow` - Yellow accent colour

Colour values must be in hex format (`#RRGGBB` or `#RGB`). When using a `base` theme, only specify colours you want to override. When not using a base, all 14 colour fields are required.

Custom themes appear in the theme selection screen alongside built-in themes.

## CI Status Display

When viewing a worktree with an associated PR/MR, lazyworktree automatically retrieves and displays CI check statuses in the information pane.

* `✓` **Green** - Passed
* `✗` **Red** - Failed
* `●` **Yellow** - Pending/Running
* `○` **Grey** - Skipped
* `⊘` **Grey** - Cancelled

CI status is retrieved lazily (only for the selected worktree) and cached for 30 seconds to maintain UI responsiveness. Press `p` to force a refresh of CI status.

## Custom Commands

Define custom keybindings in `~/.config/lazyworktree/config.yaml`. Commands run interactively (TUI suspends) and appear in the command palette. Use `show_output` to pipe output through the pager.

Defaults: `t` opens tmux, `Z` opens zellij. Override by defining `custom_commands.t` or `custom_commands.Z`. The palette lists active sessions matching `session_prefix` (default: `wt-`).

### Configuration Format

Add a `custom_commands` section to your config:

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
  a: # Open CLaude CLI in the selected workspace in a new kitty tab
    command: "kitten @ launch --type tab --cwd $WORKTREE_PATH -- claude"
    description: Open Claude
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
| `command` | string | **required** | The command to execute |
| `description` | string | `""` | Description shown in the help screen and command palette |
| `show_help` | bool | `false` | Whether to show this command in the help screen (`?`) and footer hints |
| `wait` | bool | `false` | Wait for key press after command completes (useful for quick commands like `ls` or `make test`) |
| `show_output` | bool | `false` | Run non-interactively and show stdout/stderr in the pager (ignores `wait`) |
| `tmux` | object | `null` | Configure a tmux session instead of executing a single command |
| `zellij` | object | `null` | Configure a zellij session instead of executing a single command |

#### tmux fields

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `session_name` | string | `wt:$WORKTREE_NAME` | Session name (env vars supported, special chars replaced) |
| `attach` | bool | `true` | Attach immediately; if false, show modal with instructions |
| `on_exists` | string | `switch` | Behaviour if session exists: `switch`, `attach`, `kill`, `new` |
| `windows` | list | `[ { name: "shell" } ]` | Window definitions for the session |

If `windows` is omitted or empty, lazyworktree creates a single `shell` window.

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

If `windows` is omitted or empty, lazyworktree creates a single `shell` tab.
Zellij session names cannot include `/`, `\`, or `:`, so lazyworktree replaces them with `-`.

#### zellij window fields

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `name` | string | `window-N` | Tab name (supports env vars) |
| `command` | string | `""` | Command to run in the tab (empty uses your default shell) |
| `cwd` | string | `$WORKTREE_PATH` | Working directory for the tab (supports env vars) |

### Environment Variables

Custom commands have access to the same environment variables as init/terminate commands:

* `WORKTREE_BRANCH`: Name of the git branch
* `MAIN_WORKTREE_PATH`: Path to the main repository
* `WORKTREE_PATH`: Path to the selected worktree
* `WORKTREE_NAME`: Name of the worktree (directory name)
* `REPO_NAME`: Name of the repository (from GitHub/GitLab)

### Supported Key Formats

Custom commands support the same key formats as built-in keybindings:

* **Single keys**: `e`, `s`, `t`, `l`, etc.
* **Modifier combinations**: `ctrl+e`, `ctrl+t`, `alt+s`, etc.
* **Special keys**: `enter`, `esc`, `tab`, `space`, etc.

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

**Custom commands take precedence over built-in keys.** If you define a custom command with key `s`, it overrides the built-in sort toggle.

## Custom Initialisation and Termination

Create a `.wt` file in your main repository to define commands that run when creating or removing a worktree. Format inspired by [wt](https://github.com/taecontrol/wt).

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

The following environment variables are available to your commands:

* `WORKTREE_BRANCH`: Name of the git branch.
* `MAIN_WORKTREE_PATH`: Path to the main repository.
* `WORKTREE_PATH`: Path to the new worktree being created or removed.
* `WORKTREE_NAME`: Name of the worktree (directory name).

### Security: Trust on First Use (TOFU)

Since `.wt` files can execute arbitrary commands, lazyworktree uses a **Trust on First Use** security model.

* **First Run**: When encountering a new or modified `.wt` file, lazyworktree pauses and displays the commands. Select **Trust** (run and save), **Block** (skip), or **Cancel**.
* **Trusted**: Once trusted, commands run silently in the background until the `.wt` file changes again.
* **Persistence**: Trusted file hashes are stored in `~/.local/share/lazyworktree/trusted.json`.

Configure via `trust_mode` in `config.yaml`:

* **`tofu`** (Default): Prompts for confirmation on new or changed files. Secure and usable.
* **`never`**: Never runs commands from `.wt` files. Safest for untrusted environments.
* **`always`**: Always runs commands without prompting. Useful for personal/internal environments but risky.

### Special Commands

* `link_topsymlinks`: A built-in automation command (not a shell command) that executes without TOFU prompts once the `.wt` file is trusted. It performs the following:
  * Symlinks all untracked and ignored files from the root of the main worktree to the new worktree (excluding subdirectories).
  * Symlinks common editor configurations (`.vscode`, `.idea`, `.cursor`, `.claude`).
  * Ensures a `tmp/` directory exists in the new worktree.
  * Automatically runs `direnv allow` if a `.envrc` file is present.

## Branch Naming Conventions

Special characters are converted to hyphens for Git compatibility. Leading/trailing hyphens are removed, consecutive hyphens collapsed. Length capped at 50 (manual) or 100 (auto) characters.

| Input | Converted |
|-------|-----------|
| `feature.new` | `feature-new` |
| `bug fix here` | `bug-fix-here` |
| `feature:test` | `feature-test` |

## Automatically Generated Branch Names

Configure `branch_name_script` to generate branch names via a helper tool, for example [aichat](https://github.com/sigoden/aichat/) or [claude code](https://claude.com/product/claude-code).

* **PRs/issues:** Script outputs a title available via `{generated}` placeholder.
* **Diffs:** Script outputs a complete branch name.

> [!NOTE]
> Smaller, faster models are usually sufficient for short branch names. Choose a tool and model that fit your workflow.

### Configuration

Add `branch_name_script` to your `~/.config/lazyworktree/config.yaml`:

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

When creating worktrees from PRs or issues, the following placeholders are available:

* `{number}` - The PR/issue number
* `{title}` - The original sanitised PR/issue title (always available)
* `{generated}` - The generated title (falls back to `{title}` if the script is not configured or returns empty output)
* `{pr_author}` - The PR author's username (PRs only, sanitised)

**Examples:**

| Template | Result (PR #2 by @alice: "Add AI session management") | Generated: `feat-ai-session-manager` |
|----------|-------------------------------------------------------|----------------------------------------|
| `pr-{number}-{title}` | `pr-2-add-ai-session-management` | Not used |
| `pr-{number}-{generated}` | `pr-2-feat-ai-session-manager` | Used |
| `pr-{number}-{pr_author}-{title}` | `pr-2-alice-add-ai-session-management` | Not used |
| `pr-{number}-{pr_author}-{generated}` | `pr-2-alice-feat-ai-session-manager` | Used |

If the script fails or returns empty output, `{generated}` automatically falls back to the sanitised original title.

### Script Requirements

The script receives content on stdin (diff, issue, or PR title+body) and outputs the branch name on stdout (first line used). Timeout: 30 seconds. Falls back to the original title if the script fails.

### Environment Variables

Available to scripts: `LAZYWORKTREE_TYPE` (pr/issue/diff), `LAZYWORKTREE_NUMBER`, `LAZYWORKTREE_TEMPLATE`, `LAZYWORKTREE_SUGGESTED_NAME`.

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

## Screenshots

### Light Theme (dracula-light theme)

<img width="1754" height="1134" alt="image" src="https://github.com/user-attachments/assets/d3559158-18f3-4a46-b4d9-2a762b6adae1" />

### Command Palette (dracula theme)

<img width="1754" height="1077" alt="image" src="https://github.com/user-attachments/assets/c765db31-0419-40f6-99c4-328a686447b1" />

### Branch creation (dracula theme)

<img width="1760" height="1072" alt="image" src="https://github.com/user-attachments/assets/f705c330-d1d7-4d09-9f56-85de7d37543a" />

### Files in commit view (dracula theme)

<img width="3835" height="2116" alt="files-in-git-commit-view" src="https://github.com/user-attachments/assets/031bd043-c1a2-4f88-837e-89e26812cea7" />

### Create a branch from a Issue (clean-light theme)

<https://github.com/user-attachments/assets/a733b95f-cd11-48a9-be58-810866aff1a2>

## How does it compare?

lazyworktree covers a broader set of use cases than most Git worktree tools,
especially for interactive and human-driven workflows.

For a fair and detailed comparison with other popular worktree managers
(including their respective strengths and trade-offs), see
the [COMPARAISON](./COMPARAISON.md) document.

## Trivia

Previously, this was a Python textual application; however, the startup time proved excessive, prompting a migration to a Go-based [charmbracelet bubble](https://github.com/charmbracelet/bubbles) Terminal User Interface. The original Python implementation remains available for review or testing at <https://github.com/chmouel/lazyworktree/tree/python>

## Copyright

[Apache-2.0](./LICENSE)

## Authors

### Chmouel Boudjnah

* 🐘 Fediverse - <[@chmouel@chmouel.com](https://fosstodon.org/@chmouel)>
* 🐦 Twitter - <[@chmouel](https://twitter.com/chmouel)>
* 📝 Blog  - <[https://blog.chmouel.com](https://blog.chmouel.com)>
