package app

import (
	"strings"

	"github.com/Saagilat/tes3mp-easy/internal/launcher"
)

// ProtonSource tells where the folder reported by ProtonStatus came from, so the
// Installation screen can label an automatic discovery as such.
type ProtonSource string

const (
	// ProtonFromConfig means the user picked the folder in Installation.
	ProtonFromConfig ProtonSource = "config"
	// ProtonDiscovered means the folder was found in a Steam library.
	ProtonDiscovered ProtonSource = "discovered"
	// ProtonMissing means no usable Proton is available.
	ProtonMissing ProtonSource = "missing"
)

// ProtonStatus is the Proton folder the launcher would use right now: the
// configured one wins, otherwise Steam libraries are searched. Err is set when
// the folder is unusable, so the UI can show a hint instead of a path.
type ProtonStatus struct {
	Path   string
	Source ProtonSource
	Err    error
}

// ProtonStatus resolves the effective Proton folder. It runs only when the
// caller needs it (Installation → Advanced), so the client does not scan Steam
// libraries on start.
func (s *Service) ProtonStatus() ProtonStatus {
	if s == nil || s.goos != "linux" {
		return ProtonStatus{Source: ProtonMissing}
	}
	if raw := strings.TrimSpace(s.Config().ProtonPath); raw != "" {
		path, err := launcher.ProtonFolder(raw, s.homeDir)
		status := ProtonStatus{Path: raw, Source: ProtonFromConfig, Err: err}
		if err == nil {
			status.Path = path
		}
		return status
	}
	path, err := launcher.FindProton(s.homeDir)
	if err != nil {
		return ProtonStatus{Source: ProtonMissing, Err: err}
	}
	return ProtonStatus{Path: path, Source: ProtonDiscovered}
}
