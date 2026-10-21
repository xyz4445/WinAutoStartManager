//go:build windows

// Package elevate starts processes through the UAC elevation prompt.
package elevate

import (
	"fmt"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	shell32             = windows.NewLazySystemDLL("shell32.dll")
	procShellExecuteExW = shell32.NewProc("ShellExecuteExW")
)

const (
	seeMaskNoCloseProcess = 0x00000040
	seeMaskNoAsync        = 0x00000100
	swShowNormal          = 1
)

type shellExecuteInfo struct {
	CbSize       uint32
	FMask        uint32
	Hwnd         windows.Handle
	LpVerb       *uint16
	LpFile       *uint16
	LpParameters *uint16
	LpDirectory  *uint16
	NShow        int32
	HInstApp     windows.Handle
	LpIDList     uintptr
	LpClass      *uint16
	HkeyClass    windows.Handle
	DwHotKey     uint32
	HIcon        windows.Handle
	HProcess     windows.Handle
}

// IsElevated reports whether the current process has an elevated token.
func IsElevated() bool {
	var token windows.Token
	if err := windows.OpenProcessToken(windows.CurrentProcess(), windows.TOKEN_QUERY, &token); err != nil {
		return false
	}
	defer token.Close()
	return token.IsElevated()
}

func joinArgs(args []string) string {
	quoted := make([]string, 0, len(args))
	for _, a := range args {
		quoted = append(quoted, syscall.EscapeArg(a))
	}
	return strings.Join(quoted, " ")
}

func shellExecute(verb, file, params, dir string) (windows.Handle, error) {
	verbPtr, err := windows.UTF16PtrFromString(verb)
	if err != nil {
		return 0, err
	}
	filePtr, err := windows.UTF16PtrFromString(file)
	if err != nil {
		return 0, err
	}
	paramsPtr, err := windows.UTF16PtrFromString(params)
	if err != nil {
		return 0, err
	}
	var dirPtr *uint16
	if dir != "" {
		dirPtr, err = windows.UTF16PtrFromString(dir)
		if err != nil {
			return 0, err
		}
	}

	sei := shellExecuteInfo{
		FMask:        seeMaskNoCloseProcess | seeMaskNoAsync,
		LpVerb:       verbPtr,
		LpFile:       filePtr,
		LpParameters: paramsPtr,
		LpDirectory:  dirPtr,
		NShow:        swShowNormal,
	}
	sei.CbSize = uint32(unsafe.Sizeof(sei))

	r, _, callErr := procShellExecuteExW.Call(uintptr(unsafe.Pointer(&sei)))
	if r == 0 {
		if callErr == windows.ERROR_CANCELLED {
			return 0, fmt.Errorf("用户取消了提权")
		}
		return 0, fmt.Errorf("提权启动失败: %v", callErr)
	}
	return sei.HProcess, nil
}

// Run starts exe with administrator rights and does not wait for it.
func Run(exe string, args []string) (uint32, error) {
	return RunRaw(exe, joinArgs(args), "")
}

// RunRaw starts exe with administrator rights using a raw parameter string.
func RunRaw(exe, params, dir string) (uint32, error) {
	h, err := shellExecute("runas", exe, params, dir)
	if err != nil {
		return 0, err
	}
	defer windows.CloseHandle(h)
	pid, err := windows.GetProcessId(h)
	if err != nil {
		return 0, err
	}
	return pid, nil
}

// RunWait starts exe with administrator rights and waits for it to exit.
func RunWait(exe string, args []string) (uint32, error) {
	return RunWaitRaw(exe, joinArgs(args), "")
}

// RunWaitRaw waits for an elevated process started with a raw parameter string.
func RunWaitRaw(exe, params, dir string) (uint32, error) {
	h, err := shellExecute("runas", exe, params, dir)
	if err != nil {
		return 0, err
	}
	defer windows.CloseHandle(h)

	if _, err := windows.WaitForSingleObject(h, windows.INFINITE); err != nil {
		return 0, err
	}
	var code uint32
	if err := windows.GetExitCodeProcess(h, &code); err != nil {
		return 0, err
	}
	return code, nil
}
