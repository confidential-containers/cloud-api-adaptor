package initdata

type InitData struct {
	Algorithm string            `toml:"algorithm"`
	Version   string            `toml:"version"`
	Data      map[string]string `toml:"data,omitempty"`
}
