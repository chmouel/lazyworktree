# Contributing

Thank you for considering a contribution to LazyWorktree. This guide covers the development workflow, conventions, and standards the project follows.

<div class="mint-callout">
  <p><strong>Refer to this page when:</strong> you want to build from source, run tests, submit changes, or understand the project's development practices.</p>
</div>

## Prerequisites

- Go 1.26 or later
- Git for runtime and development
- `uv`: Python tooling for documentation (`brew install uv` on macOS)
- `golangci-lint`: linting (`brew install golangci-lint` or see [installation docs](https://golangci-lint.run/usage/install/))
- `gofumpt`: Go formatting (`go install mvdan.cc/gofumpt@latest`)

## Clone and Build

```bash
git clone https://github.com/chmouel/lazyworktree.git
cd lazyworktree
make build
```

The binary is placed at `bin/lazyworktree`.

## Development Workflow

Read `DESIGN.md` in the repository root when you are changing architecture,
cross-cutting flows, or ownership across multiple subsystems.

### Quality Check

Run the full quality pipeline before submitting changes:

```bash
make sanity
```

This executes, in order:

1. `make lint` runs `golangci-lint` with `--fix`.
2. `make format` runs `gofumpt` to normalise formatting.
3. `make test` runs `go test ./...`.

Because `make sanity` includes `--fix` and `-w` steps, it may rewrite files in
your working tree.

### Individual Targets

| Target | Command | Description |
| --- | --- | --- |
| Build | `make build` | Compile to `bin/lazyworktree` |
| Lint | `make lint` | Run `golangci-lint` |
| Format | `make format` | Apply `gofumpt` formatting |
| Test | `make test` | Run all Go tests |
| Coverage | `make coverage` | Write a per-function coverage summary to `coverage.out` |

### Test Coverage

The project targets 55 %+ coverage, focused on critical paths (git operations,
configuration loading, key handlers). Run a coverage summary with:

```bash
make coverage
```

## Documentation Workflow

Documentation uses [MkDocs](https://www.mkdocs.org/) with the [Material theme](https://squidfunk.github.io/mkdocs-material/). Use `uv` or `uvx` for documentation tooling; do not use `pip`.

| Target | Command | Description |
| --- | --- | --- |
| Sync | `make docs-sync` | Regenerate reference pages from source code |
| Check | `make docs-check` | Verify sync is up to date + strict build |
| Build | `make docs-build` | Build the documentation site |
| Serve | `make docs-serve` | Serve locally on `0.0.0.0:7827` |

A typical documentation workflow:

```bash
# Edit docs/ files
make docs-sync        # Regenerate references from source
make docs-check       # Verify everything is in order
make docs-serve       # Preview locally
```

For documentation or other user-facing text changes, run `make docs-check`
before submitting.

## CI Pipeline

The project uses GitHub Actions with four workflow files:

| Workflow | Purpose |
| --- | --- |
| `ci.yml` | Runs docs synchronisation checks and pre-commit validation on matching push and pull request changes |
| `pages.yml` | Validates docs, builds the website and docs, and deploys them to GitHub Pages from `main` |
| `nightly.yml` | Generates coverage output and updates the README coverage badge on a schedule |
| `releaser.yaml` | Builds tagged releases with GoReleaser and refreshes release notes |

## Commit Conventions

The project follows [Conventional Commits 1.0.0](https://www.conventionalcommits.org/en/v1.0.0/):

- Title: 50 characters maximum
- Body: 70 characters per line; use a cohesive paragraph unless bullets improve clarity.
- Tense: use past tense.
- Content: state what changed and why; do not describe how.

Examples:

```
feat: added worktree notes synchronisation

Allowed teams to share implementation context across machines
by storing notes in a committable JSON file.
```

```
fix: corrected branch name fallback on script timeout

The {generated} placeholder silently returned an empty string
when the AI script exceeded 30 seconds.
```

## Documentation Style

- Use British spelling throughout (`colour`, `synchronise`, `behaviour`).
- Use a professional butler tone: clear, helpful, dignified, and never pompous.
- Remove overly casual Americanisms.
- Maintain technical precision and readability.
- Add the `<div class="mint-callout">` pattern to the top of every new or expanded page.

## Code Style

- Theme colours: use theme fields for all UI rendering; never hard-code colours.
- When adding or changing CLI commands, arguments, or flags, update:
    - Shell completion
    - `README.md`
    - `lazyworktree.1`
    - Internal help text/template in `internal/app/screen/help.go`
    - Generated CLI docs via `make docs-sync`
    - Relevant website docs
- When adding features, options, or keybindings, update:
    - `README.md`
    - `lazyworktree.1` man page
    - Internal help text/template in `internal/app/screen/help.go`
    - Documentation site (if applicable)
- Add focused tests for changed behaviour where practical. Explain any gaps in your handoff or pull request notes.

## Filing Bug Reports

A good bug report includes:

- Version: output of `lazyworktree --version`
- Terminal: name, version, and font
- Operating system: name and version
- Debug log: run with `--debug-log /tmp/lw-debug.log` and attach the log
- Reproduction steps: minimal steps to trigger the issue
- Expected and actual behaviour: what you expected and what happened
