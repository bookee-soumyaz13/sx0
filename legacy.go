package main

import (
	"os"
	"path/filepath"
	"strings"
)

const (
	productName = "sx0"
	legacyName  = "px0"
)

// envProduct reads SX0_<suffix>, then PX0_<suffix> so existing shells keep
// working after the rename.
func envProduct(suffix string) string {
	if v := strings.TrimSpace(os.Getenv("SX0_" + suffix)); v != "" {
		return v
	}
	return strings.TrimSpace(os.Getenv("PX0_" + suffix))
}

func copyFileIfAbsent(dst, src string) {
	if dst == "" || src == "" || dst == src {
		return
	}
	if _, err := os.Stat(dst); err == nil {
		return
	}
	data, err := os.ReadFile(src)
	if err != nil {
		return
	}
	mode := os.FileMode(0o644)
	if info, err := os.Stat(src); err == nil {
		mode = info.Mode().Perm()
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return
	}
	_ = os.WriteFile(dst, data, mode)
}

func homeDotFile(product, file string) string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ""
	}
	return filepath.Join(home, "."+product, file)
}
