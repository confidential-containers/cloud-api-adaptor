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

// SkipTestIfRunningInCI skips the test if running in CI environment.
func SkipTestIfRunningInCI(t *testing.T) {
	value, exported := os.LookupEnv("CI")

	if exported && value == "true" {
		t.Skip("This test is disabled on CI. Skipping.")
	}
}
