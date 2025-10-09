// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package azure

import (
	"fmt"
	"strings"
	"testing"

	"golang.org/x/crypto/ssh"
)

func TestAzureMasking(t *testing.T) {
	toBeRedacted := map[string]string{
		"client id":     "a client id",
		"tenant id":     "a tenant id",
		"client secret": "a client secret",
	}
	region := "a region"

	cloudCfg := Config{
		ClientId:     toBeRedacted["client id"],
		TenantId:     toBeRedacted["tenant id"],
		ClientSecret: toBeRedacted["client secret"],
		Region:       region,
	}

	checkLine := func(verb string) {
		logline := fmt.Sprintf(verb, cloudCfg.Redact())
		for k, v := range toBeRedacted {
			if strings.Contains(logline, v) {
				t.Errorf("For verb %s: %s contains the %s: %s", verb, logline, k, v)
			}
		}
		if !strings.Contains(logline, region) {
			t.Errorf("For verb %s: %s doesn't contain the region name: %s", verb, logline, region)
		}
	}

	checkLine("%v")
	checkLine("%s")
}

func TestGenerateSSHKeyPair(t *testing.T) {
	publicKeyBytes, err := generateSSHPublicKey()
	if err != nil {
		t.Fatalf("Failed to generate SSH key pair: %v", err)
	}

	if len(publicKeyBytes) == 0 {
		t.Error("Generated public key bytes are empty")
	}

	_, _, _, _, err = ssh.ParseAuthorizedKey(publicKeyBytes)
	if err != nil {
		t.Errorf("Failed to parse generated public key: %v", err)
	}
}
