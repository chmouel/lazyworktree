# lazyworktree - Lazy Git Worktree Manager

A [BubbleTea](https://github.com/charmbracelet/bubbletea)-based Terminal User Interface designed for efficient Git worktree management. Visualise the repository's status, oversee branches, and navigate between worktrees with ease.

![Go](https://img.shields.io/badge/go-1.25%2B-blue)
![Coverage](https://img.shields.io/badge/Coverage-66.6%25-yellow)

## Features

- **Worktree Management**: Create, rename, delete, absorb, and prune merged worktrees.
- **Base Selection**: Select a base branch or commit from a list, or enter a reference when creating a worktree.
- **Forge Integration**: Fetch and display associated Pull Request (GitHub) or Merge Request (GitLab) status, including CI check results (via `gh` or `glab` CLI).
- **Create from PR/MR**: Establish worktrees directly from open pull or merge requests via the command palette.
- **Status at a Glance**: View dirty state, ahead/behind counts, and divergence from main.
- **[Tmux](https://github.com/tmux/tmux/) Integration**: Create and manage tmux sessions per worktree with multi-window support.
- **Diff Viewer**: View diff with optional [delta](https://github.com/dandavison/delta) support.
- **Repo Automation**: `.wt` init/terminate commands with [TOFU](https://en.wikipedia.org/wiki/Trust_on_first_use) security.
- **LazyGit Integration**: Launch [lazygit](https://github.com/jesseduffield/lazygit) directly for the currently selected worktree.

## Screenshots

<img width="3708" height="2462" alt="image" src="https://github.com/user-attachments/assets/8ef892ff-bc57-4960-ba9d-13fa78eccc79" />

## Prerequisites

- **Go**: 1.25+ (for building from source)
- **Git**: 2.31+ (recommended)
- **Forge CLI**: GitHub CLI (`gh`) or GitLab CLI (`glab`) for repo resolution and PR/MR status.

**Optional:**

- **delta**: For syntax-highlighted diffs. (highly recommended)
- **lazygit**: For full TUI git control.
- **tmux**: For TMUX integration support.

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
brew install lazyworktree
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
pm() { worktree_jump ~/path/to/your/main/repo "$@"; }
```

You can now run `pm` to open the Terminal User Interface, select a worktree, and upon pressing `Enter`, your shell will change directory to that location.

To jump directly to a worktree by name with shell completion enabled, use the following:

```bash
pm() { worktree_jump ~/path/to/your/main/repo "$@"; }
_pm() { _worktree_jump ~/path/to/your/main/repo; }
compdef _pm pm
```

Should you require a shortcut to the last-selected worktree, use the built-in `worktree_go_last` helper, which reads the `.last-selected` file:

```bash
alias pl='worktree_go_last ~/path/to/your/main/repo'
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

By default, `t` opens a tmux session with a single `shell` window. You may override this by defining `custom_commands.t`. When `attach` is true, lazyworktree attaches to the session immediately; when false, it displays an information modal with instructions for manual attachment.

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
    description: Open tmux
    show_help: true
    tmux:
      session_name: "${REPO_NAME}_wt_$WORKTREE_NAME"
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

#### tmux fields

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `session_name` | string | `${REPO_NAME}_wt_$WORKTREE_NAME` | tmux session name (supports env vars) |
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
| `c` | Create new worktree |
| `m` | Rename selected worktree |
| `D` | Delete selected worktree |
| `d` | View diff (auto-refreshes) |
| `F` | Full-screen diff viewer |
| `A` | Absorb worktree into main |
| `X` | Prune merged worktrees |
| `!` | Run arbitrary command in selected worktree |
| `p` | Fetch PR/MR status (also refreshes CI checks) |
| `o` | Open PR/MR in browser |
| `ctrl+p`, `P` | Command palette |
| `g` | Open LazyGit |
| `r` | Refresh list |
| `R` | Fetch all remotes |
| `f`, `/` | Filter worktrees |
| `ctrl+j`, `ctrl+k` | Move selection and fill filter input |
| `s` | Toggle sort (Name/Last Active) |
| `?` | Show help |

**Command Palette Actions:**

- **Create from PR/MR**: Select an open PR/MR to establish a worktree. A name is auto-generated (`pr{number}-{sanitized-title}`) which you may edit.
- **Create from changes**: Establish a new worktree from current uncommitted changes in the selected worktree. This stashes all changes (including untracked files), creates a new worktree, and applies the stashed changes to it. Requires a worktree to be selected with uncommitted changes present.

### Mouse Controls

- **Click**: Select and focus panes or items
- **Scroll Wheel**: Scroll through lists and content
  - Worktree table (left pane)
  - Info/Diff viewer (right top pane)
  - Log table (right bottom pane)

## Configuration

Worktrees are expected to be organised under `~/.local/share/worktrees/<repo_name>` by default, although the application attempts to resolve locations via `gh repo view` or `glab repo view`. Should the repository name not be detectable, lazyworktree falls back to a local `local-<hash>` key for cache and last-selected storage.

### Global Configuration (YAML)

lazyworktree reads `~/.config/lazyworktree/config.yaml` (or `.yml`) for default settings. An example configuration is provided below (also available in [config.example.yaml](./config.example.yaml)):

```yaml
worktree_dir: ~/.local/share/worktrees
sort_by_active: true
auto_fetch_prs: false
search_auto_select: false
fuzzy_finder_input: false
max_untracked_diffs: 10
max_diff_chars: 200000
theme: dracula  # Options: "dracula" (default), "narna", "clean-light", "solarized-dark",
                #          "solarized-light", "gruvbox-dark", "gruvbox-light",
                #          "nord", "monokai", "catppuccin-mocha"
delta_path: delta
pager: "less --use-color --wordwrap -qcR -P 'Press q to exit..'"
delta_args:
  - --syntax-theme
  - Dracula
trust_mode: "tofu" # Options: "tofu" (default), "never", "always"
merge_method: "rebase" # Options: "rebase" (default), "merge"
init_commands:
  - link_topsymlinks
terminate_commands:
  - echo "Cleaning up $WORKTREE_NAME"
custom_commands:
  e:
    command: nvim
    description: Open editor
    show_help: true
    wait: false
```

Notes:

- `--worktree-dir` overrides the `worktree_dir` setting.
- `theme` selects the colour theme. Available themes: `dracula`, `narna`, `clean-light`, `solarized-dark`, `solarized-light`, `gruvbox-dark`, `gruvbox-light`, `nord`, `monokai`, `catppuccin-mocha`. Default: `dracula`.
- `init_commands` and `terminate_commands` execute prior to any repository-specific `.wt` commands (if present).
- Set `sort_by_active` to `false` to sort by path.
- Set `auto_fetch_prs` to `true` to fetch PR data upon startup.
- Set `search_auto_select` to `true` to commence with the filter focused and allow `Enter` to select the first match (alternatively, pass `--search-auto-select`).
- Set `fuzzy_finder_input` to `true` to enable fuzzy finder suggestions in input dialogs. When enabled, typing in text input fields displays fuzzy-filtered suggestions from available options. Use arrow keys to navigate suggestions and Enter to select.
- Use `max_untracked_diffs: 0` to conceal untracked diffs; `max_diff_chars: 0` disables truncation.
- Execute `lazyworktree --show-syntax-themes` to display the default delta `--syntax-theme` values for each UI theme.
- Use `lazyworktree --theme <name>` to select a UI theme directly; the supported names correspond to those listed above.
- `delta_args` configures arguments passed to `delta` (defaults follow the UI theme: Dracula → `Dracula`, Narna → `OneHalfDark`, Clean-Light → `GitHub`, Solarized Dark → `Solarized (dark)`, Solarized Light → `Solarized (light)`, Gruvbox Dark → `Gruvbox Dark`, Gruvbox Light → `Gruvbox Light`, Nord → `Nord`, Monokai → `Monokai Extended`, Catppuccin Mocha → `Catppuccin Mocha`).
- `delta_path` specifies the path to the delta executable (default: `delta`). Set to an empty string to disable delta and use plain git diff output.
- `pager` designates the pager for `show_output` commands (default: `$PAGER`, fallback `less --use-color --wordwrap -qcR -P 'Press q to exit..'`, then `more`, then `cat`). When the pager is `less`, lazyworktree configures `LESS=` and `LESSHISTFILE=-` to disregard user defaults.
- `merge_method` controls how the "Absorb worktree" action integrates changes into main: `rebase` (default) rebases the feature branch onto main then fast-forwards; `merge` creates a merge commit.
- `branch_name_script` executes a script to generate branch name suggestions when creating worktrees from changes. The script receives the git diff on stdin and should output a branch name. Refer to [AI-powered branch names](#ai-powered-branch-names) below.

## Themes

lazyworktree includes built-in themes:

| Theme | Background | Best For |
|-------|-----------|----------|
| **dracula** | Dark (#282A36) | Dark terminals, vibrant colours, default |
| **narna** | Charcoal (#0D1117) | Dark terminals, blue highlights |
| **clean-light** | White (#FFFFFF) | Light terminals, soft colours |
| **solarized-dark** | Deep teal (#002B36) | Classic Solarized dark palette |
| **solarized-light** | Cream (#FDF6E3) | Classic Solarized light palette |
| **gruvbox-dark** | Dark grey (#282828) | Gruvbox dark, warm accents |
| **gruvbox-light** | Sand (#FBF1C7) | Gruvbox light, earthy tones |
| **nord** | Midnight blue (#2E3440) | Nord calm cyan accents |
| **monokai** | Olive black (#272822) | Monokai bright neon accents |
| **catppuccin-mocha** | Mocha (#1E1E2E) | Catppuccin Mocha pastels |

To select a theme, configure it in your configuration file:

```yaml
theme: dracula  # or any listed above
```

## CI Status Display

When viewing a worktree with an associated PR/MR, lazyworktree automatically retrieves and displays CI check statuses in the information pane:

- `✓` **Green** - Passed
- `✗` **Red** - Failed
- `●` **Yellow** - Pending/Running
- `○` **Grey** - Skipped
- `⊘` **Grey** - Cancelled

CI status is retrieved lazily (only for the selected worktree) and cached for 30 seconds to maintain UI responsiveness. Press `p` to force a refresh of CI status.

## AI-Powered Branch Names

When creating a worktree from changes (via the command palette), you may configure an external script to suggest branch names. The script receives the git diff on stdin and should output a single branch name.

This proves useful for integrating AI tools such as `aichat`, `claude`, or any other command-line tool capable of generating meaningful branch names from code changes.

### Configuration

Add `branch_name_script` to your `~/.config/lazyworktree/config.yaml`:

```yaml
# Using aichat with Gemini
branch_name_script: "aichat -m gemini:gemini-2.5-flash-lite 'Generate a short git branch name (no spaces, use hyphens) for this diff. Output only the branch name, nothing else.'"
```

### How It Works

1. Upon selecting "Create from changes" in the command palette
2. Should `branch_name_script` be configured, the current diff is piped to the script
3. The script's output (first line only) serves as the suggested branch name
4. You may edit the suggestion prior to confirmation

### Script Requirements

- The script receives the git diff on stdin
- It should output only the branch name (first line is used)
- Should the script fail or return empty output, the default name (`{current-branch}-changes`) is employed
- The script operates under a 30-second timeout to prevent hanging

## Trivia

Previously, this was a Python textual application; however, the startup time proved excessive, prompting a migration to a Go-based [charmbracelet bubble](https://github.com/charmbracelet/bubbles) Terminal User Interface. The original Python implementation remains available for review or testing at <https://github.com/chmouel/lazyworktree/tree/python>

## Copyright

[Apache-2.0](./LICENSE)

## Authors

### Chmouel Boudjnah

- 🐘 Fediverse - <[@chmouel@chmouel.com](https://fosstodon.org/@chmouel)>
- 🐦 Twitter - <[@chmouel](https://twitter.com/chmouel)>
- 📝 Blog  - <[https://blog.chmouel.com](https://blog.chmouel.com)>
