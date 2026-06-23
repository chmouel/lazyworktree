# CI Status Display

Shows CI check statuses for worktrees with associated PR/MR:

* `✓` Green - Passed | `✗` Red - Failed | `●` Yellow - Pending | `○` Grey - Skipped | `⊘` Grey - Cancelled

Status is fetched lazily and cached for 30 seconds. Press `r` to refresh.
In terminals that support OSC-8 hyperlinks, the PR/MR number in the Status info panel is clickable.
When a worktree has an associated PR/MR, the Info pane shows a coloured state badge for `Open`, `Merged`, or `Closed`. For the primary worktree, details for a linked merged or closed PR/MR, including the state badge, are hidden.
