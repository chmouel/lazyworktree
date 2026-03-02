# Shell Integration

Shell helpers change directory to the selected worktree on exit. Optional but recommended.

## Quick Usage

Without helper functions, use the simple command form in any POSIX-like shell:

```bash
cd "$(lazyworktree)"
```

With helper functions loaded, you can wrap this in a reusable shell command and preserve consistent behaviour across repositories.

## Shell Setup

=== "Bash"

    **Option A:** Source the helper from a local clone:

    ```bash
    # Add to .bashrc
    source /path/to/lazyworktree/shell/functions.bash

    jt() { worktree_jump $(git rev-parse --show-toplevel) "$@"; }
    ```

    **Option B:** Download the helper:

    ```bash
    mkdir -p ~/.shell/functions
    curl -sL https://raw.githubusercontent.com/chmouel/lazyworktree/refs/heads/main/shell/functions.bash -o ~/.shell/functions/lazyworktree.bash

    # Add to .bashrc
    source ~/.shell/functions/lazyworktree.bash

    jt() { worktree_jump $(git rev-parse --show-toplevel) "$@"; }
    ```

    **With completion:**

    ```bash
    source /path/to/lazyworktree/shell/functions.bash

    jt() { worktree_jump $(git rev-parse --show-toplevel) "$@"; }
    _jt() { _worktree_jump $(git rev-parse --show-toplevel); }
    complete -o nospace -F _jt jt
    ```

    **Jump to last-selected worktree:**

    ```bash
    alias pl='worktree_go_last $(git rev-parse --show-toplevel)'
    ```

=== "Zsh"

    **Option A:** Source the helper from a local clone:

    ```zsh
    # Add to .zshrc
    source /path/to/lazyworktree/shell/functions.zsh

    jt() { worktree_jump $(git rev-parse --show-toplevel) "$@"; }
    ```

    **Option B:** Download the helper:

    ```zsh
    mkdir -p ~/.shell/functions
    curl -sL https://raw.githubusercontent.com/chmouel/lazyworktree/refs/heads/main/shell/functions.zsh -o ~/.shell/functions/lazyworktree.zsh

    # Add to .zshrc
    source ~/.shell/functions/lazyworktree.zsh

    jt() { worktree_jump $(git rev-parse --show-toplevel) "$@"; }
    ```

    **With completion:**

    ```zsh
    source /path/to/lazyworktree/shell/functions.zsh

    jt() { worktree_jump $(git rev-parse --show-toplevel) "$@"; }
    _jt() { _worktree_jump $(git rev-parse --show-toplevel); }
    compdef _jt jt
    ```

    **Jump to last-selected worktree:**

    ```zsh
    alias pl='worktree_go_last $(git rev-parse --show-toplevel)'
    ```

=== "Fish"

    **Option A:** Source the helper from a local clone:

    ```fish
    # Add to ~/.config/fish/config.fish
    source /path/to/lazyworktree/shell/functions.fish

    function jt
        worktree_jump $(git rev-parse --show-toplevel) $argv
    end
    ```

    **Option B:** Download the helper:

    ```fish
    mkdir -p ~/.config/fish/conf.d
    curl -sL https://raw.githubusercontent.com/chmouel/lazyworktree/refs/heads/main/shell/functions.fish -o ~/.config/fish/conf.d/lazyworktree.fish

    # Add to ~/.config/fish/config.fish
    function jt
        worktree_jump $(git rev-parse --show-toplevel) $argv
    end
    ```

    **With completion:**

    ```fish
    source /path/to/lazyworktree/shell/functions.fish

    function jt
        worktree_jump $(git rev-parse --show-toplevel) $argv
    end

    complete -c jt -f -a '(_worktree_jump $(git rev-parse --show-toplevel))'
    ```

    **Jump to last-selected worktree:**

    ```fish
    function pl
        worktree_go_last $(git rev-parse --show-toplevel)
    end
    ```

## Shell Completion

Generate completion scripts for the `lazyworktree` command itself:

```bash
# Bash
eval "$(lazyworktree completion bash --code)"

# Zsh
eval "$(lazyworktree completion zsh --code)"

# Fish
lazyworktree completion fish --code > ~/.config/fish/completions/lazyworktree.fish
```

Or simply run `lazyworktree completion` to see instructions for your shell.

Package manager installations (deb, rpm, AUR) include completions automatically.

## Troubleshooting

- If `cd "$(lazyworktree)"` does not change directory, confirm `lazyworktree` is in your `PATH`.
- If output is empty, ensure a worktree is selected before quitting the TUI.
- If shell profile changes do not load, restart your terminal session.
