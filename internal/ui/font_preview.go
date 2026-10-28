package ui

import (
	"os"
	"path/filepath"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/Saagilat/tes3mp-easy/internal/fonts"
	"github.com/Saagilat/tes3mp-easy/internal/i18n"
	"github.com/Saagilat/tes3mp-easy/internal/openmwcfg"
	"github.com/Saagilat/tes3mp-easy/internal/ui/style"
)

const (
	// fontPreviewSample has Latin, Cyrillic, digits and punctuation, so a set
	// that lost a glyph shows it at once instead of at the first server
	// message.
	fontPreviewSample = "Съешь ещё этих мягких булок — ABC 123?"
	// fontPreviewConsoleSample looks like the line the MonoFont slot draws, so
	// a proportional file in the console slot is visible at once.
	fontPreviewConsoleSample = `player->AddItem "gold_001" 100`
	// fontPreviewMaxSize keeps three samples inside the column: canvas.Text
	// neither wraps nor truncates a line, it just overflows.
	fontPreviewMaxSize = 32
	// fontPreviewDefaultSize is the sample size on a screen without font size
	// fields (the Fonts dialog): the OpenMW default.
	fontPreviewDefaultSize = openmwcfg.DefaultFontSize
)

// fontPreviewFiles is where the sample reads the TTF files from and how a set
// without files is fetched. An empty dir means the launcher does not know the
// OpenMW folder yet; a nil ensure keeps the download off (tests, no service).
type fontPreviewFiles struct {
	dir      string
	cacheDir string
	// ensure downloads the set into cacheDir off the UI thread. It reports a
	// per-file failure instead of the launcher font, so the sample never lies
	// about the chosen set.
	ensure func(setID string) error
}

// fontPreview shows the three files of the chosen set the way openmw_font.xml
// uses them: interface, Daedric and console. When the caller also edits
// settings.cfg, the samples carry the size the game draws them with.
type fontPreview struct {
	tr    *i18n.Translator
	files fontPreviewFiles
	setID string

	slots    []*fontPreviewSlot
	sizeNote *widget.Label
	retry    *widget.Button
	box      *fyne.Container

	showSize bool
	fontSize int
	factor   float64

	downloading bool
	downloadErr error
	// generation drops the result of a download the user replaced by another
	// set while it was running.
	generation int
	// async runs the download and the UI update that follows it. Production
	// goes through a goroutine and fyne.Do; tests run both inline.
	async func(work func(), done func())
}

// fontPreviewSlot is one line of the sample: the slot name, the text in the
// file of that slot and the state of that file.
type fontPreviewSlot struct {
	caption *widget.Label
	text    *canvas.Text
	status  *widget.Label
}

// newFontPreview builds the sample for the font dialog and the wizard step.
func newFontPreview(tr *i18n.Translator, files fontPreviewFiles,
	setID string,
) *fontPreview {
	preview := &fontPreview{tr: tr, files: files,
		setID: fonts.ResolveSet(setID).ID}
	preview.async = func(work func(), done func()) {
		go func() {
			work()
			fyne.Do(done)
		}()
	}
	preview.fontSize = fontPreviewDefaultSize
	preview.factor = 1

	captions := []string{
		i18n.KeyFontsPreviewSlotUI,
		i18n.KeyFontsPreviewSlotDaedric,
		i18n.KeyFontsPreviewSlotConsole,
	}
	samples := []string{
		fontPreviewSample,
		fontPreviewSample,
		fontPreviewConsoleSample,
	}
	rows := make([]fyne.CanvasObject, 0, len(captions)*3+2)
	for index, key := range captions {
		slot := &fontPreviewSlot{caption: style.MutedLabel(tr.T(key))}
		slot.text = canvas.NewText(samples[index],
			theme.Color(theme.ColorNameForeground))
		slot.text.TextSize = fontPreviewDefaultSize
		slot.text.FontSource = theme.TextFont()
		slot.status = style.MutedLabel("")
		slot.status.Hide()
		preview.slots = append(preview.slots, slot)
		rows = append(rows, slot.caption, slot.text, slot.status)
	}
	preview.sizeNote = style.MutedLabel("")
	preview.sizeNote.Hide()
	preview.retry = widget.NewButton(tr.T(i18n.KeyFontsRetry),
		preview.retryDownload)
	preview.retry.Hide()
	rows = append(rows, style.MutedLabel(tr.T(i18n.KeyFontsPreviewHint)),
		preview.sizeNote, preview.retry)
	preview.box = container.NewVBox(rows...)
	preview.selectSet(preview.setID)
	return preview
}

// object is the block to place under the picker.
func (p *fontPreview) object() fyne.CanvasObject {
	return p.box
}

// selectSet swaps the sample to another set without touching config.json and
// downloads the files when neither Fonts/ nor the cache has them.
func (p *fontPreview) selectSet(setID string) {
	p.setID = fonts.ResolveSet(setID).ID
	p.downloading = false
	p.downloadErr = nil
	p.render()
	p.ensureFiles()
}

// setScaling ties the sample to the font size and scaling factor of
// settings.cfg. A non-positive value is ignored: the form validates the
// entries, and the preview must not show a size the game cannot use.
func (p *fontPreview) setScaling(fontSize int, factor float64) {
	if fontSize <= 0 || factor <= 0 {
		return
	}
	p.showSize = true
	p.fontSize = fontSize
	p.factor = factor
	p.applySize()
	p.renderSize()
}

// effectiveSize is the pixel size OpenMW draws the interface with.
func (p *fontPreview) effectiveSize() int {
	if !p.showSize {
		return fontPreviewDefaultSize
	}
	size := int(float64(p.fontSize)*p.factor + 0.5)
	if size < 1 {
		size = 1
	}
	return size
}

func (p *fontPreview) sampleSize() float32 {
	size := p.effectiveSize()
	if size > fontPreviewMaxSize {
		size = fontPreviewMaxSize
	}
	return float32(size)
}

func (p *fontPreview) applySize() {
	size := p.sampleSize()
	for _, slot := range p.slots {
		slot.text.TextSize = size
		slot.text.Refresh()
	}
}

// render draws every slot from its own file: the three files are the interface,
// the Daedric slot and the console, not three copies of one font.
func (p *fontPreview) render() {
	assets := fonts.ResolveSet(p.setID).Assets
	p.applySize()
	for index, slot := range p.slots {
		slot.text.FontSource = theme.TextFont()
		slot.status.Hide()
		if index >= len(assets) {
			slot.text.Refresh()
			continue
		}
		name := assets[index].Name
		if data, ok := p.face(name); ok {
			slot.text.FontSource = fyne.NewStaticResource(name, data)
		} else {
			p.renderMissing(slot)
		}
		slot.text.Refresh()
	}
	p.renderSize()
	if p.downloadErr != nil {
		p.retry.Show()
	} else {
		p.retry.Hide()
	}
}

// renderMissing explains an absent file instead of quietly swapping the font: a
// download in flight is a state, and a failed one is a state with a reason.
func (p *fontPreview) renderMissing(slot *fontPreviewSlot) {
	switch {
	case p.files.dir == "":
		slot.status.SetText(p.tr.T(i18n.KeyFontsPreviewNoFolder))
	case p.downloading:
		slot.status.SetText(p.tr.T(i18n.KeyFontsPreviewDownloading))
	case p.downloadErr != nil:
		slot.status.SetText(p.tr.Tf(i18n.KeyFontsPreviewFailed,
			p.downloadErr.Error()))
	default:
		slot.status.SetText(p.tr.T(i18n.KeyFontsPreviewMissing))
	}
	slot.status.Show()
}

// retryDownload repeats the fetch of the set after a failure; the row status
// goes back to "Downloading…" on its own.
func (p *fontPreview) retryDownload() {
	p.retry.Hide()
	p.ensureFiles()
}

// renderSize prints the arithmetic of the sample: font size, scaling factor and
// the pixel size that comes out. A size outside the column is named, not hidden.
func (p *fontPreview) renderSize() {
	if !p.showSize {
		p.sizeNote.Hide()
		return
	}
	effective := p.effectiveSize()
	if effective > fontPreviewMaxSize {
		p.sizeNote.SetText(p.tr.Tf(i18n.KeyFontsPreviewSizeClipped,
			p.fontSize, formatScaling(p.factor), effective,
			fontPreviewMaxSize))
	} else {
		p.sizeNote.SetText(p.tr.Tf(i18n.KeyFontsPreviewSize,
			p.fontSize, formatScaling(p.factor), effective))
	}
	p.sizeNote.Show()
}

// ensureFiles downloads a set that is neither installed nor cached. The files
// stay in the client cache: the OpenMW directory is written by the install that
// follows, and it reuses this download.
func (p *fontPreview) ensureFiles() {
	if p.files.ensure == nil || p.files.dir == "" || !p.missingFiles() {
		return
	}
	p.generation++
	generation := p.generation
	setID := p.setID
	p.downloading = true
	p.downloadErr = nil
	p.render()
	var downloadErr error
	p.async(func() {
		downloadErr = p.files.ensure(setID)
	}, func() {
		if generation != p.generation {
			return
		}
		p.downloading = false
		p.downloadErr = downloadErr
		p.render()
	})
}

// missingFiles reports whether any slot has no file in Fonts/ or the cache.
func (p *fontPreview) missingFiles() bool {
	assets := fonts.ResolveSet(p.setID).Assets
	for index := range p.slots {
		if index >= len(assets) {
			return false
		}
		if _, ok := p.face(assets[index].Name); !ok {
			return true
		}
	}
	return false
}

// face reads the TTF file of one slot. The cache is checked second: the preview
// puts a missing set there, and Install copies the verified file from the same
// place, so the sample cannot show a file the install rejects.
func (p *fontPreview) face(name string) ([]byte, bool) {
	for _, dir := range []string{p.files.dir, p.files.cacheDir} {
		if dir == "" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err == nil && len(data) > 0 {
			return data, true
		}
	}
	return nil, false
}
