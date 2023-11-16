//go:build cgo

// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package libvirt

// Code copied from https://github.com/openshift/cluster-api-provider-libvirt

import (
	"bytes"
	"fmt"
	"io"

	libvirtxml "libvirt.org/go/libvirtxml"
)

type image interface {
	size() (uint64, error)
	importImage(func(io.Reader) error, libvirtxml.StorageVolume) error
	string() string
}

// inMemoryImage represents an image backed by a byte array in memory
type inMemoryImage struct {
	data []byte
}

// newImageFromBytes creates a new image implementation backed by an in-memory byte array
func newImageFromBytes(source []byte) (image, error) {
	return &inMemoryImage{data: source}, nil
}

func (i *inMemoryImage) string() string {
	return fmt.Sprintf("plain bytes of size [%d]", len(i.data))
}

func (i *inMemoryImage) size() (uint64, error) {
	return uint64(len(i.data)), nil
}

func (i *inMemoryImage) importImage(copier func(io.Reader) error, vol libvirtxml.StorageVolume) error {
	return copier(bytes.NewReader(i.data))
}
