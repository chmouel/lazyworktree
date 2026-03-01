# Themes

Built-in themes:

| Theme | Notes | Best For |
|-------|-------|----------|
| **dracula** | Dark (#282A36) | Dark terminals, vibrant colours, default fallback |
| **dracula-light** | White (#FFFFFF) | Light terminals, Dracula colours, default light theme |
| **narna** | Charcoal (#0D1117) | Dark terminals, blue highlights |
| **clean-light** | White (#FFFFFF) | Light terminals, cyan accent |
| **catppuccin-latte** | Soft white (#EFF1F5) | Catppuccin Latte light palette |
| **rose-pine-dawn** | Warm white (#FAF4ED) | Rosé Pine Dawn warm palette |
| **one-light** | Light grey (#FAFAFA) | Atom One Light |
| **everforest-light** | Beige (#F3EFDA) | Everforest nature light |
| **solarized-dark** | Deep teal (#002B36) | Classic Solarized dark palette |
| **solarized-light** | Cream (#FDF6E3) | Classic Solarized light palette |
| **gruvbox-dark** | Dark grey (#282828) | Gruvbox dark, warm accents |
| **gruvbox-light** | Sand (#FBF1C7) | Gruvbox light, earthy tones |
| **nord** | Midnight blue (#2E3440) | Nord calm cyan accents |
| **monokai** | Olive black (#272822) | Monokai bright neon accents |
| **catppuccin-mocha** | Mocha (#1E1E2E) | Catppuccin Mocha pastels |
| **modern** | Zinc (#18181B) | Sleek modern dark theme with violet accents |
| **tokyo-night** | Storm (#24283B) | Tokyo Night Storm with blue highlights |
| **one-dark** | Dark (#282C34) | Atom One Dark classic palette |
| **rose-pine** | Midnight (#191724) | Rosé Pine dark and moody |
| **ayu-mirage** | Mirage (#212733) | Ayu Mirage modern look |
| **everforest-dark** | Dark (#2D353B) | Everforest nature dark |
| **kanagawa** | Wave (#1F1F28) | Kanagawa Wave inspired by Japanese art |

Set in config: `theme: dracula`

### Custom Themes

Define custom themes that inherit from built-in themes or define new colour schemes.

**Inherit from built-in:**

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

**Defining a complete theme (all 11 colour fields required):**

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

**Inherit from other custom themes:**

```yaml
custom_themes:
  base-custom:
    base: dracula
    accent: "#FF0000"
  derived:
    base: base-custom
    accent: "#00FF00"
```

**Colour fields:** `accent`, `accent_fg`, `accent_dim`, `border`, `border_dim`, `muted_fg`, `text_fg`, `success_fg`, `warn_fg`, `error_fg`, `cyan`.

Values must be hex (`#RRGGBB` or `#RGB`). With `base`, only override what you need. Without `base`, all 11 fields are required. Custom themes appear alongside built-in themes.

