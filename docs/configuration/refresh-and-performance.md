# Refresh and Performance

Balance freshness and responsiveness with these settings.

<div class="mint-callout">
  <p><strong>Use this page when:</strong> you want to tune update frequency, diff limits, or input behaviour.</p>
</div>

## Worktree List Refresh

Control how and when the worktree list updates.

```yaml
auto_refresh: true        # Enable background refresh (default: true)
refresh_interval: 5       # Seconds between refreshes (default: 5)
sort_mode: switched       # Sort order: switched, active, or path
```

### Behaviour

- When `auto_refresh` is enabled, LazyWorktree re-scans the worktree list at the configured interval
- When `auto_refresh` is disabled, the list only updates on manual refresh or after worktree operations
- `refresh_interval` sets the cadence in seconds — lower values give fresher data, higher values reduce git process spawning

### Guidance for large repositories

Repositories with many worktrees (20+) or large working trees may benefit from a longer refresh interval:

```yaml
refresh_interval: 15
```

This reduces the frequency of `git worktree list` and status checks, which can be slow on repositories with extensive untracked or modified files.

### Sort modes

| Mode | Description |
| --- | --- |
| `switched` | Most recently switched-to worktree first (default) |
| `active` | Worktrees with uncommitted changes first |
| `path` | Alphabetical by worktree path |

## CI Refresh

```yaml
ci_auto_refresh: true     # Periodic CI status refresh (default: true)
```

When enabled, LazyWorktree polls the GitHub or GitLab API for CI check status at regular intervals. Only worktrees with pending or running CI jobs are refreshed — completed checks are not re-fetched until the cache expires.

!!! tip
    If you hit API rate limits (especially on large GitHub organisations), disable CI auto-refresh and use manual refresh instead:
    ```yaml
    ci_auto_refresh: false
    ```

## Diff Rendering Limits

Cap the amount of diff content rendered in the details pane to keep the UI responsive.

```yaml
max_untracked_diffs: 10   # Maximum number of untracked files to diff (default: 10)
max_diff_chars: 50000     # Maximum total diff characters to render (default: 50000)
```

### Behaviour

- `max_untracked_diffs` limits how many untracked files are included in the diff view. Untracked files in large build directories (`node_modules`, `target/`, `.build/`) can generate enormous diffs.
- `max_diff_chars` limits the total character count of the rendered diff. Large binary files or generated code can exceed this easily.
- Setting either value to `0` disables that limit entirely.

!!! warning
    Setting `max_diff_chars: 0` on a repository with large generated files may cause noticeable rendering delays. If diffs feel sluggish, try lowering this value first.

### Tuning for slow diffs

If the details pane takes noticeably long to update:

```yaml
max_untracked_diffs: 5
max_diff_chars: 20000
```

This reduces the volume of content the renderer processes without losing visibility of the most important changes.

## Search and Input Behaviour

```yaml
search_auto_select: false    # Start with filter focused (default: false)
fuzzy_finder_input: true     # Fuzzy suggestions in input dialogues (default: true)
palette_mru: true            # Most-recently-used ordering in command palette (default: true)
palette_mru_limit: 10        # Number of recent entries to track (default: 10)
```

### Settings explained

| Setting | Effect |
| --- | --- |
| `search_auto_select` | When `true`, the filter input is focused immediately upon opening the worktree list. Useful if you routinely search rather than scroll. |
| `fuzzy_finder_input` | When `true`, input dialogues (branch name, worktree path) offer fuzzy completion suggestions. Disable if suggestions interfere with manual entry. |
| `palette_mru` | When `true`, the command palette sorts entries by most recently used. When `false`, entries appear in definition order. |
| `palette_mru_limit` | Number of recent command palette entries to remember. Set to `0` to disable MRU tracking. |
