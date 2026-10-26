package ui

import (
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"github.com/Saagilat/tes3mp-easy/internal/i18n"
	"github.com/Saagilat/tes3mp-easy/internal/openmwcfg"
	"github.com/Saagilat/tes3mp-easy/internal/ui/style"
)

// uiVideoForm edits the four settings.cfg values the launcher manages. It is
// shared by the Installation row and the wizard so both validate the same way.
type uiVideoForm struct {
	ttf       *widget.Entry
	fontSize  *widget.Entry
	scaling   *widget.Entry
	framerate *widget.Entry
}

func newUIVideoForm(values openmwcfg.Settings) *uiVideoForm {
	form := &uiVideoForm{
		ttf:       widget.NewEntry(),
		fontSize:  widget.NewEntry(),
		scaling:   widget.NewEntry(),
		framerate: widget.NewEntry(),
	}
	form.set(values)
	return form
}

func (f *uiVideoForm) set(values openmwcfg.Settings) {
	f.ttf.SetText(strconv.Itoa(values.TTFResolution))
	f.fontSize.SetText(strconv.Itoa(values.FontSize))
	f.scaling.SetText(strconv.FormatFloat(values.ScalingFactor, 'g', -1, 64))
	f.framerate.SetText(strconv.Itoa(values.FramerateLimit))
}

// values parses the entries and checks the accepted ranges. A parse failure and
// an out-of-range value are reported the same way: the form cannot be saved.
func (f *uiVideoForm) values() (openmwcfg.Settings, error) {
	ttf, err := strconv.Atoi(strings.TrimSpace(f.ttf.Text))
	if err != nil {
		return openmwcfg.Settings{}, err
	}
	fontSize, err := strconv.Atoi(strings.TrimSpace(f.fontSize.Text))
	if err != nil {
		return openmwcfg.Settings{}, err
	}
	scaling, err := strconv.ParseFloat(strings.TrimSpace(f.scaling.Text), 64)
	if err != nil {
		return openmwcfg.Settings{}, err
	}
	framerate, err := strconv.Atoi(strings.TrimSpace(f.framerate.Text))
	if err != nil {
		return openmwcfg.Settings{}, err
	}
	values := openmwcfg.Settings{
		TTFResolution:  ttf,
		FontSize:       fontSize,
		ScalingFactor:  scaling,
		FramerateLimit: framerate,
	}
	if err := values.Validate(); err != nil {
		return openmwcfg.Settings{}, err
	}
	return values, nil
}

// objects renders the labelled fields with the accepted ranges as a hint.
func (f *uiVideoForm) objects(tr *i18n.Translator) []fyne.CanvasObject {
	return []fyne.CanvasObject{
		widget.NewLabel(tr.T(i18n.KeyInstallUIVideoTTF)),
		f.ttf,
		widget.NewLabel(tr.T(i18n.KeyInstallUIVideoFontSize)),
		f.fontSize,
		widget.NewLabel(tr.T(i18n.KeyInstallUIVideoScaling)),
		f.scaling,
		widget.NewLabel(tr.T(i18n.KeyInstallUIVideoFramerate)),
		f.framerate,
		style.MutedLabel(tr.T(i18n.KeyInstallUIVideoFramerateHint)),
		style.MutedLabel(tr.T(i18n.KeyInstallUIVideoRanges)),
	}
}

// uiVideoStateLabel is the one-line verdict shown in the checklist and in the
// wizard step.
func uiVideoStateLabel(tr *i18n.Translator, state openmwcfg.State) string {
	switch {
	case state.Configured:
		return tr.T(i18n.KeyInstallUIVideoReady)
	case !state.Exists:
		return tr.T(i18n.KeyInstallUIVideoNoFile)
	default:
		return tr.T(i18n.KeyInstallUIVideoDefaults)
	}
}

// showUIVideoDialog edits settings.cfg from the Installation checklist.
func (s *shell) showUIVideoDialog() {
	tr := s.service.Translator()
	values := openmwcfg.Defaults()
	if state, err := s.service.ClientSettings(); err == nil {
		values = state.Values
	}
	form := newUIVideoForm(values)

	problem := widget.NewLabel("")
	problem.Wrapping = fyne.TextWrapWord
	problem.Importance = widget.DangerImportance
	problem.Hide()

	content := container.NewVBox(append([]fyne.CanvasObject{
		style.MutedLabel(tr.T(i18n.KeyInstallUIVideoHint)),
	}, append(form.objects(tr), problem)...)...)

	popup := dialog.NewCustomWithoutButtons(tr.T(i18n.KeyInstallUIVideoTitle),
		content, s.window)
	cancel := widget.NewButton(tr.T(i18n.KeyCommonCancel), popup.Hide)
	save := widget.NewButton(tr.T(i18n.KeyCommonSave), func() {
		values, err := form.values()
		if err != nil {
			problem.SetText(tr.T(i18n.KeyInstallUIVideoRanges))
			problem.Show()
			return
		}
		if _, err := s.service.UpdateClientSettings(values); err != nil {
			problem.SetText(s.failureBody(err))
			problem.Show()
			return
		}
		popup.Hide()
		s.showNotice(tr.T(i18n.KeyInstallUIVideoTitle),
			tr.T(i18n.KeyInstallUIVideoSaved), nil)
		s.showScreen(screenInstall)
	})
	save.Importance = widget.HighImportance
	popup.SetButtons([]fyne.CanvasObject{cancel, save})
	popup.Resize(fyne.NewSize(560, 520))
	popup.Show()
}
