# Fonts and Rendering

Use this guide when icons or symbols render incorrectly.

## Symptom: strange characters in UI

Set plain text icons:

```yaml
icon_set: text
```

## Symptom: icons missing or inconsistent

- install a Nerd Font patched terminal font
- ensure terminal profile uses that font
- restart terminal session after font/profile updates

## Theme readability checks

- verify contrast in selected theme
- test both light and dark mode if terminal theme changes by context

Theme selection reference:

- [Themes](../themes.md)
- [Display and Themes](../configuration/display-and-themes.md)
