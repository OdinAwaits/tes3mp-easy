package main

import (
	"errors"
	"fmt"
	"io"

	"github.com/Saagilat/tes3mp-easy/internal/contract"
	"github.com/Saagilat/tes3mp-easy/internal/ops"
	"github.com/Saagilat/tes3mp-easy/internal/systemd"
)

func writeJSONSuccess(stdout, stderr io.Writer, data any) int {
	if err := contract.Write(stdout, contract.Success(data)); err != nil {
		return writeOutputFailure(stderr, err)
	}
	return int(ops.ExitOK)
}

func writeOutputFailure(stderr io.Writer, err error) int {
	fmt.Fprintf(stderr, "tes3mp-easy-ops: write output: %v\n", err)
	return int(ops.ExitInternal)
}

func writeSystemdFailure(stdout, stderr io.Writer, jsonMode bool, command string, err error) int {
	exit := ops.ExitServiceUnavailable
	if errors.Is(err, systemd.ErrInvalidUnit) {
		exit = ops.ExitInvalidArgument
	}
	return writeError(stdout, stderr, jsonMode,
		ops.NewError(exit, fmt.Sprintf("%s: %v", command, err)))
}
