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

// Test instanceTypes
func TestInstanceTypes(t *testing.T) {
	var list instanceTypes
	err := list.Set("t2.micro,t2.small")
	if err != nil {
		t.Errorf("List Set failed, %v", err)
	}
	if len(list) != 2 {
		t.Errorf("Expect 2 length, got %d", len(list))
	}
	if list.String() != "t2.micro, t2.small" {
		t.Errorf("Expect 't2.micro, t2.small', got %s", list.String())
	}
}

// Test empty instanceTypes
func TestEmptyInstanceTypes(t *testing.T) {
	var list instanceTypes
	err := list.Set("")
	if err != nil {
		t.Errorf("List Set failed, %v", err)
	}
	if len(list) != 0 {
		t.Errorf("Expect 0 length, got %d", len(list))
	}
	if list.String() != "" {
		t.Errorf("Expect '', got %s", list.String())
	}
}

// Test securityGroupIds
func TestSecurityGroupIds(t *testing.T) {
	var list securityGroupIds
	err := list.Set("sg-1234,sg-5678")
	if err != nil {
		t.Errorf("List Set failed, %v", err)
	}
	if len(list) != 2 {
		t.Errorf("Expect 2 length, got %d", len(list))
	}
	if list.String() != "sg-1234, sg-5678" {
		t.Errorf("Expect 'sg-1234, sg-5678', got %s", list.String())
	}
}

// Test empty securityGroupIds
func TestEmptySecurityGroupIds(t *testing.T) {
	var list securityGroupIds
	err := list.Set("")
	if err != nil {
		t.Errorf("List Set failed, %v", err)
	}

	if list.String() != "" {
		t.Errorf("Expect '', got %s", list.String())
	}
}
