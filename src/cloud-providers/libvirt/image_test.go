//go:build cgo

// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package libvirt

import (
	"fmt"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	libvirtxml "libvirt.org/go/libvirtxml"
)

const (
	testVolumeName = "test-volume"
)

// TestInMemoryImage tests the core functionality of inMemoryImage
func TestInMemoryImage(t *testing.T) {
	tests := []struct {
		name string
		data []byte
	}{
		{
			name: "text data",
			data: []byte("test image data"),
		},
		{
			name: "empty data",
			data: []byte{},
		},
		{
			name: "binary data",
			data: []byte{0x00, 0x01, 0x02, 0xFF, 0xFE, 0xFD},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test creation
			img, err := newImageFromBytes(tt.data)
			require.NoError(t, err)
			assert.NotNil(t, img)

			// Test size()
			size, err := img.size()
			require.NoError(t, err)
			assert.Equal(t, uint64(len(tt.data)), size)

			// Test string()
			str := img.string()
			expectedStr := fmt.Sprintf("plain bytes of size [%d]", len(tt.data))
			assert.Equal(t, expectedStr, str)
		})
	}
}

// TestInMemoryImageImport tests the importImage functionality
func TestInMemoryImageImport(t *testing.T) {
	tests := []struct {
		name        string
		data        []byte
		expectError bool
		copierFunc  func(*[]byte) func(io.Reader) error
		validate    func(*testing.T, []byte, []byte)
	}{
		{
			name: "successful import",
			data: []byte("test data"),
			copierFunc: func(captured *[]byte) func(io.Reader) error {
				return func(rdr io.Reader) error {
					data, err := io.ReadAll(rdr)
					*captured = data
					return err
				}
			},
			validate: func(t *testing.T, expected, actual []byte) {
				assert.Equal(t, expected, actual)
			},
		},
		{
			name: "copier error",
			data: []byte("test data"),
			copierFunc: func(captured *[]byte) func(io.Reader) error {
				return func(rdr io.Reader) error {
					return assert.AnError
				}
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			img, err := newImageFromBytes(tt.data)
			require.NoError(t, err)

			var capturedData []byte
			copier := tt.copierFunc(&capturedData)

			volumeDef := libvirtxml.StorageVolume{Name: testVolumeName}
			err = img.importImage(copier, volumeDef)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				if tt.validate != nil {
					tt.validate(t, tt.data, capturedData)
				}
			}
		})
	}
}
