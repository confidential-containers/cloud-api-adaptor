package cdh

import (
	"fmt"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

const (
	ConfigFilePath = "/run/peerpod/cdh.toml"
	Socket         = "unix:///run/confidential-containers/cdh.sock"
)

type Credential struct{}

type Config struct {
	Socket      string       `toml:"socket"`
	KBC         KBCConfig    `toml:"kbc"`
	Credentials []Credential `toml:"credentials"`
}

type KBCConfig struct {
	Name string `toml:"name"`
	URL  string `toml:"url"`
}

func parseAAKBCParams(aaKBCParams string) (*Config, error) {
	parts := strings.SplitN(aaKBCParams, "::", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("Invalid aa-kbs-params input: %s", aaKBCParams)
	}
	name, url := parts[0], parts[1]
	kbcConfig := KBCConfig{name, url}
	return &Config{Socket, kbcConfig, []Credential{}}, nil
}

func CreateConfigFile(aaKBCParams string) (string, error) {
	config, err := parseAAKBCParams(aaKBCParams)
	if err != nil {
		return "", err
	}
	bytes, err := toml.Marshal(config)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}
