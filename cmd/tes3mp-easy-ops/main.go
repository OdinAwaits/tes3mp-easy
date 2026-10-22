// Command tes3mp-easy-ops is the administrative CLI for tes3mp-easy.
//
// This is the v2 scaffold: only help and argument validation are implemented.
// Commands and the JSON envelope arrive in later steps from docs/v2/03-contracts.md.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
)

const usage = `Usage: tes3mp-easy-ops [global flags] <command> [command flags]

Global flags and commands will be added with the ops contract.
For now the binary only validates invocation; see docs/v2/04-ops.md for the
target command set.
`

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 1 && (args[0] == "-h" || args[0] == "--help" || args[0] == "-help") {
		fmt.Fprint(stdout, usage)
		return 0
	}

	fs := flag.NewFlagSet("tes3mp-easy-ops", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() { fmt.Fprint(stderr, usage) }

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}

	if fs.NArg() == 0 {
		fmt.Fprintln(stderr, "tes3mp-easy-ops: command is required")
		return 2
	}

	fmt.Fprintf(stderr, "tes3mp-easy-ops: command %q is not implemented yet\n", fs.Arg(0))
	return 1
}
