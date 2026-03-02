// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package gcp

import (
	"os"
	"path/filepath"
)

func expandUser(filePath string) (expandedPath string, err error) {
	if filePath[:2] == "~/" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, filePath[2:]), nil
	}
	return filePath, nil
}
