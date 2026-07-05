package textfile

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
)

func NormalizeEditableList(text string) string {
	out := SplitRuleLines(text)
	if len(out) == 0 {
		return ""
	}
	return strings.Join(out, "\r\n") + "\r\n"
}

func SplitRuleLines(text string) []string {
	lines := strings.Split(NormalizeNewlines(text), "\n")
	var out []string
	seen := map[string]bool{}
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key := strings.ToLower(line)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, line)
	}
	return out
}

func CountRules(text string) int {
	n := 0
	for _, line := range strings.Split(NormalizeNewlines(text), "\n") {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "#") {
			n++
		}
	}
	return n
}

func ReadTail(path string, maxBytes int64) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return "", err
	}
	start := int64(0)
	if info.Size() > maxBytes {
		start = info.Size() - maxBytes
	}
	if _, err := f.Seek(start, 0); err != nil {
		return "", err
	}
	buf := bytes.Buffer{}
	if _, err := buf.ReadFrom(f); err != nil {
		return "", err
	}
	return NormalizeNewlines(buf.String()), nil
}

func WriteTextAtomic(path, text string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, filepath.Base(path)+".tmp.")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := tmp.WriteString(text); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return replaceFile(tmpName, path)
}

func NormalizeNewlines(text string) string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	return text
}

func NewestLinesFirst(text string) string {
	lines := strings.Split(strings.TrimRight(NormalizeNewlines(text), "\n"), "\n")
	return strings.Join(ReverseStrings(lines), "\r\n")
}

func ReverseStrings(in []string) []string {
	out := make([]string, 0, len(in))
	for i := len(in) - 1; i >= 0; i-- {
		out = append(out, in[i])
	}
	return out
}
