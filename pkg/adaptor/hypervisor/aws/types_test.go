//go:build aws

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
