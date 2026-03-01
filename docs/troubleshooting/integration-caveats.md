# Integration Caveats

Known caveats across shells, pagers, and command execution modes.

<div class="mint-callout">
  <p><strong>Use this page when:</strong> commands, pagers, or multiplexer features behave unexpectedly in LazyWorktree.</p>
</div>

## Shell Execution Mode Differences

`--exec` and custom command execution invoke your shell with different flags depending on the detected shell:

| Shell | Flags | Effect |
| --- | --- | --- |
| `zsh` | `-ilc` | Interactive + login: sources `.zshrc`, `.zprofile`, `.zshenv` |
| `bash` | `-ic` | Interactive: sources `.bashrc` (but not `.bash_profile` unless login) |
| Others | `-lc` | Login only: sources login profile but not interactive config |

### Implications

- **Aliases and functions** defined in `.bashrc` or `.zshrc` are available in `zsh` and `bash` modes but may not be available in other shells
- **Environment variables** set in login profiles (`.bash_profile`, `.zprofile`) are loaded in all modes
- **Shell startup time** affects command execution — heavyweight `.zshrc` files with plugin managers can add noticeable delay
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

- **Interactive pagers** (those that accept keyboard input) require `git_pager_interactive: true` — without it, the pager may hang or not display correctly
- **Command-mode pagers** (those that process stdin and exit) may require `git_pager_command_mode: true`
- **CI log formatting scripts** should be tested independently in your shell before configuring them as `ci_script_pager`:
    ```bash
    echo "sample log output" | your-pager-command
    ```

## Multiplexer Caveats

### Session name sanitisation

tmux and zellij session names are sanitised to remove characters that are invalid in session identifiers. If your worktree path contains special characters, the resulting session name may differ from what you expect.

### Existing session behaviour

When a multiplexer session already exists for a worktree, behaviour depends on the `on_exists` configuration:

- `attach`: attach to the existing session (default)
- `replace`: close the existing session and create a new one
- `ignore`: do nothing

### CLI `exec` limitations

`new-tab` commands are not supported in CLI `exec` mode. They require a running multiplexer session, which is only available inside the TUI. Use `shell` or `command` types for commands that need to work from the CLI.

## Trust Model Caveats for `.wt`

The TOFU (Trust On First Use) model for `.wt` file execution has several implications:

- **`trust_mode: never`** blocks all `.wt` command execution — init and terminate hooks will never run, regardless of the `.wt` file's content
- **Modified `.wt` files** trigger trust re-evaluation in `tofu` mode. Any change to the file (including whitespace) invalidates the stored hash, and you will be prompted to re-approve. This is by design — it prevents silently executing changed commands
- **Trust decisions** are stored per-file in `~/.local/share/lazyworktree/trusted.json`, keyed by a hash of the file content. Deleting this file resets all trust decisions
- **Team workflows**: if multiple developers modify `.wt` files, each developer will be prompted independently when the hash changes. There is no way to share trust decisions across machines
