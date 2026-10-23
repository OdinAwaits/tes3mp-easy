package ui

import (
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/Saagilat/tes3mp-easy/internal/app"
	"github.com/Saagilat/tes3mp-easy/internal/i18n"
)

// openFolderCommand returns the platform command used to show the log folder.
// It is separate from execution so command construction stays testable.
func openFolderCommand(goos, folder string) (string, []string) {
	switch goos {
	case "windows":
		return "explorer.exe", []string{folder}
	case "darwin":
		return "open", []string{folder}
	default:
		return "xdg-open", []string{folder}
	}
}

func (s *shell) openLogFolder() {
	path := s.service.LogPath()
	if strings.TrimSpace(path) == "" {
		return
	}
	folder := filepath.Dir(path)
	name, args := openFolderCommand(runtime.GOOS, folder)
	command := exec.Command(name, args...)
	if err := command.Start(); err != nil {
		s.showErrorWithRetry(err, s.openLogFolder)
		return
	}
	go func() {
		_ = command.Wait()
	}()
	s.service.LogInfof("opened log folder %s", folder)
}

// showLogs displays the recent client log and offers refresh/open-folder
// actions. The log is local and bounded, so reading it on the UI thread is
// inexpensive.
func (s *shell) showLogs() {
	tr := s.service.Translator()
	path := widget.NewLabelWithStyle(s.service.LogPath(),
		fyne.TextAlignLeading, fyne.TextStyle{Monospace: true})
	path.Wrapping = fyne.TextWrapWord

	logText := widget.NewLabelWithStyle("", fyne.TextAlignLeading,
		fyne.TextStyle{Monospace: true})
	logText.Wrapping = fyne.TextWrapWord
	scroll := container.NewVScroll(logText)
	scroll.SetMinSize(fyne.NewSize(780, 360))

	load := func() {
		lines, err := s.service.ReadLogTail(app.TailLines)
		if err != nil {
			logText.SetText(s.failureBody(err))
			return
		}
		if len(lines) == 0 {
			logText.SetText(tr.T(i18n.KeyLogsEmpty))
			return
		}
		logText.SetText(strings.Join(lines, "\n"))
	}
	refresh := widget.NewButtonWithIcon(tr.T(i18n.KeyLogsRefresh),
		theme.ViewRefreshIcon(), load)
	open := widget.NewButtonWithIcon(tr.T(i18n.KeyLogsOpenFolder),
		theme.FolderOpenIcon(), s.openLogFolder)
	hint := widget.NewLabel(tr.Tf(i18n.KeyLogsHint, app.TailLines))
	hint.Wrapping = fyne.TextWrapWord

	content := container.NewBorder(
		container.NewVBox(path),
		container.NewVBox(hint, container.NewHBox(open, layout.NewSpacer(), refresh)),
		nil, nil, scroll,
	)
	popup := dialog.NewCustom(tr.T(i18n.KeyLogsTitle), tr.T(i18n.KeyCommonClose),
		content, s.window)
	popup.Resize(fyne.NewSize(860, 560))
	load()
	popup.Show()
}
