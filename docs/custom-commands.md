# Custom Commands

Custom commands let you bind shell commands, tmux sessions, zellij sessions, OCI container execution, or output views to keys. Commands can appear in help/footer and in the command palette.

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

=== "Container command"

    ```yaml
    custom_commands:
      C:
        command: "go test ./..."
        description: Tests in container
        show_output: true
        container:
          image: "golang:1.22"
    ```

=== "Container + tmux"

    ```yaml
    custom_commands:
      ctrl+l:
        description: Dev container
        tmux:
          session_name: "dev:$WORKTREE_NAME"
          windows:
            - name: test
              command: "go test -v ./..."
            - name: shell
        container:
          image: "golang:1.22"
          extra_args:
            - "--network=host"
    ```

=== "Container (image only)"

    ```yaml
    custom_commands:
      ctrl+d:
        description: Default shell in container
        container:
          image: "ubuntu:24.04"
          interactive: true
    ```

=== "Container + entrypoint"

    ```yaml
    custom_commands:
      ctrl+c:
        description: Claude Code in container
        container:
          image: "ghcr.io/anthropics/claude-code:latest"
          entrypoint: "/bin/bash"
          interactive: true
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
| `command` | string | `""` | Command to execute (optional when using a container) |
| `description` | string | `""` | Shown in help and palette |
| `show_help` | bool | `false` | Show in help screen (`?`) and footer |
| `wait` | bool | `false` | Wait for keypress after completion |
| `show_output` | bool | `false` | Show stdout/stderr in pager (ignores `wait`) |
| `new_tab` | bool | `false` | Launch in new terminal tab. Can be used with tmux/zellij (Kitty with remote control enabled, WezTerm, or iTerm) |
| `tmux` | object | `null` | Configure tmux session |
| `zellij` | object | `null` | Configure zellij session |
| `container` | object | `null` | Run inside an OCI container; specifying just an image uses its defaults (combinable with tmux/zellij) |

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

### Container Fields

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `image` | string | **required** | Container image (e.g. `golang:1.22`, `node:20`) |
| `runtime` | string | auto-detect | Container runtime binary (`docker` or `podman`; podman preferred) |
| `mounts` | list | `[]` | Additional bind mounts (worktree auto-mounted to working dir) |
| `env` | map | `{}` | Extra environment variables for the container |
| `working_dir` | string | `/workspace` | Working directory inside the container |
| `extra_args` | list | `[]` | Additional docker/podman run arguments |
| `entrypoint` | string | `""` | Override the image's default entrypoint |
| `interactive` | bool | `false` | Allocate TTY for interactive use (`-it` flags) |

Each mount entry has:

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `source` | string | **required** | Host path |
| `target` | string | **required** | Container path |
| `read_only` | bool | `false` | Mount as read-only |

The worktree path is automatically mounted to the working directory. If a user-specified mount targets the same path as `working_dir`, the automatic mount is skipped. WORKTREE_* environment variables are forwarded into the container automatically.

**`entrypoint` vs `command`:** The `entrypoint` overrides the container image's default entrypoint (the binary that runs inside the container), whilst `command` provides arguments passed to that entrypoint via `sh -c`. When both are set, the entrypoint runs with the command as its argument. When only `entrypoint` is set (no `command`), the container runs the entrypoint directly — useful for interactive shells or tools that need no additional arguments. When only `command` is set, it runs under the image's default entrypoint. When neither is set, the container runs with its image defaults.

When combined with `tmux` or `zellij`, each window/tab command is individually wrapped in a container invocation.

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
