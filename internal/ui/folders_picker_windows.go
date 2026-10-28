//go:build windows

package ui

import (
	"fmt"
	"unsafe"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver"
	"golang.org/x/sys/windows"
)

const (
	// bifReturnOnlyFSDirs hides files: the caller wants one folder.
	bifReturnOnlyFSDirs = 0x00000001
	// bifEditBox lets the user paste a path, the same escape hatch the Fyne
	// fallback offers as "Enter path…".
	bifEditBox = 0x00000010
	// bifNewDialogStyle is the resizable dialog with a "New folder" button.
	bifNewDialogStyle = 0x00000040
	bifUseNewUI       = bifNewDialogStyle | bifEditBox

	maxPath     = 260
	maxLongPath = 32768

	// coinitApartmentThreaded is required by the new-style folder dialog.
	coinitApartmentThreaded = 0x2
	// comSOK and comSFalse are the HRESULTs that own a CoUninitialize call;
	// RPC_E_CHANGED_MODE means another library already initialized the thread.
	comSOK    = 0
	comSFalse = 1
)

var (
	shell32                 = windows.NewLazySystemDLL("shell32.dll")
	ole32                   = windows.NewLazySystemDLL("ole32.dll")
	procSHBrowseForFolderW  = shell32.NewProc("SHBrowseForFolderW")
	procSHGetPathFromIDList = shell32.NewProc("SHGetPathFromIDListW")
	procSHParseDisplayName  = shell32.NewProc("SHParseDisplayName")
	procCoInitializeEx      = ole32.NewProc("CoInitializeEx")
	procCoUninitialize      = ole32.NewProc("CoUninitialize")
	procCoTaskMemFree       = ole32.NewProc("CoTaskMemFree")
)

// browseInfoW mirrors BROWSEINFOW from shlobj.h. The field order and the
// pointer sizes are what SHBrowseForFolderW expects.
type browseInfoW struct {
	owner        uintptr
	root         uintptr
	displayName  *uint16
	title        *uint16
	flags        uint32
	callback     uintptr
	callbackData uintptr
	image        int32
}

// platformPickFolder opens the Windows shell folder dialog. It is the classic
// shell dialog rather than the Explorer picker (IFileOpenDialog): it needs no
// COM interface definitions and no extra module, and it still offers the shell
// places, the "New folder" button and path entry.
func platformPickFolder(parent fyne.Window, title, current string) (string,
	bool, error,
) {
	result, _, _ := procCoInitializeEx.Call(0, coinitApartmentThreaded)
	if result == comSOK || result == comSFalse {
		defer procCoUninitialize.Call()
	}

	titlePtr, err := windows.UTF16PtrFromString(title)
	if err != nil {
		return "", false, err
	}
	root := folderRoot(current)
	if root != 0 {
		defer procCoTaskMemFree.Call(root)
	}
	displayName := make([]uint16, maxPath)
	info := browseInfoW{
		owner:       ownerWindow(parent),
		root:        root,
		displayName: &displayName[0],
		title:       titlePtr,
		flags:       bifReturnOnlyFSDirs | bifUseNewUI,
	}
	pidl, _, _ := procSHBrowseForFolderW.Call(uintptr(unsafe.Pointer(&info)))
	if pidl == 0 {
		// An empty PIDL is the user pressing Cancel.
		return "", false, nil
	}
	defer procCoTaskMemFree.Call(pidl)

	path := make([]uint16, maxLongPath)
	ok, _, callErr := procSHGetPathFromIDList.Call(pidl,
		uintptr(unsafe.Pointer(&path[0])))
	if ok == 0 {
		return "", false, fmt.Errorf("SHGetPathFromIDList: %w", callErr)
	}
	return windows.UTF16ToString(path), true, nil
}

// ownerWindow is the HWND the dialog belongs to, so it stays modal to the
// launcher window.
func ownerWindow(parent fyne.Window) uintptr {
	if parent == nil {
		return 0
	}
	native, ok := parent.(driver.NativeWindow)
	if !ok {
		return 0
	}
	owner := uintptr(0)
	native.RunNative(func(context any) {
		if win, ok := context.(driver.WindowsWindowContext); ok {
			owner = win.HWND
		}
	})
	return owner
}

// folderRoot is the PIDL of the folder the dialog starts in, or 0 for the
// shell default. The caller frees it.
func folderRoot(current string) uintptr {
	start := folderStart(current)
	if start == "" {
		return 0
	}
	name, err := windows.UTF16PtrFromString(start)
	if err != nil {
		return 0
	}
	var pidl uintptr
	result, _, _ := procSHParseDisplayName.Call(uintptr(unsafe.Pointer(name)),
		0, uintptr(unsafe.Pointer(&pidl)), 0, 0)
	if result != comSOK {
		return 0
	}
	return pidl
}
