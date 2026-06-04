# Diff, Pager, and Editor

Configure output viewers for diffs, logs, and file edits.

## Diff Formatter

- `git_pager`: formatter command (default `delta`)
- empty `git_pager`: disable formatter
- `git_pager_args`: formatter arguments
- omitted `git_pager_args` with `delta`: auto-select syntax theme for the final UI theme, including CLI theme overrides

Optional compatibility flags:

- `git_pager_interactive`: for interactive tools (for example `tig`)
- `git_pager_command_mode`: for tools that invoke git commands directly

## Pager

- `pager`: output pager command
- defaults to `$PAGER`, fallback to `less`

## CI Log Pager

- `ci_script_pager`: custom pager script for CI logs
- falls back to `pager` when unset

CI script environment variables:

- `LW_CI_JOB_NAME`
- `LW_CI_JOB_NAME_CLEAN`
- `LW_CI_RUN_ID`
- `LW_CI_STARTED_AT`

## Editor

- `editor`: command used for status-pane edit actions
- defaults to `$EDITOR`, fallback to `nvim`

## Commit Message Generation

- `commit.auto_generate_command`: command run by `Ctrl+O` in the commit screen
- receives the staged diff on stdin
- first output line becomes the subject
- third line onwards become the body
