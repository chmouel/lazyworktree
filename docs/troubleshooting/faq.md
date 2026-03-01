# FAQ

## Do I need to use shell integration?

No. Shell helpers are optional, but they make `cd`-to-selected-worktree smoother.

## Can I keep one notes file across repositories?

Yes. Set `worktree_notes_path` to store notes in a shared JSON file.

## Why is my PR branch name different after create-from-PR?

LazyWorktree may preserve PR branch names for PR authors, otherwise it may use generated names.

## Why are `.wt` commands not running?

Check `trust_mode` and repository trust decisions (TOFU prompts and trust hash state).

## Can I use custom command keys from CLI?

Yes. Use `lazyworktree exec --key=<key>` with optional `--workspace`.

## Why does `new-tab` not work from CLI `exec`?

`new-tab` is intentionally unsupported in CLI mode.

## Where should I start debugging visual issues?

Start with [Fonts and Rendering](fonts-and-rendering.md), then review theme and icon settings.
