//go:build windows

package gui

import (
	"testing"
	"unsafe"
)

func TestNativeDialogStructSizes(t *testing.T) {
	// Win64 layouts: OPENFILENAMEW is 152 bytes, BROWSEINFOW is 56 bytes.
	if got := unsafe.Sizeof(openFileName{}); got != 152 {
		t.Fatalf("OPENFILENAMEW size = %d, want 152", got)
	}
	if got := unsafe.Sizeof(browseInfo{}); got != 64 {
		t.Fatalf("BROWSEINFOW size = %d, want 64", got)
	}
}

func TestNativeDialogStructOffsets(t *testing.T) {
	var ofn openFileName
	if got := unsafe.Offsetof(ofn.LpstrFile); got != 48 {
		t.Fatalf("lpstrFile offset = %d, want 48", got)
	}
	if got := unsafe.Offsetof(ofn.NMaxFile); got != 56 {
		t.Fatalf("nMaxFile offset = %d, want 56", got)
	}
	if got := unsafe.Offsetof(ofn.Flags); got != 96 {
		t.Fatalf("flags offset = %d, want 96", got)
	}

	var bi browseInfo
	if got := unsafe.Offsetof(bi.UlFlags); got != 32 {
		t.Fatalf("ulFlags offset = %d, want 32", got)
	}
}
