# Configuration

Default worktree location: `~/.local/share/worktrees/<organization>-<repo_name>`.

<div class="lw-callout">
  <p><strong>Find settings quickly:</strong> use the map below, then jump to the relevant settings group.</p>
</div>

## Configuration Map

| If you need to... | Go to |
| --- | --- |
| Change theme, icons, or layout behaviour | [Settings -> Themes / Worktree list and refresh](#settings) |
| Tune filter/search and command palette behaviour | [Settings -> Search and palette](#settings) |
| Adjust diff, pager, and editor integration | [Settings -> Diff, pager, and editor](#settings) |
| Configure automation and notes persistence | [Settings -> Worktree lifecycle](#settings) |
| Configure naming scripts and templates | [Settings -> Branch naming](#settings) |
| Override settings per repository | [Git Configuration](#git-configuration) |

## Global Configuration (YAML)

Reads `~/.config/lazyworktree/config.yaml`. Example (also in [`config.example.yaml`](https://github.com/chmouel/lazyworktree/blob/main/config.example.yaml)):

```yaml
worktree_dir: ~/.local/share/worktrees
sort_mode: switched  # Options: "path", "active" (commit date), "switched" (last accessed)
layout: default      # Pane arrangement: "default" or "top"
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
pr_branch_name_template: "pr-{number}-{title}" # Placeholders: {number}, {title}, {generated}, {pr_author}
# Automatic branch name generation (see "Automatically Generated Branch Names")
branch_name_script: "" # Script to generate names from diff/issue/PR content
# Automatic worktree note generation when creating from PR/MR or issue
worktree_note_script: "" # Script to generate notes from PR/issue title+body
# Optional shared note storage file (single JSON for all repositories)
worktree_notes_path: "" # e.g. ~/.local/share/lazyworktree/worktree-notes.json
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

## Configuration Precedence

Highest to lowest priority:

1. **CLI overrides** (`--config` flag)
2. **Git local configuration** (`git config --local`)
3. **Git global configuration** (`git config --global`)
4. **YAML configuration file** (`~/.config/lazyworktree/config.yaml`)
5. **Built-in defaults**

## Git Configuration

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

## Settings

### Themes

- `theme`: colour theme (auto-detected: `dracula` dark, `dracula-light` light). See [Themes](themes.md).
- `lazyworktree --show-syntax-themes`: show delta syntax-theme defaults.
- `lazyworktree --theme <name>`: select UI theme.

### Worktree list and refresh

- `sort_mode`: `"switched"` (last accessed, default), `"active"` (commit date), or `"path"` (alphabetical).
- `layout`: pane arrangement - `"default"` (worktrees left, status/git status/commit stacked right; notes pane splits the left column when a note exists) or `"top"` (worktrees full-width top, notes row below when a note exists, status/git status/commit side-by-side bottom). Toggle at runtime with `L`.
- `auto_refresh`: background refresh of git metadata (default: true).
- `ci_auto_refresh`: periodically refresh CI status for GitHub repositories (default: false).
- `refresh_interval`: refresh frequency in seconds (default: 10).
- `icon_set`: choose icon set (`nerd-font-v3`, `text`).
- `max_untracked_diffs`, `max_diff_chars`: limits for diff display (0 disables).
- `max_name_length`: maximum display length for worktree names (default: 95, 0 disables truncation).

### Search and palette

- `search_auto_select`: start with filter focused (or use `--search-auto-select`).
- `fuzzy_finder_input`: show fuzzy suggestions in input dialogues.
- `palette_mru`: enable MRU sorting in command palette (default: true). Control count with `palette_mru_limit` (default: 5).

### Diff, pager, and editor

- `git_pager`: diff formatter (default: `delta`). Empty string disables formatting.
- `git_pager_args`: arguments for `git_pager`. Auto-selects syntax theme for delta.
- `git_pager_interactive`: set `true` for interactive viewers like `diffnav` or `tig`.
- `git_pager_command_mode`: set `true` for command-based diff viewers like `lumen` that run their own git commands (for example `lumen diff`).
- `pager`: pager for output display (default: `$PAGER`, fallback to `less`).
- `ci_script_pager`: pager for CI logs with direct terminal control. Falls back to `pager`.

Example to strip GitHub Actions timestamps:

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

- `editor`: editor for Status pane `e` key (default: `$EDITOR`, fallback to `nvim`).

### Worktree lifecycle

- `init_commands`, `terminate_commands`: run before repository `.wt` commands.
- `worktree_notes_path`: optional path to store all worktree notes in one shared JSON file. In this mode, note keys are repo/worktree-relative (not absolute paths), making cross-system sync easier.

### Sync and multiplexers

- `merge_method`: `"rebase"` (default) or `"merge"`. Controls Absorb and Sync (`S`) behaviour.
- `session_prefix`: prefix for tmux/zellij sessions (default: `wt-`). Palette filters by this prefix.

### Branch naming

- `branch_name_script`: script for automatic branch suggestions. See [Automatically Generated Branch Names](branch-naming.md).
- `issue_branch_name_template`: template with placeholders `{number}`, `{title}`, `{generated}`.
- `pr_branch_name_template`: template with placeholders `{number}`, `{title}`, `{generated}`, `{pr_author}`.
- `worktree_note_script`: script for automatic worktree notes when creating from PR/MR or issue.

### Custom create menu

- `custom_create_menus`: add custom items to the creation menu (`c` key). Supports `interactive` and `post_command`.
