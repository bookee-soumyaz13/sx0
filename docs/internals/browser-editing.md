# Browser Typing & Save

px0's viewer accepts typing in the browser and writes the file through `POST /api/save`. Agent edits (`Alt+E`) stay available. Huge-file reads are unchanged: the raw buffer is loaded only when you first type, and files over 2 MB / 50,000 lines stay agent-only.

## How an edit works

1. Click in the code view (not the git diff, not Markdown preview, not an image).
2. Type. The first keystroke fetches `/api/raw` into an in-memory line array on the tab.
3. While the tab is dirty, rows render as escaped text (syntax colours return after a successful save).
4. `Cmd/Ctrl+S` or the footer **Save** button POSTs the buffer. The server writes atomically (temp file in the same directory, fsync, rename), evicts the highlight cache, and closes any LSP document for that path.
5. If the file changed on disk since it was opened, save returns 409 and the UI asks to overwrite.

## Guards

- Same `localPost` gate as agent edits: POST from px0's own page, Host is an IP or `localhost`. Hostname tunnels cannot save.
- Workspace `safePath` only. Symlinks, directories, missing files, and paths outside the root are refused.
- `-no-edit` turns the UI back into a viewer (`/api/meta` reports `"edit": false`).
- Unsaved tabs confirm before close and `beforeunload`. Re-index skips dirty tabs so a refresh cannot wipe a buffer.

## Vim

With `editor.vimMode`, Normal mode still navigates. `i` / `a` / `I` / `A` / `o` / `O` enter Insert; `Esc` returns to Normal. `x` and `dd` delete. Visual `d` deletes the selection. `c` / `e` still hand the selection to a coding agent.
