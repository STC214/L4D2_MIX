package textfile

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNormalizeEditableListTrimsCommentsAndDuplicates(t *testing.T) {
	input := "  alpha  \r\n# comment\r\nALPHA\r\nbeta\n\r\n"
	got := NormalizeEditableList(input)
	want := "alpha\r\nbeta\r\n"
	if got != want {
		t.Fatalf("NormalizeEditableList() = %q, want %q", got, want)
	}
}

func TestReadTailNormalizesNewlines(t *testing.T) {
	path := filepath.Join(t.TempDir(), "matchmaking_row_filter.log")
	if err := os.WriteFile(path, []byte("one\r\ntwo\rthree\n"), 0644); err != nil {
		t.Fatal(err)
	}
	got, err := ReadTail(path, 1024)
	if err != nil {
		t.Fatal(err)
	}
	want := "one\ntwo\nthree\n"
	if got != want {
		t.Fatalf("ReadTail() = %q, want %q", got, want)
	}
}

func TestWriteTextAtomicReplacesExistingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "blocked_keywords.txt")
	if err := os.WriteFile(path, []byte("old"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := WriteTextAtomic(path, "new\r\n"); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "new\r\n" {
		t.Fatalf("file content = %q", data)
	}
}
