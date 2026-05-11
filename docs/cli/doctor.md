# doctor

Report CLI, repository, and tooling health for automation.

## Synopsis

```bash
lazyworktree doctor
lazyworktree doctor --json
```

## Why use it first

`doctor` is the safest entry point for coding agents and scripts. It confirms:

- whether a repository is visible from the current directory
- whether worktrees can be listed
- whether a config file was loaded
- whether helper tools such as `git`, `gh`, and `glab` are available

Unlike most operational commands, `doctor --json` remains useful even when setup is incomplete.

## Examples

```bash
lazyworktree doctor --json
lazyworktree doctor --json | jq '{repo: .repository.repo, worktree_count: .repository.worktree_count}'
```

## Output notes

- Human output is brief and diagnostic.
- `--json` writes structured output to stdout only.
- Setup gaps are reported in the JSON payload instead of causing a hard failure.
