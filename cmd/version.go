// Copyright Confidential Containers Contributors
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
	fmt.Printf("%s version %s\n", programName, VERSION)
	fmt.Printf("  commit: %s\n", COMMIT)
	fmt.Printf("  go: %s\n", runtime.Version())
}
