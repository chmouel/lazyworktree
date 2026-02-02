---
paths:
  - "README.md"
  - "lazyworktree.1"
  - "internal/app/screen/help.go"
---

# Documentation Rules

When updating documentation:

- **British spelling**: Use British English spelling throughout (colour, behaviour, initialise).
- **Professional butler style**: Clear, helpful, dignified but not pompous.
- **Three-way sync**: Keep README.md, lazyworktree.1 (man page), and help screen in sync.
- **Feature completeness**: When adding features or keybindings, update all three sources.
- **Man page format**: Follow troff/groff formatting conventions for the man page.
- **Technical precision**: Maintain accuracy whilst ensuring readability.
- Keep README prose tight. Avoid verbosity.

## Brevity Rules

1. **Remove filler**: "optionally", "interactively", "in order to", "It is important to note"
2. **Collapse phrases**: "It provides a structured workflow" → "It offers a workflow"
3. **Terse bullets**: Short phrases, not full sentences
4. **Limit examples**: 2-3 per concept, not 4-5
5. **No redundant prose**: If a table covers it, don't repeat in text

## Phrases to Tighten

| Verbose | Tighter |
|---------|---------|
| "This allows you to..." | Direct statement |
| "The following X are available" | Just list them |
| "In order to" | "To" |
| "for example here is" | "Example:" |
| "can be specified explicitly or auto-generated" | "explicit or auto-generated" |
| "When viewing X, Y shows Z" | "X shows Z" |

## Target

Keep README under 850 lines. Current baseline: ~810 lines.
