# Common Problems

This page covers frequent setup and workflow issues.

## LazyWorktree opens but looks broken

- set `icon_set: text`
- verify terminal font supports required glyphs
- see [Fonts and Rendering](fonts-and-rendering.md)

## CI logs do not display as expected

- check `pager` and `ci_script_pager` settings
- verify pager command is available in your shell
- see [Integration Caveats](integration-caveats.md)

## Custom commands behave differently in CLI and TUI

- confirm command type and key binding in config
- note that `new-tab` commands are not supported in CLI `exec`

## `.wt` hooks are not running

- check `trust_mode`
- verify trust prompt was accepted for repository `.wt`
- inspect trusted hash store at `~/.local/share/lazyworktree/trusted.json`
