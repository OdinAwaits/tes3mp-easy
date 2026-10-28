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
	"github.com/Saagilat/tes3mp-easy/internal/ui/style"
)

// fontPreviewSample has Latin, Cyrillic, digits and punctuation, so a set that
// lost a glyph shows it at once instead of at the first server message.
const fontPreviewSample = "Съешь ещё этих мягких булок — ABC 123?"

// fontPreview shows the interface font of the chosen set. It reads the file
// the client already has in Fonts/ and falls back to the launcher font until
// the download finishes: an empty sample would look like a broken preview.
type fontPreview struct {
	text *canvas.Text
	note *widget.Label
	dir  string
	box  *fyne.Container
}

// newFontPreview builds the sample for the font dialog and the wizard step.
// dir is the Fonts folder of the client; an empty dir keeps the theme font.
func newFontPreview(tr *i18n.Translator, dir, setID string) *fontPreview {
	preview := &fontPreview{dir: dir}
	preview.text = canvas.NewText(fontPreviewSample,
		theme.Color(theme.ColorNameForeground))
	preview.text.TextSize = 22
	preview.note = style.MutedLabel(tr.T(i18n.KeyFontsPreviewHint))
	preview.box = container.NewVBox(preview.text, preview.note)
	preview.selectSet(setID)
	return preview
}

// object is the block to place under the picker.
func (p *fontPreview) object() fyne.CanvasObject {
	return p.box
}

// selectSet swaps the sample to another set without touching config.json.
func (p *fontPreview) selectSet(setID string) {
	source := theme.TextFont()
	if data, ok := p.face(setID); ok {
		source = fyne.NewStaticResource(fonts.ResolveSet(setID).Assets[0].Name,
			data)
	}
	p.text.FontSource = source
	p.text.Refresh()
}

// face returns the installed interface font of the set. Only the first asset
// is used: it is the one the game draws the interface with.
func (p *fontPreview) face(setID string) ([]byte, bool) {
	if p.dir == "" {
		return nil, false
	}
	assets := fonts.ResolveSet(setID).Assets
	if len(assets) == 0 {
		return nil, false
	}
	data, err := os.ReadFile(filepath.Join(p.dir, assets[0].Name))
	if err != nil || len(data) == 0 {
		return nil, false
	}
	return data, true
}
