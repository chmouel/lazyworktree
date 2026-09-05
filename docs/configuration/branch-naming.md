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

## Suggested Names

When a worktree is created without an explicit name, the branch name prompt is
pre-filled with a random `adjective-noun` suggestion, such as
`scrupulous-stable`. Accept it or type your own.

Suggestions are drawn from curated lists of more than 200 adjectives and more
than 250 nouns, giving over 58,000 pairings. Every word is lowercase ASCII
without punctuation, so a suggestion is always safe to use as both a Git ref
and a directory name.

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
