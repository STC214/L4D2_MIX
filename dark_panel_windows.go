//go:build windows

package main

import (
	"syscall"
	"unsafe"
)

const darkPanelClass = "L4D2MixDarkPanel"

// Win32 PAINTSTRUCT: HDC, BOOL, RECT, two BOOLs, and 32 reserved bytes.
type hostPaintStruct struct {
	HDC                  uintptr
	Erase                int32
	Paint                rect
	Restore, Incremental int32
	Reserved             [32]byte
}

var hostBeginPaint = user32.NewProc("BeginPaint")
var hostEndPaint = user32.NewProc("EndPaint")

func registerDarkPanelClass() bool {
	instance, _, _ := procGetModuleHandleW.Call(0)
	wc := wndClassEx{Size: uint32(unsafe.Sizeof(wndClassEx{})), WndProc: syscall.NewCallback(darkPanelWndProc), Instance: instance, Cursor: loadCursor(32512), Background: app.contentBrush, ClassName: utf16(darkPanelClass)}
	ret, _, _ := procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))
	return ret != 0
}
func darkPanelBrush(hwnd uintptr) uintptr {
	if hwnd == app.sidebar {
		return app.sidebarBrush
	}
	return app.contentBrush
}
func darkPanelWndProc(hwnd uintptr, message uint32, wParam, lParam uintptr) uintptr {
	switch message {
	case WM_COMMAND, WM_DRAWITEM:
		if hwnd == app.sidebar {
			// Buttons now have a real container parent. Keep the host's command
			// dispatch and owner-draw handlers as the single source of behavior.
			ret, _, _ := procSendMessageW.Call(app.hwnd, uintptr(message), wParam, lParam)
			return ret
		}
	case WM_ERASEBKGND:
		fillRect(wParam, clientRect(hwnd), darkPanelBrush(hwnd))
		return 1
	case 0x000F:
		var ps hostPaintStruct
		hdc, _, _ := hostBeginPaint.Call(hwnd, uintptr(unsafe.Pointer(&ps)))
		if hdc != 0 {
			fillRect(hdc, clientRect(hwnd), darkPanelBrush(hwnd))
		}
		hostEndPaint.Call(hwnd, uintptr(unsafe.Pointer(&ps)))
		return 0
	case WM_CTLCOLORSTATIC:
		// Unlike nested STATIC containers, route colours to the actual themed host.
		ret, _, _ := procSendMessageW.Call(app.hwnd, uintptr(message), wParam, lParam)
		return ret
	}
	ret, _, _ := procDefWindowProcW.Call(hwnd, uintptr(message), wParam, lParam)
	return ret
}
func paintHostWindow(hwnd uintptr) {
	var ps hostPaintStruct
	hdc, _, _ := hostBeginPaint.Call(hwnd, uintptr(unsafe.Pointer(&ps)))
	if hdc != 0 {
		paintChromeBackground(hwnd, hdc)
	}
	hostEndPaint.Call(hwnd, uintptr(unsafe.Pointer(&ps)))
}
