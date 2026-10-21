//go:build windows

package schedule

import "golang.org/x/sys/windows"

const cpACP = 0 // CP_ACP: the system's current ANSI code page.

// decodeANSI converts bytes written in the system ANSI code page (for example
// GBK on Chinese Windows) to a string.
func decodeANSI(b []byte) (string, bool) {
	if len(b) == 0 {
		return "", true
	}
	n, err := windows.MultiByteToWideChar(cpACP, 0, &b[0], int32(len(b)), nil, 0)
	if err != nil || n <= 0 {
		return "", false
	}
	buf := make([]uint16, n)
	written, err := windows.MultiByteToWideChar(cpACP, 0, &b[0], int32(len(b)), &buf[0], n)
	if err != nil || written <= 0 {
		return "", false
	}
	return windows.UTF16ToString(buf[:written]), true
}
