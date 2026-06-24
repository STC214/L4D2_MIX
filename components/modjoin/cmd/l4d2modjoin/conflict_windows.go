//go:build windows

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"

	"l4d2-mod-join/internal/modscan"
	"l4d2-mod-join/internal/vpkmerge"
)

const (
	idConflictList      = 2101
	idConflictCombo     = 2102
	idConflictRecommend = 2103
	idConflictConfirm   = 2104
	idConflictCancel    = 2105
	idConflictThumbs    = 2106
	lbnSelChange        = 1
	lbsNotify           = 0x0001
	cbnSelChange        = 1
	cbAddString         = 0x0143
	cbGetCurSel         = 0x0147
	cbResetContent      = 0x014B
	cbSetCurSel         = 0x014E
	wsVScroll           = 0x00200000
	cbsDropDownList     = 0x0003
	esMultiline         = 0x0004
	esAutovscroll       = 0x0040
	esReadonly          = 0x0800
	swpNoMove           = 0x0002
	swpNoSize           = 0x0001
	swpShowWindow       = 0x0040
	rdwInvalidate       = 0x0001
	rdwErase            = 0x0004
	rdwAllChildren      = 0x0080
	rdwUpdateNow        = 0x0100
	dtEndEllipsis       = 0x8000

	conflictWindowWidth  = 1120
	conflictWindowHeight = 760
)

type conflictResolver struct {
	hwnd, overlay, list, combo, detail, status, thumbs uintptr
	groups                                             []modscan.ConflictGroup
	selections                                         map[string]string
	addonTitles                                        map[string]string
	thumbFiles                                         map[string]string
	hiddenControls                                     []hiddenControl
	result                                             modscan.Result
	output                                             string
	current                                            int
}

type hiddenControl struct {
	handle  uintptr
	visible bool
}

var resolver *conflictResolver

func registerConflictClass(instance, iconLarge, iconSmall uintptr) {
	className := utf16("L4D2ModJoinConflictWindow")
	wc := wndClassEx{
		Size: uint32(unsafe.Sizeof(wndClassEx{})), WndProc: syscall.NewCallback(conflictWindowProc),
		Instance: instance, Icon: iconLarge, IconSm: iconSmall,
		Background: colorWindow + 1, ClassName: className,
	}
	procRegisterClass.Call(uintptr(unsafe.Pointer(&wc)))
	overlayClass := utf16("L4D2ModJoinConflictOverlay")
	overlay := wndClassEx{
		Size: uint32(unsafe.Sizeof(wndClassEx{})), WndProc: syscall.NewCallback(conflictOverlayProc),
		Instance: instance, Background: colorWindow + 1, ClassName: overlayClass,
	}
	procRegisterClass.Call(uintptr(unsafe.Pointer(&overlay)))
	thumbClass := utf16("L4D2ModJoinThumbPanel")
	thumbs := wndClassEx{
		Size: uint32(unsafe.Sizeof(wndClassEx{})), WndProc: syscall.NewCallback(conflictThumbProc),
		Instance: instance, Background: colorWindow + 1, ClassName: thumbClass,
	}
	procRegisterClass.Call(uintptr(unsafe.Pointer(&thumbs)))
}

func openConflictResolver(groups []modscan.ConflictGroup, existing map[string]string, result modscan.Result, output string) {
	if len(groups) == 0 || resolver != nil {
		return
	}
	selections := map[string]string{}
	for _, group := range groups {
		selected := ""
		for _, path := range group.Paths {
			if contains(group.Packages, existing[path]) {
				selected = existing[path]
				break
			}
		}
		if selected == "" {
			selected = group.Recommended
		}
		for _, path := range group.Paths {
			selections[path] = selected
		}
	}
	resolver = &conflictResolver{
		groups: groups, selections: selections, addonTitles: map[string]string{}, thumbFiles: map[string]string{},
		result: result, output: output, current: 0,
	}
	embedParent := embeddedParent()
	if embedParent == 0 {
		procEnableWindow.Call(ui.hwnd, 0)
	}
	windowStyle := uintptr(wsOverlapped | wsClipChildren)
	parent := ui.hwnd
	x, y := 260, 150
	if embedParent != 0 {
		var client rect
		procGetClientRect.Call(ui.hwnd, uintptr(unsafe.Pointer(&client)))
		hideConflictCoveredControls()
		resolver.overlay = makeControl(
			ui.hwnd, "L4D2ModJoinConflictOverlay", "",
			wsChild|wsVisible|wsClipChildren|wsClipSiblings,
			0, 0, int(client.Right), int(client.Bottom), 0,
		)
		if resolver.overlay == 0 {
			showConflictCoveredControls()
			resolver = nil
			logLine("无法创建冲突处理遮罩层，主页面保持可用。")
			return
		}
		windowStyle = wsChild | wsClipChildren | wsClipSiblings
		parent = resolver.overlay
		x, y = centeredConflictPosition(client)
	}
	hwnd, _, _ := procCreateWindow.Call(
		0,
		uintptr(unsafe.Pointer(utf16("L4D2ModJoinConflictWindow"))),
		uintptr(unsafe.Pointer(utf16("批量处理 MOD 冲突"))),
		windowStyle,
		uintptr(x), uintptr(y), conflictWindowWidth, conflictWindowHeight,
		parent, 0, 0, 0,
	)
	if hwnd == 0 {
		if resolver.overlay != 0 {
			user32.NewProc("DestroyWindow").Call(resolver.overlay)
		}
		showConflictCoveredControls()
		resolver = nil
		if embedParent == 0 {
			procEnableWindow.Call(ui.hwnd, 1)
		}
		logLine("无法创建冲突处理窗口。")
	} else {
		if embeddedParent() == 0 {
			enableDarkTitleBar(hwnd)
		}
		if resolver.overlay != 0 {
			procRedrawWindow.Call(resolver.overlay, 0, 0, rdwInvalidate|rdwErase|rdwAllChildren|rdwUpdateNow)
		}
		procShowWindow.Call(hwnd, swShow)
		procUpdateWindow.Call(hwnd)
		procRedrawWindow.Call(hwnd, 0, 0, rdwInvalidate|rdwErase|rdwAllChildren|rdwUpdateNow)
		if embeddedParent() != 0 {
			if !ensureConflictResolverOnTop() {
				procDestroyWindow := user32.NewProc("DestroyWindow")
				if resolver.overlay != 0 {
					procDestroyWindow.Call(resolver.overlay)
				} else {
					procDestroyWindow.Call(hwnd)
				}
				showConflictCoveredControls()
				resolver = nil
				logLine("冲突处理窗口无法置于标签页前方，已恢复主页面，请重试。")
			}
		}
	}
}

func ensureConflictResolverOnTop() bool {
	if resolver == nil || resolver.hwnd == 0 {
		return true
	}
	layoutConflictResolver()
	if resolver.overlay != 0 {
		if len(resolver.hiddenControls) == 0 {
			hideConflictCoveredControls()
		}
		if result, _, _ := procSetWindowPos.Call(
			resolver.overlay,
			0, // HWND_TOP
			0, 0, 0, 0,
			swpNoMove|swpNoSize|swpShowWindow,
		); result == 0 {
			return false
		}
	}
	result, _, _ := procSetWindowPos.Call(
		resolver.hwnd,
		0, // HWND_TOP
		0, 0, 0, 0,
		swpNoMove|swpNoSize|swpShowWindow,
	)
	if result == 0 {
		return false
	}
	procRedrawWindow.Call(resolver.overlay, 0, 0, rdwInvalidate|rdwErase|rdwAllChildren|rdwUpdateNow)
	procRedrawWindow.Call(resolver.hwnd, 0, 0, rdwInvalidate|rdwErase|rdwAllChildren|rdwUpdateNow)
	procInvalidateRect.Call(resolver.thumbs, 0, 1)
	user32.NewProc("SetFocus").Call(resolver.hwnd)
	return true
}

func centeredConflictPosition(client rect) (int, int) {
	return maxInt(0, (int(client.Right)-conflictWindowWidth)/2),
		maxInt(0, (int(client.Bottom)-conflictWindowHeight)/2)
}

func layoutConflictResolver() {
	if resolver == nil || resolver.hwnd == 0 || resolver.overlay == 0 {
		return
	}
	var client rect
	procGetClientRect.Call(ui.hwnd, uintptr(unsafe.Pointer(&client)))
	procSetWindowPos.Call(
		resolver.overlay,
		0,
		0, 0,
		uintptr(client.Right), uintptr(client.Bottom),
		swpShowWindow,
	)
	x, y := centeredConflictPosition(client)
	procSetWindowPos.Call(
		resolver.hwnd,
		0,
		uintptr(x), uintptr(y),
		conflictWindowWidth, conflictWindowHeight,
		swpShowWindow,
	)
}

func hideConflictCoveredControls() {
	if resolver == nil {
		return
	}
	for _, handle := range []uintptr{ui.scan, ui.merge, ui.deploy, ui.restore} {
		if handle != 0 {
			visible, _, _ := procIsWindowVisible.Call(handle)
			resolver.hiddenControls = append(resolver.hiddenControls, hiddenControl{
				handle:  handle,
				visible: visible != 0,
			})
			procShowWindow.Call(handle, 0)
		}
	}
}

func showConflictCoveredControls() {
	if resolver == nil {
		return
	}
	for _, control := range resolver.hiddenControls {
		if control.handle != 0 && control.visible {
			procShowWindow.Call(control.handle, swShow)
		}
	}
	resolver.hiddenControls = nil
}

func conflictOverlayProc(hwnd uintptr, message uint32, wParam, lParam uintptr) uintptr {
	switch message {
	case wmPaint:
		var ps paintStruct
		hdc, _, _ := procBeginPaint.Call(hwnd, uintptr(unsafe.Pointer(&ps)))
		var client rect
		procGetClientRect.Call(hwnd, uintptr(unsafe.Pointer(&client)))
		procFillRect.Call(hdc, uintptr(unsafe.Pointer(&client)), ui.bgBrush)
		procEndPaint.Call(hwnd, uintptr(unsafe.Pointer(&ps)))
		return 0
	case wmEraseBkgnd:
		var client rect
		procGetClientRect.Call(hwnd, uintptr(unsafe.Pointer(&client)))
		procFillRect.Call(wParam, uintptr(unsafe.Pointer(&client)), ui.bgBrush)
		return 1
	}
	result, _, _ := procDefWindowProc.Call(hwnd, uintptr(message), wParam, lParam)
	return result
}

func conflictWindowProc(hwnd uintptr, message uint32, wParam, lParam uintptr) uintptr {
	switch message {
	case wmCreate:
		resolver.hwnd = hwnd
		createConflictUI(hwnd)
		return 0
	case wmCommand:
		id := int(wParam & 0xffff)
		code := int((wParam >> 16) & 0xffff)
		switch {
		case id == idConflictList && code == lbnSelChange:
			index, _, _ := procSendMessage.Call(resolver.list, 0x0188, 0, 0)
			if int(index) >= 0 && int(index) < len(resolver.groups) {
				resolver.current = int(index)
				refreshConflictDetails()
			}
		case id == idConflictCombo && code == cbnSelChange:
			updateCurrentSelection()
			procInvalidateRect.Call(resolver.thumbs, 0, 1)
		case id == idConflictRecommend:
			applyAllRecommendations()
		case id == idConflictConfirm:
			confirmConflictSelections()
		case id == idConflictCancel:
			closeConflictResolver(false)
		}
		return 0
	case wmCtlColorStatic:
		procSetTextColor.Call(wParam, 0x00E8E4DF)
		procSetBkMode.Call(wParam, 1)
		return ui.bgBrush
	case wmCtlColorEdit, wmCtlColorList:
		procSetTextColor.Call(wParam, 0x00F0ECE8)
		procSetBkColor.Call(wParam, 0x00352E2A)
		return ui.fieldBrush
	case wmPaint:
		paintConflictWindow(hwnd)
		return 0
	case wmEraseBkgnd:
		return 1
	case wmClose:
		closeConflictResolver(false)
		return 0
	case wmDestroy:
		return 0
	}
	result, _, _ := procDefWindowProc.Call(hwnd, uintptr(message), wParam, lParam)
	return result
}

func createConflictUI(hwnd uintptr) {
	makeLabel(hwnd, "以下冲突已按竞争 MOD 聚合。每组选择一次，会应用到该组全部冲突文件。", 30, 96, 1030, 26)
	makeLabel(hwnd, "左侧选择冲突组，下方下拉框选择要保留的版本，中间只显示当前选中 MOD 的大缩略图。", 30, 124, 1030, 24)
	resolver.list = makeControl(hwnd, "LISTBOX", "",
		wsChild|wsVisible|wsBorder|wsVScroll|wsTabStop|lbsNotify|lbsNoIntegral,
		30, 158, 330, 430, idConflictList)
	resolver.thumbs = makeControl(hwnd, "L4D2ModJoinThumbPanel", "",
		wsChild|wsVisible|wsBorder, 390, 158, 330, 430, idConflictThumbs)
	resolver.detail = makeControl(hwnd, "EDIT", "",
		wsChild|wsVisible|wsBorder|wsVScroll|esMultiline|esAutovscroll|esReadonly, 750, 158, 330, 430, 0)
	makeLabel(hwnd, "保留此 MOD 的冲突版本：", 390, 606, 300, 24)
	resolver.combo = makeControl(hwnd, "COMBOBOX", "",
		wsChild|wsVisible|wsBorder|wsTabStop|cbsDropDownList, 390, 632, 330, 200, idConflictCombo)
	resolver.status = makeControl(hwnd, "STATIC", "", wsChild|wsVisible, 750, 612, 330, 58, 0)
	makeButton(hwnd, "全部采用推荐", 30, 690, 170, 42, idConflictRecommend)
	makeButton(hwnd, "取消", 820, 690, 100, 42, idConflictCancel)
	makeButton(hwnd, "确认并开始合并", 934, 690, 150, 42, idConflictConfirm)

	for index, group := range resolver.groups {
		label := fmt.Sprintf("%02d  %d 个冲突文件 · %d 个 MOD", index+1, len(group.Paths), len(group.Packages))
		procSendMessage.Call(resolver.list, lbAddString, 0, uintptr(unsafe.Pointer(utf16(label))))
	}
	procSendMessage.Call(resolver.list, lbSetCurSel, 0, 0)
	refreshConflictDetails()
}

func paintConflictWindow(hwnd uintptr) {
	var ps paintStruct
	hdc, _, _ := procBeginPaint.Call(hwnd, uintptr(unsafe.Pointer(&ps)))
	var client rect
	procGetClientRect.Call(hwnd, uintptr(unsafe.Pointer(&client)))
	width, height := client.Right, client.Bottom
	if width <= 0 || height <= 0 {
		procEndPaint.Call(hwnd, uintptr(unsafe.Pointer(&ps)))
		return
	}
	memoryDC, _, _ := procCreateCompatDC.Call(hdc)
	bitmap, _, _ := procCreateCompatBM.Call(hdc, uintptr(width), uintptr(height))
	oldBitmap, _, _ := procSelectObject.Call(memoryDC, bitmap)
	procFillRect.Call(memoryDC, uintptr(unsafe.Pointer(&client)), ui.bgBrush)
	header, _, _ := procCreateBrush.Call(0x00352B25)
	headerRect := rect{0, 0, width, 76}
	procFillRect.Call(memoryDC, uintptr(unsafe.Pointer(&headerRect)), header)
	procSetBkMode.Call(memoryDC, 1)
	procSetTextColor.Call(memoryDC, 0x00F3F0EC)
	oldFont, _, _ := procSelectObject.Call(memoryDC, ui.titleFont)
	title := "批量处理 MOD 冲突"
	procTextOut.Call(memoryDC, 30, 18, uintptr(unsafe.Pointer(utf16(title))), uintptr(len([]rune(title))))
	procSelectObject.Call(memoryDC, ui.font)
	procSetTextColor.Call(memoryDC, 0x00AFA8A2)
	subtitle := "聚合决策 · 推荐提示 · 一次确认"
	procTextOut.Call(memoryDC, 31, 52, uintptr(unsafe.Pointer(utf16(subtitle))), uintptr(len([]rune(subtitle))))
	procSelectObject.Call(memoryDC, oldFont)
	procBitBlt.Call(hdc, 0, 0, uintptr(width), uintptr(height), memoryDC, 0, 0, 0x00CC0020)
	procSelectObject.Call(memoryDC, oldBitmap)
	procDeleteObject.Call(bitmap)
	procDeleteDC.Call(memoryDC)
	procDeleteObject.Call(header)
	procEndPaint.Call(hwnd, uintptr(unsafe.Pointer(&ps)))
}

func refreshConflictDetails() {
	if resolver == nil || resolver.current < 0 || resolver.current >= len(resolver.groups) {
		return
	}
	group := resolver.groups[resolver.current]
	sample := group.Paths
	if len(sample) > 5 {
		sample = sample[:5]
	}
	detail := fmt.Sprintf(
		"冲突组 %d / %d\r\n\r\n参与 MOD：\r\n%s\r\n\r\n冲突文件：%d 个\r\n%s",
		resolver.current+1, len(resolver.groups),
		strings.Join(group.Packages, "\r\n"),
		len(group.Paths), strings.Join(sample, "\r\n"),
	)
	if len(group.Paths) > len(sample) {
		detail += fmt.Sprintf("\r\n……另有 %d 个", len(group.Paths)-len(sample))
	}
	detail += "\r\n\r\n候选 MOD 详情：\r\n" + strings.Join(conflictPackageSummaries(group, resolver.result), "\r\n\r\n")
	setText(resolver.detail, detail)

	procSendMessage.Call(resolver.combo, cbResetContent, 0, 0)
	selected := resolver.selections[group.Paths[0]]
	selectedIndex := 0
	for index, name := range group.Packages {
		label := name
		if name == group.Recommended {
			label += "  （推荐）"
		}
		procSendMessage.Call(resolver.combo, cbAddString, 0, uintptr(unsafe.Pointer(utf16(label))))
		if name == selected {
			selectedIndex = index
		}
	}
	procSendMessage.Call(resolver.combo, cbSetCurSel, uintptr(selectedIndex), 0)
	setText(resolver.status, "推荐提示：\r\n"+group.Reason)
	procInvalidateRect.Call(resolver.thumbs, 0, 1)
}

func conflictThumbProc(hwnd uintptr, message uint32, wParam, lParam uintptr) uintptr {
	switch message {
	case wmPaint:
		var ps paintStruct
		hdc, _, _ := procBeginPaint.Call(hwnd, uintptr(unsafe.Pointer(&ps)))
		drawConflictThumbnails(hwnd, hdc)
		procEndPaint.Call(hwnd, uintptr(unsafe.Pointer(&ps)))
		return 0
	case wmEraseBkgnd:
		return 1
	}
	result, _, _ := procDefWindowProc.Call(hwnd, uintptr(message), wParam, lParam)
	return result
}

func drawConflictThumbnails(hwnd, hdc uintptr) {
	var client rect
	procGetClientRect.Call(hwnd, uintptr(unsafe.Pointer(&client)))
	procFillRect.Call(hdc, uintptr(unsafe.Pointer(&client)), ui.bgBrush)
	if resolver == nil || resolver.current < 0 || resolver.current >= len(resolver.groups) {
		return
	}
	group := resolver.groups[resolver.current]
	infos := packageInfosByName(resolver.result)
	selected := resolver.selections[group.Paths[0]]
	if !contains(group.Packages, selected) {
		selected = group.Recommended
	}
	graphics := uintptr(0)
	if gdiplusToken != 0 {
		procGdipCreateGraphicsFromHDC.Call(hdc, uintptr(unsafe.Pointer(&graphics)))
	}
	defer func() {
		if graphics != 0 {
			procGdipDeleteGraphics.Call(graphics)
		}
	}()
	drawSelectedThumbnail(hdc, graphics, client, infos[selected], selected)
}

func drawSelectedThumbnail(hdc, graphics uintptr, client rect, info vpkmerge.PackageInfo, name string) {
	card := rect{client.Left + 10, client.Top + 10, client.Right - 10, client.Bottom - 10}
	border, _, _ := procCreateBrush.Call(0x00FF9A50)
	innerBrush, _, _ := procCreateBrush.Call(0x00352E2A)
	procFillRect.Call(hdc, uintptr(unsafe.Pointer(&card)), border)
	inner := rect{card.Left + 2, card.Top + 2, card.Right - 2, card.Bottom - 2}
	procFillRect.Call(hdc, uintptr(unsafe.Pointer(&inner)), innerBrush)
	procDeleteObject.Call(border)
	procDeleteObject.Call(innerBrush)

	imageBox := rect{inner.Left + 10, inner.Top + 10, inner.Right - 10, inner.Bottom - 44}
	if thumb := thumbnailFile(info); thumb != "" && graphics != 0 {
		drawImageFile(graphics, thumb, imageBox)
	} else {
		placeholder, _, _ := procCreateBrush.Call(0x002D2825)
		procFillRect.Call(hdc, uintptr(unsafe.Pointer(&imageBox)), placeholder)
		procDeleteObject.Call(placeholder)
		drawText(hdc, "无预览", imageBox, 0x00AFA8A2, 0)
	}
	label := rect{inner.Left + 10, imageBox.Bottom + 8, inner.Right - 10, inner.Bottom - 6}
	drawText(hdc, name, label, 0x00F0ECE8, dtEndEllipsis)
}

func drawText(hdc uintptr, text string, area rect, color uint32, flags uintptr) {
	procSetBkMode.Call(hdc, 1)
	procSetTextColor.Call(hdc, uintptr(color))
	procSelectObject.Call(hdc, ui.font)
	p := utf16(text)
	procDrawText.Call(hdc, uintptr(unsafe.Pointer(p)), ^uintptr(0), uintptr(unsafe.Pointer(&area)), 0x0001|0x0004|0x0020|flags)
}

func drawImageFile(graphics uintptr, path string, box rect) {
	image := uintptr(0)
	if status, _, _ := procGdipLoadImageFromFile.Call(uintptr(unsafe.Pointer(utf16(path))), uintptr(unsafe.Pointer(&image))); status != 0 || image == 0 {
		return
	}
	defer procGdipDisposeImage.Call(image)
	var width, height uint32
	procGdipGetImageWidth.Call(image, uintptr(unsafe.Pointer(&width)))
	procGdipGetImageHeight.Call(image, uintptr(unsafe.Pointer(&height)))
	if width == 0 || height == 0 {
		return
	}
	target := fitRect(box, int32(width), int32(height))
	procGdipDrawImageRectI.Call(
		graphics, image,
		uintptr(target.Left), uintptr(target.Top),
		uintptr(target.Right-target.Left), uintptr(target.Bottom-target.Top),
	)
}

func fitRect(box rect, imageW, imageH int32) rect {
	boxW, boxH := box.Right-box.Left, box.Bottom-box.Top
	if boxW <= 0 || boxH <= 0 || imageW <= 0 || imageH <= 0 {
		return box
	}
	drawW, drawH := boxW, imageH*boxW/imageW
	if drawH > boxH {
		drawH = boxH
		drawW = imageW * boxH / imageH
	}
	left := box.Left + (boxW-drawW)/2
	top := box.Top + (boxH-drawH)/2
	return rect{left, top, left + drawW, top + drawH}
}

func packageInfosByName(result modscan.Result) map[string]vpkmerge.PackageInfo {
	infos := map[string]vpkmerge.PackageInfo{}
	for _, info := range result.Packages {
		infos[filepath.Base(info.Path)] = info
	}
	return infos
}

func thumbnailFile(info vpkmerge.PackageInfo) string {
	if resolver == nil || info.Path == "" {
		return ""
	}
	if cached, ok := resolver.thumbFiles[info.Path]; ok {
		if cached == "-" {
			return ""
		}
		return cached
	}
	for _, candidate := range []struct {
		path string
		ext  string
	}{
		{"addonimage.jpg", ".jpg"},
		{"addonimage.png", ".png"},
		{"addonimage.jpeg", ".jpg"},
	} {
		data, err := vpkmerge.ReadFile(info.Path, candidate.path)
		if err != nil || len(data) == 0 {
			continue
		}
		cacheDir := filepath.Join(ui.stateDir, "thumb-cache")
		if err := os.MkdirAll(cacheDir, 0755); err != nil {
			break
		}
		name := info.Digest
		if name == "" {
			name = strings.TrimSuffix(filepath.Base(info.Path), filepath.Ext(info.Path))
		}
		thumb := filepath.Join(cacheDir, safeCacheName(name)+candidate.ext)
		if _, err := os.Stat(thumb); os.IsNotExist(err) {
			if writeErr := os.WriteFile(thumb, data, 0644); writeErr != nil {
				break
			}
		}
		resolver.thumbFiles[info.Path] = thumb
		return thumb
	}
	resolver.thumbFiles[info.Path] = "-"
	return ""
}

func safeCacheName(value string) string {
	var b strings.Builder
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			b.WriteRune(r)
		}
	}
	if b.Len() == 0 {
		return "thumb"
	}
	return b.String()
}

func conflictPackageSummaries(group modscan.ConflictGroup, result modscan.Result) []string {
	infos := packageInfosByName(result)
	categoryByPackage := map[string]string{}
	for _, category := range result.Categories {
		for _, name := range category.Packages {
			categoryByPackage[name] = category.Title
		}
	}
	summaries := make([]string, 0, len(group.Packages))
	for _, name := range group.Packages {
		info := infos[name]
		lines := []string{name}
		if title := addonTitleCached(info.Path); title != "" {
			lines = append(lines, "  标题: "+title)
		}
		if info.Path != "" {
			lines = append(lines, "  路径: "+info.Path)
			if stat, err := os.Stat(info.Path); err == nil {
				lines = append(lines, "  大小: "+formatBytes(stat.Size()))
			}
		}
		if category := categoryByPackage[name]; category != "" {
			lines = append(lines, "  分类: "+category)
		}
		lines = append(lines, fmt.Sprintf("  包内文件: %d", len(info.Files)))
		lines = append(lines, fmt.Sprintf("  本组冲突命中: %d/%d", countCoveredPaths(info, group.Paths), len(group.Paths)))
		if visuals := visualHints(info, 4); len(visuals) > 0 {
			lines = append(lines, "  图片/贴图资源: "+strings.Join(visuals, " ; "))
		} else {
			lines = append(lines, "  图片/贴图资源: 未发现 addonimage 或常见材质预览文件")
		}
		if name == group.Recommended {
			lines = append(lines, "  当前推荐: 是")
		}
		summaries = append(summaries, strings.Join(lines, "\r\n"))
	}
	return summaries
}

func addonTitleCached(path string) string {
	if path == "" {
		return ""
	}
	if resolver != nil && resolver.addonTitles != nil {
		if title, ok := resolver.addonTitles[path]; ok {
			return title
		}
	}
	title := addonTitle(path)
	if resolver != nil && resolver.addonTitles != nil {
		resolver.addonTitles[path] = title
	}
	return title
}

func addonTitle(path string) string {
	data, err := vpkmerge.ReadFile(path, "addoninfo.txt")
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		lower := strings.ToLower(line)
		if !strings.HasPrefix(lower, "addontitle") {
			continue
		}
		value := strings.TrimSpace(line[len("addontitle"):])
		return strings.Trim(value, "\"")
	}
	return ""
}

func countCoveredPaths(info vpkmerge.PackageInfo, paths []string) int {
	owned := map[string]bool{}
	for _, file := range info.Files {
		owned[file.Path] = true
	}
	count := 0
	for _, path := range paths {
		if owned[path] {
			count++
		}
	}
	return count
}

func visualHints(info vpkmerge.PackageInfo, limit int) []string {
	if limit <= 0 {
		return nil
	}
	var hints []string
	for _, file := range info.Files {
		path := strings.ToLower(file.Path)
		if path == "addonimage.jpg" || path == "addonimage.png" || strings.HasPrefix(path, "materials/vgui/") {
			hints = append(hints, file.Path)
		} else if strings.HasPrefix(path, "materials/") && (strings.HasSuffix(path, ".vtf") || strings.HasSuffix(path, ".vmt")) {
			hints = append(hints, file.Path)
		}
		if len(hints) >= limit {
			break
		}
	}
	if len(hints) == limit {
		hints = append(hints, "...")
	}
	return hints
}

func formatBytes(size int64) string {
	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}
	value := float64(size)
	for _, suffix := range []string{"KB", "MB", "GB"} {
		value /= unit
		if value < unit {
			return fmt.Sprintf("%.1f %s", value, suffix)
		}
	}
	return fmt.Sprintf("%.1f TB", value/unit)
}

func updateCurrentSelection() {
	if resolver == nil || resolver.current >= len(resolver.groups) {
		return
	}
	index, _, _ := procSendMessage.Call(resolver.combo, cbGetCurSel, 0, 0)
	group := resolver.groups[resolver.current]
	if int(index) < 0 || int(index) >= len(group.Packages) {
		return
	}
	selected := group.Packages[int(index)]
	for _, path := range group.Paths {
		resolver.selections[path] = selected
	}
}

func applyAllRecommendations() {
	for _, group := range resolver.groups {
		for _, path := range group.Paths {
			resolver.selections[path] = group.Recommended
		}
	}
	refreshConflictDetails()
	if resolver.current >= 0 && resolver.current < len(resolver.groups) {
		group := resolver.groups[resolver.current]
		for index, name := range group.Packages {
			if name == group.Recommended {
				procSendMessage.Call(resolver.combo, cbSetCurSel, uintptr(index), 0)
				break
			}
		}
	}
	procInvalidateRect.Call(resolver.thumbs, 0, 1)
	setText(resolver.status, "已为全部冲突组选择推荐方案。\r\n你仍可逐组调整，然后一次确认。")
}

func confirmConflictSelections() {
	updateCurrentSelection()
	if err := saveConflictGroupSelections(resolver.output, resolver.result, resolver.selections); err != nil {
		setText(resolver.status, "保存失败："+err.Error())
		return
	}
	closeConflictResolver(true)
}

func closeConflictResolver(startMerge bool) {
	if resolver == nil {
		return
	}
	hwnd := resolver.hwnd
	overlay := resolver.overlay
	showConflictCoveredControls()
	resolver = nil
	procDestroyWindow := user32.NewProc("DestroyWindow")
	if overlay != 0 {
		procDestroyWindow.Call(overlay)
	} else {
		procDestroyWindow.Call(hwnd)
		procEnableWindow.Call(ui.hwnd, 1)
	}
	user32.NewProc("SetForegroundWindow").Call(ui.hwnd)
	if startMerge {
		postEvent(appEvent{Kind: "merge-ready"})
	} else {
		logLine("已取消冲突选择，未开始合并。")
	}
}
