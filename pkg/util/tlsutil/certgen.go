// Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package tlsutil

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"time"
)

// Certificate generation is based on https://github.com/golang/go/blob/master/src/crypto/tls/generate_cert.go

// Automatic mutual TLS configuration works as follows
//
// 1. At start up, cloud-api-adaptor generates the following two sets of certificate/key pairs
//    * Self-signed client certificate and its private key
//    * Self-signed server CA certificate and its private key
// 2. Before creating a peer pod VM, cloud-api-adaptor generates a pair of server certificate and private key using the server CA certificate.
// 3. When creating a peer pod VM, cloud-api-adaptor sends the generated server certificate/key as well as client certificate to a newly created pod VM via cloud-init data
// 4. agent-protocol-adaptor starts TLS listener using the server cert/key
// 5. cloud-api-adaptor initiates TLS connection to agent-protocol-forwarder using the client cert/key
// 6. agent-protocol-adaptor validates incoming TLS connection using the client certificate
// 7. cloud-api-adaptor validates the server certificate sent from agent-protocol-forwarder using the server CA certificate

const (
	validFor = 2 * 365 * 24 * time.Hour
)

type CAService interface {
	RootCertificate() (certPEM []byte)
	Issue(serverName string) (certPEM, keyPEM []byte, err error)
}

type caService struct {
	orgName string
	certPEM []byte
	keyPEM  []byte
}

func NewCAService(orgName string) (CAService, error) {

	certPEM, keyPEM, err := generateCertificate(orgName, "", nil, nil, false, true)

	if err != nil {
		return nil, fmt.Errorf("failed to set up a CA service for %q", orgName)
	}

	s := &caService{
		orgName: orgName,
		certPEM: certPEM,
		keyPEM:  keyPEM,
	}

	return s, nil
}

func (s *caService) RootCertificate() (certPEM []byte) {
	return s.certPEM
}

// Issue generates a server certificate for serverName and its private key
func (s *caService) Issue(serverName string) (certPEM, keyPEM []byte, err error) {

	serverCertPEM, serverKeyPEM, err := generateCertificate(s.orgName, serverName, s.certPEM, s.keyPEM, false, false)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to issue a server certificate for %q: %w", serverName, err)
	}

	return serverCertPEM, serverKeyPEM, nil
}

// NewClientCertificate generates a self-signed client certificate for orgName and its private key
func NewClientCertificate(orgName string) (certPEM, keyPEM []byte, err error) {

	certPEM, keyPEM, err = generateCertificate(orgName, "", nil, nil, true, false)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate a client certificate for %q", orgName)
	}

	return certPEM, keyPEM, nil
}

func decodePEM(pemBytes []byte) ([]byte, error) {

	firstBlock, remainingBlocks := pem.Decode(pemBytes)
	if firstBlock == nil {
		return nil, errors.New("no PEM data is found")
	} else if len(remainingBlocks) != 0 {
		return nil, errors.New("more than one PEM block is found")
	}

	return firstBlock.Bytes, nil
}

func encodePEM(dataType string, der []byte) ([]byte, error) {

	var buf bytes.Buffer
	if err := pem.Encode(&buf, &pem.Block{Type: dataType, Bytes: der}); err != nil {
		return nil, fmt.Errorf("failed to encode PEM data type %q: %w", dataType, err)
	}

	return buf.Bytes(), nil
}

func generateCertificate(orgName, serverName string, parentCertPEM, parentKeyPEM []byte, isClient, isCA bool) (certPEM, keyPEM []byte, err error) {

	var (
		signerCert, parentCert *x509.Certificate
		signerKey, parentKey   interface{}
	)

	// Load a parent certificate

	if parentCertPEM != nil {
		parentCertDER, err := decodePEM(parentCertPEM)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to decode a parent certificate PEM: %w", err)
		}

		parentCert, err = x509.ParseCertificate(parentCertDER)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to parse a parent certificate: %w", err)
		}
	}

	// Load a parent private key

	if parentKeyPEM != nil {
		parentKeyDER, err := decodePEM(parentKeyPEM)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to decode a parent key PEM: %w", err)
		}

		parentKey, err = x509.ParsePKCS8PrivateKey(parentKeyDER)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to parse a parent key: %w", err)
		}
	}
	// Prepare a certificate template

	notBefore := time.Now().UTC().Add(-5 * time.Minute)
	notAfter := notBefore.Add(validFor)

	if parentCert != nil {
		if notBefore.Before(parentCert.NotBefore) {
			notBefore = parentCert.NotBefore.UTC()
		}
		if parentCert.NotAfter.Before(notAfter) {
			notAfter = parentCert.NotAfter.UTC()
		}
	}

	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate a serial number of a new certificate: %w", err)
	}

	var authType x509.ExtKeyUsage
	if isClient {
		authType = x509.ExtKeyUsageClientAuth
	} else {
		authType = x509.ExtKeyUsageServerAuth
	}

	certTemplate := x509.Certificate{
		SerialNumber:          serialNumber,
		Subject:               pkix.Name{Organization: []string{orgName}},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{authType},
		BasicConstraintsValid: true,
	}

	if serverName != "" {
		certTemplate.Subject.CommonName = serverName
		certTemplate.DNSNames = []string{serverName}
	}

	if isCA {
		certTemplate.IsCA = true
		certTemplate.KeyUsage |= x509.KeyUsageCertSign
	}

	// Generate a private key

	// TODO: Support key algorithms other than ECDSA P-256
	curve := elliptic.P256()
	key, err := ecdsa.GenerateKey(curve, rand.Reader)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate ECDSA key for %s: %w", curve.Params().Name, err)
	}

	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to convert a private key to PKCS #8 form: %w", err)
	}

	keyPEM, err = encodePEM("PRIVATE KEY", keyDER)
	if err != nil {
		return nil, nil, fmt.Errorf("failed ot encode a private key to PEM: %w", err)
	}

	// Create a certificate

	if parentCert != nil {
		signerCert = parentCert
		signerKey = parentKey
	} else {
		// self-signed certificate
		signerCert = &certTemplate
		signerKey = key
	}

	certDER, err := x509.CreateCertificate(rand.Reader, &certTemplate, signerCert, &key.PublicKey, signerKey)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create a certificate: %w", err)
	}

	certPEM, err = encodePEM("CERTIFICATE", certDER)
	if err != nil {
		return nil, nil, fmt.Errorf("failed ot encode a certificate to PEM: %w", err)
	}

	return certPEM, keyPEM, nil
}
