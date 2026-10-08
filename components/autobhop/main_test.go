package main

import (
	"path/filepath"
	"testing"
	"time"
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

func TestWorkerCompletionRetainsNewOwner(t *testing.T) {
	old := make(chan struct{})
	current := make(chan struct{})
	state.stop, state.running = current, true
	defer func() { state.stop, state.running = nil, false }()
	markStopped(old)
	if state.stop != current || !state.running {
		t.Fatal("old worker cleared new owner")
	}
	markStopped(current)
	if state.stop != nil || state.running {
		t.Fatal("current worker did not finish")
	}
}

func TestClientBaseWaitStopsWithoutWaitingForNextPoll(t *testing.T) {
	stop := make(chan struct{})
	timer := time.AfterFunc(10*time.Millisecond, func() { close(stop) })
	defer timer.Stop()
	start := time.Now()
	base, stopped := waitForClientBase(0, stop)
	if base != 0 || !stopped || time.Since(start) > 500*time.Millisecond {
		t.Fatalf("stop delayed: base=%x stopped=%v elapsed=%v", base, stopped, time.Since(start))
	}
}
