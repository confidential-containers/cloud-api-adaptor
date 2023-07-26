// (C) Copyright IBM Corp. 2022.
// SPDX-License-Identifier: Apache-2.0

package interceptor

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewInterceptor(t *testing.T) {

	socketName := "dummy.sock"

	i := NewInterceptor(socketName, "")
	if i == nil {
		t.Fatal("Expect non nil, got nil")
	}
}

func TestIsTargetPath(t *testing.T) {
	path := "/path/to/target"

	assert.False(t, isTargetPath(path, ""))
	assert.False(t, isTargetPath("", ""))
	assert.False(t, isTargetPath(path, "mock path"))
	assert.True(t, isTargetPath(path, "/path/to/target"))
}
