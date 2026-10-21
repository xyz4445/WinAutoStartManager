//go:build windows

package gui

import (
	"errors"
	"fmt"
	"sync"
	"unsafe"

	webview2 "github.com/jchv/go-webview2"
	"golang.org/x/sys/windows"
)

const (
	logicalWindowWidth  = 1000
	logicalWindowHeight = 740
	logicalMinWidth     = 860
	logicalMinHeight    = 560
	workAreaPercent     = 90
	baseDPI             = 96
)

const (
	processPerMonitorDPIAware = 2
	// DPI_AWARENESS_CONTEXT_PER_MONITOR_AWARE_V2 is the pseudo-handle -4.
	dpiAwarenessContextPerMonitorV2 = ^uintptr(3)
	spiGetWorkArea                  = 0x0030
	logPixelsX                      = 88
)

type windowMetrics struct {
	Width     int
	Height    int
	MinWidth  int
	MinHeight int
}

type nativeRect struct {
	Left   int32
	Top    int32
	Right  int32
	Bottom int32
}

var (
	shcore = windows.NewLazySystemDLL("shcore.dll")

	procSetProcessDpiAwarenessContext = user32.NewProc("SetProcessDpiAwarenessContext")
	procGetDpiForSystem               = user32.NewProc("GetDpiForSystem")
	procGetDC                         = user32.NewProc("GetDC")
	procReleaseDC                     = user32.NewProc("ReleaseDC")
	procSystemParametersInfoW         = user32.NewProc("SystemParametersInfoW")
	procSetWindowPos                  = user32.NewProc("SetWindowPos")

	procSetProcessDpiAwareness = shcore.NewProc("SetProcessDpiAwareness")
	procGetProcessDpiAwareness = shcore.NewProc("GetProcessDpiAwareness")

	gdi32             = windows.NewLazySystemDLL("gdi32.dll")
	procGetDeviceCaps = gdi32.NewProc("GetDeviceCaps")

	ntdll             = windows.NewLazySystemDLL("ntdll.dll")
	procRtlMoveMemory = ntdll.NewProc("RtlMoveMemory")

	enableDPIAwarenessOnce sync.Once
	enableDPIAwarenessErr  error
)

// enablePerMonitorDPI tells Windows to let this process draw the WebView2
// client area at the native resolution of the monitor. Without this, Windows
// bitmap-scales the whole client area at every display scale other than 100%,
// making all page text and icons blurry.
func enablePerMonitorDPI() error {
	enableDPIAwarenessOnce.Do(func() {
		enableDPIAwarenessErr = setPerMonitorDPI()
	})
	return enableDPIAwarenessErr
}

func setPerMonitorDPI() error {
	var firstErr error

	if err := procSetProcessDpiAwarenessContext.Find(); err == nil {
		ok, _, callErr := procSetProcessDpiAwarenessContext.Call(dpiAwarenessContextPerMonitorV2)
		if ok != 0 {
			return nil
		}
		if awareness, err := processDPIAwareness(); err == nil && awareness == processPerMonitorDPIAware {
			return nil
		}
		firstErr = callErr
	}

	if err := procSetProcessDpiAwareness.Find(); err == nil {
		hr, _, callErr := procSetProcessDpiAwareness.Call(processPerMonitorDPIAware)
		if int32(hr) >= 0 {
			return nil
		}
		if awareness, err := processDPIAwareness(); err == nil && awareness == processPerMonitorDPIAware {
			return nil
		}
		if firstErr == nil {
			firstErr = callErr
		}
	}

	if firstErr == nil {
		firstErr = errors.New("系统不支持 Per-Monitor DPI 感知")
	}
	return fmt.Errorf("启用 Per-Monitor DPI 失败: %w", firstErr)
}

func processDPIAwareness() (int32, error) {
	if err := procGetProcessDpiAwareness.Find(); err != nil {
		return 0, err
	}
	var awareness int32
	hr, _, _ := procGetProcessDpiAwareness.Call(0, uintptr(unsafe.Pointer(&awareness)))
	if int32(hr) < 0 {
		return 0, fmt.Errorf("GetProcessDpiAwareness 返回 0x%08x", uint32(hr))
	}
	return awareness, nil
}

// systemDPI returns the DPI of the primary display. It must be called after
// enablePerMonitorDPI so the result is not virtualized to 96 DPI.
func systemDPI() uint32 {
	if err := procGetDpiForSystem.Find(); err == nil {
		dpi, _, _ := procGetDpiForSystem.Call()
		if dpi >= baseDPI && dpi <= 960 {
			return uint32(dpi)
		}
	}

	hdc, _, _ := procGetDC.Call(0)
	if hdc != 0 {
		defer procReleaseDC.Call(0, hdc)
		dpi, _, _ := procGetDeviceCaps.Call(hdc, logPixelsX)
		if dpi >= baseDPI && dpi <= 960 {
			return uint32(dpi)
		}
	}
	return baseDPI
}

// primaryWorkArea returns the primary monitor work area in physical pixels.
func primaryWorkArea() (int, int) {
	var r nativeRect
	ok, _, _ := procSystemParametersInfoW.Call(
		spiGetWorkArea,
		0,
		uintptr(unsafe.Pointer(&r)),
		0,
	)
	if ok == 0 {
		return 0, 0
	}
	return int(r.Right - r.Left), int(r.Bottom - r.Top)
}

func scaledByDPI(value int, dpi uint32) int {
	if dpi == 0 {
		dpi = baseDPI
	}
	return (value*int(dpi) + baseDPI/2) / baseDPI
}

// windowMetricsForDPI keeps the window at the same logical size on every
// display scale, while shrinking it enough to fit inside the monitor work
// area. The minimum size is scaled too and can never exceed the initial size.
func windowMetricsForDPI(dpi uint32, workWidth, workHeight int) windowMetrics {
	if dpi == 0 {
		dpi = baseDPI
	}
	m := windowMetrics{
		Width:     scaledByDPI(logicalWindowWidth, dpi),
		Height:    scaledByDPI(logicalWindowHeight, dpi),
		MinWidth:  scaledByDPI(logicalMinWidth, dpi),
		MinHeight: scaledByDPI(logicalMinHeight, dpi),
	}
	if workWidth > 0 && workHeight > 0 {
		maxWidth := workWidth * workAreaPercent / 100
		maxHeight := workHeight * workAreaPercent / 100
		if m.Width > maxWidth {
			m.Width = maxWidth
		}
		if m.Height > maxHeight {
			m.Height = maxHeight
		}
	}
	if m.MinWidth > m.Width {
		m.MinWidth = m.Width
	}
	if m.MinHeight > m.Height {
		m.MinHeight = m.Height
	}
	return m
}

func initialWindowOptions(dpi uint32, workWidth, workHeight int) webview2.WindowOptions {
	m := windowMetricsForDPI(dpi, workWidth, workHeight)
	return webview2.WindowOptions{
		Title:  "开机启动管理器",
		Width:  uint(m.Width),
		Height: uint(m.Height),
		Center: true,
	}
}

func minSizeForDPI(dpi uint32, windowWidth, windowHeight int) (int, int) {
	m := windowMetricsForDPI(dpi, 0, 0)
	if m.MinWidth > windowWidth {
		m.MinWidth = windowWidth
	}
	if m.MinHeight > windowHeight {
		m.MinHeight = windowHeight
	}
	return m.MinWidth, m.MinHeight
}

const (
	wmDpiChanged  = 0x02E0
	swpNoZOrder   = 0x0004
	swpNoActivate = 0x0010
)

// applyDPISuggestedRect follows the rectangle recommended by Windows when the
// window moves to a monitor with a different scale. The minimum size must be
// updated before SetWindowPos: when moving from 200% to 100%, the old minimum
// is larger than the suggested rectangle and would otherwise prevent the
// window from shrinking. Resizing the window then sends WM_SIZE, which makes
// go-webview2 resize its WebView2 controller as well.
func applyDPISuggestedRect(hwnd, lparam uintptr, dpi uint32) (int, int) {
	if lparam == 0 || procRtlMoveMemory.Find() != nil {
		return 0, 0
	}
	// The window procedure receives lParam as uintptr. Copy it through
	// RtlMoveMemory instead of casting it directly to a Go pointer; go vet
	// rightly rejects the latter.
	var r nativeRect
	procRtlMoveMemory.Call(uintptr(unsafe.Pointer(&r)), lparam, unsafe.Sizeof(r))
	width := int(r.Right - r.Left)
	height := int(r.Bottom - r.Top)
	if width <= 0 || height <= 0 {
		return 0, 0
	}
	if windowDPIChanged != nil {
		windowDPIChanged(dpi, width, height)
	}
	procSetWindowPos.Call(
		hwnd,
		0,
		uintptr(r.Left),
		uintptr(r.Top),
		uintptr(width),
		uintptr(height),
		swpNoZOrder|swpNoActivate,
	)
	return width, height
}
