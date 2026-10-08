package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestManifestRejectsInvalidFilesBeforeTouchingAddons(t *testing.T) {
	root := t.TempDir()
	addons := filepath.Join(root, "game", "addons")
	hash := strings.Repeat("0", 64)
	for _, files := range [][]builtFile{
		nil,
		{{Name: "../outside.vpk", SHA256: hash}},
		{{Name: "nested/out.vpk", SHA256: hash}},
		{{Name: "file.txt", SHA256: hash}},
		{{Name: "out.vpk", Size: -1, SHA256: hash}},
		{{Name: "out.vpk", SHA256: "broken"}},
		{{Name: "out.vpk", SHA256: hash}, {Name: "OUT.vpk", SHA256: hash}},
	} {
		if _, _, err := deployAndDisable(buildManifest{Files: files}, filepath.Join(root, "output"), addons, filepath.Join(root, "state"), nil); err == nil {
			t.Fatalf("invalid files accepted: %#v", files)
		}
	}
	if _, err := os.Stat(filepath.Dir(addons)); !os.IsNotExist(err) {
		t.Fatalf("invalid manifest touched game directory: %v", err)
	}
	if err := validateBuiltFiles([]builtFile{{Name: "legacy.vpk"}}, false); err != nil {
		t.Fatalf("legacy manifest rejected: %v", err)
	}
	if err := validateBuiltFiles([]builtFile{{Name: "valid.vpk", SHA256: hash}}, true); err != nil {
		t.Fatal(err)
	}
}
