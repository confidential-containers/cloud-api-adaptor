// Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"path/filepath"
	"strings"
)

// TODO make these normalize functions generic

func NormalizeVolumeID(volumeID string) string {
	normalizedVolumeID := strings.ReplaceAll(volumeID, "###", ".")
	normalizedVolumeID = strings.ReplaceAll(normalizedVolumeID, "#", ".")

	return filepath.Base(normalizedVolumeID)
}

func NormalizeVMID(vmID string) string {
	split := strings.Split(vmID, "/")
	return split[len(split)-1]
}
