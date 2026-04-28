package tftp

import (
	"path/filepath"
	"testing"
)

func TestLoadConfig_MissingFile(t *testing.T) {
	dir := t.TempDir()
	cfg, err := loadConfig(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Root != "" || cfg.Interface != "" {
		t.Errorf("want zero config, got %+v", cfg)
	}
}

func TestSaveLoadConfig_Roundtrip(t *testing.T) {
	dir := t.TempDir()
	in := TFTPConfig{
		Interface:    "eth0",
		Root:         filepath.Join(dir, "tftp-root"),
		ReadEnabled:  true,
		WriteEnabled: false,
		BlockSize:    1468,
	}
	if err := saveConfig(dir, in); err != nil {
		t.Fatalf("save: %v", err)
	}
	out, err := loadConfig(dir)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if out != in {
		t.Errorf("want %+v, got %+v", in, out)
	}
}

func TestSaveConfig_AtomicWrite(t *testing.T) {
	dir := t.TempDir()
	if err := saveConfig(dir, TFTPConfig{Root: "/x"}); err != nil {
		t.Fatal(err)
	}
	entries, err := filepath.Glob(filepath.Join(dir, "*.tmp"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("leftover .tmp file(s): %v", entries)
	}
}
