// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package aws

import (
	"fmt"
	"strings"
	"testing"
)

func TestAWSMasking(t *testing.T) {
	secretKey := "abcdefg"
	region := "eu-gb"
	cloudCfg := Config{
		SecretKey: secretKey,
		Region:    region,
	}
	checkLine := func(verb string) {
		logline := fmt.Sprintf(verb, cloudCfg.Redact())
		if strings.Contains(logline, secretKey) {
			t.Errorf("For verb %s: %s contains the secret key: %s", verb, logline, secretKey)
		}
		if !strings.Contains(logline, region) {
			t.Errorf("For verb %s: %s doesn't contain the region name: %s", verb, logline, region)
		}
	}
	checkLine("%v")
	checkLine("%s")

	if cloudCfg.SecretKey != secretKey {
		t.Errorf("Original SecretKey field value has been overwritten")
	}
}

func TestEmptyList(t *testing.T) {
	var list instanceTypes
	err := list.Set("")
	if err != nil {
		t.Errorf("List Set failed, %v", err)
	}
	if len(list) != 0 {
		t.Errorf("Expect 0 length, got %d", len(list))
	}
}
