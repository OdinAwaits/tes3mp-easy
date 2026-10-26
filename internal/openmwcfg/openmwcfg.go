// Package openmwcfg edits the values the launcher owns in OpenMW's
// settings.cfg.
//
// The file belongs to OpenMW: the first game launch creates it and it holds
// dozens of keys the launcher does not understand. The editor therefore keeps
// every line it was not asked to change, preserves the section order, comments
// and the original line ending, and only touches four keys. A write is atomic
// and leaves a .bak-<timestamp> copy of the previous file, so a bad edit can be
// reverted by hand.
package openmwcfg

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/Saagilat/tes3mp-easy/internal/atomicfile"
)

// FileName is the file inside the OpenMW configuration directory.
const FileName = "settings.cfg"

const (
	// SectionGUI holds the interface values.
	SectionGUI = "GUI"
	// SectionVideo holds the video values.
	SectionVideo = "Video"

	keyTTFResolution  = "ttf resolution"
	keyFontSize       = "font size"
	keyScalingFactor  = "scaling factor"
	keyFramerateLimit = "framerate limit"

	maxFileSize = 1 << 20
	newFileMode = 0o644
)

// Accepted ranges. They cover what OpenMW itself accepts and reject typos such
// as a font size of 160.
const (
	MinTTFResolution  = 16
	MaxTTFResolution  = 48
	MinFontSize       = 8
	MaxFontSize       = 32
	MinScalingFactor  = 0.5
	MaxScalingFactor  = 4
	MinFramerateLimit = 0
	MaxFramerateLimit = 360
)

// Defaults of the individual keys, used for a partial file.
const (
	DefaultTTFResolution  = 18
	DefaultFontSize       = 16
	DefaultFramerateLimit = 60
)

// ErrInvalid marks a settings.cfg that cannot be read or a rejected value.
var ErrInvalid = errors.New("invalid settings.cfg")

// InvalidError carries the key that caused the rejection. Key is empty for a
// whole-file problem such as a symlink or a malformed value.
type InvalidError struct {
	Key     string
	Message string
}

// Error implements error.
func (e *InvalidError) Error() string {
	if e.Key != "" {
		return fmt.Sprintf("settings.cfg: %s: %s", e.Key, e.Message)
	}
	return "settings.cfg: " + e.Message
}

// Unwrap allows errors.Is(err, ErrInvalid).
func (e *InvalidError) Unwrap() error { return ErrInvalid }

func invalidf(key, format string, args ...any) error {
	return &InvalidError{Key: key, Message: fmt.Sprintf(format, args...)}
}

// State describes the current file and the effective values.
type State struct {
	// Path is the absolute settings.cfg path.
	Path string
	// Exists reports whether the file is present. OpenMW creates it on the
	// first launch.
	Exists bool
	// Configured reports whether all four managed keys are present.
	Configured bool
	// Values are the managed values; missing keys fall back to Defaults.
	Values Settings
}

// Load reads settings.cfg. A missing file is not an error: the returned state
// has Exists=false and the default values.
func Load(path string) (State, error) {
	clean, err := cleanPath(path)
	if err != nil {
		return State{}, err
	}
	state := State{Path: clean, Values: Defaults()}
	data, exists, _, err := readFile(clean)
	if err != nil {
		return State{}, err
	}
	if !exists {
		return state, nil
	}
	doc := parseDocument(data)
	state.Exists = true
	values := Defaults()
	present := 0
	if value, ok, err := doc.intValue(SectionGUI, keyTTFResolution,
		DefaultTTFResolution); err != nil {
		return State{}, err
	} else if ok {
		values.TTFResolution = value
		present++
	}
	if value, ok, err := doc.intValue(SectionGUI, keyFontSize,
		DefaultFontSize); err != nil {
		return State{}, err
	} else if ok {
		values.FontSize = value
		present++
	}
	if value, ok, err := doc.floatValue(SectionGUI, keyScalingFactor); err != nil {
		return State{}, err
	} else if ok {
		values.ScalingFactor = value
		present++
	}
	if value, ok, err := doc.intValue(SectionVideo, keyFramerateLimit,
		DefaultFramerateLimit); err != nil {
		return State{}, err
	} else if ok {
		values.FramerateLimit = value
		present++
	}
	if err := values.Validate(); err != nil {
		return State{}, err
	}
	state.Values = values
	state.Configured = present == 4
	return state, nil
}

// Update writes the four managed keys and returns the reloaded state. Every
// other line, comment and the line ending style are preserved. An existing file
// is copied to .bak-<timestamp> before the atomic replacement.
func Update(path string, values Settings) (State, error) {
	clean, err := cleanPath(path)
	if err != nil {
		return State{}, err
	}
	if err := values.Validate(); err != nil {
		return State{}, err
	}
	data, exists, mode, err := readFile(clean)
	if err != nil {
		return State{}, err
	}
	if err := os.MkdirAll(filepath.Dir(clean), 0o700); err != nil {
		return State{}, fmt.Errorf("create %s: %w", filepath.Dir(clean), err)
	}
	doc := parseDocument(data)
	doc.set(SectionGUI, keyTTFResolution, strconv.Itoa(values.TTFResolution))
	doc.set(SectionGUI, keyFontSize, strconv.Itoa(values.FontSize))
	doc.set(SectionGUI, keyScalingFactor, formatFloat(values.ScalingFactor))
	doc.set(SectionVideo, keyFramerateLimit, strconv.Itoa(values.FramerateLimit))
	out := doc.bytes()
	if exists && bytes.Equal(out, data) {
		return Load(clean)
	}
	if exists {
		backup := fmt.Sprintf("%s.bak-%s", clean,
			time.Now().UTC().Format("20060102-150405"))
		if err := atomicfile.WriteFile(backup, data, mode); err != nil {
			return State{}, fmt.Errorf("back up %s: %w", clean, err)
		}
	}
	if err := atomicfile.WriteFile(clean, out, mode); err != nil {
		return State{}, fmt.Errorf("write %s: %w", clean, err)
	}
	return Load(clean)
}

func cleanPath(path string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", errors.New("settings.cfg path is empty")
	}
	if !filepath.IsAbs(path) {
		return "", fmt.Errorf("settings.cfg path %q is not absolute", path)
	}
	return filepath.Clean(path), nil
}

func readFile(path string) ([]byte, bool, os.FileMode, error) {
	info, err := os.Lstat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, false, newFileMode, nil
	}
	if err != nil {
		return nil, false, 0, fmt.Errorf("read %s: %w", path, err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return nil, false, 0, invalidf("", "%s is not a regular file", path)
	}
	if info.Size() > maxFileSize {
		return nil, false, 0, invalidf("", "%s is larger than %d bytes",
			path, maxFileSize)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, false, 0, fmt.Errorf("read %s: %w", path, err)
	}
	mode := info.Mode().Perm()
	if mode == 0 {
		mode = newFileMode
	}
	return data, true, mode, nil
}

func formatFloat(value float64) string {
	return strconv.FormatFloat(value, 'f', -1, 64)
}

// Settings are the four values the launcher manages. Everything else in the
// file is left untouched.
type Settings struct {
	// TTFResolution is the [GUI] "ttf resolution" in pixels.
	TTFResolution int
	// FontSize is the [GUI] "font size" in points.
	FontSize int
	// ScalingFactor is the [GUI] "scaling factor" of the interface.
	ScalingFactor float64
	// FramerateLimit is the [Video] "framerate limit"; 0 means unlimited.
	FramerateLimit int
}

// Defaults are the values OpenMW ships with. They are used for keys a file does
// not define yet.
func Defaults() Settings {
	return Settings{
		TTFResolution:  DefaultTTFResolution,
		FontSize:       DefaultFontSize,
		ScalingFactor:  1,
		FramerateLimit: DefaultFramerateLimit,
	}
}

// Validate reports whether every value is inside the accepted range.
func (s Settings) Validate() error {
	switch {
	case s.TTFResolution < MinTTFResolution ||
		s.TTFResolution > MaxTTFResolution:
		return invalidf(keyTTFResolution, "must be %d..%d, got %d",
			MinTTFResolution, MaxTTFResolution, s.TTFResolution)
	case s.FontSize < MinFontSize || s.FontSize > MaxFontSize:
		return invalidf(keyFontSize, "must be %d..%d, got %d",
			MinFontSize, MaxFontSize, s.FontSize)
	case s.ScalingFactor < MinScalingFactor || s.ScalingFactor > MaxScalingFactor:
		return invalidf(keyScalingFactor, "must be %.1f..%.1f, got %g",
			float64(MinScalingFactor), float64(MaxScalingFactor), s.ScalingFactor)
	case s.FramerateLimit < MinFramerateLimit ||
		s.FramerateLimit > MaxFramerateLimit:
		return invalidf(keyFramerateLimit, "must be %d..%d, got %d",
			MinFramerateLimit, MaxFramerateLimit, s.FramerateLimit)
	}
	return nil
}
