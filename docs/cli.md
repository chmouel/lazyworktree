# CLI Usage

The CLI supports listing, creating, deleting, renaming, and executing commands in worktrees without launching the full TUI.

<div class="lw-callout">
  <p><strong>Quick command cookbook:</strong> <code>lazyworktree list</code>, <code>lazyworktree create ...</code>, <code>lazyworktree delete ...</code>, and <code>lazyworktree exec ...</code>.</p>
</div>

## Config Overrides

```bash
lazyworktree --worktree-dir ~/worktrees

# Override config values from CLI
lazyworktree --config lw.theme=nord --config lw.sort_mode=active
```

## Listing Worktrees

```bash
lazyworktree list              # Table output (default)
lazyworktree list --pristine   # Paths only (scripting)
lazyworktree list --json       # JSON output
lazyworktree ls                # Alias
```

`--pristine` and `--json` are mutually exclusive.

## Creating Worktrees

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
lazyworktree create -P -q "dark"             # Pre-filter interactive PR selection
lazyworktree create -I --query "login"       # Pre-filter interactive issue selection
lazyworktree create --from-pr 123 --no-workspace        # Branch only, no worktree
lazyworktree create --from-issue 42 --no-workspace      # Branch only, no worktree
lazyworktree create -I --no-workspace                    # Interactive issue, branch only
lazyworktree create -P --no-workspace                    # Interactive PR, branch only
lazyworktree create my-feature --exec 'npm test'        # Run command after creation
```

`--exec` runs after successful creation. It executes in the new worktree directory, or in the current directory when used with `--no-workspace`.

Shell mode follows your shell:

- `zsh -ilc`
- `bash -ic`
- otherwise `-lc`

PR creation behaviour:

- Worktree name always uses the generated worktree name.
- Local branch name uses the PR branch when you are the PR author.
- Otherwise it uses the generated name.
- If requester identity cannot be resolved, it falls back to the PR branch name.

For complete CLI docs, run `man lazyworktree` or `lazyworktree --help`.

## Deleting Worktrees

```bash
lazyworktree delete                # Delete worktree and branch
lazyworktree delete --no-branch    # Delete worktree only
```

## Renaming Worktrees

```bash
lazyworktree rename new-feature-name              # rename current worktree (detected from cwd)
lazyworktree rename feature new-feature-name
lazyworktree rename /path/to/worktree new-worktree-name
```

During rename, the branch is renamed only if the current worktree directory name matches the branch name.

## Running Commands in Worktrees

Execute a shell command or trigger a custom command key action:

=== "Run shell command"

    ```bash
    lazyworktree exec --workspace=my-feature "make test"
    lazyworktree exec -w my-feature "git status"

    # Auto-detect worktree from current directory
    cd ~/worktrees/repo/my-feature
    lazyworktree exec "npm run build"
    ```

=== "Run custom command key"

    ```bash
    lazyworktree exec --key=t --workspace=my-feature  # Launch tmux session
    lazyworktree exec -k z -w my-feature              # Launch zellij session

    # Auto-detect worktree and trigger key action
    cd ~/worktrees/repo/my-feature
    lazyworktree exec --key=t
    ```

`exec` command behaviour:

- Uses `--workspace` (`-w`) to target a worktree by name or path.
- Auto-detects worktree from current directory when `--workspace` is omitted.
- Accepts either a positional shell command or `--key` to trigger a custom command.
- Sets `WORKTREE_*` environment variables (same as custom commands in the TUI).
- Supports shell, tmux, zellij, and show-output command types.
- `new-tab` commands are not supported in CLI mode.
