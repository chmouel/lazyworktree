# Integration Caveats

Known caveats across shells, pagers, and command execution modes.


## Shell Execution Mode Differences

`--exec` and custom command execution invoke your shell with different flags depending on the detected shell:

| Shell | Flags | Effect |
| --- | --- | --- |
| `zsh` | `-ilc` | Interactive + login: sources `.zshrc`, `.zprofile`, `.zshenv` |
| `bash` | `-ic` | Interactive: sources `.bashrc` (but not `.bash_profile` unless login) |
| Others | `-lc` | Login only: sources login profile but not interactive config |

### Implications

- Aliases and functions in `.bashrc` or `.zshrc` are available in `zsh` and `bash` modes, but may not be available in other shells.
- Login-profile variables in `.bash_profile` and `.zprofile` are loaded in all modes.
- Large `.zshrc` files with plugin managers can slow command execution.
- If a command works in your terminal but fails in LazyWorktree, the shell mode difference is the most likely cause

### Debugging

Run a command with explicit shell flags to reproduce the LazyWorktree environment:

```bash
# Simulate zsh execution mode
zsh -ilc 'your-command-here'

# Simulate bash execution mode
bash -ic 'your-command-here'
```

## Pager Integration Caveats

LazyWorktree pipes diff and CI log output through your configured pager. Not all pagers work identically in a TUI context.

### Tested combinations

| Pager | Works | Notes |
| --- | --- | --- |
| `delta` | Yes | Set `git_pager_interactive: true` for full interactivity |
| `diff-so-fancy` | Yes | Pipe to `less`: `diff-so-fancy \| less` |
| `diffnav` | Yes | Set `git_pager_interactive: true` |
| `less` | Yes | Default pager, works out of the box |
| `bat` | Yes | Works for CI log viewing |

### Common issues

- Interactive pagers that accept keyboard input require `git_pager_interactive: true`. Without it, the pager may hang or display incorrectly.
- Command-mode pagers that read stdin and exit may require `git_pager_command_mode: true`.
- Test CI log formatting scripts in your shell before setting them as `ci_script_pager`:
    ```bash
    echo "sample log output" | your-pager-command
    ```

## Multiplexer Caveats

### Session name sanitisation

tmux, zellij, and Herdr session names are sanitised to remove characters that are invalid in session identifiers. If your worktree path contains special characters, the resulting session name may differ from what you expect.

### Existing session behaviour

When a multiplexer session already exists for a worktree, behaviour depends on the `on_exists` configuration:

- `attach`: attach to the existing session (default)
- `replace`: close the existing session and create a new one
- `ignore`: do nothing

### CLI `exec` limitations

CLI `exec` has no terminal tab to open, so `new_tab` is handled per command type. tmux and zellij commands ignore it and attach in the current terminal. Herdr and plain commands with `new_tab` are rejected; remove the setting for commands that need to work from the CLI.

## Trust Model Caveats for `.wt`

The TOFU (Trust On First Use) model for `.wt` file execution has several implications:

- `trust_mode: never` skips all `.wt` commands, including init and terminate hooks.
- In `tofu` mode, changing a `.wt` file invalidates its stored hash, even if the change is whitespace. LazyWorktree prompts you to approve the file again before running the changed commands.
- LazyWorktree stores trust decisions by file content hash in `~/.local/share/lazyworktree/trusted.json`. Deleting this file resets all trust decisions.
- Each developer must approve a changed `.wt` file. Trust decisions are local and cannot be shared across machines.
