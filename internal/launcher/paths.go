// Package launcher prepares the active OpenMW configuration and starts TES3MP.
//
// Windows starts tes3mp.exe directly. Linux starts it through Proton with the
// Steam compatibility environment from the legacy client. All process startup
// goes through an Executor so platform behavior can be tested without Wine.
package launcher

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/Saagilat/tes3mp-easy/internal/morrowind"
	"github.com/Saagilat/tes3mp-easy/internal/tes3mp"
)

const (
	// SteamAppID is Morrowind's Steam application id.
	SteamAppID = "22320"
	// OpenMWCSExecutable is the editor shipped with some OpenMW builds.
	OpenMWCSExecutable = "openmw-cs.exe"
)

// Options are the paths and platform inputs needed to resolve a launch.
type Options struct {
	GOOS          string
	WorkDir       string
	MorrowindRoot string
	ProtonPath    string
	PrefixDir     string
	ConfigDir     string
	HomeDir       string
}

// Environment is the resolved set of paths for one launch.
type Environment struct {
	GOOS       string
	WorkDir    string
	TES3MPDir  string
	Executable string
	ConfigDir  string
	PrefixDir  string
	ProtonDir  string
	ProtonExe  string
	SteamRoot  string
	Morrowind  morrowind.Installation
}

// Spec is one process invocation. Env contains only variables added to the
// inherited environment.
type Spec struct {
	Name string
	Args []string
	Env  []string
	Dir  string
}

// Process is a started game process.
type Process interface {
	Wait() error
}

// Executor starts a process. Production uses CommandExecutor.
type Executor func(context.Context, Spec) (Process, error)

// OpenMWConfigDir computes the OpenMW configuration directory without
// requiring Morrowind or TES3MP to be installed. It is used by the font
// installer, which runs before the first launch.
func OpenMWConfigDir(options Options) (string, error) {
	goos := strings.ToLower(strings.TrimSpace(options.GOOS))
	if goos == "" {
		goos = runtime.GOOS
	}
	if goos != "windows" && goos != "linux" {
		return "", fmt.Errorf("unsupported operating system %q", goos)
	}
	if strings.TrimSpace(options.ConfigDir) != "" {
		return filepath.Abs(options.ConfigDir)
	}
	home := strings.TrimSpace(options.HomeDir)
	if home == "" {
		var err error
		home, err = os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("find home directory: %w", err)
		}
	}
	if !filepath.IsAbs(home) {
		return "", fmt.Errorf("home directory %q is not absolute", home)
	}
	if goos == "windows" {
		return filepath.Join(home, "Documents", "My Games", "OpenMW"), nil
	}
	prefix := strings.TrimSpace(options.PrefixDir)
	if prefix == "" {
		workDir, err := absoluteDirectory(options.WorkDir,
			"client work directory")
		if err != nil {
			return "", err
		}
		prefix = filepath.Join(workDir, "prefix")
	}
	prefix, err := filepath.Abs(prefix)
	if err != nil {
		return "", fmt.Errorf("Proton prefix %q: %w", prefix, err)
	}
	return filepath.Join(prefix, "pfx", "drive_c", "users", "steamuser",
		"Documents", "My Games", "OpenMW"), nil
}

// Resolve validates the configured paths and discovers Proton on Linux.
func Resolve(options Options) (Environment, error) {
	goos := strings.ToLower(strings.TrimSpace(options.GOOS))
	if goos == "" {
		goos = runtime.GOOS
	}
	if goos != "windows" && goos != "linux" {
		return Environment{}, fmt.Errorf("unsupported operating system %q", goos)
	}
	workDir, err := absoluteDirectory(options.WorkDir, "client work directory")
	if err != nil {
		return Environment{}, err
	}
	if strings.TrimSpace(options.MorrowindRoot) == "" {
		return Environment{}, errors.New("Morrowind folder is not configured")
	}
	installation, err := morrowind.Inspect(options.MorrowindRoot)
	if err != nil {
		return Environment{}, err
	}
	executable := filepath.Join(workDir, "tes3mp", tes3mp.Executable)
	info, err := os.Lstat(executable)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Environment{}, fmt.Errorf(
				"TES3MP is not installed: %s is missing", executable)
		}
		return Environment{}, fmt.Errorf("inspect TES3MP: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return Environment{}, fmt.Errorf("%s is not a regular file", executable)
	}

	home := strings.TrimSpace(options.HomeDir)
	if home == "" {
		home, err = os.UserHomeDir()
		if err != nil {
			return Environment{}, fmt.Errorf("find home directory: %w", err)
		}
	}
	if !filepath.IsAbs(home) {
		return Environment{}, fmt.Errorf("home directory %q is not absolute", home)
	}
	env := Environment{
		GOOS:       goos,
		WorkDir:    workDir,
		TES3MPDir:  filepath.Dir(executable),
		Executable: executable,
		Morrowind:  installation,
	}
	if goos == "windows" {
		env.ConfigDir = filepath.Join(home, "Documents", "My Games", "OpenMW")
	} else {
		env.PrefixDir = strings.TrimSpace(options.PrefixDir)
		if env.PrefixDir == "" {
			env.PrefixDir = filepath.Join(workDir, "prefix")
		}
		env.PrefixDir, err = filepath.Abs(env.PrefixDir)
		if err != nil {
			return Environment{}, fmt.Errorf("Proton prefix %q: %w",
				env.PrefixDir, err)
		}
		env.ConfigDir = filepath.Join(env.PrefixDir, "pfx", "drive_c",
			"users", "steamuser", "Documents", "My Games", "OpenMW")
		if strings.TrimSpace(options.ProtonPath) == "" {
			env.ProtonDir, err = DiscoverProton(home)
			if err != nil {
				return Environment{}, err
			}
		} else {
			env.ProtonDir, err = protonDir(options.ProtonPath, home)
			if err != nil {
				return Environment{}, err
			}
		}
		env.ProtonExe = filepath.Join(env.ProtonDir, "proton")
		env.SteamRoot = steamRoot(env.ProtonDir, home)
	}
	if strings.TrimSpace(options.ConfigDir) != "" {
		selected, err := filepath.Abs(options.ConfigDir)
		if err != nil {
			return Environment{}, fmt.Errorf("OpenMW config directory %q: %w",
				options.ConfigDir, err)
		}
		env.ConfigDir = selected
	}
	return env, nil
}
