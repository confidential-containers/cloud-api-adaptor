// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package libvirt

import (
	"bytes"

	"github.com/kdomanski/iso9660"
)

const (
	userDataFilename   = "user-data"
	metaDataFilename   = "meta-data"
	vendorDataFilename = "vendor-data"
	ciDataVolumeName   = "cidata"
)

// createCloudInit produces a cloud init ISO file as a data blob with a userdata and a metadata section
func createCloudInit(userData, metaData []byte) ([]byte, error) {
	writer, err := iso9660.NewWriter()
	if err != nil {
		return nil, err
	}
	defer writer.Cleanup() //nolint:errcheck // no need to check error in deferal

	err = writer.AddFile(bytes.NewReader(userData), userDataFilename)
	if err != nil {
		return nil, err
	}

	err = writer.AddFile(bytes.NewReader(metaData), metaDataFilename)
	if err != nil {
		return nil, err
	}

	err = writer.AddFile(bytes.NewReader([]byte{}), vendorDataFilename)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer

	err = writer.WriteTo(&buf, ciDataVolumeName)
	if err != nil {
		return nil, err
	}

	// done
	return buf.Bytes(), nil
}
