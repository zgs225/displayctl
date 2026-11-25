package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadSettingsMissingFileIsZero(t *testing.T) {
	t.Setenv("DISPLAYCTL_DIR", t.TempDir())
	if got := LoadSettings().FixedDPI(); got != 0 {
		t.Errorf("FixedDPI() = %d, want 0 (no config.toml)", got)
	}
}

func TestLoadSettingsPinnedDPI(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "config.toml"), []byte("[defaults]\ndpi = 168\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DISPLAYCTL_DIR", dir)
	if got := LoadSettings().FixedDPI(); got != 168 {
		t.Errorf("FixedDPI() = %d, want 168", got)
	}
}

func TestLoadSettingsBadTOMLIsZero(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "config.toml"), []byte("not = [valid"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DISPLAYCTL_DIR", dir)
	if got := LoadSettings().FixedDPI(); got != 0 {
		t.Errorf("FixedDPI() = %d, want 0 on parse error", got)
	}
}
