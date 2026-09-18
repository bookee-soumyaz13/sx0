package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSaveWritesFile(t *testing.T) {
	s, root := newTestServer(t)
	code, out := agentPostJSON(t, s, "/api/save", map[string]any{
		"path":    "greet.go",
		"content": "package main\n\nfunc greet() {}\n",
	})
	if code != 200 {
		t.Fatalf("save = %d %v, want 200", code, out)
	}
	body, err := os.ReadFile(filepath.Join(root, "greet.go"))
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "package main\n\nfunc greet() {}\n" {
		t.Fatalf("wrote %q", body)
	}
	if out["path"] != "greet.go" {
		t.Fatalf("path = %v", out["path"])
	}
}

func TestSaveRejectsTraversal(t *testing.T) {
	s, _ := newTestServer(t)
	code, _ := agentPostJSON(t, s, "/api/save", map[string]any{
		"path":    "../outside.go",
		"content": "nope\n",
	})
	if code != 400 {
		t.Fatalf("traversal save = %d, want 400", code)
	}
}

func TestSaveRequiresLocalPost(t *testing.T) {
	s, _ := newTestServer(t)
	code, _ := get(t, s, "/api/save")
	if code != http.StatusMethodNotAllowed {
		t.Fatalf("GET save = %d, want 405", code)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/save", bytes.NewReader([]byte(`{"path":"greet.go","content":"x"}`)))
	req.Host = "evil.example"
	req.Header.Set("Origin", "https://evil.example")
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("cross-origin save = %d, want 403", rec.Code)
	}
}

func TestSaveConflictAndForce(t *testing.T) {
	s, root := newTestServer(t)
	st, err := os.Stat(filepath.Join(root, "greet.go"))
	if err != nil {
		t.Fatal(err)
	}
	mtime := fileMtime(st)

	if err := os.WriteFile(filepath.Join(root, "greet.go"), []byte("changed-on-disk\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	code, out := agentPostJSON(t, s, "/api/save", map[string]any{
		"path":    "greet.go",
		"content": "from-browser\n",
		"mtime":   mtime,
	})
	if code != http.StatusConflict {
		t.Fatalf("stale save = %d %v, want 409", code, out)
	}
	onDisk, _ := os.ReadFile(filepath.Join(root, "greet.go"))
	if string(onDisk) != "changed-on-disk\n" {
		t.Fatalf("conflict overwrote disk: %q", onDisk)
	}

	code, out = agentPostJSON(t, s, "/api/save", map[string]any{
		"path":    "greet.go",
		"content": "from-browser\n",
		"mtime":   mtime,
		"force":   true,
	})
	if code != 200 {
		t.Fatalf("force save = %d %v, want 200", code, out)
	}
	onDisk, _ = os.ReadFile(filepath.Join(root, "greet.go"))
	if string(onDisk) != "from-browser\n" {
		t.Fatalf("force wrote %q", onDisk)
	}
}

func TestSaveDisabled(t *testing.T) {
	prev := editDisabled
	editDisabled = true
	t.Cleanup(func() { editDisabled = prev })

	s, root := newTestServer(t)
	before, _ := os.ReadFile(filepath.Join(root, "greet.go"))
	code, out := agentPostJSON(t, s, "/api/save", map[string]any{
		"path":    "greet.go",
		"content": "should-not-write\n",
	})
	if code != http.StatusForbidden {
		t.Fatalf("disabled save = %d %v, want 403", code, out)
	}
	after, _ := os.ReadFile(filepath.Join(root, "greet.go"))
	if string(after) != string(before) {
		t.Fatalf("disabled save still wrote %q", after)
	}
}

func TestSaveRefusesMissingAndDirectory(t *testing.T) {
	s, _ := newTestServer(t)
	code, _ := agentPostJSON(t, s, "/api/save", map[string]any{
		"path":    "no-such.go",
		"content": "x\n",
	})
	if code != 404 {
		t.Fatalf("missing = %d, want 404", code)
	}
	code, _ = agentPostJSON(t, s, "/api/save", map[string]any{
		"path":    "sub",
		"content": "x\n",
	})
	if code != 400 {
		t.Fatalf("directory = %d, want 400", code)
	}
}

func TestSaveRefusesSymlink(t *testing.T) {
	s, root := newTestServer(t)
	target := filepath.Join(root, "greet.go")
	link := filepath.Join(root, "link.go")
	if err := os.Symlink(target, link); err != nil {
		t.Skip("symlink not supported")
	}
	code, out := agentPostJSON(t, s, "/api/save", map[string]any{
		"path":    "link.go",
		"content": "via-symlink\n",
	})
	if code != 400 {
		t.Fatalf("symlink save = %d %v, want 400", code, out)
	}
	body, _ := os.ReadFile(target)
	if strings.Contains(string(body), "via-symlink") {
		t.Fatal("symlink save wrote through to the target")
	}
}

func TestMetaAdvertisesEdit(t *testing.T) {
	s, _ := newTestServer(t)
	code, out := get(t, s, "/api/meta")
	if code != 200 {
		t.Fatalf("meta = %d", code)
	}
	if out["edit"] != true {
		t.Fatalf("edit = %v, want true", out["edit"])
	}

	prev := editDisabled
	editDisabled = true
	t.Cleanup(func() { editDisabled = prev })
	_, out = get(t, s, "/api/meta")
	if out["edit"] != false {
		t.Fatalf("edit with -no-edit = %v, want false", out["edit"])
	}
}

