package app

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Saagilat/tes3mp-easy/internal/launcher"
	"github.com/Saagilat/tes3mp-easy/internal/tes3mp"
)

// EditorState describes the OpenMW-CS executable shipped next to tes3mp.exe.
// OpenMW-CS is not bundled with TES3MP, so a missing editor is a normal state
// the UI explains instead of an error.
type EditorState struct {
	// Path is the expected executable location inside the work directory.
	Path string
	// Found reports whether the executable is present.
	Found bool
}

// EditorState inspects the TES3MP directory for openmw-cs.exe. The work
// directory must be configured; an unavailable directory is reported as an
// environment failure like every other installation check.
func (s *Service) EditorState() (EditorState, error) {
	if s == nil {
		return EditorState{}, errors.New("app service is nil")
	}
	workDir, err := s.absoluteWorkDir()
	if err != nil {
		return EditorState{}, err
	}
	path := filepath.Join(tes3mp.Dir(workDir), launcher.OpenMWCSExecutable)
	info, err := os.Stat(path)
	switch {
	case err == nil && !info.IsDir():
		return EditorState{Path: path, Found: true}, nil
	case err == nil, errors.Is(err, os.ErrNotExist):
		return EditorState{Path: path}, nil
	default:
		return EditorState{Path: path}, fmt.Errorf("inspect OpenMW-CS: %w", err)
	}
}
