package sshutil

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/pem"
	"testing"
)

func TestRsaPrivateKeyPEM(t *testing.T) {
	pKey, err := rsa.GenerateKey(rand.Reader, 3072)
	if err != nil {
		t.Error("failed to generate key")
	}
	pKeyBytes := RsaPrivateKeyPEM(pKey)
	p, _ := pem.Decode(pKeyBytes)
	if p == nil {
		t.Error("Failed to decode Pem")
	}
}
