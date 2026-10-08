//go:build windows

package main

import (
	"runtime"
	"testing"
	"unsafe"
)

func TestDarkPanelPaintsDarkInsteadOfSystemWhite(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	initTheme()
	defer cleanupTheme()
	if !registerDarkPanelClass() {
		t.Fatal("panel class registration failed")
	}
	instance, _, _ := procGetModuleHandleW.Call(0)
	defer user32.NewProc("UnregisterClassW").Call(uintptr(unsafe.Pointer(utf16(darkPanelClass))), instance)
	hwnd, _, _ := procCreateWindowExW.Call(0, uintptr(unsafe.Pointer(utf16(darkPanelClass))), 0, WS_OVERLAPPEDWINDOW, 0, 0, 320, 240, 0, 0, instance, 0)
	if hwnd == 0 {
		t.Fatal("panel creation failed")
	}
	defer procDestroyWindow.Call(hwnd)
	screen, _, _ := user32.NewProc("GetDC").Call(0)
	defer user32.NewProc("ReleaseDC").Call(0, screen)
	dc, _, _ := gdi32.NewProc("CreateCompatibleDC").Call(screen)
	defer gdi32.NewProc("DeleteDC").Call(dc)
	bitmap, _, _ := gdi32.NewProc("CreateCompatibleBitmap").Call(screen, 1, 1)
	defer procDeleteObject.Call(bitmap)
	old, _, _ := procSelectObject.Call(dc, bitmap)
	defer procSelectObject.Call(dc, old)
	if dc == 0 || bitmap == 0 {
		t.Fatal("test bitmap creation failed")
	}
	ret, _, _ := procSendMessageW.Call(hwnd, WM_ERASEBKGND, dc, 0)
	pixel, _, _ := gdi32.NewProc("GetPixel").Call(dc, 0, 0)
	var brush struct {
		Style uint32
		Color uint32
		Hatch uintptr
	}
	n, _, _ := gdi32.NewProc("GetObjectW").Call(app.contentBrush, unsafe.Sizeof(brush), uintptr(unsafe.Pointer(&brush)))
	if ret != 1 || n == 0 || pixel != uintptr(brush.Color) || pixel == 0xFFFFFF {
		t.Fatalf("unexpected erase: handled=%d pixel=%06x expected=%06x", ret, pixel, brush.Color)
	}
}
