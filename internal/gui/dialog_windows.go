//go:build windows

package gui

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	comdlg32                 = windows.NewLazySystemDLL("comdlg32.dll")
	shell32                  = windows.NewLazySystemDLL("shell32.dll")
	ole32                    = windows.NewLazySystemDLL("ole32.dll")
	procGetOpenFileNameW     = comdlg32.NewProc("GetOpenFileNameW")
	procSHBrowseForFolderW   = shell32.NewProc("SHBrowseForFolderW")
	procSHGetPathFromIDListW = shell32.NewProc("SHGetPathFromIDListW")
	procCoTaskMemFree        = ole32.NewProc("CoTaskMemFree")
)

type openFileName struct {
	LStructSize       uint32
	HwndOwner         windows.Handle
	HInstance         windows.Handle
	LpstrFilter       *uint16
	LpstrCustomFilter *uint16
	NMaxCustFilter    uint32
	NFilterIndex      uint32
	LpstrFile         *uint16
	NMaxFile          uint32
	LpstrFileTitle    *uint16
	NMaxFileTitle     uint32
	LpstrInitialDir   *uint16
	LpstrTitle        *uint16
	Flags             uint32
	NFileOffset       uint16
	NFileExtension    uint16
	LpstrDefExt       *uint16
	LCustData         uintptr
	LpfnHook          uintptr
	LpTemplateName    *uint16
	PvReserved        unsafe.Pointer
	DwReserved        uint32
	FlagsEx           uint32
}

type browseInfo struct {
	HwndOwner      windows.Handle
	PidlRoot       uintptr
	PszDisplayName *uint16
	LpszTitle      *uint16
	UlFlags        uint32
	Lpfn           uintptr
	LParam         uintptr
	IImage         int32
}

func exeFilter() []uint16 {
	var f []uint16
	for _, part := range []string{
		"可执行文件 (*.exe)", "*.exe",
		"所有文件 (*.*)", "*.*",
	} {
		f = append(f, windows.StringToUTF16(part)...)
	}
	return append(f, 0)
}

func utf16ToGo(buf []uint16) string {
	n := 0
	for n < len(buf) && buf[n] != 0 {
		n++
	}
	return windows.UTF16ToString(buf[:n])
}

// openFileDialog shows the native "open file" dialog. A cancel returns "".
func openFileDialog(owner uintptr, current string) (string, error) {
	filter := exeFilter()
	title, err := windows.UTF16PtrFromString("选择要启动的程序")
	if err != nil {
		return "", err
	}

	var buf [2048]uint16
	if current != "" {
		src := windows.StringToUTF16(current)
		copy(buf[:len(buf)-1], src)
	}

	ofn := openFileName{
		HwndOwner:   windows.Handle(owner),
		LpstrFilter: &filter[0],
		LpstrFile:   &buf[0],
		NMaxFile:    uint32(len(buf)),
		LpstrTitle:  title,
		Flags:       0x00001000 | 0x00000800 | 0x00000008, // FILEMUSTEXIST | PATHMUSTEXIST | NOCHANGEDIR
	}
	ofn.LStructSize = uint32(unsafe.Sizeof(ofn))

	r, _, callErr := procGetOpenFileNameW.Call(uintptr(unsafe.Pointer(&ofn)))
	if r == 0 {
		if errno, ok := callErr.(windows.Errno); ok && errno == 0x00007536 {
			return "", nil // FNERR_INVALIDFILENAME
		}
		return "", nil
	}
	return utf16ToGo(buf[:]), nil
}

// browseFolderDialog shows the native folder picker. A cancel returns "".
func browseFolderDialog(owner uintptr, current string) (string, error) {
	_ = windows.CoInitializeEx(0, windows.COINIT_APARTMENTTHREADED)

	title, err := windows.UTF16PtrFromString("选择工作目录")
	if err != nil {
		return "", err
	}
	var display [260]uint16
	bi := browseInfo{
		HwndOwner:      windows.Handle(owner),
		PszDisplayName: &display[0],
		LpszTitle:      title,
		UlFlags:        0x00000001 | 0x00000040, // RETURNONLYFSDIRS | NEWDIALOGSTYLE
	}

	pidl, _, _ := procSHBrowseForFolderW.Call(uintptr(unsafe.Pointer(&bi)))
	if pidl == 0 {
		return "", nil
	}
	defer procCoTaskMemFree.Call(pidl)

	var path [windows.MAX_PATH]uint16
	ok, _, _ := procSHGetPathFromIDListW.Call(pidl, uintptr(unsafe.Pointer(&path[0])))
	if ok == 0 {
		return "", nil
	}
	return windows.UTF16ToString(path[:]), nil
}
