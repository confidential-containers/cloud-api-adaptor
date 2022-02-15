// (C) Copyright IBM Corp. 2022.
// SPDX-License-Identifier: Apache-2.0

package cloudinit

import (
	"reflect"
	"strings"
	"testing"

	"gopkg.in/yaml.v2"
)

func TestUserData(t *testing.T) {
	cloudConfig := &CloudConfig{
		WriteFiles: []WriteFile{
			{Path: "/123", Content: "Hello\n"},
			{Path: "/456", Content: "Hello\nWorld\n", Owner: "root:root"},
		},
	}

	userData, err := cloudConfig.Generate()
	if err != nil {
		t.Fatalf("Expect no error, got %v", err)
	}

	firstLine := userData[0:strings.Index(userData, "\n")]

	if e, a := "#cloud-config", firstLine; e != a {
		t.Fatalf("Expect %q, got %q", e, a)
	}

	var output CloudConfig

	if err := yaml.Unmarshal([]byte(userData), &output); err != nil {
		t.Fatalf("Expect no error, got %v", err)
	}

	if e, a := cloudConfig, &output; !reflect.DeepEqual(e, a) {
		t.Fatalf("Expect %#v, got %#v", e, a)
	}
}
