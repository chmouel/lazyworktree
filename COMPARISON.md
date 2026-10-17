# Comparison with Other Git Worktree Tools

This document compares **lazyworktree** with other Git worktree management tools.

It is designed to be **decision-oriented** and **evidence-driven**, focusing on:

* The primary workflow each tool optimizes for
* Where lazyworktree provides unique value
* Where other tools are objectively better choices
* Realistic trade-offs in complexity, automation, and environment constraints

This document intentionally avoids marketing positioning and instead focuses on **fit-for-purpose tool selection**.

---

## Ecosystem Positioning (By Workflow Category)

This comparison groups tools by the workflows they optimise for.

### Human-Interactive Workspace Managers

| Tool         | Primary Value                                                  |
| ------------ | -------------------------------------------------------------- |
| lazyworktree | Deep interactive workspace + forge + CI visibility             |
| jean         | AI-first interactive workflows with persistent Claude sessions |
| branchlet    | Lightweight interactive TUI with minimal cognitive load        |
| gwm          | Fast interactive CLI + fuzzy selection                         |

These tools optimise for **humans actively driving development**.

---

### Automation / CLI / Script-Friendly Tools

| Tool      | Primary Value                                               |
| --------- | ----------------------------------------------------------- |
| gtr       | Excellent scripting ergonomics + editor + AI tool launching |
| wtp       | Deterministic automation + environment bootstrap hooks      |
| newt      | Minimal wrapper with predictable directory layout           |
| treekanga | CLI-first with smart branch handling + navigation helpers   |

These tools optimise for **repeatability and shell-first workflows**.

---

### Parallel Agents / AI-Oriented Workflows

| Tool         | Primary Value                                             |
| ------------ | --------------------------------------------------------- |
| worktrunk    | Parallel agent orchestration + LLM commit workflows       |
| kosho        | Multi-agent isolation with command execution per worktree |
| worktree-cli | MCP integration for AI assistants                         |

These tools optimise for **AI or multi-agent development models**.

---

### Bare Repo / CI / Server Environments

| Tool | Primary Value                                |
| ---- | -------------------------------------------- |
| wtm  | Bare repo workflow + CI-safe cleanup + hooks |

These tools optimise for **automation environments and shared servers**.

---

## High-Level Positioning

lazyworktree intentionally trades:

* Simplicity
* Scriptability
* Minimal dependencies

for:

* Rich interactive workflows
* Deep repository context visibility
* Integrated development workspace orchestration

The core design philosophy is **DWIM (Do What I Mean)**.

Examples:

* Creating a worktree from a PR fetches code, tracks branch, names directory.
* Absorbing a worktree assumes feature completion and merges/rebases safely.
* Opening terminal automatically attaches to per-worktree session.

---

## Core Worktree Lifecycle Capabilities

| Capability                           | lazyworktree | Typical CLI Tools |
| ------------------------------------ | ------------ | ----------------- |
| Create / delete worktrees            | Yes          | Yes               |
| Rename worktrees                     | Yes          | Rare              |
| Cross-worktree cherry-pick           | Yes          | Rare              |
| Feature completion workflow (absorb) | Yes          | Usually manual    |
| Create from dirty working tree       | Yes          | Rare              |
| Smart pruning merged trees           | Yes          | Often manual      |

### Where Other Tools Win

* CLI tools have smaller mental model
* Automation tools provide deterministic behaviour
* Parallel tools scale better to many simultaneous trees

---

## Interface and Interaction Model

| Capability                  | lazyworktree | CLI Tools |
| --------------------------- | ------------ | --------- |
| Full TUI workspace          | Yes          | No        |
| Pure CLI                    | Limited      | Yes       |
| SSH / high latency friendly | Moderate     | Excellent |
| Script-friendly             | Limited      | Excellent |

lazyworktree is intentionally **human-first, not pipeline-first**.

---

## Automation and Hooks

| Capability                   | lazyworktree | CLI Automation Tools |
| ---------------------------- | ------------ | -------------------- |
| Hook system                  | Yes          | Yes                  |
| Secure trust model (TOFU)    | Yes          | Rare                 |
| Built-in workflow primitives | Yes          | Usually external     |
| Works with zero config       | No           | Often Yes            |

Trade-off: lazyworktree provides **policy + power**, others provide **predictability + simplicity**.

---

## Forge / PR / CI Integration

| Capability              | lazyworktree | Most Other Tools |
| ----------------------- | ------------ | ---------------- |
| PR/MR status visibility | Yes          | Rare             |
| CI check visibility     | Yes          | Rare             |
| Create worktree from PR | Yes          | Rare             |

Trade-offs:

* Requires gh / glab
* Not ideal for offline or minimal installs

---

## Session and Terminal Integration

| Capability               | lazyworktree | Others       |
| ------------------------ | ------------ | ------------ |
| tmux orchestration       | Deep         | Usually none |
| Zellij support           | Yes          | Rare         |
| Shell navigation helpers | Yes          | Some tools   |

lazyworktree assumes session-based workflows.

---

## Configuration and Operational Complexity

| Attribute       | lazyworktree | Minimal Tools |
| --------------- | ------------ | ------------- |
| Config surface  | Large        | Small         |
| Learning curve  | High         | Low           |
| Failure surface | Higher       | Lower         |
| Upgrade risk    | Higher       | Lower         |

This is intentional: capability over minimalism.

---

## AI and Parallel Agent Workflows

| Capability                     | lazyworktree | AI-Native Tools |
| ------------------------------ | ------------ | --------------- |
| Built-in AI features           | No           | Often Yes       |
| Multi-agent orchestration      | External     | Often Core      |
| MCP / AI assistant integration | External     | Some tools      |

lazyworktree is AI-compatible but not AI-native.

---

## jean Comparison

### jean Strengths

* Built-in AI generation (commits, branches, PR content)
* Persistent Claude sessions per branch
* Smaller codebase and easier mental model
* Native editor integrations

### jean Weaknesses

* GitHub only
* Fewer worktree operations
* No CI visibility
* No code browsing in TUI
* No automation security model

---

### lazyworktree Strengths vs jean

* Multi-forge support
* CI visibility
* Advanced lifecycle operations
* Secure automation
* Workspace-style UI

---

### Decision Heuristic

Choose jean if:

* AI automation is primary workflow
* GitHub only
* Want simpler tool

Choose lazyworktree if:

* Need deep repo awareness
* Need lifecycle operations
* Need CI + forge visibility
* Prefer workspace model

---

## When NOT to Use lazyworktree

Avoid if you need:

* Headless or CI workflows
* Heavy shell scripting
* Minimal dependencies
* Deterministic automation

Prefer:

* gtr / wtp for automation
* wtm for CI / bare repos
* worktrunk / kosho for parallel agents

---

## Summary

lazyworktree is a workspace tool for people working across several Git worktrees.

It prioritizes:

* Context
* Safety
* Visibility
* Workflow compression

Other tools outperform it when:

* Simplicity matters more than capability
* Automation is primary
* Environment is constrained
* AI-first workflows are required

---

## Final Positioning

* lazyworktree -> Human workspace orchestration
* jean -> AI-first worktree UX
* CLI tools -> System-level worktree utilities
* Agent tools -> Parallel execution environments

Each is valid when used in its optimal workflow domain.
