![Go](https://img.shields.io/badge/go-1.25%2B-blue) ![Coverage](https://img.shields.io/badge/Coverage-59.0%25-yellow)

# LazyWorktree

<img align="right" width="180" height="180" alt="logo" src="./website/assets/logo.png" />

LazyWorktree is a terminal UI for managing Git worktrees with a keyboard-first
workflow.

Built with [BubbleTea](https://github.com/charmbracelet/bubbletea), it focuses
on fast iteration, clear state visibility, and tight Git tooling integration.

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
- CLI overview: <https://chmouel.github.io/lazyworktree/docs/cli/overview/>
- CLI flags reference: <https://chmouel.github.io/lazyworktree/docs/cli/flags/>
- CLI commands reference: <https://chmouel.github.io/lazyworktree/docs/cli/commands/>
- Troubleshooting: <https://chmouel.github.io/lazyworktree/docs/troubleshooting/common-problems/>
- Screenshots: <https://chmouel.github.io/lazyworktree/docs/screenshots/>

## Screenshot

![lazyworktree screenshot](./website/assets/screenshot-main.png)

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
