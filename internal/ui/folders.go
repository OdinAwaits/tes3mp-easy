package ui

import (
	"os"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/widget"

	"github.com/Saagilat/tes3mp-easy/internal/i18n"
)

// chooseFolder opens the native folder picker and stores the selection through
// apply. current only positions the dialog: an empty or missing folder falls
// back to the platform default.
func (s *shell) chooseFolder(title, current string, apply func(string)) {
	picker := dialog.NewFolderOpen(func(uri fyne.ListableURI, err error) {
		if err != nil {
			s.showError(err)
			return
		}
		if uri == nil {
			return
		}
		apply(uri.Path())
	}, s.window)
	if start := folderURI(current); start != nil {
		picker.SetLocation(start)
	}
	picker.Show()
}

// folderURI is the start location of the picker, or nil when current is empty
// or is not a directory.
func folderURI(current string) fyne.ListableURI {
	current = strings.TrimSpace(current)
	if current == "" {
		return nil
	}
	info, err := os.Stat(current)
	if err != nil || !info.IsDir() {
		return nil
	}
	listable, err := storage.ListerForURI(storage.NewFileURI(current))
	if err != nil {
		return nil
	}
	return listable
}

// pathButtons are the two ways a folder is edited: the native picker first and
// typed entry second, because the picker cannot open on some desktops (Wayland
// without xdg-desktop-portal, remote X11). Both actions share the caller's
// setter, so typing a path behaves exactly like picking one.
func (s *shell) pathButtons(tr *i18n.Translator, title, primary, current string,
	apply func(string),
) fyne.CanvasObject {
	picker := widget.NewButton(primary, func() {
		s.chooseFolder(title, current, apply)
	})
	manual := widget.NewButton(tr.T(i18n.KeyCommonEnterPath), func() {
		s.enterPath(title, current, apply)
	})
	manual.Importance = widget.LowImportance
	return container.NewHBox(picker, manual)
}

// enterPath is the keyboard fallback of chooseFolder: it asks for a path and
// stores it through the supplied setter. An empty value means "use the default".
func (s *shell) enterPath(title, current string, apply func(string)) {
	tr := s.service.Translator()
	entry := widget.NewEntry()
	entry.SetText(current)
	content := container.NewVBox(widget.NewLabel(title), entry)
	popup := dialog.NewCustomConfirm(title, tr.T(i18n.KeyCommonSave),
		tr.T(i18n.KeyCommonCancel), content, func(ok bool) {
			if !ok {
				return
			}
			apply(strings.TrimSpace(entry.Text))
		}, s.window)
	popup.Resize(fyne.NewSize(560, 200))
	popup.Show()
}
