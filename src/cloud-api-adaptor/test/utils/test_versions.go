// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"fmt"
	"os"
	"sync"

	yaml "gopkg.in/yaml.v2"
)

// TestVersions represents the test versions.yaml
type TestVersions struct {
	ContainerImage map[string]ContainerImage `yaml:"test_images"`
}

type ContainerImage struct {
	Registry string `yaml:"registry"`
	Tag      string `yaml:"tag"`
}

func (c ContainerImage) getImage() string {
	return c.Registry + ":" + c.Tag
}

// Use singleton to reduce multiple file reads
var lock = &sync.Mutex{}
var testVersions *TestVersions

func getTestVersions() (*TestVersions, error) {

	if testVersions == nil {
		lock.Lock()
		defer lock.Unlock()
		// Relative to test/e2e
		yamlFile, err := os.ReadFile("versions.yaml")
		if err != nil {
			return nil, err
		}

		if err := yaml.Unmarshal(yamlFile, &testVersions); err != nil {
			return nil, err
		}
	}

	return testVersions, nil
}

func GetImage(image string) (string, error) {
	testVersions, err := getTestVersions()
	if err != nil {
		return "", fmt.Errorf("getTestImage: failed to read image: %s, with error %v", image, err)
	}
	return testVersions.ContainerImage[image].getImage(), nil
}
