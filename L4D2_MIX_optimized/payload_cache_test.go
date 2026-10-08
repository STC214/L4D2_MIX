package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func payloadFixture(t *testing.T) string {
	t.Helper()
	t.Setenv("LOCALAPPDATA", t.TempDir())
	t.Setenv("L4D2_MIX_HOST_ROOT", t.TempDir())
	root, err := extractPayload()
	if err != nil {
		t.Fatal(err)
	}
	return root
}
func TestPriorityPayloadDoesNotExtractOtherComponents(t *testing.T) {
	t.Setenv("LOCALAPPDATA", t.TempDir())
	t.Setenv("L4D2_MIX_HOST_ROOT", t.TempDir())
	root, err := extractPayloadGroup(true)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(filepath.Join(root, "L4D2AutobhopVPKW.exe")); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"L4D2RowFilterManager.exe", "L4D2ModJoin.exe"} {
		if _, err = os.Stat(filepath.Join(root, name)); !os.IsNotExist(err) {
			t.Fatalf("priority extracted %s: %v", name, err)
		}
	}
	if _, err = extractPayloadGroup(false); err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(filepath.Join(root, "L4D2ModJoin.exe")); err != nil {
		t.Fatal(err)
	}
}
func TestPayloadCacheRepairsMissingAndChangedFiles(t *testing.T) {
	root := payloadFixture(t)
	target := filepath.Join(root, "L4D2AutobhopVPKW.exe")
	expected, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.Remove(target); err != nil {
		t.Fatal(err)
	}
	if _, err = extractPayload(); err != nil {
		t.Fatal(err)
	}
	actual, _ := os.ReadFile(target)
	if string(actual) != string(expected) {
		t.Fatal("missing file not restored")
	}
	// Preserve size: mtime changes must invalidate the fast path.
	actual[0] ^= 0xff
	if err = os.WriteFile(target, actual, 0644); err != nil {
		t.Fatal(err)
	}
	if err = os.Chtimes(target, time.Now().Add(time.Minute), time.Now().Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if _, err = extractPayload(); err != nil {
		t.Fatal(err)
	}
	actual, _ = os.ReadFile(target)
	if string(actual) != string(expected) {
		t.Fatal("changed file not repaired")
	}
}
func TestPayloadCacheFullVerificationAndExpiry(t *testing.T) {
	for _, mode := range []string{"forced", "expired", "version", "malformed"} {
		t.Run(mode, func(t *testing.T) {
			root := payloadFixture(t)
			target := filepath.Join(root, "L4D2AutobhopVPKW.exe")
			expected, _ := os.ReadFile(target)
			info, _ := os.Stat(target)
			changed := append([]byte(nil), expected...)
			changed[0] ^= 0xff
			if err := os.WriteFile(target, changed, 0644); err != nil {
				t.Fatal(err)
			}
			if err := os.Chtimes(target, info.ModTime(), info.ModTime()); err != nil {
				t.Fatal(err)
			}
			cachePath := filepath.Join(root, ".startup-cache.json")
			data, _ := os.ReadFile(cachePath)
			var cache payloadCache
			if err := json.Unmarshal(data, &cache); err != nil {
				t.Fatal(err)
			}
			switch mode {
			case "forced":
				t.Setenv("L4D2_MIX_VERIFY_PAYLOAD", "1")
			case "expired":
				stamp := cache.Files[target]
				stamp.Verified = time.Now().Add(-25 * time.Hour).UnixNano()
				cache.Files[target] = stamp
			case "version":
				cache.Version = "previous-build"
			}
			if mode != "forced" {
				data, _ = json.Marshal(cache)
				if mode == "malformed" {
					data = []byte("broken")
				}
				if err := os.WriteFile(cachePath, data, 0644); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := extractPayload(); err != nil {
				t.Fatal(err)
			}
			actual, _ := os.ReadFile(target)
			if string(actual) != string(expected) {
				t.Fatal("full validation did not repair preserved-mtime corruption")
			}
		})
	}
}
func TestWarmPayloadDoesNotRewriteCache(t *testing.T) {
	root := payloadFixture(t)
	path := filepath.Join(root, ".startup-cache.json")
	before, _ := os.Stat(path)
	if _, err := extractPayload(); err != nil {
		t.Fatal(err)
	}
	after, _ := os.Stat(path)
	if !before.ModTime().Equal(after.ModTime()) {
		t.Fatal("warm cache needlessly rewritten")
	}
}
func TestReusablePayloadStampRejectsFutureAndOldTime(t *testing.T) {
	path := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(path, []byte("abc"), 0644); err != nil {
		t.Fatal(err)
	}
	info, _ := os.Stat(path)
	now := time.Now()
	for _, delta := range []time.Duration{-25 * time.Hour, time.Hour} {
		stamp := payloadStamp{Size: info.Size(), MTime: info.ModTime().UnixNano(), Verified: now.Add(delta).UnixNano()}
		if reusablePayloadStamp(stamp, info, now) {
			t.Fatal("invalid age accepted")
		}
	}
}
