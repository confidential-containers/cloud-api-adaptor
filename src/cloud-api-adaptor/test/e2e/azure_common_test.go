// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"testing"

	armcompute "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v4"
)

func TestVMNameHasPrefix(t *testing.T) {
	name := "podvm-simple-test-12345678"
	tests := []struct {
		name   string
		vm     *armcompute.VirtualMachine
		prefix string
		want   bool
	}{
		{name: "pod name prefix", vm: &armcompute.VirtualMachine{Name: &name}, prefix: "podvm-simple-test-", want: true},
		{name: "different prefix", vm: &armcompute.VirtualMachine{Name: &name}, prefix: "podvm-other-", want: false},
		{name: "missing VM name", vm: &armcompute.VirtualMachine{}, prefix: "podvm-", want: false},
		{name: "missing VM", prefix: "podvm-", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := vmNameHasPrefix(tt.vm, tt.prefix); got != tt.want {
				t.Errorf("vmNameHasPrefix() = %v, want %v", got, tt.want)
			}
		})
	}
}
