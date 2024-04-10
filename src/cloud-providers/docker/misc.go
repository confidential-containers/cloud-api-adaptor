// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package docker

import (
	"context"
	"fmt"
	"os"
)

const DockerUserDataUrl = "/peerpod/userdata.json"

// Method to check if the VM is running on Docker
func IsDocker(ctx context.Context) bool {

	// Check for .dockerenv file under /
	// If the file exists, then the VM is running on Docker
	_, err := os.ReadFile("/.dockerenv")
	return err == nil
}

// Method to retrieve userData from a well defined path
// and return it as a string
func GetUserData(ctx context.Context, url string) ([]byte, error) {

	// If url is empty then return empty string
	if url == "" {
		return nil, fmt.Errorf("url is empty")
	}

	// The url is a file path
	// Read the file and return its content
	userData, err := os.ReadFile(url)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %s", err)
	}

	return userData, nil
}
