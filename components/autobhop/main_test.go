package main

import (
	"path/filepath"
	"testing"
)

func TestParseSavedConfigAcceptsUTF8BOM(t *testing.T) {
	data := append(
		[]byte{0xEF, 0xBB, 0xBF},
		[]byte(`{"player_base":"0x726BD8","poll_ms":1,"source":"bom-test"}`)...,
	)

	cfg, err := parseSavedConfig(data)
	if err != nil {
		t.Fatalf("parseSavedConfig() error = %v", err)
	}
	if cfg.PlayerBase != "0x726BD8" {
		t.Fatalf("PlayerBase = %q, want %q", cfg.PlayerBase, "0x726BD8")
	}
	if cfg.PollMS != 1 {
		t.Fatalf("PollMS = %d, want 1", cfg.PollMS)
	}
	if cfg.Source != "bom-test" {
		t.Fatalf("Source = %q, want %q", cfg.Source, "bom-test")
	}
}

func TestConfigPathUsesMixDataRoot(t *testing.T) {
	root := t.TempDir()
	t.Setenv("L4D2_MIX_DATA_ROOT", root)

	want := filepath.Join(root, "data", "autobhop-settings.json")
	if got := configPath(); got != want {
		t.Fatalf("configPath() = %q, want %q", got, want)
	}
}
