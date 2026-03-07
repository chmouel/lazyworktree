# note

Show or edit worktree notes from the CLI without launching the TUI.

## Subcommands

### `note show [worktree-name]`

Print the raw note text to stdout. Defaults to the worktree detected from the
current directory. Suitable for piping and scripting.

### `note edit [worktree-name]`

Open the note in `$EDITOR` (falls back to `$VISUAL`, then `vi`). The file uses
YAML frontmatter for metadata (icon, color, updated_at) followed by a markdown body.

| Flag | Type | Usage |
| --- | --- | --- |
| `--input`, `-i` | `string` | Read note from file (use `-` for stdin) |

## Examples

```bash
# Show note for current worktree
lazyworktree note show

# Show note for a specific worktree
lazyworktree note show my-feature

# Edit in your editor
lazyworktree note edit my-feature

# Set from stdin
echo "release prep" | lazyworktree note edit -i -

# Set from file
lazyworktree note edit my-feature -i note.md
```

## Note format

When editing, the file uses YAML frontmatter:

```markdown
---
icon: rocket
color: "#FF0000"
---
This is the note body in markdown.
```

The `icon` field is optional and sets a custom icon for the worktree in the TUI.
The `color` field is optional and sets a colour for the worktree name (hex, supported named colour, or 256 index); see **Set worktree colour** in the TUI command palette.
