![Go](https://img.shields.io/badge/go-1.25%2B-blue) ![Coverage](https://img.shields.io/badge/Coverage-62.0%25-yellow)

# LazyWorktree

<img align="right" width="180" height="180" alt="logo" src="./website/assets/logo.png" />

LazyWorktree is a terminal UI for managing Git worktrees with a keyboard-first
workflow.

It offer a development workflow around Git worktrees, allowing you to do
various operations based on your current worktree you are working on.

Built with [BubbleTea](https://github.com/charmbracelet/bubbletea), it focuses
on fast iteration, clear state visibility, and tight Git tooling integration
including tmux/zellij sessions and OCI container execution (docker/podman).
Custom commands can be bound to keys or exposed only in the command palette by
prefixing the config key with `_`.

## Documentation

Primary documentation lives on the docs site:

- <https://chmouel.github.io/lazyworktree/docs/>

Useful entry points:

- Introduction: <https://chmouel.github.io/lazyworktree/docs/>
- Getting started: <https://chmouel.github.io/lazyworktree/docs/getting-started/>
- Core workflows: <https://chmouel.github.io/lazyworktree/docs/core/worktree-operations/>
- Navigation and keybindings: <https://chmouel.github.io/lazyworktree/docs/core/navigation-and-keybindings/>
- Configuration overview: <https://chmouel.github.io/lazyworktree/docs/configuration/overview/>
- Configuration reference: <https://chmouel.github.io/lazyworktree/docs/configuration/reference/>
- Full configuration example: [`config.example.yaml`](config.example.yaml)
- CLI overview: <https://chmouel.github.io/lazyworktree/docs/cli/overview/>
- CLI flags reference: <https://chmouel.github.io/lazyworktree/docs/cli/flags/>
- CLI commands reference: <https://chmouel.github.io/lazyworktree/docs/cli/commands/>
- Troubleshooting: <https://chmouel.github.io/lazyworktree/docs/troubleshooting/common-problems/>

## Screenshot

![lazyworktree screenshot](./website/assets/screenshot-main.png)

_[You can see more screenshots here](https://chmouel.github.io/lazyworktree/#screenshots)_

## Main Features

- Worktree management — Create worktrees from branches, PRs/MRs, or issues; delete, list, and switch between them
- CI & PR/MR status — See GitHub Actions and GitLab CI results, check PR/MR details, view logs
- Notes & taskboard — Write markdown notes per worktree or tasks to track what you're working on; set a short description to replace the directory name in the list
- Agent sessions pane — See open Claude and pi sessions attached to the selected worktree by default, with a toggle for historical idle sessions
- Command palette — Quick access to all actions and custom commands with `?`, including an explicit **Open commit screen** action; use `_` prefix for palette-only commands
- Tmux and Zellij support — Automatically open worktrees in new tmux windows/panes or zellij tabs
- Docker/Podman support — Run commands in Docker or Podman containers tied to the worktree
- Custom commands — Set up shell commands in config, bind them to keys, show them in the palette
- Shell helpers — `cd "$(lazyworktree)"` shortcut and shell completion for bash, zsh, and fish (making it easy to jump to worktrees from the terminal)
- Hooks — `.wt` files per worktree to automate setup and cleanup tasks
- Customize Worktree display — Show branch names, PR/MR status, CI status, descriptions, and more in the list; configure colors, tags and icons.

## Installation

### Homebrew (macOS)

```bash
brew tap chmouel/lazyworktree https://github.com/chmouel/lazyworktree
brew install lazyworktree --cask
```

### Arch Linux

```bash
yay -S lazyworktree-bin
```

### From source

```bash
go install github.com/chmouel/lazyworktree/cmd/lazyworktree@latest
```

## Quick Start

```bash
cd /path/to/your/repository
lazyworktree
```

## Shell Integration

To jump to the selected worktree from your shell:

```bash
cd "$(lazyworktree)"
```

For shell integration helpers, see:

- <https://github.com/chmouel/lazyworktree/blob/main/shell/README.md>

## Requirements

- Git 2.31+
- Forge CLI (`gh` or `glab`) for PR/MR status

Optional tools are documented here:

- <https://chmouel.github.io/lazyworktree/docs/getting-started/#requirements>

## Development

Build the binary:

```bash
make build
```

Run full checks:

```bash
make sanity
```

Preview docs locally:

```bash
brew install uv # if not already installed
make docs-serve
```

Build docs locally:

```bash
make docs-build
```

Synchronise generated docs references:

```bash
make docs-sync
```

Run docs synchronisation and strict docs checks:

```bash
make docs-check
```

## Licence

[Apache-2.0](./LICENSE)
