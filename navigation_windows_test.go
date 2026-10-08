//go:build windows && amd64

package main

import (
	"runtime"
	"syscall"
	"testing"
	"unsafe"
)

func hiddenNavigationFixture(t *testing.T) (uintptr, *int) {
	t.Helper()
	runtime.LockOSThread()
	app = appState{}
	loadingLabels = [3]uintptr{}
	var hwnd uintptr
	instance, _, _ := procGetModuleHandleW.Call(0)
	const class = "L4D2MixHiddenNavigationTest"
	t.Cleanup(func() {
		if hwnd != 0 {
			procDestroyWindow.Call(hwnd)
		}
		user32.NewProc("UnregisterClassW").Call(uintptr(unsafe.Pointer(utf16(class))), instance)
		user32.NewProc("UnregisterClassW").Call(uintptr(unsafe.Pointer(utf16(darkPanelClass))), instance)
		procDeleteObject.Call(app.font)
		procDeleteObject.Call(app.titleFont)
		cleanupTheme()
		app = appState{}
		loadingLabels = [3]uintptr{}
		runtime.UnlockOSThread()
	})
	initTheme()
	if !registerDarkPanelClass() {
		t.Fatal("dark panel registration failed")
	}
	draws := new(int)
	callback := syscall.NewCallback(func(hwnd uintptr, message uint32, wParam, lParam uintptr) uintptr {
		switch message {
		case WM_COMMAND:
			return wndProc(hwnd, message, wParam, lParam)
		case WM_DRAWITEM:
			*draws++
			return 123
		case WM_CTLCOLORSTATIC:
			return wndProc(hwnd, message, wParam, lParam)
		}
		result, _, _ := procDefWindowProcW.Call(hwnd, uintptr(message), wParam, lParam)
		return result
	})
	wc := wndClassEx{Size: uint32(unsafe.Sizeof(wndClassEx{})), WndProc: callback, Instance: instance, ClassName: utf16(class), Background: app.bgBrush}
	ok, _, _ := procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))
	if ok == 0 {
		t.Fatal("hidden host registration failed")
	}
	hwnd, _, _ = procCreateWindowExW.Call(0, uintptr(unsafe.Pointer(utf16(class))), 0, WS_OVERLAPPEDWINDOW, 0, 0, 1400, 900, 0, 0, instance, 0)
	if hwnd == 0 {
		t.Fatal("hidden host creation failed")
	}
	app.hwnd = hwnd
	app.children = [3]childApp{{name: "bhop"}, {name: "filter"}, {name: "mods"}}
	createControls(hwnd)
	layout(hwnd)
	return hwnd, draws
}

func TestNavigationPageSwitchAndRefreshPreserveDisabledState(t *testing.T) {
	host, _ := hiddenNavigationFixture(t)
	for i := range app.children {
		page := child("STATIC", "hidden component", 0, 0, 0, 200, 200, app.pageHosts[i], 200+i)
		if page == 0 {
			t.Fatal("hidden component creation failed")
		}
		app.children[i].hwnd = page
		app.children[i].ready = true
		app.children[i].enabledBeforeHide = true
	}
	for cycle := 0; cycle < 10; cycle++ {
		for _, item := range []struct {
			button uintptr
			page   int
		}{{app.filterBtn, 1}, {app.bhopBtn, 0}, {app.modsBtn, 2}} {
			procSendMessageW.Call(item.button, 0x00F5, 0, 0)
			layout(host)
			for i, component := range app.children {
				enabled, _, _ := procIsWindowEnabled.Call(component.hwnd)
				if component.active != (i == item.page) || (enabled != 0) != (i == item.page) {
					t.Fatalf("page state mismatch: cycle=%d selected=%d page=%d active=%v enabled=%d", cycle, item.page, i, component.active, enabled)
				}
			}
		}
	}
	current := app.current
	page := app.children[current].hwnd
	// A component may disable its own root while an operation is running.
	procEnableWindow.Call(page, 0)
	forceRefreshCurrentPage()
	enabled, _, _ := procIsWindowEnabled.Call(page)
	if enabled != 0 {
		t.Fatal("forced refresh re-enabled a component that disabled itself")
	}
	setPage((current + 1) % 3)
	setPage(current)
	enabled, _, _ = procIsWindowEnabled.Call(page)
	if enabled != 0 || !app.children[current].active {
		t.Fatal("switching pages lost the component's disabled state")
	}
	procEnableWindow.Call(page, 1)
	forceRefreshCurrentPage()
	enabled, _, _ = procIsWindowEnabled.Call(page)
	if enabled == 0 {
		t.Fatal("forced refresh disabled an enabled component")
	}
	visible, _, _ := user32.NewProc("IsWindowVisible").Call(host)
	if visible != 0 {
		t.Fatal("background fixture exposed its host")
	}
}

func mappedNavigationRect(t *testing.T, hwnd, parent uintptr) rect {
	t.Helper()
	var r rect
	ok, _, _ := user32.NewProc("GetWindowRect").Call(hwnd, uintptr(unsafe.Pointer(&r)))
	if ok == 0 {
		t.Fatal("GetWindowRect failed")
	}
	user32.NewProc("MapWindowPoints").Call(0, parent, uintptr(unsafe.Pointer(&r)), 2)
	return r
}

func navigationHit(hwnd uintptr, x, y int32) uintptr {
	// POINT is passed by value in the x64 Win32 ABI. No screen interaction.
	packed := uint64(uint32(x)) | uint64(uint32(y))<<32
	hit, _, _ := user32.NewProc("ChildWindowFromPointEx").Call(hwnd, uintptr(packed), 0)
	return hit
}

func TestNavigationButtonsReceiveHitTestsAndClicks(t *testing.T) {
	host, _ := hiddenNavigationFixture(t)
	for _, button := range []uintptr{app.filterBtn, app.bhopBtn, app.modsBtn} {
		r := mappedNavigationRect(t, button, host)
		hit := navigationHit(host, (r.Left+r.Right)/2, (r.Top+r.Bottom)/2)
		if hit == app.sidebar {
			r = mappedNavigationRect(t, button, app.sidebar)
			hit = navigationHit(app.sidebar, (r.Left+r.Right)/2, (r.Top+r.Bottom)/2)
		}
		if hit != button {
			t.Errorf("navigation hit blocked: button=%x actual=%x sidebar=%x", button, hit, app.sidebar)
		}
		parent, _, _ := user32.NewProc("GetParent").Call(button)
		if parent != app.sidebar {
			t.Errorf("button must belong to sidebar: parent=%x sidebar=%x", parent, app.sidebar)
		}
	}
	for cycle := 0; cycle < 10; cycle++ {
		for _, item := range []struct {
			hwnd uintptr
			page int
		}{{app.filterBtn, 1}, {app.bhopBtn, 0}, {app.modsBtn, 2}} {
			procSendMessageW.Call(item.hwnd, 0x00F5, 0, 0) // BM_CLICK, real button notification.
			if app.current != item.page {
				t.Fatalf("button click lost: current=%d want=%d cycle=%d", app.current, item.page, cycle)
			}
			enabled, _, _ := procIsWindowEnabled.Call(item.hwnd)
			if enabled == 0 {
				t.Fatal("navigation disabled by page switching")
			}
		}
	}
}

func TestNavigationOrderAndOwnerDrawForwarding(t *testing.T) {
	host, draws := hiddenNavigationFixture(t)
	filter := mappedNavigationRect(t, app.filterBtn, host)
	bhop := mappedNavigationRect(t, app.bhopBtn, host)
	mods := mappedNavigationRect(t, app.modsBtn, host)
	if filter.Top >= bhop.Top || bhop.Top >= mods.Top || mods.Top != 266 {
		t.Fatalf("unexpected order/third position: filter=%d bhop=%d mods=%d", filter.Top, bhop.Top, mods.Top)
	}
	result, _, _ := procSendMessageW.Call(app.sidebar, WM_DRAWITEM, 0, 0)
	if result != 123 || *draws != 1 {
		t.Fatalf("owner draw notification lost: result=%d draws=%d", result, *draws)
	}
}
