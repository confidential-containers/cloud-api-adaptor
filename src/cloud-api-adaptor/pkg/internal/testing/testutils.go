// (C) Copyright Red Hat 2022.
// SPDX-License-Identifier: Apache-2.0

// Package testutils provides utilities for testing.
package testutils

import (
	"os"
	"testing"
)

// SkipTestIfNotRoot skips the test if not running as root user.
func SkipTestIfNotRoot(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("This test requires root privileges. Skipping.")
	}
}
