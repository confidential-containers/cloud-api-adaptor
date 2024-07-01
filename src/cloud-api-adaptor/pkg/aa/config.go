package aa

import (
	"fmt"
	"strings"

	toml "github.com/pelletier/go-toml/v2"
)

const (
	DefaultAaConfigPath = "/run/peerpod/aa.toml"
)

type AAConfig struct {
	TokenCfg struct {
		CocoAs struct {
			URL string `toml:"url"`
		} `toml:"coco_as"`
		Kbs struct {
			URL  string `toml:"url"`
			CERT string `toml:"cert,omitempty"`
		} `toml:"kbs"`
	} `toml:"token_configs"`
}

func parseAAKBCParams(aaKBCParams string) (string, string, error) {
	parts := strings.SplitN(aaKBCParams, "::", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("Invalid aa-kbs-params input: %s", aaKBCParams)
	}
	name, url := parts[0], parts[1]
	return name, url, nil
}

func CreateConfigFile(aaKBCParams, kbsCert string) (string, error) {
	name, url, err := parseAAKBCParams(aaKBCParams)
	config := AAConfig{}
	if err != nil {
		return "", err
	}
	if kbsCert != "" && name == "cc_kbc" {
		config.TokenCfg.Kbs.CERT = kbsCert
	}

	// Assume KBS and AS has same endpoint
	// Need a new parameter in addition to aaKBCParams if deploy AS and KBS separately.
	config.TokenCfg.CocoAs.URL = url
	config.TokenCfg.Kbs.URL = url

	bytes, err := toml.Marshal(config)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}
