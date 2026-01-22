## Project

A TUI for managing GIT worktrees.
Read the README.md if you really need to know what this project is all about.

## Building

- use `make build` for testing build errors.
- if you ever add a new argument or flag to the cli then make sure to update the completion and README.md
- Don't ever do commit unless you are being explicitely asked for it.
- If you get asked to commit then use this rules:
  - Follow Conventional Commits 1.0.0.
  - 50 chars for title 70 chars for body.
  - Cohesive long phrase or paragraph unless multiple points are needed.
  - Use bullet points only if necessary for clarity.
  - Past tense.
  - State **what** and **why** only (no “how”).

## Documentation

- For any user-facing changes (features, options, keybindings, etc.), ensure you update:
  - `README.md`
  - `lazyworktree.1` man page
  - Internal help (`NewHelpScreen.helpText`)
- Documentation and help string style guidelines:
  - Consistent British spelling.
  - Professional butler style: clear, helpful, dignified but not pompous
  - Remove any overly casual Americanisms
  - Keep technical precision whilst maintaining readability

## UI

- UI colours must come from theme fields, avoid hardcoded colours in rendering.

## Before Finishing

- Always Run `make sanity` which will run `golangci-lint`, `gofumpt`, and `go test`.
- Add tests for any new functionality.
- Make sure coverage is top notch
