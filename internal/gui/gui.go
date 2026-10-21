//go:build windows

// Package gui hosts the WebView2 window and exposes Go functions to the page.
package gui

import (
	"fmt"
	"path/filepath"
	"time"

	webview2 "github.com/jchv/go-webview2"

	"WinAutoStartManager/internal/apply"
	"WinAutoStartManager/internal/config"
	"WinAutoStartManager/internal/logging"
	"WinAutoStartManager/internal/settings"
)

// Run opens the GUI and blocks until the window is closed.
func Run() error {
	// This must happen before the first window is created. Otherwise Windows
	// treats the process as DPI-unaware and bitmap-scales the WebView2 client
	// area at every display scale above 100%.
	if err := enablePerMonitorDPI(); err != nil {
		return err
	}

	app, err := NewApp()
	if err != nil {
		return err
	}

	dataDir, err := config.Dir()
	if err != nil {
		return err
	}
	if st, err := settings.Load(dataDir); err == nil {
		_ = logging.Prune(dataDir, time.Now(), st.LogRetentionDays)
	}

	dpi := systemDPI()
	workWidth, workHeight := primaryWorkArea()
	windowOptions := initialWindowOptions(dpi, workWidth, workHeight)
	w := webview2.NewWithOptions(webview2.WebViewOptions{
		AutoFocus:     true,
		DataPath:      filepath.Join(dataDir, "webview2"),
		WindowOptions: windowOptions,
	})
	if w == nil {
		return fmt.Errorf("无法创建窗口：请确认已安装 Microsoft Edge WebView2 运行时")
	}
	defer w.Destroy()

	app.hwnd = uintptr(w.Window())
	applyWindowIcon(app.hwnd, app.exePath)
	installCloseGuard(app, func(newDPI uint32, width, height int) {
		minWidth, minHeight := minSizeForDPI(newDPI, width, height)
		w.SetSize(minWidth, minHeight, webview2.HintMin)
	})

	minWidth, minHeight := minSizeForDPI(dpi, int(windowOptions.Width), int(windowOptions.Height))
	w.SetSize(minWidth, minHeight, webview2.HintMin)

	bindings := []struct {
		name string
		fn   any
	}{
		{"loadApp", func() (Bootstrap, error) { return app.loadApp() }},
		{"saveItems", func(items []config.Item) (Bootstrap, error) { return app.saveItems(items) }},
		{"setAutoStart", func(enabled bool) (apply.Result, error) { return app.setAutoStart(enabled) }},
		{"saveSettings", func(dir string, migrate bool, retention int) (SettingsResult, error) {
			return app.saveSettings(dir, migrate, retention)
		}},
		{"restartApp", func() error { return app.restartApp() }},
		{"runItem", func(it config.Item) (RunResult, error) { return app.runItem(it) }},
		{"setDirty", func(dirty bool) bool { app.setDirty(dirty); return true }},
		{"pickExe", func(current string) (string, error) { return app.pickExe(current) }},
		{"pickDir", func(current string) (string, error) { return app.pickDir(current) }},
		{"openLogs", func() error { return app.openLogs() }},
		{"openDataDir", func() error { return app.openDataDir() }},
	}
	for _, b := range bindings {
		if err := w.Bind(b.name, b.fn); err != nil {
			return fmt.Errorf("注册 %s 失败: %w", b.name, err)
		}
	}

	w.SetHtml(assetsHTML())
	w.Run()
	return nil
}
