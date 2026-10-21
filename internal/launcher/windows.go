//go:build windows

package launcher

import (
	"errors"
	"fmt"
	"path/filepath"
	"unsafe"

	"golang.org/x/sys/windows"

	"WinAutoStartManager/internal/config"
)

// StartProcess launches an item with the current process's elevation level.
func StartProcess(it config.Item) (uint32, error) {
	exe := config.ExpandPath(it.Path)
	if exe == "" {
		return 0, fmt.Errorf("程序路径为空")
	}
	workDir := config.ExpandPath(it.WorkDir)
	if workDir == "" {
		workDir = filepath.Dir(exe)
	}

	cmdLine, err := windows.UTF16PtrFromString(BuildCommandLine(exe, it.Args))
	if err != nil {
		return 0, fmt.Errorf("命令行无效: %w", err)
	}
	dir, err := windows.UTF16PtrFromString(workDir)
	if err != nil {
		return 0, fmt.Errorf("工作目录无效: %w", err)
	}

	var si windows.StartupInfo
	si.Cb = uint32(unsafe.Sizeof(si))
	var pi windows.ProcessInformation

	err = windows.CreateProcess(nil, cmdLine, nil, nil, false, windows.CREATE_NO_WINDOW, nil, dir, &si, &pi)
	if err != nil {
		if errors.Is(err, windows.ERROR_ELEVATION_REQUIRED) {
			return 0, fmt.Errorf("该程序要求管理员权限，请勾选“需要管理员权限”")
		}
		return 0, fmt.Errorf("启动 %s 失败: %w", it.Name, err)
	}
	defer windows.CloseHandle(pi.Thread)
	defer windows.CloseHandle(pi.Process)
	return pi.ProcessId, nil
}
