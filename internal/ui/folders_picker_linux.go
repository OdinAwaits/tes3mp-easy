//go:build linux && !android

package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver"
	"fyne.io/fyne/v2/storage"

	"github.com/rymdport/portal"
	"github.com/rymdport/portal/filechooser"
)

// platformPickFolder asks xdg-desktop-portal for the system folder dialog: on
// GNOME and KDE it is the dialog every other application shows, with
// bookmarks, recent places, network and cloud folders. An error means the
// portal or the D-Bus session is missing, and chooseFolder falls back to the
// Fyne list.
func platformPickFolder(parent fyne.Window, title, current string) (string,
	bool, error,
) {
	options := &filechooser.OpenFileOptions{
		Directory:     true,
		CurrentFolder: folderStart(current),
	}
	uris, err := filechooser.OpenFile(portalWindowHandle(parent), title, options)
	if err != nil {
		return "", false, err
	}
	if len(uris) == 0 {
		return "", false, nil
	}
	uri, err := storage.ParseURI(uris[0])
	if err != nil {
		return "", false, err
	}
	return uri.Path(), true, nil
}

// portalWindowHandle is the parent window in the form the portal expects. A
// Wayland session has no such handle yet, so the dialog opens unparented
// instead of failing.
func portalWindowHandle(parent fyne.Window) string {
	if parent == nil {
		return ""
	}
	native, ok := parent.(driver.NativeWindow)
	if !ok {
		return ""
	}
	handle := ""
	native.RunNative(func(context any) {
		if x11, ok := context.(driver.X11WindowContext); ok {
			handle = portal.FormatX11WindowHandle(x11.WindowHandle)
		}
	})
	return handle
}
