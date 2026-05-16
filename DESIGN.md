# Design

This document is the authoritative architecture note for LazyWorktree.
Use it when changing package ownership, runtime flow, or other cross-cutting
behaviour.

For a contributor-focused summary, see
[`docs/development/architecture.md`](docs/development/architecture.md).

## Top-Level Structure

LazyWorktree has two entry paths that share core subsystems:

- the Bubble Tea TUI in `internal/app`
- the scripting and machine-facing CLI in `internal/cli`

`cmd/lazyworktree` stays intentionally small. Command wiring and startup live in
`internal/bootstrap`, while shared behaviour lives in packages such as
`internal/git`, `internal/config`, `internal/security`, `internal/theme`, and
`internal/multiplexer`.

## Ownership Boundaries

- `internal/bootstrap` owns command wiring, config bootstrap, and mode
  selection.
- `internal/app` owns the interactive model, rendering, async UI flows, and
  modal screens.
- `internal/cli` owns non-interactive and machine-readable operations.
- `internal/git` owns Git, GitHub, and GitLab command execution and parsing.
- `internal/config` owns global config loading, Git config overlays, and `.wt`
  repository config loading.
- `internal/security` owns trust decisions for `.wt` lifecycle hooks.
- `internal/theme` owns built-in themes, custom themes, and terminal detection.
- `internal/multiplexer` owns tmux, zellij, shell, and container command
  assembly.

## Design Rules

- Keep TUI-specific behaviour in `internal/app`; do not route CLI logic through
  the UI model.
- Keep command environment shaping and escaping rules shared so TUI and CLI do
  not drift.
- Keep rendering theme-backed; avoid hardcoded colours in UI code.
- Treat `.wt` execution as a trust-gated boundary and preserve the existing
  TOFU model unless an explicit security design change is intended.

## When To Update This File

Update this note when a change affects:

- package or subsystem ownership
- startup and control-flow boundaries
- config precedence rules
- trust and execution boundaries
- other architectural decisions that contributors need before editing
