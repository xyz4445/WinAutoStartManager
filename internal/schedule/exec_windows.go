//go:build windows

package schedule

import (
	"os/exec"
	"syscall"

	"golang.org/x/sys/windows"
)

// newCommand builds an external command that runs without a console window.
// The GUI process has no console of its own, so without CREATE_NO_WINDOW every
// schtasks call would flash a command prompt.
func newCommand(name string, args ...string) *exec.Cmd {
	cmd := exec.Command(name, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: windows.CREATE_NO_WINDOW,
	}
	return cmd
}
