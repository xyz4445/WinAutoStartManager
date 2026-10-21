//go:build windows

package schedule

import (
	"testing"

	"golang.org/x/sys/windows"
)

func TestNewCommandHidesConsoleWindow(t *testing.T) {
	cmd := newCommand("schtasks", "/Query")
	if cmd.SysProcAttr == nil {
		t.Fatal("SysProcAttr must be set so schtasks does not flash a console")
	}
	if !cmd.SysProcAttr.HideWindow {
		t.Fatal("HideWindow must be set")
	}
	if cmd.SysProcAttr.CreationFlags&windows.CREATE_NO_WINDOW == 0 {
		t.Fatal("CREATE_NO_WINDOW must be set")
	}
}
