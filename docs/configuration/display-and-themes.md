# Display and Themes

Use these settings to control appearance, icon rendering, and layout behaviour.

## Theme Selection

- `theme`: explicit theme name
- empty `theme`: auto-detect based on terminal background
- CLI override: `lazyworktree --theme <name>`

Built-in themes and details:

- [Themes](../themes.md)

## Layout and Pane Arrangement

- `layout`: `default` or `top`
- runtime toggle: `L`

Layout controls how worktree, status, git status, commit, and notes panes are arranged.

## Icon Rendering

- `icon_set`: `nerd-font-v3` or `text`

If glyphs render incorrectly, use `text` or install a patched Nerd Font.

## Name Truncation

- `max_name_length`: max display width for worktree names
- set to `0` to disable truncation
