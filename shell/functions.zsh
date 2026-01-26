# lazyworktree shell functions
# Review and customize these functions as needed for your workflow
# Example function to jump between git worktrees using lazyworktree and GitHub
# CLI
_git_repo_slug() {
    local dir="$1"
    local url provider slug

    url=$(cd "$dir" 2>/dev/null && git remote get-url origin 2>/dev/null) || return 1

    case "$url" in
        *github.com* ) provider=github ;;
        *gitlab.com* ) provider=gitlab ;;
        * ) provider=unknown ;;
    esac

    slug=$(echo "$url" | sed -E 's#^.*[:/]([^/]+/[^/]+)(\.git)?$#\1#')

    [[ -n "$slug" ]] || return 1
    print "$provider:$slug"
}

# Example function to jump between git worktrees using lazyworktree
worktree_jump() {
    local dir="$1"; shift
    local id repo slug

    if [[ -z "$dir" || ! -d "$dir" ]]; then
        print -u2 "worktree_jump: invalid directory: $dir"
        return 1
    fi

    id=$(_git_repo_slug "$dir" 2>/dev/null)

    if [[ -n "$id" ]]; then
        slug=${id#*:}
        repo="$slug"
    else
        repo=$(basename "$dir")
    fi

    local wt_root="$HOME/.local/share/worktrees/$repo"

    # Direct jump if worktree name provided
    if [[ -n "$1" && -d "$wt_root/$1" ]]; then
        cd "$wt_root/$1" || return 1
        return
    fi

    cd "$dir" || return 1

    local tmp selected
    tmp=$(mktemp "${TMPDIR:-/tmp}/lazyworktree.selection.XXXXXX") || return 1
    lazyworktree --output-selection="$tmp" # --search-auto-select # Add search auto select if desired
    local rc=$?
    if [[ $rc -ne 0 ]]; then
        rm -f "$tmp"
        return $rc
    fi

    if [[ -s "$tmp" ]]; then
        selected=$(<"$tmp")
        [[ -n "$selected" && -d "$selected" ]] && cd "$selected" || true
    fi
    rm -f "$tmp"
}

worktree_go_last() {
    local dir="$1"
    local id repo slug last_selected selected

    if [[ -z "$dir" || ! -d "$dir" ]]; then
        print -u2 "worktree_go_last: invalid directory: $dir"
        return 1
    fi

    id=$(_git_repo_slug "$dir" 2>/dev/null)
    if [[ -n "$id" ]]; then
        slug=${id#*:}
        repo="$slug"
    else
        repo=$(basename "$dir")
    fi

    last_selected="$HOME/.local/share/worktrees/$repo/.last-selected"
    if [[ -f "$last_selected" ]]; then
        selected=$(<"$last_selected")
        if [[ -n "$selected" && -d "$selected" ]]; then
            cd "$selected" || return 1
            return
        fi
    fi

    print -u2 "No last selected worktree found"
    return 1
}

_worktree_jump() {
    local dir="$1"
    local id repo slug wt_root

    id=$(_git_repo_slug "$dir" 2>/dev/null)
    if [[ -n "$id" ]]; then
        slug=${id#*:}
        repo="$slug"
    else
        repo=$(basename "$dir")
    fi

    wt_root="$HOME/.local/share/worktrees/$repo"
    [[ -d "$wt_root" ]] || return

    local -a dirs
    dirs=(${wt_root}/*(/:t))
    _describe 'worktree' dirs
}

# Adjust at your will
# if [[ -d ~/git/myrepo ]]; then
#     pm() { worktree_jump ~/git/myrepo "$@" }
#     _pm() { _worktree_jump ~/git/myrepo }
#     compdef _pm pm
# fi
#
# vim: ft=zsh
