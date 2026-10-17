# setup-hooks

Install agent session hooks for Claude Code, the Codex CLI, and the Copilot
CLI.

## Synopsis

```bash
lazyworktree setup-hooks
lazyworktree setup-hooks --dry-run
```

## What it does

`setup-hooks` adds lifecycle hooks to the user-level configurations of
supported agents:

- Claude Code uses `~/.claude/settings.json` (`SessionStart`,
  `UserPromptSubmit`, `Stop`, `SessionEnd`, and question-dialog lifecycle
  events)
- Codex CLI uses `$CODEX_HOME/hooks.json` when `CODEX_HOME` is set; otherwise
  `~/.codex/hooks.json` (`SessionStart`, `UserPromptSubmit`, and `Stop`)
- Copilot CLI uses `$COPILOT_HOME/hooks/lazyworktree.json` when
  `COPILOT_HOME` is set, otherwise `~/.copilot/hooks/lazyworktree.json`
  (`SessionStart`, `UserPromptSubmit`, `Stop`, `SessionEnd`, and
  question-dialog lifecycle events)

Each hook calls the hidden `lazyworktree agent-event` shim. It writes a small
event file that the TUI reads on its next refresh. The file includes the
agent's process ID, so LazyWorktree can confirm liveness with a PID check
instead of scanning the process table. These events also make Codex CLI and
Copilot CLI sessions visible in the agents pane. Neither agent exposes a
stable transcript format, so LazyWorktree does not parse their sessions.

Claude Code and Copilot CLI also report when a question dialog opens and when
you answer it. The pane shows the agent as waiting for your input, then
returns to thinking. Codex CLI does not expose an equivalent question-dialog
hook.

Hook events are the default liveness source. The older process-table scan
with ps/lsof is deprecated and disabled by default. If you cannot install
hooks, re-enable it with `agent_sessions.process_scan: true` in the
[configuration](../configuration/reference.md).

## Safety

- Existing settings are preserved; only the lazyworktree hook entries are
  merged in.
- A timestamped backup (for example `settings.json.bak-20260712-203500`) is
  written before any modification.
- The command is idempotent: running it again reports that the hooks are
  already installed and changes nothing.
- The hook shim always exits successfully, so a broken spool can never
  disrupt an agent session.

## Agent-specific notes

- Codex CLI requires approval before newly installed hooks can run. Open a
  Codex session and run the `/hooks` command to trust the new entries.
- Copilot CLI loads hook files at startup. Restart any running `copilot`
  session after installing.

## Options

| Flag | Description |
| ---- | ----------- |
| `--dry-run` | Show the configuration changes without writing any files. |

## Examples

```bash
# Preview the changes first
lazyworktree setup-hooks --dry-run

# Install the hooks
lazyworktree setup-hooks
```
