// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package util

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
)

// SSHTestKeyPair holds both public and private SSH keys for testing
type SSHTestKeyPair struct {
	PublicKey  ssh.PublicKey
	PrivateKey string // PEM format
}

func generateTestSSHKeyPair() (*SSHTestKeyPair, error) {
	// Generate RSA private key
	privKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, err
	}

	// Generate SSH public key
	pubKey, err := ssh.NewPublicKey(&privKey.PublicKey)
	if err != nil {
		return nil, err
	}

	// Encode private key to PEM
	privateKeyPEM := pem.EncodeToMemory(
		&pem.Block{
			Type:  "RSA PRIVATE KEY",
			Bytes: x509.MarshalPKCS1PrivateKey(privKey),
		},
	)

	return &SSHTestKeyPair{
		PublicKey:  pubKey,
		PrivateKey: string(privateKeyPEM),
	}, nil
}

func TestStatelessTOFUCallback_AcceptsAnyKey(t *testing.T) {
	// Create stateless TOFU callback
	callback := createStatelessTOFUCallback()

	// Generate test key
	keyPair, err := generateTestSSHKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate test key: %v", err)
	}

	// Test with hostname
	hostname := "test-vm"
	remote := &net.TCPAddr{IP: net.ParseIP("192.168.1.100"), Port: 22}

	err = callback(hostname, remote, keyPair.PublicKey)
	if err != nil {
		t.Errorf("Stateless TOFU should accept any key: %v", err)
	}

	// Test multiple connections with different keys (should all succeed)
	differentKeyPair, err := generateTestSSHKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate different test key: %v", err)
	}

	err = callback(hostname, remote, differentKeyPair.PublicKey)
	if err != nil {
		t.Errorf("Stateless TOFU should accept different key: %v", err)
	}
}

func TestStatelessTOFUCallback_IPAddressHandling(t *testing.T) {
	// Create stateless TOFU callback
	callback := createStatelessTOFUCallback()

	// Generate test key
	keyPair, err := generateTestSSHKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate test key: %v", err)
	}
	testKey := keyPair.PublicKey

	// Test with empty hostname (should use IP from remote address)
	hostname := ""
	remote := &net.TCPAddr{IP: net.ParseIP("192.168.1.100"), Port: 22}

	err = callback(hostname, remote, testKey)
	if err != nil {
		t.Errorf("Connection with empty hostname should succeed: %v", err)
	}

	// Test with hostname containing port (should be stripped)
	hostnameWithPort := "test-vm:22"
	err = callback(hostnameWithPort, remote, testKey)
	if err != nil {
		t.Errorf("Connection with hostname containing port should succeed: %v", err)
	}
}

func TestCreateSSHClientConfig_StatelessTOFU(t *testing.T) {
	// Generate test key pair
	keyPair, err := generateTestSSHKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate SSH key pair: %v", err)
	}

	config := &SSHConfig{
		PrivateKey: keyPair.PrivateKey,
		Username:   "testuser",
		Timeout:    30 * time.Second,
		EnableSFTP: true,
	}

	clientConfig, err := CreateSSHClient(config)
	if err != nil {
		t.Fatalf("Failed to create SSH client config: %v", err)
	}

	if clientConfig.HostKeyCallback == nil {
		t.Errorf("HostKeyCallback should not be nil with stateless TOFU")
	}

	if clientConfig.User != "testuser" {
		t.Errorf("Expected username 'testuser', got '%s'", clientConfig.User)
	}

	if clientConfig.Timeout != 30*time.Second {
		t.Errorf("Expected timeout 30s, got %v", clientConfig.Timeout)
	}
}

func TestCreateSSHClientConfig_AllowlistMode(t *testing.T) {
	// Create temporary directory for allowlist
	tempDir, err := os.MkdirTemp("", "ssh_allowlist_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Generate test key pair
	keyPair, err := generateTestSSHKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate SSH key pair: %v", err)
	}
	privKey := keyPair.PrivateKey
	if err != nil {
		t.Fatalf("Failed to generate SSH key pair: %v", err)
	}

	// Create a test public key file
	testKeyPair, err := generateTestSSHKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate test public key: %v", err)
	}
	testPubKey := testKeyPair.PublicKey

	pubKeyBytes := ssh.MarshalAuthorizedKey(testPubKey)
	pubKeyPath := filepath.Join(tempDir, "test_host.pub")
	if err := os.WriteFile(pubKeyPath, pubKeyBytes, 0644); err != nil {
		t.Fatalf("Failed to write test public key file: %v", err)
	}

	config := &SSHConfig{
		PrivateKey:          privKey,
		Username:            "testuser",
		Timeout:             30 * time.Second,
		HostKeyAllowlistDir: tempDir,
		EnableSFTP:          true,
	}

	clientConfig, err := CreateSSHClient(config)
	if err != nil {
		t.Fatalf("Failed to create SSH client config with allowlist: %v", err)
	}

	if clientConfig.HostKeyCallback == nil {
		t.Errorf("HostKeyCallback should not be nil with allowlist mode")
	}
}

func TestCreateSSHClientConfig_AllowlistModeInvalidDir(t *testing.T) {
	// Generate test key pair
	keyPair, err := generateTestSSHKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate SSH key pair: %v", err)
	}
	privKey := keyPair.PrivateKey
	if err != nil {
		t.Fatalf("Failed to generate SSH key pair: %v", err)
	}

	config := &SSHConfig{
		PrivateKey:          privKey,
		Username:            "testuser",
		Timeout:             30 * time.Second,
		HostKeyAllowlistDir: "/nonexistent/directory",
		EnableSFTP:          true,
	}

	_, err = CreateSSHClient(config)
	if err == nil {
		t.Errorf("Expected error for nonexistent allowlist directory")
	}
}

func TestAllowlistCallback_AcceptsAllowlistedKey(t *testing.T) {
	// Create temporary directory for allowlist
	tempDir, err := os.MkdirTemp("", "ssh_allowlist_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Generate test key
	keyPair, err := generateTestSSHKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate test key: %v", err)
	}
	testKey := keyPair.PublicKey

	// Write test key to allowlist directory
	pubKeyBytes := ssh.MarshalAuthorizedKey(testKey)
	pubKeyPath := filepath.Join(tempDir, "allowed_host.pub")
	if err := os.WriteFile(pubKeyPath, pubKeyBytes, 0644); err != nil {
		t.Fatalf("Failed to write test public key file: %v", err)
	}

	// Create allowlist callback
	callback, err := createAllowlistCallback(tempDir)
	if err != nil {
		t.Fatalf("Failed to create allowlist callback: %v", err)
	}

	// Test with allowlisted key
	hostname := "test-vm"
	remote := &net.TCPAddr{IP: net.ParseIP("192.168.1.100"), Port: 22}

	err = callback(hostname, remote, testKey)
	if err != nil {
		t.Errorf("Allowlisted key should be accepted: %v", err)
	}
}

func TestAllowlistCallback_RejectsNonAllowlistedKey(t *testing.T) {
	// Create temporary directory for allowlist
	tempDir, err := os.MkdirTemp("", "ssh_allowlist_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Generate and write one key to allowlist
	keyPair, err := generateTestSSHKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate allowed key: %v", err)
	}
	allowedKey := keyPair.PublicKey

	pubKeyBytes := ssh.MarshalAuthorizedKey(allowedKey)
	pubKeyPath := filepath.Join(tempDir, "allowed_host.pub")
	if err := os.WriteFile(pubKeyPath, pubKeyBytes, 0644); err != nil {
		t.Fatalf("Failed to write allowed public key file: %v", err)
	}

	// Generate a different key (not in allowlist)
	testKeyPair, err := generateTestSSHKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate test key: %v", err)
	}
	testKey := testKeyPair.PublicKey

	// Create allowlist callback
	callback, err := createAllowlistCallback(tempDir)
	if err != nil {
		t.Fatalf("Failed to create allowlist callback: %v", err)
	}

	// Test with non-allowlisted key
	hostname := "test-vm"
	remote := &net.TCPAddr{IP: net.ParseIP("192.168.1.100"), Port: 22}

	err = callback(hostname, remote, testKey)
	if err == nil {
		t.Errorf("Non-allowlisted key should be rejected")
	}

	// Verify error message mentions allowlist
	expectedSubstring := "not in allowlist"
	if err != nil && !contains(err.Error(), expectedSubstring) {
		t.Errorf("Error message should mention allowlist, got: %v", err)
	}
}

func TestLoadAllowedKeys_EmptyDirectory(t *testing.T) {
	// Create temporary empty directory
	tempDir, err := os.MkdirTemp("", "ssh_allowlist_empty_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	_, err = loadAllowedKeys(tempDir)
	if err == nil {
		t.Errorf("Expected error for empty allowlist directory")
	}

	expectedSubstring := "no valid SSH public key files found"
	if !contains(err.Error(), expectedSubstring) {
		t.Errorf("Error message should mention no valid files, got: %v", err)
	}
}

func TestLoadAllowedKeys_NonExistentDirectory(t *testing.T) {
	_, err := loadAllowedKeys("/nonexistent/directory")
	if err == nil {
		t.Errorf("Expected error for nonexistent directory")
	}

	expectedSubstring := "allowlist directory does not exist"
	if !contains(err.Error(), expectedSubstring) {
		t.Errorf("Error message should mention directory does not exist, got: %v", err)
	}
}

func TestLoadAllowedKeys_FileSizeLimit(t *testing.T) {
	// Create temporary directory
	tempDir, err := os.MkdirTemp("", "ssh_allowlist_size_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a file that's too large
	largeContent := make([]byte, MaxKeyFileSize+1)
	for i := range largeContent {
		largeContent[i] = 'A'
	}

	largePath := filepath.Join(tempDir, "large.pub")
	if err := os.WriteFile(largePath, largeContent, 0644); err != nil {
		t.Fatalf("Failed to write large file: %v", err)
	}

	// Create a valid small key file
	keyPair, err := generateTestSSHKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate test key: %v", err)
	}
	testKey := keyPair.PublicKey
	if err != nil {
		t.Fatalf("Failed to generate test key: %v", err)
	}
	pubKeyBytes := ssh.MarshalAuthorizedKey(testKey)
	validPath := filepath.Join(tempDir, "valid.pub")
	if err := os.WriteFile(validPath, pubKeyBytes, 0644); err != nil {
		t.Fatalf("Failed to write valid key file: %v", err)
	}

	// Load keys - should skip large file but load valid file
	allowedKeys, err := loadAllowedKeys(tempDir)
	if err != nil {
		t.Fatalf("Should successfully load valid keys despite large file: %v", err)
	}

	if len(allowedKeys) != 1 {
		t.Errorf("Expected 1 key loaded (large file should be skipped), got %d", len(allowedKeys))
	}
}

func TestLoadAllowedKeys_DuplicateFingerprints(t *testing.T) {
	// Create temporary directory
	tempDir, err := os.MkdirTemp("", "ssh_allowlist_dup_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Generate one test key
	keyPair, err := generateTestSSHKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate test key: %v", err)
	}
	testKey := keyPair.PublicKey
	if err != nil {
		t.Fatalf("Failed to generate test key: %v", err)
	}
	pubKeyBytes := ssh.MarshalAuthorizedKey(testKey)

	// Write the same key to two different files
	file1 := filepath.Join(tempDir, "key1.pub")
	file2 := filepath.Join(tempDir, "key2.pub")

	if err := os.WriteFile(file1, pubKeyBytes, 0644); err != nil {
		t.Fatalf("Failed to write first key file: %v", err)
	}
	if err := os.WriteFile(file2, pubKeyBytes, 0644); err != nil {
		t.Fatalf("Failed to write second key file: %v", err)
	}

	// Load keys - should only have one unique key
	allowedKeys, err := loadAllowedKeys(tempDir)
	if err != nil {
		t.Fatalf("Should successfully load keys despite duplicates: %v", err)
	}

	if len(allowedKeys) != 1 {
		t.Errorf("Expected 1 unique key (duplicates should be skipped), got %d", len(allowedKeys))
	}
}

func TestLoadAllowedKeys_HostKeyFormat(t *testing.T) {
	// Create temporary directory
	tempDir, err := os.MkdirTemp("", "ssh_allowlist_format_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Test cases for different host key formats
	testCases := []struct {
		name        string
		keyContent  string
		shouldWork  bool
		description string
	}{
		{
			name:        "valid_rsa_key",
			keyContent:  "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQC7vK8qFi3L9hGJFZq8... root@vm1",
			shouldWork:  true,
			description: "Standard SSH RSA public key format",
		},
		{
			name:        "valid_ed25519_key",
			keyContent:  "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIG4rT3vTt99Ox5wjZX1J... user@host",
			shouldWork:  true,
			description: "Standard SSH Ed25519 public key format",
		},
		{
			name:        "valid_ecdsa_key",
			keyContent:  "ecdsa-sha2-nistp256 AAAAE2VjZHNhLXNoYTItbmlzdHAyNTY... user@host",
			shouldWork:  true,
			description: "Standard SSH ECDSA public key format",
		},
		{
			name:        "key_without_comment",
			keyContent:  "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQC7vK8qFi3L9hGJFZq8...",
			shouldWork:  true,
			description: "SSH public key without comment",
		},
		{
			name:        "invalid_known_hosts_format",
			keyContent:  "192.168.1.100 ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQC7vK8...",
			shouldWork:  false,
			description: "Known_hosts format (with IP prefix) should fail",
		},
		{
			name:        "invalid_malformed_key",
			keyContent:  "ssh-rsa invalid-base64-content",
			shouldWork:  false,
			description: "Malformed key should fail",
		},
		{
			name:        "empty_file",
			keyContent:  "",
			shouldWork:  false,
			description: "Empty file should fail",
		},
	}

	// Generate a real SSH key for valid test cases
	keyPair, err := generateTestSSHKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate test SSH key: %v", err)
	}
	realKey := keyPair.PublicKey
	realKeyBytes := ssh.MarshalAuthorizedKey(realKey)
	realKeyString := strings.TrimSpace(string(realKeyBytes))

	// Update test cases with real key content for valid cases
	for i := range testCases {
		if testCases[i].shouldWork && !strings.Contains(testCases[i].keyContent, "192.168.1.100") {
			// Replace placeholder with real key content, preserving the comment format
			parts := strings.Fields(testCases[i].keyContent)
			if len(parts) >= 3 {
				// Keep the comment if present
				testCases[i].keyContent = realKeyString + " " + strings.Join(parts[2:], " ")
			} else {
				testCases[i].keyContent = realKeyString
			}
		}
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create test file
			testFilePath := filepath.Join(tempDir, tc.name+".pub")
			if err := os.WriteFile(testFilePath, []byte(tc.keyContent), 0644); err != nil {
				t.Fatalf("Failed to write test file: %v", err)
			}

			// Test loadAllowedKeys
			allowedKeys, err := loadAllowedKeys(tempDir)

			if tc.shouldWork {
				if err != nil {
					t.Errorf("Expected success for %s (%s), but got error: %v", tc.name, tc.description, err)
				} else if len(allowedKeys) != 1 {
					t.Errorf("Expected 1 key loaded for %s (%s), got %d", tc.name, tc.description, len(allowedKeys))
				} else {
					// Verify fingerprint is properly calculated
					for fingerprint := range allowedKeys {
						if !strings.HasPrefix(fingerprint, "SHA256:") {
							t.Errorf("Expected SHA256 fingerprint for %s (%s), got: %s", tc.name, tc.description, fingerprint)
						}
						if len(fingerprint) < 10 { // SHA256: + some content
							t.Errorf("Fingerprint too short for %s (%s): %s", tc.name, tc.description, fingerprint)
						}
					}
				}
			} else {
				if err == nil {
					t.Errorf("Expected error for %s (%s), but succeeded", tc.name, tc.description)
				}
			}

			// Clean up test file for next iteration
			os.Remove(testFilePath)
		})
	}
}

func TestLoadAllowedKeys_FingerprintCalculation(t *testing.T) {
	// Create temporary directory
	tempDir, err := os.MkdirTemp("", "ssh_allowlist_fingerprint_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Generate test key
	keyPair, err := generateTestSSHKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate test key: %v", err)
	}
	testKey := keyPair.PublicKey

	// Write key in standard format
	pubKeyBytes := ssh.MarshalAuthorizedKey(testKey)
	keyPath := filepath.Join(tempDir, "test.pub")
	if err := os.WriteFile(keyPath, pubKeyBytes, 0644); err != nil {
		t.Fatalf("Failed to write test key file: %v", err)
	}

	// Load keys and get fingerprint
	allowedKeys, err := loadAllowedKeys(tempDir)
	if err != nil {
		t.Fatalf("Failed to load allowed keys: %v", err)
	}

	if len(allowedKeys) != 1 {
		t.Fatalf("Expected 1 key, got %d", len(allowedKeys))
	}

	// Get the fingerprint from our function
	var loadedFingerprint string
	for fingerprint := range allowedKeys {
		loadedFingerprint = fingerprint
		break
	}

	// Calculate expected fingerprint directly
	expectedFingerprint := ssh.FingerprintSHA256(testKey)

	// Verify they match
	if loadedFingerprint != expectedFingerprint {
		t.Errorf("Fingerprint mismatch. Expected: %s, Got: %s", expectedFingerprint, loadedFingerprint)
	}

	// Verify fingerprint format
	if !strings.HasPrefix(loadedFingerprint, "SHA256:") {
		t.Errorf("Fingerprint should start with 'SHA256:', got: %s", loadedFingerprint)
	}

	// Verify base64 content after SHA256:
	base64Part := strings.TrimPrefix(loadedFingerprint, "SHA256:")
	if len(base64Part) != 43 { // SHA256 hash in base64 is 43 characters
		t.Errorf("Expected 43 character base64 hash, got %d characters: %s", len(base64Part), base64Part)
	}
}

// Helper function to check if string contains substring
func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
