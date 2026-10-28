package ui

import (
	"context"
	"errors"
	"path/filepath"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"github.com/Saagilat/tes3mp-easy/internal/fonts"
	"github.com/Saagilat/tes3mp-easy/internal/i18n"
	"github.com/Saagilat/tes3mp-easy/internal/ui/style"
)

// fontSetLabel is the human name of a set in row summaries and dialogs. The
// names live in the catalog, so adding a set never needs an i18n key; only the
// "(default)" suffix is translated.
func fontSetLabel(tr *i18n.Translator, id string) string {
	set := fonts.ResolveSet(id)
	label := set.Name
	if label == "" {
		label = set.ID
	}
	if set.ID == fonts.DefaultSet {
		return label + " " + tr.T(i18n.KeyFontsSetDefault)
	}
	return label
}

// fontSetChooser is the dropdown of catalog sets shared by the Installation
// dialog and the wizard step. It keeps the identifiers next to the labels, so
// the order always matches fonts.SetIDs().
type fontSetChooser struct {
	ids    []string
	picker *widget.Select
}

// newFontSetChooser builds the dropdown and preselects the current set.
// onChanged may be nil; it receives a catalog identifier, never a label.
func newFontSetChooser(tr *i18n.Translator, current string,
	onChanged func(string),
) *fontSetChooser {
	chooser := &fontSetChooser{}
	labels := make([]string, 0, len(fonts.SetIDs()))
	for _, id := range fonts.SetIDs() {
		chooser.ids = append(chooser.ids, id)
		labels = append(labels, fontSetLabel(tr, id))
	}
	chooser.picker = widget.NewSelect(labels, nil)
	// The callback is attached after the preselection: opening the dialog must
	// not report a change that the user did not make.
	chooser.selectID(current)
	if onChanged != nil {
		chooser.picker.OnChanged = func(label string) {
			if id := chooser.id(label); id != "" {
				onChanged(id)
			}
		}
	}
	return chooser
}

// selected returns the chosen identifier, or an empty string before a choice.
func (c *fontSetChooser) selected() string {
	return c.id(c.picker.Selected)
}

// selectID marks a set without running the change callback.
func (c *fontSetChooser) selectID(id string) {
	wanted := fonts.ResolveSet(id).ID
	for index, candidate := range c.ids {
		if candidate == wanted {
			c.picker.SetSelectedIndex(index)
			return
		}
	}
}

func (c *fontSetChooser) id(label string) string {
	for index, candidate := range c.picker.Options {
		if candidate == label && index < len(c.ids) {
			return c.ids[index]
		}
	}
	return ""
}

// fontsDirOf is the Fonts folder the preview reads. A machine without a
// configured Morrowind path has no folder yet: the sample says so and stays on
// the launcher font.
func (s *shell) fontsDirOf() string {
	configDir, err := s.service.OpenMWConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(configDir, "Fonts")
}

// fontCacheDirOf is the client cache the preview downloads a missing set into.
func (s *shell) fontCacheDirOf() string {
	dir, err := s.service.FontCacheDir()
	if err != nil {
		return ""
	}
	return dir
}

// fontPreviewFiles is where the sample reads the TTF files from and how it gets
// a set that is neither installed nor cached. The cache is read as well, so the
// install after a preview download needs no network.
func (s *shell) fontPreviewFiles() fontPreviewFiles {
	files := fontPreviewFiles{
		dir:      s.fontsDirOf(),
		cacheDir: s.fontCacheDirOf(),
	}
	files.ensure = func(setID string) error {
		result, err := s.service.EnsureFontsCached(context.Background(), setID,
			nil)
		if err != nil {
			return err
		}
		if len(result.Warnings) > 0 {
			return errors.New(strings.Join(result.Warnings, "; "))
		}
		return nil
	}
	return files
}

// licenseLabel is the one-line note about where the font files come from. It
// wraps instead of being cut by a narrow column.
func licenseLabel(tr *i18n.Translator) *widget.Label {
	label := style.MutedLabel(tr.T(i18n.KeyFontsSetLicense))
	label.Wrapping = fyne.TextWrapWord
	return label
}

// showFontsDialog switches the font set from the Installation checklist. The
// files are downloaded in the background, and the row shows the progress.
func (s *shell) showFontsDialog() {
	tr := s.service.Translator()
	chooser := newFontSetChooser(tr, s.service.FontSet(), nil)
	preview := newFontPreview(tr, s.fontPreviewFiles(), chooser.selected())
	chooser.picker.OnChanged = func(label string) {
		if id := chooser.id(label); id != "" {
			preview.selectSet(id)
		}
	}

	problem := widget.NewLabel("")
	problem.Wrapping = fyne.TextWrapWord
	problem.Importance = widget.DangerImportance
	problem.Hide()

	// Same order as the wizard step: the list on the left, the sample on the
	// right, so the open dropdown never hides what is being compared.
	sets := container.NewVBox(
		style.MutedLabel(tr.T(i18n.KeyFontsSetHint)),
		widget.NewLabelWithStyle(tr.T(i18n.KeyFontsSet),
			fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		chooser.picker,
		licenseLabel(tr),
		problem,
	)
	content := fontsStepLayout(sets,
		container.NewVBox(preview.object()))
	popup := dialog.NewCustomWithoutButtons(tr.T(i18n.KeyFontsSet),
		content, s.window)
	cancel := widget.NewButton(tr.T(i18n.KeyCommonCancel), popup.Hide)
	apply := widget.NewButton(tr.T(i18n.KeyFontsSetApply), func() {
		id := chooser.selected()
		if id == "" {
			problem.SetText(tr.T(i18n.KeyFontsSetRequired))
			problem.Show()
			return
		}
		if err := s.service.SetFontSet(id); err != nil {
			problem.SetText(s.failureBody(err))
			problem.Show()
			return
		}
		popup.Hide()
		s.showScreen(screenInstall)
		s.startFontDownload()
	})
	apply.Importance = widget.HighImportance
	popup.SetButtons([]fyne.CanvasObject{cancel, apply})
	popup.Resize(fyne.NewSize(760, 440))
	popup.Show()
}
