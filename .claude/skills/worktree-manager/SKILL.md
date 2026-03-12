---
name: worktree-manager
description: Create, list, switch to, rename, delete, and manage notes for git worktrees using lazyworktree CLI
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

# Create a local branch from a PR without creating a worktree
lazyworktree create --silent --from-pr 42 --no-workspace
```

**From a GitHub/GitLab issue:**

```bash
lazyworktree create --silent --from-issue 123
# Optionally specify a base branch
lazyworktree create --silent --from-issue 123 --from-branch main

# Create a local branch from an issue without creating a worktree
lazyworktree create --silent --from-issue 123 --no-workspace
```

**Auto-generate worktree name from branch:**

```bash
lazyworktree create --silent --generate
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

# Show only the main branch worktree
lazyworktree list --main

# Human-readable table
lazyworktree list
```

Alias: `lazyworktree ls` is equivalent to `lazyworktree list`.

JSON fields: `path`, `name`, `branch`, `is_main`, `dirty`, `ahead`, `behind`, `unpushed`, `last_active`.

### Run Commands in a Worktree

Use `exec` to run commands in a specific worktree without changing directory:

```bash
# By worktree name or path
lazyworktree exec -w my-feature-name "make build"
lazyworktree exec -w /path/to/worktree "make build"

# Auto-detect worktree from current directory (no -w needed)
cd /path/to/my-feature && lazyworktree exec "git status"

# Run any shell command
lazyworktree exec -w my-feature-name "go test ./..."

# Trigger a custom command key instead of a shell command
lazyworktree exec -w my-feature-name --key t
```

Note: `--key` and a command argument are mutually exclusive.

### Rename a Worktree

```bash
# Rename current worktree (auto-detected from cwd)
lazyworktree rename --silent new-name

# Rename a specific worktree
lazyworktree rename --silent my-feature-name new-name
```

### Worktree Notes

```bash
# Show the note for the current worktree (auto-detected from cwd)
lazyworktree note show

# Show the note for a specific worktree
lazyworktree note show my-feature-name

# Edit a worktree note (opens editor)
lazyworktree note edit my-feature-name

# Set a note from a file (use '-' for stdin)
lazyworktree note edit my-feature-name --input notes.md
echo "my note" | lazyworktree note edit my-feature-name --input -
```

### Delete a Worktree

```bash
# Delete worktree and its branch (if names match)
lazyworktree delete --silent my-feature-name

# Delete worktree but keep the branch
lazyworktree delete --silent --no-branch my-feature-name

# Auto-detect worktree from cwd (positional arg is optional)
cd /path/to/worktree && lazyworktree delete --silent
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

When the user asks to **rename a worktree**:

1. Run `lazyworktree list --json` to identify the worktree
2. Run `lazyworktree rename --silent <old-name> <new-name>`
3. Report the new name to the user

When the user asks to **view or edit worktree notes**:

1. To view: `lazyworktree note show <name>` (omit name to use cwd)
2. To set from a string: `echo "content" | lazyworktree note edit <name> --input -`
3. To set from a file: `lazyworktree note edit <name> --input path/to/file`

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
