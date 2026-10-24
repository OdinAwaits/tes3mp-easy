package ui

import (
	"context"
	"fmt"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"

	"github.com/Saagilat/tes3mp-easy/internal/app"
	"github.com/Saagilat/tes3mp-easy/internal/config"
	"github.com/Saagilat/tes3mp-easy/internal/i18n"
	"github.com/Saagilat/tes3mp-easy/internal/serverapi"
)

type serverStatus struct {
	Checked     bool
	Checking    bool
	Online      bool
	NeedsUpdate bool
	Info        serverapi.Server
	Err         error
}

func (s *shell) statusFor(id string) serverStatus {
	s.statusMu.Lock()
	defer s.statusMu.Unlock()
	return s.statuses[id]
}

func (s *shell) statusText(id string) string {
	tr := s.service.Translator()
	status := s.statusFor(id)
	switch {
	case status.Err != nil:
		failure := app.Explain(status.Err)
		return fmt.Sprintf("%s · %s", failure.Code, s.failureSummary(status.Err))
	case status.Online:
		state := strings.TrimSpace(status.Info.Status)
		if state == "" {
			state = "online"
		}
		text := tr.Tf(i18n.KeyServerStatusOnline, state)
		if status.NeedsUpdate {
			text += " · " + tr.T(i18n.KeyServerStatusUpdate)
		}
		return text
	default:
		return tr.T(i18n.KeyServerStatusChecking)
	}
}

func (s *shell) statusImportance(id string) widget.Importance {
	status := s.statusFor(id)
	switch {
	case status.Err != nil:
		return widget.WarningImportance
	case status.Online:
		return widget.SuccessImportance
	default:
		return widget.LowImportance
	}
}

// checkServer runs the panel status check in the background and updates the
// inline label. Background checks never open a modal dialog.
func (s *shell) checkServer(profile config.Server, label *widget.Label,
	play *widget.Button, modsInstalled bool,
) {
	s.statusMu.Lock()
	if s.checking[profile.ID] || s.statuses[profile.ID].Checked {
		s.statusMu.Unlock()
		return
	}
	s.checking[profile.ID] = true
	s.statusMu.Unlock()

	go func() {
		info, err := s.service.CheckServer(context.Background(), profile.ID)
		if err != nil {
			s.service.LogError("check server", err)
		}
		status := serverStatus{Checked: true, Err: err}
		if err == nil {
			status.Online = true
			status.Info = info
			if state, stateErr := s.service.ModsState(profile.ID); stateErr == nil &&
				state.Installed && info.ModsSHA256 != nil &&
				!strings.EqualFold(state.SHA256, *info.ModsSHA256) {
				status.NeedsUpdate = true
			}
		}
		s.statusMu.Lock()
		s.statuses[profile.ID] = status
		s.checking[profile.ID] = false
		s.statusMu.Unlock()

		fyne.Do(func() {
			if label != nil {
				label.SetText(s.statusText(profile.ID))
				label.Importance = s.statusImportance(profile.ID)
				label.Refresh()
			}
			if play != nil {
				tr := s.service.Translator()
				play.SetText(playButtonLabel(tr, modsInstalled, status.NeedsUpdate))
			}
		})
	}()
}
