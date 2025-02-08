//go:build gcp

// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"testing"

	_ "github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/test/provisioner/gcp"
)

func TestFoo(t *testing.T) {
	t.Log("Hello, World!")
}
