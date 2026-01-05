## Project

Go TUI for managing GIT worktrees.
Read the README if you really need to know what this project do.

## Building

- use `make build` for testing build errors.
- Don't ever do commit unless you are being explicitely asked for it.
- If you get asked to commit then use this rules:
  - Follow Conventional Commits 1.0.0.
  - 50 chars for title 70 chars for body.
  - Cohesive long phrase or paragraph unless multiple points are needed.
  - Use bullet points only if necessary for clarity.
  - Past tense.
  - State **what** and **why** only (no “how”).

## Before Finishing

- Always Run `make sanity` which will run golangci-lint, gofumpt, and go test.
- Add tests for any new functionality.
- Add README.md updates for any user-facing changes. Do not sound like
it is written by a brain-dead overenthusiastic robot.
- Update lazyworktree.1 man page for any user-facing changes (features, options, keybindings, etc).
