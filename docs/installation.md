# Installation

<div class="lw-callout">
  <p><strong>Recommended:</strong> use Homebrew on macOS or AUR on Arch Linux. Use source installation if you prefer building directly from Go modules.</p>
</div>

## Choose Your Installation Path

=== "Homebrew (macOS)"

    ```bash
    brew tap chmouel/lazyworktree https://github.com/chmouel/lazyworktree
    brew install lazyworktree --cask
    ```

    If macOS Gatekeeper shows "Apple could not verify lazyworktree":

    1. System Settings -> Privacy & Security -> Open Anyway, or
    2. Remove quarantine attributes:

    ```bash
    xattr -d com.apple.quarantine /opt/homebrew/bin/lazyworktree
    ```

=== "Arch Linux"

    ```bash
    yay -S lazyworktree-bin
    ```

=== "From Source"

    Install directly:

    ```bash
    go install github.com/chmouel/lazyworktree/cmd/lazyworktree@latest
    ```

    Or clone and build locally:

    ```bash
    git clone https://github.com/chmouel/lazyworktree.git
    cd lazyworktree
    go build -o lazyworktree ./cmd/lazyworktree
    ```

## Pre-built Binaries

Release binaries are available at:

- [GitHub Releases](https://github.com/chmouel/lazyworktree/releases)

## Next Steps

<div class="mint-card-grid">
  <a class="mint-card" href="getting-started.md">
    <strong>Quickstart</strong>
    <span>Launch lazyworktree and start navigating worktrees.</span>
  </a>
  <a class="mint-card" href="core/worktree-operations.md">
    <strong>Worktree Operations</strong>
    <span>Learn creation, rename, deletion, and absorb workflows.</span>
  </a>
</div>
