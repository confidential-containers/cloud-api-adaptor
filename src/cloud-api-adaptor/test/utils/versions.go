// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"os"

	yaml "gopkg.in/yaml.v2"
)

// Relative to test/e2e
const VersionsFile = "../../versions.yaml"

// Versions represents the project's versions.yaml
type Versions struct {
	Git map[string]struct {
		URL    string `yaml:"url"`
		Ref    string `yaml:"reference"`
		Config string `yaml:"config"`
	}
}

// GetVersions unmarshals the project's versions.yaml
func GetVersions() (*Versions, error) {
	var versions Versions

	yamlFile, err := os.ReadFile(VersionsFile)
	if err != nil {
		return nil, err
	}

	if err := yaml.Unmarshal(yamlFile, &versions); err != nil {
		return nil, err
	}

	return &versions, nil
}
