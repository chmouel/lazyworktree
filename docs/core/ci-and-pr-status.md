# CI and PR/MR Status

LazyWorktree surfaces pull/merge request information and CI state directly in the status pane.


## Requirements

- **GitHub**: requires the [`gh`](https://cli.github.com/) CLI, authenticated
- **GitLab**: requires the [`glab`](https://gitlab.com/gitlab-org/cli) CLI, authenticated

LazyWorktree auto-detects the forge from your repository remote.

## Status Indicators

For worktrees linked to PR/MR items:

| Indicator | Colour | Status |
| --- | --- | --- |
| `✓` | Green | Passed |
| `✗` | Red | Failed |
| `●` | Yellow | Pending |
| `○` | Grey | Skipped |
| `⊘` | Grey | Cancelled |

Status data is fetched lazily and cached briefly for responsiveness.

![CI log viewer](../assets/ci-runs.png)

## Navigation and Actions

| Key | Action |
| --- | --- |
| `v` | View CI checks (when Status pane is focused) |
| `j` / `k` | Navigate between CI checks |
| `Enter` | Open selected check URL in browser |
| `Ctrl+v` | View selected check logs in pager |
| `Ctrl+r` | Restart CI job (GitHub Actions only) |

### Viewing CI Logs

Press `Ctrl+v` on a selected check to open its logs in your configured pager. The pager command is set via the `ci_log_pager` configuration option, falling back to `diff_pager` or `$PAGER`.

### Restarting Jobs

Press `Ctrl+r` to restart the selected CI job. This is currently supported for GitHub Actions only.

## Auto-Refresh

CI status is fetched lazily and cached for 30 seconds. To enable periodic background refresh, set:

```yaml
ci_auto_refresh: true  # default: false
```

## Remote Selection

CI and PR status are queried against a selectable remote for GitHub lookups and PR checkout. By default (`ci_remote` unset) an `upstream` remote is preferred when present, otherwise `origin`. This suits fork workflows where pull requests live on the upstream repository rather than your fork.

This setting only changes the CI/PR target on GitHub; GitLab MR and CI queries continue to use `glab`'s own repository resolution. Worktree identity, notes, and caches continue to use the repository's own origin/default discovery.

To pin a specific remote, set a remote name:

```yaml
ci_remote: origin  # default: prefer upstream, then origin
```

## PR/MR Integration

When a worktree branch has an associated pull or merge request, the status pane displays:

- PR/MR title and number
- The author's username, with a small round avatar badge on Kitty-compatible terminals
- PR/MR state as a coloured badge (`Open`, `Merged`, or `Closed`)
- CI check results
- Divergence from upstream

![PR/MR information in the Info pane, with a round author avatar badge](../assets/pr-info-avatar.png)

*The Info pane showing a linked pull request, its author avatar, state badge, and CI checks.*

Rows without a linked PR/MR do not show a PR/MR state badge.
For the primary worktree, details for a linked merged or closed PR/MR, including the state badge, are hidden.

### Creating Worktrees from PRs/MRs

Press `c` and select the PR/MR creation mode. LazyWorktree fetches open PRs/MRs and lets you select one to check out as a new worktree.

Configure branch naming for PR-created worktrees:

```yaml
pr_branch_name_template: "pr-{number}-{title}"
```

| Placeholder | Description |
| --- | --- |
| `{number}` | PR/MR number |
| `{title}` | Original sanitised PR/MR title |
| `{pr_author}` | PR author username (PR templates only) |
| `{generated}` | AI-generated title (if `branch_name_script` configured) |

### Disabling PR/MR Integration

If you do not use PRs/MRs or prefer not to install `gh`/`glab`, you can disable the integration entirely in your configuration.

## Hyperlinks and Context

In terminals that support OSC-8 hyperlinks, PR/MR identifiers and CI check names in the status pane are clickable — opening them directly in your browser.

## CI Environment Variables

When running custom commands or lifecycle hooks, LazyWorktree exposes CI context:

| Variable | Description |
| --- | --- |
| `LW_CI_JOB_NAME` | CI job identifier |
| `LW_CI_JOB_NAME_CLEAN` | Sanitised CI job name (safe for filenames) |
