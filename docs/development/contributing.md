# Contributing

Thank you for considering a contribution to LazyWorktree. This guide covers the development workflow, conventions, and standards the project follows.

<div class="mint-callout">
  <p><strong>Use this page when:</strong> you want to build from source, run tests, submit changes, or understand the project's development practices.</p>
</div>

## Prerequisites

- **Go** 1.22 or later
- **git** (used at runtime as well as for development)
- **uv** — Python tooling for documentation (`brew install uv` on macOS)
- **golangci-lint** — linting (`brew install golangci-lint` or see [installation docs](https://golangci-lint.run/usage/install/))
- **gofumpt** — code formatting (`go install mvdan.cc/gofumpt@latest`)

## Clone and Build

```bash
git clone https://github.com/chmouel/lazyworktree.git
cd lazyworktree
make build
```

The binary is placed at `bin/lazyworktree`.

## Development Workflow

### Quality Check

Run the full quality pipeline before submitting changes:

```bash
make sanity
```

This executes, in order:

1. `make lint` — runs `golangci-lint` with `--fix`
2. `make format` — runs `gofumpt` to normalise formatting
3. `make test` — runs `go test ./...`

### Individual Targets

| Target | Command | Description |
| --- | --- | --- |
| Build | `make build` | Compile to `bin/lazyworktree` |
| Lint | `make lint` | Run `golangci-lint` |
| Format | `make format` | Apply `gofumpt` formatting |
| Test | `make test` | Run all Go tests |
| Coverage | `make coverage` | Generate HTML coverage report |

### Test Coverage

The project targets 55 %+ coverage, focused on critical paths (git operations, configuration loading, key handlers). Run a coverage report with:

```bash
make coverage
```

## Documentation Workflow

Documentation uses [MkDocs](https://www.mkdocs.org/) with the [Material theme](https://squidfunk.github.io/mkdocs-material/). All documentation tooling uses `uv`/`uvx` — never use `pip` directly.

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

## CI Pipeline

The project uses GitHub Actions with four workflow files:

| Workflow | Purpose |
| --- | --- |
| `ci.yml` | Runs lint, format, and test on every push and pull request |
| `pages.yml` | Builds and deploys the documentation site to GitHub Pages |
| `nightly.yml` | Scheduled nightly checks (dependency freshness, extended tests) |
| `releaser.yaml` | Automates binary releases via GoReleaser |

## Commit Conventions

The project follows [Conventional Commits 1.0.0](https://www.conventionalcommits.org/en/v1.0.0/):

- **Title**: 50 characters maximum, imperative or past tense
- **Body**: 70 characters per line, cohesive paragraph unless bullet points aid clarity
- **Tense**: Past tense throughout
- **Content**: State **what** changed and **why** — never describe **how**

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

- **British spelling** throughout (`colour`, `synchronise`, `behaviour`)
- **Professional butler tone**: clear, helpful, dignified but never pompous
- Remove overly casual Americanisms
- Maintain technical precision whilst keeping content readable
- Every new or expanded page should include the `<div class="mint-callout">` pattern at the top

## Code Style

- **Theme colours**: all UI rendering must use theme fields — never hardcode colours
- **User-facing changes**: when adding features, options, or keybindings, update all of:
    - `README.md`
    - `lazyworktree.1` man page
    - Internal help (`NewHelpScreen.helpText`)
    - Documentation site (if applicable)

## Filing Bug Reports

A good bug report includes:

- **Version**: output of `lazyworktree --version`
- **Terminal**: name, version, and font in use
- **Operating system**: name and version
- **Debug log**: run with `--debug-log /tmp/lw-debug.log` and attach the log
- **Reproduction steps**: minimal steps to trigger the issue
- **Expected vs actual behaviour**: what you expected and what happened instead
