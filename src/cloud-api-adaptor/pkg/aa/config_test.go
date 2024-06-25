package aa

import (
	"testing"
)

func Test_parseAAKBCParams(t *testing.T) {
	url, err := parseAAKBCParams("cc_kbc::http://127.0.0.1:8080")
	if err != nil {
		t.Error(err)
	}

	expected := "http://127.0.0.1:8080"
	if url != expected {
		t.Errorf("Expected %s, got %s", expected, url)
	}
}

func TestConfigFile(t *testing.T) {
	refcfg := `[token_configs]
[token_configs.coco_as]
url = 'http://127.0.0.1:8080'

[token_configs.kbs]
url = 'http://127.0.0.1:8080'
`

	config, err := CreateConfigFile("cc_kbc::http://127.0.0.1:8080")
	if err != nil {
		t.Error(err)
	}

	if config != refcfg {
		t.Errorf("Expected: \n%s, got: \n%s", refcfg, config)
	}
}
