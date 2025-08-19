package sshutil

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"log"
)

const SSHPORT = "2222"
const PpSecureCommsVersion = "v0.2"
const KBS = "KBS"
const KBSClientSecret = "kbs-client"
const AdaptorSSHSecret = "sshclient"

var Logger = log.New(log.Writer(), "[secure-comms] ", log.LstdFlags|log.Lmsgprefix)

// RsaPrivateKeyPEM return a PEM for the RSA Private Key
func RsaPrivateKeyPEM(pKey *rsa.PrivateKey) []byte {
	return pem.EncodeToMemory(&pem.Block{
		Type:    "RSA PRIVATE KEY",
		Headers: nil,
		Bytes:   x509.MarshalPKCS1PrivateKey(pKey),
	})
}
