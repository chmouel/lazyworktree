# CLI Flags Reference

This page is generated from `internal/bootstrap/flags.go` and `internal/bootstrap/commands.go`.
Run `make docs-sync` after changing flag definitions.

## Global Flags

<!-- BEGIN GENERATED:global-flags -->
| Flag | Type | Usage |
| --- | --- | --- |
| `--config`, `-C` | `stringslice` | Override config values (repeatable): --config=lw.key=value |
| `--config-file` | `string` | Path to configuration file |
| `--debug-log` | `string` | Path to debug log file |
| `--output-selection` | `string` | Write selected worktree path to a file |
| `--search-auto-select` | `bool` | Start with filter focused |
| `--show-syntax-themes` | `bool` | List available delta syntax themes |
| `--theme`, `-t` | `string` | Override the UI theme |
| `--worktree-dir`, `-w` | `string` | Override the default worktree root directory |
<!-- END GENERATED:global-flags -->

## Command Flags

<!-- BEGIN GENERATED:command-flags -->
### `list`

| Flag | Type | Usage |
| --- | --- | --- |
| `--json` | `bool` | Output as JSON |
| `--main`, `-m` | `bool` | Show only the main branch worktree |
| `--no-agent` | `bool` | Skip agent session data in JSON output (faster for scripting) |
| `--pristine`, `-p` | `bool` | Output paths only (one per line, suitable for scripting) |

### `create`

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

### `delete`

| Flag | Type | Usage |
| --- | --- | --- |
| `--json` | `bool` | Output result as JSON |
| `--no-branch` | `bool` | Skip branch deletion |
| `--silent` | `bool` | Suppress progress messages |

### `rename`

| Flag | Type | Usage |
| --- | --- | --- |
| `--json` | `bool` | Output result as JSON |
| `--silent` | `bool` | Suppress progress messages |

### `exec`

| Flag | Type | Usage |
| --- | --- | --- |
| `--json` | `bool` | Output result as JSON; command stdout/stderr is redirected to stderr |
| `--key`, `-k` | `string` | Custom command key to trigger (e.g. 't' for tmux) |
| `--workspace`, `-w` | `string` | Target worktree name or path |

### `note`

| Flag | Type | Usage |
| --- | --- | --- |
| `--input`, `-i` | `string` | (edit) Read note from file (use '-' for stdin) |
| `--json` | `bool` | (show) Output note as JSON including metadata |

<!-- END GENERATED:command-flags -->

## Validation Rules

These runtime rules are enforced in `internal/bootstrap/commands.go`:

- `create`: `--from-pr`, `--from-issue`, `--from-pr-interactive`, and `--from-issue-interactive` are mutually exclusive.
- `create`: `--query` requires `--from-pr-interactive` or `--from-issue-interactive`.
- `create`: `--no-workspace` requires PR/issue creation mode and cannot be combined with `--with-change` or `--generate`.
- `list`: `--pristine` and `--json` are mutually exclusive.
- `exec`: use either positional command or `--key`, never both.
