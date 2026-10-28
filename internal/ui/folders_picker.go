package ui

import (
	"errors"

	"fyne.io/fyne/v2"
)

// folderPicker is the native folder dialog of the platform. ok is false when
// the user cancelled; err means the desktop cannot show the dialog at all, and
// chooseFolder then falls back to the Fyne list.
type folderPicker func(parent fyne.Window, title, current string) (path string,
	ok bool, err error)

// pickFolder is the platform implementation. It is a variable so the fallback
// branches can be tested without a desktop session.
var pickFolder folderPicker = platformPickFolder

// errNoNativePicker marks a platform that has no native folder dialog in the
// client: chooseFolder shows the Fyne picker instead.
var errNoNativePicker = errors.New("no native folder picker on this platform")
