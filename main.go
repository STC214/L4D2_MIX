package main

import (
	"crypto/sha256"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
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

	WM_APP_CHILD_READY = WM_APP + 1
	WM_APP_STATUS      = WM_APP + 2
	WM_TRAY_ICON       = WM_APP + 3
	WM_APP_CLOSE_READY = WM_APP + 4
	WM_MIX_CAN_CLOSE   = 0x80F0
	WM_MIX_ACTIVATE    = 0x80F1

	SC_MINIMIZE    = 0xF020
	SIZE_MINIMIZED = 1

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

	NIM_ADD     = 0
	NIM_DELETE  = 2
	NIF_MESSAGE = 1
	NIF_ICON    = 2
	NIF_TIP     = 4

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
)

var (
	user32   = syscall.NewLazyDLL("user32.dll")
	kernel32 = syscall.NewLazyDLL("kernel32.dll")
	gdi32    = syscall.NewLazyDLL("gdi32.dll")
	shell32  = syscall.NewLazyDLL("shell32.dll")

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
	procEnableWindow             = user32.NewProc("EnableWindow")
	procRedrawWindow             = user32.NewProc("RedrawWindow")
	procFillRect                 = user32.NewProc("FillRect")
	procDrawTextW                = user32.NewProc("DrawTextW")
	procGetModuleHandleW         = kernel32.NewProc("GetModuleHandleW")
	procCreateSolidBrush         = gdi32.NewProc("CreateSolidBrush")
	procDeleteObject             = gdi32.NewProc("DeleteObject")
	procCreateFontW              = gdi32.NewProc("CreateFontW")
	procSetTextColor             = gdi32.NewProc("SetTextColor")
	procSetBkMode                = gdi32.NewProc("SetBkMode")
	procSelectObject             = gdi32.NewProc("SelectObject")
	procShellNotifyIconW         = shell32.NewProc("Shell_NotifyIconW")
	procIsUserAnAdmin            = shell32.NewProc("IsUserAnAdmin")
	procShellExecuteW            = shell32.NewProc("ShellExecuteW")
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
	hwnd                                                       uintptr
	headerTitle, headerSub, status                             uintptr
	sidebar, content                                           uintptr
	pageHosts                                                  [3]uintptr
	bhopBtn, filterBtn, modsBtn                                uintptr
	font, titleFont                                            uintptr
	iconBig, iconSmall                                         uintptr
	bgBrush, panelBrush, cardBrush, accentBrush, selectedBrush uintptr
	current                                                    int
	trayVisible                                                bool
	children                                                   [3]childApp
	mu                                                         sync.Mutex
	pendingStatus                                              string
	closing                                                    bool
}

var app appState

func main() {
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
	app.bgBrush = solid(rgb(18, 22, 28))
	app.panelBrush = solid(rgb(25, 31, 40))
	app.cardBrush = solid(rgb(31, 37, 47))
	app.accentBrush = solid(rgb(80, 154, 255))
	app.selectedBrush = solid(rgb(42, 74, 113))
}

func cleanupTheme() {
	for _, h := range []uintptr{app.bgBrush, app.panelBrush, app.cardBrush, app.accentBrush, app.selectedBrush, app.font, app.titleFont} {
		if h != 0 {
			procDeleteObject.Call(h)
		}
	}
}

func runUI() {
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
		WS_OVERLAPPEDWINDOW|WS_CLIPCHILDREN,
		80, 55, 1320, 860,
		0, 0, hinst, 0,
	)
	app.hwnd = hwnd
	procSendMessageW.Call(hwnd, WM_SETICON, ICON_BIG, app.iconBig)
	procSendMessageW.Call(hwnd, WM_SETICON, ICON_SMALL, app.iconSmall)
	procShowWindow.Call(hwnd, SW_MAXIMIZE)
	procUpdateWindow.Call(hwnd)

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
		app.hwnd = hwnd
		createControls(hwnd)
		layout(hwnd)
		setPage(0)
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
	case WM_APP_STATUS:
		drainStatus()
		return 0
	case WM_APP_CLOSE_READY:
		deleteTrayIcon()
		procDestroyWindow.Call(hwnd)
		return 0
	case WM_TRAY_ICON:
		switch uint32(lParam) {
		case WM_LBUTTONUP, WM_LBUTTONDBLCLK, WM_RBUTTONUP:
			restoreFromTray()
		}
		return 0
	case WM_ERASEBKGND:
		fillRect(wParam, clientRect(hwnd), app.bgBrush)
		return 1
	case WM_CTLCOLORSTATIC:
		procSetBkMode.Call(wParam, TRANSPARENT)
		procSetTextColor.Call(wParam, uintptr(rgb(228, 234, 242)))
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
	app.font = createFont("Microsoft YaHei UI", -17, 400)
	app.titleFont = createFont("Microsoft YaHei UI", -25, 700)

	app.headerTitle = child("STATIC", "L4D2 MIX", SS_LEFT, 24, 14, 300, 34, hwnd, 0)
	setFont(app.headerTitle, app.titleFont)
	app.headerSub = child("STATIC", "连跳辅助 + 组服务器过滤 + MOD 分类合并 · 统一控制台", SS_LEFT, 24, 49, 680, 24, hwnd, 0)
	app.status = child("STATIC", "正在准备内置组件…", SS_LEFT, 0, 25, 470, 24, hwnd, 0)

	app.sidebar = child("STATIC", "", SS_LEFT, 0, 78, 218, 700, hwnd, 0)
	app.content = child("STATIC", "", SS_LEFT|WS_CLIPCHILDREN|WS_CLIPSIBLINGS, 230, 88, 1040, 700, hwnd, 0)
	app.pageHosts[0] = child("STATIC", "", SS_LEFT|WS_CLIPCHILDREN|WS_CLIPSIBLINGS, 0, 0, 1040, 700, app.content, 0)
	app.pageHosts[1] = child("STATIC", "", SS_LEFT|WS_CLIPCHILDREN|WS_CLIPSIBLINGS, 0, 0, 1040, 700, app.content, 0)
	app.pageHosts[2] = child("STATIC", "", SS_LEFT|WS_CLIPCHILDREN|WS_CLIPSIBLINGS, 0, 0, 1040, 700, app.content, 0)
	app.bhopBtn = child("BUTTON", "连跳辅助", BS_OWNERDRAW|WS_TABSTOP, 18, 112, 182, 52, hwnd, ID_PAGE_BHOP)
	app.filterBtn = child("BUTTON", "组服务器过滤", BS_OWNERDRAW|WS_TABSTOP, 18, 176, 182, 52, hwnd, ID_PAGE_FILTER)
	app.modsBtn = child("BUTTON", "MOD 分类合并", BS_OWNERDRAW|WS_TABSTOP, 18, 240, 182, 52, hwnd, ID_PAGE_MODS)
}

func layout(hwnd uintptr) {
	r := clientRect(hwnd)
	w, h := r.Right-r.Left, r.Bottom-r.Top
	if w <= 0 || h <= 0 {
		return
	}
	move(app.headerTitle, 24, 12, 280, 34)
	move(app.headerSub, 24, 47, 600, 24)
	move(app.status, w-500, 26, 470, 24)
	move(app.sidebar, 0, 78, 218, h-78)
	move(app.bhopBtn, 18, 112, 182, 52)
	move(app.filterBtn, 18, 176, 182, 52)
	move(app.modsBtn, 18, 240, 182, 52)
	move(app.content, 230, 88, w-248, h-106)
	for _, host := range app.pageHosts {
		move(host, 0, 0, w-248, h-106)
	}
	for i := range app.children {
		app.mu.Lock()
		childHwnd := app.children[i].hwnd
		app.mu.Unlock()
		if childHwnd != 0 {
			if i == app.current {
				activatePage(i, w-248, h-106)
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
	app.mu.Unlock()
	if ready {
		setStatus(name + "已就绪")
	} else {
		setStatus("正在加载" + name + "…")
	}
}

func prepareAndLaunchChildren() {
	root, err := extractPayload()
	if err != nil {
		setStatusAsync("组件释放失败：" + err.Error())
		return
	}
	importSummary := importLegacyModJoinState(filepath.Join(mainExeDir(), "data", "mod-join"))
	app.mu.Lock()
	app.children[0] = childApp{name: "连跳辅助", exe: filepath.Join(root, "L4D2AutobhopVPKW.exe")}
	app.children[1] = childApp{name: "组服务器过滤器", exe: filepath.Join(root, "L4D2RowFilterManager.exe")}
	app.children[2] = childApp{name: "MOD 分类合并", exe: filepath.Join(root, "L4D2ModJoin.exe")}
	app.mu.Unlock()
	if importSummary != "" {
		setStatusAsync(importSummary)
	}
	setStatusAsync("内置组件已准备，正在打开三个功能页…")
	for i := range app.children {
		go launchChild(i)
	}
}

func launchChild(index int) {
	app.mu.Lock()
	name := app.children[index].name
	exe := app.children[index].exe
	parent := app.pageHosts[index]
	if app.closing {
		app.mu.Unlock()
		return
	}
	app.mu.Unlock()

	cmd := exec.Command(exe)
	cmd.Dir = filepath.Dir(exe)
	cmd.Env = append(
		os.Environ(),
		"L4D2_MIX_PARENT="+strconv.FormatUint(uint64(parent), 10),
		"L4D2_MIX_DATA_ROOT="+mainExeDir(),
		"L4D2_MIX_ROW_FILTER_DIR="+filepath.Join(mainExeDir(), "data", "row-filter"),
		"L4D2_MIX_MOD_JOIN_ROOT="+mainExeDir(),
		"L4D2_MIX_MOD_JOIN_STATE_DIR="+filepath.Join(mainExeDir(), "data", "mod-join"),
	)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if err := cmd.Start(); err != nil {
		app.mu.Lock()
		app.children[index].err = err
		app.mu.Unlock()
		setStatusAsync(name + "启动失败：" + err.Error())
		return
	}
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
	go func() {
		_ = cmd.Wait()
		app.mu.Lock()
		if app.children[index].process == cmd.Process {
			app.children[index].process = nil
			app.children[index].hwnd = 0
			app.children[index].ready = false
		}
		close(done)
		app.mu.Unlock()
	}()
	hwnd := waitForProcessWindow(parent, uint32(cmd.Process.Pid), 20*time.Second)
	if hwnd == 0 {
		app.mu.Lock()
		app.children[index].err = errors.New("等待窗口超时")
		app.mu.Unlock()
		setStatusAsync(name + "启动失败：等待窗口超时")
		return
	}
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
	tmp := path + ".new"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return err
	}
	_ = os.Remove(path)
	return os.Rename(tmp, path)
}

func attachChild(index int, hwnd uintptr) {
	if index < 0 || index >= len(app.children) || hwnd == 0 {
		return
	}
	app.mu.Lock()
	if app.closing {
		app.mu.Unlock()
		procPostMessageW.Call(hwnd, WM_CLOSE, 0, 0)
		return
	}
	app.children[index].hwnd = hwnd
	app.children[index].ready = true
	app.children[index].enabledBeforeHide = true
	app.mu.Unlock()
	layout(app.hwnd)
	setPage(app.current)
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
		SWP_NOZORDER|SWP_SHOWWINDOW,
	)
	if !wasActive {
		if enabledBeforeHide {
			procEnableWindow.Call(hwnd, 1)
		} else {
			procEnableWindow.Call(hwnd, 0)
		}
	}
	procShowWindow.Call(hwnd, SW_SHOW)
	app.mu.Lock()
	app.children[index].active = true
	app.mu.Unlock()
	if index == 2 {
		procSendMessageW.Call(hwnd, WM_MIX_ACTIVATE, 0, 0)
	}
}

func waitForProcessWindow(parent uintptr, pid uint32, timeout time.Duration) uintptr {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		var found uintptr
		callback := syscall.NewCallback(func(hwnd, _ uintptr) uintptr {
			var windowPID uint32
			procGetWindowThreadProcessId.Call(hwnd, uintptr(unsafe.Pointer(&windowPID)))
			if windowPID == pid {
				found = hwnd
				return 0
			}
			return 1
		})
		procEnumChildWindows.Call(parent, callback, 0)
		if found != 0 {
			return found
		}
		time.Sleep(50 * time.Millisecond)
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
		canClose, _, _ := procSendMessageW.Call(modHwnd, WM_MIX_CAN_CLOSE, 0, 0)
		if canClose == 0 {
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

func extractPayload() (string, error) {
	base := os.Getenv("LOCALAPPDATA")
	if base == "" {
		var err error
		base, err = os.UserCacheDir()
		if err != nil {
			return "", err
		}
	}
	root := filepath.Join(base, "L4D2_MIX")
	dataRoot := filepath.Join(mainExeDir(), "data")
	mutable := map[string]bool{
		"runtime/matchmaking_row_filter_dll/blocked_keywords.txt":            true,
		"runtime/matchmaking_row_filter_dll/blocked_connectstrings.txt":      true,
		"runtime/matchmaking_row_filter_dll/learned_connectstrings.txt":      true,
		"runtime/matchmaking_row_filter_dll/auto_derived_connectstrings.txt": true,
		"runtime/matchmaking_row_filter_dll/row_filter_mode.txt":             true,
		"runtime/matchmaking_row_filter_dll/matchmaking_row_filter.log":      true,
	}
	err := fs.WalkDir(payload, "payload", func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel("payload", path)
		if err != nil || rel == "." {
			return err
		}
		rel = filepath.ToSlash(rel)
		target := payloadTarget(root, dataRoot, rel)
		if entry.IsDir() {
			return os.MkdirAll(target, 0755)
		}
		data, err := payload.ReadFile(path)
		if err != nil {
			return err
		}
		if mutable[rel] {
			if _, err := os.Stat(target); err == nil {
				return nil
			}
		} else if sameFileContent(target, data) {
			return nil
		}
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}
		tmp := target + ".new"
		if err := os.WriteFile(tmp, data, 0644); err != nil {
			return err
		}
		_ = os.Remove(target)
		return os.Rename(tmp, target)
	})
	return root, err
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
	brush := app.cardBrush
	if selected {
		brush = app.selectedBrush
	}
	if item.ItemState&ODS_SELECTED != 0 {
		brush = app.accentBrush
	}
	fillRect(item.HDC, item.RcItem, brush)
	procSetBkMode.Call(item.HDC, TRANSPARENT)
	procSetTextColor.Call(item.HDC, uintptr(rgb(238, 244, 252)))
	procSelectObject.Call(item.HDC, app.font)
	text := "连跳辅助"
	if item.CtlID == ID_PAGE_FILTER {
		text = "组服务器过滤"
	} else if item.CtlID == ID_PAGE_MODS {
		text = "MOD 分类合并"
	}
	p := utf16(text)
	r := item.RcItem
	procDrawTextW.Call(item.HDC, uintptr(unsafe.Pointer(p)), ^uintptr(0), uintptr(unsafe.Pointer(&r)), DT_CENTER|DT_VCENTER|DT_SINGLELINE)
}

func setStatusAsync(text string) {
	app.mu.Lock()
	app.pendingStatus = text
	app.mu.Unlock()
	procPostMessageW.Call(app.hwnd, WM_APP_STATUS, 0, 0)
}

func drainStatus() {
	app.mu.Lock()
	text := app.pendingStatus
	app.pendingStatus = ""
	app.mu.Unlock()
	if text != "" {
		setStatus(text)
	}
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
	deleteTrayIcon()
	procShowWindow.Call(app.hwnd, SW_RESTORE)
	refreshAfterRestore()
	procSetForegroundWindow.Call(app.hwnd)
}

func refreshAfterRestore() {
	layout(app.hwnd)
	refreshCurrentPage()
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
		procSendMessageW.Call(childHwnd, WM_MIX_ACTIVATE, 0, 0)
	}
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
