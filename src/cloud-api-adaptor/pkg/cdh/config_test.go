package cdh

import (
	"fmt"
	"log"
	"strings"
	"testing"

	"github.com/pelletier/go-toml/v2"
)

func TestCDHConfigFileFromAAKBCParams(t *testing.T) {
	refdoc := `
socket = "%s"
credentials = []
[kbc]
name = "cc_kbc"
url = "http://1.2.3.4:8080"
`
	refdoc = fmt.Sprintf(refdoc, Socket)
	var refcfg Config
	err := toml.Unmarshal([]byte(refdoc), &refcfg)
	if err != nil {
		panic(err)
	}

	config, err := parseAAKBCParams("cc_kbc::http://1.2.3.4:8080")
	if err != nil {
		t.Error(err)
	}

	if config.KBC.Name != refcfg.KBC.Name {
		t.Errorf("Expected %s, got %s", refcfg.KBC.Name, config.KBC.Name)
	}
	if config.KBC.URL != refcfg.KBC.URL {
		t.Errorf("Expected %s, got %s", refcfg.KBC.URL, config.KBC.URL)
	}
	if config.Socket != refcfg.Socket {
		t.Errorf("Expected %s, got %s", refcfg.Socket, config.Socket)
	}
	if len(config.Credentials) != 0 {
		t.Errorf("Expected empty credentials array")
	}
}

func TestCDHConfigFileFromAAKBCParamsKBSCert(t *testing.T) {
	refdoc := `
socket = "%s"
credentials = []

[kbc]
name = "cc_kbc"
url = "http://1.2.3.4:8080"
kbs_cert = "testcert"
`
	refdoc = fmt.Sprintf(refdoc, Socket)
	var refcfg Config
	err := toml.Unmarshal([]byte(refdoc), &refcfg)
	if err != nil {
		panic(err)
	}

	configStr, err := CreateConfigFile("cc_kbc::http://1.2.3.4:8080", "testcert")
	var cfg Config
	err = toml.Unmarshal([]byte(configStr), &cfg)
	if err != nil {
		panic(err)
	}
	if err != nil {
		t.Error(err)
	}

	if cfg.KBC.URL != refcfg.KBC.URL {
		t.Errorf("Expected %s, got %s", refcfg.KBC.URL, cfg.KBC.URL)
	}
	if cfg.KBC.KBSCert != refcfg.KBC.KBSCert {
		t.Errorf("Expected %s, got %s", refcfg.KBC.KBSCert, cfg.KBC.KBSCert)
	}
	if cfg.Socket != refcfg.Socket {
		t.Errorf("Expected %s, got %s", refcfg.Socket, cfg.Socket)
	}
	if len(cfg.Credentials) != 0 {
		t.Errorf("Expected empty credentials array")
	}
}

func TestCDHConfigFileFromInvalidAAKBCParamsKBSCert(t *testing.T) {
	refdoc := `
socket = "%s"
credentials = []
[kbc]
name = "abc_kbc"
url = "http://1.2.3.4:8080"
kbs_cert = "testcert"
`
	refdoc = fmt.Sprintf(refdoc, Socket)
	var refcfg Config
	err := toml.Unmarshal([]byte(refdoc), &refcfg)
	if err != nil {
		panic(err)
	}

	configStr, err := CreateConfigFile("abc_kbc::http://1.2.3.4:8080", "testcert")
	log.Printf("config string %s", configStr)
	if strings.Contains(configStr, "kbs_cert") {
		t.Errorf("kbs_cert is not expected to be included in config file, but got %s", configStr)
	}
	var cfg Config
	err = toml.Unmarshal([]byte(configStr), &cfg)
	if err != nil {
		panic(err)
	}
	if err != nil {
		t.Error(err)
	}

}
