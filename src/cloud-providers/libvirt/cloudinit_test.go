// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package libvirt

import (
	CR "crypto/rand"
	"io"
	"math/rand"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"libvirt.org/go/libvirtxml"

	"github.com/kdomanski/iso9660"
)

func TestCloudInit(t *testing.T) {
	tests := []struct {
		name            string
		userDataContent []byte
		metaDataContent []byte
		expectedFiles   map[string][]byte
	}{
		{
			name:            "basic cloud-init",
			userDataContent: []byte("userdata"),
			metaDataContent: []byte("metadata"),
			expectedFiles: map[string][]byte{
				userDataFilename: []byte("userdata"),
				metaDataFilename: []byte("metadata"),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isoData, err := createCloudInit(tt.userDataContent, tt.metaDataContent)
			require.NoError(t, err)

			files := verifyISOContents(t, isoData)

			for filename, expectedContent := range tt.expectedFiles {
				assert.Equal(t, expectedContent, files[filename])
			}
		})
	}
}

func TestInMemoryCopier(t *testing.T) {
	// generate some test data
	size := rand.Intn(1000) + 1000
	buf := make([]byte, size)
	_, err := CR.Read(buf)
	require.NoError(t, err)
	// build the image abstraction
	img, err := newImageFromBytes(buf)
	require.NoError(t, err)

	sizeFromImg, err := img.size()
	require.NoError(t, err)
	assert.Equal(t, uint64(size), sizeFromImg)

	var otherBuf []byte
	err = img.importImage(func(rdr io.Reader) error {
		bufRead, err := io.ReadAll(rdr)
		otherBuf = bufRead
		return err
	}, libvirtxml.StorageVolume{})
	require.NoError(t, err)

	assert.Equal(t, buf, otherBuf)
}

func TestCreateCloudInitVariations(t *testing.T) {
	largeData := make([]byte, 10000)
	for i := range largeData {
		largeData[i] = byte('A' + (i % 26))
	}

	tests := []struct {
		name              string
		userDataContent   []byte
		metaDataContent   []byte
		expectError       bool
		verifyFileCount   bool
		expectedFileCount int
		verifyVendorData  bool
	}{
		{
			name:            "empty data",
			userDataContent: []byte(""),
			metaDataContent: []byte(""),
			expectError:     false,
		},
		{
			name:            "large data",
			userDataContent: largeData,
			metaDataContent: []byte("instance-id: test-instance\nlocal-hostname: test-host"),
			expectError:     false,
		},
		{
			name:              "special characters",
			userDataContent:   []byte("#cloud-config\nusers:\n  - name: test\n    ssh-authorized-keys:\n      - ssh-rsa AAAAB3..."),
			metaDataContent:   []byte("instance-id: test-123\nlocal-hostname: test-host-456"),
			expectError:       false,
			verifyFileCount:   true,
			expectedFileCount: 3,
		},
		{
			name:             "verify vendor data",
			userDataContent:  []byte("userdata"),
			metaDataContent:  []byte("metadata"),
			expectError:      false,
			verifyVendorData: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isoData, err := createCloudInit(tt.userDataContent, tt.metaDataContent)

			if tt.expectError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.NotEmpty(t, isoData)

			files := verifyISOContents(t, isoData)

			if tt.verifyFileCount {
				assert.Equal(t, tt.expectedFileCount, len(files))
			}

			if tt.verifyVendorData {
				assert.Contains(t, files, vendorDataFilename)
				assert.Equal(t, []byte{}, files[vendorDataFilename])
			}
		})
	}
}

func verifyISOContents(t *testing.T, isoData []byte) map[string][]byte {
	t.Helper()

	file, err := os.CreateTemp("", "CloudInit-*.iso")
	require.NoError(t, err)
	defer os.Remove(file.Name())

	err = os.WriteFile(file.Name(), isoData, os.ModePerm)
	require.NoError(t, err)

	isoFile, err := os.Open(file.Name())
	require.NoError(t, err)
	defer isoFile.Close()

	isoImg, err := iso9660.OpenImage(isoFile)
	require.NoError(t, err)

	rootFile, err := isoImg.RootDir()
	require.NoError(t, err)

	children, err := rootFile.GetChildren()
	require.NoError(t, err)

	files := make(map[string][]byte)
	for _, child := range children {
		data, err := io.ReadAll(child.Reader())
		require.NoError(t, err)
		files[child.Name()] = data
	}

	return files
}
