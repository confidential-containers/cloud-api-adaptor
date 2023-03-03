//go:build !cgo

// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package provisioner

func NewLibvirtProvisioner(properties map[string]string) (CloudProvisioner, error) {
	panic("CGO is not enabled")
}