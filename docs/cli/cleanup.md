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

## Non-interactive cleanup

```bash
lazyworktree cleanup --all
lazyworktree cleanup --non-interactive # Alias for --all
```

`--all` removes every candidate without reading from standard input. This
includes dirty merged worktrees and orphaned directories, so inspect with the
interactive command first if the repository state is uncertain.

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
deleted, and whether the removal failed.

```json
{
  "worktrees": 1,
  "branches": 1,
  "orphans": 0,
  "failures": 0,
  "items": [
    {
      "kind": "worktree",
      "path": "/home/you/worktrees/repo/feature",
      "branch": "feature",
      "source": "pr",
      "branch_deleted": true,
      "failed": false
    },
    {
      "kind": "branch",
      "branch": "stale",
      "source": "git",
      "branch_deleted": true,
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

Terminate commands run before a worktree is removed. Orphaned directories are
revalidated against Git immediately before deletion. Any failed candidate
removal causes a non-zero exit.
