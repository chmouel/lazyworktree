# Automatically Generated Worktree Notes

Configure `worktree_note_script` to generate initial worktree notes when creating from a PR/MR or issue. The script receives the selected item's title and body on stdin and can produce multiline output. If the script fails or outputs nothing, creation continues and no note is saved.

To store notes in a single synchronisable JSON file, set `worktree_notes_path`. When enabled, keys are stored relative to the repository under `worktree_dir` instead of absolute filesystem paths.

### Configuration

```yaml
worktree_note_script: "aichat -m gemini:gemini-2.5-flash-lite 'Summarise this ticket into practical implementation notes.'"
```

### Script Requirements

Receives content on stdin, outputs note text on stdout. Timeout: 30s.

### Environment Variables

`LAZYWORKTREE_TYPE` (pr/issue), `LAZYWORKTREE_NUMBER`, `LAZYWORKTREE_TITLE`, `LAZYWORKTREE_URL`.
