# Shell integration

Lazyworktree provides shell integration helpers to enhance your workflow when working with Git worktrees. The "jump" helper changes your current directory to the selected worktree on exit, using `--output-selection` to write the selected path to a temporary file.

Shell integration scripts are available for Bash, Zsh, and Fish.

## Bash

**Option A:** Source the helper from a local clone:

```bash
# Add to .bashrc
source /path/to/lazyworktree/shell/functions.bash

jt() { worktree_jump ~/path/to/your/main/repo "$@"; }
```

**Option B:** Download the helper:

```bash
mkdir -p ~/.shell/functions
curl -sL https://raw.githubusercontent.com/chmouel/lazyworktree/refs/heads/main/shell/functions.bash -o ~/.shell/functions/lazyworktree.bash

# Add to .bashrc
source ~/.shell/functions/lazyworktree.bash

jt() { worktree_jump ~/path/to/your/main/repo "$@"; }
```

**With completion:**

```bash
source /path/to/lazyworktree/shell/functions.bash

jt() { worktree_jump ~/path/to/your/main/repo "$@"; }
_jt() { _worktree_jump ~/path/to/your/main/repo; }
complete -o nospace -F _jt jt
```

To add a shortcut to the last-selected worktree:

```bash
alias pl='worktree_go_last ~/path/to/your/main/repo'
```

## Zsh

**Option A:** Source the helper from a local clone:

```bash
# Add to .zshrc
source /path/to/lazyworktree/shell/functions.zsh

jt() { worktree_jump ~/path/to/your/main/repo "$@"; }
```

**Option B:** Download the helper:

```bash
mkdir -p ~/.shell/functions
curl -sL https://raw.githubusercontent.com/chmouel/lazyworktree/refs/heads/main/shell/functions.zsh -o ~/.shell/functions/lazyworktree.zsh

# Add to .zshrc
source ~/.shell/functions/lazyworktree.zsh

jt() { worktree_jump ~/path/to/your/main/repo "$@"; }
```

**With completion:**

```bash
source /path/to/lazyworktree/shell/functions.zsh

jt() { worktree_jump ~/path/to/your/main/repo "$@"; }
_jt() { _worktree_jump ~/path/to/your/main/repo; }
compdef _jt jt
```

To add a shortcut to the last-selected worktree:

```bash
alias pl='worktree_go_last ~/path/to/your/main/repo'
```

## Fish

**Option A:** Source the helper from a local clone:

```fish
# Add to ~/.config/fish/config.fish
source /path/to/lazyworktree/shell/functions.fish

function jt
    worktree_jump ~/path/to/your/main/repo $argv
end
```

**Option B:** Download the helper:

```fish
mkdir -p ~/.config/fish/conf.d
curl -sL https://raw.githubusercontent.com/chmouel/lazyworktree/refs/heads/main/shell/functions.fish -o ~/.config/fish/conf.d/lazyworktree.fish

# Add to ~/.config/fish/config.fish
function jt
    worktree_jump ~/path/to/your/main/repo $argv
end
```

**With completion:**

```fish
source /path/to/lazyworktree/shell/functions.fish

function jt
    worktree_jump ~/path/to/your/main/repo $argv
end

complete -c jt -f -a '(_worktree_jump ~/path/to/your/main/repo)'
```

To add a shortcut to the last-selected worktree:

```fish
function pl
    worktree_go_last ~/path/to/your/main/repo
end
```

## Shell Completion

Generate completion scripts for bash, zsh, or fish:

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
