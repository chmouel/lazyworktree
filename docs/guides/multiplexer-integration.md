# Multiplexer Integration

LazyWorktree can create and manage tmux, zellij, or Herdr sessions directly from the TUI, giving each worktree a dedicated terminal environment.


## Overview

With multiplexer integration:

- Create dedicated tmux, zellij, or Herdr sessions for each worktree
- Define custom window/tab layouts with specific commands
- Switch to existing sessions or create new ones
- Manage sessions from the [command palette](../core/command-palette.md)

Default keybindings: `t` for tmux, `Z` for zellij, `H` for Herdr (configurable via [custom commands](../custom-commands.md)).

## tmux Configuration

### Basic Session

```yaml
custom_commands:
  t:
    description: Tmux session
    show_help: true
    tmux:
      attach: true
      on_exists: switch
```

### Multi-Window Session

```yaml
custom_commands:
  t:
    description: Tmux with layout
    show_help: true
    tmux:
      attach: true
      on_exists: switch
      windows:
        - name: editor
          command: nvim
        - name: shell
          command: zsh
        - name: lazygit
          command: lazygit
        - name: tests
          command: npm run test:watch
          cwd: $WORKTREE_PATH/tests
```

### tmux Field Reference

| Field | Type | Default | Description |
| --- | --- | --- | --- |
| `session_name` | string | `#<number> <title>` for pull request worktrees, otherwise the worktree name | Session name (env vars supported, special chars replaced) |
| `attach` | bool | `true` | Attach immediately; if `false`, show modal with instructions |
| `on_exists` | string | `switch` | Behaviour if session exists: `switch`, `attach`, `kill`, `new` |
| `windows` | list | `[{name: "shell"}]` | Window definitions for the session |

#### Window Fields

| Field | Type | Default | Description |
| --- | --- | --- | --- |
| `name` | string | `window-N` | Window name (supports env vars) |
| `command` | string | `""` | Command to run (empty uses default shell) |
| `cwd` | string | `$WORKTREE_PATH` | Working directory (supports env vars) |

## zellij Configuration

### Basic Session

```yaml
custom_commands:
  Z:
    description: Zellij
    show_help: true
    zellij:
      session_name: "wt:$WORKTREE_NAME"
      attach: true
      on_exists: switch
```

### Multi-Tab Session

```yaml
custom_commands:
  Z:
    description: Zellij with tabs
    show_help: true
    zellij:
      session_name: "wt-$WORKTREE_NAME"
      attach: true
      on_exists: switch
      windows:
        - name: editor
          command: nvim
        - name: shell
        - name: server
          command: npm run dev
        - name: tests
          command: npm run test:watch
```

### zellij Field Reference

| Field | Type | Default | Description |
| --- | --- | --- | --- |
| `session_name` | string | `wt:$WORKTREE_NAME` | Session name (env vars supported, special chars replaced) |
| `attach` | bool | `true` | Attach immediately; if `false`, show modal with instructions |
| `on_exists` | string | `switch` | Behaviour if session exists: `switch`, `attach`, `kill`, `new` |
| `windows` | list | `[{name: "shell"}]` | Tab definitions for the session |

#### Tab Fields

| Field | Type | Default | Description |
| --- | --- | --- | --- |
| `name` | string | `window-N` | Tab name (supports env vars) |
| `command` | string | `""` | Command to run (empty uses default shell) |
| `cwd` | string | `$WORKTREE_PATH` | Working directory (supports env vars) |

!!! important
    Session names with `/`, `\`, or `:` are replaced with `-`. If `windows` is empty, a single shell window/tab is created by default.

## Herdr Configuration

Herdr organises terminals into workspaces, each containing tabs. LazyWorktree maps configured `windows` to tabs: the first window becomes the workspace's initial tab, and its `name` and `cwd` are applied to that tab, whilst each additional window becomes another tab.

Two modes are available:

- `workspace` (default): one workspace per worktree inside an existing Herdr session.
- `session`: one named Herdr session per worktree, with its own workspace layout.

### Workspace per Worktree

```yaml
custom_commands:
  H:
    description: Herdr
    show_help: true
    herdr:
      mode: workspace
      attach: true
      on_exists: switch
      windows:
        - name: editor
          command: nvim
        - name: shell
        - name: tests
          command: npm run test:watch
```

By default the workspace is created in the session LazyWorktree is running in, or in the default Herdr session when no session is ambient. An empty `herdr_session` inherits the current session; set `herdr_session: default` to target the default session explicitly. A configured session that matches the one you are attached to reuses your current Herdr socket, so focusing works even when Herdr was started with a custom socket path. LazyWorktree starts a detached Herdr server when the target session is not running.

### Worktree Grouping

Workspaces are opened with `herdr worktree open`, as Herdr's own worktree picker does, so each one nests under its repository in the Herdr sidebar. Herdr opens the repository's main workspace alongside when it is not already open, and the repository is trusted on your behalf. When the worktree has no main checkout (a bare repository) or Herdr refuses the request, LazyWorktree falls back to a plain workspace. Set `worktree_group: false` to always create plain workspaces.

Grouped workspaces are recognised by their checkout path as well as by the LazyWorktree tag, so a workspace opened through Herdr's own picker is reused, listed in the command palette, and replaced by `on_exists: kill` like one LazyWorktree created. Herdr refuses to close a repository workspace while worktree workspaces are grouped under it; close those first.

Deleting a worktree from LazyWorktree, or with `lazyworktree delete` or `lazyworktree cleanup`, closes its Herdr workspaces in every running Herdr session, along with any programmes running in them. The workspace hosting LazyWorktree itself is left open.

### Creating Worktrees with Herdr

Herdr can create the worktree itself, so each new worktree arrives as a workspace grouped under its repository. This is off by default; enable it in your configuration:

```yaml
use_herdr_for_workspace_create: true
herdr_workspace_create_layout: H # optional
```

For a single run, pass `--config=lw.use_herdr_for_workspace_create=true`. The setting applies to every creation path: `lazyworktree create`, the TUI, and the shell helpers that launch it.

- Without `herdr_workspace_create_layout`, the worktree gets Herdr's plain workspace, labelled with the worktree name, in the current Herdr session (or the default session outside Herdr).
- With a layout key, the custom command bound to that key supplies the session, label, tabs and environment, exactly as pressing the key in the worktrees pane would. A binding in the worktrees pane wins over a universal one. The command's `worktree_group` setting is ignored, because creation through Herdr always groups. When the key is missing or has no workspace-mode `herdr` block, LazyWorktree warns and uses the plain workspace.
- `herdr_session` is expanded before the worktree exists, so only the worktree, branch and repository variables are available to it; pull request, issue and other contextual values are empty at that point.

LazyWorktree still prepares branches itself, so branch tracking and pull request upstreams are unchanged. A detached commit or tag is checked out with Git and then opened in Herdr. Should Herdr fail, the worktree is created with Git instead, and a branch Herdr left behind is deleted only when LazyWorktree created it.

The layout is applied once the init commands have finished. If they fail and the worktree is removed, its workspace is closed too; if the worktree is kept, the workspace stays as Herdr's plain workspace. A workspace that was already open is never laid out or closed by creation, because LazyWorktree cannot prove it created it; deleting the worktree closes it as usual.

When a `.wt` file awaits trust, **Block (Skip)** skips its commands but still applies the layout, whilst **Cancel Setup** keeps the worktree and skips both the commands and the layout.

New workspaces are focused only inside a Herdr client attached to the target session, and only when the layout command sets `attach` (plain workspaces always qualify). A TUI started by the shell helpers, or with `--output-selection`, never focuses them, so that it stays in front until you leave it and the helper can change directory. In the TUI, Herdr warnings go to the debug log; the CLI prints them on stderr.

### Named Session per Worktree

```yaml
custom_commands:
  H:
    description: Herdr session
    show_help: true
    herdr:
      mode: session
      session_name: "wt-$WORKTREE_NAME"
      attach: true
      on_exists: switch
      windows:
        - name: editor
          command: nvim
        - name: shell
```

### Herdr Field Reference

| Field | Type | Default | Description |
| --- | --- | --- | --- |
| `mode` | string | `workspace` | `workspace` creates a workspace per worktree; `session` creates a named Herdr session |
| `session_name` | string | `#<number> <title>` for pull request worktrees, otherwise the worktree name | Workspace label in workspace mode, session name in session mode (env vars supported; session names default to `session_prefix` plus the worktree name) |
| `herdr_session` | string | `""` | Herdr session hosting the workspace (workspace mode only; empty inherits the current session, `default` targets the default session) |
| `attach` | bool | `true` | Attach or focus immediately; if `false`, show modal with instructions |
| `on_exists` | string | `switch` | Behaviour if the workspace or session exists: `switch`, `attach`, `kill`, `new` (for Herdr, `attach` is an alias of `switch`) |
| `worktree_group` | bool | `true` | Open workspaces through Herdr's worktree support so they nest under their repository in the sidebar; `false` creates plain workspaces |
| `windows` | list | `[{name: "shell"}]` | Tab definitions for the workspace |

#### Tab Fields

| Field | Type | Default | Description |
| --- | --- | --- | --- |
| `name` | string | `window-N` | Tab label (supports env vars) |
| `command` | string | `""` | Command to run (empty uses default shell) |
| `cwd` | string | `$WORKTREE_PATH` | Working directory (supports env vars) |

!!! note
    Herdr session names are limited to letters, digits, `.`, `_`, and `-`, with a maximum of 64 bytes. Other characters are replaced with `-`, and a name that sanitises to nothing is rejected rather than silently falling back to the default session. Workspace labels are used exactly as configured, so they may contain spaces, colons, slashes or any other characters. The first tab takes the `name` and `cwd` configured for the first window. In workspace mode, LazyWorktree focuses the workspace directly when already running inside Herdr; otherwise it focuses a reused workspace before attaching. When the configured `herdr_session` differs from the session hosting the current client, LazyWorktree shows attach instructions instead, because focusing a workspace in another session cannot display it. In session mode, Herdr cannot switch between sessions from inside a running client, so LazyWorktree always shows attach instructions instead of nesting a second client — including when the session already exists.

!!! warning
    Herdr offers no reliable completion marker for a workspace layout. When a previous attempt failed part-way, a retry reuses the workspace that was already created and does **not** re-run the configured commands, so a partially configured workspace can be reused as-is. Use `on_exists: kill` to discard it and build a fresh layout; `on_exists: new` instead builds a fresh workspace under an incremented label and leaves the old one in place. In session mode, these options apply when the session is running, and reusing a running session ensures the worktree workspace exists, creating it when the previous attempt never got that far.

!!! note
    `new_tab` is honoured when LazyWorktree runs outside Herdr. Inside Herdr, workspace mode focuses the workspace in place and session mode shows attach instructions, because Herdr cannot nest a client or switch sessions from within a running client. A workspace in another session also shows attach instructions.

!!! warning
    In session mode, `on_exists` acts on the normalised session name, because Herdr sessions carry no worktree tag. If sanitisation makes two worktrees share a session name, killing or incrementing one can target the other worktree's session. Workspaces are found by their worktree tag, so this does not apply to them.

## on_exists Behaviour

Controls what happens when the worktree already has a session (see [Session Discovery](#session-discovery)):

| Value | Behaviour |
| --- | --- |
| `switch` | Switch to the existing session (default) |
| `attach` | Attach to the existing session in the current terminal |
| `kill` | Kill the existing session and create a new one |
| `new` | Create a new session with an incremented name (e.g., `wt:feature-1`) |

!!! note
    For Herdr, `attach` and `switch` both reuse the existing workspace or session; `attach` is effectively an alias of `switch`. A reused workspace is focused before the external client attaches, so the attach lands on the right workspace. When Herdr is already nested inside a running client, session mode shows attach instructions rather than switching.

## Environment Variables

All session configuration fields support environment variable substitution:

| Variable | Description | Example |
| --- | --- | --- |
| `$WORKTREE_NAME` | Name of the worktree | `my-feature` |
| `$WORKTREE_BRANCH` | Branch name for the worktree | `feature/my-feature` |
| `$WORKTREE_PATH` | Full path to the worktree directory | `/home/user/worktrees/my-feature` |
| `$MAIN_WORKTREE_PATH` | Path to the main/root worktree | `/home/user/repo` |
| `$REPO_NAME` | Repository key, usually `owner/repo` | `chmouel/lazyworktree` |
| `$REPO_OWNER` | Repository owner when available | `chmouel` |
| `$REPO_REPONAME` | Repository name without the owner | `lazyworktree` |
| `$LAZYWORKTREE_TYPE` | Source context, such as `pr`, `issue`, or `diff` when known | `pr` |
| `$LAZYWORKTREE_NUMBER` | PR/MR or issue number when known | `42` |
| `$LAZYWORKTREE_TITLE` | PR/MR or issue title when known | `Add worktree context` |

Contextual `LAZYWORKTREE_*` values are empty when no source context is known.

### Example Using Environment Variables

```yaml
custom_commands:
  t:
    description: Project session
    tmux:
      session_name: "$REPO_NAME:$WORKTREE_NAME"
      windows:
        - name: "$WORKTREE_BRANCH"
          command: nvim
          cwd: $WORKTREE_PATH
        - name: root
          command: zsh
          cwd: $MAIN_WORKTREE_PATH
```

## Session Discovery

LazyWorktree tags the tmux sessions and Herdr workspaces it creates with the worktree they belong to: tmux sessions carry a `@lazyworktree_path` user option and Herdr workspaces a `lazyworktree_id` token. The tag, rather than the name, is what finds the session again, so `session_name` may be anything you find readable and changing it later does not orphan existing sessions. The worktree's own session is reused under its existing name. Without `session_name`, sessions and workspaces are named `#<number> <title>` for pull request worktrees (shortened to 40 characters) and after the worktree otherwise, and a session still using an earlier default name (`wt-<worktree>` or `wt:<worktree>`) is picked up and tagged.

When no tagged session exists, a session with the configured name is reused only if it is untagged, and it is tagged on the spot. `on_exists: kill` never closes such an untagged session, since it may be one you made yourself; only tagged sessions and those under an earlier default name are replaced. A name already used by another worktree's session is reported as a conflict instead of being reused or killed; change `session_name` or use `on_exists: new`. If several sessions are tagged for one worktree (for instance after `on_exists: new`), the one using the configured name is preferred, and `on_exists: kill` refuses to choose between them.

Several untagged sessions sharing one name (Herdr allows duplicate workspace labels) are treated the same way: one is reused, and `on_exists: kill` refuses to close any of them.

zellij sessions and Herdr sessions (session mode) carry no tag and are still found by name only. The workspace inside a Herdr session is tagged as usual.

Herdr forgets workspace tokens when its server restarts, although workspaces and labels survive. Grouped workspaces (see [Worktree Grouping](#worktree-grouping)) are still found by their checkout path. Plain workspaces are found by their name, and running the custom command again tags them once more. Until then, the palette lists a plain workspace only if its label starts with `session_prefix`, and if its name no longer matches `session_name` (for instance after a pull request title change) a new workspace is created instead.

### Session Prefix

`session_prefix` lists untagged sessions in the command palette, such as zellij sessions or tmux sessions created by earlier releases:

```yaml
session_prefix: "wt-"
```

Tagged sessions are always listed, whatever their name. Untagged ones are listed only when their names start with the prefix. Use a distinctive prefix like `wt-` or `work-` to keep LazyWorktree sessions apart from others.

## New Terminal Tab

Launch multiplexer sessions in a new terminal tab instead of the current one:

```yaml
custom_commands:
  t:
    description: Tmux in new tab
    new_tab: true
    tmux:
      attach: true
```

!!! tip
    New-tab support requires Kitty (with remote control enabled), WezTerm, or iTerm.

## CLI Integration

Launch multiplexer sessions from scripts or the command line:

```bash
# Launch tmux session for a specific worktree
lazyworktree exec --key=t --workspace=my-feature

# Launch zellij session
lazyworktree exec -k Z -w my-feature

# Auto-detect worktree from current directory
cd ~/worktrees/repo/my-feature
lazyworktree exec --key=t
```

## Command Palette Session Management

Press `F1`, `Ctrl+p`, or `:` to open the command palette, where you can:

- View all active LazyWorktree sessions, each labelled with its full name and the worktree it belongs to
- Switch between sessions quickly
- Create new sessions for worktrees that lack one
- Sessions are sorted by MRU (Most Recently Used) order
