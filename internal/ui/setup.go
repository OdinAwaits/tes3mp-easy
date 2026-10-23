package ui

import (
	"context"
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"github.com/Saagilat/tes3mp-easy/internal/i18n"
	"github.com/Saagilat/tes3mp-easy/internal/tes3mp"
)

// showSetup is the K02 preparation dialog. The full first-run wizard is K05;
// this dialog exposes the same app scenarios without owning any install logic.
func (s *shell) showSetup() {
	tr := s.service.Translator()
	cfg := s.service.Config()

	path := widget.NewEntry()
	path.SetText(cfg.MorrowindPath)
	path.SetPlaceHolder(".../Morrowind")

	morrowindStatus := widget.NewLabel("")
	morrowindStatus.Wrapping = fyne.TextWrapWord
	tes3mpStatus := widget.NewLabel("")
	tes3mpStatus.Wrapping = fyne.TextWrapWord
	progress := widget.NewProgressBar()
	progress.Min = 0
	progress.Max = 1

	savePath := func(value string) error {
		settings := s.settings()
		settings.MorrowindPath = value
		if err := s.service.UpdateSettings(settings); err != nil {
			morrowindStatus.SetText(tr.Tf(i18n.KeySetupMorrowindIncomplete,
				err.Error()))
			return err
		}
		return nil
	}
	check := func(value string) {
		installation, err := s.service.CheckMorrowind(value)
		if err != nil {
			morrowindStatus.SetText(tr.Tf(i18n.KeySetupMorrowindIncomplete,
				err.Error()))
			return
		}
		path.SetText(installation.Root)
		if savePath(installation.Root) != nil {
			return
		}
		morrowindStatus.SetText(tr.Tf(i18n.KeySetupMorrowindReady,
			installation.Root))
	}
	find := func() {
		installation, err := s.service.DetectMorrowind()
		if err != nil {
			morrowindStatus.SetText(tr.Tf(i18n.KeySetupMorrowindIncomplete,
				err.Error()))
			return
		}
		path.SetText(installation.Root)
		morrowindStatus.SetText(tr.Tf(i18n.KeySetupMorrowindReady,
			installation.Root))
	}
	refreshTES3MP := func() {
		state, err := s.service.TES3MPState()
		switch {
		case err != nil:
			tes3mpStatus.SetText(tr.Tf(i18n.KeySetupFailed, err.Error()))
		case state.Current:
			tes3mpStatus.SetText(tr.Tf(i18n.KeySetupTES3MPReady,
				state.Version))
		case state.Installed:
			tes3mpStatus.SetText(tr.Tf(i18n.KeySetupTES3MPOutdated,
				state.Version, tes3mp.Version))
		default:
			tes3mpStatus.SetText(tr.T(i18n.KeySetupTES3MPMissing))
		}
	}

	findButton := widget.NewButton(tr.T(i18n.KeySetupFind), find)
	chooseButton := widget.NewButton(tr.T(i18n.KeySetupChoose), func() {
		dialog.NewFolderOpen(func(uri fyne.ListableURI, err error) {
			if err != nil || uri == nil {
				return
			}
			check(uri.Path())
		}, s.window).Show()
	})
	checkButton := widget.NewButton(tr.T(i18n.KeySetupCheck), func() {
		check(path.Text)
	})

	installButton := widget.NewButton(tr.T(i18n.KeySetupInstall), nil)
	installButton.Importance = widget.HighImportance
	installButton.OnTapped = func() {
		installButton.Disable()
		progress.SetValue(0)
		tes3mpStatus.SetText(tr.T(i18n.KeySetupVerifying))
		ctx := context.Background()
		go func() {
			_, err := s.service.InstallTES3MP(ctx, func(update tes3mp.Progress) {
				fyne.Do(func() {
					progress.SetValue(progressFraction(update))
					tes3mpStatus.SetText(progressLabel(tr, update))
				})
			})
			fyne.Do(func() {
				installButton.Enable()
				if err != nil {
					tes3mpStatus.SetText(tr.Tf(i18n.KeySetupFailed, err.Error()))
					return
				}
				progress.SetValue(1)
				tes3mpStatus.SetText(tr.T(i18n.KeySetupDone))
			})
		}()
	}

	content := container.NewVBox(
		widget.NewLabelWithStyle(tr.T(i18n.KeySetupMorrowind),
			fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		path,
		container.NewHBox(findButton, chooseButton, checkButton),
		morrowindStatus,
		widget.NewSeparator(),
		widget.NewLabelWithStyle(tr.Tf(i18n.KeySetupTES3MP, tes3mp.Version),
			fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		tes3mpStatus,
		container.NewBorder(nil, nil, nil, installButton, progress),
	)
	dialog.NewCustom(tr.T(i18n.KeySetupTitle), tr.T(i18n.KeyCommonClose),
		content, s.window).Show()

	if cfg.MorrowindPath == "" {
		find()
	} else {
		check(cfg.MorrowindPath)
	}
	refreshTES3MP()
}

func progressFraction(update tes3mp.Progress) float64 {
	if update.Stage != tes3mp.StageDownloading || update.Total <= 0 {
		return 1
	}
	value := float64(update.Downloaded) / float64(update.Total)
	if value < 0 {
		return 0
	}
	if value > 1 {
		return 1
	}
	return value
}

func progressLabel(tr *i18n.Translator, update tes3mp.Progress) string {
	switch update.Stage {
	case tes3mp.StageDownloading:
		if update.Total > 0 {
			return tr.Tf(i18n.KeySetupDownloading,
				formatBytes(update.Downloaded), formatBytes(update.Total))
		}
		return tr.Tf(i18n.KeySetupDownloading, formatBytes(update.Downloaded),
			"?")
	case tes3mp.StageExtracting:
		return tr.T(i18n.KeySetupExtracting)
	case tes3mp.StageInstalling:
		return tr.T(i18n.KeySetupInstalling)
	default:
		return tr.T(i18n.KeySetupVerifying)
	}
}

func formatBytes(value int64) string {
	const (
		kib = 1024
		mib = 1024 * kib
		gib = 1024 * mib
	)
	switch {
	case value >= gib:
		return fmt.Sprintf("%.1f GiB", float64(value)/gib)
	case value >= mib:
		return fmt.Sprintf("%.1f MiB", float64(value)/mib)
	case value >= kib:
		return fmt.Sprintf("%.1f KiB", float64(value)/kib)
	default:
		return fmt.Sprintf("%d B", value)
	}
}
