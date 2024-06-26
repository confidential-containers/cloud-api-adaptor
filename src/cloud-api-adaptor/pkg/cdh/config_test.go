package cdh

import (
	"fmt"
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

	config, err := parseAAKBCParams("cc_kbc::http://1.2.3.4:8080", "")
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

	config, err := parseAAKBCParams("cc_kbc::http://1.2.3.4:8080", "testcert")
	if err != nil {
		t.Error(err)
	}

	if config.KBC.Name != refcfg.KBC.Name {
		t.Errorf("Expected %s, got %s", refcfg.KBC.Name, config.KBC.Name)
	}
	if config.KBC.URL != refcfg.KBC.URL {
		t.Errorf("Expected %s, got %s", refcfg.KBC.URL, config.KBC.URL)
	}
	if config.KBC.KBSCert != refcfg.KBC.KBSCert {
		t.Errorf("Expected %s, got %s", refcfg.KBC.KBSCert, config.KBC.KBSCert)
	}
	if config.Socket != refcfg.Socket {
		t.Errorf("Expected %s, got %s", refcfg.Socket, config.Socket)
	}
	if len(config.Credentials) != 0 {
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
kbs_cert = "sdf"
`
	refdoc = fmt.Sprintf(refdoc, Socket)
	var refcfg Config
	err := toml.Unmarshal([]byte(refdoc), &refcfg)
	if err != nil {
		panic(err)
	}

	config, err := parseAAKBCParams("abc_kbc::http://1.2.3.4:8080", "testcert")
	if err != nil {
		t.Error(err)
	}

	if config.KBC.Name != refcfg.KBC.Name {
		t.Errorf("Expected %s, got %s", refcfg.KBC.Name, config.KBC.Name)
	}
	if config.KBC.URL != refcfg.KBC.URL {
		t.Errorf("Expected %s, got %s", refcfg.KBC.URL, config.KBC.URL)
	}
	if config.KBC.KBSCert != "" {
		t.Errorf("Expected %s, got %s", "", config.KBC.KBSCert)
	}
	if config.Socket != refcfg.Socket {
		t.Errorf("Expected %s, got %s", refcfg.Socket, config.Socket)
	}
	if len(config.Credentials) != 0 {
		t.Errorf("Expected empty credentials array")
	}
}
