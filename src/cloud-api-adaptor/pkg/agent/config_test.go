package agent

import (
	"testing"

	"github.com/pelletier/go-toml/v2"
)

func TestAgentConfigFile(t *testing.T) {
	refdoc := `server_addr = 'unix:///run/kata-containers/agent.sock'
guest_components_procs = 'none'
`
	var refcfg agentConfig
	err := toml.Unmarshal([]byte(refdoc), &refcfg)
	if err != nil {
		panic(err)
	}

	if refcfg.ServerAddr != ServerAddr {
		t.Errorf("Expected %s, got %s", ServerAddr, refcfg.ServerAddr)
	}
	if refcfg.GuestComponentsProcs != GuestComponentsProcs {
		t.Errorf("Expected %s, got %s", GuestComponentsProcs, refcfg.GuestComponentsProcs)
	}

	configstr, err := CreateConfigFile("")
	if err != nil {
		panic(err)
	}
	if refdoc != string(configstr) {
		t.Errorf("Expected %s, got %s", refdoc, configstr)
	}
}

func TestAgentConfigFileWithAuthJsonFile(t *testing.T) {
	refdoc := `server_addr = 'unix:///run/kata-containers/agent.sock'
guest_components_procs = 'none'
image_registry_auth = 'file:///run/peerpod/auth.json'
`
	authJsonFile := "/run/peerpod/auth.json"

	configstr, err := CreateConfigFile(authJsonFile)
	if err != nil {
		panic(err)
	}
	if refdoc != string(configstr) {
		t.Errorf("Expected %s, got %s", refdoc, configstr)
	}

	var config agentConfig
	err = toml.Unmarshal([]byte(configstr), &config)
	if err != nil {
		panic(err)
	}

	if config.ImageRegistryAuth != "file://"+authJsonFile {
		t.Errorf("Expected %s, got %s", config.ImageRegistryAuth, authJsonFile)
	}
}
