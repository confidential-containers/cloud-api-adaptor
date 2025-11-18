// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"errors"
	"flag"
)

func Parse(programName string, args []string, fn func(flags *flag.FlagSet)) {
	flags := flag.NewFlagSet(programName, flag.ContinueOnError)
	flags.SetOutput(flag.CommandLine.Output())

	fn(flags)

	if err := flags.Parse(args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			Exit(0)
		} else {
			Exit(1)
		}
	}
}
