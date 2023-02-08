// (C) Copyright IBM Corp. 2022.
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"fmt"
	"runtime"
)

var (
	VERSION = "unknown"
	COMMIT  = "unknown"
)

func ShowVersion(programName string) {
	fmt.Printf("%s, version: %s, commit: %v\n", programName, VERSION, COMMIT)

	fmt.Printf("go: %s\n", runtime.Version())
}
