// Command tes3mp-easy-pack builds the server binaries and writes the
// reproducible release archive consumed by bootstrap.sh.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/Saagilat/tes3mp-easy/internal/release"
	"github.com/Saagilat/tes3mp-easy/internal/version"
)

const usage = `Usage: tes3mp-easy-pack [flags]

Builds tes3mp-easy-ops and tes3mp-easy-panel for linux/amd64 and writes
dist/tes3mp-easy-<version>-linux-amd64.tar.gz plus a .sha256 sidecar.

Flags:
  --version <version>  release version (default: project version)
  --source <dir>       repository root (default: current directory)
  --output <dir>       output directory (default: dist)
  --help               print this help and exit
`

type options struct {
	version string
	source  string
	output  string
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	opts, err := parseOptions(args)
	if err != nil {
		fmt.Fprintf(stderr, "tes3mp-easy-pack: %v\n", err)
		return 2
	}
	if opts.version == "" {
		fmt.Fprint(stdout, usage)
		return 0
	}
	if err := pack(opts, stdout); err != nil {
		fmt.Fprintf(stderr, "tes3mp-easy-pack: %v\n", err)
		return 1
	}
	return 0
}

func parseOptions(args []string) (options, error) {
	var opts options
	flags := flag.NewFlagSet("tes3mp-easy-pack", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	flags.Usage = func() {}
	flags.StringVar(&opts.version, "version", version.Version, "")
	flags.StringVar(&opts.source, "source", ".", "")
	flags.StringVar(&opts.output, "output", "dist", "")
	help := flags.Bool("help", false, "")
	flags.BoolVar(help, "h", false, "")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return options{}, errors.New("use --help for usage")
		}
		return options{}, err
	}
	if *help {
		opts.version = ""
		return opts, nil
	}
	if flags.NArg() != 0 {
		return options{}, fmt.Errorf("unexpected argument %q", flags.Arg(0))
	}
	if opts.version == "" {
		return options{}, errors.New("--version must not be empty")
	}
	if opts.source == "" {
		return options{}, errors.New("--source must not be empty")
	}
	if opts.output == "" {
		return options{}, errors.New("--output must not be empty")
	}
	return opts, nil
}

func pack(opts options, stdout io.Writer) error {
	if err := release.ValidateVersion(opts.version); err != nil {
		return err
	}
	source, err := filepath.Abs(opts.source)
	if err != nil {
		return fmt.Errorf("resolve source: %w", err)
	}
	info, err := os.Stat(source)
	if err != nil {
		return fmt.Errorf("source directory: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("source %s is not a directory", source)
	}
	goTool, err := exec.LookPath("go")
	if err != nil {
		return errors.New("go tool is not available")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	temp, err := os.MkdirTemp("", "tes3mp-easy-pack-*")
	if err != nil {
		return fmt.Errorf("create temporary directory: %w", err)
	}
	defer os.RemoveAll(temp)

	binaries := make([]release.Binary, 0, 2)
	for _, item := range []struct {
		name string
		pkg  string
	}{
		{release.OpsBinary, "./cmd/tes3mp-easy-ops"},
		{release.PanelBinary, "./cmd/tes3mp-easy-panel"},
	} {
		output := filepath.Join(temp, item.name)
		build := exec.CommandContext(ctx, goTool, "build",
			"-trimpath",
			"-buildvcs=false",
			"-ldflags=-s -w -buildid= -X github.com/Saagilat/tes3mp-easy/internal/version.Version="+opts.version,
			"-o", output,
			item.pkg,
		)
		build.Dir = source
		build.Env = append(os.Environ(),
			"CGO_ENABLED=0", "GOOS=linux", "GOARCH=amd64")
		if data, err := build.CombinedOutput(); err != nil {
			return fmt.Errorf("build %s: %w\n%s", item.name, err, data)
		}
		binaries = append(binaries, release.Binary{Name: item.name, Path: output})
	}

	result, err := release.Create(release.Options{
		SourceDir: source,
		OutputDir: opts.output,
		Version:   opts.version,
		GOOS:      "linux",
		GOARCH:    "amd64",
		Binaries:  binaries,
	})
	if err != nil {
		return err
	}
	fmt.Fprintf(stdout, "archive: %s\n", result.ArchivePath)
	fmt.Fprintf(stdout, "sha256:  %s\n", result.SHA256)
	fmt.Fprintf(stdout, "checksums: %s\n", result.ChecksumPath)
	return nil
}
