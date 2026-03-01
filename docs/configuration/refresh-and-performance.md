# Refresh and Performance

Balance freshness and responsiveness with these settings.

## Worktree List Refresh

- `auto_refresh`: enable background refresh
- `refresh_interval`: refresh cadence in seconds
- `sort_mode`: `switched`, `active`, or `path`

## CI Refresh

- `ci_auto_refresh`: periodic CI refresh for GitHub repositories

## Diff Rendering Limits

- `max_untracked_diffs`: cap untracked diff rendering count
- `max_diff_chars`: cap total diff characters
- set either to `0` to disable that limit

## Search and Input Behaviour

- `search_auto_select`: start filter-focused
- `fuzzy_finder_input`: fuzzy suggestions in input dialogues
- `palette_mru`: recent-first palette ordering
- `palette_mru_limit`: number of recent entries

## Next Steps

<div class="mint-card-grid">
  <a class="mint-card" href="diff-pager-and-editor.md">
    <strong>Diff/Pager/Editor</strong>
    <span>Optimise diff and log viewing toolchain.</span>
  </a>
  <a class="mint-card" href="../core/navigation-and-keybindings.md">
    <strong>Navigation and Keys</strong>
    <span>Combine performance tuning with efficient keyboard flow.</span>
  </a>
</div>
