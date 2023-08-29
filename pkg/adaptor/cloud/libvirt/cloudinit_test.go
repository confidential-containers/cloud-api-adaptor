// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package libvirt

import (
	CR "crypto/rand"
	"fmt"
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

	file, err := os.CreateTemp("", "CloudInit-*.iso")
	require.NoError(t, err)
	defer os.Remove(file.Name())

	fmt.Printf("temp file: %s", file.Name())

	userDataContent := []byte("userdata")
	metaDataContent := []byte("metadata")

	isoData, err := createCloudInit(userDataContent, metaDataContent)
	require.NoError(t, err)

	err = os.WriteFile(file.Name(), isoData, os.ModePerm)
	require.NoError(t, err)

	isoFile, err := os.Open(file.Name())
	require.NoError(t, err)

	isoImg, err := iso9660.OpenImage(isoFile)
	require.NoError(t, err)

	rootFile, err := isoImg.RootDir()
	require.NoError(t, err)

	children, err := rootFile.GetChildren()
	require.NoError(t, err)

	files := make(map[string][]byte)
	for _, child := range children {
		key := child.Name()
		data, err := io.ReadAll(child.Reader())
		require.NoError(t, err)

		files[key] = data
	}

	assert.Equal(t, userDataContent, files[userDataFilename])
	assert.Equal(t, metaDataContent, files[metaDataFilename])

	err = isoFile.Close()
	require.NoError(t, err)
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
