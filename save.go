package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode/utf8"
)

// editDisabled is the -no-edit flag. Like gitDisabled, it is set once in main
// before anything reads it. When set, /api/save is refused and the UI stays a
// viewer; agent edits are unaffected.
var editDisabled bool

const (
	// Cap for a single in-browser save. Viewing still uses maxFileBytes (64 MB);
	// editing loads the whole file into the tab, so this stays small.
	editMaxBytes = 2 << 20
	editMaxLines = 50000
)

type saveReq struct {
	Path    string `json:"path"`
	Content string `json:"content"`
	Mtime   string `json:"mtime"`
	Force   bool   `json:"force"`
}

func fileMtime(st os.FileInfo) string {
	return strconv.FormatInt(st.ModTime().UnixNano(), 10)
}

func (s *Server) handleSave(w http.ResponseWriter, r *http.Request) {
	if !localPost(w, r) {
		return
	}
	if editDisabled {
		fail(w, http.StatusForbidden, "inline editing is disabled")
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, editMaxBytes+1<<20))
	if err != nil {
		fail(w, 400, "failed to read body")
		return
	}
	var req saveReq
	if err := json.Unmarshal(body, &req); err != nil {
		fail(w, 400, "invalid JSON")
		return
	}
	if req.Path == "" {
		fail(w, 400, "missing path")
		return
	}
	if !utf8.ValidString(req.Content) {
		fail(w, 400, "content is not valid UTF-8")
		return
	}
	if len(req.Content) > editMaxBytes {
		fail(w, http.StatusRequestEntityTooLarge, fmt.Sprintf("file too large to save here (%d bytes)", len(req.Content)))
		return
	}
	if strings.Count(req.Content, "\n") > editMaxLines {
		fail(w, http.StatusRequestEntityTooLarge, "file too large to save here")
		return
	}

	abs, rel, ok := s.safePath(req.Path)
	if !ok {
		fail(w, 400, "bad path")
		return
	}
	st, err := os.Lstat(abs)
	if err != nil {
		fail(w, 404, err.Error())
		return
	}
	if st.Mode()&os.ModeSymlink != 0 {
		fail(w, 400, "refusing to write through a symlink")
		return
	}
	if st.IsDir() {
		fail(w, 400, "is a directory")
		return
	}
	if req.Mtime != "" && !req.Force && fileMtime(st) != req.Mtime {
		fail(w, http.StatusConflict, "file changed on disk")
		return
	}

	if err := writeFileAtomic(abs, []byte(req.Content), st.Mode().Perm()); err != nil {
		fail(w, 500, err.Error())
		return
	}

	Evict(abs)
	if s.lsp != nil {
		s.lsp.CloseDoc(abs, rel)
	}

	nst, err := os.Stat(abs)
	if err != nil {
		fail(w, 500, err.Error())
		return
	}
	if uiVerbose {
		uiStatus("ok", "save", fmt.Sprintf("%s · %s", rel, formatBytes(int64(len(req.Content)))), 0, os.Stdout)
	}
	writeJSON(w, map[string]any{
		"ok":    true,
		"path":  rel,
		"size":  nst.Size(),
		"mtime": fileMtime(nst),
	})
}

// writeFileAtomic writes data to path by creating a sibling temp file, fsyncing,
// then renaming over the destination so a crash cannot leave a truncated file.
func writeFileAtomic(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	f, err := os.CreateTemp(dir, ".sx0-save-*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	cleanup := true
	defer func() {
		if cleanup {
			os.Remove(tmp)
		}
	}()
	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err := f.Chmod(perm); err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		if err2 := os.Remove(path); err2 != nil && !os.IsNotExist(err2) {
			return err
		}
		if err := os.Rename(tmp, path); err != nil {
			return err
		}
	}
	cleanup = false
	return nil
}
