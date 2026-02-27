---
name: worktree-manage
description: Create, list, switch to, and delete git worktrees using lazyworktree CLI
---

# Worktree Management

Manage git worktrees for the current repository using the `lazyworktree` CLI.

## Current Worktrees
!`lazyworktree list --json`

## Current Working Directory
!`pwd`

## Current Branch
!`git branch --show-current`

---

## Available Operations

### Create a Worktree

**From the current branch (new feature work):**
```bash
# Auto-generated name (branch-adjective-noun pattern)
lazyworktree create --silent

# With a specific name
lazyworktree create --silent my-feature-name

# Carry over uncommitted changes to the new worktree
lazyworktree create --silent --with-change my-feature-name

# From a specific branch
lazyworktree create --silent --from-branch main my-feature-name
```

**From a GitHub/GitLab PR:**
```bash
lazyworktree create --silent --from-pr 42
```

**From a GitHub/GitLab issue:**
```bash
lazyworktree create --silent --from-issue 123
# Optionally specify a base branch
lazyworktree create --silent --from-issue 123 --from-branch main
```

**Run a command after creation:**
```bash
lazyworktree create --silent --from-branch main my-feature --exec "npm install"
```

All create commands print the created worktree path to stdout on success.

### List Worktrees

```bash
# JSON output (for parsing)
lazyworktree list --json

# Paths only (one per line)
lazyworktree list --pristine

# Human-readable table
lazyworktree list
```

JSON fields: `path`, `name`, `branch`, `is_main`, `dirty`, `ahead`, `behind`, `unpushed`, `last_active`.

### Run Commands in a Worktree

Use `exec` to run commands in a specific worktree without changing directory:

```bash
# By worktree name
lazyworktree exec -w my-feature-name "make build"

# Run any shell command
lazyworktree exec -w my-feature-name "git status"
lazyworktree exec -w my-feature-name "go test ./..."
```

### Delete a Worktree

```bash
# Delete worktree and its branch (if names match)
lazyworktree delete --silent my-feature-name

# Delete worktree but keep the branch
lazyworktree delete --silent --no-branch my-feature-name
```

---

## Workflow Instructions

When the user asks to **create a worktree**:
1. Determine the source: current branch, a specific branch, a PR number, or an issue number
2. Run the appropriate `lazyworktree create --silent` command
3. Capture the output path
4. Report the created worktree path to the user
5. If the user wants to work in it immediately, use `lazyworktree exec -w <name>` for subsequent commands

When the user asks to **switch to a worktree**:
1. Run `lazyworktree list --json` to find available worktrees
2. Identify the target by name, branch, or path
3. For running commands in the target worktree, use: `lazyworktree exec -w <name> "<command>"`
4. Tell the user they can switch their shell with: `cd <path>`

When the user asks to **list worktrees**:
1. Run `lazyworktree list --json` and present the results
2. Highlight dirty worktrees, ahead/behind status, and last active times

When the user asks to **delete a worktree**:
1. Run `lazyworktree list --json` to show current worktrees
2. Confirm which worktree to delete
3. Run `lazyworktree delete --silent <name>`

## Important Notes

- The `--silent` flag suppresses progress messages to stderr; stdout still contains the result path
- Worktree names are automatically sanitised (special characters replaced with hyphens)
- The main worktree cannot be deleted
- Use `exec -w` to run commands in a worktree without changing your shell's working directory
- If the user says "switch to" or "work in" a worktree, prefer using `exec -w` for running commands there and inform the user of the path for manual `cd`
