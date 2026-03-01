# Custom Commands

Custom commands let you bind shell commands, tmux sessions, zellij sessions, or output views to keys. Commands can appear in help/footer and in the command palette.

<div class="lw-callout">
  <p><strong>Defaults:</strong> <code>t</code> opens tmux and <code>Z</code> opens zellij. Override either key with your own command definition.</p>
</div>

## Quick Start Patterns

=== "Simple command"

    ```yaml
    custom_commands:
      e:
        command: nvim
        description: Editor
        show_help: true
    ```

=== "Show command output"

    ```yaml
    custom_commands:
      o:
        command: git status -sb
        description: Status
        show_output: true
    ```

=== "Tmux session"

    ```yaml
    custom_commands:
      t:
        description: Tmux
        tmux:
          session_name: "wt:$WORKTREE_NAME"
          attach: true
          on_exists: switch
          windows:
            - name: shell
              command: zsh
            - name: lazygit
              command: lazygit
    ```

## Complete Configuration Example

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

Palette lists sessions matching `session_prefix` (default: `wt-`).

## Field Reference

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `command` | string | **required** | Command to execute |
| `description` | string | `""` | Shown in help and palette |
| `show_help` | bool | `false` | Show in help screen (`?`) and footer |
| `wait` | bool | `false` | Wait for keypress after completion |
| `show_output` | bool | `false` | Show stdout/stderr in pager (ignores `wait`) |
| `new_tab` | bool | `false` | Launch in new terminal tab. Can be used with tmux/zellij (Kitty with remote control enabled, WezTerm, or iTerm) |
| `tmux` | object | `null` | Configure tmux session |
| `zellij` | object | `null` | Configure zellij session |

### tmux Fields

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `session_name` | string | `wt:$WORKTREE_NAME` | Session name (env vars supported, special chars replaced) |
| `attach` | bool | `true` | Attach immediately; if false, show modal with instructions |
| `on_exists` | string | `switch` | Behaviour if session exists: `switch`, `attach`, `kill`, `new` |
| `windows` | list | `[ { name: "shell" } ]` | Window definitions for the session |

If `windows` is empty, a single `shell` window is created.

### tmux Window Fields

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `name` | string | `window-N` | Window name (supports env vars) |
| `command` | string | `""` | Command to run in the window (empty uses your default shell) |
| `cwd` | string | `$WORKTREE_PATH` | Working directory for the window (supports env vars) |

### zellij Fields

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `session_name` | string | `wt:$WORKTREE_NAME` | Session name (env vars supported, special chars replaced) |
| `attach` | bool | `true` | Attach immediately; if false, show modal with instructions |
| `on_exists` | string | `switch` | Behaviour if session exists: `switch`, `attach`, `kill`, `new` |
| `windows` | list | `[ { name: "shell" } ]` | Tab definitions for the session |

If `windows` is empty, a single `shell` tab is created. Session names with `/`, `\`, `:` are replaced with `-`.

### zellij Window Fields

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `name` | string | `window-N` | Tab name (supports env vars) |
| `command` | string | `""` | Command to run in the tab (empty uses your default shell) |
| `cwd` | string | `$WORKTREE_PATH` | Working directory for the tab (supports env vars) |

## Environment Variables

Available to commands and templates:

- `WORKTREE_BRANCH`
- `MAIN_WORKTREE_PATH`
- `WORKTREE_PATH`
- `WORKTREE_NAME`
- `REPO_NAME`

## Supported Key Formats

- Single keys: `e`, `s`
- Modifiers: `ctrl+e`, `alt+t`
- Special keys: `enter`, `esc`, `tab`, `space`

Example:

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

## Key Precedence

Custom command keys override built-in keys.
