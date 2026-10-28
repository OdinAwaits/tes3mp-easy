//go:build (!linux && !windows) || android

package ui

import "fyne.io/fyne/v2"

// platformPickFolder reports that this build has no native folder dialog, so
// chooseFolder shows the Fyne list.
func platformPickFolder(parent fyne.Window, title, current string) (string,
	bool, error,
) {
	return "", false, errNoNativePicker
}
