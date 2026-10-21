//go:build !windows

package schedule

// decodeANSI maps bytes one-to-one so that arbitrary output still yields valid
// UTF-8 text. The shipped application only runs on Windows.
func decodeANSI(b []byte) (string, bool) {
	r := make([]rune, len(b))
	for i, c := range b {
		r[i] = rune(c)
	}
	return string(r), true
}
