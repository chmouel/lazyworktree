# Configuration Overview

LazyWorktree supports layered configuration from defaults up to CLI overrides.

<div class="mint-callout">
  <p><strong>Use this page when:</strong> you need to understand where settings come from and how to apply them safely.</p>
</div>

## Configuration Sources

Highest to lowest precedence:

1. CLI `--config` overrides
2. Git local configuration (`git config --local`)
3. Git global configuration (`git config --global`)
4. YAML file (`~/.config/lazyworktree/config.yaml`)
5. built-in defaults

## Global YAML

Primary config file:

- `~/.config/lazyworktree/config.yaml`

Reference example:

- [`config.example.yaml`](https://github.com/chmouel/lazyworktree/blob/main/config.example.yaml)

## Git Configuration Prefix

Use `lw.` keys:

```bash
git config --global lw.theme nord
git config --local lw.sort_mode switched
```

List configured keys:

```bash
git config --global --get-regexp "^lw\."
git config --local --get-regexp "^lw\."
```

## Configuration Areas

- [Reference](reference.md)
- [Display and Themes](display-and-themes.md)
- [Refresh and Performance](refresh-and-performance.md)
- [Diff, Pager, and Editor](diff-pager-and-editor.md)
- [Lifecycle Hooks](lifecycle-hooks.md)
- [Branch Naming](branch-naming.md)
- [Custom Themes](custom-themes.md)

## Next Steps

<div class="mint-card-grid">
  <a class="mint-card" href="reference.md">
    <strong>Configuration Reference</strong>
    <span>Generated key, type, default, and description table from source.</span>
  </a>
  <a class="mint-card" href="display-and-themes.md">
    <strong>Display and Themes</strong>
    <span>Pick theme, icon mode, and layout defaults.</span>
  </a>
  <a class="mint-card" href="refresh-and-performance.md">
    <strong>Refresh and Performance</strong>
    <span>Tune refresh cadence and list rendering behaviour.</span>
  </a>
</div>
