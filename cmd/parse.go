// (C) Copyright IBM Corp. 2022.
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"errors"
	"flag"
)

func Parse(programName string, args []string, fn func(flags *flag.FlagSet)) {

	var versionFlag bool

	flags := flag.NewFlagSet(programName, flag.ContinueOnError)
	flags.SetOutput(flag.CommandLine.Output())
	flags.BoolVar(&versionFlag, "version", false, "Show version information")

	fn(flags)

	switch programName {
	case "ibmcloud":
		if len(args) < 10 {
			flags.PrintDefaults()
			Exit(1)
		}
	case "aws":
		if len(args) < 2 {
			flags.PrintDefaults()
			Exit(1)
		}

	}

	if err := flags.Parse(args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			Exit(0)
		} else {
			Exit(1)
		}
	}

	if versionFlag {
		ShowVersion(programName)
		Exit(0)
	}
}
