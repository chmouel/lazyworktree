# lazyworktree - Lazy Git Worktree Manager

A Bubble Tea-based TUI for managing Git worktrees efficiently. Visualize status, manage branches, and jump between worktrees with ease.

![Go](https://img.shields.io/badge/go-1.25%2B-blue)

## Features

- **Worktree Management**: Create, rename, delete, absorb, and prune merged worktrees.
- **Command Palette**: Fuzzy search and run actions quickly.
- **Status at a Glance**: View dirty state, ahead/behind counts, and divergence from main.
- **Forge Integration**: Fetch and display associated Pull Request (GitHub) or Merge Request (GitLab) status (via `gh` or `glab` CLI).
- **Diff Viewer**: Three-part diff with optional `delta` support and a full-screen viewer.
- **Commit Details**: Open commit metadata and diffs directly from the log pane.
- **Repo Automation**: `.wt` init/terminate commands with TOFU security.
- **LazyGit Integration**: Launch `lazygit` directly for the selected worktree.
- **Shell Integration**: Jump (cd) directly to selected worktrees upon exit.

## Screenshots

<img width="3835" height="2107" alt="image" src="https://github.com/user-attachments/assets/17ac93bf-6247-496c-981d-7156b8186b33" />

## Prerequisites

- **Go**: 1.25+ (for building from source)
- **Git**: 2.31+ (recommended)
- **Forge CLI**: GitHub CLI (`gh`) or GitLab CLI (`glab`) for repo resolution and PR/MR status.

**Optional:**

- **delta**: For syntax-highlighted diffs.
- **lazygit**: For full TUI git control.

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

You can override the default worktree root:

```bash
lazyworktree --worktree-dir ~/worktrees
```

### Pre-built Binaries

Pre-built binaries for various platforms are available in the [Releases](https://github.com/chmouel/lazyworktree/releases) section.

## Shell Integration (Zsh)

To enable the "jump" functionality (changing your shell's current directory on exit), add the helper functions from `shell/functions.shell` to your `.zshrc`. The helper uses `--output-selection` to write the selected path to a temp file.

Example configuration:

```bash
# Add to .zshrc
source /path/to/lazyworktree/shell/functions.shell

# Create an alias for a specific repository
# worktree storage key is derived from the origin remote (e.g. github.com:owner/repo)
# and falls back to the directory basename when no remote is set.
pm() { worktree_jump ~/path/to/your/main/repo "$@"; }
```

Now you can run `pm` to open the TUI, select a worktree, and upon pressing `Enter`, your shell will `cd` into that directory.

You can also jump directly to a worktree by name and enable completion:

```bash
pm() { worktree_jump ~/path/to/your/main/repo "$@"; }
_pm() { _worktree_jump ~/path/to/your/main/repo; }
compdef _pm pm
```

If you want a shortcut to the last-selected worktree, use the built-in
`worktree_go_last` helper (reads the `.last-selected` file):

```bash
alias pl='worktree_go_last ~/path/to/your/main/repo'
```

## Custom Initialization and Termination

You can create a `.wt` file in your main repository to define custom commands to run when creating or removing a worktree. This format is inspired by [wt](https://github.com/taecontrol/wt).

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

Since `.wt` files allow executing arbitrary commands found in a repository, `lazyworktree` implements a **Trust on First Use** security model to prevent malicious repositories from running code on your machine automatically.

- **First Run**: When `lazyworktree` encounters a new or modified `.wt` file, it will pause and display the commands it intends to run. You can **Trust** (run and save), **Block** (skip for now), or **Cancel** the operation.
- **Trusted**: Once trusted, commands run silently in the background until the `.wt` file changes again.
- **Persistence**: Trusted file hashes are stored in `~/.local/share/lazyworktree/trusted.json`.

You can configure this behavior in `config.yaml` via the `trust_mode` setting:

- **`tofu`** (Default): Prompts for confirmation on new or changed files. Secure and usable.
- **`never`**: Never runs commands from `.wt` files. Safest for untrusted environments.
- **`always`**: Always runs commands without prompting. Useful for personal/internal environments but risky.

### Special Commands

- `link_topsymlinks`: This is a high-level automation command that:
  - Symlinks all untracked and ignored files from the root of the main worktree to the new worktree (excluding subdirectories).
  - Symlinks common editor configurations (`.vscode`, `.idea`, `.cursor`, `.claude`).
  - Ensures a `tmp/` directory exists in the new worktree.
  - Automatically runs `direnv allow` if a `.envrc` file is present.

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
| `p` | Fetch PR/MR status |
| `o` | Open PR/MR in browser |
| `ctrl+p` | Command palette |
| `g` | Open LazyGit |
| `r` | Refresh list |
| `R` | Fetch all remotes |
| `f`, `/` | Filter worktrees |
| `s` | Toggle sort (Name/Last Active) |
| `?` | Show help |

## Configuration

Worktrees are expected to be organized under
`~/.local/share/worktrees/<repo_name>` by default, though the script attempts
to resolve locations via `gh repo view` or `glab repo view`.

### Global Config (YAML)

lazyworktree reads `~/.config/lazyworktree/config.yaml` (or `.yml`) for default
settings. Example (also in [config.example.yaml](./config.example.yaml)):

```yaml
worktree_dir: ~/.local/share/worktrees
sort_by_active: true
auto_fetch_prs: false
max_untracked_diffs: 10
max_diff_chars: 200000
trust_mode: "tofu" # Options: "tofu" (default), "never", "always"
init_commands:
  - link_topsymlinks
terminate_commands:
  - echo "Cleaning up $WORKTREE_NAME"
```

Notes:

- `--worktree-dir` overrides `worktree_dir`.
- `init_commands` and `terminate_commands` run before any repo-specific `.wt`
  commands (if present).
- Set `sort_by_active` to `false` to sort by path.
- Set `auto_fetch_prs` to `true` to fetch PR data on startup.
- Use `max_untracked_diffs: 0` to hide untracked diffs; `max_diff_chars: 0` disables truncation.

## Speed performance

`lazyworktree` is designed to be super snappy:

- **Caching**: It stores the state of your worktrees in `.worktree-cache.json` within your repository-specific folder in your worktree root. This allows the TUI to render in milliseconds upon startup.
- **Background Updates**: As soon as the UI is visible, a background task refreshes the data from Git and updates the cache automatically.
- **Welcome Screen**: If no worktrees are detected (e.g., during first-time use or in an unconfigured directory), a welcome screen guides you through the setup.

## Copyright

[Apache-2.0](./LICENSE)

## Authors

### Chmouel Boudjnah

- 🐘 Fediverse - <[@chmouel@chmouel.com](https://fosstodon.org/@chmouel)>
- 🐦 Twitter - <[@chmouel](https://twitter.com/chmouel)>
- 📝 Blog  - <[https://blog.chmouel.com](https://blog.chmouel.com)>
