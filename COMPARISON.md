# Comparison with Other Git Worktree Tools

This document compares **lazyworktree** with other Git worktree management tools.
It aims to be **honest and practical**, highlighting both where lazyworktree
excels and where **other tools are objectively better choices** depending on
constraints and workflow.

---

## High-Level Positioning

| Tool | Strength |
|----|----|
| **lazyworktree** | Full interactive environment for humans |
| [jean](https://github.com/coollabsio/jean-tui) | AI-powered workflows + persistent Claude sessions |
| [git-worktree-runner (gtr)](https://github.com/coderabbitai/git-worktree-runner) | Best CLI + scripting ergonomics |
| [worktrunk (wt)](https://github.com/max-sixty/worktrunk) | Parallel / AI-agent workflows |
| [worktree-plus (wtp)](https://github.com/satococoa/wtp) | Minimal, predictable automation |
| [worktree-cli](https://github.com/fnebenfuehr/worktree-cli) | AI-focused with MCP integration |
| [branchlet](https://github.com/raghavpillai/branchlet) | Lightweight TUI with low cognitive load |
| [gwm](https://github.com/shutootaki/gwm) | Fast fuzzy navigation |
| [Treekanga](https://github.com/garrettkrohn/treekanga) | Smart CLI with Editor/Shell integrations |
| [newt](https://github.com/cdzombak/newt) | Fast, opinionated directory structure |
| [kosho](https://github.com/carlsverre/kosho) | Command-centric, agent-oriented |
| [wtm](https://github.com/jarredkenny/worktree-manager) | Bare-repo / CI / server environments |

lazyworktree intentionally trades **simplicity and scriptability** for **interactive power**.

It is built on the **DWIM (Do What I Mean)** principle, enabling **intuitiveness** by anticipating user intent rather than requiring explicit, verbose instructions. For example:

* **Smart Creation**: Creating a worktree from a PR automatically fetches code, tracks the branch, and names the directory meaningfully (e.g., `pr-123-fix-bug`).
* **Intelligent Absorb**: "Absorbing" a worktree doesn't just delete it; it intelligently rebases or merges changes into main and cleans up artifacts, assuming "I am finished with this feature" is the goal.
* **Context Aware**: Opening a terminal (`t`) automatically creates or attaches to a dedicated tmux session for that specific worktree, setting the correct working directory and window names.

---

## Core Worktree Management

| Feature | lazyworktree | gtr | wt | wtp |
|-------|--------------|-----|----|-----|
| Create / delete worktrees | ✅ | ✅ | ✅ | ✅ |
| Rename worktrees | ✅ | ❌ | ❌ | ❌ |
| Cherry-pick commits between worktrees | ✅ | ❌ | ❌ | ❌ |
| Absorb into main | ✅ | ❌ | ⚠️ manual | ❌ |
| Prune merged worktrees | ✅ | ⚠️ manual | ⚠️ manual | ⚠️ limited |
| Create from uncommitted changes | ✅ | ❌ | ❌ | ❌ |

### Where other tools win

* **gtr / wtp**: simpler mental model, fewer moving parts
* **wtp**: extremely predictable behaviour suitable for automation
* **wt**: optimized for creating many short-lived worktrees quickly

---

## Interface & Workflow

| Feature | lazyworktree | gtr | wtp | branchlet |
|-------|--------------|-----|-----|-----------|
| Full TUI | ✅ | ❌ | ❌ | ✅ |
| Zero-UI CLI | ❌ | ✅ | ✅ | ❌ |
| Works well over SSH / low latency | ⚠️ | ✅ | ✅ | ⚠️ |
| Easy to script | ❌ | ✅ | ✅ | ❌ |

### Clear advantages of other tools

* **gtr** is superior for:
  * shell pipelines
  * scripting
  * headless usage
* **wtp** is better when:
  * you want “do exactly one thing”
  * no UI dependencies
* **branchlet** is faster to understand for first-time users

lazyworktree is not optimized for scripting or non-interactive environments.

---

## Automation & Hooks

| Feature | lazyworktree | gtr | wt | wtp |
|-------|--------------|-----|----|-----|
| Hooks | ✅ | ✅ | ✅ | ✅ |
| Secure hook execution (TOFU) | ✅ | ❌ | ❌ | ❌ |
| Built-in automation primitives | ✅ | ❌ | ❌ | ❌ |
| Works without config | ❌ | ✅ | ✅ | ✅ |

### Where other tools win

* **gtr**:
  * hooks live in git config
  * easier to reason about in shared repos
* **wtp**:
  * fewer abstractions
  * easier to debug
* **wt**:
  * intentionally avoids policy decisions

lazyworktree’s automation is more powerful but **more complex**.

---

## Forge / PR Integration

| Feature | lazyworktree | others |
|-------|--------------|--------|
| PR/MR status | ✅ | ❌ |
| CI checks | ✅ | ❌ |
| Create worktree from PR | ✅ | ❌ |

### Trade-off

This integration:

* adds dependencies (`gh`, `glab`)
* may be undesirable in minimal or offline setups

Other tools avoid this entirely.

---

## tmux / Shell Integration

| Feature | lazyworktree | wt | gtr |
|-------|--------------|----|-----|
| tmux orchestration | ✅ | ⚠️ basic | ❌ |
| Shell jump | ✅ | ✅ | ⚠️ manual |
| Multi-window sessions | ✅ | ❌ | ❌ |

### Where others win

* **wt**:
  * simpler shell integration
  * easier to reason about
* **gtr**:
  * no tmux dependency
  * fewer assumptions

lazyworktree assumes tmux-heavy workflows.

---

## Configuration & Maintenance

| Aspect | lazyworktree | gtr | wtp |
|-----|--------------|-----|-----|
| Configuration size | Large | Small | Small |
| Learning curve | High | Low | Low |
| Failure modes | More | Fewer | Fewer |
| Upgrade risk | Higher | Lower | Lower |

This is an **explicit trade-off**:
 optimizes for capability, not minimalism.

---

## When NOT to use lazyworktree

lazyworktree is **not the best choice** if you:

* need headless or CI usage
* rely heavily on shell scripting
* want minimal dependencies
* prefer explicit Git commands
* manage worktrees mostly via automation

In these cases:

* use **gtr** or **wtp**

---

## jean

A modern worktree TUIs that looks very similar to lazyworktree is jean. Here is
a detailed comparison between the two tools.

### Architecture & Codebase

| Aspect | lazyworktree | jean |
|--------|--------------|------|
| Language | Go (Python → Go migration) | Go |
| Lines of code | ~27,000 | ~14,000 |
| Dependencies | Charmbracelet (TUI), Git, gh/glab, tmux/zellij | Charmbracelet (TUI), Git, gh, tmux, Claude CLI |
| Configuration | YAML `.lazyworktree.yaml` + `.wt` files | JSON `~/.config/jean/config.json` + `jean.json` scripts |
| Primary design goal | Full interactive power | AI-powered workflows with Claude |

### Feature Comparison

| Feature | lazyworktree | jean |
|---------|--------------|------|
| **Core Worktree Ops** | Create, delete, rename, cherry-pick, absorb | Create, delete, rename, merge (local) |
| **AI Integration** | Via custom commands (external scripts) | Built-in: commit messages, branch names, PR content (11+ models) |
| **Claude/IDE Integration** | Lazygit via `g` key | Persistent Claude CLI sessions + 7 editors (code, cursor, nvim, vim, subl, atom, zed) |
| **PR/MR Management** | GitHub/GitLab, full status + CI checks | GitHub only, basic PR operations (create draft, merge, view) |
| **Session Management** | tmux/zellij with templates | tmux only, dual mode (Claude + terminal) |
| **Code Viewing** | Diff viewer with delta, commit log | Diff access for AI context only |
| **Theme System** | 15 built-in themes | 5 built-in themes |
| **Automation** | `.wt` files with TOFU security, custom commands | Setup scripts (`jean.json`), no security model |
| **Output Selection** | Yes (via CLI flags) | No |
| **File Tree View** | Yes | No |
| **Search/Filtering** | Per-pane filtering | Global search in modals |
| **Hotkeys** | 20+ keybindings | 20+ keybindings |
| **Commit Interface** | Via external git | Dedicated commit modal with AI generation |

### Strengths & Weaknesses

#### jean Advantages

* **AI-first design**: Integrated OpenRouter API for commit messages, branch naming, PR content
* **Claude ecosystem**: Persistent Claude CLI sessions tracked per branch with initialization state
* **Simpler codebase**: ~14K lines vs 27K, easier to understand and extend
* **Editor integration**: 7 popular editors supported natively
* **Workflow automation**: Auto-commit, auto-rename, auto-PR creation with AI
* **Minimal dependencies**: No need for `glab` (GitLab), simpler setup
* **Settings UI**: Visual settings menu instead of config files

#### jean Disadvantages

* **Less comprehensive**: No cherry-pick, no absorb, limited merge support (local only)
* **GitHub-only**: No GitLab/Gitea support (lazyworktree supports both)
* **Limited forge integration**: No CI checks display, simpler PR management
* **No code viewer**: Can't browse diffs or commit logs in TUI (unlike lazyworktree's diff viewer)
* **Smaller feature surface**: Fewer worktree operations, no command palette
* **No TOFU security**: Setup scripts run without trust verification
* **Single session type focus**: tmux-centric, no Zellij support (lazyworktree supports both)
* **Less production-proven**: Newer tool, lower battle-tested status

#### lazyworktree Advantages

* **Richer information display**: 3-pane layout with diff viewer and commit log
* **Multi-platform forge**: GitHub + GitLab + Gitea support
* **Advanced CI integration**: Real-time CI status checks with caching
* **More worktree operations**: Absorb, cherry-pick, prune merged
* **Flexible automation**: Command palette, custom commands, TOFU security model
* **Session flexibility**: tmux AND Zellij support with templates
* **More themes**: 15 vs 5 options
* **Better SSH support**: Tested for high-latency environments
* **Output selection mode**: For piping worktree selection to shell scripts

#### lazyworktree Disadvantages

* **Higher complexity**: 27K lines, more learning curve
* **More configuration needed**: YAML + `.wt` files per repo
* **No built-in AI**: Requires external scripts for AI workflows
* **No IDE/editor integration**: Use shell opener or external integration
* **External dependencies**: Requires `gh` AND `glab` for full forge support
* **No dedicated commit UI**: Uses external git/editor for commits
* **Slower to grasp**: More modes, more keybindings, steeper learning curve

### When to Choose Each

#### Choose **jean** if you

* Work with Claude AI and want persistent sessions
* Want simple, focused worktree management
* Prefer AI-generated commits/PRs over manual entry
* Use GitHub exclusively
* Like visual settings menus over config files
* Want a minimal, learnable tool (~14K lines)
* Develop with VS Code, Cursor, or Neovim
* Need auto-naming and auto-commit workflows

#### Choose **lazyworktree** if you

* Need advanced worktree operations (absorb, cherry-pick)
* Use GitHub/GitLab/Gitea (multi-platform)
* Want real-time CI status visibility
* Prefer code browsing within TUI (diffs, commit logs)
* Need flexible session management (tmux/Zellij)
* Require TOFU-secured automation
* Work over SSH with high latency
* Value comprehensive feature depth over simplicity
* Need output selection for scripting

### Code Quality Comparison

| Aspect | lazyworktree | jean |
|--------|--------------|------|
| Test coverage | 62.6% | Minimal (basic tests only) |
| Commit discipline | Conventional commits | Conventional commits |
| Refactoring status | In progress (`app.go` large) | Clean separation of concerns |
| Documentation | Comprehensive (README, guides) | Good (README, CLAUDE.md) |
| Stability | Production-ready | Stable but newer |

---

## General Git TUIs (lazygit, gitui)

Tools like [lazygit](https://github.com/jesseduffield/lazygit) or [gitui](https://github.com/extrawurst/gitui) are excellent general-purpose Git interfaces. They do support worktrees, but they treat them as just another list to manage.

**lazyworktree** has been heavily inspired by the ease of use and "lazy" philosophy of **lazygit**. It is designed to complement it, featuring a **built-in integration** (via the `g` key) that allows you to launch lazygit directly inside the currently selected worktree for full Git control.

**jean** takes a different approach, focusing on AI automation and Claude integration, but remains a worktree-first tool.

Both **lazyworktree** and **jean** remain different from general Git TUIs because they treat the **worktree as the primary unit of work**, building the entire workflow (switching, creating from PRs, opening in editor/tmux) around that concept.

---

## Summary

**lazyworktree** provides the **largest feature surface** and the richest interactive experience for Git worktrees, with the deepest forge integration (GitHub/GitLab/Gitea), most advanced CI status display, and most flexible automation.

**jean** is a specialized worktree manager optimized for **AI-powered workflows with Claude ecosystem integration**, offering simplicity and AI automation at the cost of feature depth.

Other tools are objectively better when:

* simplicity matters more than power (use **branchlet** or **wtp**)
* scripting and automation are primary (use **gtr** or **wtp**)
* environments are constrained (use **newt** or **wtm**)
* users prefer explicit Git semantics (use any CLI tool)
* you need AI integration and Claude sessions (use **jean**)

**Positioning:**

* **lazyworktree** is a **comprehensive workspace manager for humans** with maximum interactive capability
* **jean** is an **AI-first worktree manager** designed around Claude workflows
* Other tools remain excellent **worktree utilities for systems** and specialized use cases
