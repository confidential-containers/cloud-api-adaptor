// Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0
// This code is adapted from https://github.com/kubernetes/client-go/blob/kubernetes-1.22.17/transport/transport.go
package tlsutil

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
)

// TLSConfig holds the information needed to set up a TLS transport.
type TLSConfig struct {
	CAFile     string // Path of the PEM-encoded server trusted root certificates.
	CertFile   string // Path of the PEM-encoded client certificate.
	KeyFile    string // Path of the PEM-encoded client key.
	SkipVerify bool   // Server should be accessed without verifying the certificate. For testing only.

	CAData   []byte // Bytes of the PEM-encoded server trusted root certificates. Supercedes CAFile.
	CertData []byte // Bytes of the PEM-encoded client certificate. Supercedes CertFile.
	KeyData  []byte // Bytes of the PEM-encoded client key. Supercedes KeyFile.
}

// HasCA returns whether the configuration has a certificate authority or not.
func (t *TLSConfig) HasCA() bool {
	return len(t.CAData) > 0 || len(t.CAFile) > 0
}

// HasCertAuth returns whether the configuration has certificate authentication or not.
func (t *TLSConfig) HasCertAuth() bool {
	return (len(t.CertData) != 0 || len(t.CertFile) != 0) && (len(t.KeyData) != 0 || len(t.KeyFile) != 0)
}

// loadTLSFiles copies the data from the CertFile, KeyFile, and CAFile fields into the CertData,
// KeyData, and CAFile fields, or returns an error. If no error is returned, all three fields are
// either populated or were empty to start.
func loadTLSFiles(t *TLSConfig) error {
	var err error
	t.CAData, err = dataFromSliceOrFile(t.CAData, t.CAFile)
	if err != nil {
		return err
	}

	t.CertData, err = dataFromSliceOrFile(t.CertData, t.CertFile)
	if err != nil {
		return err
	}

	t.KeyData, err = dataFromSliceOrFile(t.KeyData, t.KeyFile)
	if err != nil {
		return err
	}
	return nil
}

// dataFromSliceOrFile returns data from the slice (if non-empty), or from the file,
// or an error if an error occurred reading the file
func dataFromSliceOrFile(data []byte, file string) ([]byte, error) {
	if len(data) > 0 {
		return data, nil
	}
	if len(file) > 0 {
		fileData, err := os.ReadFile(file)
		if err != nil {
			return []byte{}, err
		}
		return fileData, nil
	}
	return nil, nil
}

// rootCertPool returns nil if caData is empty.  When passed along, this will mean "use system CAs".
// When caData is not empty, it will be the ONLY information used in the CertPool.
func rootCertPool(caData []byte) (*x509.CertPool, error) {
	// What we really want is a copy of x509.systemRootsPool, but that isn't exposed.  It's difficult to build (see the go
	// code for a look at the platform specific insanity), so we'll use the fact that RootCAs == nil gives us the system values
	// It doesn't allow trusting either/or, but hopefully that won't be an issue
	if len(caData) == 0 {
		return nil, nil
	}

	// if we have caData, use it
	certPool := x509.NewCertPool()
	if ok := certPool.AppendCertsFromPEM(caData); !ok {
		return nil, createErrorParsingCAData(caData)
	}
	return certPool, nil
}

// createErrorParsingCAData ALWAYS returns an error.  We call it because know we failed to AppendCertsFromPEM
// but we don't know the specific error because that API is just true/false
func createErrorParsingCAData(pemCerts []byte) error {
	for len(pemCerts) > 0 {
		var block *pem.Block
		block, pemCerts = pem.Decode(pemCerts)
		if block == nil {
			return fmt.Errorf("unable to parse bytes as PEM block")
		}

		if block.Type != "CERTIFICATE" || len(block.Headers) != 0 {
			continue
		}

		if _, err := x509.ParseCertificate(block.Bytes); err != nil {
			return fmt.Errorf("failed to parse certificate: %w", err)
		}
	}
	return fmt.Errorf("no valid certificate authority data seen")
}

// GetTLSConfigFor returns a tls.Config that will provide the transport level security defined
// by the provided Config. Will return nil if no transport level security is requested.
func GetTLSConfigFor(t *TLSConfig) (*tls.Config, error) {
	if !t.HasCA() && !t.HasCertAuth() && !t.SkipVerify {
		return nil, nil
	}
	if t.HasCA() && t.SkipVerify {
		return nil, fmt.Errorf("specifying a root certificates file with the insecure flag is not allowed")
	}
	if err := loadTLSFiles(t); err != nil {
		return nil, err
	}

	tlsConfig := &tls.Config{
		// Can't use SSLv3 because of POODLE and BEAST
		// Can't use TLSv1.0 because of POODLE and BEAST using CBC cipher
		// Can't use TLSv1.1 because of RC4 cipher usage
		MinVersion:         tls.VersionTLS12,
		InsecureSkipVerify: t.SkipVerify,
	}

	if t.HasCA() {
		rootCAs, err := rootCertPool(t.CAData)
		if err != nil {
			return nil, fmt.Errorf("unable to load root certificates: %w", err)
		}
		tlsConfig.RootCAs = rootCAs

		// Enable mutual authentication
		tlsConfig.ClientAuth = tls.RequireAndVerifyClientCert
		tlsConfig.ClientCAs = rootCAs
	}

	if t.HasCertAuth() {
		// If key/cert were provided, verify them before setting up
		// tlsConfig.GetClientCertificate.
		cert, err := tls.X509KeyPair(t.CertData, t.KeyData)
		if err != nil {
			return nil, err
		}
		tlsConfig.Certificates = []tls.Certificate{cert}

	}

	return tlsConfig, nil
}
