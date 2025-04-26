package initdata

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	toml "github.com/pelletier/go-toml/v2"
	"io"
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
		return "", fmt.Errorf("Error creating initdata digest, algorithm %s is not supported", alg)
	}
}

func Parse(reader io.Reader) (*InitData, error) {
	b64Reader := base64.NewDecoder(base64.StdEncoding, reader)
	gzipReader, err := gzip.NewReader(b64Reader)
	if err != nil {
		return nil, err
	}

	initdataToml, err := io.ReadAll(gzipReader)
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
