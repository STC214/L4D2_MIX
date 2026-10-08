package vpkmerge

import (
	"hash/crc32"
	"os"
	"path/filepath"
	"testing"
)

func TestReadContentRejectsTruncatedHugeEntry(t *testing.T) {
	path := filepath.Join(t.TempDir(), "small.vpk")
	if err := os.WriteFile(path, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	for _, entry := range []sourceEntry{
		{vpk: path, length: ^uint32(0)},
		{vpk: path, offset: 2, length: 1},
		{vpk: path, dataBase: -2, length: 1},
	} {
		if _, err := readContent(entry); err == nil {
			t.Fatalf("invalid entry accepted: %#v", entry)
		}
	}
}

func TestOutputContentRejectsChangingSource(t *testing.T) {
	entry := &treeEntry{sourceEntry: sourceEntry{path: "test.txt", crc: crc32.ChecksumIEEE([]byte("old"))}, outLength: 3}
	if err := validateOutputContent(entry, []byte("old")); err != nil {
		t.Fatal(err)
	}
	for _, data := range []string{"new", "longer", ""} {
		if err := validateOutputContent(entry, []byte(data)); err == nil {
			t.Fatalf("changed source %q accepted", data)
		}
	}
}

func TestPlanValidatesAllPathsBeforeWriting(t *testing.T) {
	root := t.TempDir()
	for _, groups := range [][]Group{
		{{Output: "../outside.vpk"}},
		{{Output: "nested/out.vpk"}},
		{{Output: ""}},
		{{Output: "first.vpk"}, {Output: "FIRST.vpk"}},
		{{Output: "input.vpk", Packages: []string{"input.vpk"}}},
		{{Output: "out.vpk", Packages: []string{"../input.vpk"}}},
		{{Output: "first.vpk"}, {Output: "../outside.vpk"}},
	} {
		if err := Run(Plan{Input: root, Output: root, Groups: groups}, nil); err == nil {
			t.Fatalf("invalid plan accepted: %#v", groups)
		}
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 0 {
		t.Fatalf("invalid plan wrote files: %v %v", entries, err)
	}
	if err := validatePlanPaths(Plan{Input: filepath.Join(root, "in"), Output: filepath.Join(root, "out"), Groups: []Group{{Output: "ok.vpk", Packages: []string{"input.vpk"}}}}); err != nil {
		t.Fatal(err)
	}
}
