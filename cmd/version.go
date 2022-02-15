// (C) Copyright IBM Corp. 2022.
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"fmt"
	"runtime"
)

var (
	version   = "unknown"
	gitCommit = ""
)

func ShowVersion(programName string) {

	fmt.Printf("%s version %s\n", programName, version)

	if gitCommit != "" {
		fmt.Printf("commit: %s\n", gitCommit)
	}

	fmt.Printf("go: %s\n", runtime.Version())
}
