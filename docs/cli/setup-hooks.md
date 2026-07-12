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

- **Claude Code** — `~/.claude/settings.json` (`SessionStart`,
  `UserPromptSubmit`, `Stop`, `SessionEnd`, and question-dialog lifecycle
  events)
- **Codex CLI** — `$CODEX_HOME/hooks.json` when `CODEX_HOME` is set, otherwise
  `~/.codex/hooks.json` (`SessionStart`, `UserPromptSubmit`, and `Stop`)
- **Copilot CLI** — `$COPILOT_HOME/hooks/lazyworktree.json` when
  `COPILOT_HOME` is set, otherwise `~/.copilot/hooks/lazyworktree.json`
  (`SessionStart`, `UserPromptSubmit`, `Stop`, `SessionEnd`, and
  question-dialog lifecycle events)

Each hook invokes the hidden `lazyworktree agent-event` shim, which records a
small event file that the TUI consumes on its next refresh. Hook events give
lazyworktree precise session state — including the agent process id — so it
can track liveness with a cheap PID probe instead of scanning the process
table, and it enables Codex CLI and Copilot CLI sessions to appear in the
agents pane at all (neither exposes a stable transcript format, so
lazyworktree does not parse them). For Claude Code and Copilot CLI, the
question-dialog events also distinguish an agent waiting for your answer from
one that is still thinking. Codex CLI does not currently expose an equivalent
question-dialog hook.

Hook events are the default liveness source: the former process-table scan
(ps/lsof) is deprecated and disabled by default. If you cannot install hooks,
re-enable it with `agent_sessions.process_scan: true` in the
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

- **Codex CLI** requires newly installed hooks to be approved before they
  run. After installing, open a Codex session and run the `/hooks` command to
  trust the new entries.
- **Copilot CLI** loads hook files at startup. Restart any running `copilot`
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
