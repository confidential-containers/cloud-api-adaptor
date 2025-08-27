// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package util

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"strings"

	"golang.org/x/crypto/ssh"
)

// SSHConfig holds SSH key configuration
type SSHConfig struct {
	PublicKey      string
	PrivateKey     string
	PublicKeyPath  string
	PrivateKeyPath string
	Username       string
	EnableSFTP     bool
}

// InitializeSSHKeys sets up SSH keys for authentication
// If SSH keys are provided, use them. Otherwise auto-generate when needed.
func InitializeSSHKeys(config *SSHConfig) error {
	// Step 1: Try to load keys from files
	if err := loadKeysFromFiles(config); err != nil {
		return err
	}

	// Step 2: Check if we have everything we need
	needsGeneration := config.PublicKey == "" || (config.EnableSFTP && config.PrivateKey == "")
	
	if needsGeneration {
		return generateAndSetKeys(config)
	}

	return nil
}

// GenerateSSHKeyPair generates a new RSA SSH key pair
func GenerateSSHKeyPair() (publicKey, privateKey string, err error) {
	// Generate RSA private key
	privKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return "", "", fmt.Errorf("failed to generate private key: %w", err)
	}

	// Encode private key to PEM
	privateKeyPEM := pem.EncodeToMemory(
		&pem.Block{
			Type:  "RSA PRIVATE KEY",
			Bytes: x509.MarshalPKCS1PrivateKey(privKey),
		},
	)

	// Generate SSH public key
	pubKey, err := ssh.NewPublicKey(&privKey.PublicKey)
	if err != nil {
		return "", "", fmt.Errorf("failed to generate public key: %w", err)
	}

	publicKeyBytes := ssh.MarshalAuthorizedKey(pubKey)

	return string(publicKeyBytes), string(privateKeyPEM), nil
}

// ValidateSSHPublicKey validates that the provided string is a valid SSH public key
func ValidateSSHPublicKey(pubKey string) error {
	pubKey = strings.TrimSpace(pubKey)
	if pubKey == "" {
		return fmt.Errorf("public key is empty")
	}

	// Try to parse the public key
	_, _, _, _, err := ssh.ParseAuthorizedKey([]byte(pubKey))
	if err != nil {
		return fmt.Errorf("failed to parse SSH public key: %w", err)
	}

	return nil
}

// ValidateSSHPrivateKey validates that the provided string is a valid SSH private key
func ValidateSSHPrivateKey(privKey string) error {
	privKey = strings.TrimSpace(privKey)
	if privKey == "" {
		return fmt.Errorf("private key is empty")
	}

	// Try to parse the private key
	_, err := ssh.ParsePrivateKey([]byte(privKey))
	if err != nil {
		return fmt.Errorf("failed to parse SSH private key: %w", err)
	}

	return nil
}

// ReadAndValidatePublicKey reads and validates a public key file
func ReadAndValidatePublicKey(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	
	pubKey := strings.TrimSpace(string(data))
	if err := ValidateSSHPublicKey(pubKey); err != nil {
		return "", fmt.Errorf("invalid public key: %w", err)
	}
	
	return pubKey, nil
}

// ReadAndValidatePrivateKey reads and validates a private key file
func ReadAndValidatePrivateKey(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	
	privKey := strings.TrimSpace(string(data))
	if err := ValidateSSHPrivateKey(privKey); err != nil {
		return "", fmt.Errorf("invalid private key: %w", err)
	}
	
	return privKey, nil
}

// loadKeysFromFiles attempts to load SSH keys from configured file paths
func loadKeysFromFiles(config *SSHConfig) error {
	if config.PublicKeyPath == "" {
		return nil
	}

	// Load public key
	pubKey, err := ReadAndValidatePublicKey(config.PublicKeyPath)
	if err != nil {
		// Non-fatal, will auto-generate if needed
		return nil
	}

	config.PublicKey = pubKey

	// Load private key if needed for SFTP
	if config.EnableSFTP && config.PrivateKeyPath != "" {
		privKey, err := ReadAndValidatePrivateKey(config.PrivateKeyPath)
		if err != nil {
			// Non-fatal, will trigger re-generation for matching pair
			return nil
		}
		config.PrivateKey = privKey
	}

	return nil
}

// generateAndSetKeys generates a new SSH key pair and sets it in config
func generateAndSetKeys(config *SSHConfig) error {
	pubKey, privKey, err := GenerateSSHKeyPair()
	if err != nil {
		return fmt.Errorf("failed to generate SSH key pair: %w", err)
	}

	if pubKey == "" || privKey == "" {
		return fmt.Errorf("generated SSH key pair is empty")
	}

	config.PublicKey = pubKey
	if config.EnableSFTP {
		config.PrivateKey = privKey
	}
	
	return nil
}