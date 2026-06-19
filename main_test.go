package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestExtractPayloadPreservesMutableRules(t *testing.T) {
	localAppData := t.TempDir()
	hostRoot := t.TempDir()
	t.Setenv("LOCALAPPDATA", localAppData)
	t.Setenv("L4D2_MIX_HOST_ROOT", hostRoot)

	root, err := extractPayload()
	if err != nil {
		t.Fatalf("extractPayload() error = %v", err)
	}

	required := []string{
		"L4D2AutobhopVPKW.exe",
		"L4D2RowFilterManager.exe",
		"L4D2ModJoin.exe",
	}
	for _, rel := range required {
		if _, err := os.Stat(filepath.Join(root, rel)); err != nil {
			t.Fatalf("required payload %q missing: %v", rel, err)
		}
	}

	dataRequired := []string{
		filepath.Join("data", "matchmaking_probe_loader", "L4D2MatchmakingProbeLoader.exe"),
		filepath.Join("data", "row-filter", "matchmaking_row_filter.dll"),
		filepath.Join("data", "row-filter", "launch_row_filter_early_admin.ps1"),
	}
	for _, rel := range dataRequired {
		if _, err := os.Stat(filepath.Join(hostRoot, rel)); err != nil {
			t.Fatalf("required data payload %q missing: %v", rel, err)
		}
	}

	rules := filepath.Join(hostRoot, "data", "row-filter", "blocked_keywords.txt")
	const custom = "keep-my-custom-rule\r\n"
	if err := os.WriteFile(rules, []byte(custom), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := extractPayload(); err != nil {
		t.Fatalf("second extractPayload() error = %v", err)
	}
	got, err := os.ReadFile(rules)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != custom {
		t.Fatalf("mutable rules overwritten: got %q", got)
	}
}

func TestImportLegacyModJoinStateCopiesMissingAndPreservesConflicts(t *testing.T) {
	source := t.TempDir()
	destination := t.TempDir()

	if err := os.WriteFile(filepath.Join(source, ".l4d2modjoin-deployment.json"), []byte(`{"version":1}`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "l4d2modjoin-settings.json"), []byte(`{"source":"legacy"}`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(destination, "l4d2modjoin-settings.json"), []byte(`{"source":"current"}`), 0644); err != nil {
		t.Fatal(err)
	}

	report := importLegacyModJoinStateFrom(destination, []string{source})
	if len(report.Imported) != 1 || report.Imported[0] != ".l4d2modjoin-deployment.json" {
		t.Fatalf("unexpected imported files: %#v", report.Imported)
	}
	if len(report.Preserved) != 1 || report.Preserved[0] != "l4d2modjoin-settings.json" {
		t.Fatalf("unexpected preserved files: %#v", report.Preserved)
	}
	if _, err := os.Stat(filepath.Join(destination, ".l4d2modjoin-deployment.json")); err != nil {
		t.Fatalf("deployment registry was not imported: %v", err)
	}
	current, err := os.ReadFile(filepath.Join(destination, "l4d2modjoin-settings.json"))
	if err != nil || string(current) != `{"source":"current"}` {
		t.Fatalf("current settings were overwritten: %q, %v", current, err)
	}
	legacy, err := os.ReadFile(filepath.Join(destination, "legacy-import", "l4d2modjoin-settings.json"))
	if err != nil || string(legacy) != `{"source":"legacy"}` {
		t.Fatalf("legacy conflict was not preserved: %q, %v", legacy, err)
	}

	marker, err := os.ReadFile(filepath.Join(destination, "legacy-import-v1.json"))
	if err != nil {
		t.Fatal(err)
	}
	var stored legacyImportReport
	if err := json.Unmarshal(marker, &stored); err != nil || stored.Version != 1 {
		t.Fatalf("invalid import marker: %v %#v", err, stored)
	}
}
