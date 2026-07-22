package main

import (
	"fmt"
	"os"

	"github.com/ikigenba/autotune/internal/cli"
)

const version = "autotune dev"

func main() {
	opts, err := cli.Parse(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		fmt.Fprint(os.Stderr, cli.UsageText)
		os.Exit(cli.Usage.ExitCode())
	}
	if opts.Help {
		fmt.Fprint(os.Stdout, cli.UsageText)
		return
	}
	if opts.Version {
		fmt.Fprintln(os.Stdout, version)
		return
	}

	// Full application wiring arrives with the Phase 09 composition root.
}
