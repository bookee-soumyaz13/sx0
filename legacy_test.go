package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEnvProduct(t *testing.T) {
	t.Setenv("SX0_REPO", "")
	t.Setenv("PX0_REPO", "old/px0")
	if got := envProduct("REPO"); got != "old/px0" {
		t.Fatalf("legacy env: got %q, want old/px0", got)
	}

	t.Setenv("SX0_REPO", "new/sx0")
	if got := envProduct("REPO"); got != "new/sx0" {
		t.Fatalf("new env should win: got %q", got)
	}
}

func TestCopyFileIfAbsent(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.txt")
	dst := filepath.Join(dir, "dst.txt")
	if err := os.WriteFile(src, []byte("hello"), 0o600); err != nil {
		t.Fatal(err)
	}
	copyFileIfAbsent(dst, src)
	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "hello" {
		t.Fatalf("got %q, want hello", got)
	}

	if err := os.WriteFile(src, []byte("changed"), 0o600); err != nil {
		t.Fatal(err)
	}
	copyFileIfAbsent(dst, src)
	got, err = os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "hello" {
		t.Fatalf("existing dest was overwritten: %q", got)
	}
}

func TestSettingsMigrateFromLegacy(t *testing.T) {
	cfg := isolateSettings(t)
	legacy := filepath.Join(cfg, legacyName)
	if err := os.MkdirAll(legacy, 0o755); err != nil {
		t.Fatal(err)
	}
	payload := []byte(`{"editor.fontSize": 18, "agent": "claude"}`)
	if err := os.WriteFile(filepath.Join(legacy, "settings.json"), payload, 0o644); err != nil {
		t.Fatal(err)
	}

	m := readMergedSettingsMap()
	if m["editor.fontSize"] != 18.0 {
		t.Fatalf("migrated fontSize = %v, want 18", m["editor.fontSize"])
	}
	if m["agent"] != "claude" {
		t.Fatalf("migrated agent = %v, want claude", m["agent"])
	}

	dst := filepath.Join(cfg, productName, "settings.json")
	if _, err := os.Stat(dst); err != nil {
		t.Fatalf("expected copy at %s: %v", dst, err)
	}
}

func TestSettingsMigrateDoesNotOverwrite(t *testing.T) {
	cfg := isolateSettings(t)
	if err := os.MkdirAll(filepath.Join(cfg, productName), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(cfg, legacyName), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfg, productName, "settings.json"), []byte(`{"editor.fontSize": 20}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfg, legacyName, "settings.json"), []byte(`{"editor.fontSize": 10}`), 0o644); err != nil {
		t.Fatal(err)
	}

	m := readMergedSettingsMap()
	if m["editor.fontSize"] != 20.0 {
		t.Fatalf("sx0 settings should win, got %v", m["editor.fontSize"])
	}
}

func TestUpdateStateMigrateFromLegacy(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_STATE_HOME", tmp)
	legacy := filepath.Join(tmp, legacyName)
	if err := os.MkdirAll(legacy, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacy, "update_check.json"), []byte(`{"latest_ver":"0.9.9"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	s, err := readUpdateState()
	if err != nil {
		t.Fatal(err)
	}
	if s.LatestVer != "0.9.9" {
		t.Fatalf("LatestVer = %q, want 0.9.9", s.LatestVer)
	}
}

func TestDistinctIDMigrateFromLegacy(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	legacyDir := filepath.Join(home, "."+legacyName)
	if err := os.MkdirAll(legacyDir, 0o755); err != nil {
		t.Fatal(err)
	}
	const id = "0123456789abcdef0123456789abcdef"
	if err := os.WriteFile(filepath.Join(legacyDir, "anonymous_id"), []byte(id+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	got := getOrGenerateDistinctID()
	if got != id {
		t.Fatalf("id = %q, want %q", got, id)
	}
	copied, err := os.ReadFile(filepath.Join(home, "."+productName, "anonymous_id"))
	if err != nil {
		t.Fatal(err)
	}
	if string(copied) != id+"\n" {
		t.Fatalf("copied id %q", copied)
	}
}
