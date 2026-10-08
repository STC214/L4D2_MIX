package main

import (
	"os"
	"runtime"
	"testing"
	"time"
	"unsafe"
)

func hiddenStaticWindow(parent uintptr) uintptr {
	style := uintptr(WS_OVERLAPPEDWINDOW)
	if parent != 0 {
		style = WS_CHILD
	}
	hwnd, _, _ := procCreateWindowExW.Call(0, uintptr(unsafe.Pointer(utf16("STATIC"))), 0, style, 0, 0, 32, 32, parent, 0, 0, 0)
	return hwnd
}

func TestChildWindowMatchesRequiresPIDParentAndReady(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	parent := hiddenStaticWindow(0)
	if parent == 0 {
		t.Fatal("hidden parent creation failed")
	}
	defer procDestroyWindow.Call(parent)
	child := hiddenStaticWindow(parent)
	if child == 0 {
		t.Fatal("hidden child creation failed")
	}
	pid := uint32(os.Getpid())
	if childWindowMatches(child, parent, pid) {
		t.Fatal("unready child accepted")
	}
	ok, _, _ := user32.NewProc("SetPropW").Call(child, uintptr(unsafe.Pointer(utf16("L4D2MixReady"))), 1)
	if ok == 0 {
		t.Fatal("ready property failed")
	}
	if !childWindowMatches(child, parent, pid) {
		t.Fatal("matching child rejected")
	}
	if childWindowMatches(child, 0, pid) || childWindowMatches(child, parent, pid+1) || childWindowMatches(0, parent, pid) {
		t.Fatal("mismatched child accepted")
	}
	if result, responded := sendMessageBounded(child, WM_NULL, 100); !responded || result != 0 {
		t.Fatalf("zero message result confused with failure: %d %v", result, responded)
	}
	procDestroyWindow.Call(child)
	if childWindowMatches(child, parent, pid) {
		t.Fatal("destroyed child accepted")
	}
}

func TestSendMessageBoundedUnresponsiveThread(t *testing.T) {
	created := make(chan uintptr, 1)
	stop := make(chan struct{})
	done := make(chan struct{})
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		defer close(done)
		hwnd := hiddenStaticWindow(0)
		created <- hwnd
		<-stop // Deliberately do not pump messages; never show this window.
		if hwnd != 0 {
			procDestroyWindow.Call(hwnd)
		}
	}()
	defer func() { close(stop); <-done }()
	hwnd := <-created
	if hwnd == 0 {
		t.Fatal("hidden window creation failed")
	}
	start := time.Now()
	_, responded := sendMessageBounded(hwnd, WM_NULL, 100)
	if responded || time.Since(start) > 2*time.Second {
		t.Fatalf("unresponsive thread not bounded: response=%v elapsed=%v", responded, time.Since(start))
	}
}
