package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

const (
	appTitle = "L4D2 Autobhop VPK-W"

	wmAppStatus = 0x8001
	wmTrayIcon  = 0x8002
	wmAppDebug  = 0x8003
	wmAppConfig = 0x8004

	idProcessCombo = 101
	idRefreshBtn   = 102
	idStartBtn     = 103
	idStopBtn      = 104
	idTitleEdit    = 105
	idBaseCombo    = 106
	idGroundEdit   = 107
	idMoveTypeEdit = 108
	idFlagsEdit    = 109
	idPollEdit     = 110
	idStatusText   = 111
	idDetectBtn    = 112

	statusIdle = iota
	statusScanning
	statusRunning
	statusStopped
	statusErrorOpenProcess
	statusErrorClientDLL
	statusGameClosed
	statusBadSettings

	PROCESS_VM_READ                   = 0x0010
	PROCESS_QUERY_LIMITED_INFO        = 0x1000
	TH32CS_SNAPMODULE          uint32 = 0x00000008
	TH32CS_SNAPMODULE32        uint32 = 0x00000010
	INVALID_HANDLE_VALUE              = ^uintptr(0)

	CB_ADDSTRING       = 0x0143
	CB_GETCURSEL       = 0x0147
	CB_SETCURSEL       = 0x014E
	CB_RESETCONTENT    = 0x014B
	CB_SHOWDROPDOWN    = 0x014F
	BN_CLICKED         = 0
	CBN_SELCHANGE      = 1
	WM_CLOSE           = 0x0010
	WM_COMMAND         = 0x0111
	WM_CREATE          = 0x0001
	WM_DESTROY         = 0x0002
	WM_CTLCOLORSTATIC  = 0x0138
	WM_CTLCOLOREDIT    = 0x0133
	WM_CTLCOLORLISTBOX = 0x0134
	WM_SETFONT         = 0x0030
	WM_GETTEXT         = 0x000D
	WM_GETTEXTLENGTH   = 0x000E
	WM_SETTEXT         = 0x000C
	WM_SETICON         = 0x0080
	WM_SYSCOMMAND      = 0x0112
	WM_LBUTTONUP       = 0x0202
	WM_RBUTTONUP       = 0x0205
	WM_LBUTTONDBLCLK   = 0x0203
	BM_SETCHECK        = 0x00F1
	BST_CHECKED        = 1
	SC_MINIMIZE        = 0xF020
	WS_OVERLAPPED      = 0x00000000
	WS_CAPTION         = 0x00C00000
	WS_SYSMENU         = 0x00080000
	WS_MINIMIZEBOX     = 0x00020000
	WS_VISIBLE         = 0x10000000
	WS_CHILD           = 0x40000000
	WS_CLIPCHILDREN    = 0x02000000
	WS_CLIPSIBLINGS    = 0x04000000
	WS_TABSTOP         = 0x00010000
	WS_VSCROLL         = 0x00200000
	WS_BORDER          = 0x00800000
	CBS_DROPDOWNLIST   = 0x0003
	ES_AUTOHSCROLL     = 0x0080
	ES_READONLY        = 0x0800
	BS_PUSHBUTTON      = 0x00000000
	SS_LEFT            = 0x00000000
	IMAGE_ICON         = 1
	ICON_SMALL         = 0
	ICON_BIG           = 1
	LR_DEFAULTCOLOR    = 0x0000
	SW_SHOW            = 5
	SW_HIDE            = 0
	SW_RESTORE         = 9
	COLOR_WINDOW       = 5
	VK_SPACE           = 0x20
	WM_KEYDOWN         = 0x0100
	WM_KEYUP           = 0x0101
	SPACE_KEY_LPARAM   = 0x390000
	NIM_ADD            = 0x00000000
	NIM_DELETE         = 0x00000002
	NIF_MESSAGE        = 0x00000001
	NIF_ICON           = 0x00000002
	NIF_TIP            = 0x00000004

	FL_ONGROUND     = 1 << 0
	MOVETYPE_LADDER = 9
	INVALID_EHANDLE = 0xFFFFFFFF
)

var (
	user32   = syscall.NewLazyDLL("user32.dll")
	kernel32 = syscall.NewLazyDLL("kernel32.dll")
	gdi32    = syscall.NewLazyDLL("gdi32.dll")

	procCreateWindowExW          = user32.NewProc("CreateWindowExW")
	procDefWindowProcW           = user32.NewProc("DefWindowProcW")
	procDestroyWindow            = user32.NewProc("DestroyWindow")
	procDispatchMessageW         = user32.NewProc("DispatchMessageW")
	procEnumWindows              = user32.NewProc("EnumWindows")
	procGetAsyncKeyState         = user32.NewProc("GetAsyncKeyState")
	procGetForegroundWindow      = user32.NewProc("GetForegroundWindow")
	procGetMessageW              = user32.NewProc("GetMessageW")
	procGetWindowTextLengthW     = user32.NewProc("GetWindowTextLengthW")
	procGetWindowTextW           = user32.NewProc("GetWindowTextW")
	procGetWindowThreadProcessId = user32.NewProc("GetWindowThreadProcessId")
	procIsWindow                 = user32.NewProc("IsWindow")
	procIsWindowVisible          = user32.NewProc("IsWindowVisible")
	procLoadCursorW              = user32.NewProc("LoadCursorW")
	procLoadImageW               = user32.NewProc("LoadImageW")
	procPostMessageW             = user32.NewProc("PostMessageW")
	procRegisterClassExW         = user32.NewProc("RegisterClassExW")
	procSendMessageW             = user32.NewProc("SendMessageW")
	procSetForegroundWindow      = user32.NewProc("SetForegroundWindow")
	procSetBkMode                = gdi32.NewProc("SetBkMode")
	procSetTextColor             = gdi32.NewProc("SetTextColor")
	procSetWindowTextW           = user32.NewProc("SetWindowTextW")
	procShowWindow               = user32.NewProc("ShowWindow")
	procTranslateMessage         = user32.NewProc("TranslateMessage")

	procCloseHandle              = kernel32.NewProc("CloseHandle")
	procCreateToolhelp32Snapshot = kernel32.NewProc("CreateToolhelp32Snapshot")
	procGetModuleHandleW         = kernel32.NewProc("GetModuleHandleW")
	procOpenProcess              = kernel32.NewProc("OpenProcess")
	procReadProcessMemory        = kernel32.NewProc("ReadProcessMemory")
	procModule32FirstW           = kernel32.NewProc("Module32FirstW")
	procModule32NextW            = kernel32.NewProc("Module32NextW")

	procCreateSolidBrush = gdi32.NewProc("CreateSolidBrush")
	procCreateFontW      = gdi32.NewProc("CreateFontW")

	procShellNotifyIconW = syscall.NewLazyDLL("shell32.dll").NewProc("Shell_NotifyIconW")
)

type point struct{ x, y int32 }
type msg struct {
	hwnd    uintptr
	message uint32
	wParam  uintptr
	lParam  uintptr
	time    uint32
	pt      point
}
type wndClassEx struct {
	cbSize        uint32
	style         uint32
	lpfnWndProc   uintptr
	cbClsExtra    int32
	cbWndExtra    int32
	hInstance     uintptr
	hIcon         uintptr
	hCursor       uintptr
	hbrBackground uintptr
	lpszMenuName  *uint16
	lpszClassName *uint16
	hIconSm       uintptr
}
type moduleEntry32 struct {
	dwSize        uint32
	th32ModuleID  uint32
	th32ProcessID uint32
	glblcntUsage  uint32
	proccntUsage  uint32
	modBaseAddr   uintptr
	modBaseSize   uint32
	hModule       uintptr
	szModule      [256]uint16
	szExePath     [260]uint16
}
type guid struct {
	data1 uint32
	data2 uint16
	data3 uint16
	data4 [8]byte
}
type notifyIconData struct {
	cbSize           uint32
	hwnd             uintptr
	uID              uint32
	uFlags           uint32
	uCallbackMessage uint32
	hIcon            uintptr
	szTip            [128]uint16
	dwState          uint32
	dwStateMask      uint32
	szInfo           [256]uint16
	uVersion         uint32
	szInfoTitle      [64]uint16
	dwInfoFlags      uint32
	guidItem         guid
	hBalloonIcon     uintptr
}
type targetWindow struct {
	hwnd  uintptr
	pid   uint32
	title string
}
type settings struct {
	hwnd             uintptr
	pid              uint32
	playerBaseOffset uintptr
	groundOffset     uintptr
	moveTypeOffset   uintptr
	mFlagsOffset     uintptr
	pollInterval     time.Duration
}
type runtimeDebug struct {
	groundEntity uint32
	moveType     uint32
	flags        uint32
	onGround     bool
	onLadder     bool
	jumpDown     bool
	readFailures uint64
	playerPtr    uint32
}
type savedConfig struct {
	PlayerBase      string `json:"player_base"`
	GroundEntity    string `json:"m_h_ground_entity"`
	MoveType        string `json:"movetype"`
	MFlags          string `json:"m_flags"`
	PollMS          int    `json:"poll_ms"`
	UpdatedAt       string `json:"updated_at"`
	Source          string `json:"source"`
	DetectionScore  int    `json:"detection_score,omitempty"`
	DetectionDetail string `json:"detection_detail,omitempty"`
}
type detectResult struct {
	cfg    savedConfig
	score  int
	detail string
	ok     bool
}
type detectInputs struct {
	playerBases []uintptr
	grounds     []uintptr
	moveTypes   []uintptr
	flags       []uintptr
	pollMS      int
}
type appState struct {
	hwnd             uintptr
	controls         map[int]uintptr
	targets          []targetWindow
	stop             chan struct{}
	running          bool
	mu               sync.Mutex
	debugMu          sync.Mutex
	statusText       string
	debug            runtimeDebug
	pendingConfig    savedConfig
	hasPendingConfig bool
	lastDebugPost    time.Time
	font             uintptr
	bgBrush          uintptr
	fieldBrush       uintptr
	iconBig          uintptr
	iconSmall        uintptr
	trayVisible      bool
}

var state = &appState{controls: make(map[int]uintptr)}

func main() {
	runtime.LockOSThread()
	embedParent := embeddedParent()

	hInstance, _, _ := procGetModuleHandleW.Call(0)
	className := utf16Ptr("L4D2AutobhopVPKW")
	cursor, _, _ := procLoadCursorW.Call(0, uintptr(32512))
	state.iconBig = loadAppIcon(hInstance, 256)
	state.iconSmall = loadAppIcon(hInstance, 16)
	bg, _, _ := procCreateSolidBrush.Call(0x202020)
	field, _, _ := procCreateSolidBrush.Call(0x2C2C2C)
	state.bgBrush = bg
	state.fieldBrush = field

	wc := wndClassEx{
		cbSize:        uint32(unsafe.Sizeof(wndClassEx{})),
		lpfnWndProc:   syscall.NewCallback(wndProc),
		hInstance:     hInstance,
		hIcon:         state.iconBig,
		hCursor:       cursor,
		hbrBackground: bg,
		lpszClassName: className,
		hIconSm:       state.iconSmall,
	}
	procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))

	windowStyle := uintptr(WS_OVERLAPPED | WS_CAPTION | WS_SYSMENU | WS_MINIMIZEBOX | WS_VISIBLE)
	parent := uintptr(0)
	x, y, width, height := int32(200), int32(120), int32(660), int32(540)
	if embedParent != 0 {
		windowStyle = WS_CHILD | WS_CLIPCHILDREN | WS_CLIPSIBLINGS
		parent = embedParent
		x, y, width, height = 0, 0, 1040, 700
	}
	hwnd, _, _ := procCreateWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(className)),
		uintptr(unsafe.Pointer(utf16Ptr(appTitle))),
		windowStyle,
		uintptr(x), uintptr(y), uintptr(width), uintptr(height),
		parent, 0, hInstance, 0,
	)
	state.hwnd = hwnd
	applyWindowIcons(hwnd)
	if embedParent == 0 {
		procShowWindow.Call(hwnd, SW_SHOW)
	}

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

func embeddedParent() uintptr {
	value := strings.TrimSpace(os.Getenv("L4D2_MIX_PARENT"))
	if value == "" {
		return 0
	}
	parent, err := strconv.ParseUint(value, 10, 64)
	if err != nil {
		return 0
	}
	return uintptr(parent)
}

func wndProc(hwnd uintptr, msg uint32, wParam, lParam uintptr) uintptr {
	switch msg {
	case WM_CREATE:
		createUI(hwnd)
		refreshTargets()
		setStatus(statusIdle)
		return 0
	case WM_COMMAND:
		id := int(wParam & 0xffff)
		code := int((wParam >> 16) & 0xffff)
		if id == idProcessCombo && code == CBN_SELCHANGE {
			syncSelectedTitle()
		} else if code == BN_CLICKED {
			switch id {
			case idRefreshBtn:
				refreshTargets()
				procSendMessageW.Call(state.controls[idProcessCombo], CB_SHOWDROPDOWN, 1, 0)
			case idDetectBtn:
				startAutoDetect()
			case idStartBtn:
				startWorker()
			case idStopBtn:
				stopWorker(statusStopped)
			}
		}
		return 0
	case wmAppStatus:
		setStatus(int(wParam))
		return 0
	case wmAppDebug:
		renderStatus()
		return 0
	case wmAppConfig:
		applyPendingConfig()
		return 0
	case wmTrayIcon:
		switch uint32(lParam) {
		case WM_LBUTTONUP, WM_LBUTTONDBLCLK, WM_RBUTTONUP:
			restoreFromTray()
		}
		return 0
	case WM_SYSCOMMAND:
		if wParam&0xfff0 == SC_MINIMIZE {
			minimizeToTray()
			return 0
		}
	case WM_CTLCOLORSTATIC, WM_CTLCOLOREDIT, WM_CTLCOLORLISTBOX:
		hdc := wParam
		procSetBkMode.Call(hdc, 1)
		procSetTextColor.Call(hdc, 0xF2F2F2)
		if msg == WM_CTLCOLORSTATIC {
			return state.bgBrush
		}
		return state.fieldBrush
	case WM_CLOSE:
		deleteTrayIcon()
		stopWorker(statusStopped)
		procDestroyWindow.Call(hwnd)
		return 0
	case WM_DESTROY:
		deleteTrayIcon()
		stopWorker(statusStopped)
		user32.NewProc("PostQuitMessage").Call(0)
		return 0
	}
	ret, _, _ := procDefWindowProcW.Call(hwnd, uintptr(msg), wParam, lParam)
	return ret
}

func createUI(hwnd uintptr) {
	state.font, _, _ = procCreateFontW.Call(
		neg(16), 0, 0, 0, 500, 0, 0, 0, 1, 0, 0, 5, 0,
		uintptr(unsafe.Pointer(utf16Ptr("Segoe UI"))),
	)
	createText(hwnd, 22, 18, 500, 26, "L4D2 连跳 VPK-W", true)
	createText(hwnd, 22, 52, 600, 20, "读取 VPK-like 玩家状态，并用外置状态机模拟空格输入。", false)

	createText(hwnd, 26, 92, 110, 22, "游戏窗口", false)
	state.controls[idProcessCombo] = createControl("COMBOBOX", "", WS_CHILD|WS_VISIBLE|WS_TABSTOP|CBS_DROPDOWNLIST|WS_VSCROLL, 150, 88, 330, 260, hwnd, idProcessCombo)
	state.controls[idRefreshBtn] = createControl("BUTTON", "刷新", WS_CHILD|WS_VISIBLE|WS_TABSTOP|BS_PUSHBUTTON, 500, 87, 110, 30, hwnd, idRefreshBtn)

	createText(hwnd, 26, 136, 110, 22, "窗口标题", false)
	state.controls[idTitleEdit] = createControl("EDIT", "", WS_CHILD|WS_VISIBLE|WS_TABSTOP|WS_BORDER|ES_AUTOHSCROLL|ES_READONLY, 150, 132, 460, 28, hwnd, idTitleEdit)

	createText(hwnd, 26, 180, 110, 22, "PlayerBase", false)
	state.controls[idBaseCombo] = createControl("COMBOBOX", "", WS_CHILD|WS_VISIBLE|WS_TABSTOP|CBS_DROPDOWNLIST, 150, 176, 160, 180, hwnd, idBaseCombo)
	for _, v := range []string{"0x726BD8", "0x73A574", "0x7C4424", "0x7C4450", "0x7C4644"} {
		sendString(state.controls[idBaseCombo], CB_ADDSTRING, v)
	}
	procSendMessageW.Call(state.controls[idBaseCombo], CB_SETCURSEL, 0, 0)

	createText(hwnd, 335, 180, 120, 22, "m_hGroundEntity", false)
	state.controls[idGroundEdit] = createControl("EDIT", "0x14C", WS_CHILD|WS_VISIBLE|WS_TABSTOP|WS_BORDER|ES_AUTOHSCROLL, 465, 176, 145, 28, hwnd, idGroundEdit)

	createText(hwnd, 26, 224, 110, 22, "movetype", false)
	state.controls[idMoveTypeEdit] = createControl("EDIT", "0x178", WS_CHILD|WS_VISIBLE|WS_TABSTOP|WS_BORDER|ES_AUTOHSCROLL, 150, 220, 160, 28, hwnd, idMoveTypeEdit)

	createText(hwnd, 335, 224, 120, 22, "mFlags 调试", false)
	state.controls[idFlagsEdit] = createControl("EDIT", "0xF0", WS_CHILD|WS_VISIBLE|WS_TABSTOP|WS_BORDER|ES_AUTOHSCROLL, 465, 220, 145, 28, hwnd, idFlagsEdit)

	createText(hwnd, 26, 268, 110, 22, "轮询 ms", false)
	state.controls[idPollEdit] = createControl("EDIT", "1", WS_CHILD|WS_VISIBLE|WS_TABSTOP|WS_BORDER|ES_AUTOHSCROLL, 150, 264, 160, 28, hwnd, idPollEdit)

	applySavedConfig(loadSavedConfig())

	state.controls[idDetectBtn] = createControl("BUTTON", "自动探测", WS_CHILD|WS_VISIBLE|WS_TABSTOP|BS_PUSHBUTTON, 150, 322, 130, 36, hwnd, idDetectBtn)
	state.controls[idStartBtn] = createControl("BUTTON", "启动", WS_CHILD|WS_VISIBLE|WS_TABSTOP|BS_PUSHBUTTON, 300, 322, 130, 36, hwnd, idStartBtn)
	state.controls[idStopBtn] = createControl("BUTTON", "停止", WS_CHILD|WS_VISIBLE|WS_TABSTOP|BS_PUSHBUTTON, 450, 322, 130, 36, hwnd, idStopBtn)
	state.controls[idStatusText] = createControl("STATIC", "", WS_CHILD|WS_VISIBLE|SS_LEFT, 26, 385, 590, 105, hwnd, idStatusText)
}

func loadAppIcon(hInstance uintptr, size int32) uintptr {
	icon, _, _ := procLoadImageW.Call(
		hInstance,
		1,
		IMAGE_ICON,
		uintptr(size),
		uintptr(size),
		LR_DEFAULTCOLOR,
	)
	return icon
}

func applyWindowIcons(hwnd uintptr) {
	if state.iconBig != 0 {
		procSendMessageW.Call(hwnd, WM_SETICON, ICON_BIG, state.iconBig)
	}
	if state.iconSmall != 0 {
		procSendMessageW.Call(hwnd, WM_SETICON, ICON_SMALL, state.iconSmall)
	}
}

func minimizeToTray() {
	if state.hwnd == 0 {
		return
	}
	if addTrayIcon() {
		procShowWindow.Call(state.hwnd, SW_HIDE)
	}
}

func restoreFromTray() {
	if state.hwnd == 0 {
		return
	}
	deleteTrayIcon()
	procShowWindow.Call(state.hwnd, SW_RESTORE)
	procSetForegroundWindow.Call(state.hwnd)
}

func addTrayIcon() bool {
	if state.trayVisible || state.hwnd == 0 {
		return state.trayVisible
	}
	nid := newTrayIconData()
	ok, _, _ := procShellNotifyIconW.Call(NIM_ADD, uintptr(unsafe.Pointer(&nid)))
	state.trayVisible = ok != 0
	return state.trayVisible
}

func deleteTrayIcon() bool {
	if !state.trayVisible || state.hwnd == 0 {
		return !state.trayVisible
	}
	nid := newTrayIconData()
	ok, _, _ := procShellNotifyIconW.Call(NIM_DELETE, uintptr(unsafe.Pointer(&nid)))
	if ok != 0 {
		state.trayVisible = false
	}
	return !state.trayVisible
}

func newTrayIconData() notifyIconData {
	var nid notifyIconData
	nid.cbSize = uint32(unsafe.Sizeof(nid))
	nid.hwnd = state.hwnd
	nid.uID = 1
	nid.uFlags = NIF_MESSAGE | NIF_ICON | NIF_TIP
	nid.uCallbackMessage = wmTrayIcon
	nid.hIcon = state.iconSmall
	if nid.hIcon == 0 {
		nid.hIcon = state.iconBig
	}
	copyUTF16(nid.szTip[:], appTitle)
	return nid
}

func createText(parent uintptr, x, y, w, h int32, text string, title bool) uintptr {
	ctrl := createControl("STATIC", text, WS_CHILD|WS_VISIBLE|SS_LEFT, x, y, w, h, parent, 0)
	if title {
		font, _, _ := procCreateFontW.Call(neg(22), 0, 0, 0, 700, 0, 0, 0, 1, 0, 0, 5, 0, uintptr(unsafe.Pointer(utf16Ptr("Segoe UI"))))
		procSendMessageW.Call(ctrl, WM_SETFONT, font, 1)
	}
	return ctrl
}

func createControl(class, text string, style uintptr, x, y, w, height int32, parent uintptr, id int) uintptr {
	hwnd, _, _ := procCreateWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(utf16Ptr(class))),
		uintptr(unsafe.Pointer(utf16Ptr(text))),
		style,
		uintptr(x), uintptr(y), uintptr(w), uintptr(height),
		parent, uintptr(id), 0, 0,
	)
	if state.font != 0 {
		procSendMessageW.Call(hwnd, WM_SETFONT, state.font, 1)
	}
	return hwnd
}

func refreshTargets() {
	combo := state.controls[idProcessCombo]
	procSendMessageW.Call(combo, CB_RESETCONTENT, 0, 0)
	state.targets = enumerateGameWindows()
	for _, t := range state.targets {
		sendString(combo, CB_ADDSTRING, fmt.Sprintf("%s  [pid %d]", t.title, t.pid))
	}
	if len(state.targets) > 0 {
		procSendMessageW.Call(combo, CB_SETCURSEL, 0, 0)
		syncSelectedTitle()
		setStatus(statusIdle)
		return
	}
	sendString(combo, CB_ADDSTRING, "未找到可见的 Left 4 Dead 2 窗口")
	procSendMessageW.Call(combo, CB_SETCURSEL, 0, 0)
	setWindowText(state.controls[idTitleEdit], "")
	setStatus(statusScanning)
}

func syncSelectedTitle() {
	idx := int(send(state.controls[idProcessCombo], CB_GETCURSEL, 0, 0))
	if idx >= 0 && idx < len(state.targets) {
		setWindowText(state.controls[idTitleEdit], getWindowTitle(state.targets[idx].hwnd))
		return
	}
	setWindowText(state.controls[idTitleEdit], "")
}

func selectedTarget() (targetWindow, bool) {
	idx := int(send(state.controls[idProcessCombo], CB_GETCURSEL, 0, 0))
	if idx >= 0 && idx < len(state.targets) {
		return state.targets[idx], true
	}
	return targetWindow{}, false
}

func enumerateGameWindows() []targetWindow {
	var out []targetWindow
	cb := syscall.NewCallback(func(hwnd uintptr, lParam uintptr) uintptr {
		if isWindowVisible(hwnd) && getWindowTextLength(hwnd) > 0 {
			title := getWindowTitle(hwnd)
			if strings.Contains(strings.ToLower(title), "left 4 dead 2") {
				var pid uint32
				procGetWindowThreadProcessId.Call(hwnd, uintptr(unsafe.Pointer(&pid)))
				out = append(out, targetWindow{hwnd: hwnd, pid: pid, title: title})
			}
		}
		return 1
	})
	procEnumWindows.Call(cb, 0)
	return out
}

func startWorker() {
	state.mu.Lock()
	if state.running {
		state.mu.Unlock()
		return
	}
	cfg, ok := readSettings()
	if !ok {
		state.mu.Unlock()
		setStatus(statusBadSettings)
		return
	}
	if err := saveConfig(settingsToSavedConfig(cfg, "manual-start", 0, "saved from valid Start settings")); err != nil {
		state.mu.Unlock()
		setStatusText("偏移保存失败，已取消启动：" + err.Error())
		return
	}
	state.stop = make(chan struct{})
	stop := state.stop
	state.running = true
	state.debugMu.Lock()
	state.debug = runtimeDebug{}
	state.lastDebugPost = time.Time{}
	state.debugMu.Unlock()
	state.mu.Unlock()

	postStatus(statusScanning)
	go runBhop(cfg, stop)
}

func stopWorker(status int) {
	state.mu.Lock()
	if state.running && state.stop != nil {
		close(state.stop)
	}
	state.running = false
	state.stop = nil
	state.mu.Unlock()
	setStatus(status)
}

func startAutoDetect() {
	state.mu.Lock()
	running := state.running
	state.mu.Unlock()
	if running {
		setStatusText("请先停止运行，再执行自动探测。")
		return
	}
	target, ok := selectedTarget()
	if !ok || target.pid == 0 {
		setStatusText("自动探测需要先选择 L4D2 窗口。")
		return
	}
	inputs := detectInputs{
		playerBases: playerBaseCandidates(),
		grounds:     groundCandidates(),
		moveTypes:   moveTypeCandidates(),
		flags:       flagsCandidates(),
		pollMS:      currentPollMS(),
	}
	setStatusText("自动探测：请在本地地图内连续跳几次，持续 6 秒。")
	go func() {
		result := autoDetectOffsets(target.pid, inputs)
		if !result.ok {
			postStatusText("自动探测失败：" + result.detail)
			return
		}
		if err := saveConfig(result.cfg); err != nil {
			postStatusText("自动探测已找到偏移，但保存配置失败：" + err.Error())
			return
		}
		postApplyConfig(result.cfg)
		postStatusText(fmt.Sprintf("自动探测已保存偏移。评分=%d %s", result.score, result.detail))
	}()
}

func readSettings() (settings, bool) {
	var cfg settings
	if idx := int(send(state.controls[idProcessCombo], CB_GETCURSEL, 0, 0)); idx >= 0 && idx < len(state.targets) {
		cfg.hwnd = state.targets[idx].hwnd
		cfg.pid = state.targets[idx].pid
		setWindowText(state.controls[idTitleEdit], getWindowTitle(cfg.hwnd))
	}
	if cfg.hwnd == 0 || cfg.pid == 0 {
		return cfg, false
	}
	cfg.playerBaseOffset = parseHex(comboText(state.controls[idBaseCombo]))
	cfg.groundOffset = parseHex(getControlText(state.controls[idGroundEdit]))
	cfg.moveTypeOffset = parseHex(getControlText(state.controls[idMoveTypeEdit]))
	cfg.mFlagsOffset = parseHex(getControlText(state.controls[idFlagsEdit]))
	ms, err := strconv.Atoi(strings.TrimSpace(getControlText(state.controls[idPollEdit])))
	if err != nil || ms < 1 || cfg.playerBaseOffset == 0 || cfg.groundOffset == 0 || cfg.moveTypeOffset == 0 || cfg.mFlagsOffset == 0 {
		return cfg, false
	}
	if ms > 50 {
		ms = 50
	}
	cfg.pollInterval = time.Duration(ms) * time.Millisecond
	return cfg, true
}

func autoDetectOffsets(pid uint32, inputs detectInputs) detectResult {
	hProcess, _, _ := procOpenProcess.Call(PROCESS_VM_READ|PROCESS_QUERY_LIMITED_INFO, 0, uintptr(pid))
	if hProcess == 0 {
		return detectResult{detail: "could not open process"}
	}
	defer procCloseHandle.Call(hProcess)

	clientBase := waitForClientBaseTimeout(pid, 5*time.Second)
	if clientBase == 0 {
		return detectResult{detail: "client.dll was not ready"}
	}

	playerBase, playerScore, playerPtr := bestPlayerBase(hProcess, clientBase, inputs.playerBases)
	if playerBase == 0 || playerPtr == 0 {
		return detectResult{detail: "no plausible local player pointer"}
	}

	groundOffset, groundScore := probeGroundOffset(hProcess, uintptr(playerPtr), inputs.grounds, inputs.flags)
	moveTypeOffset, moveTypeScore := bestOffset(hProcess, uintptr(playerPtr), inputs.moveTypes, scoreMoveType)
	mFlagsOffset, flagsScore := bestOffset(hProcess, uintptr(playerPtr), inputs.flags, scoreFlags)
	total := playerScore + groundScore + moveTypeScore + flagsScore
	if playerScore < 60 || groundScore < 40 || moveTypeScore < 40 || flagsScore < 40 {
		return detectResult{detail: fmt.Sprintf("low confidence player=%d ground=%d movetype=%d flags=%d total=%d", playerScore, groundScore, moveTypeScore, flagsScore, total)}
	}
	if groundOffset == 0 || moveTypeOffset == 0 || mFlagsOffset == 0 {
		return detectResult{detail: fmt.Sprintf("missing candidate player=%d ground=%d movetype=%d flags=%d", playerScore, groundScore, moveTypeScore, flagsScore)}
	}

	detail := fmt.Sprintf("player=0x%X ground=0x%X movetype=0x%X flags=0x%X", playerBase, groundOffset, moveTypeOffset, mFlagsOffset)
	cfg := savedConfig{
		PlayerBase:      hexOffset(playerBase),
		GroundEntity:    hexOffset(groundOffset),
		MoveType:        hexOffset(moveTypeOffset),
		MFlags:          hexOffset(mFlagsOffset),
		PollMS:          inputs.pollMS,
		UpdatedAt:       time.Now().Format(time.RFC3339),
		Source:          "auto-detect",
		DetectionScore:  total,
		DetectionDetail: detail,
	}
	return detectResult{cfg: cfg, score: total, detail: detail, ok: true}
}

func bestPlayerBase(process uintptr, clientBase uintptr, candidates []uintptr) (uintptr, int, uint32) {
	var bestOffset uintptr
	var bestScore int
	var bestPtr uint32
	for _, offset := range candidates {
		score := 0
		var last uint32
		for i := 0; i < 8; i++ {
			ptr, ok := readUint32(process, clientBase+offset)
			if ok && plausiblePointer(ptr) {
				score += 10
				last = ptr
			}
			time.Sleep(25 * time.Millisecond)
		}
		if score > bestScore {
			bestOffset = offset
			bestScore = score
			bestPtr = last
		}
	}
	return bestOffset, bestScore, bestPtr
}

func bestOffset(process uintptr, playerPtr uintptr, candidates []uintptr, scorer func(uint32) int) (uintptr, int) {
	var bestOffset uintptr
	var bestScore int
	for _, offset := range candidates {
		score := 0
		for i := 0; i < 8; i++ {
			value, ok := readUint32(process, playerPtr+offset)
			if ok {
				score += 2 + scorer(value)
			}
			time.Sleep(10 * time.Millisecond)
		}
		if score > bestScore {
			bestOffset = offset
			bestScore = score
		}
	}
	return bestOffset, bestScore
}

func probeGroundOffset(process uintptr, playerPtr uintptr, groundOffsets []uintptr, flagOffsets []uintptr) (uintptr, int) {
	flagOffset, flagScore := bestOffset(process, playerPtr, flagOffsets, scoreFlags)
	if flagOffset == 0 || flagScore < 40 {
		return 0, 0
	}

	type stats struct {
		reads        int
		matches      int
		transitions  int
		lastCategory int
		groundHits   int
		airHits      int
	}
	all := make(map[uintptr]*stats, len(groundOffsets))
	for _, offset := range groundOffsets {
		all[offset] = &stats{lastCategory: -1}
	}

	deadline := time.Now().Add(6 * time.Second)
	for time.Now().Before(deadline) {
		flags, ok := readUint32(process, playerPtr+flagOffset)
		if !ok {
			time.Sleep(25 * time.Millisecond)
			continue
		}
		legacyGround := legacyCanPressJump(flags)
		for _, offset := range groundOffsets {
			value, ok := readUint32(process, playerPtr+offset)
			if !ok {
				continue
			}
			category := 0
			if value == INVALID_EHANDLE {
				category = 1
			} else if value != 0 && value < 0x10000000 {
				category = 2
			} else {
				category = 3
			}

			s := all[offset]
			s.reads++
			if category == 1 {
				s.airHits++
				if !legacyGround {
					s.matches++
				}
			}
			if category == 2 {
				s.groundHits++
				if legacyGround {
					s.matches++
				}
			}
			if s.lastCategory >= 0 && s.lastCategory != category {
				s.transitions++
			}
			s.lastCategory = category
		}
		time.Sleep(25 * time.Millisecond)
	}

	var bestOffset uintptr
	var bestScore int
	for offset, s := range all {
		if s.reads < 20 || s.groundHits == 0 || s.airHits == 0 {
			continue
		}
		score := s.matches*4 + s.transitions*6 + s.groundHits + s.airHits
		if score > bestScore {
			bestOffset = offset
			bestScore = score
		}
	}
	return bestOffset, bestScore
}

func playerBaseCandidates() []uintptr {
	return uniqueOffsets([]uintptr{
		parseHex(comboText(state.controls[idBaseCombo])),
		0x726BD8,
		0x73A574,
		0x7C4424,
		0x7C4450,
		0x7C4644,
	})
}

func groundCandidates() []uintptr {
	candidates := []uintptr{
		parseHex(getControlText(state.controls[idGroundEdit])),
		0x14C,
		0x148,
		0x150,
		0x154,
	}
	for offset := uintptr(0x80); offset <= 0x280; offset += 4 {
		candidates = append(candidates, offset)
	}
	return uniqueOffsets(candidates)
}

func moveTypeCandidates() []uintptr {
	return uniqueOffsets([]uintptr{
		parseHex(getControlText(state.controls[idMoveTypeEdit])),
		0x178,
		0x174,
		0x17C,
		0x180,
	})
}

func flagsCandidates() []uintptr {
	return uniqueOffsets([]uintptr{
		parseHex(getControlText(state.controls[idFlagsEdit])),
		0xF0,
		0xEC,
		0xF4,
	})
}

func uniqueOffsets(in []uintptr) []uintptr {
	seen := make(map[uintptr]bool)
	var out []uintptr
	for _, v := range in {
		if v == 0 || seen[v] {
			continue
		}
		seen[v] = true
		out = append(out, v)
	}
	return out
}

func plausiblePointer(ptr uint32) bool {
	return ptr >= 0x10000 && ptr < 0xF0000000
}

func scoreGroundEntity(value uint32) int {
	if value == INVALID_EHANDLE {
		return 8
	}
	if value != 0 && value < 0x10000000 {
		return 6
	}
	if value == 0 {
		return 1
	}
	return 0
}

func scoreMoveType(value uint32) int {
	if value <= 15 {
		if value == MOVETYPE_LADDER || value == 2 || value == 3 {
			return 8
		}
		return 6
	}
	return 0
}

func scoreFlags(value uint32) int {
	if value < 0x10000 {
		score := 5
		if value&FL_ONGROUND != 0 {
			score += 3
		}
		return score
	}
	return 0
}

func waitForClientBaseTimeout(pid uint32, timeout time.Duration) uintptr {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		base := clientDLLBase(pid)
		if base != 0 {
			return base
		}
		time.Sleep(250 * time.Millisecond)
	}
	return 0
}

func runBhop(cfg settings, stop <-chan struct{}) {
	hProcess, _, _ := procOpenProcess.Call(PROCESS_VM_READ|PROCESS_QUERY_LIMITED_INFO, 0, uintptr(cfg.pid))
	if hProcess == 0 {
		postStatus(statusErrorOpenProcess)
		markStopped()
		return
	}
	defer procCloseHandle.Call(hProcess)

	clientBase, stopped := waitForClientBase(cfg.pid, stop)
	if stopped {
		postStatus(statusStopped)
		markStopped()
		return
	}
	if clientBase == 0 {
		postStatus(statusErrorClientDLL)
		markStopped()
		return
	}
	postStatus(statusRunning)

	ticker := time.NewTicker(cfg.pollInterval)
	defer ticker.Stop()
	var jumpDown bool
	var readFailures uint64
	for {
		select {
		case <-stop:
			if jumpDown {
				postKey(cfg.hwnd, WM_KEYUP)
			}
			return
		case <-ticker.C:
			if !isWindow(cfg.hwnd) {
				if jumpDown {
					postKey(cfg.hwnd, WM_KEYUP)
				}
				postStatus(statusGameClosed)
				markStopped()
				return
			}
			key, _, _ := procGetAsyncKeyState.Call(VK_SPACE)
			if key&0x8000 == 0 {
				if jumpDown {
					postKey(cfg.hwnd, WM_KEYUP)
					jumpDown = false
					updateRuntimeDebug(runtimeDebug{jumpDown: jumpDown, readFailures: readFailures})
				}
				continue
			}
			playerPtr, ok := readUint32(hProcess, clientBase+cfg.playerBaseOffset)
			if !ok || playerPtr == 0 {
				readFailures++
				updateRuntimeDebug(runtimeDebug{jumpDown: jumpDown, readFailures: readFailures})
				continue
			}
			groundEntity, okGround := readUint32(hProcess, uintptr(playerPtr)+cfg.groundOffset)
			moveType, okMoveType := readUint32(hProcess, uintptr(playerPtr)+cfg.moveTypeOffset)
			flags, okFlags := readUint32(hProcess, uintptr(playerPtr)+cfg.mFlagsOffset)
			if !okFlags {
				readFailures++
				updateRuntimeDebug(runtimeDebug{playerPtr: playerPtr, jumpDown: jumpDown, readFailures: readFailures})
				continue
			}
			if !okGround {
				groundEntity = 0
			}
			if !okMoveType {
				moveType = 0
			}
			grounded, groundValid := groundFromEntityHandle(groundEntity)
			if !okGround || !groundValid {
				grounded = legacyCanPressJump(flags)
			}
			onLadder := moveType == MOVETYPE_LADDER
			if grounded && !onLadder && !jumpDown {
				postKey(cfg.hwnd, WM_KEYDOWN)
				jumpDown = true
			} else if (!grounded || onLadder) && jumpDown {
				postKey(cfg.hwnd, WM_KEYUP)
				jumpDown = false
			}
			updateRuntimeDebug(runtimeDebug{
				groundEntity: groundEntity,
				moveType:     moveType,
				flags:        flags,
				onGround:     grounded,
				onLadder:     onLadder,
				jumpDown:     jumpDown,
				readFailures: readFailures,
				playerPtr:    playerPtr,
			})
		}
	}
}

func waitForClientBase(pid uint32, stop <-chan struct{}) (uintptr, bool) {
	for {
		select {
		case <-stop:
			return 0, true
		default:
		}
		base := clientDLLBase(pid)
		if base != 0 {
			return base, false
		}
		time.Sleep(750 * time.Millisecond)
	}
}

func clientDLLBase(pid uint32) uintptr {
	snap, _, _ := procCreateToolhelp32Snapshot.Call(uintptr(TH32CS_SNAPMODULE|TH32CS_SNAPMODULE32), uintptr(pid))
	if snap == INVALID_HANDLE_VALUE || snap == 0 {
		return 0
	}
	defer procCloseHandle.Call(snap)

	var me moduleEntry32
	me.dwSize = uint32(unsafe.Sizeof(me))
	ok, _, _ := procModule32FirstW.Call(snap, uintptr(unsafe.Pointer(&me)))
	for ok != 0 {
		if strings.EqualFold(syscall.UTF16ToString(me.szModule[:]), "client.dll") {
			return me.modBaseAddr
		}
		ok, _, _ = procModule32NextW.Call(snap, uintptr(unsafe.Pointer(&me)))
	}
	return 0
}

func readUint32(process uintptr, addr uintptr) (uint32, bool) {
	var value uint32
	var read uintptr
	ok, _, _ := procReadProcessMemory.Call(process, addr, uintptr(unsafe.Pointer(&value)), unsafe.Sizeof(value), uintptr(unsafe.Pointer(&read)))
	return value, ok != 0 && read == unsafe.Sizeof(value)
}

func legacyCanPressJump(flags uint32) bool {
	return flags != 0x80 && flags != 0x82 && flags != 0x280 && flags != 0x282
}

func groundFromEntityHandle(value uint32) (bool, bool) {
	if value == INVALID_EHANDLE {
		return false, true
	}
	if value != 0 && value < 0x10000000 {
		return true, true
	}
	return false, false
}

func postKey(hwnd uintptr, message uint32) {
	procSendMessageW.Call(hwnd, uintptr(message), VK_SPACE, SPACE_KEY_LPARAM)
}

func markStopped() {
	state.mu.Lock()
	state.running = false
	state.stop = nil
	state.mu.Unlock()
}

func setStatus(code int) {
	text := map[int]string{
		statusIdle:             "就绪：请选择游戏窗口，确认偏移后启动。",
		statusScanning:         "等待游戏窗口或 client.dll 加载...",
		statusRunning:          "运行中：在 L4D2 窗口中按住空格。",
		statusStopped:          "已停止。",
		statusErrorOpenProcess: "无法以只读权限打开游戏进程。",
		statusErrorClientDLL:   "无法读取 client.dll，可能地图尚未加载或游戏已关闭。",
		statusGameClosed:       "游戏窗口已关闭，已停止。",
		statusBadSettings:      "设置无效：请检查进程、偏移和轮询间隔。",
	}[code]
	state.debugMu.Lock()
	state.statusText = text
	state.debugMu.Unlock()
	renderStatus()
}

func postStatus(code int) {
	procPostMessageW.Call(state.hwnd, wmAppStatus, uintptr(code), 0)
}

func setStatusText(text string) {
	state.debugMu.Lock()
	state.statusText = text
	state.debugMu.Unlock()
	renderStatus()
}

func postStatusText(text string) {
	state.debugMu.Lock()
	state.statusText = text
	state.debugMu.Unlock()
	procPostMessageW.Call(state.hwnd, wmAppDebug, 0, 0)
}

func updateRuntimeDebug(debug runtimeDebug) {
	state.debugMu.Lock()
	state.debug = debug
	shouldPost := time.Since(state.lastDebugPost) >= 100*time.Millisecond
	if shouldPost {
		state.lastDebugPost = time.Now()
	}
	state.debugMu.Unlock()
	if shouldPost {
		procPostMessageW.Call(state.hwnd, wmAppDebug, 0, 0)
	}
}

func renderStatus() {
	state.debugMu.Lock()
	status := state.statusText
	debug := state.debug
	state.debugMu.Unlock()
	if status == "" {
		status = "就绪：请选择游戏窗口，确认偏移后启动。"
	}
	text := fmt.Sprintf(
		"%s\r\n地面句柄=0x%X  移动类型=%d  判定在地=%t  梯子=%t\r\n玩家=0x%X  mFlags=0x%X  跳跃按下=%t  读取失败=%d",
		status,
		debug.groundEntity,
		debug.moveType,
		debug.onGround,
		debug.onLadder,
		debug.playerPtr,
		debug.flags,
		debug.jumpDown,
		debug.readFailures,
	)
	setWindowText(state.controls[idStatusText], text)
}

func settingsToSavedConfig(cfg settings, source string, score int, detail string) savedConfig {
	return savedConfig{
		PlayerBase:      hexOffset(cfg.playerBaseOffset),
		GroundEntity:    hexOffset(cfg.groundOffset),
		MoveType:        hexOffset(cfg.moveTypeOffset),
		MFlags:          hexOffset(cfg.mFlagsOffset),
		PollMS:          int(cfg.pollInterval / time.Millisecond),
		UpdatedAt:       time.Now().Format(time.RFC3339),
		Source:          source,
		DetectionScore:  score,
		DetectionDetail: detail,
	}
}

func loadSavedConfig() savedConfig {
	path := configPath()
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		for _, legacyPath := range legacyConfigPaths() {
			data, err = os.ReadFile(legacyPath)
			if err == nil {
				if cfg, parseErr := parseSavedConfig(data); parseErr == nil {
					if saveConfig(cfg) == nil {
						_ = os.Remove(legacyPath)
					}
					return cfg
				}
			}
		}
	}
	if err != nil {
		return savedConfig{}
	}
	cfg, err := parseSavedConfig(data)
	if err != nil {
		return savedConfig{}
	}
	return cfg
}

func parseSavedConfig(data []byte) (savedConfig, error) {
	var cfg savedConfig
	data = bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})
	if err := json.Unmarshal(data, &cfg); err != nil {
		return savedConfig{}, err
	}
	return cfg, nil
}

func applySavedConfig(cfg savedConfig) {
	if cfg.PlayerBase != "" {
		selectComboValue(state.controls[idBaseCombo], cfg.PlayerBase)
	}
	if cfg.GroundEntity != "" {
		setWindowText(state.controls[idGroundEdit], cfg.GroundEntity)
	}
	if cfg.MoveType != "" {
		setWindowText(state.controls[idMoveTypeEdit], cfg.MoveType)
	}
	if cfg.MFlags != "" {
		setWindowText(state.controls[idFlagsEdit], cfg.MFlags)
	}
	if cfg.PollMS > 0 {
		setWindowText(state.controls[idPollEdit], strconv.Itoa(cfg.PollMS))
	}
}

func postApplyConfig(cfg savedConfig) {
	state.mu.Lock()
	state.pendingConfig = cfg
	state.hasPendingConfig = true
	state.mu.Unlock()
	procPostMessageW.Call(state.hwnd, wmAppConfig, 0, 0)
}

func applyPendingConfig() {
	state.mu.Lock()
	if !state.hasPendingConfig {
		state.mu.Unlock()
		return
	}
	cfg := state.pendingConfig
	state.pendingConfig = savedConfig{}
	state.hasPendingConfig = false
	state.mu.Unlock()
	applySavedConfig(cfg)
}

func saveConfig(cfg savedConfig) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	path := configPath()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0644)
}

func configPath() string {
	return filepath.Join(appRoot(), "data", "autobhop-settings.json")
}

func legacyConfigPaths() []string {
	return []string{
		filepath.Join(appRoot(), "data", "l4d2-autobhop-vpk-w.offsets.json"),
		filepath.Join(appRoot(), "l4d2-autobhop-vpk-w.offsets.json"),
	}
}

func appRoot() string {
	if root := strings.TrimSpace(os.Getenv("L4D2_MIX_DATA_ROOT")); root != "" {
		return filepath.Clean(root)
	}
	exe, err := os.Executable()
	if err == nil && exe != "" {
		return filepath.Dir(exe)
	}
	return "."
}

func currentPollMS() int {
	ms, err := strconv.Atoi(strings.TrimSpace(getControlText(state.controls[idPollEdit])))
	if err != nil || ms < 1 {
		return 1
	}
	if ms > 50 {
		return 50
	}
	return ms
}

func selectComboValue(combo uintptr, value string) {
	value = normalizeHexText(value)
	procSendMessageW.Call(combo, CB_RESETCONTENT, 0, 0)
	selected := 0
	values := uniqueHexStrings(append([]string{value}, playerBaseCandidatesStatic()...))
	for i, v := range values {
		sendString(combo, CB_ADDSTRING, v)
		if strings.EqualFold(v, value) {
			selected = i
		}
	}
	procSendMessageW.Call(combo, CB_SETCURSEL, uintptr(selected), 0)
}

func playerBaseCandidatesStatic() []string {
	return []string{"0x726BD8", "0x73A574", "0x7C4424", "0x7C4450", "0x7C4644"}
}

func uniqueHexStrings(in []string) []string {
	seen := make(map[string]bool)
	var out []string
	for _, v := range in {
		v = normalizeHexText(v)
		if v == "" || seen[strings.ToLower(v)] {
			continue
		}
		seen[strings.ToLower(v)] = true
		out = append(out, v)
	}
	return out
}

func hexOffset(v uintptr) string {
	return fmt.Sprintf("0x%X", v)
}

func normalizeHexText(s string) string {
	v := parseHex(s)
	if v == 0 {
		return ""
	}
	return hexOffset(v)
}

func send(hwnd uintptr, msg uint32, wParam, lParam uintptr) uintptr {
	ret, _, _ := procSendMessageW.Call(hwnd, uintptr(msg), wParam, lParam)
	return ret
}

func sendString(hwnd uintptr, msg uint32, text string) {
	procSendMessageW.Call(hwnd, uintptr(msg), 0, uintptr(unsafe.Pointer(utf16Ptr(text))))
}

func getControlText(hwnd uintptr) string {
	length := send(hwnd, WM_GETTEXTLENGTH, 0, 0)
	buf := make([]uint16, length+1)
	send(hwnd, WM_GETTEXT, uintptr(len(buf)), uintptr(unsafe.Pointer(&buf[0])))
	return syscall.UTF16ToString(buf)
}

func comboText(hwnd uintptr) string {
	idx := int(send(hwnd, CB_GETCURSEL, 0, 0))
	if idx < 0 {
		return ""
	}
	return getControlText(hwnd)
}

func setWindowText(hwnd uintptr, text string) {
	procSetWindowTextW.Call(hwnd, uintptr(unsafe.Pointer(utf16Ptr(text))))
}

func getWindowTextLength(hwnd uintptr) int {
	ret, _, _ := procGetWindowTextLengthW.Call(hwnd)
	return int(ret)
}

func getWindowTitle(hwnd uintptr) string {
	n := getWindowTextLength(hwnd)
	buf := make([]uint16, n+1)
	procGetWindowTextW.Call(hwnd, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	return syscall.UTF16ToString(buf)
}

func isWindow(hwnd uintptr) bool {
	ret, _, _ := procIsWindow.Call(hwnd)
	return ret != 0
}

func foregroundWindow() uintptr {
	hwnd, _, _ := procGetForegroundWindow.Call()
	return hwnd
}

func isWindowVisible(hwnd uintptr) bool {
	ret, _, _ := procIsWindowVisible.Call(hwnd)
	return ret != 0
}

func parseHex(s string) uintptr {
	s = strings.TrimSpace(strings.ToLower(s))
	s = strings.TrimPrefix(s, "0x")
	v, err := strconv.ParseUint(s, 16, 64)
	if err != nil {
		return 0
	}
	return uintptr(v)
}

func utf16Ptr(s string) *uint16 {
	p, _ := syscall.UTF16PtrFromString(s)
	return p
}

func copyUTF16(dst []uint16, s string) {
	src := syscall.StringToUTF16(s)
	if len(src) > len(dst) {
		src = src[:len(dst)]
		src[len(src)-1] = 0
	}
	copy(dst, src)
}

func neg(v uintptr) uintptr {
	return ^(v - 1)
}
