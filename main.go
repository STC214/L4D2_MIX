package main

import (
	"crypto/sha256"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

//go:embed payload
var payload embed.FS

const (
	appTitle = "L4D2 MIX"

	WM_CREATE         = 0x0001
	WM_NULL           = 0x0000
	WM_DESTROY        = 0x0002
	WM_SIZE           = 0x0005
	WM_CLOSE          = 0x0010
	WM_ERASEBKGND     = 0x0014
	WM_DRAWITEM       = 0x002B
	WM_GETMINMAXINFO  = 0x0024
	WM_SETFONT        = 0x0030
	WM_SETICON        = 0x0080
	WM_COMMAND        = 0x0111
	WM_SYSCOMMAND     = 0x0112
	WM_CTLCOLORSTATIC = 0x0138
	WM_LBUTTONUP      = 0x0202
	WM_LBUTTONDBLCLK  = 0x0203
	WM_RBUTTONUP      = 0x0205
	WM_APP            = 0x8000

	WM_APP_CHILD_READY  = WM_APP + 1
	WM_APP_STATUS       = WM_APP + 2
	WM_TRAY_ICON        = WM_APP + 3
	WM_APP_CLOSE_READY  = WM_APP + 4
	WM_APP_CHILD_FAILED = WM_APP + 5
	WM_MIX_CAN_CLOSE    = 0x80F0
	WM_MIX_ACTIVATE     = 0x80F1

	SC_MINIMIZE    = 0xF020
	SIZE_MINIMIZED = 1

	WS_MAXIMIZE         = 0x01000000
	WS_OVERLAPPEDWINDOW = 0x00CF0000
	WS_VISIBLE          = 0x10000000
	WS_CHILD            = 0x40000000
	WS_TABSTOP          = 0x00010000
	WS_CLIPCHILDREN     = 0x02000000
	WS_CLIPSIBLINGS     = 0x04000000
	SS_LEFT             = 0x00000000
	BS_OWNERDRAW        = 0x0000000B

	SW_HIDE     = 0
	SW_MAXIMIZE = 3
	SW_SHOW     = 5
	SW_RESTORE  = 9

	SWP_NOZORDER   = 0x0004
	SWP_SHOWWINDOW = 0x0040

	ICON_SMALL = 0
	ICON_BIG   = 1
	IMAGE_ICON = 1

	ID_PAGE_BHOP   = 101
	ID_PAGE_FILTER = 102
	ID_PAGE_MODS   = 103
	ID_TRAY_INJECT = 201
	ID_TRAY_CLEAN  = 202
	ID_TRAY_SHOW   = 203
	ID_TRAY_EXIT   = 204

	ID_ROW_FILTER_START = 1003
	ID_ROW_FILTER_CLEAN = 1010

	NIM_ADD     = 0
	NIM_DELETE  = 2
	NIF_MESSAGE = 1
	NIF_ICON    = 2
	NIF_TIP     = 4

	MF_STRING       = 0x0000
	MF_SEPARATOR    = 0x0800
	TPM_RIGHTBUTTON = 0x0002
	TPM_RETURNCMD   = 0x0100

	TRANSPARENT   = 1
	DT_LEFT       = 0x0000
	DT_CENTER     = 0x0001
	DT_VCENTER    = 0x0004
	DT_SINGLELINE = 0x0020
	ODS_SELECTED  = 0x0001

	RDW_INVALIDATE  = 0x0001
	RDW_ERASE       = 0x0004
	RDW_ALLCHILDREN = 0x0080
	RDW_UPDATENOW   = 0x0100

	MOVEFILE_REPLACE_EXISTING = 0x00000001
	MOVEFILE_WRITE_THROUGH    = 0x00000008
)

var (
	user32   = syscall.NewLazyDLL("user32.dll")
	kernel32 = syscall.NewLazyDLL("kernel32.dll")
	gdi32    = syscall.NewLazyDLL("gdi32.dll")
	shell32  = syscall.NewLazyDLL("shell32.dll")
	dwmapi   = syscall.NewLazyDLL("dwmapi.dll")

	procRegisterClassExW         = user32.NewProc("RegisterClassExW")
	procCreateWindowExW          = user32.NewProc("CreateWindowExW")
	procDefWindowProcW           = user32.NewProc("DefWindowProcW")
	procGetMessageW              = user32.NewProc("GetMessageW")
	procTranslateMessage         = user32.NewProc("TranslateMessage")
	procDispatchMessageW         = user32.NewProc("DispatchMessageW")
	procPostQuitMessage          = user32.NewProc("PostQuitMessage")
	procShowWindow               = user32.NewProc("ShowWindow")
	procUpdateWindow             = user32.NewProc("UpdateWindow")
	procDestroyWindow            = user32.NewProc("DestroyWindow")
	procSendMessageW             = user32.NewProc("SendMessageW")
	procPostMessageW             = user32.NewProc("PostMessageW")
	procSetWindowTextW           = user32.NewProc("SetWindowTextW")
	procMoveWindow               = user32.NewProc("MoveWindow")
	procGetClientRect            = user32.NewProc("GetClientRect")
	procLoadCursorW              = user32.NewProc("LoadCursorW")
	procLoadImageW               = user32.NewProc("LoadImageW")
	procEnumChildWindows         = user32.NewProc("EnumChildWindows")
	procGetWindowThreadProcessId = user32.NewProc("GetWindowThreadProcessId")
	procIsWindow                 = user32.NewProc("IsWindow")
	procIsWindowVisible          = user32.NewProc("IsWindowVisible")
	procIsWindowEnabled          = user32.NewProc("IsWindowEnabled")
	procSetWindowPos             = user32.NewProc("SetWindowPos")
	procSetForegroundWindow      = user32.NewProc("SetForegroundWindow")
	procGetCursorPos             = user32.NewProc("GetCursorPos")
	procCreatePopupMenu          = user32.NewProc("CreatePopupMenu")
	procAppendMenuW              = user32.NewProc("AppendMenuW")
	procTrackPopupMenu           = user32.NewProc("TrackPopupMenu")
	procDestroyMenu              = user32.NewProc("DestroyMenu")
	procEnableWindow             = user32.NewProc("EnableWindow")
	procRedrawWindow             = user32.NewProc("RedrawWindow")
	procFillRect                 = user32.NewProc("FillRect")
	procDrawTextW                = user32.NewProc("DrawTextW")
	procGetModuleHandleW         = kernel32.NewProc("GetModuleHandleW")
	procMoveFileExW              = kernel32.NewProc("MoveFileExW")
	procCreateSolidBrush         = gdi32.NewProc("CreateSolidBrush")
	procDeleteObject             = gdi32.NewProc("DeleteObject")
	procCreateFontW              = gdi32.NewProc("CreateFontW")
	procSetTextColor             = gdi32.NewProc("SetTextColor")
	procSetBkMode                = gdi32.NewProc("SetBkMode")
	procSelectObject             = gdi32.NewProc("SelectObject")
	procShellNotifyIconW         = shell32.NewProc("Shell_NotifyIconW")
	procIsUserAnAdmin            = shell32.NewProc("IsUserAnAdmin")
	procShellExecuteW            = shell32.NewProc("ShellExecuteW")
	procDwmSetWindowAttribute    = dwmapi.NewProc("DwmSetWindowAttribute")
)

type point struct{ X, Y int32 }
type rect struct{ Left, Top, Right, Bottom int32 }
type msg struct {
	HWnd           uintptr
	Message        uint32
	WParam, LParam uintptr
	Time           uint32
	Pt             point
}
type wndClassEx struct {
	Size                               uint32
	Style                              uint32
	WndProc                            uintptr
	ClsExtra, WndExtra                 int32
	Instance, Icon, Cursor, Background uintptr
	MenuName, ClassName                *uint16
	IconSm                             uintptr
}
type minMaxInfo struct {
	Reserved     point
	MaxSize      point
	MaxPosition  point
	MinTrackSize point
	MaxTrackSize point
}
type drawItemStruct struct {
	CtlType, CtlID, ItemID, ItemAction, ItemState uint32
	HwndItem, HDC                                 uintptr
	RcItem                                        rect
	ItemData                                      uintptr
}
type guid struct {
	Data1 uint32
	Data2 uint16
	Data3 uint16
	Data4 [8]byte
}
type notifyIconData struct {
	CbSize                        uint32
	HWnd                          uintptr
	UID, UFlags, UCallbackMessage uint32
	HIcon                         uintptr
	SzTip                         [128]uint16
	DwState, DwStateMask          uint32
	SzInfo                        [256]uint16
	UVersion                      uint32
	SzInfoTitle                   [64]uint16
	DwInfoFlags                   uint32
	GuidItem                      guid
	HBalloonIcon                  uintptr
}
type childApp struct {
	name              string
	exe               string
	process           *os.Process
	hwnd              uintptr
	ready             bool
	err               error
	enabledBeforeHide bool
	active            bool
	done              chan struct{}
}
type appState struct {
	hwnd                                                         uintptr
	headerTitle, headerSub, status                               uintptr
	sidebar, content                                             uintptr
	pageHosts                                                    [3]uintptr
	bhopBtn, filterBtn, modsBtn                                  uintptr
	font, titleFont                                              uintptr
	iconBig, iconSmall                                           uintptr
	bgBrush, headerBrush, sidebarBrush, contentBrush             uintptr
	cardBrush, cardHoverBrush, accentBrush, accentAltBrush       uintptr
	navBhopBrush, navBhopSelectedBrush, navBhopAccentBrush       uintptr
	navFilterBrush, navFilterSelectedBrush, navFilterAccentBrush uintptr
	navModsBrush, navModsSelectedBrush, navModsAccentBrush       uintptr
	statusBrush                                                  uintptr
	current                                                      int
	trayVisible                                                  bool
	children                                                     [3]childApp
	mu                                                           sync.Mutex
	pendingStatuses                                              []string
	closing                                                      bool
}

var loadingLabels [3]uintptr
var app appState

func main() {
	startupMark("main")
	runtime.LockOSThread()
	if !isAdmin() {
		relaunchAsAdmin()
		return
	}
	initTheme()
	defer cleanupTheme()
	runUI()
}

func isAdmin() bool {
	ret, _, _ := procIsUserAnAdmin.Call()
	return ret != 0
}

func relaunchAsAdmin() {
	exe, err := os.Executable()
	if err != nil {
		return
	}
	procShellExecuteW.Call(
		0,
		uintptr(unsafe.Pointer(utf16("runas"))),
		uintptr(unsafe.Pointer(utf16(exe))),
		0,
		uintptr(unsafe.Pointer(utf16(filepath.Dir(exe)))),
		SW_SHOW,
	)
}

func initTheme() {
	app.bgBrush = solid(rgb(10, 14, 20))
	app.headerBrush = solid(rgb(13, 18, 27))
	app.sidebarBrush = solid(rgb(16, 23, 32))
	app.contentBrush = solid(rgb(20, 26, 35))
	app.cardBrush = solid(rgb(29, 38, 50))
	app.cardHoverBrush = solid(rgb(36, 48, 64))
	app.statusBrush = solid(rgb(22, 32, 43))
	app.accentBrush = solid(rgb(65, 177, 214))
	app.accentAltBrush = solid(rgb(120, 194, 132))
	app.navBhopBrush = solid(rgb(27, 46, 62))
	app.navBhopSelectedBrush = solid(rgb(34, 78, 104))
	app.navBhopAccentBrush = solid(rgb(70, 198, 235))
	app.navFilterBrush = solid(rgb(28, 48, 42))
	app.navFilterSelectedBrush = solid(rgb(36, 82, 65))
	app.navFilterAccentBrush = solid(rgb(114, 211, 145))
	app.navModsBrush = solid(rgb(47, 36, 61))
	app.navModsSelectedBrush = solid(rgb(82, 54, 104))
	app.navModsAccentBrush = solid(rgb(202, 139, 255))
}

func cleanupTheme() {
	for _, h := range []uintptr{
		app.bgBrush, app.headerBrush, app.sidebarBrush, app.contentBrush,
		app.cardBrush, app.cardHoverBrush, app.statusBrush,
		app.accentBrush, app.accentAltBrush,
		app.navBhopBrush, app.navBhopSelectedBrush, app.navBhopAccentBrush,
		app.navFilterBrush, app.navFilterSelectedBrush, app.navFilterAccentBrush,
		app.navModsBrush, app.navModsSelectedBrush, app.navModsAccentBrush,
		app.font, app.titleFont,
	} {
		if h != 0 {
			procDeleteObject.Call(h)
		}
	}
}

func runUI() {
	if !registerDarkPanelClass() {
		return
	}
	hinst, _, _ := procGetModuleHandleW.Call(0)
	app.iconBig = loadIcon(hinst, 256)
	app.iconSmall = loadIcon(hinst, 32)
	className := utf16("L4D2MixHostWindow")
	wc := wndClassEx{
		Size:       uint32(unsafe.Sizeof(wndClassEx{})),
		WndProc:    syscall.NewCallback(wndProc),
		Instance:   hinst,
		Icon:       app.iconBig,
		Cursor:     loadCursor(32512),
		Background: app.bgBrush,
		ClassName:  className,
		IconSm:     app.iconSmall,
	}
	if ret, _, _ := procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc))); ret == 0 {
		return
	}
	hwnd, _, _ := procCreateWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(className)),
		uintptr(unsafe.Pointer(utf16(appTitle+" - 连跳、服务器过滤与 MOD 合并"))),
		WS_OVERLAPPEDWINDOW|WS_CLIPCHILDREN|WS_MAXIMIZE,
		80, 55, 1320, 860,
		0, 0, hinst, 0,
	)
	app.hwnd = hwnd
	enableDarkTitleBar(hwnd)
	procSendMessageW.Call(hwnd, WM_SETICON, ICON_BIG, app.iconBig)
	procSendMessageW.Call(hwnd, WM_SETICON, ICON_SMALL, app.iconSmall)
	procShowWindow.Call(hwnd, SW_MAXIMIZE)
	procUpdateWindow.Call(hwnd)
	startupMark("host_first_paint")

	var m msg
	for {
		ret, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		if int32(ret) <= 0 {
			break
		}
		procTranslateMessage.Call(uintptr(unsafe.Pointer(&m)))
		procDispatchMessageW.Call(uintptr(unsafe.Pointer(&m)))
	}
}

func wndProc(hwnd uintptr, message uint32, wParam, lParam uintptr) uintptr {
	switch message {
	case WM_CREATE:
		app.children = [3]childApp{{name: "连跳辅助"}, {name: "组服务器过滤器"}, {name: "MOD 分类合并"}}
		app.hwnd = hwnd
		createControls(hwnd)
		layout(hwnd)
		setPage(0)
		addTrayIcon()
		go prepareAndLaunchChildren()
		return 0
	case WM_COMMAND:
		switch int(wParam & 0xffff) {
		case ID_PAGE_BHOP:
			setPage(0)
		case ID_PAGE_FILTER:
			setPage(1)
		case ID_PAGE_MODS:
			setPage(2)
		}
		return 0
	case WM_DRAWITEM:
		drawOwnerButton((*drawItemStruct)(unsafe.Pointer(lParam)))
		return 1
	case WM_SIZE:
		if wParam == SIZE_MINIMIZED {
			minimizeToTray()
			return 0
		}
		layout(hwnd)
		return 0
	case WM_GETMINMAXINFO:
		info := (*minMaxInfo)(unsafe.Pointer(lParam))
		info.MinTrackSize = point{X: 1200, Y: 810}
		return 0
	case WM_SYSCOMMAND:
		if wParam&0xfff0 == SC_MINIMIZE {
			minimizeToTray()
			return 0
		}
	case WM_APP_CHILD_READY:
		attachChild(int(wParam), lParam)
		return 0
	case WM_APP_CHILD_FAILED:
		index := int(wParam)
		if index >= 0 && index < len(app.children) {
			app.mu.Lock()
			name, err := app.children[index].name, app.children[index].err
			app.mu.Unlock()
			if err != nil {
				text := name + "加载失败：" + err.Error()
				procSetWindowTextW.Call(loadingLabels[index], uintptr(unsafe.Pointer(utf16(text))))
				if index == app.current {
					procShowWindow.Call(loadingLabels[index], SW_SHOW)
				}
				if index == app.current {
					setStatus(text)
				}
			}
		}
		return 0
	case WM_APP_STATUS:
		drainStatus()
		return 0
	case WM_APP_CLOSE_READY:
		deleteTrayIcon()
		procDestroyWindow.Call(hwnd)
		return 0
	case WM_TRAY_ICON:
		switch uint32(lParam) {
		case WM_LBUTTONUP, WM_LBUTTONDBLCLK:
			restoreFromTray()
		case WM_RBUTTONUP:
			showTrayMenu()
		}
		return 0
	case 0x000F:
		paintHostWindow(hwnd)
		return 0
	case WM_ERASEBKGND:
		paintChromeBackground(hwnd, wParam)
		return 1
	case WM_CTLCOLORSTATIC:
		procSetBkMode.Call(wParam, TRANSPARENT)
		procSetTextColor.Call(wParam, uintptr(rgb(228, 234, 242)))
		switch lParam {
		case app.sidebar:
			return app.sidebarBrush
		case app.content:
			return app.contentBrush
		}
		for _, host := range app.pageHosts {
			if lParam == host {
				return app.contentBrush
			}
		}
		for _, label := range loadingLabels {
			if lParam == label {
				return app.contentBrush
			}
		}
		if lParam == app.status {
			return app.statusBrush
		}
		return app.bgBrush
	case WM_CLOSE:
		requestClose()
		return 0
	case WM_DESTROY:
		deleteTrayIcon()
		procPostQuitMessage.Call(0)
		return 0
	}
	ret, _, _ := procDefWindowProcW.Call(hwnd, uintptr(message), wParam, lParam)
	return ret
}

func createControls(hwnd uintptr) {
	app.font = createFont("Microsoft YaHei UI", -18, 400)
	app.titleFont = createFont("Microsoft YaHei UI", -30, 700)

	app.headerTitle = child("STATIC", "L4D2 MIX", SS_LEFT, 28, 16, 300, 38, hwnd, 0)
	setFont(app.headerTitle, app.titleFont)
	app.headerSub = child("STATIC", "组服务器过滤 + 连跳辅助 + MOD 分类合并  /  一站式控制台", SS_LEFT, 28, 57, 760, 26, hwnd, 0)
	app.status = child("STATIC", "正在准备内置组件…", SS_LEFT, 0, 25, 500, 28, hwnd, 0)

	app.sidebar = child(darkPanelClass, "", WS_CLIPCHILDREN|WS_CLIPSIBLINGS, 0, 96, 232, 700, hwnd, 0)
	app.content = child(darkPanelClass, "", SS_LEFT|WS_CLIPCHILDREN|WS_CLIPSIBLINGS, 248, 106, 1040, 700, hwnd, 0)
	app.pageHosts[0] = child(darkPanelClass, "", SS_LEFT|WS_CLIPCHILDREN|WS_CLIPSIBLINGS, 0, 0, 1040, 700, app.content, 0)
	app.pageHosts[1] = child(darkPanelClass, "", SS_LEFT|WS_CLIPCHILDREN|WS_CLIPSIBLINGS, 0, 0, 1040, 700, app.content, 0)
	app.pageHosts[2] = child(darkPanelClass, "", SS_LEFT|WS_CLIPCHILDREN|WS_CLIPSIBLINGS, 0, 0, 1040, 700, app.content, 0)
	for i := range app.pageHosts {
		loadingLabels[i] = child("STATIC", "正在加载"+app.children[i].name+"…", SS_LEFT, 280, 138, 760, 40, hwnd, 0)
		setFont(loadingLabels[i], app.font)
	}
	// Navigation belongs to the sidebar, not to the host as an overlapping
	// sibling. A normal dark panel claims HTCLIENT and would intercept hits.
	app.filterBtn = child("BUTTON", "组服务器过滤", BS_OWNERDRAW|WS_TABSTOP, 20, 34, 192, 56, app.sidebar, ID_PAGE_FILTER)
	app.bhopBtn = child("BUTTON", "连跳辅助", BS_OWNERDRAW|WS_TABSTOP, 20, 102, 192, 56, app.sidebar, ID_PAGE_BHOP)
	app.modsBtn = child("BUTTON", "MOD 分类合并", BS_OWNERDRAW|WS_TABSTOP, 20, 170, 192, 56, app.sidebar, ID_PAGE_MODS)
}

func layout(hwnd uintptr) {
	r := clientRect(hwnd)
	w, h := r.Right-r.Left, r.Bottom-r.Top
	if w <= 0 || h <= 0 {
		return
	}
	move(app.headerTitle, 28, 14, 300, 40)
	move(app.headerSub, 28, 57, 760, 26)
	move(app.status, w-540, 28, 508, 30)
	move(app.sidebar, 0, 96, 232, h-96)
	// Coordinates are relative to the sidebar (host y = sidebar y + button y).
	move(app.filterBtn, 20, 34, 192, 56)
	move(app.bhopBtn, 20, 102, 192, 56)
	move(app.modsBtn, 20, 170, 192, 56)
	move(app.content, 248, 106, w-276, h-128)
	for _, label := range loadingLabels {
		move(label, 280, 138, w-340, 40)
	}
	for _, host := range app.pageHosts {
		move(host, 0, 0, w-276, h-128)
	}
	for i := range app.children {
		app.mu.Lock()
		childHwnd := app.children[i].hwnd
		app.mu.Unlock()
		if childHwnd != 0 {
			if i == app.current {
				activatePage(i, w-276, h-128)
			} else {
				deactivatePage(i)
			}
		}
	}
}

func setPage(index int) {
	if index < 0 || index >= len(app.children) {
		return
	}
	app.current = index
	r := clientRect(app.content)
	pageW, pageH := r.Right-r.Left, r.Bottom-r.Top

	// First disable and remove every inactive page. Hiding the whole page host
	// exposes the host's solid background, which acts as an opaque page mask.
	for i := range app.children {
		if i != index {
			deactivatePage(i)
		}
	}
	for i, host := range app.pageHosts {
		if i != index {
			procShowWindow.Call(host, SW_HIDE)
		}
	}

	// Erase the previous page before exposing the selected tab.
	procRedrawWindow.Call(app.content, 0, 0, RDW_INVALIDATE|RDW_ERASE|RDW_ALLCHILDREN|RDW_UPDATENOW)
	activatePage(index, pageW, pageH)
	invalidate(app.bhopBtn)
	invalidate(app.filterBtn)
	invalidate(app.modsBtn)
	app.mu.Lock()
	ready := app.children[index].ready
	name := app.children[index].name
	childErr := app.children[index].err
	app.mu.Unlock()
	for i, label := range loadingLabels {
		if i == index && !ready {
			procSetWindowPos.Call(label, 0, 0, 0, 0, 0, 0x0001|0x0002|SWP_SHOWWINDOW)
		} else {
			procShowWindow.Call(label, SW_HIDE)
		}
	}
	if childErr != nil {
		setStatus(name + "加载失败：" + childErr.Error())
	} else if ready {
		setStatus(name + "已就绪")
	} else {
		setStatus("正在加载" + name + "…")
	}
}

func prepareAndLaunchChildren() {
	startupMark("prepare_begin")
	root, err := extractPayloadGroup(true)
	if err != nil {
		setChildError(0, err)
	}
	app.mu.Lock()
	for i, exe := range []string{"L4D2AutobhopVPKW.exe", "L4D2RowFilterManager.exe", "L4D2ModJoin.exe"} {
		app.children[i].exe = filepath.Join(root, exe)
	}
	closing := app.closing
	app.mu.Unlock()
	if closing {
		return
	}
	type preparationResult struct {
		err     error
		summary string
	}
	prepared := make(chan preparationResult, 1)
	var prepareOnce sync.Once
	prepareRemaining := func() {
		prepareOnce.Do(func() {
			go func() {
				_, err := extractPayloadGroup(false)
				summary := ""
				if err == nil {
					startupMark("remaining_payload_ready")
					summary = importLegacyModJoinState(filepath.Join(mainExeDir(), "data", "mod-join"))
				}
				prepared <- preparationResult{err: err, summary: summary}
			}()
		})
	}
	if err == nil {
		startupMark("priority_payload_ready")
		// Launch the default loader first, then overlap remaining disk work
		// with its initialization; don't serialize both waits.
		launchChild(0, prepareRemaining)
	}
	prepareRemaining() // Also run if default extraction/process creation failed.
	app.mu.Lock()
	closing = app.closing
	app.mu.Unlock()
	if closing {
		return
	}
	result := <-prepared
	if result.err != nil {
		setChildError(1, result.err)
		setChildError(2, result.err)
		return
	}
	if result.summary != "" {
		setStatusAsync(result.summary)
	}
	for i := 1; i < len(app.children); i++ {
		go launchChild(i, nil)
	}

}

func launchChild(index int, afterStart func()) {
	app.mu.Lock()
	exe := app.children[index].exe
	parent := app.pageHosts[index]
	if app.closing {
		app.mu.Unlock()
		return
	}
	app.mu.Unlock()

	startupMark(fmt.Sprintf("child_%d_start_begin", index))
	cmd := exec.Command(exe)
	cmd.Dir = filepath.Dir(exe)
	cmd.Env = append(
		os.Environ(),
		"L4D2_MIX_START_NS="+strconv.FormatInt(startupOrigin.UnixNano(), 10),
		"L4D2_MIX_PARENT="+strconv.FormatUint(uint64(parent), 10),
		"L4D2_MIX_DATA_ROOT="+mainExeDir(),
		"L4D2_MIX_ROW_FILTER_DIR="+filepath.Join(mainExeDir(), "data", "row-filter"),
		"L4D2_MIX_MOD_JOIN_ROOT="+mainExeDir(),
		"L4D2_MIX_MOD_JOIN_STATE_DIR="+filepath.Join(mainExeDir(), "data", "mod-join"),
	)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if err := cmd.Start(); err != nil {
		setChildError(index, err)
		return
	}
	startupMark(fmt.Sprintf("child_%d_process_started", index))
	done := make(chan struct{})
	app.mu.Lock()
	if app.closing {
		app.mu.Unlock()
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		return
	}
	app.children[index].process = cmd.Process
	app.children[index].done = done
	app.mu.Unlock()
	if afterStart != nil {
		afterStart()
	}
	go func() {
		_ = cmd.Wait()
		app.mu.Lock()
		if app.children[index].process == cmd.Process {
			app.children[index].process = nil
			app.children[index].hwnd = 0
			app.children[index].ready = false
		}
		closing := app.closing
		close(done)
		app.mu.Unlock()
		if !closing {
			setChildError(index, errors.New("组件进程已退出"))
		}
	}()
	hwnd := waitForProcessWindow(parent, uint32(cmd.Process.Pid), 20*time.Second, done)
	if hwnd == 0 {
		app.mu.Lock()
		closing := app.closing
		app.mu.Unlock()
		if closing {
			return
		}
		failure := errors.New("等待初始化完成超时")
		select {
		case <-done:
			failure = errors.New("进程在初始化完成前退出")
		default:
		}
		_ = cmd.Process.Kill()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
		}
		setChildError(index, failure)
		return
	}

	startupMark(fmt.Sprintf("child_%d_ready", index))
	procPostMessageW.Call(app.hwnd, WM_APP_CHILD_READY, uintptr(index), hwnd)
}

func mainExeDir() string {
	if root := strings.TrimSpace(os.Getenv("L4D2_MIX_HOST_ROOT")); root != "" {
		return filepath.Clean(root)
	}
	exe, err := os.Executable()
	if err == nil && exe != "" {
		return filepath.Dir(exe)
	}
	return "."
}

type legacyImportReport struct {
	Version   int      `json:"version"`
	Source    string   `json:"source,omitempty"`
	Imported  []string `json:"imported,omitempty"`
	Preserved []string `json:"preserved,omitempty"`
	Skipped   []string `json:"skipped,omitempty"`
	Error     string   `json:"error,omitempty"`
}

func importLegacyModJoinState(destination string) string {
	candidates := []string{}
	if configured := strings.TrimSpace(os.Getenv("L4D2_MIX_LEGACY_MOD_JOIN_DATA")); configured != "" {
		candidates = append(candidates, configured)
	}
	candidates = append(candidates,
		filepath.Join(mainExeDir(), "..", "..", "L4D2_MOD_JOIN", "dist", "data"),
		`F:\Project\03_Game_Tools\L4D2_MOD_JOIN\dist\data`,
	)
	report := importLegacyModJoinStateFrom(destination, candidates)
	if report.Source == "" {
		return ""
	}
	if report.Error != "" {
		return "MOD 独立版状态导入未完全成功：" + report.Error
	}
	if len(report.Imported) > 0 {
		return fmt.Sprintf("已从独立版导入 %d 个 MOD 状态文件。", len(report.Imported))
	}
	return ""
}

func importLegacyModJoinStateFrom(destination string, candidates []string) legacyImportReport {
	reportPath := filepath.Join(destination, "legacy-import-v1.json")
	if data, err := os.ReadFile(reportPath); err == nil {
		var previous legacyImportReport
		if json.Unmarshal(data, &previous) == nil && previous.Version == 1 {
			return previous
		}
	}

	var source string
	for _, candidate := range candidates {
		candidate = filepath.Clean(candidate)
		if samePath(candidate, destination) {
			continue
		}
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			source = candidate
			break
		}
	}
	report := legacyImportReport{Version: 1, Source: source}
	if source == "" {
		return report
	}
	if err := os.MkdirAll(destination, 0755); err != nil {
		report.Error = err.Error()
		return report
	}

	names := []string{
		"mod-scan-report.json",
		"mod-conflict-policy.json",
		"l4d2modjoin-build.json",
		".l4d2modjoin-deployment.json",
		"l4d2modjoin-settings.json",
	}
	for _, name := range names {
		src := filepath.Join(source, name)
		srcData, err := os.ReadFile(src)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			report.Error = appendError(report.Error, name+": "+err.Error())
			continue
		}
		dst := filepath.Join(destination, name)
		if dstData, err := os.ReadFile(dst); err == nil {
			if sha256.Sum256(dstData) == sha256.Sum256(srcData) {
				report.Skipped = append(report.Skipped, name)
				continue
			}
			preserveDir := filepath.Join(destination, "legacy-import")
			if err := os.MkdirAll(preserveDir, 0755); err != nil {
				report.Error = appendError(report.Error, name+": "+err.Error())
				continue
			}
			if err := writeFileAtomic(filepath.Join(preserveDir, name), srcData); err != nil {
				report.Error = appendError(report.Error, name+": "+err.Error())
				continue
			}
			report.Preserved = append(report.Preserved, name)
			continue
		}
		if err := writeFileAtomic(dst, srcData); err != nil {
			report.Error = appendError(report.Error, name+": "+err.Error())
			continue
		}
		report.Imported = append(report.Imported, name)
	}
	reportData, _ := json.MarshalIndent(report, "", "  ")
	_ = writeFileAtomic(reportPath, append(reportData, '\n'))
	return report
}

func appendError(current, next string) string {
	if current == "" {
		return next
	}
	return current + "; " + next
}

func samePath(a, b string) bool {
	aa, _ := filepath.Abs(a)
	bb, _ := filepath.Abs(b)
	return strings.EqualFold(filepath.Clean(aa), filepath.Clean(bb))
}

func writeFileAtomic(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := tmp.Write(data); err != nil {
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

func attachChild(index int, hwnd uintptr) {
	if index < 0 || index >= len(app.children) || hwnd == 0 {
		return
	}
	app.mu.Lock()
	if app.children[index].process == nil || !childWindowMatches(hwnd, app.pageHosts[index], uint32(app.children[index].process.Pid)) {
		app.mu.Unlock()
		return
	}
	if app.closing {
		app.mu.Unlock()
		procPostMessageW.Call(hwnd, WM_CLOSE, 0, 0)
		return
	}
	app.children[index].hwnd = hwnd
	app.children[index].ready = true
	app.children[index].enabledBeforeHide = true
	app.mu.Unlock()
	procShowWindow.Call(loadingLabels[index], SW_HIDE)
	layout(app.hwnd)
	setPage(app.current)
	user32.NewProc("SetPropW").Call(hwnd, uintptr(unsafe.Pointer(utf16("L4D2MixAttached"))), 1)
	startupMark(fmt.Sprintf("child_%d_attached", index))
}

func deactivatePage(index int) {
	if index < 0 || index >= len(app.children) {
		return
	}
	app.mu.Lock()
	hwnd := app.children[index].hwnd
	wasActive := app.children[index].active
	app.mu.Unlock()
	if hwnd != 0 {
		// Disabling the page root makes every child control non-interactive
		// without overwriting controls that the component disabled itself.
		if wasActive {
			enabled, _, _ := procIsWindowEnabled.Call(hwnd)
			app.mu.Lock()
			app.children[index].enabledBeforeHide = enabled != 0
			app.mu.Unlock()
		}
		procEnableWindow.Call(hwnd, 0)
		procShowWindow.Call(hwnd, SW_HIDE)
		app.mu.Lock()
		app.children[index].active = false
		app.mu.Unlock()
	}
	if app.pageHosts[index] != 0 {
		procShowWindow.Call(app.pageHosts[index], SW_HIDE)
	}
}

func activatePage(index int, width, height int32) {
	if index < 0 || index >= len(app.children) {
		return
	}
	if app.pageHosts[index] != 0 {
		procShowWindow.Call(app.pageHosts[index], SW_SHOW)
	}
	app.mu.Lock()
	hwnd := app.children[index].hwnd
	wasActive := app.children[index].active
	enabledBeforeHide := app.children[index].enabledBeforeHide
	app.mu.Unlock()
	if hwnd == 0 {
		return
	}
	procSetWindowPos.Call(
		hwnd,
		0,
		0,
		0,
		uintptr(width),
		uintptr(height),
		SWP_NOZORDER,
	)
	if !wasActive {
		if enabledBeforeHide {
			procEnableWindow.Call(hwnd, 1)
		} else {
			procEnableWindow.Call(hwnd, 0)
		}
	}
	procShowWindow.Call(hwnd, SW_SHOW)
	if !wasActive {
		procRedrawWindow.Call(hwnd, 0, 0, RDW_INVALIDATE|RDW_ERASE|RDW_ALLCHILDREN|RDW_UPDATENOW)
	}
	app.mu.Lock()
	app.children[index].active = true
	app.mu.Unlock()
	if index == 2 {
		procPostMessageW.Call(hwnd, WM_MIX_ACTIVATE, 0, 0)
	}
}

func waitForProcessWindow(parent uintptr, pid uint32, timeout time.Duration, done <-chan struct{}) uintptr {
	deadline := time.Now().Add(timeout)
	var found uintptr
	getParent := user32.NewProc("GetParent")
	getProp := user32.NewProc("GetPropW")
	readyName := utf16("L4D2MixReady")
	// Allocate one callback per launch, not one permanent Go callback every poll.
	callback := syscall.NewCallback(func(hwnd, _ uintptr) uintptr {
		var windowPID uint32
		procGetWindowThreadProcessId.Call(hwnd, uintptr(unsafe.Pointer(&windowPID)))
		owner, _, _ := getParent.Call(hwnd)
		if windowPID == pid && owner == parent {
			ready, _, _ := getProp.Call(hwnd, uintptr(unsafe.Pointer(readyName)))
			if ready != 0 {
				found = hwnd
				return 0
			}
		}
		return 1
	})
	for time.Now().Before(deadline) {
		select {
		case <-done:
			return 0
		default:
		}
		app.mu.Lock()
		closing := app.closing
		app.mu.Unlock()
		if closing {
			return 0
		}
		found = 0
		procEnumChildWindows.Call(parent, callback, 0)
		if found != 0 {
			return found
		}
		time.Sleep(10 * time.Millisecond)
	}
	return 0
}

func requestClose() {
	app.mu.Lock()
	if app.closing {
		app.mu.Unlock()
		return
	}
	modHwnd := app.children[2].hwnd
	app.mu.Unlock()

	if modHwnd != 0 {
		canClose, responded := sendMessageBounded(modHwnd, WM_MIX_CAN_CLOSE, 1000)
		if !responded {
			setStatus("MOD 组件暂未响应关闭检查，请稍后重试；当前任务保持运行。")
			return
		}
		if canClose == 0 {
			visible, _, _ := procIsWindowVisible.Call(app.hwnd)
			if visible == 0 {
				restoreFromTray()
			}
			setPage(2)
			setStatus("MOD 分类合并任务或冲突处理仍在进行，请完成后再关闭。")
			return
		}
	}

	app.mu.Lock()
	app.closing = true
	type closingChild struct {
		hwnd    uintptr
		process *os.Process
		done    chan struct{}
	}
	children := make([]closingChild, len(app.children))
	for i := range app.children {
		children[i] = closingChild{
			hwnd: app.children[i].hwnd, process: app.children[i].process, done: app.children[i].done,
		}
	}
	app.mu.Unlock()

	setStatus("正在安全关闭所有功能组件…")
	for _, child := range children {
		if child.hwnd != 0 {
			procPostMessageW.Call(child.hwnd, WM_CLOSE, 0, 0)
		} else if child.process != nil {
			_ = child.process.Kill()
		}
	}
	go func() {
		for _, child := range children {
			if child.done == nil {
				continue
			}
			select {
			case <-child.done:
			case <-time.After(5 * time.Second):
				if child.process != nil {
					_ = child.process.Kill()
				}
				select {
				case <-child.done:
				case <-time.After(2 * time.Second):
				}
			}
		}
		procPostMessageW.Call(app.hwnd, WM_APP_CLOSE_READY, 0, 0)
	}()
}

func replaceFile(source, destination string) error {
	src := utf16(source)
	dst := utf16(destination)
	ret, _, callErr := procMoveFileExW.Call(
		uintptr(unsafe.Pointer(src)),
		uintptr(unsafe.Pointer(dst)),
		MOVEFILE_REPLACE_EXISTING|MOVEFILE_WRITE_THROUGH,
	)
	if ret == 0 {
		if callErr != syscall.Errno(0) {
			return callErr
		}
		return errors.New("MoveFileExW failed")
	}
	return nil
}

func payloadTarget(componentRoot, dataRoot, rel string) string {
	const rowFilterPrefix = "runtime/matchmaking_row_filter_dll/"
	const loaderPrefix = "runtime/matchmaking_probe_loader/"
	switch {
	case strings.HasPrefix(rel, rowFilterPrefix):
		return filepath.Join(dataRoot, "row-filter", filepath.FromSlash(strings.TrimPrefix(rel, rowFilterPrefix)))
	case strings.HasPrefix(rel, loaderPrefix):
		return filepath.Join(dataRoot, "matchmaking_probe_loader", filepath.FromSlash(strings.TrimPrefix(rel, loaderPrefix)))
	default:
		return filepath.Join(componentRoot, filepath.FromSlash(rel))
	}
}

func sameFileContent(path string, data []byte) bool {
	current, err := os.ReadFile(path)
	if err != nil || len(current) != len(data) {
		return false
	}
	return sha256.Sum256(current) == sha256.Sum256(data)
}

func drawOwnerButton(item *drawItemStruct) {
	if item == nil {
		return
	}
	selected := (item.CtlID == ID_PAGE_BHOP && app.current == 0) ||
		(item.CtlID == ID_PAGE_FILTER && app.current == 1) ||
		(item.CtlID == ID_PAGE_MODS && app.current == 2)
	brush, selectedBrush, accentBrush, text := navButtonStyle(item.CtlID)
	if selected {
		brush = selectedBrush
	}
	if item.ItemState&ODS_SELECTED != 0 {
		brush = app.cardHoverBrush
	}
	fillRect(item.HDC, item.RcItem, brush)
	highlight := item.RcItem
	highlight.Bottom = highlight.Top + 1
	highlight.Left += 8
	highlight.Right -= 8
	fillRect(item.HDC, highlight, app.cardHoverBrush)
	if selected {
		strip := item.RcItem
		strip.Right = strip.Left + 6
		fillRect(item.HDC, strip, accentBrush)
		glow := item.RcItem
		glow.Left += 6
		glow.Right = glow.Left + 2
		fillRect(item.HDC, glow, accentBrush)
	}
	procSetBkMode.Call(item.HDC, TRANSPARENT)
	procSetTextColor.Call(item.HDC, uintptr(rgb(238, 244, 252)))
	procSelectObject.Call(item.HDC, app.font)
	p := utf16(text)
	r := item.RcItem
	r.Left += 14
	procDrawTextW.Call(item.HDC, uintptr(unsafe.Pointer(p)), ^uintptr(0), uintptr(unsafe.Pointer(&r)), DT_LEFT|DT_VCENTER|DT_SINGLELINE)
}

func navButtonStyle(id uint32) (brush, selectedBrush, accentBrush uintptr, text string) {
	switch id {
	case ID_PAGE_FILTER:
		return app.navFilterBrush, app.navFilterSelectedBrush, app.navFilterAccentBrush, "  ◇  组服务器过滤"
	case ID_PAGE_MODS:
		return app.navModsBrush, app.navModsSelectedBrush, app.navModsAccentBrush, "  ✦  MOD 分类合并"
	default:
		return app.navBhopBrush, app.navBhopSelectedBrush, app.navBhopAccentBrush, "  ⚡  连跳辅助"
	}
}

func paintChromeBackground(hwnd, hdc uintptr) {
	r := clientRect(hwnd)
	fillRect(hdc, r, app.bgBrush)
	header := rect{Left: 0, Top: 0, Right: r.Right, Bottom: 96}
	fillRect(hdc, header, app.headerBrush)
	sidebar := rect{Left: 0, Top: 96, Right: 232, Bottom: r.Bottom}
	fillRect(hdc, sidebar, app.sidebarBrush)
	content := rect{Left: 232, Top: 96, Right: r.Right, Bottom: r.Bottom}
	fillRect(hdc, content, app.bgBrush)
	status := rect{Left: r.Right - 552, Top: 22, Right: r.Right - 24, Bottom: 66}
	if status.Left < 820 {
		status.Left = 820
	}
	fillRect(hdc, status, app.statusBrush)
	accent := rect{Left: 0, Top: 0, Right: r.Right, Bottom: 3}
	fillRect(hdc, accent, app.accentBrush)
	sideAccent := rect{Left: 232, Top: 96, Right: 236, Bottom: r.Bottom}
	fillRect(hdc, sideAccent, app.accentAltBrush)
}

func enableDarkTitleBar(hwnd uintptr) {
	enabled := int32(1)
	if result, _, _ := procDwmSetWindowAttribute.Call(hwnd, 20, uintptr(unsafe.Pointer(&enabled)), unsafe.Sizeof(enabled)); int32(result) != 0 {
		procDwmSetWindowAttribute.Call(hwnd, 19, uintptr(unsafe.Pointer(&enabled)), unsafe.Sizeof(enabled))
	}
}

func setStatusAsync(text string) {
	app.mu.Lock()
	app.pendingStatuses = append(app.pendingStatuses, text)
	app.mu.Unlock()
	procPostMessageW.Call(app.hwnd, WM_APP_STATUS, 0, 0)
}

func drainStatus() {
	app.mu.Lock()
	statuses := append([]string(nil), app.pendingStatuses...)
	app.pendingStatuses = nil
	app.mu.Unlock()
	text := chooseStatus(statuses)
	if text != "" {
		setStatus(text)
	}
}

func chooseStatus(statuses []string) string {
	if len(statuses) == 0 {
		return ""
	}
	for i := len(statuses) - 1; i >= 0; i-- {
		if isImportantStatus(statuses[i]) {
			return statuses[i]
		}
	}
	return statuses[len(statuses)-1]
}

func isImportantStatus(text string) bool {
	for _, marker := range []string{"失败", "错误", "超时", "未完全成功"} {
		if strings.Contains(text, marker) {
			return true
		}
	}
	return false
}

func setStatus(text string) {
	procSetWindowTextW.Call(app.status, uintptr(unsafe.Pointer(utf16(text))))
}

func minimizeToTray() {
	if addTrayIcon() {
		procShowWindow.Call(app.hwnd, SW_HIDE)
	}
}

func restoreFromTray() {
	procShowWindow.Call(app.hwnd, SW_SHOW)
	procShowWindow.Call(app.hwnd, SW_MAXIMIZE)
	refreshAfterRestore()
	procSetForegroundWindow.Call(app.hwnd)
	procUpdateWindow.Call(app.hwnd)
}

func showTrayMenu() {
	menu, _, _ := procCreatePopupMenu.Call()
	if menu == 0 {
		return
	}
	defer procDestroyMenu.Call(menu)
	appendTrayMenuItem(menu, ID_TRAY_INJECT, "注入启动")
	appendTrayMenuItem(menu, ID_TRAY_CLEAN, "纯净启动")
	procAppendMenuW.Call(menu, MF_SEPARATOR, 0, 0)
	appendTrayMenuItem(menu, ID_TRAY_SHOW, "显示窗口")
	procAppendMenuW.Call(menu, MF_SEPARATOR, 0, 0)
	appendTrayMenuItem(menu, ID_TRAY_EXIT, "退出")

	var cursor point
	if ret, _, _ := procGetCursorPos.Call(uintptr(unsafe.Pointer(&cursor))); ret == 0 {
		return
	}
	procSetForegroundWindow.Call(app.hwnd)
	command, _, _ := procTrackPopupMenu.Call(
		menu,
		TPM_RIGHTBUTTON|TPM_RETURNCMD,
		uintptr(cursor.X), uintptr(cursor.Y),
		0, app.hwnd, 0,
	)
	// Required for notification-area context menus so Windows reliably
	// dismisses the menu before the next tray interaction.
	procPostMessageW.Call(app.hwnd, WM_NULL, 0, 0)
	switch command {
	case ID_TRAY_INJECT:
		triggerRowFilterCommand(ID_ROW_FILTER_START, "注入启动")
	case ID_TRAY_CLEAN:
		triggerRowFilterCommand(ID_ROW_FILTER_CLEAN, "纯净启动")
	case ID_TRAY_SHOW:
		restoreFromTray()
	case ID_TRAY_EXIT:
		requestClose()
	}
}

func appendTrayMenuItem(menu uintptr, id uintptr, label string) {
	procAppendMenuW.Call(menu, MF_STRING, id, uintptr(unsafe.Pointer(utf16(label))))
}

func triggerRowFilterCommand(command uintptr, label string) {
	app.mu.Lock()
	rowFilterHwnd := app.children[1].hwnd
	ready := app.children[1].ready
	app.mu.Unlock()
	if !ready || rowFilterHwnd == 0 {
		setStatus("组服务器过滤器仍在加载，稍后再试。")
		return
	}
	posted, _, _ := procPostMessageW.Call(rowFilterHwnd, WM_COMMAND, command, 0)
	if posted == 0 {
		setStatus("组服务器过滤器已退出或暂时不可用。")
		return
	}
	setStatus("已从托盘触发" + label + "。")
}

func refreshAfterRestore() {
	layout(app.hwnd)
	forceRefreshCurrentPage()
	for _, handle := range []uintptr{
		app.hwnd, app.sidebar, app.content,
		app.bhopBtn, app.filterBtn, app.modsBtn,
		app.pageHosts[app.current],
	} {
		if handle != 0 {
			procRedrawWindow.Call(handle, 0, 0, RDW_INVALIDATE|RDW_ERASE|RDW_ALLCHILDREN|RDW_UPDATENOW)
		}
	}
	app.mu.Lock()
	childHwnd := app.children[app.current].hwnd
	app.mu.Unlock()
	if childHwnd != 0 {
		procRedrawWindow.Call(childHwnd, 0, 0, RDW_INVALIDATE|RDW_ERASE|RDW_ALLCHILDREN|RDW_UPDATENOW)
		procPostMessageW.Call(childHwnd, WM_MIX_ACTIVATE, 0, 0)
		procUpdateWindow.Call(childHwnd)
	}
}

func forceRefreshCurrentPage() {
	r := clientRect(app.content)
	pageW, pageH := r.Right-r.Left, r.Bottom-r.Top
	// Capture the component's current enabled state, not the stale state from
	// its previous activation (it may have disabled itself during a task).
	deactivatePage(app.current)
	for i := range app.children {
		if i != app.current {
			deactivatePage(i)
		}
	}
	for i, host := range app.pageHosts {
		if i != app.current {
			procShowWindow.Call(host, SW_HIDE)
		}
	}
	host := app.pageHosts[app.current]
	if host != 0 {
		procShowWindow.Call(host, SW_HIDE)
		move(host, 0, 0, pageW, pageH)
		procRedrawWindow.Call(app.content, 0, 0, RDW_INVALIDATE|RDW_ERASE|RDW_ALLCHILDREN|RDW_UPDATENOW)
		procShowWindow.Call(host, SW_SHOW)
	}
	activatePage(app.current, pageW, pageH)
}

func refreshCurrentPage() {
	r := clientRect(app.content)
	pageW, pageH := r.Right-r.Left, r.Bottom-r.Top
	for i := range app.children {
		if i != app.current {
			deactivatePage(i)
		}
	}
	for i, host := range app.pageHosts {
		if i != app.current {
			procShowWindow.Call(host, SW_HIDE)
		}
	}
	activatePage(app.current, pageW, pageH)
	invalidate(app.bhopBtn)
	invalidate(app.filterBtn)
	invalidate(app.modsBtn)
}

func addTrayIcon() bool {
	if app.trayVisible {
		return true
	}
	var data notifyIconData
	data.CbSize = uint32(unsafe.Sizeof(data))
	data.HWnd = app.hwnd
	data.UID = 1
	data.UFlags = NIF_MESSAGE | NIF_ICON | NIF_TIP
	data.UCallbackMessage = WM_TRAY_ICON
	data.HIcon = app.iconSmall
	copy(data.SzTip[:], syscall.StringToUTF16(appTitle+" - 三功能控制台"))
	ret, _, _ := procShellNotifyIconW.Call(NIM_ADD, uintptr(unsafe.Pointer(&data)))
	app.trayVisible = ret != 0
	return app.trayVisible
}

func deleteTrayIcon() {
	if !app.trayVisible {
		return
	}
	var data notifyIconData
	data.CbSize = uint32(unsafe.Sizeof(data))
	data.HWnd = app.hwnd
	data.UID = 1
	procShellNotifyIconW.Call(NIM_DELETE, uintptr(unsafe.Pointer(&data)))
	app.trayVisible = false
}

func child(class, text string, style uint32, x, y, w, h int32, parent uintptr, id int) uintptr {
	hwnd, _, _ := procCreateWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(utf16(class))),
		uintptr(unsafe.Pointer(utf16(text))),
		uintptr(WS_CHILD|WS_VISIBLE|style),
		uintptr(x), uintptr(y), uintptr(w), uintptr(h),
		parent, uintptr(id), 0, 0,
	)
	setFont(hwnd, app.font)
	return hwnd
}

func setFont(hwnd, font uintptr) {
	if hwnd != 0 && font != 0 {
		procSendMessageW.Call(hwnd, WM_SETFONT, font, 1)
	}
}

func move(hwnd uintptr, x, y, w, h int32) {
	if hwnd != 0 {
		procMoveWindow.Call(hwnd, uintptr(x), uintptr(y), uintptr(w), uintptr(h), 1)
	}
}

func clientRect(hwnd uintptr) rect {
	var r rect
	procGetClientRect.Call(hwnd, uintptr(unsafe.Pointer(&r)))
	return r
}

func fillRect(hdc uintptr, r rect, brush uintptr) {
	procFillRect.Call(hdc, uintptr(unsafe.Pointer(&r)), brush)
}

func invalidate(hwnd uintptr) {
	user32.NewProc("InvalidateRect").Call(hwnd, 0, 1)
}

func solid(color uint32) uintptr {
	ret, _, _ := procCreateSolidBrush.Call(uintptr(color))
	return ret
}

func createFont(name string, height, weight int32) uintptr {
	ret, _, _ := procCreateFontW.Call(
		uintptr(height), 0, 0, 0, uintptr(weight), 0, 0, 0,
		1, 0, 0, 5, 0,
		uintptr(unsafe.Pointer(utf16(name))),
	)
	return ret
}

func loadCursor(id uintptr) uintptr {
	ret, _, _ := procLoadCursorW.Call(0, id)
	return ret
}

func loadIcon(instance uintptr, size int32) uintptr {
	ret, _, _ := procLoadImageW.Call(instance, 1, IMAGE_ICON, uintptr(size), uintptr(size), 0)
	return ret
}

func rgb(r, g, b byte) uint32 {
	return uint32(r) | uint32(g)<<8 | uint32(b)<<16
}

func utf16(s string) *uint16 {
	p, _ := syscall.UTF16PtrFromString(strings.ReplaceAll(s, "\x00", ""))
	return p
}

func setChildError(index int, err error) {
	app.mu.Lock()
	app.children[index].err = err
	closing := app.closing
	app.mu.Unlock()
	if !closing {
		procPostMessageW.Call(app.hwnd, WM_APP_CHILD_FAILED, uintptr(index), 0)
	}
}
