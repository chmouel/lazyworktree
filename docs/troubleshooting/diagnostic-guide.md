# Diagnostic Guide

A structured approach to diagnosing problems with LazyWorktree, from enabling debug output through to filing a useful bug report.

<div class="mint-callout">
  <p><strong>Use this page when:</strong> something is not working as expected and you want a systematic way to identify the cause.</p>
</div>

## Step 1: Enable Debug Logging

Before investigating further, capture a debug log. This records internal decisions, git commands executed, API calls, and timing information.

### Via CLI flag

```bash
lazyworktree --debug-log /tmp/lw-debug.log
```

### Via configuration

```yaml
debug_log: /tmp/lw-debug.log
```

### What to look for

- **Error lines**: search for `ERROR` or `error` — these indicate failed operations
- **Timeout lines**: search for `timeout` — suggests a slow script or network issue
- **Git command output**: the log records every git command and its exit code
- **API responses**: GitHub/GitLab API calls and their HTTP status codes appear here

## Step 2: Identify Symptom Category

Use the decision tree below to find the relevant section or page:

**Visual or rendering problems** (garbled icons, wrong colours, layout issues)

:   Start with [Fonts and Rendering](fonts-and-rendering.md). If colours are the issue, check your terminal's true-colour support and theme settings.

**CI or PR/MR status not appearing**

:   Check authentication first — GitHub requires a `GITHUB_TOKEN` or `gh` CLI authentication, GitLab requires `GITLAB_TOKEN`. Then verify `disable_pr` is not set to `true` in your configuration. Review the debug log for API errors.

**Custom commands not running**

:   Verify the key binding does not conflict with a built-in binding (see [Keybindings](../keybindings.md)). Check `trust_mode` if the command is defined in a `.wt` file. The debug log records command dispatch attempts.

**Performance issues** (slow refresh, laggy UI)

:   See [Refresh and Performance](../configuration/refresh-and-performance.md) to tune refresh intervals and diff limits. Large repositories with many worktrees or untracked files benefit from higher `refresh_interval` values and lower `max_untracked_diffs`.

**Lifecycle hooks (`.wt`) not running**

:   Check `trust_mode` in your configuration. In `tofu` mode (the default), you must accept the trust prompt on first use. If the `.wt` file has been modified since you last trusted it, the hash will no longer match and you will be prompted again. Inspect `~/.local/share/lazyworktree/trusted.json` to see which files are currently trusted.

## Step 3: Common Error Patterns

### LazyWorktree opens but looks broken

**Symptoms**: garbled characters, missing icons, boxes where glyphs should be.

**Diagnosis**:

1. Test whether your terminal supports the required glyphs:
    ```bash
    echo -e "\uf126 \uf09b \ue725"
    ```
    If these render as boxes or question marks, your font lacks Nerd Font glyphs.

2. Switch to plain-text icons as an immediate workaround:
    ```yaml
    icon_set: text
    ```

3. For full icon support, install a [Nerd Font](https://www.nerdfonts.com/) (v3 or later), set it as your terminal's font, and restart the terminal session.

See [Fonts and Rendering](fonts-and-rendering.md) for further detail.

### CI logs do not display as expected

**Symptoms**: empty CI pane, stale status, or formatting errors.

**Diagnosis**:

1. Confirm authentication is configured (`GITHUB_TOKEN`, `gh auth status`, or `GITLAB_TOKEN`)
2. Check `pager` and `ci_script_pager` settings — an incorrectly configured pager can swallow output
3. Verify the pager command is available in your shell:
    ```bash
    which delta  # or your configured pager
    ```
4. Test the pager independently:
    ```bash
    echo "test" | delta
    ```
5. Review the debug log for API errors or HTTP status codes

### Custom commands behave differently in CLI and TUI

**Symptoms**: a command works in the TUI but fails with `lazyworktree exec`, or vice versa.

**Diagnosis**:

- The TUI and CLI use different shell execution modes. `zsh` uses `-ilc` (interactive + login), `bash` uses `-ic`, and other shells use `-lc`. This means shell functions, aliases, and startup scripts may behave differently.
- `new-tab` commands are intentionally unsupported in CLI `exec` mode — they require a multiplexer session.
- Confirm the command type and key binding match your intended execution context.

### `.wt` hooks are not running

**Symptoms**: init or terminate commands in your `.wt` file are silently skipped.

**Diagnosis**:

1. Check `trust_mode` in your configuration:
    - `tofu` (default): prompts on first use and when the file changes
    - `never`: blocks all `.wt` execution — hooks will never run
    - `always`: runs without prompting
2. If you previously selected **Block** for a repository's `.wt` file, the decision is stored in `~/.local/share/lazyworktree/trusted.json`. Remove the entry to be prompted again.
3. Verify the `.wt` file is in the repository root (not in a subdirectory).
4. Check the debug log for trust evaluation messages.

## Step 4: Filing a Bug Report

If the diagnostic steps above do not resolve your issue, [open an issue on GitHub](https://github.com/chmouel/lazyworktree/issues) with the following information:

- [ ] **Version**: output of `lazyworktree --version`
- [ ] **Terminal**: name, version, and font (e.g., "iTerm2 3.5.0, JetBrains Mono Nerd Font")
- [ ] **Operating system**: name and version
- [ ] **Debug log**: attach the output of `--debug-log` (redact any tokens or sensitive paths)
- [ ] **Reproduction steps**: minimal steps to trigger the issue, starting from a clean state
- [ ] **Expected vs actual behaviour**: what you expected and what happened instead
- [ ] **Configuration**: relevant sections of your `.lazyworktree.yaml` or global config (redact sensitive values)
