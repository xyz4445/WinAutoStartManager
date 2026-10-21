//go:build windows

// Package notify shows a transient Windows balloon notification.
package notify

import (
	"fmt"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	user32   = windows.NewLazySystemDLL("user32.dll")
	shell32  = windows.NewLazySystemDLL("shell32.dll")
	kernel32 = windows.NewLazySystemDLL("kernel32.dll")

	procRegisterClassExW = user32.NewProc("RegisterClassExW")
	procCreateWindowExW  = user32.NewProc("CreateWindowExW")
	procDestroyWindow    = user32.NewProc("DestroyWindow")
	procDefWindowProcW   = user32.NewProc("DefWindowProcW")
	procLoadIconW        = user32.NewProc("LoadIconW")
	procShellNotifyIconW = shell32.NewProc("Shell_NotifyIconW")
)

const (
	nimAdd    = 0x00000000
	nimDelete = 0x00000002

	nifIcon = 0x00000002
	nifTip  = 0x00000004
	nifInfo = 0x00000010

	niifWarning = 0x00000002

	defaultIcon = 32515 // IDI_WARNING
	iconUID     = 1
)

type wndClassEx struct {
	CbSize        uint32
	Style         uint32
	LpfnWndProc   uintptr
	CbClsExtra    int32
	CbWndExtra    int32
	HInstance     windows.Handle
	HIcon         windows.Handle
	HCursor       windows.Handle
	HbrBackground windows.Handle
	LpszMenuName  *uint16
	LpszClassName *uint16
	HIconSm       windows.Handle
}

type notifyIconData struct {
	CbSize           uint32
	HWnd             windows.Handle
	UID              uint32
	UFlags           uint32
	UCallbackMessage uint32
	HIcon            windows.Handle
	SzTip            [128]uint16
	DwState          uint32
	DwStateMask      uint32
	SzInfo           [256]uint16
	UVersion         uint32
	SzInfoTitle      [64]uint16
	DwInfoFlags      uint32
	GuidItem         windows.GUID
	HBalloonIcon     windows.Handle
}

var wndProc = windows.NewCallback(func(hwnd, msg, wp, lp uintptr) uintptr {
	r, _, _ := procDefWindowProcW.Call(hwnd, msg, wp, lp)
	return r
})

func copyUTF16(dst []uint16, s string) {
	src := windows.StringToUTF16(s)
	n := len(dst) - 1
	if len(src)-1 < n {
		n = len(src) - 1
	}
	copy(dst[:n], src[:n])
	dst[n] = 0
}

func createHiddenWindow(className string) (windows.Handle, error) {
	var hInstance windows.Handle
	if err := windows.GetModuleHandleEx(0, nil, &hInstance); err != nil {
		return 0, err
	}
	class, _ := windows.UTF16PtrFromString(className)
	wc := wndClassEx{
		CbSize:        uint32(unsafe.Sizeof(wndClassEx{})),
		LpfnWndProc:   wndProc,
		HInstance:     hInstance,
		LpszClassName: class,
	}
	if r, _, err := procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc))); r == 0 {
		// Class may already be registered by an earlier call in this process.
		if errno, ok := err.(windows.Errno); !ok || errno != 1410 { // ERROR_CLASS_ALREADY_EXISTS
			return 0, fmt.Errorf("注册通知窗口类失败: %v", err)
		}
	}
	title, _ := windows.UTF16PtrFromString("AutoStartManager")
	hwnd, _, err := procCreateWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(class)),
		uintptr(unsafe.Pointer(title)),
		0,
		0, 0, 0, 0,
		0, 0,
		uintptr(hInstance),
		0,
	)
	if hwnd == 0 {
		return 0, fmt.Errorf("创建通知窗口失败: %v", err)
	}
	return windows.Handle(hwnd), nil
}

// Show displays a balloon notification for about six seconds.
func Show(title, message string) error {
	hwnd, err := createHiddenWindow("AutoStartManagerNotify")
	if err != nil {
		return err
	}
	defer procDestroyWindow.Call(uintptr(hwnd))

	icon, _, _ := procLoadIconW.Call(0, uintptr(defaultIcon))

	var nid notifyIconData
	nid.CbSize = uint32(unsafe.Sizeof(nid))
	nid.HWnd = hwnd
	nid.UID = iconUID
	nid.UFlags = nifIcon | nifTip | nifInfo
	nid.HIcon = windows.Handle(icon)
	nid.UVersion = 0
	nid.DwInfoFlags = niifWarning
	copyUTF16(nid.SzTip[:], title)
	copyUTF16(nid.SzInfo[:], message)
	copyUTF16(nid.SzInfoTitle[:], title)

	if r, _, err := procShellNotifyIconW.Call(nimAdd, uintptr(unsafe.Pointer(&nid))); r == 0 {
		return fmt.Errorf("显示通知失败: %v", err)
	}
	time.Sleep(6 * time.Second)
	procShellNotifyIconW.Call(nimDelete, uintptr(unsafe.Pointer(&nid)))
	return nil
}
