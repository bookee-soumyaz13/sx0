# Browser Editing

Click in a source tab and type. px0 keeps the virtualized viewer — there is no Monaco or textarea overlay — and writes the file when you save.

## Shortcuts

| Shortcut | Action |
| :--- | :--- |
| Type / Backspace / Enter / Tab | Edit at the caret |
| `Cmd/Ctrl+S` | Save |
| `Cmd/Ctrl+Z` / `Cmd/Ctrl+Shift+Z` | Undo / redo |
| `Cmd/Ctrl+X` | Cut the selection |
| Footer **Save** | Same as `Cmd/Ctrl+S` |

Agent edits (`Alt+E`) still work on a selection. Diff view and Markdown preview stay read-only; switch to source to type.

Files larger than 2 MB or 50,000 lines cannot be edited here. Use `Alt+E` or `-no-edit` to keep px0 as a viewer (`px0 -no-edit`).
