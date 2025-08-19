package initdata

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"strings"

	toml "github.com/pelletier/go-toml/v2"
)

type InitDataBody struct {
	Algorithm string            `toml:"algorithm"`
	Version   string            `toml:"version"`
	Data      map[string]string `toml:"data,omitempty"`
}

type InitData struct {
	Body   *InitDataBody
	Digest string
}

func digest(alg string, body []byte) (string, error) {
	switch alg {
	case "sha256":
		hash := sha256.Sum256(body)
		return hex.EncodeToString(hash[:]), nil
	case "sha384":
		hash := sha512.Sum384(body)
		return hex.EncodeToString(hash[:]), nil
	case "sha512":
		hash := sha512.Sum512(body)
		return hex.EncodeToString(hash[:]), nil
	default:
		return "", fmt.Errorf("error creating initdata digest, algorithm %s is not supported", alg)
	}
}

func decode(reader io.Reader) ([]byte, error) {
	b64Reader := base64.NewDecoder(base64.StdEncoding, reader)
	gzipReader, err := gzip.NewReader(b64Reader)
	if err != nil {
		return nil, err
	}

	return io.ReadAll(gzipReader)
}

func Parse(reader io.Reader) (*InitData, error) {
	initdataToml, err := decode(reader)
	if err != nil {
		return nil, err
	}

	body := &InitDataBody{}
	err = toml.Unmarshal(initdataToml, body)
	if err != nil {
		return nil, err
	}

	digest, err := digest(body.Algorithm, initdataToml)
	if err != nil {
		return nil, err
	}

	initdata := &InitData{
		Body:   body,
		Digest: digest,
	}

	return initdata, nil
}

func Encode(initdataStr string) (string, error) {
	var val bytes.Buffer
	b64Writer := base64.NewEncoder(base64.StdEncoding, &val)
	gzWriter := gzip.NewWriter(b64Writer)

	if _, err := gzWriter.Write([]byte(initdataStr)); err != nil {
		return "", err
	}

	if err := gzWriter.Close(); err != nil {
		return "", err
	}

	if err := b64Writer.Close(); err != nil {
		return "", err
	}

	return val.String(), nil
}

// Used in e2e testing
func DecodeAnnotation(annotation string) ([]byte, error) {
	reader := strings.NewReader(annotation)
	return decode(reader)
}
