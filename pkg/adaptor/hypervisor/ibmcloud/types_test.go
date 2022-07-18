package ibmcloud

import (
	"fmt"
	"strings"
	"testing"
)

func TestMasking(t *testing.T) {
	apiKey := "abcdefg"
	zoneName := "eu-gb"
	cloudCfg := Config{
		ApiKey:   apiKey,
		ZoneName: zoneName,
	}
	checkLine := func(verb string) {
		logline := fmt.Sprintf(verb, &cloudCfg)
		if strings.Contains(logline, apiKey) {
			t.Errorf("For verb %s: %s contains the api key: %s", verb, logline, apiKey)
		}
		if !strings.Contains(logline, zoneName) {
			t.Errorf("For verb %s: %s doesn't contain the zone name: %s", verb, logline, zoneName)
		}
	}
	checkLine("%v")
	checkLine("%s")
	checkLine("%q")
}
