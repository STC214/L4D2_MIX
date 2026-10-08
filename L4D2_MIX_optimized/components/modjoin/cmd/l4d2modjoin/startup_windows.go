//go:build windows

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"unsafe"
)

type startupState struct {
	SettingsError          bool
	Source, Output, Addons string
	Settings               appSettings
	Messages               []string
}

func prepareStartupState(root, stateDir, base string, detect func() string) startupState {
	result := startupState{Source: filepath.Join(base, "workshop"), Output: filepath.Join(base, "merged")}
	n, err := migrateRootStateFiles(root, stateDir)
	if err != nil {
		result.Messages = append(result.Messages, "状态迁移未完全完成："+err.Error())
	} else if n > 0 {
		result.Messages = append(result.Messages, fmt.Sprintf("已迁移 %d 个状态文件。", n))
	}
	if _, err = migrateDeploymentRegistry(stateDir); err != nil {
		result.Messages = append(result.Messages, "部署记录迁移失败："+err.Error())
	}
	settings, err := loadAppSettings(stateDir)
	if err != nil {
		result.SettingsError = true
		result.Messages = append(result.Messages, err.Error())
		settings = appSettings{Version: 1, WeaponSoundVolumePercent: 100}
	}
	result.Settings = settings
	// A saved location is authoritative. Avoid probing C-Z on every startup.
	result.Addons = settings.Addons
	if result.Addons == "" {
		startupMark("directory_detect_begin")
		result.Addons = detect()
		startupMark("directory_detect_end")
	}
	if settings.Source != "" {
		result.Source = settings.Source
	} else if result.Addons != "" {
		candidate := filepath.Join(result.Addons, "workshop")
		if info, statErr := os.Stat(candidate); statErr == nil && info.IsDir() {
			result.Source = candidate
		}
	}
	if settings.Output != "" {
		result.Output = settings.Output
	}
	if result.Addons == "" {
		result.Messages = append(result.Messages, "未自动找到游戏目录，请手动选择。")
	}
	return result
}
func setStartupInputs(enabled bool) {
	value := uintptr(0)
	if enabled {
		value = 1
	}
	for _, handle := range []uintptr{ui.source, ui.output, ui.addons, ui.weaponVolume, ui.weaponCustom} {
		procEnableWindow.Call(handle, value)
	}
	getDlgItem := user32.NewProc("GetDlgItem")
	for _, id := range []uintptr{idBrowseSrc, idBrowseOut, idBrowseGame} {
		h, _, _ := getDlgItem.Call(ui.hwnd, id)
		procEnableWindow.Call(h, value)
	}
}

func startupFixedDrives() []string {
	kernel := syscall.NewLazyDLL("kernel32.dll")
	mask, _, _ := kernel.NewProc("GetLogicalDrives").Call()
	getType := kernel.NewProc("GetDriveTypeW")
	var drives []string
	for i := 0; i < 26; i++ {
		if mask&(1<<i) == 0 {
			continue
		}
		root := fmt.Sprintf("%c:\\", 'A'+i)
		ptr, _ := syscall.UTF16PtrFromString(root)
		kind, _, _ := getType.Call(uintptr(unsafe.Pointer(ptr)))
		// Fixed disks only: network/removable probes can block on inaccessible media.
		if kind == 3 {
			drives = append(drives, root)
		}
	}
	return drives
}
