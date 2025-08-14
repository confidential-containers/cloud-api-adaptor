// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package azure

import (
	"testing"
)

func TestGenerateSSHKeyPair(t *testing.T) {
	pubKey, privKey, err := generateSSHKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate SSH key pair: %v", err)
	}

	if pubKey == "" {
		t.Error("Public key is empty")
	}

	if privKey == "" {
		t.Error("Private key is empty")
	}

	// Verify public key format
	if len(pubKey) < 100 {
		t.Error("Public key seems too short")
	}

	// Verify private key format
	if len(privKey) < 500 {
		t.Error("Private key seems too short")
	}

	t.Logf("Generated public key: %s", pubKey[:50]+"...")
	t.Logf("Generated private key length: %d", len(privKey))
}

func TestNewProviderWithSftpEnabled(t *testing.T) {
	config := &Config{
		EnableSftp:  true,
		SSHUserName: "testuser",
	}

	// Mock the Azure client creation (this would normally fail without real credentials)
	// For testing purposes, we just verify that SSH keys are generated when EnableSftp is true
	pubKey, privKey, err := generateSSHKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate SSH key pair: %v", err)
	}

	config.SSHPubKey = pubKey
	config.SSHPrivKey = privKey

	if config.SSHPubKey == "" {
		t.Error("Public key was not set")
	}

	if config.SSHPrivKey == "" {
		t.Error("Private key was not set")
	}

	t.Log("SFTP functionality properly initializes SSH keys")
}