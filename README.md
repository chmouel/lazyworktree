# LazyWorktree - Effortless Git worktree management for the terminal

<img align="right" width="180" height="180" alt="lw-logo" src="https://github.com/user-attachments/assets/77b63679-40b8-494c-a62d-19ccc39ac38e" />

A [BubbleTea](https://github.com/charmbracelet/bubbletea)-based Terminal User Interface designed for efficient Git worktree management. Visualise the repository's status, oversee branches, and navigate between worktrees with ease.

![Go](https://img.shields.io/badge/go-1.25%2B-blue)
![Coverage](https://img.shields.io/badge/Coverage-63.7%25-yellow)

## Features

- **Worktree Management**: Create, rename, delete, absorb, and prune merged worktrees.
- **Cherry-pick Commits**: Copy commits from one worktree to another via an interactive worktree picker.
- **Commit Log Details**: Log pane shows author initials alongside commit subjects.
- **Base Selection**: Select a base branch or commit from a list, or enter a reference when creating a worktree.
- **Forge Integration**: Fetch and display associated Pull Request (GitHub) or Merge Request (GitLab) status, including CI check results (via `gh` or `glab` CLI).
- **Create from PR/MR**: Create worktrees directly from open pull or merge requests, GitHub (or GitHub enterprise) or GitLab supported.
- **Create from current branch**: Start a worktree from the branch you are standing on, and use the prompt’s checkbox to carry over any in-progress changes.
- **Create from Issue**: Create worktrees from GitHub/GitLab issues with automatic branch name generation based on issue title.
- **Status at a Glance**: View dirty state, ahead/behind counts, and divergence from main.
- **[Tmux](https://github.com/tmux/tmux/) Integration**: Create and manage tmux sessions per worktree with multi-window support.
- **[Zellij](https://zellij.dev/)**: Create and manage zellij sessions per worktree with multi-tab support.
- **Diff Viewer**: View diff with optional [delta](https://github.com/dandavison/delta) support.
- **Repo Automation**: `.wt` init/terminate commands with [TOFU](https://en.wikipedia.org/wiki/Trust_on_first_use) security.
- **LazyGit Integration**: Launch [lazygit](https://github.com/jesseduffield/lazygit) directly for the currently selected worktree.

## Screenshot

<img width="3797" height="2110" alt="image" src="https://github.com/user-attachments/assets/cf5ba9c2-1f38-4865-8503-49ad0d8d5462" />

_See other [Screenshots below](#screenshots)_

## Prerequisites

- **Go**: 1.25+ (for building from source)
- **Git**: 2.31+ (recommended)
- **Forge CLI**: GitHub CLI (`gh`) or GitLab CLI (`glab`) for repo resolution and PR/MR status.

**Optional:**

- **delta**: For syntax-highlighted diffs. (highly recommended)
- **lazygit**: For full TUI git control.
- **tmux**: For TMUX integration support.
- **zellij**: For zellij integration support.

## Installation

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

To override the default worktree root, use the following:

```bash
lazyworktree --worktree-dir ~/worktrees
```

### Pre-built Binaries

Pre-built binaries for various platforms are provided in the [Releases](https://github.com/chmouel/lazyworktree/releases) section.

### 🍺 Homebrew

```shell
brew tap chmouel/lazyworktree https://github.com/chmouel/lazyworktree
brew install lazyworktree --cask
```

For shell integration with the "jump" functionality, download and source the [helper shell functions](./shell/functions.shell):

```bash
# Download the helper functions
mkdir -p ~/.shell/functions
curl -sL https://raw.githubusercontent.com/chmouel/lazyworktree/refs/heads/main/shell/functions.shell -o ~/.shell/functions/lazyworktree.shell

# Review and customize the functions if needed
# nano ~/.shell/functions/lazyworktree.shell

# Add to .zshrc
source ~/.shell/functions/lazyworktree.shell

# Create an alias for a specific repository
jt() { worktree_jump ~/path/to/your/main/repo "$@"; }
```

## [Arch](https://aur.archlinux.org/packages/lazyworktree-bin)

```shell
yay -S lazyworktree-bin
```

## Shell Integration (Zsh)

To enable the "jump" functionality, which changes your shell's current directory upon exit, append the helper functions from `shell/functions.shell` to your `.zshrc`. The helper uses `--output-selection` to write the selected path to a temporary file.

Example configuration:

```bash
# Add to .zshrc
source /path/to/lazyworktree/shell/functions.shell

# Create an alias for a specific repository
# worktree storage key is derived from the origin remote (e.g. github.com:owner/repo)
# and falls back to the directory basename when no remote is set.
jt() { worktree_jump ~/path/to/your/main/repo "$@"; }
```

You can now run `jt` to open the Terminal User Interface, select a worktree, and upon pressing `Enter`, your shell will change directory to that location.

To jump directly to a worktree by name with shell completion enabled, use the following:

```bash
jt() { worktree_jump ~/path/to/your/main/repo "$@"; }
_jt() { _worktree_jump ~/path/to/your/main/repo; }
compdef _jt jt
```

Should you require a shortcut to the last-selected worktree, use the built-in `worktree_go_last` helper, which reads the `.last-selected` file:

```bash
alias pl='worktree_go_last ~/path/to/your/main/repo'
```

## Shell Completion

Generate completion scripts for bash, zsh, or fish:

```bash
# Bash
eval "$(lazyworktree --completion bash)"

# Zsh
eval "$(lazyworktree --completion zsh)"

# Fish
lazyworktree --completion fish > ~/.config/fish/completions/lazyworktree.fish
```

Package manager installations (deb, rpm, AUR) include completions automatically.

## Branch Naming Conventions

When creating worktrees with manual branch names, special characters are automatically converted to hyphens for Git and terminal multiplexer (tmux/zellij) compatibility. Branch names can contain letters, numbers, and hyphens; all other characters are converted to hyphens.

| Input | Converted |
|-------|-----------|
| `feature.new` | `feature-new` |
| `bug fix here` | `bug-fix-here` |
| `feature:test` | `feature-test` |

Leading/trailing hyphens are removed, consecutive hyphens collapsed, and length is capped at 50 characters (manual input) or 100 characters (auto-generated).

### Examples

```bash
# Creating a worktree with user input
#
# Type: feature.new
# Creates branch: feature-new

# From a PR with a title "Add user.authentication feature"
# Creates branch: add-user-authentication-feature

# From a GitHub/GitLab issue "#42: Fix the login API"
# Creates branch: issue-42-fix-the-login-api
```

## Custom Initialization and Termination

You may create a `.wt` file in your main repository to define custom commands that execute when creating or removing a worktree. This format is inspired by [wt](https://github.com/taecontrol/wt).

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

- `WORKTREE_BRANCH`: Name of the git branch.
- `MAIN_WORKTREE_PATH`: Path to the main repository.
- `WORKTREE_PATH`: Path to the new worktree being created or removed.
- `WORKTREE_NAME`: Name of the worktree (directory name).

### Security: Trust on First Use (TOFU)

Since `.wt` files permit the execution of arbitrary commands found within a repository, `lazyworktree` implements a **Trust on First Use** security model to prevent malicious repositories from automatically executing code on your system.

- **First Run**: Upon encountering a new or modified `.wt` file, `lazyworktree` will pause and display the commands it intends to execute. You may select **Trust** (run and save), **Block** (skip for now), or **Cancel** the operation.
- **Trusted**: Once trusted, commands run silently in the background until the `.wt` file changes again.
- **Persistence**: Trusted file hashes are stored in `~/.local/share/lazyworktree/trusted.json`.

This behaviour may be configured in `config.yaml` via the `trust_mode` setting:

- **`tofu`** (Default): Prompts for confirmation on new or changed files. Secure and usable.
- **`never`**: Never runs commands from `.wt` files. Safest for untrusted environments.
- **`always`**: Always runs commands without prompting. Useful for personal/internal environments but risky.

### Special Commands

- `link_topsymlinks`: A built-in automation command (not a shell command) that executes without TOFU prompts once the `.wt` file is trusted. It performs the following:
  - Symlinks all untracked and ignored files from the root of the main worktree to the new worktree (excluding subdirectories).
  - Symlinks common editor configurations (`.vscode`, `.idea`, `.cursor`, `.claude`).
  - Ensures a `tmp/` directory exists in the new worktree.
  - Automatically runs `direnv allow` if a `.envrc` file is present.

## Custom Commands

You may define custom keybindings in your `~/.config/lazyworktree/config.yaml` to execute commands within the selected worktree. Custom commands execute interactively (the Terminal User Interface suspends, much like when launching `lazygit`) and appear in the command palette. Should you set `show_output`, lazyworktree pipes the command output through the configured pager.

By default, `t` opens a tmux session with a single `shell` window and `Z` opens a zellij session with the same layout fields. You may override these by defining `custom_commands.t` or `custom_commands.Z`. When `attach` is true, lazyworktree attaches to the session immediately; when false, it displays an information modal with instructions for manual attachment.

The command palette automatically lists all active tmux and zellij sessions that start with the configured session prefix (default: `wt-`) under separate "Active Tmux Sessions" and "Active Zellij Sessions" sections that appear after the Multiplexer section. Selecting an active session allows you to quickly switch to it without manually typing session names. You can customise the session prefix by setting `session_prefix` in your configuration file.

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
    tmux:
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
  Z: # Open a zellij session with multiple tabs
    description: Zellij
    show_help: true
    zellij:
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
| `session_name` | string | `wt:$WORKTREE_NAME` | tmux session name (supports env vars). Colons, slashes, and backslashes are replaced with `-`. |
| `attach` | bool | `true` | If true, attach/switch immediately; if false, show info modal with attach instructions |
| `on_exists` | string | `switch` | Behavior if session exists: `switch`, `attach`, `kill`, `new` |
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
| `session_name` | string | `wt:$WORKTREE_NAME` | zellij session name (supports env vars) |
| `attach` | bool | `true` | If true, attach immediately; if false, show info modal with attach instructions |
| `on_exists` | string | `switch` | Behavior if session exists: `switch`, `attach`, `kill`, `new` |
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

- `WORKTREE_BRANCH`: Name of the git branch
- `MAIN_WORKTREE_PATH`: Path to the main repository
- `WORKTREE_PATH`: Path to the selected worktree
- `WORKTREE_NAME`: Name of the worktree (directory name)
- `REPO_NAME`: Name of the repository (from GitHub/GitLab)

### Supported Key Formats

Custom commands support the same key formats as built-in keybindings:

- **Single keys**: `e`, `s`, `t`, `l`, etc.
- **Modifier combinations**: `ctrl+e`, `ctrl+t`, `alt+s`, etc.
- **Special keys**: `enter`, `esc`, `tab`, `space`, etc.

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

**Custom commands take precedence over built-in keys.** If you define a custom command with key `s`, it shall override the built-in sort toggle. This permits you to fully customise your workflow.

## Key Bindings

| Key | Action |
| --- | --- |
| `Enter` | Jump to worktree (exit and cd) |
| `c` | Create new worktree (from branch, commit, PR/MR, or issue) |
| `m` | Rename selected worktree |
| `D` | Delete selected worktree |
| `d` | View diff in pager (respects pager config) |
| `A` | Absorb worktree into main |
| `X` | Prune merged worktrees (auto-refreshes PR data, then checks PR/branch merge status) |
| `!` | Run arbitrary command in selected worktree (with command history) |
| `p` | Fetch PR/MR status (also refreshes CI checks) |
| `o` | Open PR/MR in browser |
| `ctrl+p`, `:` | Command palette |
| `g` | Open LazyGit |
| `r` | Refresh list |
| `R` | Fetch all remotes |
| `S` | Synchronise with upstream (git pull, then git push, current branch only, requires a clean worktree, honours merge_method) |
| `P` | Push to upstream branch (current branch only, requires a clean worktree, prompts to set upstream when missing) |
| `f` | Filter focused pane (worktrees, files, commits) |
| `/` | Search focused pane (incremental) |
| `alt+n`, `alt+p` | Move selection and fill filter input |
| `↑`, `↓` | Move selection (filter active, no fill) |
| `s` | Cycle sort mode (Path / Last Active / Last Switched) |
| `Home` | Go to first item in focused pane |
| `End` | Go to last item in focused pane |
| `?` | Show help |
| `1` | Switch to Worktree pane (or toggle zoom if already focused) |
| `2` | Switch to Status pane (or toggle zoom if already focused) |
| `3` | Switch to Log pane (or toggle zoom if already focused) |
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

- `alt+n`, `alt+p`: Navigate and update filter input with selected item
- `↑`, `↓`, `ctrl+j`, `ctrl+k`: Navigate list without changing filter input
- `Enter`: Exit filter mode (filter remains active)
- `Esc`, `Ctrl+C`: Exit filter mode

When a filter is active, the pane title shows a filter indicator with `[Esc] Clear` hint. Press `Esc` to clear the filter.

**Search Mode:**

- Type to jump to the first matching item
- `n`, `N`: Next / previous match
- `Enter`: Close search
- `Esc`, `Ctrl+C`: Clear search

**Command History (! command):**

When running arbitrary commands with `!`, command history is persisted per repository:

- `↑`, `↓`: Navigate through command history (most recent first)
- Commands are automatically saved after execution
- History is limited to 100 entries per repository
- Stored in `~/.local/share/lazyworktree/<repo-key>/.command-history.json`

**Command Palette Actions:**

- **Select theme**: Change the application theme with live preview. Available themes: `dracula`, `dracula-light`, `narna`, `clean-light`, `catppuccin-latte`, `rose-pine-dawn`, `one-light`, `everforest-light`, `everforest-dark`, `solarized-dark`, `solarized-light`, `gruvbox-dark`, `gruvbox-light`, `nord`, `monokai`, `catppuccin-mocha`, `modern`, `tokyo-night`, `one-dark`, `rose-pine`, `ayu-mirage`.
- **Create from current branch**: Choose this option after pressing `c` (or opening the palette) to copy the branch you are currently on. A friendly random name is suggested for the new branch (you may edit it), and when uncommitted changes exist an “Include current file changes” checkbox appears beside the name prompt; Tab switches focus to the checkbox, Space toggles it, and enabling it stashes the work and reapplies it inside the new worktree. With the checkbox ticked, any configured `branch_name_script` receives the diff to generate the suggested branch name, so the AI helpers run just as before.

### Mouse Controls

- **Click**: Select and focus panes or items
- **Scroll Wheel**: Scroll through lists and content
  - Worktree table (left pane)
  - Status pane (right top pane)
  - Log table (right bottom pane)

## Configuration

Worktrees are expected to be organised under `~/.local/share/worktrees/<repo_name>` by default, although the application attempts to resolve locations via `gh repo view` or `glab repo view`. Should the repository name not be detectable, lazyworktree falls back to a local `local-<hash>` key for cache and last-selected storage.

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
theme: ""       # Leave empty to auto-detect based on terminal background color
                # (defaults to "dracula" for dark, "dracula-light" for light).
                # Options: "dracula", "dracula-light", "narna", "clean-light",
                #          "solarized-dark", "solarized-light", "gruvbox-dark",
                #          "gruvbox-light", "nord", "monokai", "catppuccin-mocha"
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
# AI-powered branch name generation (works for changes, issues, and PRs)
branch_name_script: "" # Script to generate branch names from diff/issue/PR content
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

Notes:

- `--config` specifies a custom configuration file path, allowing you to override the default XDG config directory location (e.g., `lazyworktree --config ~/my-configs/lazyworktree.yaml`).
- `--worktree-dir` overrides the `worktree_dir` setting.
- `theme` selects the colour theme. Available themes: `dracula`, `dracula-light`, `narna`, `clean-light`, `catppuccin-latte`, `rose-pine-dawn`, `one-light`, `everforest-light`, `everforest-dark`, `solarized-dark`, `solarized-light`, `gruvbox-dark`, `gruvbox-light`, `nord`, `monokai`, `catppuccin-mocha`, `modern`, `tokyo-night`, `one-dark`, `rose-pine`, `ayu-mirage`. Default: auto-detected (`dracula` for dark, `dracula-light` for light).
- `init_commands` and `terminate_commands` execute prior to any repository-specific `.wt` commands (if present).
- `sort_mode` controls the default sort order: `"switched"` (last accessed, default), `"active"` (last commit date), or `"path"` (alphabetical). The old `sort_by_active` option is still supported for backwards compatibility.
- Set `auto_fetch_prs` to `true` to fetch PR data upon startup.
- Set `auto_refresh` to `false` to disable background refresh of git metadata and working tree status.
- Set `refresh_interval` to control background refresh frequency in seconds (e.g., `10`).
- Set `show_icons: false` to disable icons.
- Set `search_auto_select` to `true` to commence with the filter focused (alternatively, pass `--search-auto-select`).
- Set `fuzzy_finder_input` to `true` to enable fuzzy finder suggestions in input dialogs. When enabled, typing in text input fields displays fuzzy-filtered suggestions from available options. Use arrow keys to navigate suggestions and Enter to select.
- The command palette includes MRU (Most Recently Used) sorting by default (`palette_mru: true`). A "Recently Used" section appears at the top of the palette showing your most frequently used commands. The number of commands shown is controlled by `palette_mru_limit` (default: 5). Usage history is stored per-repository in `.command-palette-history.json`. Set `palette_mru: false` to disable this feature.
- Use `max_untracked_diffs: 0` to conceal untracked diffs; `max_diff_chars: 0` disables truncation.
- Execute `lazyworktree --show-syntax-themes` to display the default delta `--syntax-theme` values for each UI theme.
- Use `lazyworktree --theme <name>` to select a UI theme directly; the supported names correspond to those listed above.
- `git_pager` specifies the diff formatter/pager command (default: `delta`). Set to an empty string to disable diff formatting and use plain git diff output. You may also use alternatives such as `diff-so-fancy` or interactive tools like `diffnav` (requires `git_pager_interactive: true`). VS Code users can set `git_pager: code` to open diffs via `git difftool` with proper side-by-side comparison (one VS Code window per changed file).
- `git_pager_args` configures arguments passed to `git_pager`. If omitted and `git_pager` is `delta`, lazyworktree selects a `--syntax-theme` matching your UI theme (Dracula → `Dracula`, Dracula-Light → `Monokai Extended Light`, Narna → `OneHalfDark`, Clean-Light → `GitHub`, Catppuccin Latte → `Catppuccin Latte`, Rosé Pine Dawn → `GitHub`, One Light → `OneHalfLight`, Everforest Light → `Gruvbox Light`, Solarized Dark → `Solarized (dark)`, Solarized Light → `Solarized (light)`, Gruvbox Dark → `Gruvbox Dark`, Gruvbox Light → `Gruvbox Light`, Nord → `Nord`, Monokai → `Monokai Extended`, Catppuccin Mocha → `Catppuccin Mocha`).
- `git_pager_interactive` enables interactive diff viewers that require terminal control (default: `false`). Set to `true` for tools like `diffnav`, `ftdv`, or `tig` that provide keyboard navigation. When enabled, only unstaged changes (`git diff`) are shown, as interactive tools cannot handle the combined diff format. Non-interactive formatters like `delta` and `diff-so-fancy` should keep this set to `false`.
- `pager` designates the pager for `show_output` commands and the diff viewer (default: `$PAGER`, fallback `less --use-color --wordwrap -qcR -P 'Press q to exit..'`, then `more`, then `cat`). When the pager is `less`, lazyworktree configures `LESS=` and `LESSHISTFILE=-` to disregard user defaults.
- `editor` sets the editor command for the Status pane `e` key (default: config value, then `$EDITOR`, then `nvim`, then `vi`).
- `merge_method` controls how the "Absorb worktree" action integrates changes into main and how `S` synchronises with upstream: `rebase` (default) rebases the feature branch onto main then fast-forwards, and uses `git pull --rebase=true`; `merge` creates a merge commit and performs a standard `git pull`.
- `session_prefix` defines the prefix for tmux and zellij session names (default: `wt-`). The command palette filters active sessions by this prefix. Sessions created by lazyworktree will use this prefix, and the palette will only show sessions matching this prefix. Note: tmux does not permit colons (`:`) in session names, so any colons in the prefix will be automatically converted to hyphens (`-`).
- `branch_name_script` executes a script to generate branch name suggestions when creating worktrees from changes, issues, or PRs. The script receives the git diff (for changes), issue title+body (for issues), or PR title+body (for PRs) on stdin and should output a title (for PRs/issues) or branch name (for diffs). The output is available via the `{generated}` placeholder in templates. Refer to [AI-powered branch names](#ai-powered-branch-names) below.
- `issue_branch_name_template` defines the template for issue branch names with placeholders: `{number}`, `{title}` (original title), `{generated}` (AI-generated title, falls back to `{title}`) (default: `"issue-{number}-{title}"`). Examples: `issue-123-fix-bug-in-login`, `issue-123-fix-auth-bug` (using `{generated}`), `fix/123-fix-bug-in-login`.
- `pr_branch_name_template` defines the template for PR branch names with placeholders: `{number}`, `{title}` (original title), `{generated}` (AI-generated title, falls back to `{title}`) (default: `"pr-{number}-{title}"`). Examples: `pr-123-fix-bug`, `pr-123-feat-session-manager` (using `{generated}`), `123-fix-bug`.
- `custom_create_menus` adds custom items to the worktree creation menu (`c` key). Each entry requires a `label` and `command`; `description` is optional. The workflow: you first select a base branch, then the command runs to generate a branch name. By default, commands run non-interactively with a 30-second timeout and their stdout is captured directly. Set `interactive: true` for TUI-based commands (like `jayrah browse` or `fzf`); this suspends lazyworktree, runs the command in the terminal with no timeout, and captures stdout via a temp file. The command output (first line only, whitespace trimmed, case preserved) is used as the suggested branch name. Optionally, specify `post_command` to run a command in the new worktree directory after creation (runs after global/repo `init_commands`); non-interactive post-commands have a 30-second timeout, whilst `post_interactive: true` suspends the TUI with no timeout for interactive post-commands. Post-commands have access to environment variables like `WORKTREE_BRANCH`, `WORKTREE_PATH`, etc. If a post-command fails, the error is shown but the worktree is kept.

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

## CI Status Display

When viewing a worktree with an associated PR/MR, lazyworktree automatically retrieves and displays CI check statuses in the information pane.

- `✓` **Green** - Passed
- `✗` **Red** - Failed
- `●` **Yellow** - Pending/Running
- `○` **Grey** - Skipped
- `⊘` **Grey** - Cancelled

CI status is retrieved lazily (only for the selected worktree) and cached for 30 seconds to maintain UI responsiveness. Press `p` to force a refresh of CI status.

## AI-Powered Branch Names

When creating a worktree from changes (via the command palette), issues, or PRs, you may configure an external script to suggest branch names or titles using AI.

**For PRs and issues:** The script should output a **title** (e.g., `feat-ai-session-manager`) that will be sanitized and available via the `{generated}` placeholder in your template. You can choose whether to use the AI-generated title, the original title, or both.

**For diffs:** The script should output a complete branch name.

This proves useful for integrating AI tools such as [aichat](https://github.com/sigoden/aichat/), [claude code](https://claude.com/product/claude-code), or any other command-line tool capable of generating meaningful branch names from code changes.

> [!NOTE]
> There's no need for a large or cutting-edge model for branch generation. Smaller models are usually cheaper and much faster. Google's `gemini-2.5-flash-lite` is currently the fastest and most reliable choice.

### Configuration

Add `branch_name_script` to your `~/.config/lazyworktree/config.yaml`:

```yaml
# For PRs/issues: generate a title (available via {generated} placeholder)
branch_name_script: "aichat -m gemini:gemini-2.5-flash-lite 'Generate a short title for this PR or issue. Output only the title (like feat-ai-session-manager), nothing else.'"

# Choose which template to use:
pr_branch_name_template: "pr-{number}-{generated}"  # Use AI-generated title
# pr_branch_name_template: "pr-{number}-{title}"    # Use original PR title
# pr_branch_name_template: "pr-{number}-{generated}-{title}"  # Use both!

# For diffs: generate a complete branch name
# branch_name_script: "aichat -m gemini:gemini-2.5-flash-lite 'Generate a short git branch name (no spaces, use hyphens) for this diff. Output only the branch name, nothing else.'"
```

### Template Placeholders

When creating worktrees from PRs or issues, the following placeholders are available:

- `{number}` - The PR/issue number
- `{title}` - The original sanitized PR/issue title (always available)
- `{generated}` - The AI-generated title (falls back to `{title}` if script not configured or returns empty)

**Examples:**

| Template | Result (PR #2: "Add AI session management") | AI generates: `feat-ai-session-manager` |
|----------|---------------------------------------------|----------------------------------------|
| `pr-{number}-{title}` | `pr-2-add-ai-session-management` | Not used |
| `pr-{number}-{generated}` | `pr-2-feat-ai-session-manager` | Used |
| `pr-{number}-{generated}-{title}` | `pr-2-feat-ai-session-manager-add-ai-session-management` | Both used |

If the AI script fails or returns empty output, `{generated}` automatically falls back to the sanitized original title.

### How It Works

1. When you press `c` (or open the command palette) and choose "Create from current branch", the base picker highlights that option; if the selected worktree contains uncommitted modifications, the branch-name prompt surfaces an "Include current file changes" checkbox so you can decide whether to stash and move them into the new worktree. Tab/Shift+Tab cycle focus between the input and checkbox, while Space toggles the box when it is focused.
2. Should `branch_name_script` be configured:
   - **For PRs:** The PR title and body are piped to the script. The script outputs a title, which is sanitized and made available via the `{generated}` placeholder. Your `pr_branch_name_template` determines how it's used.
   - **For issues:** The issue title and body are piped to the script. The script outputs a title, which is sanitized and made available via the `{generated}` placeholder. Your `issue_branch_name_template` determines how it's used.
   - **For diffs:** The git diff is piped to the script. The script outputs a complete branch name.
3. The final branch name is suggested to you.
4. You may edit the suggestion prior to confirmation.

**Example for PR #2 with AI script:**

- Original PR title: "Add AI session management"
- AI script generates: `feat-ai-session-manager`
- Template: `pr-{number}-{generated}`
- Placeholders replaced: `{number}` → `2`, `{generated}` → `feat-ai-session-manager`
- Result: `pr-2-feat-ai-session-manager`

### Script Requirements

- The script receives content on stdin:
  - Git diff for changes
  - Issue title+body for issues
  - PR title+body for PRs
- Output requirements:
  - **For PRs/issues:** Output only a title (e.g., `feat-ai-session-manager`). First line is used.
  - **For diffs:** Output a complete branch name (e.g., `feature/add-caching`). First line is used.
- Should the script fail or return empty output:
  - For changes: the default name (`{current-branch}-changes`) is employed
  - For issues: the issue's actual title is used in the template
  - For PRs: the PR's actual title is used in the template
- The script operates under a 30-second timeout to prevent hanging.

### Environment Variables

The script receives additional context via environment variables:

- `LAZYWORKTREE_TYPE`: The type of creation (`"pr"`, `"issue"`, or `"diff"`)
- `LAZYWORKTREE_NUMBER`: The PR/issue number (empty for diff-based creation)
- `LAZYWORKTREE_TEMPLATE`: The configured template (e.g., `"pr-{number}-{title}"`)
- `LAZYWORKTREE_SUGGESTED_NAME`: The template-generated branch name using the original PR/issue title

These variables allow scripts to adapt behaviour based on context. Examples:

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

- 🐘 Fediverse - <[@chmouel@chmouel.com](https://fosstodon.org/@chmouel)>
- 🐦 Twitter - <[@chmouel](https://twitter.com/chmouel)>
- 📝 Blog  - <[https://blog.chmouel.com](https://blog.chmouel.com)>
