# CLI `cleanup`

Remove merged worktrees, stale branches, and orphaned worktree directories.

## Interactive cleanup

```bash
lazyworktree cleanup
```

The command displays a numbered list. Select one or more entries with:

- a single number, such as `2`
- comma-separated numbers, such as `1,4`
- a range, such as `2-5`
- a combination, such as `1,3-5`
- `all` to select every displayed candidate

Press Enter without a selection to cancel.

Dirty merged worktrees remain visible with a warning. Selecting one records it
as skipped rather than removing the worktree or its branch.

## Non-interactive cleanup

```bash
lazyworktree cleanup --all
lazyworktree cleanup --non-interactive # Alias for --all
```

`--all` attempts every candidate without reading from standard input. Registered
worktrees with uncommitted changes are reported and skipped, while clean
worktrees, stale branches, and orphaned directories remain eligible for
cleanup. Worktree status is rechecked immediately before removal, so changes
made after candidate discovery are protected as well.

## JSON output

```bash
lazyworktree cleanup --all --json
```

`--json` emits a single JSON object to standard output describing the result. It
requires `--all`, since the interactive prompt cannot coexist with machine
output. Progress messages, including terminate command notices, are suppressed.

The object reports aggregate counts alongside a per-item list. Each item records
its `kind` (`worktree`, `branch`, or `orphan`), the worktree `path`, its
`branch`, the detection `source` (`pr`, `git`, or `both`), whether the branch was
deleted, whether the item was skipped, and whether the removal failed. Skipped
worktrees include a `skip_reason`; safety skips do not cause a non-zero exit,
while actual removal failures do.

```json
{
  "worktrees": 1,
  "branches": 1,
  "orphans": 0,
  "skipped": 0,
  "failures": 0,
  "items": [
    {
      "kind": "worktree",
      "path": "/home/you/worktrees/repo/feature",
      "branch": "feature",
      "source": "pr",
      "branch_deleted": true,
      "skipped": false,
      "failed": false
    },
    {
      "kind": "branch",
      "branch": "stale",
      "source": "git",
      "branch_deleted": true,
      "skipped": false,
      "failed": false
    }
  ]
}
```

## Candidate detection

Cleanup considers:

- worktrees whose PR/MR is merged
- worktrees whose branch is merged into the main branch
- merged local branches without worktrees when `prune_stale_branches` is enabled
- non-hidden directories in the repository's worktree directory that Git no longer registers

Terminate commands run only after a worktree has passed the final clean-status
check. Orphaned directories are revalidated against Git immediately before
deletion. Any failed candidate removal causes a non-zero exit; skipped dirty
worktrees are reported without being treated as failures.
