//go:build ibmcloud

// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package ibmcloud

import (
	"fmt"
	"strings"
	"testing"
)

func TestIBMCloudMasking(t *testing.T) {
	apiKey := "abcdefg"
	zoneName := "eu-gb"
	cloudCfg := Config{
		APIKey:   apiKey,
		ZoneName: zoneName,
	}
	checkLine := func(verb string) {
		logline := fmt.Sprintf(verb, cloudCfg.Redact())
		if strings.Contains(logline, apiKey) {
			t.Errorf("For verb %s: %s contains the api key: %s", verb, logline, apiKey)
		}
		if !strings.Contains(logline, zoneName) {
			t.Errorf("For verb %s: %s doesn't contain the zone name: %s", verb, logline, zoneName)
		}
	}
	checkLine("%v")
	checkLine("%s")

	if cloudCfg.APIKey != apiKey {
		t.Errorf("Original ApiKey field value has been overwritten")
	}
}

func TestEmptyList(t *testing.T) {
	var list instanceProfiles
	err := list.Set("")
	if err != nil {
		t.Errorf("List Set failed, %v", err)
	}
	if len(list) != 0 {
		t.Errorf("Expect 0 length, got %d", len(list))
	}
}

func TestImageIDs(t *testing.T) {
	var images Images
	err := images.Set("abc,xyz")
	if err != nil {
		t.Errorf("Image Set failed, %v", err)
	}
	if len(images) != 2 {
		t.Errorf("Expect 2 length, got %d", len(images))
	}
}

func TestImagesToString(t *testing.T) {
	var images Images
	ids := "123-1231-123,adasd-ada-adad,asd-123-asd"
	err := images.Set(ids)
	if err != nil {
		t.Errorf("Image Set failed, %v", err)
	}
	if ids != images.String() {
		t.Errorf("Expect %v, got %v", ids, images.String())
	}
}
