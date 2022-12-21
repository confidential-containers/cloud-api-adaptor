// (C) Copyright IBM Corp. 2022.
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"fmt"
	"runtime"

	"github.com/confidential-containers/cloud-api-adaptor/pkg/util"
)

func ShowVersion(programName string) {
	fmt.Printf("%s, version: %s, commit: %v\n", programName, util.VERSION, util.COMMIT)

	fmt.Printf("go: %s\n", runtime.Version())
}
