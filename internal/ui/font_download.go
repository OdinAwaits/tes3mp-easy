package ui

import (
	"context"
	"errors"
	"sync"

	"fyne.io/fyne/v2"

	"github.com/Saagilat/tes3mp-easy/internal/fonts"
	"github.com/Saagilat/tes3mp-easy/internal/i18n"
)

// fontDownload is the background font installation started after the wizard.
// It outlives the Installation screen, so the row polls this state instead of
// owning a dialog.
type fontDownload struct {
	mu          sync.Mutex
	downloading bool
	cancel      context.CancelFunc
	err         error
	warnings    []string
	// canceled marks a user-requested stop so the row can offer Retry instead
	// of silently reporting the set as missing.
	canceled bool
	// progress and shown keep the latest download update and the one last
	// rendered; throttling avoids rebuilding the screen on every HTTP chunk.
	progress fonts.Progress
	shown    fonts.Progress
	// generation prevents a cancelled run from overwriting the next one.
	generation int
}

type fontDownloadSnapshot struct {
	downloading bool
	canceled    bool
	err         error
	warnings    []string
	progress    fonts.Progress
	hasProgress bool
}

func (s *shell) fontState() fontDownloadSnapshot {
	s.fonts.mu.Lock()
	defer s.fonts.mu.Unlock()
	return fontDownloadSnapshot{
		downloading: s.fonts.downloading,
		canceled:    s.fonts.canceled,
		err:         s.fonts.err,
		warnings:    append([]string(nil), s.fonts.warnings...),
		progress:    s.fonts.progress,
		hasProgress: s.fonts.shown.Stage != "",
	}
}

// startFontDownload installs the selected set without blocking the window.
// Starting while another run is active is a no-op; use retry after cancel.
func (s *shell) startFontDownload() {
	s.fonts.mu.Lock()
	if s.fonts.downloading {
		s.fonts.mu.Unlock()
		return
	}
	// Choosing a set in the wizard or the Installation row means the files are
	// wanted again, so a legacy skip_fonts flag must not keep them disabled.
	if settings := s.settings(); settings.SkipFonts {
		settings.SkipFonts = false
		if err := s.service.UpdateSettings(settings); err != nil {
			s.service.LogError("enable fonts", err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	s.fonts.generation++
	generation := s.fonts.generation
	s.fonts.downloading = true
	s.fonts.cancel = cancel
	s.fonts.err = nil
	s.fonts.warnings = nil
	s.fonts.canceled = false
	s.fonts.progress = fonts.Progress{}
	s.fonts.shown = fonts.Progress{}
	s.fonts.mu.Unlock()
	s.refreshFontRows()

	go func() {
		result, err := s.service.InstallFontsWithProgress(ctx,
			func(update fonts.Progress) {
				s.updateFontProgress(generation, update)
			})
		fyne.Do(func() {
			s.fonts.mu.Lock()
			if generation != s.fonts.generation {
				s.fonts.mu.Unlock()
				return
			}
			s.fonts.downloading = false
			s.fonts.cancel = nil
			if errors.Is(err, context.Canceled) {
				err = nil
			}
			s.fonts.err = err
			s.fonts.warnings = append([]string(nil), result.Warnings...)
			s.fonts.mu.Unlock()
			s.refreshFontRows()
		})
	}()
}

// updateFontProgress stores the latest download update and repaints the row at
// most every fontProgressStep, so a fast download does not rebuild the screen
// on every HTTP chunk.
func (s *shell) updateFontProgress(generation int, update fonts.Progress) {
	s.fonts.mu.Lock()
	if generation != s.fonts.generation {
		s.fonts.mu.Unlock()
		return
	}
	s.fonts.progress = update
	refresh := fontProgressRefreshDue(s.fonts.shown, update)
	if refresh {
		s.fonts.shown = update
	}
	s.fonts.mu.Unlock()
	if refresh {
		fyne.Do(s.refreshFontRows)
	}
}

// fontProgressStep is the minimum visible change between row redraws.
const fontProgressStep = 0.05

// fontProgressRefreshDue reports whether an update changes the stage, the file
// or enough of the current file's progress to be worth a redraw.
func fontProgressRefreshDue(shown, update fonts.Progress) bool {
	if shown.Stage != update.Stage || shown.Asset != update.Asset {
		return true
	}
	if update.Stage != fonts.StageDownloading || update.Total <= 0 {
		return false
	}
	delta := fontProgressFraction(update) - fontProgressFraction(shown)
	return delta >= fontProgressStep || delta <= -fontProgressStep
}

// fontProgressFraction is the bar value for one font file. Stages without byte
// counters are complete for the purpose of the bar.
func fontProgressFraction(update fonts.Progress) float64 {
	if update.Stage != fonts.StageDownloading || update.Total <= 0 {
		return 1
	}
	return fraction(update.Downloaded, update.Total)
}

// fontProgressLabel names the file being downloaded or installed.
func fontProgressLabel(tr *i18n.Translator, update fonts.Progress) string {
	switch update.Stage {
	case fonts.StageDownloading:
		if update.Asset == "" {
			return tr.T(i18n.KeyFontsDownloading)
		}
		if update.Total > 0 {
			return tr.Tf(i18n.KeyFontsDownloadingBytes, update.Asset,
				formatBytes(update.Downloaded), formatBytes(update.Total))
		}
		return tr.Tf(i18n.KeyFontsDownloadingAsset, update.Asset)
	case fonts.StageInstalling:
		return tr.Tf(i18n.KeyFontsInstalling, update.Asset)
	default:
		return tr.T(i18n.KeyFontsVerifying)
	}
}

// fraction clamps a byte ratio to the progress-bar range.
func fraction(done, total int64) float64 {
	if total <= 0 {
		return 0
	}
	value := float64(done) / float64(total)
	if value < 0 {
		return 0
	}
	if value > 1 {
		return 1
	}
	return value
}

// cancelFontDownload stops the run and clears its progress without reporting a
// failure: the user asked for it.
func (s *shell) cancelFontDownload() {
	s.fonts.mu.Lock()
	if !s.fonts.downloading {
		s.fonts.mu.Unlock()
		return
	}
	cancel := s.fonts.cancel
	s.fonts.generation++
	s.fonts.downloading = false
	s.fonts.cancel = nil
	s.fonts.err = nil
	s.fonts.warnings = nil
	s.fonts.canceled = true
	s.fonts.progress = fonts.Progress{}
	s.fonts.shown = fonts.Progress{}
	s.fonts.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	s.refreshFontRows()
}

// refreshFontRows redraws only the Installation screen, so a background
// download never pulls the user away from Play or Diagnostics.
func (s *shell) refreshFontRows() {
	if s.screen != screenInstall || s.body == nil || s.window == nil {
		return
	}
	s.showScreen(screenInstall)
}
