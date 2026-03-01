# Branch Naming

Branch names are sanitised for Git compatibility and can be generated automatically.

## Sanitisation Rules

- special characters -> hyphens
- leading/trailing hyphens removed
- consecutive hyphens collapsed
- length capped (manual 50, auto 100)

Examples:

| Input | Converted |
| --- | --- |
| `feature.new` | `feature-new` |
| `bug fix here` | `bug-fix-here` |
| `feature:test` | `feature-test` |

## Auto-Generated Names

Use `branch_name_script` to generate names from issue/PR/diff context.

Typical templates:

- `issue-{number}-{title}`
- `issue-{number}-{generated}`
- `pr-{number}-{generated}`
- `pr-{number}-{pr_author}-{title}`

If generation fails, `{generated}` falls back to `{title}`.

## Script Contract

- input via stdin
- output first line on stdout
- timeout: 30s

Environment variables available to scripts:

- `LAZYWORKTREE_TYPE`
- `LAZYWORKTREE_NUMBER`
- `LAZYWORKTREE_TEMPLATE`
- `LAZYWORKTREE_SUGGESTED_NAME`

Full examples and template details:

- [Branch Naming Conventions](../branch-naming.md)
