// Command tes3mp-easy-panel serves the tes3mp-easy HTTPS panel.
//
// This step provides the HTTPS listener, ACME TLS material, password
// authentication, sessions, CSRF protection, and the public server/mods/backups
// API and pages. Admin server management is added in later steps.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/Saagilat/tes3mp-easy/internal/panel"
)

const usage = `Usage: tes3mp-easy-panel [flags]
       tes3mp-easy-panel reset-password [flags]

Commands:
  reset-password            replace the password hash with a new temporary
                            password, print it and exit

Flags:
  --root <path>             data root (default /tes3mp-easy; env TES3MP_PANEL_ROOT)
  --data-dir <path>         panel data directory (default <root>/panel-data;
                            env TES3MP_PANEL_DATA_DIR)
  --https-addr <addr>       HTTPS listener (default :443;
                            env TES3MP_PANEL_HTTPS_ADDR)
  --public-address <ip>     public IP used in the ACME certificate; required in
                            acme mode (env TES3MP_PANEL_PUBLIC_ADDRESS)
  --tls-mode <mode>         acme or files (default acme;
                            env TES3MP_PANEL_TLS_MODE)
  --acme-directory <url>    ACME directory (default Let's Encrypt production;
                            env TES3MP_PANEL_ACME_DIRECTORY)
  --acme-email <address>    optional ACME account contact
                            (env TES3MP_PANEL_ACME_EMAIL)
  --acme-profile <name>     ACME certificate profile (default shortlived;
                            env TES3MP_PANEL_ACME_PROFILE)
  --tls-cert <path>         certificate for files mode
                            (default <data-dir>/tls/cert.pem;
                            env TES3MP_PANEL_TLS_CERT)
  --tls-key <path>          private key for files mode
                            (default <data-dir>/tls/key.pem;
                            env TES3MP_PANEL_TLS_KEY)
  --version                 print version and exit
  --help                    print this help and exit

The panel only listens on :443. In acme mode it obtains and renews an RFC 8738
Let's Encrypt certificate for the public IP through HTTP-01, temporarily binding
:80 only while a challenge is active. Port 80 must be reachable from the
internet and free at issuance time. The shortlived profile is required by Let's
Encrypt for IP certificates. The account key lives in
<data-dir>/acme/account.key; cert.pem and key.pem are written with mode 0600.

In files mode the panel never contacts an ACME server and serves the existing
certificate/key pair. The 443 listener requires CAP_NET_BIND_SERVICE in the
systemd unit; logs go to stdout/stderr.

If auth.json is absent, startup creates a random temporary password, stores its
hash in <data-dir>/auth.json (0600) and writes the password to
<data-dir>/initial-password.txt (0600). The first login must change it. Use
reset-password if that file was lost.
`

func main() {
	ctx, stop := signal.NotifyContext(context.Background(),
		os.Interrupt, syscall.SIGTERM)
	defer stop()
	os.Exit(runContext(ctx, os.Args[1:], os.Getenv, os.Stdout, os.Stderr))
}

func run(args []string, getenv func(string) string, stdout, stderr io.Writer) int {
	return runContext(context.Background(), args, getenv, stdout, stderr)
}

func runContext(ctx context.Context, args []string, getenv func(string) string,
	stdout, stderr io.Writer) int {
	if wantsHelp(args) {
		fmt.Fprint(stdout, usage)
		return 0
	}
	if len(args) > 0 && args[0] == "reset-password" {
		return runResetPassword(args[1:], getenv, stdout, stderr)
	}

	cfg, showVersion, err := parseConfig(args, getenv)
	if err != nil {
		fmt.Fprintf(stderr, "tes3mp-easy-panel: %v\n", err)
		return 2
	}
	if showVersion {
		fmt.Fprintf(stdout, "tes3mp-easy-panel %s\n", panel.Version)
		return 0
	}
	if err := cfg.Validate(); err != nil {
		fmt.Fprintf(stderr, "tes3mp-easy-panel: invalid config: %v\n", err)
		return 2
	}

	logger := log.New(stderr, "panel: ", log.LstdFlags|log.LUTC)
	server, err := panel.New(cfg, nil, logger)
	if err != nil {
		fmt.Fprintf(stderr, "tes3mp-easy-panel: %v\n", err)
		return 1
	}
	if err := server.Serve(ctx); err != nil {
		fmt.Fprintf(stderr, "tes3mp-easy-panel: %v\n", err)
		return 1
	}
	return 0
}

func runResetPassword(args []string, getenv func(string) string,
	stdout, stderr io.Writer) int {
	cfg, showVersion, err := parseConfig(args, getenv)
	if err != nil {
		fmt.Fprintf(stderr, "tes3mp-easy-panel: %v\n", err)
		return 2
	}
	if showVersion {
		fmt.Fprintf(stdout, "tes3mp-easy-panel %s\n", panel.Version)
		return 0
	}

	password, err := panel.ResetPassword(cfg)
	if err != nil {
		fmt.Fprintf(stderr, "tes3mp-easy-panel: reset password: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "temporary password: %s\n", password)
	fmt.Fprintf(stdout, "written to %s\n",
		filepath.Join(cfg.DataDir, panel.InitialPasswordFileName))
	return 0
}

func parseConfig(args []string, getenv func(string) string) (panel.Config, bool, error) {
	cfg := panel.ConfigFromEnv(getenv)

	fs := flag.NewFlagSet("tes3mp-easy-panel", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.Usage = func() {}

	fs.StringVar(&cfg.Root, "root", cfg.Root, "")
	fs.StringVar(&cfg.DataDir, "data-dir", cfg.DataDir, "")
	fs.StringVar(&cfg.HTTPSAddr, "https-addr", cfg.HTTPSAddr, "")
	fs.StringVar(&cfg.PublicAddress, "public-address", cfg.PublicAddress, "")
	fs.StringVar(&cfg.TLSMode, "tls-mode", cfg.TLSMode, "")
	fs.StringVar(&cfg.ACMEDirectory, "acme-directory", cfg.ACMEDirectory, "")
	fs.StringVar(&cfg.ACMEEmail, "acme-email", cfg.ACMEEmail, "")
	fs.StringVar(&cfg.ACMEProfile, "acme-profile", cfg.ACMEProfile, "")
	fs.StringVar(&cfg.CertFile, "tls-cert", cfg.CertFile, "")
	fs.StringVar(&cfg.KeyFile, "tls-key", cfg.KeyFile, "")
	showVersion := fs.Bool("version", false, "")

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return panel.Config{}, false, errors.New("use --help for usage")
		}
		return panel.Config{}, false, err
	}
	if fs.NArg() != 0 {
		return panel.Config{}, false,
			fmt.Errorf("unexpected argument %q", fs.Arg(0))
	}

	set := make(map[string]bool)
	fs.Visit(func(f *flag.Flag) { set[f.Name] = true })
	for _, check := range []struct {
		name  string
		value string
	}{
		{name: "root", value: cfg.Root},
		{name: "data-dir", value: cfg.DataDir},
		{name: "https-addr", value: cfg.HTTPSAddr},
		{name: "tls-mode", value: cfg.TLSMode},
		{name: "acme-directory", value: cfg.ACMEDirectory},
		{name: "acme-profile", value: cfg.ACMEProfile},
		{name: "tls-cert", value: cfg.CertFile},
		{name: "tls-key", value: cfg.KeyFile},
	} {
		if set[check.name] && check.value == "" {
			return panel.Config{}, false,
				fmt.Errorf("--%s must not be empty", check.name)
		}
	}

	return cfg.Normalize(), *showVersion, nil
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
