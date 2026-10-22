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

const osReleaseEnv = "TES3MP_EASY_OS_RELEASE"

func runDoctor(opts options, args []string, client *systemd.Client,
	stdout, stderr io.Writer,
) int {
	if parseErr := parseDoctorArgs(args); parseErr != nil {
		return writeError(stdout, stderr, opts.json, parseErr)
	}

	osReleasePath := doctor.DefaultOSReleasePath
	if path := os.Getenv(osReleaseEnv); path != "" {
		osReleasePath = path
	}

	report := doctor.Run(context.Background(), doctor.Options{
		Root:           opts.root,
		OSReleasePath:  osReleasePath,
		CommandTimeout: opts.commandTimeout,
		Systemd:        client,
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
