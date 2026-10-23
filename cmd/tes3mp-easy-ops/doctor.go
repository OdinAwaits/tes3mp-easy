package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/Saagilat/tes3mp-easy/internal/doctor"
	"github.com/Saagilat/tes3mp-easy/internal/ops"
	"github.com/Saagilat/tes3mp-easy/internal/systemd"
)

const (
	osReleaseEnv      = "TES3MP_EASY_OS_RELEASE"
	procRootEnv       = "TES3MP_EASY_PROC_ROOT"
	sysctlConfEnv     = "TES3MP_EASY_SYSCTL_CONF"
	journaldConfEnv   = "TES3MP_EASY_JOURNALD_CONF"
	runtimeRootEnv    = "TES3MP_EASY_RUNTIME_ROOT"
	skipOwnerCheckEnv = "TES3MP_EASY_DOCTOR_SKIP_OWNERS"
)

func runDoctor(opts options, args []string, client *systemd.Client,
	stdout, stderr io.Writer,
) int {
	if parseErr := parseDoctorArgs(args); parseErr != nil {
		return writeError(stdout, stderr, opts.json, parseErr)
	}

	report := doctor.Run(context.Background(), doctor.Options{
		Root:           opts.root,
		OSReleasePath:  envPath(osReleaseEnv, doctor.DefaultOSReleasePath),
		ProcRoot:       envPath(procRootEnv, doctor.DefaultProcRoot),
		SysctlPath:     envPath(sysctlConfEnv, doctor.DefaultSysctlPath),
		JournaldConfig: envPath(journaldConfEnv, doctor.DefaultJournaldConfigPath),
		RuntimeRoot:    envPath(runtimeRootEnv, doctor.DefaultRuntimeRoot),
		CommandTimeout: opts.commandTimeout,
		Systemd:        client,
		SkipOwnership:  os.Getenv(skipOwnerCheckEnv) == "1",
	})

	if opts.json {
		if code := writeJSONSuccess(stdout, stderr, report); code != 0 {
			return code
		}
	} else if err := writeDoctorHuman(stdout, report); err != nil {
		return writeOutputFailure(stderr, err)
	}

	if report.Status == doctor.StatusError {
		return int(ops.ExitChecksFailed)
	}
	return int(ops.ExitOK)
}

func envPath(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}

func parseDoctorArgs(args []string) *ops.Error {
	fs := flag.NewFlagSet("doctor", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.Usage = func() {}

	if err := fs.Parse(args); err != nil {
		return ops.NewError(ops.ExitInvalidArgument,
			fmt.Sprintf("doctor: %v", err))
	}
	if fs.NArg() != 0 {
		return ops.NewError(ops.ExitInvalidArgument,
			fmt.Sprintf("doctor: unexpected argument %q", fs.Arg(0)))
	}
	return nil
}

func writeDoctorHuman(w io.Writer, report doctor.Report) error {
	var (
		out                strings.Builder
		ok, warn, failures int
	)
	for _, check := range report.Checks {
		switch check.Status {
		case doctor.StatusOK:
			ok++
		case doctor.StatusWarn:
			warn++
		case doctor.StatusError:
			failures++
		}

		fmt.Fprintf(&out, "%s: %s", check.Name, check.Status)
		if check.Message != "" {
			fmt.Fprintf(&out, " (%s)", check.Message)
		}
		out.WriteByte('\n')
	}
	fmt.Fprintf(&out, "doctor: %s (%d ok, %d warn, %d error)\n",
		report.Status, ok, warn, failures)
	_, err := io.WriteString(w, out.String())
	return err
}
