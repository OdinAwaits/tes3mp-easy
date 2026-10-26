package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"github.com/Saagilat/tes3mp-easy/internal/fonts"
	"github.com/Saagilat/tes3mp-easy/internal/i18n"
	"github.com/Saagilat/tes3mp-easy/internal/ui/style"
)

// fontSetLabelKeys maps catalog identifiers to their translations. A set that
// is not listed here is shown by its identifier instead of being hidden.
var fontSetLabelKeys = map[string]string{
	fonts.SetNoto:  i18n.KeyFontsSetNoto,
	fonts.SetInter: i18n.KeyFontsSetInter,
	fonts.SetPlex:  i18n.KeyFontsSetPlex,
}

// fontSetLabel is the human name of a set in row summaries and dialogs.
func fontSetLabel(tr *i18n.Translator, id string) string {
	if key, ok := fontSetLabelKeys[id]; ok {
		return tr.T(key)
	}
	return fonts.ResolveSet(id).ID
}

// fontSetChooser is the radio list of catalog sets shared by the Installation
// dialog and the wizard step. It keeps the identifiers next to the translated
// labels, so the order always matches fonts.SetIDs().
type fontSetChooser struct {
	ids   []string
	group *widget.RadioGroup
}

// newFontSetChooser builds the list and preselects the current set. onChanged
// may be nil; it receives a catalog identifier, never a translated label.
func newFontSetChooser(tr *i18n.Translator, current string,
	onChanged func(string),
) *fontSetChooser {
	chooser := &fontSetChooser{}
	labels := make([]string, 0, len(fonts.SetIDs()))
	for _, id := range fonts.SetIDs() {
		chooser.ids = append(chooser.ids, id)
		labels = append(labels, fontSetLabel(tr, id))
	}
	chooser.group = widget.NewRadioGroup(labels, nil)
	chooser.selectID(current)
	if onChanged != nil {
		chooser.group.OnChanged = func(label string) {
			if id := chooser.id(label); id != "" {
				onChanged(id)
			}
		}
	}
	return chooser
}

// selected returns the chosen identifier, or an empty string before a choice.
func (c *fontSetChooser) selected() string {
	return c.id(c.group.Selected)
}

// selectID marks a set without running the change callback.
func (c *fontSetChooser) selectID(id string) {
	wanted := fonts.ResolveSet(id).ID
	for index, candidate := range c.ids {
		if candidate == wanted {
			c.group.SetSelected(c.group.Options[index])
			return
		}
	}
}

func (c *fontSetChooser) id(label string) string {
	for index, candidate := range c.group.Options {
		if candidate == label && index < len(c.ids) {
			return c.ids[index]
		}
	}
	return ""
}

// showFontsDialog switches the font set from the Installation checklist. The
// files are downloaded in the background, and the row shows the progress.
func (s *shell) showFontsDialog() {
	tr := s.service.Translator()
	chooser := newFontSetChooser(tr, s.service.FontSet(), nil)

	problem := widget.NewLabel("")
	problem.Wrapping = fyne.TextWrapWord
	problem.Importance = widget.DangerImportance
	problem.Hide()

	content := container.NewVBox(
		style.MutedLabel(tr.T(i18n.KeyFontsSetHint)),
		chooser.group,
		style.MutedLabel(tr.T(i18n.KeyFontsSetLicense)),
		problem,
	)
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
	popup.Resize(fyne.NewSize(560, 380))
	popup.Show()
}
