# lazyworktree shell functions for Fish shell
# Review and customize these functions as needed for your workflow
# Example function to jump between git worktrees using lazyworktree and GitHub CLI

function _git_repo_slug --description "Extract repository slug from git remote"
    set -l dir "$argv[1]"
    set -l url
    set -l provider
    set -l slug

    if not test -d "$dir"
        return 1
    end

    set -l url (cd "$dir" 2>/dev/null; and git remote get-url origin 2>/dev/null)
    if test -z "$url"
        return 1
    end

    # Determine provider
    if string match -q '*github.com*' "$url"
        set provider github
    else if string match -q '*gitlab.com*' "$url"
        set provider gitlab
    else
        set provider unknown
    end

    # Extract slug from URL
    set slug (echo "$url" | sed -E 's#^.*[:/]([^/]+/[^/]+)(\.git)?$#\1#')

    if test -z "$slug"
        return 1
    end

    echo "$provider:$slug"
end

# Example function to jump between git worktrees using lazyworktree
function worktree_jump --description "Jump to a git worktree using lazyworktree"
    set -l dir "$argv[1]"
    set -e argv[1]
    set -l id
    set -l repo
    set -l slug

    if test -z "$dir" -o ! -d "$dir"
        echo "worktree_jump: invalid directory: $dir" >&2
        return 1
    end

    set id (_git_repo_slug "$dir" 2>/dev/null)

    if test -n "$id"
        set slug (string split ':' "$id")[2]
        set repo "$slug"
    else
        set repo (basename "$dir")
    end

    set -l wt_root "$HOME/.local/share/worktrees/$repo"

    # Direct jump if worktree name provided
    if test -n "$argv[1]" -a -d "$wt_root/$argv[1]"
        cd "$wt_root/$argv[1]"
        return $status
    end

    cd "$dir"
    or return 1

    set -l tmp (mktemp -t lazyworktree.selection.XXXXXX)
    or return 1

    lazyworktree --output-selection="$tmp"
    set -l rc $status

    if test $rc -ne 0
        rm -f "$tmp"
        return $rc
    end

    if test -s "$tmp"
        set -l selected (cat "$tmp")
        if test -n "$selected" -a -d "$selected"
            cd "$selected"
        end
    end

    rm -f "$tmp"
end

function worktree_go_last --description "Jump to the last selected worktree"
    set -l dir "$argv[1]"
    set -l id
    set -l repo
    set -l slug
    set -l last_selected
    set -l selected

    if test -z "$dir" -o ! -d "$dir"
        echo "worktree_go_last: invalid directory: $dir" >&2
        return 1
    end

    set id (_git_repo_slug "$dir" 2>/dev/null)
    if test -n "$id"
        set slug (string split ':' "$id")[2]
        set repo "$slug"
    else
        set repo (basename "$dir")
    end

    set last_selected "$HOME/.local/share/worktrees/$repo/.last-selected"
    if test -f "$last_selected"
        set selected (cat "$last_selected")
        if test -n "$selected" -a -d "$selected"
            cd "$selected"
            return $status
        end
    end

    echo "No last selected worktree found" >&2
    return 1
end

function _worktree_jump --description "Completion helper for worktree_jump"
    set -l dir "$argv[1]"
    set -l id
    set -l repo
    set -l slug
    set -l wt_root

    set id (_git_repo_slug "$dir" 2>/dev/null)
    if test -n "$id"
        set slug (string split ':' "$id")[2]
        set repo "$slug"
    else
        set repo (basename "$dir")
    end

    set wt_root "$HOME/.local/share/worktrees/$repo"
    test -d "$wt_root"
    or return

    for wt in $wt_root/*/
        basename "$wt"
    end
end

# Adjust at your will
# if test -d ~/git/myrepo
#     function jt
#         worktree_jump ~/git/myrepo $argv
#     end
#     complete -c jt -f -a '(_worktree_jump ~/git/myrepo)'
# end
