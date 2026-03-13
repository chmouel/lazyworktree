# CLI Commands Reference

This page is generated from `internal/bootstrap/commands.go`. Run `make docs-sync` after changing command definitions.

<!-- BEGIN GENERATED:cli-commands -->
| Command | Usage | Args | Aliases | Guide |
| --- | --- | --- | --- | --- |
| `list` | List all worktrees | `-` | `ls` | [`list`](list.md) |
| `create` | Create a new worktree | `[worktree-name]` | - | [`create`](create.md) |
| `delete` | Delete a worktree | `[worktree-path]` | - | [`delete`](delete.md) |
| `rename` | Rename a worktree | `<new-name> \| <worktree> <new-name>` | - | [`rename`](rename.md) |
| `exec` | Run a command or trigger a key action in a worktree | `[command]` | - | [`exec`](exec.md) |
| `note` | Show or edit worktree notes | `-` | - | [`note`](note.md) |

## `list`

List all worktrees

| Flag | Type | Usage |
| --- | --- | --- |
| `--json` | `bool` | Output as JSON |
| `--main`, `-m` | `bool` | Show only the main branch worktree |
| `--no-agent` | `bool` | Skip agent session data in JSON output (faster for scripting) |
| `--pristine`, `-p` | `bool` | Output paths only (one per line, suitable for scripting) |

## `create`

Create a new worktree

| Flag | Type | Usage |
| --- | --- | --- |
| `--description` | `string` | Set a description on the new worktree |
| `--exec`, `-x` | `string` | Run a shell command after creation (in the created worktree, or current directory with --no-workspace) |
| `--exec-mode` | `string` | Shell invocation mode for --exec: direct\|shell\|login-shell (default: login-shell) |
| `--from-branch`, `--branch` | `string` | Create worktree from branch (defaults to current branch) |
| `--from-issue` | `int` | Create worktree from issue number |
| `--from-issue-interactive`, `-I` | `bool` | Interactively select an issue to create worktree from |
| `--from-pr` | `int` | Create worktree from PR number |
| `--from-pr-interactive`, `-P` | `bool` | Interactively select a PR to create worktree from |
| `--generate` | `bool` | Generate name automatically from the current branch |
| `--json` | `bool` | Output result as JSON |
| `--no-workspace`, `-N` | `bool` | Create local branch and switch to it without creating a worktree (requires --from-pr, --from-pr-interactive, --from-issue, or --from-issue-interactive) |
| `--note` | `string` | Set a note on the new worktree |
| `--note-file` | `string` | Read note from file (use '-' for stdin) |
| `--output-selection` | `string` | Write created worktree path to a file |
| `--query`, `-q` | `string` | Pre-filter interactive selection (pre-fills fzf search or filters numbered list); requires --from-pr-interactive or --from-issue-interactive |
| `--silent` | `bool` | Suppress progress messages |
| `--tags` | `string` | Comma-separated tags for the new worktree |
| `--with-change` | `bool` | Carry over uncommitted changes to the new worktree |

## `delete`

Delete a worktree

| Flag | Type | Usage |
| --- | --- | --- |
| `--json` | `bool` | Output result as JSON |
| `--no-branch` | `bool` | Skip branch deletion |
| `--silent` | `bool` | Suppress progress messages |

## `rename`

Rename a worktree

| Flag | Type | Usage |
| --- | --- | --- |
| `--json` | `bool` | Output result as JSON |
| `--silent` | `bool` | Suppress progress messages |

## `exec`

Run a command or trigger a key action in a worktree

| Flag | Type | Usage |
| --- | --- | --- |
| `--json` | `bool` | Output result as JSON; command stdout/stderr is redirected to stderr |
| `--key`, `-k` | `string` | Custom command key to trigger (e.g. 't' for tmux) |
| `--workspace`, `-w` | `string` | Target worktree name or path |

## `note`

Show or edit worktree notes

| Flag | Type | Usage |
| --- | --- | --- |
| `--input`, `-i` | `string` | (edit) Read note from file (use '-' for stdin) |
| `--json` | `bool` | (show) Output note as JSON including metadata |

<!-- END GENERATED:cli-commands -->
