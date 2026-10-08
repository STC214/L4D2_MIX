//go:build windows

package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStartupUsesSavedPathsWithoutDetection(t *testing.T) {
	root := t.TempDir()
	state := filepath.Join(root, "data")
	if err := os.MkdirAll(state, 0755); err != nil {
		t.Fatal(err)
	}
	settings := appSettings{Version: 1, Source: "saved-source", Output: "saved-output", Addons: "saved-addons", WeaponSoundVolumePercent: 42, WeaponSoundVolumeConfigured: true, CustomWeaponSoundVolume: "33"}
	if err := saveAppSettings(state, settings); err != nil {
		t.Fatal(err)
	}
	result := prepareStartupState(root, state, root, func() string { t.Fatal("saved paths triggered drive probing"); return "" })
	if result.Source != settings.Source || result.Output != settings.Output || result.Addons != settings.Addons || result.Settings.CustomWeaponSoundVolume != "33" {
		t.Fatalf("settings lost: %#v", result)
	}
}
func TestStartupMigratesBeforeReadingSettings(t *testing.T) {
	root := t.TempDir()
	state := filepath.Join(root, "data")
	if err := saveAppSettings(root, appSettings{Version: 1, Addons: "legacy-addons", Source: "legacy-source"}); err != nil {
		t.Fatal(err)
	}
	result := prepareStartupState(root, state, root, func() string { t.Fatal("migration should have supplied path"); return "" })
	if result.Addons != "legacy-addons" || result.Source != "legacy-source" {
		t.Fatalf("migration lost settings: %#v", result)
	}
}
func TestStartupDetectsOnlyWhenNoSavedPath(t *testing.T) {
	root := t.TempDir()
	addons := filepath.Join(root, "game", "addons")
	if err := os.MkdirAll(filepath.Join(addons, "workshop"), 0755); err != nil {
		t.Fatal(err)
	}
	calls := 0
	result := prepareStartupState(root, filepath.Join(root, "data"), root, func() string { calls++; return addons })
	if calls != 1 || result.Source != filepath.Join(addons, "workshop") {
		t.Fatalf("invalid detection: %#v calls=%d", result, calls)
	}
}
func TestStartupMalformedSettingsArePreserved(t *testing.T) {
	root := t.TempDir()
	state := filepath.Join(root, "data")
	if err := os.MkdirAll(state, 0755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(state, settingsName)
	if err := os.WriteFile(path, []byte("broken"), 0644); err != nil {
		t.Fatal(err)
	}
	result := prepareStartupState(root, state, root, func() string { return "" })
	actual, _ := os.ReadFile(path)
	if string(actual) != "broken" || len(result.Messages) == 0 {
		t.Fatal("malformed settings not preserved/reported")
	}
}
