# Configuration Reference

This page is generated from `internal/config/config.go`. Run `make docs-sync` after changing config parsing/defaults.

<!-- BEGIN GENERATED:config-reference -->
| Key | Type | Default | Description |
| --- | --- | --- | --- |
| `worktree_dir` | `string` | `none` | Root directory for managed worktrees. |
| `theme` | `string` | `auto-detect` | UI theme selection. |
| `icon_set` | `enum(nerd-font-v3\|text)` | `nerd-font-v3` | Icon rendering mode for terminal compatibility. |
| `layout` | `enum(default\|top)` | `default` | Pane layout strategy. |
| `sort_mode` | `enum(path\|active\|switched)` | `switched` | Primary sort behaviour in the worktree list. |
| `sort_by_active` | `bool (legacy)` | `none` | Compatibility key for older sort configuration. |
| `auto_refresh` | `bool` | `true` | Enable background refresh of repository state. |
| `refresh_interval` | `int` | `10` | Background refresh cadence in seconds. |
| `ci_auto_refresh` | `bool` | `false` | Enable periodic CI refresh for GitHub repositories. |
| `auto_fetch_prs` | `bool` | `false` | Automatically fetch PR/MR data. |
| `disable_pr` | `bool` | `false` | Disable PR/MR integration. |
| `search_auto_select` | `bool` | `false` | Focus filter and auto-select first match. |
| `fuzzy_finder_input` | `bool` | `false` | Enable fuzzy helper input in selection dialogues. |
| `max_name_length` | `int` | `95` | Maximum displayed worktree name length. |
| `max_untracked_diffs` | `int` | `10` | Limit number of untracked file diffs rendered. |
| `max_diff_chars` | `int` | `200000` | Maximum characters read from diff output. |
| `git_pager` | `string` | `delta` | Diff formatter/pager command. |
| `git_pager_args` | `[]string` | `auto-matched delta syntax theme` | Extra arguments passed to configured git pager. |
| `delta_path` | `string (legacy)` | `none` | Legacy alias for git_pager. |
| `delta_args` | `[]string (legacy)` | `none` | Legacy alias for git_pager_args. |
| `git_pager_interactive` | `bool` | `false` | Use interactive pager mode for terminal-native tools. |
| `git_pager_command_mode` | `bool` | `false` | Use command mode for pagers that run git themselves. |
| `pager` | `string` | `none` | Pager for command output views. |
| `ci_script_pager` | `string` | `none` | Dedicated pager for CI logs. |
| `editor` | `string` | `none` | Editor used in file open actions. |
| `merge_method` | `enum(rebase\|merge)` | `rebase` | Absorb strategy for integrating a worktree. |
| `trust_mode` | `enum(tofu\|never\|always)` | `tofu` | Trust policy for repository `.wt` commands. |
| `branch_name_script` | `string` | `none` | Script to generate branch naming suggestions. |
| `issue_branch_name_template` | `string` | `issue-{number}-{title}` | Template for issue-based branch naming. |
| `pr_branch_name_template` | `string` | `pr-{number}-{title}` | Template for PR-based branch naming. |
| `worktree_note_script` | `string` | `none` | Script to prefill worktree notes from issue/PR context. |
| `worktree_notes_path` | `string` | `none` | Optional shared JSON file path for notes storage. |
| `session_prefix` | `string` | `wt-` | Prefix for tmux/zellij session names. |
| `palette_mru` | `bool` | `true` | Enable MRU sorting in command palette. |
| `palette_mru_limit` | `int` | `5` | Maximum MRU items in command palette. |
| `init_commands` | `[]string` | `none` | Global commands run after worktree creation. |
| `terminate_commands` | `[]string` | `none` | Global commands run before worktree removal. |
| `custom_commands` | `map[string]object` | `built-in keys: t, Z` | Custom key bindings and command actions. |
| `custom_create_menus` | `[]object` | `none` | Custom create menu entries. |
| `custom_themes` | `map[string]object` | `none` | Custom theme definitions. |
| `debug_log` | `string` | `none` | Debug log file path. |
<!-- END GENERATED:config-reference -->

For examples and grouped explanations, see:

- [Configuration Overview](overview.md)
- [Display and Themes](display-and-themes.md)
- [Refresh and Performance](refresh-and-performance.md)
- [Diff, Pager, and Editor](diff-pager-and-editor.md)
- [Lifecycle Hooks](lifecycle-hooks.md)
- [Branch Naming](branch-naming.md)
- [Custom Themes](custom-themes.md)
