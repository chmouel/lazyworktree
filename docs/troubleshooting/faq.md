# FAQ

<div class="mint-callout">
  <p><strong>Use this page when:</strong> you have a quick question about LazyWorktree's behaviour or capabilities.</p>
</div>

## Do I need to use shell integration?

No. Shell helpers are entirely optional. LazyWorktree works as a standalone TUI without any shell integration.

Shell integration makes `cd`-to-selected-worktree smoother by exporting the selected path back to your calling shell. Without it, you can still navigate worktrees, but switching your shell's working directory requires a manual `cd` after exiting. See [Shell Integration](../shell-integration.md) for setup instructions.

## Can I keep one notes file across repositories?

Yes. Set `worktree_notes_path` to store all worktree notes in a single JSON file:

```yaml
worktree_notes_path: ".lazyworktree/notes.json"
```

When this is set, note keys are stored relative to `worktree_dir` rather than as absolute paths, making the file portable across machines. Commit the JSON file to share notes with your team.

## Where are notes stored by default?

Without `worktree_notes_path`, notes are stored in the repository's local git config (`git config --local`). This means they are specific to each clone and are not shared or synchronised automatically.

## Why is my PR branch name different after create-from-PR?

When creating a worktree from a PR, LazyWorktree uses the configured `pr_branch_name_template` to generate the branch name. If `branch_name_script` is configured and the template includes `{generated}`, the AI-generated name may differ from the original PR branch. If the script fails or times out, `{generated}` falls back to `{title}`.

To preserve the original PR branch name exactly, use a template without `{generated}`:

```yaml
pr_branch_name_template: "pr-{number}-{title}"
```

## Why are `.wt` commands not running?

Check your `trust_mode` configuration:

- `tofu` (default): prompts on first use and when the `.wt` file changes. If you selected **Block**, remove the entry from `~/.local/share/lazyworktree/trusted.json` to be prompted again.
- `never`: blocks all `.wt` execution. Change to `tofu` or `always` to enable hooks.
- `always`: runs without prompting.

Also verify the `.wt` file is in the repository root, not a subdirectory. See the [Diagnostic Guide](diagnostic-guide.md) for further steps.

## Can I use custom command keys from CLI?

Yes. Use `lazyworktree exec` with the `--key` flag:

```bash
lazyworktree exec --key=e              # Run custom command bound to 'e'
lazyworktree exec --key=e --workspace  # Run in a specific workspace
```

This executes the custom command as if you had pressed the key in the TUI, but without launching the interface.

## Why does `new-tab` not work from CLI `exec`?

The `new-tab` command type is intentionally unsupported in CLI mode. It requires a running multiplexer session (tmux or zellij) to create a new tab, which is only available when LazyWorktree is running inside the TUI. Use `shell` or `command` types for CLI-compatible commands.

## How do I change the worktree sort order?

Set `sort_mode` in your configuration:

```yaml
sort_mode: switched   # Most recently switched-to first (default)
# sort_mode: active   # Worktrees with uncommitted changes first
# sort_mode: path     # Alphabetical by path
```

## How do I use a different diff viewer?

Configure the `pager` setting to use your preferred diff tool:

```yaml
pager: "delta"                    # delta with default settings
# pager: "diff-so-fancy | less"  # diff-so-fancy piped to less
```

For interactive pagers, you may also need:

```yaml
git_pager_interactive: true
```

See [Diff, Pager, and Editor](../configuration/diff-pager-and-editor.md) and [Integration Caveats](integration-caveats.md) for tested combinations.

## Can I disable PR/MR fetching entirely?

Yes. Set `disable_pr` to skip all GitHub/GitLab API calls:

```yaml
disable_pr: true
```

This is useful for repositories where you do not use PRs, or to avoid API rate limits on large organisations.

## Where should I start debugging visual issues?

Start with [Fonts and Rendering](fonts-and-rendering.md) to check your font and terminal colour support. If the issue is theme-related, try switching to a different built-in theme. For all other issues, follow the [Diagnostic Guide](diagnostic-guide.md).
