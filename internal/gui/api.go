//go:build windows

package gui

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"WinAutoStartManager/internal/appdata"
	"WinAutoStartManager/internal/apply"
	"WinAutoStartManager/internal/config"
	"WinAutoStartManager/internal/elevate"
	"WinAutoStartManager/internal/launcher"
	"WinAutoStartManager/internal/logging"
	"WinAutoStartManager/internal/schedule"
	"WinAutoStartManager/internal/settings"
	"WinAutoStartManager/internal/state"
)

// App holds the state shared by all bindings.
type App struct {
	mu      sync.Mutex
	dataDir string
	exePath string
	logDir  string
	hwnd    uintptr
	dirty   atomic.Bool
}

// setDirty records whether the page has unsaved list edits. The window close
// guard reads it to decide whether to warn.
func (a *App) setDirty(dirty bool) {
	a.dirty.Store(dirty)
}

func (a *App) hasUnsaved() bool {
	return a.dirty.Load()
}

// NewApp builds the GUI application state.
func NewApp() (*App, error) {
	dir, err := config.Dir()
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("创建数据目录失败: %w", err)
	}
	exe, err := os.Executable()
	if err != nil {
		return nil, err
	}
	return &App{dataDir: dir, exePath: exe, logDir: filepath.Join(dir, "logs")}, nil
}

// combineTaskStatus merges the two tiers' states. "missing" is definitive, so
// it wins over an unreadable task; otherwise an unreadable task makes the whole
// state unknown instead of pretending auto-start is off.
func combineTaskStatus(admin, user schedule.Status) schedule.Status {
	if admin == schedule.StatusMissing || user == schedule.StatusMissing {
		return schedule.StatusMissing
	}
	if admin == schedule.StatusUnknown || user == schedule.StatusUnknown {
		return schedule.StatusUnknown
	}
	if admin == schedule.StatusStale || user == schedule.StatusStale {
		return schedule.StatusStale
	}
	return schedule.StatusOK
}

func (a *App) taskStatus() schedule.Status {
	admin, _ := schedule.Check(a.exePath, schedule.TierAdmin, schedule.ExecRunner)
	user, _ := schedule.Check(a.exePath, schedule.TierUser, schedule.ExecRunner)
	return combineTaskStatus(admin, user)
}

func (a *App) loadApp() (Bootstrap, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.loadAppLocked()
}

func (a *App) loadAppLocked() (Bootstrap, error) {
	cfg, err := config.LoadFrom(filepath.Join(a.dataDir, "config.json"))
	if err != nil {
		return Bootstrap{}, err
	}
	st, err := settings.Load(a.dataDir)
	if err != nil {
		return Bootstrap{}, err
	}
	defDir, err := appdata.DefaultRoot()
	if err != nil {
		return Bootstrap{}, err
	}
	runs := map[string]*state.Run{}
	for _, tier := range []string{string(schedule.TierAdmin), string(schedule.TierUser)} {
		r, _ := state.Load(a.dataDir, tier)
		runs[tier] = r
	}
	b := buildBootstrap(cfg, a.taskStatus(), runs)
	b.ConfigPath = filepath.Join(a.dataDir, "config.json")
	b.LogDir = a.logDir
	b.ExePath = a.exePath
	b.DataDir = a.dataDir
	b.DefaultDataDir = defDir
	b.LogRetentionDays = st.LogRetentionDays
	return b, nil
}

func (a *App) saveItems(items []config.Item) (Bootstrap, error) {
	a.mu.Lock()
	for i := range items {
		if strings.TrimSpace(items[i].ID) == "" {
			items[i].ID = config.NewItem("", "").ID
		}
	}
	cfg := &config.Config{Version: 1, Items: items}
	errs := cfg.ValidateForSave()
	var saveErr error
	if len(errs) > 0 {
		saveErr = errors.New(strings.Join(errs, "；"))
	} else {
		saveErr = cfg.SaveTo(filepath.Join(a.dataDir, "config.json"))
	}
	a.mu.Unlock()
	if saveErr != nil {
		return Bootstrap{}, saveErr
	}
	return a.loadApp()
}

// setAutoStart installs or removes both scheduled tasks. The operation runs
// in an elevated helper process, so it may prompt for UAC once.
func (a *App) setAutoStart(enabled bool) (apply.Result, error) {
	a.mu.Lock()
	exe := a.exePath
	a.mu.Unlock()

	f, err := os.CreateTemp("", "autostart-tasks-*.json")
	if err != nil {
		return apply.Result{Error: err.Error()}, nil
	}
	path := f.Name()
	f.Close()
	os.Remove(path)

	args := []string{"--apply-tasks", path}
	if !enabled {
		args = append(args, "--remove-tasks")
	}
	if _, err := elevate.RunWait(exe, args); err != nil {
		os.Remove(path)
		return apply.Result{Error: err.Error()}, nil
	}
	res, err := apply.ReadResult(path)
	os.Remove(path)
	if err != nil {
		return apply.Result{Error: err.Error()}, nil
	}
	return res, nil
}

// SettingsResult is returned after saving settings.
type SettingsResult struct {
	Bootstrap       Bootstrap `json:"bootstrap"`
	RestartRequired bool      `json:"restartRequired"`
}

// saveSettings applies the log-retention window and, when the directory
// changed, migrates the managed files and persists the new location.
func (a *App) saveSettings(dir string, migrate bool, retention int) (SettingsResult, error) {
	newDir, err := appdata.ValidateDir(dir)
	if err != nil {
		return SettingsResult{}, err
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	oldDir := a.dataDir
	restart := !strings.EqualFold(filepath.Clean(oldDir), newDir)
	if restart {
		defDir, defErr := appdata.DefaultRoot()
		toDefault := defErr == nil && strings.EqualFold(filepath.Clean(defDir), newDir)
		if migrate {
			if err := appdata.Migrate(oldDir, newDir, toDefault); err != nil {
				return SettingsResult{}, err
			}
		}
		if err := os.MkdirAll(newDir, 0o755); err != nil {
			return SettingsResult{}, fmt.Errorf("创建数据目录失败: %w", err)
		}
		if toDefault {
			if err := appdata.ClearOverride(); err != nil {
				return SettingsResult{}, err
			}
		} else if err := appdata.SetOverride(newDir); err != nil {
			return SettingsResult{}, err
		}
		a.dataDir = newDir
		a.logDir = filepath.Join(newDir, "logs")
	}

	st := &settings.Settings{LogRetentionDays: retention}
	if err := settings.Save(a.dataDir, st); err != nil {
		return SettingsResult{}, err
	}
	_ = logging.Prune(a.dataDir, time.Now(), st.LogRetentionDays)

	boot, err := a.loadAppLocked()
	if err != nil {
		return SettingsResult{}, err
	}
	return SettingsResult{Bootstrap: boot, RestartRequired: restart}, nil
}

// restartApp starts a fresh instance and exits shortly after. Only used after
// the data directory changed, so the new WebView2 cache path can take effect.
func (a *App) restartApp() error {
	a.mu.Lock()
	exe := a.exePath
	a.mu.Unlock()

	cmd := exec.Command(exe)
	cmd.Dir = filepath.Dir(exe)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("重启程序失败，请手动关闭后重新打开: %w", err)
	}
	go func() {
		time.Sleep(400 * time.Millisecond)
		os.Exit(0)
	}()
	return nil
}

func (a *App) runItem(it config.Item) (RunResult, error) {
	exe := config.ExpandPath(it.Path)
	if _, err := os.Stat(exe); err != nil {
		return RunResult{Error: "程序不存在: " + exe}, nil
	}
	workDir := config.ExpandPath(it.WorkDir)
	if workDir == "" {
		workDir = filepath.Dir(exe)
	}
	if it.Elevated {
		pid, err := elevate.RunRaw(exe, it.Args, workDir)
		if err != nil {
			return RunResult{Error: err.Error()}, nil
		}
		return RunResult{OK: true, PID: pid}, nil
	}
	pid, err := launcher.StartProcess(it)
	if err != nil {
		return RunResult{Error: err.Error()}, nil
	}
	return RunResult{OK: true, PID: pid}, nil
}

func (a *App) openLogs() error {
	if err := os.MkdirAll(a.logDir, 0o755); err != nil {
		return err
	}
	return exec.Command("explorer.exe", a.logDir).Start()
}

func (a *App) openDataDir() error {
	if err := os.MkdirAll(a.dataDir, 0o755); err != nil {
		return err
	}
	return exec.Command("explorer.exe", a.dataDir).Start()
}

func (a *App) pickExe(current string) (string, error) {
	return openFileDialog(a.hwnd, current)
}

func (a *App) pickDir(current string) (string, error) {
	return browseFolderDialog(a.hwnd, current)
}
