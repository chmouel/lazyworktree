# CI Status Display

Shows CI check statuses for worktrees with associated PR/MR:

* `✓` Green - Passed | `✗` Red - Failed | `●` Yellow - Pending | `○` Grey - Skipped | `⊘` Grey - Cancelled

Status is fetched lazily and cached for 30 seconds. Press `p` to refresh.
In terminals that support OSC-8 hyperlinks, the PR/MR number in the Status info panel is clickable.

