//go:build windows

package notify

import (
	"testing"
	"unsafe"
)

func TestNotifyIconDataLayout(t *testing.T) {
	// NOTIFYICONDATAW v4 is 976 bytes on Win64.
	if got := unsafe.Sizeof(notifyIconData{}); got != 976 {
		t.Fatalf("NOTIFYICONDATAW size = %d, want 976", got)
	}
	var nid notifyIconData
	if got := unsafe.Offsetof(nid.SzInfo); got != 304 {
		t.Fatalf("szInfo offset = %d, want 304", got)
	}
	if got := unsafe.Offsetof(nid.SzInfoTitle); got != 820 {
		t.Fatalf("szInfoTitle offset = %d, want 820", got)
	}
}
