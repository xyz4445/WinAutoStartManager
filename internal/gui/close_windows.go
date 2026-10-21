//go:build windows

package gui

import (
	"sync/atomic"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	wmClose         = 0x0010
	mbYesNo         = 0x00000004
	mbIconWarning   = 0x00000030
	mbDefButton2    = 0x00000100
	mbSetForeground = 0x00010000
	idYes           = 6
)

var (
	user32                = windows.NewLazySystemDLL("user32.dll")
	procSetWindowLongPtrW = user32.NewProc("SetWindowLongPtrW")
	procSetWindowLongW    = user32.NewProc("SetWindowLongW")
	procCallWindowProcW   = user32.NewProc("CallWindowProcW")
	procDefWindowProcW    = user32.NewProc("DefWindowProcW")
	procSendMessageW      = user32.NewProc("SendMessageW")
	procExtractIconExW    = shell32.NewProc("ExtractIconExW")
	gwlpWndProc           = ^uintptr(3)
)

var (
	closeApp         *App
	oldWindowProc    uintptr
	windowProcEntry  uintptr
	closePrompting   atomic.Bool
	windowDPIChanged func(dpi uint32, width, height int)
)

// installCloseGuard subclasses the top-level window and warns before closing
// with unsaved list edits. Confirming the prompt discards those edits; the
// next launch then shows the last saved list. The same subclass follows the
// rectangle Windows recommends after a monitor DPI change.
func installCloseGuard(app *App, onDPIChanged func(dpi uint32, width, height int)) {
	if app.hwnd == 0 {
		return
	}
	closeApp = app
	windowDPIChanged = onDPIChanged
	windowProcEntry = syscall.NewCallback(closeWindowProc)
	if unsafe.Sizeof(uintptr(0)) == 8 {
		oldWindowProc, _, _ = procSetWindowLongPtrW.Call(app.hwnd, gwlpWndProc, windowProcEntry)
	} else {
		oldWindowProc, _, _ = procSetWindowLongW.Call(app.hwnd, gwlpWndProc, windowProcEntry)
	}
}

func closeWindowProc(hwnd, msg, wparam, lparam uintptr) uintptr {
	if msg == wmDpiChanged {
		applyDPISuggestedRect(hwnd, lparam, uint32(wparam&0xffff))
		return 0
	}
	if msg == wmClose && closeApp != nil && closeApp.hasUnsaved() {
		// The message box runs its own modal loop, so a second close request
		// could otherwise stack another prompt on top of the first one.
		if !closePrompting.CompareAndSwap(false, true) {
			return 0
		}
		text, _ := windows.UTF16PtrFromString("有未保存的更改。\n\n退出后这些修改会丢失，下次启动会使用上次保存的清单。\n\n确定退出吗？")
		caption, _ := windows.UTF16PtrFromString("开机启动管理器")
		ret, _ := windows.MessageBox(
			windows.HWND(hwnd),
			text,
			caption,
			mbYesNo|mbIconWarning|mbDefButton2|mbSetForeground,
		)
		closePrompting.Store(false)
		if ret != idYes {
			return 0
		}
	}
	if oldWindowProc != 0 {
		r, _, _ := procCallWindowProcW.Call(oldWindowProc, hwnd, msg, wparam, lparam)
		return r
	}
	r, _, _ := procDefWindowProcW.Call(hwnd, msg, wparam, lparam)
	return r
}

// applyWindowIcon extracts the first icon embedded in the exe and sets it on
// the window, so the title bar and taskbar use it even before Explorer
// refreshes its icon cache.
func applyWindowIcon(hwnd uintptr, exePath string) {
	if hwnd == 0 || exePath == "" {
		return
	}
	exe, err := windows.UTF16PtrFromString(exePath)
	if err != nil {
		return
	}
	var large, small uintptr
	procExtractIconExW.Call(
		uintptr(unsafe.Pointer(exe)),
		0,
		uintptr(unsafe.Pointer(&large)),
		uintptr(unsafe.Pointer(&small)),
		1,
	)
	if large != 0 {
		procSendMessageW.Call(hwnd, 0x0080, 1, large) // WM_SETICON, ICON_BIG
	}
	if small != 0 {
		procSendMessageW.Call(hwnd, 0x0080, 0, small) // WM_SETICON, ICON_SMALL
	}
}
