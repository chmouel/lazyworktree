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

- Getting started: <https://chmouel.github.io/lazyworktree/docs/getting-started/>
- Installation: <https://chmouel.github.io/lazyworktree/docs/installation/>
- Keybindings: <https://chmouel.github.io/lazyworktree/docs/keybindings/>
- Configuration: <https://chmouel.github.io/lazyworktree/docs/configuration/>
- Custom commands: <https://chmouel.github.io/lazyworktree/docs/custom-commands/>
- CLI usage: <https://chmouel.github.io/lazyworktree/docs/cli/>
- Themes: <https://chmouel.github.io/lazyworktree/docs/themes/>
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


```bash
make sanity
```

Preview docs locally:

```bash
make docs-serve
```

Build docs locally:

```bash
make docs-build
```

## Licence

[Apache-2.0](./LICENSE)
