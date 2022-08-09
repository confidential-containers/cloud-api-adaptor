//go:build ibmcloud

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
		ApiKey:   apiKey,
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

	if cloudCfg.ApiKey != apiKey {
		t.Errorf("Original ApiKey field value has been overwritten")
	}
}
