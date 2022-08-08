// (C) Copyright IBM Corp. 2022.
// SPDX-License-Identifier: Apache-2.0

package interceptor

import (
	"testing"
)

func TestNewInterceptor(t *testing.T) {

	socketName := "dummy.sock"

	i := NewInterceptor(socketName, "")
	if i == nil {
		t.Fatal("Expect non nil, got nil")
	}
}
