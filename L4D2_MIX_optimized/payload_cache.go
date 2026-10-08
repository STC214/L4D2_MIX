package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

type payloadFile struct {
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
}
type payloadStamp struct {
	Size     int64 `json:"size"`
	MTime    int64 `json:"mtime"`
	Verified int64 `json:"verified"`
}
type payloadCache struct {
	Version string                  `json:"version"`
	Files   map[string]payloadStamp `json:"files"`
}

var payloadExtractMu sync.Mutex

func componentRoot() (string, error) {
	base := os.Getenv("LOCALAPPDATA")
	if base == "" {
		var err error
		base, err = os.UserCacheDir()
		if err != nil {
			return "", err
		}
	}
	return filepath.Join(base, "L4D2_MIX"), nil
}
func extractPayload() (string, error) { return extractPayloadSelection(nil) }
func extractPayloadGroup(priority bool) (string, error) {
	return extractPayloadSelection(func(rel string) bool { return (rel == "L4D2AutobhopVPKW.exe") == priority })
}
func reusablePayloadStamp(stamp payloadStamp, info os.FileInfo, now time.Time) bool {
	age := now.Sub(time.Unix(0, stamp.Verified))
	return stamp.Size == info.Size() && stamp.MTime == info.ModTime().UnixNano() && stamp.Verified > 0 && age >= 0 && age < 24*time.Hour
}
func extractPayloadSelection(include func(string) bool) (string, error) {
	payloadExtractMu.Lock()
	defer payloadExtractMu.Unlock()
	root, err := componentRoot()
	if err != nil {
		return "", err
	}
	raw, err := payload.ReadFile("payload/startup-manifest.json")
	if err != nil {
		return root, err
	}
	var manifest map[string]payloadFile
	if err = json.Unmarshal(raw, &manifest); err != nil {
		return root, err
	}
	digest := sha256.Sum256(raw)
	version := hex.EncodeToString(digest[:])
	cachePath := filepath.Join(root, ".startup-cache.json")
	cache := payloadCache{Version: version, Files: map[string]payloadStamp{}}
	if old, readErr := os.ReadFile(cachePath); readErr == nil {
		var previous payloadCache
		if json.Unmarshal(old, &previous) == nil && previous.Version == version && previous.Files != nil {
			cache = previous
		}
	}
	keys := make([]string, 0, len(manifest))
	for rel := range manifest {
		keys = append(keys, rel)
	}
	sort.Strings(keys)
	mutable := map[string]bool{
		"runtime/matchmaking_row_filter_dll/blocked_keywords.txt":            true,
		"runtime/matchmaking_row_filter_dll/blocked_connectstrings.txt":      true,
		"runtime/matchmaking_row_filter_dll/learned_connectstrings.txt":      true,
		"runtime/matchmaking_row_filter_dll/auto_derived_connectstrings.txt": true,
		"runtime/matchmaking_row_filter_dll/row_filter_mode.txt":             true,
		"runtime/matchmaking_row_filter_dll/matchmaking_row_filter.log":      true,
	}
	hits, checked, written := 0, 0, 0
	now := time.Now()
	for _, rel := range keys {
		if include != nil && !include(rel) {
			continue
		}
		if !filepath.IsLocal(filepath.FromSlash(rel)) || strings.Contains(rel, "\\") {
			return root, fmt.Errorf("invalid payload path: %q", rel)
		}
		entry := manifest[rel]
		target := payloadTarget(root, filepath.Join(mainExeDir(), "data"), rel)
		info, statErr := os.Stat(target)
		if statErr == nil && mutable[rel] {
			continue
		}
		if statErr == nil && info.Mode().IsRegular() && info.Size() == entry.Size && os.Getenv("L4D2_MIX_VERIFY_PAYLOAD") != "1" && reusablePayloadStamp(cache.Files[target], info, now) {
			hits++
			continue
		}
		checked++
		valid := false
		if statErr == nil && info.Mode().IsRegular() && info.Size() == entry.Size {
			data, readErr := os.ReadFile(target)
			if readErr == nil {
				sum := sha256.Sum256(data)
				valid = hex.EncodeToString(sum[:]) == entry.SHA256
			}
		}
		if !valid {
			data, readErr := payload.ReadFile("payload/" + rel)
			if readErr != nil {
				return root, readErr
			}
			sum := sha256.Sum256(data)
			if int64(len(data)) != entry.Size || hex.EncodeToString(sum[:]) != entry.SHA256 {
				return root, fmt.Errorf("payload manifest mismatch: %s", rel)
			}
			if err = os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return root, err
			}
			if err = writeFileAtomic(target, data); err != nil {
				return root, err
			}
			info, err = os.Stat(target)
			if err != nil {
				return root, err
			}
			written++
		}
		cache.Files[target] = payloadStamp{Size: info.Size(), MTime: info.ModTime().UnixNano(), Verified: now.UnixNano()}
	}
	// Cache is an optimization, not a requirement for running the application.
	if data, marshalErr := json.Marshal(cache); checked > 0 && marshalErr == nil {
		_ = os.MkdirAll(root, 0755)
		_ = writeFileAtomic(cachePath, data)
	}
	startupMark(fmt.Sprintf("payload_cache_hit_%d_checked_%d_written_%d", hits, checked, written))
	return root, nil
}
