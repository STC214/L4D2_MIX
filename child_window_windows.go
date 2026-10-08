package main

import "unsafe"

// Revalidate queued readiness notifications: the process may have exited before
// the host dispatches the message, and Windows can recycle window handles.
func childWindowMatches(hwnd, parent uintptr, pid uint32) bool {
	valid, _, _ := user32.NewProc("IsWindow").Call(hwnd)
	if valid == 0 || hwnd == 0 || pid == 0 {
		return false
	}
	var actualPID uint32
	procGetWindowThreadProcessId.Call(hwnd, uintptr(unsafe.Pointer(&actualPID)))
	actualParent, _, _ := user32.NewProc("GetParent").Call(hwnd)
	ready, _, _ := user32.NewProc("GetPropW").Call(hwnd, uintptr(unsafe.Pointer(utf16("L4D2MixReady"))))
	return actualPID == pid && actualParent == parent && ready != 0
}

// Never wait indefinitely on another process's UI thread. Failure retains the
// close veto so a busy/unresponsive MOD component is not killed mid-write.
func sendMessageBounded(hwnd uintptr, message uint32, timeoutMS uint32) (uintptr, bool) {
	if hwnd == 0 || timeoutMS == 0 {
		return 0, false
	}
	var result uintptr
	ok, _, _ := user32.NewProc("SendMessageTimeoutW").Call(hwnd, uintptr(message), 0, 0,
		0x0001|0x0002|0x0020, uintptr(timeoutMS), uintptr(unsafe.Pointer(&result)))
	return result, ok != 0
}
