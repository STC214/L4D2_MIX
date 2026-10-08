//go:build windows

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

var startupOrigin = time.Now()
var startupTraceMu sync.Mutex
var startupPaintOnce sync.Once

func init() {
	if ns, err := strconv.ParseInt(os.Getenv("L4D2_MIX_START_NS"), 10, 64); err == nil {
		startupOrigin = time.Unix(0, ns)
	}
}
func startupMark(phase string) {
	// Opt-in disk tracing: normal startup adds no logging I/O.
	if os.Getenv("L4D2_MIX_TRACE_STARTUP") != "1" {
		return
	}
	root := os.Getenv("L4D2_MIX_DATA_ROOT")
	if root == "" {
		root = os.Getenv("L4D2_MIX_HOST_ROOT")
	}
	if root == "" {
		exe, err := os.Executable()
		if err != nil {
			return
		}
		root = filepath.Dir(exe)
	}
	exe, _ := os.Executable()
	record := struct {
		Session      int64   `json:"session"`
		PID          int     `json:"pid"`
		Phase        string  `json:"phase"`
		Milliseconds float64 `json:"ms"`
	}{startupOrigin.UnixNano(), os.Getpid(), phase, float64(time.Since(startupOrigin).Microseconds()) / 1000}
	data, err := json.Marshal(record)
	if err != nil {
		return
	}
	startupTraceMu.Lock()
	defer startupTraceMu.Unlock()
	dir := filepath.Join(root, "data", "startup-traces")
	if os.MkdirAll(dir, 0755) != nil {
		return
	}
	f, err := os.OpenFile(filepath.Join(dir, filepath.Base(exe)+".jsonl"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = f.Write(append(data, '\n'))
}
func startupReady(hwnd uintptr) {
	key, _ := syscall.UTF16PtrFromString("L4D2MixReady")
	ret, _, _ := syscall.NewLazyDLL("user32.dll").NewProc("SetPropW").Call(hwnd, uintptr(unsafe.Pointer(key)), 1)
	if ret != 0 {
		startupMark("initialization_complete")
	}
}
func startupFirstPaint() { startupPaintOnce.Do(func() { startupMark("first_paint") }) }
