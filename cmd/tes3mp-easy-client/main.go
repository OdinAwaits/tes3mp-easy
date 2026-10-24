// Command tes3mp-easy-client is the desktop client for players.
//
// The client owns the Fyne shell, local configuration, setup wizard, mods and
// launcher. Filesystem and HTTP scenarios live in internal/app so they can be
// tested without a display.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"

	clientapp "github.com/Saagilat/tes3mp-easy/internal/app"
	"github.com/Saagilat/tes3mp-easy/internal/config"
	"github.com/Saagilat/tes3mp-easy/internal/ui"
)

const usage = `Usage: tes3mp-easy-client [flags]

Flags:
  --config <path>   configuration file (default: per-user config.json)
  --version         print version and exit
  --help            print this help and exit

The client stores config.json with mode 0600 because it contains server
passwords. Diagnostics go to logs/client.log in the per-user data directory.
`

type options struct {
	config  string
	version bool
}

func main() {
	os.Exit(run(os.Args[1:], os.Getenv, os.Stdout, os.Stderr, ui.Run))
}

func run(args []string, getenv func(string) string, stdout, stderr io.Writer,
	launch func(*clientapp.Service) error,
) int {
	if wantsHelp(args) {
		fmt.Fprint(stdout, usage)
		return 0
	}
	opts, err := parseOptions(args)
	if err != nil {
		fmt.Fprintf(stderr, "tes3mp-easy-client: %v\n", err)
		return 2
	}
	if opts.version {
		fmt.Fprintf(stdout, "tes3mp-easy-client %s (TES3MP %s)\n",
			clientapp.Version, clientapp.TES3MPVersion)
		return 0
	}

	paths, err := config.PathsFor(runtime.GOOS, getenv)
	if err != nil {
		fmt.Fprintf(stderr, "tes3mp-easy-client: %v\n", err)
		return 1
	}
	if opts.config != "" {
		absolute, err := filepath.Abs(opts.config)
		if err != nil {
			fmt.Fprintf(stderr, "tes3mp-easy-client: config path: %v\n", err)
			return 2
		}
		paths.ConfigFile = absolute
	}
	if err := paths.Validate(); err != nil {
		fmt.Fprintf(stderr, "tes3mp-easy-client: invalid paths: %v\n", err)
		return 1
	}

	service, err := clientapp.OpenWithEnv(paths, getenv)
	if err != nil {
		fmt.Fprintf(stderr, "tes3mp-easy-client: %v\n", err)
		return 1
	}
	defer service.Close()
	if launch == nil {
		fmt.Fprintln(stderr, "tes3mp-easy-client: ui launcher is nil")
		return 1
	}
	if err := launch(service); err != nil {
		fmt.Fprintf(stderr, "tes3mp-easy-client: %v\n", err)
		return 1
	}
	return 0
}

func parseOptions(args []string) (options, error) {
	var opts options
	flags := flag.NewFlagSet("tes3mp-easy-client", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	flags.Usage = func() {}
	flags.StringVar(&opts.config, "config", "", "")
	flags.BoolVar(&opts.version, "version", false, "")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return options{}, errors.New("use --help for usage")
		}
		return options{}, err
	}
	if flags.NArg() != 0 {
		return options{}, fmt.Errorf("unexpected argument %q", flags.Arg(0))
	}
	configSet := false
	flags.Visit(func(item *flag.Flag) {
		if item.Name == "config" {
			configSet = true
		}
	})
	if configSet && opts.config == "" {
		return options{}, errors.New("--config must not be empty")
	}
	return opts, nil
}

func wantsHelp(args []string) bool {
	for _, arg := range args {
		if arg == "--" {
			return false
		}
		switch arg {
		case "-h", "-help", "--help":
			return true
		}
	}
	return false
}
