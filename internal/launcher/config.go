package launcher

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Saagilat/tes3mp-easy/internal/atomicfile"
	"github.com/Saagilat/tes3mp-easy/internal/fonts"
	"github.com/Saagilat/tes3mp-easy/internal/locale"
)

// ConfigOptions describes the server profile copied into the active OpenMW
// configuration.
type ConfigOptions struct {
	ConfigDir    string
	ProfilePath  string
	Localization bool
	// MorrowindEncoding is the detected encoding of the original ESM files.
	MorrowindEncoding string
	Fonts             bool
	Now               func() time.Time
}

// ConfigResult describes the active openmw.cfg replacement.
type ConfigResult struct {
	Path    string
	Backup  string
	Created bool
}

// PrepareConfig copies the server profile into the active openmw.cfg. The
// profile is read-only; fallback and encoding lines are adjusted in the copy.
// An existing different config is preserved as openmw.cfg.bak-<timestamp>.
func PrepareConfig(options ConfigOptions) (ConfigResult, error) {
	if strings.TrimSpace(options.ConfigDir) == "" {
		return ConfigResult{}, errors.New("OpenMW config directory is empty")
	}
	if !filepath.IsAbs(options.ConfigDir) {
		return ConfigResult{}, fmt.Errorf("OpenMW config directory %q is not absolute",
			options.ConfigDir)
	}
	if strings.TrimSpace(options.ProfilePath) == "" {
		return ConfigResult{}, errors.New("server profile path is empty")
	}
	profilePath, err := filepath.Abs(options.ProfilePath)
	if err != nil {
		return ConfigResult{}, fmt.Errorf("server profile %q: %w",
			options.ProfilePath, err)
	}
	info, err := os.Lstat(profilePath)
	if err != nil {
		return ConfigResult{}, fmt.Errorf("server profile %s: %w", profilePath, err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return ConfigResult{}, fmt.Errorf("server profile %s is not a regular file",
			profilePath)
	}
	profile, err := os.ReadFile(profilePath)
	if err != nil {
		return ConfigResult{}, fmt.Errorf("read server profile: %w", err)
	}
	if err := validateProfile(profile); err != nil {
		return ConfigResult{}, fmt.Errorf("server profile %s: %w",
			profilePath, err)
	}
	configDir := filepath.Clean(options.ConfigDir)
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		return ConfigResult{}, fmt.Errorf("create OpenMW config directory: %w", err)
	}
	active := filepath.Join(configDir, "openmw.cfg")
	result := ConfigResult{Path: active}
	if info, err := os.Lstat(active); err == nil {
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			return ConfigResult{}, fmt.Errorf(
				"active OpenMW config %s is not a regular file", active)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return ConfigResult{}, fmt.Errorf("inspect active OpenMW config: %w", err)
	}

	data := append([]byte(nil), profile...)
	if options.Fonts {
		data = fonts.UpdateConfig(data)
	}
	if options.Localization || strings.EqualFold(strings.TrimSpace(options.MorrowindEncoding), locale.Encoding) {
		data = locale.UpdateEncoding(data, true)
	} else {
		data = locale.UpdateEncoding(data, false)
	}

	if existing, err := os.ReadFile(active); err == nil {
		if bytes.Equal(existing, data) {
			return result, nil
		}
		now := time.Now
		if options.Now != nil {
			now = options.Now
		}
		backup, err := writeBackup(active, existing, now())
		if err != nil {
			return ConfigResult{}, err
		}
		result.Backup = backup
	} else if !errors.Is(err, os.ErrNotExist) {
		return ConfigResult{}, fmt.Errorf("read active OpenMW config: %w", err)
	}
	if err := atomicfile.WriteFile(active, data, 0o600); err != nil {
		return ConfigResult{}, fmt.Errorf("write active OpenMW config: %w", err)
	}
	result.Created = true
	return result, nil
}

func validateProfile(data []byte) error {
	if len(bytes.TrimSpace(data)) == 0 {
		return errors.New("profile is empty")
	}
	if bytes.IndexByte(data, 0) >= 0 {
		return errors.New("profile contains NUL")
	}
	dataLines := 0
	contentLines := 0
	hasMorrowind := false
	for _, raw := range strings.Split(string(data), "\n") {
		line := strings.TrimSpace(strings.TrimSuffix(raw, "\r"))
		switch {
		case strings.HasPrefix(line, "data="):
			value := strings.TrimSpace(strings.TrimPrefix(line, "data="))
			value = strings.Trim(value, `"`)
			if value == "" {
				return errors.New("profile has an empty data= line")
			}
			dataLines++
		case strings.HasPrefix(line, "content="):
			value := strings.TrimSpace(strings.TrimPrefix(line, "content="))
			if value == "" {
				return errors.New("profile has an empty content= line")
			}
			contentLines++
			if strings.EqualFold(value, "Morrowind.esm") {
				hasMorrowind = true
			}
		}
	}
	switch {
	case dataLines == 0:
		return errors.New("profile has no data= lines")
	case contentLines == 0:
		return errors.New("profile has no content= lines")
	case !hasMorrowind:
		return errors.New("profile does not load Morrowind.esm")
	}
	return nil
}

func writeBackup(active string, data []byte, now time.Time) (string, error) {
	base := active + ".bak-" + now.UTC().Format("20060102-150405")
	for suffix := 0; suffix < 100; suffix++ {
		candidate := base
		if suffix > 0 {
			candidate = fmt.Sprintf("%s-%d", base, suffix+1)
		}
		if _, err := os.Lstat(candidate); errors.Is(err, os.ErrNotExist) {
			if err := atomicfile.WriteFile(candidate, data, 0o600); err != nil {
				return "", fmt.Errorf("write OpenMW config backup: %w", err)
			}
			return candidate, nil
		} else if err != nil {
			return "", fmt.Errorf("inspect OpenMW config backup: %w", err)
		}
	}
	return "", errors.New("cannot choose a unique OpenMW config backup name")
}
