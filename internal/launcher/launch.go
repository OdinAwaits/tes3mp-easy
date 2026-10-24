package launcher

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// OpenMWCSURL is shown when the editor is not bundled with TES3MP.
const OpenMWCSURL = "https://openmw.org/downloads/"

// Sentinel launch failures are classified into stable T3E codes by internal/app.
var (
	ErrTES3MPMissing = errors.New("TES3MP is not installed")
	ErrProton        = errors.New("Proton runtime is not usable")
	ErrConfig        = errors.New("game configuration is invalid")
	ErrStart         = errors.New("TES3MP could not be started")
)

// LaunchOptions adds the profile, endpoint and font/localization choices to the
// paths. ServerPassword is written only to tes3mp-client-default.cfg and must
// never be logged.
type LaunchOptions struct {
	Options
	ProfilePath    string
	ServerAddress  string
	ServerPort     int
	ServerPassword string
	Localization   bool
	Fonts          bool
	Now            func() time.Time
	Executor       Executor
}

// Result describes a prepared and started process.
type Result struct {
	Environment  Environment
	Config       ConfigResult
	ClientConfig ClientConfigResult
	Spec         Spec
	Process      Process
	Warnings     []string
}

// Launch resolves the platform paths, prepares prefix/config and starts TES3MP
// without waiting for it to exit.
func Launch(ctx context.Context, options LaunchOptions) (Result, error) {
	if strings.TrimSpace(options.ServerAddress) == "" {
		return Result{}, errors.New("server address is empty")
	}
	if options.ServerPort < 1 || options.ServerPort > 65535 {
		return Result{}, fmt.Errorf("server port %d is outside 1..65535",
			options.ServerPort)
	}
	env, err := Resolve(options.Options)
	if err != nil {
		return Result{}, err
	}
	return launchExecutable(ctx, options, env, env.Executable, true)
}

// OpenMWCS starts openmw-cs.exe from the TES3MP installation.
func OpenMWCS(ctx context.Context, options LaunchOptions) (Result, error) {
	env, err := Resolve(options.Options)
	if err != nil {
		return Result{}, err
	}
	executable := filepath.Join(env.TES3MPDir, OpenMWCSExecutable)
	info, err := os.Lstat(executable)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Result{}, fmt.Errorf(
				"OpenMW-CS is not included in TES3MP; download it from %s and place %s in %s",
				OpenMWCSURL, OpenMWCSExecutable, env.TES3MPDir)
		}
		return Result{}, fmt.Errorf("inspect OpenMW-CS: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return Result{}, fmt.Errorf("%s is not a regular file", executable)
	}
	return launchExecutable(ctx, options, env, executable, false)
}

func launchExecutable(ctx context.Context, options LaunchOptions,
	env Environment, executable string, writeClientConfig bool,
) (Result, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if env.ProtonExe != "" {
		info, err := os.Lstat(env.ProtonExe)
		if err != nil {
			return Result{}, fmt.Errorf("%w: %v", ErrProton, err)
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() ||
			info.Mode()&0o111 == 0 {
			return Result{}, fmt.Errorf(
				"%w: %s is not executable", ErrProton, env.ProtonExe)
		}
	}
	warnings, err := preparePrefix(env)
	if err != nil {
		return Result{}, fmt.Errorf("%w: %v", ErrConfig, err)
	}
	config, err := PrepareConfig(ConfigOptions{
		ConfigDir:         env.ConfigDir,
		ProfilePath:       options.ProfilePath,
		Localization:      options.Localization,
		Fonts:             options.Fonts,
		MorrowindEncoding: env.Morrowind.Encoding,
		Now:               options.Now,
	})
	if err != nil {
		return Result{}, fmt.Errorf("%w: %v", ErrConfig, err)
	}
	var clientConfig ClientConfigResult
	if writeClientConfig {
		clientConfig, err = PrepareClientConfig(ClientConfigOptions{
			TES3MPDir: env.TES3MPDir,
			Address:   options.ServerAddress,
			Port:      options.ServerPort,
			Password:  options.ServerPassword,
			Now:       options.Now,
		})
		if err != nil {
			return Result{}, fmt.Errorf("%w: %v", ErrConfig, err)
		}
	}
	spec, err := buildSpec(env, executable)
	if err != nil {
		return Result{}, fmt.Errorf("%w: %v", ErrStart, err)
	}
	executor := options.Executor
	if executor == nil {
		executor = CommandExecutor
	}
	process, err := executor(ctx, spec)
	if err != nil {
		return Result{}, fmt.Errorf("%w: %v", ErrStart, err)
	}
	return Result{
		Environment:  env,
		Config:       config,
		ClientConfig: clientConfig,
		Spec:         spec,
		Process:      process,
		Warnings:     warnings,
	}, nil
}

func buildSpec(env Environment, executable string) (Spec, error) {
	if strings.TrimSpace(executable) == "" {
		return Spec{}, errors.New("launcher executable is empty")
	}
	if env.GOOS == "windows" {
		return Spec{Name: executable, Dir: env.TES3MPDir}, nil
	}
	if env.ProtonExe == "" {
		return Spec{}, errors.New("Proton executable is not configured")
	}
	return Spec{
		Name: env.ProtonExe,
		Args: []string{"run", executable},
		Env: []string{
			"SteamAppId=" + SteamAppID,
			"SteamGameId=" + SteamAppID,
			"STEAM_COMPAT_DATA_PATH=" + env.PrefixDir,
			"STEAM_COMPAT_CLIENT_INSTALL_PATH=" + env.SteamRoot,
		},
		Dir: env.TES3MPDir,
	}, nil
}

func preparePrefix(env Environment) ([]string, error) {
	if env.GOOS != "linux" {
		return nil, nil
	}
	if env.PrefixDir == "" {
		return nil, errors.New("Proton prefix is empty")
	}
	if info, err := os.Lstat(env.PrefixDir); err == nil {
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return nil, fmt.Errorf("Proton prefix %s is not a directory",
				env.PrefixDir)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("inspect Proton prefix: %w", err)
	}
	driveC := filepath.Join(env.PrefixDir, "pfx", "drive_c")
	if err := os.MkdirAll(driveC, 0o700); err != nil {
		return nil, fmt.Errorf("create Proton prefix: %w", err)
	}
	link := filepath.Join(driveC, "Morrowind")
	var warnings []string
	info, err := os.Lstat(link)
	switch {
	case errors.Is(err, os.ErrNotExist):
		if err := os.Symlink(env.Morrowind.Root, link); err != nil {
			return nil, fmt.Errorf("link Morrowind into Proton prefix: %w", err)
		}
	case err != nil:
		return nil, fmt.Errorf("inspect Proton prefix link: %w", err)
	case info.Mode()&os.ModeSymlink == 0 && info.IsDir():
		warnings = append(warnings, fmt.Sprintf(
			"Proton prefix already has a Morrowind directory at %s; keeping it",
			link))
		return warnings, nil
	case info.Mode()&os.ModeSymlink == 0:
		return nil, fmt.Errorf("Proton prefix path %s is not a directory", link)
	default:
		target, err := os.Readlink(link)
		if err != nil {
			return nil, fmt.Errorf("read Proton prefix link: %w", err)
		}
		if filepath.Clean(target) == filepath.Clean(env.Morrowind.Root) {
			return nil, nil
		}
		if err := os.Remove(link); err != nil {
			return nil, fmt.Errorf("replace Proton prefix link: %w", err)
		}
		if err := os.Symlink(env.Morrowind.Root, link); err != nil {
			return nil, fmt.Errorf("link Morrowind into Proton prefix: %w", err)
		}
	}
	return warnings, nil
}
