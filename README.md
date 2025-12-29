# lazyworktree - Lazy Git Worktree Manager

A Textual-based TUI for managing Git worktrees efficiently. Visualize status, manage branches, and jump between worktrees with ease.

![Python](https://img.shields.io/badge/python-3.12%2B-blue)

## Features

- **Worktree Management**: Create, rename, delete, and absorb worktrees seamlessly.
- **Status at a Glance**: View dirty state, ahead/behind counts, and divergence from main.
- **Forge Integration**: Fetch and display associated Pull Request (GitHub) or Merge Request (GitLab) status (via `gh` or `glab` CLI).
- **Diff Viewer**: Integrated diff viewer with optional `delta` support.
- **LazyGit Integration**: Launch `lazygit` directly for the selected worktree.
- **Shell Integration**: Jump (cd) directly to selected worktrees upon exit.

# Screenshots

<img width="1752" height="1071" alt="image" src="https://github.com/user-attachments/assets/b1143b79-dec3-4c72-be69-f88c49f6d160" />
<img width="1734" height="1077" alt="image" src="https://github.com/user-attachments/assets/047cba26-7886-436c-a782-0530f8efe12e" />

## Prerequisites

- **Python**: 3.12+
- **Git**: 2.31+ (recommended)
- **Forge CLI**: GitHub CLI (`gh`) or GitLab CLI (`glab`) for repo resolution and PR/MR status.
- **uv**: Recommended for dependency management and running.

**Optional:**

- **delta**: For syntax-highlighted diffs.
- **lazygit**: For full TUI git control.

## Run

### Directly with uvx

no drama no worries, just install uv from your favorite method and run:

```shell
uvx git+https://github.com/chmouel/lazyworktree@main
```

this will run `lazyworktree` directly without installing anything globally.

## Installation

### Using uv (Recommended)

Clone the repository and run directly:

```bash
git clone https://github.com/yourusername/lazyworktree.git
cd lazyworktree
uv run main.py
```

You can override the default worktree root:

```bash
uv run main.py --worktree-dir ~/worktrees
```

## Shell Integration (Zsh)

To enable the "jump" functionality (changing your shell's current directory on exit), add the helper functions from `shell/functions.shell` to your `.zshrc`.

Example configuration:

```bash
# Add to .zshrc
source /path/to/lazyworktree/shell/functions.shell

# Create an alias for a specific repository
# assumes worktrees are stored in ~/.local/share/worktrees/<repo_name>
pm() { worktree_jump ~/path/to/your/main/repo "$@"; }
```

Now you can run `pm` to open the TUI, select a worktree, and upon pressing `Enter`, your shell will `cd` into that directory.

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
| `D` | Delete selected worktree |
| `d` | View diff (auto-refreshes) |
| `p` | Fetch PR/MR status |
| `g` | Open LazyGit |
| `r` | Refresh list |
| `/` | Filter worktrees |
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
