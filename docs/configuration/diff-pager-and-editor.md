# Diff, Pager, and Editor

Configure output viewers for diffs, logs, and file edits.

## Diff Formatter

- `git_pager`: formatter command (default `delta`)
- empty `git_pager`: disable formatter
- `git_pager_args`: formatter arguments

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

## Next Steps

<div class="mint-card-grid">
  <a class="mint-card" href="../core/ci-and-pr-status.md">
    <strong>CI and PR/MR Status</strong>
    <span>Use pager settings for better CI log readability.</span>
  </a>
  <a class="mint-card" href="../troubleshooting/integration-caveats.md">
    <strong>Integration Caveats</strong>
    <span>Handle edge cases with pagers, shells, and terminals.</span>
  </a>
</div>
