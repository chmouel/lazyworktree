# Custom Themes

Create custom themes by inheriting from built-ins or defining full colour sets.

## Inherit from Built-In Theme

```yaml
custom_themes:
  my-dark:
    base: dracula
    accent: "#FF6B9D"
    text_fg: "#E8E8E8"

  my-light:
    base: dracula-light
    accent: "#0066CC"
```

## Define a Full Theme

Without `base`, all 11 colour fields are required.

```yaml
custom_themes:
  completely-custom:
    accent: "#00FF00"
    accent_fg: "#000000"
    accent_dim: "#2A2A2A"
    border: "#3A3A3A"
    border_dim: "#2A2A2A"
    muted_fg: "#888888"
    text_fg: "#FFFFFF"
    success_fg: "#00FF00"
    warn_fg: "#FFFF00"
    error_fg: "#FF0000"
    cyan: "#00FFFF"
```

## Required Colour Keys

- `accent`
- `accent_fg`
- `accent_dim`
- `border`
- `border_dim`
- `muted_fg`
- `text_fg`
- `success_fg`
- `warn_fg`
- `error_fg`
- `cyan`

Hex format accepted: `#RRGGBB` or `#RGB`.

For built-in theme list, see [Themes](../themes.md).
