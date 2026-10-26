package ui

import (
	"context"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"github.com/Saagilat/tes3mp-easy/internal/i18n"
	"github.com/Saagilat/tes3mp-easy/internal/locale"
	"github.com/Saagilat/tes3mp-easy/internal/ui/style"
)

// languageDialog installs one optional game language pack for every server
// profile. It is an explicit action, not a wizard step: without a pack the game
// keeps its original language, which is a normal setup and never an error.
type languageDialog struct {
	shell *shell
	state locale.State
	pack  locale.Pack

	popup    *dialog.CustomDialog
	status   *widget.Label
	detail   *widget.Label
	problem  *widget.Label
	progress *widget.ProgressBar

	packChoice  *widget.RadioGroup
	variantBox  *fyne.Container
	variants    *widget.RadioGroup
	variantHint *widget.Label

	install *widget.Button
	remove  *widget.Button
	cancel  *widget.Button

	busy    bool
	changed bool
}

// showLanguageDialog opens the single-step language pack dialog from the
// Installation row and from the ⋯ menu.
func (s *shell) showLanguageDialog() {
	tr := s.service.Translator()
	state, err := s.service.LocalizationState()
	if err != nil {
		s.showError(err)
		return
	}
	packs := locale.Packages()
	if len(packs) == 0 {
		s.showNotice(tr.T(i18n.KeyLocaleTitle),
			tr.T(i18n.KeyInstallLanguageOriginal), nil)
		return
	}
	d := &languageDialog{shell: s, state: state, pack: packs[0]}
	if state.Installed {
		if pack, ok := locale.PackFor(state.Language); ok {
			d.pack = pack
		}
	}

	d.status = widget.NewLabel(localizationStatusText(tr, state))
	d.status.Wrapping = fyne.TextWrapWord
	d.detail = widget.NewLabelWithStyle("", fyne.TextAlignLeading,
		fyne.TextStyle{Monospace: true})
	d.detail.Wrapping = fyne.TextWrapWord
	d.detail.Importance = widget.LowImportance
	d.problem = widget.NewLabel("")
	d.problem.Wrapping = fyne.TextWrapWord
	d.problem.Importance = widget.DangerImportance
	d.problem.Hide()
	d.progress = widget.NewProgressBar()
	d.progress.Min = 0
	d.progress.Max = 1
	d.progress.Hide()
	d.variantBox = container.NewVBox()
	d.variantHint = style.MutedLabel("")

	objects := []fyne.CanvasObject{style.MutedLabel(tr.T(i18n.KeyLocaleQuestion))}
	if len(packs) > 1 {
		d.packChoice = widget.NewRadioGroup(localizationLabels(tr), nil)
		d.packChoice.Horizontal = false
		d.packChoice.SetSelected(localizationLabel(tr, d.pack.Language))
		d.packChoice.OnChanged = func(string) {
			if pack, ok := localizationPack(tr, d.packChoice.Selected); ok {
				d.pack = pack
			}
			d.renderVariants(tr)
			d.updateButtons()
		}
		objects = append(objects, d.packChoice)
	} else {
		objects = append(objects, container.NewHBox(
			widget.NewLabelWithStyle(localizationLabel(tr, d.pack.Language),
				fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			style.MutedLabel(tr.T(i18n.KeyLocaleOnlyPack)),
		))
	}
	d.renderVariants(tr)
	d.refreshDetail()
	objects = append(objects, d.variantBox, d.variantHint, d.status,
		d.detail, d.problem, d.progress)

	popup := dialog.NewCustomWithoutButtons(tr.T(i18n.KeyLocaleTitle),
		container.NewVScroll(container.NewVBox(objects...)), s.window)
	d.popup = popup
	if len(packs) > 1 {
		popup.Resize(fyne.NewSize(640, 520))
	} else {
		popup.Resize(fyne.NewSize(620, 440))
	}
	d.install = widget.NewButton(tr.T(i18n.KeyInstallInstall), d.installPack)
	d.install.Importance = widget.HighImportance
	d.remove = widget.NewButton(tr.T(i18n.KeyLanguageWizardRemove),
		d.removePack)
	d.remove.Importance = widget.DangerImportance
	cancel := widget.NewButton(tr.T(i18n.KeyCommonCancel), d.close)
	d.cancel = cancel
	popup.SetButtons([]fyne.CanvasObject{cancel, d.remove, d.install})
	d.updateButtons()
	popup.Show()
}

// renderVariants rebuilds the text/voice radio list for the chosen pack. The
// voiceover option only exists when the pack ships a separate voice archive.
func (d *languageDialog) renderVariants(tr *i18n.Translator) {
	options := []string{tr.T(i18n.KeyLocaleVariantText)}
	if d.pack.HasVoices {
		options = append(options, tr.T(i18n.KeyLocaleVariantVoices))
	}
	selected := options[0]
	if len(options) > 1 && d.state.Installed &&
		d.state.Language == d.pack.Language && d.state.Voiceover {
		selected = options[1]
	}
	d.variants = widget.NewRadioGroup(options, nil)
	d.variants.Horizontal = false
	d.variants.SetSelected(selected)
	d.variants.OnChanged = func(string) {
		d.syncVariantHint(tr)
		d.updateButtons()
	}
	objects := []fyne.CanvasObject{d.variants}
	if !d.pack.HasVoices {
		objects = append(objects, style.MutedLabel(tr.T(i18n.KeyLocaleNoVoices)))
	}
	d.variantBox.Objects = objects
	d.variantBox.Refresh()
	d.syncVariantHint(tr)
}

// voices reports the selected variant. A pack without voices is always
// installed as text only.
func (d *languageDialog) voices() bool {
	if d.variants == nil || !d.pack.HasVoices {
		return false
	}
	return d.variants.Selected ==
		d.shell.service.Translator().T(i18n.KeyLocaleVariantVoices)
}

func (d *languageDialog) syncVariantHint(tr *i18n.Translator) {
	if d.variantHint == nil {
		return
	}
	d.variantHint.SetText(localeVariantHint(tr, d.pack, d.voices()))
}

func (d *languageDialog) updateButtons() {
	if d.install == nil {
		return
	}
	needed := localizationNeedsInstall(d.state, d.pack.Language, d.voices())
	if d.busy || !needed {
		d.install.Disable()
	} else {
		d.install.Enable()
	}
	if d.state.Installed {
		d.remove.Show()
	} else {
		d.remove.Hide()
	}
	if d.busy {
		d.remove.Disable()
		d.cancel.Disable()
	} else {
		d.remove.Enable()
		d.cancel.Enable()
	}
	if d.variants != nil {
		if d.busy {
			d.variants.Disable()
		} else {
			d.variants.Enable()
		}
	}
	if d.packChoice != nil {
		if d.busy {
			d.packChoice.Disable()
		} else {
			d.packChoice.Enable()
		}
	}
}

func (d *languageDialog) installPack() {
	if d.busy {
		return
	}
	tr := d.shell.service.Translator()
	chosen, voices := d.pack.Language, d.voices()
	if !localizationNeedsInstall(d.state, chosen, voices) {
		return
	}
	if err := d.shell.service.SetVoiceover(voices); err != nil {
		d.setProblem(err)
		return
	}
	d.busy = true
	d.clearProblem()
	d.progress.Show()
	d.progress.SetValue(0)
	d.status.SetText(tr.T(i18n.KeyLanguageWizardApplying))
	d.updateButtons()

	go func() {
		result, err := d.shell.service.InstallLocalization(
			context.Background(), chosen, voices, func(update locale.Progress) {
				fyne.Do(func() {
					d.progress.SetValue(localeProgressFraction(update))
					d.status.SetText(localeProgressLabel(tr, update))
				})
			})
		fyne.Do(func() {
			d.finishInstall(result, err)
		})
	}()
}

func (d *languageDialog) finishInstall(result locale.Result, err error) {
	d.busy = false
	if err != nil {
		d.setProblem(err)
		return
	}
	d.state = result.State
	d.changed = true
	d.clearProblem()
	d.progress.SetValue(1)
	text := d.shell.service.Translator().T(i18n.KeyLanguageWizardDone)
	if len(result.Warnings) > 0 {
		text += "\n\n" + strings.Join(result.Warnings, "\n")
	}
	d.status.SetText(text)
	d.refreshDetail()
	d.renderVariants(d.shell.service.Translator())
	d.updateButtons()
}

func (d *languageDialog) removePack() {
	if d.busy {
		return
	}
	d.popup.Hide()
	d.changed = false
	d.shell.removeLocalization()
}

func (d *languageDialog) close() {
	if d.busy {
		return
	}
	d.popup.Hide()
	if d.changed {
		d.changed = false
		d.shell.showScreen(d.shell.screen)
	}
}

func (d *languageDialog) refreshDetail() {
	if d.detail == nil {
		return
	}
	if !d.state.Installed {
		d.detail.SetText("")
		d.detail.Hide()
		return
	}
	d.detail.SetText(d.state.DataFiles)
	d.detail.Show()
}

func (d *languageDialog) setProblem(err error) {
	if err == nil {
		return
	}
	d.problem.SetText(d.shell.failureBody(err))
	d.problem.Show()
	d.updateButtons()
}

func (d *languageDialog) clearProblem() {
	if d.problem != nil {
		d.problem.Hide()
	}
}

// localizationStatusText is the dialog status line. A missing pack is a normal
// setup, so it stays neutral and never reads like a failure.
func localizationStatusText(tr *i18n.Translator, state locale.State) string {
	if !state.Installed {
		return tr.T(i18n.KeyInstallLanguageOriginal)
	}
	return tr.Tf(i18n.KeyLocaleInstalled, localizationInstallLabel(tr, state))
}

// localeVariantHint describes the selected variant with its download size.
func localeVariantHint(tr *i18n.Translator, pack locale.Pack, voices bool) string {
	if voices && pack.HasVoices {
		return tr.Tf(i18n.KeyLocaleVariantVoicesHint,
			formatBytes(pack.DownloadSize(true)))
	}
	encoding := pack.Encoding
	if encoding == "" {
		encoding = "—"
	}
	return tr.Tf(i18n.KeyLocaleVariantTextHint,
		formatBytes(pack.DownloadSize(false)), encoding)
}

func localizationLabels(tr *i18n.Translator) []string {
	packs := locale.Packages()
	labels := make([]string, 0, len(packs))
	for _, pack := range packs {
		labels = append(labels, localizationLabel(tr, pack.Language))
	}
	return labels
}

func localizationPack(tr *i18n.Translator, label string) (locale.Pack, bool) {
	for _, pack := range locale.Packages() {
		if label == localizationLabel(tr, pack.Language) {
			return pack, true
		}
	}
	return locale.Pack{}, false
}

// localizationNeedsInstall reports whether the requested package or voiceover
// choice differs from the installed overlay.
func localizationNeedsInstall(state locale.State, language string,
	voices bool,
) bool {
	if !state.Installed || !strings.EqualFold(state.Language, language) {
		return true
	}
	return state.Voiceover != voices
}

// localizationInstallLabel names an installed overlay the way the Installation
// row shows it: "Russian · text + voice".
func localizationInstallLabel(tr *i18n.Translator, state locale.State) string {
	label := localizationLabel(tr, state.Language)
	if state.Voiceover {
		return tr.Tf(i18n.KeyInstallLanguageVoices, label)
	}
	return tr.Tf(i18n.KeyInstallLanguageText, label)
}
