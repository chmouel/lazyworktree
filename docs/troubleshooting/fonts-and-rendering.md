# Fonts and Rendering



## Symptom: Strange Characters in UI

If you see boxes, question marks, or garbled glyphs where icons should appear, your terminal font does not include the required glyphs.

### Quick fix

Switch to plain-text icons for now:

```yaml
icon_set: text
```

### Diagnostic steps

1. Test whether your font supports Nerd Font glyphs:
    ```bash
    echo -e "\uf126 \uf09b \ue725"
    ```
2. If the characters render as boxes or question marks, your font lacks the necessary glyphs
3. Install a [Nerd Font](https://www.nerdfonts.com/) (version 3 or later), set it in your terminal profile, and restart the terminal session

!!! note
    LazyWorktree requires Nerd Font **v3** glyphs. Older Nerd Font versions may render some icons incorrectly or not at all. If you already have a Nerd Font installed but icons still look wrong, check the version.

## Symptom: Icons Missing vs Wrong Icon

These are distinct problems with different causes:

| Symptom | Likely cause | Fix |
| --- | --- | --- |
| Icons missing entirely (blank spaces) | Font lacks Nerd Font glyphs | Install a Nerd Font v3+ |
| Icons present but wrong glyph shown | Nerd Font v2 installed (codepoint changes in v3) | Upgrade to Nerd Font v3+ |
| Some icons correct, others wrong | Multiple fonts competing (fallback chain) | Set a single Nerd Font as the primary terminal font |

## Symptom: Colours Look Wrong

If colours appear washed out, inverted, or limited to 8/16 colours, your terminal may not support true colour (24-bit).

### Diagnostic steps

1. Check your `$TERM` and `$COLORTERM` environment variables:
    ```bash
    echo "TERM=$TERM  COLORTERM=$COLORTERM"
    ```
2. For true-colour support, `$COLORTERM` should be `truecolor` or `24bit`
3. If `$COLORTERM` is unset, add to your shell profile:
    ```bash
    export COLORTERM=truecolor
    ```
4. Verify true-colour rendering:
    ```bash
    printf '\e[38;2;255;100;0mTruecolor test\e[0m\n'
    ```
    This should appear in orange. If it appears in a different colour or plain white, true-colour is not active.

### Terminal-specific notes

Some terminals require additional configuration for true colour:

- **tmux**: add `set -g default-terminal "tmux-256color"` and `set -ag terminal-overrides ",*256col*:Tc"` to `~/.tmux.conf`
- **screen**: true-colour support is limited; consider tmux instead
- **SSH sessions**: ensure the remote `$TERM` is propagated correctly

## Symptom: PR/MR Author Avatars Do Not Appear

Avatar badges use the Kitty graphics protocol and are deliberately conservative.
They only appear in the Info pane when PR/MR data includes an author avatar URL
and `avatar_badges` is enabled. Each badge is rendered as a small, round avatar
with transparent corners so it blends into the surrounding pane.

```yaml
avatar_badges: auto
```

Use `avatar_badges: never` to keep plain-text author names everywhere. Use
`avatar_badges: always` only when your terminal supports Kitty graphics but is
not detected automatically.

Avatars are cached on disk under your user cache directory. The cache is
versioned, so upgrades that change the avatar appearance regenerate fresh images
automatically; you do not need to clear the cache by hand.

LazyWorktree wraps Kitty graphics registration for tmux passthrough. If avatars
do not render in tmux, ensure your tmux version and configuration allow
passthrough, for example `set -g allow-passthrough on`. If avatars still do not
render in tmux or zellij, test outside the multiplexer, then fall back to
`never` if your terminal stack cannot pass Kitty graphics sequences through
reliably.

## Tested Terminal Emulators

These terminals work with LazyWorktree's default icon and colour settings:

| Terminal | Icons | True colour | Avatar badges | Notes |
| --- | --- | --- | --- | --- |
| iTerm2 | Yes | Yes | No | Set Nerd Font in Profiles > Text |
| Alacritty | Yes | Yes | No | Set font in `alacritty.toml` |
| Kitty | Yes | Yes | Yes | Nerd Font symbols built in |
| WezTerm | Yes | Yes | Yes | Set font in `wezterm.lua` |
| Ghostty | Yes | Yes | Yes | Uses Kitty-compatible graphics |
| GNOME Terminal | Yes | Yes | No | Set Nerd Font in profile preferences |
| Windows Terminal | Yes | Yes | No | Set font face in `settings.json` |
| macOS Terminal.app | Partial | No | No | Limited colour support; use iTerm2 or another alternative |

## Theme Readability Checks

If text is hard to read or elements blend into the background:

- Verify contrast in the selected theme
- Test both light and dark mode if your terminal theme changes by context
- Try a high-contrast theme such as `clean-light` or `modern`

Theme selection reference:

- [Themes](../themes.md)
- [Display and Themes](../configuration/display-and-themes.md)
