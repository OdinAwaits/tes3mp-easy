package ui

import (
	"fmt"
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

// uiVideoField is one labelled input. The label carries the accepted range of
// the openmwcfg constants, so the form and the validator cannot drift apart.
type uiVideoField struct {
	labelKey  string
	hintKey   string
	rangeText string
	entry     *widget.Entry
}

func intRange(minimum, maximum int) string {
	return fmt.Sprintf("%d–%d", minimum, maximum)
}

func floatRange(minimum, maximum float64) string {
	return fmt.Sprintf("%g–%g", minimum, maximum)
}

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
	f.scaling.SetText(formatScaling(values.ScalingFactor))
	f.framerate.SetText(strconv.Itoa(values.FramerateLimit))
}

// formatScaling keeps the value the user typed but shows whole factors as
// "1.00", matching the hint text. Fractional factors are never rounded.
func formatScaling(value float64) string {
	text := strconv.FormatFloat(value, 'f', -1, 64)
	if !strings.Contains(text, ".") {
		text += ".00"
	}
	return text
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

// fields lists the four managed values in display order.
func (f *uiVideoForm) fields() []uiVideoField {
	return []uiVideoField{
		{
			labelKey: i18n.KeyInstallUIVideoTTF,
			hintKey:  i18n.KeyInstallUIVideoTTFHint,
			rangeText: intRange(openmwcfg.MinTTFResolution,
				openmwcfg.MaxTTFResolution),
			entry: f.ttf,
		},
		{
			labelKey: i18n.KeyInstallUIVideoFontSize,
			hintKey:  i18n.KeyInstallUIVideoFontSizeHint,
			rangeText: intRange(openmwcfg.MinFontSize,
				openmwcfg.MaxFontSize),
			entry: f.fontSize,
		},
		{
			labelKey: i18n.KeyInstallUIVideoScaling,
			hintKey:  i18n.KeyInstallUIVideoScalingHint,
			rangeText: floatRange(openmwcfg.MinScalingFactor,
				openmwcfg.MaxScalingFactor),
			entry: f.scaling,
		},
		{
			labelKey: i18n.KeyInstallUIVideoFramerate,
			hintKey:  i18n.KeyInstallUIVideoFramerateHint,
			rangeText: intRange(openmwcfg.MinFramerateLimit,
				openmwcfg.MaxFramerateLimit),
			entry: f.framerate,
		},
	}
}

// fieldLabel is a bold name with its accepted range next to it.
func uiVideoFieldLabel(tr *i18n.Translator, field uiVideoField) fyne.CanvasObject {
	name := widget.NewLabelWithStyle(tr.T(field.labelKey),
		fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	return container.NewHBox(name, style.MutedLabel(field.rangeText))
}

// objects renders every field with its label, range and hint.
func (f *uiVideoForm) objects(tr *i18n.Translator) []fyne.CanvasObject {
	fields := f.fields()
	objects := make([]fyne.CanvasObject, 0, len(fields)*3)
	for _, field := range fields {
		objects = append(objects,
			uiVideoFieldLabel(tr, field),
			field.entry,
			style.MutedLabel(tr.T(field.hintKey)),
		)
	}
	return objects
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

	content := container.NewVScroll(container.NewVBox(append([]fyne.CanvasObject{
		style.MutedLabel(tr.T(i18n.KeyInstallUIVideoHint)),
	}, append(form.objects(tr), problem)...)...))

	popup := dialog.NewCustomWithoutButtons(tr.T(i18n.KeyInstallUIVideoTitle),
		content, s.window)
	cancel := widget.NewButton(tr.T(i18n.KeyCommonCancel), popup.Hide)
	save := widget.NewButton(tr.T(i18n.KeyCommonSave), func() {
		values, err := form.values()
		if err != nil {
			problem.SetText(tr.T(i18n.KeyInstallUIVideoInvalid))
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
	popup.Resize(fyne.NewSize(640, 660))
	popup.Show()
}
