// (C) Copyright IBM Corp. 2022.
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"os"
)

// Exit calls os.Exit by default. This variable can be replaced for testing
var Exit = os.Exit
