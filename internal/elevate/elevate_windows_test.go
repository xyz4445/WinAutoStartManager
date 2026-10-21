//go:build windows

package elevate

import (
	"testing"
	"unsafe"
)

func TestShellExecuteInfoLayout(t *testing.T) {
	// SHELLEXECUTEINFOW is 112 bytes on Win64.
	if got := unsafe.Sizeof(shellExecuteInfo{}); got != 112 {
		t.Fatalf("SHELLEXECUTEINFOW size = %d, want 112", got)
	}
	var sei shellExecuteInfo
	if got := unsafe.Offsetof(sei.LpVerb); got != 16 {
		t.Fatalf("lpVerb offset = %d, want 16", got)
	}
	if got := unsafe.Offsetof(sei.NShow); got != 48 {
		t.Fatalf("nShow offset = %d, want 48", got)
	}
	if got := unsafe.Offsetof(sei.HProcess); got != 104 {
		t.Fatalf("hProcess offset = %d, want 104", got)
	}
}
