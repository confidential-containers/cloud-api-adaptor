// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package util

import (
	"context"
	"fmt"
	"io/fs"
	"log"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

const (
	// Maximum number of allowed SSH host keys
	MaxAllowedHostKeys = 100
	// Maximum size of a single SSH public key file (in bytes)
	MaxKeyFileSize = 16 * 1024 // 16KB should be more than enough for any SSH key
)

// SSHConfig holds the SSH configuration for connecting to pod VM
type SSHConfig struct {
	// Key content (populated from files or generated)
	PublicKey  string
	PrivateKey string

	// Key file paths (for loading)
	PublicKeyPath  string
	PrivateKeyPath string

	// SSH client configuration
	Username            string
	Timeout             time.Duration
	HostKeyAllowlistDir string

	// Internal settings
	EnableSFTP bool
}

// CreateSSHClient creates a complete SSH client configuration
func CreateSSHClient(config *SSHConfig) (*ssh.ClientConfig, error) {
	// Step 1: Initialize keys if needed
	if err := initializeKeys(config); err != nil {
		return nil, fmt.Errorf("failed to initialize SSH keys: %w", err)
	}

	// Step 2: Create SSH client configuration
	return createClientConfig(config)
}

// initializeKeys sets up SSH keys for authentication (internal)
func initializeKeys(config *SSHConfig) error {
	// Step 1: Try to load keys from files
	if err := loadKeysFromFiles(config); err != nil {
		return err
	}

	return nil
}

// createClientConfig creates SSH client configuration
func createClientConfig(config *SSHConfig) (*ssh.ClientConfig, error) {
	// Parse private key
	signer, err := ssh.ParsePrivateKey([]byte(config.PrivateKey))
	if err != nil {
		return nil, fmt.Errorf("failed to parse private key: %w", err)
	}

	clientConfig := &ssh.ClientConfig{
		User: config.Username,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		Timeout: config.Timeout,
	}

	// Configure host key checking based on mode
	if config.HostKeyAllowlistDir != "" {
		// Allowlist mode: only accept keys from directory
		hostKeyCallback, err := createAllowlistCallback(config.HostKeyAllowlistDir)
		if err != nil {
			return nil, fmt.Errorf("failed to create allowlist host key callback: %w", err)
		}
		clientConfig.HostKeyCallback = hostKeyCallback
	} else {
		// Default: stateless TOFU (accept any key with logging)
		clientConfig.HostKeyCallback = createStatelessTOFUCallback()
	}

	return clientConfig, nil
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
		return fmt.Errorf("failed to load public key from %s: %w", config.PublicKeyPath, err)
	}

	config.PublicKey = pubKey

	// Load private key if needed for SFTP
	if config.EnableSFTP && config.PrivateKeyPath != "" {
		privKey, err := ReadAndValidatePrivateKey(config.PrivateKeyPath)
		if err != nil {
			return fmt.Errorf("failed to load private key from %s: %w", config.PrivateKeyPath, err)

		}
		config.PrivateKey = privKey
	}

	return nil
}

// loadAllowedKeys loads SSH public keys from directory and returns a map of fingerprints
func loadAllowedKeys(allowlistDir string) (map[string]bool, error) {
	allowedKeys := make(map[string]bool)
	keyCount := 0

	// Check if directory exists
	if _, err := os.Stat(allowlistDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("allowlist directory does not exist: %s", allowlistDir)
	}

	// Walk through directory looking for .pub files
	err := filepath.WalkDir(allowlistDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip directories and non-.pub files
		if d.IsDir() || !strings.HasSuffix(d.Name(), ".pub") {
			return nil
		}

		// Check if we've reached the maximum number of keys
		if keyCount >= MaxAllowedHostKeys {
			log.Printf("Warning: Reached maximum allowed host keys (%d), skipping remaining files", MaxAllowedHostKeys)
			return filepath.SkipAll // Stop processing remaining files
		}

		// Check file size to prevent reading extremely large files
		info, err := d.Info()
		if err != nil {
			log.Printf("Warning: Failed to get file info for %s: %v", path, err)
			return nil
		}

		if info.Size() > MaxKeyFileSize {
			log.Printf("Warning: SSH public key file %s is too large (%d bytes, max %d), skipping", path, info.Size(), MaxKeyFileSize)
			return nil
		}

		// Read and parse the public key file
		data, err := os.ReadFile(path)
		if err != nil {
			log.Printf("Warning: Failed to read SSH public key file %s: %v", path, err)
			return nil // Continue processing other files
		}

		// Parse the SSH public key
		pubKey, _, _, _, err := ssh.ParseAuthorizedKey(data)
		if err != nil {
			log.Printf("Warning: Failed to parse SSH public key in file %s: %v", path, err)
			return nil // Continue processing other files
		}

		// Generate fingerprint and add to allowlist
		fingerprint := ssh.FingerprintSHA256(pubKey)
		if _, exists := allowedKeys[fingerprint]; exists {
			log.Printf("Warning: Duplicate SSH host key fingerprint %s found in %s, skipping", fingerprint, path)
			return nil
		}

		allowedKeys[fingerprint] = true
		keyCount++
		log.Printf("SSH: Loaded allowed host key from %s: %s", path, fingerprint)

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to scan allowlist directory: %w", err)
	}

	if len(allowedKeys) == 0 {
		return nil, fmt.Errorf("no valid SSH public key files found in allowlist directory: %s", allowlistDir)
	}

	log.Printf("SSH: Loaded %d allowed host keys from directory %s", len(allowedKeys), allowlistDir)
	return allowedKeys, nil
}

// createAllowlistCallback creates a host key callback that only accepts keys from the allowlist
func createAllowlistCallback(allowlistDir string) (ssh.HostKeyCallback, error) {
	allowedKeys, err := loadAllowedKeys(allowlistDir)
	if err != nil {
		return nil, err
	}

	return func(hostname string, remote net.Addr, key ssh.PublicKey) error {
		// Extract host from address if hostname is empty
		host := hostname
		if host == "" {
			if tcpAddr, ok := remote.(*net.TCPAddr); ok {
				host = tcpAddr.IP.String()
			} else {
				host = remote.String()
			}
		}

		// Remove port from hostname if present
		if strings.Contains(host, ":") {
			host, _, _ = net.SplitHostPort(host)
		}

		keyType := key.Type()
		fingerprint := ssh.FingerprintSHA256(key)

		// Check if the key is in the allowlist
		if _, allowed := allowedKeys[fingerprint]; allowed {
			log.Printf("SSH: Accepted allowlisted key for %s (%s): %s", host, keyType, fingerprint)
			return nil
		}

		// Key is not in allowlist - reject connection
		log.Printf("SSH: Rejected non-allowlisted key for %s (%s): %s", host, keyType, fingerprint)
		return fmt.Errorf("SSH host key not in allowlist for %s: %s", host, fingerprint)
	}, nil
}

// createStatelessTOFUCallback creates a host key callback that accepts any key (stateless TOFU)
func createStatelessTOFUCallback() ssh.HostKeyCallback {
	return func(hostname string, remote net.Addr, key ssh.PublicKey) error {
		// Extract host from address if hostname is empty
		host := hostname
		if host == "" {
			if tcpAddr, ok := remote.(*net.TCPAddr); ok {
				host = tcpAddr.IP.String()
			} else {
				host = remote.String()
			}
		}

		// Remove port from hostname if present
		if strings.Contains(host, ":") {
			host, _, _ = net.SplitHostPort(host)
		}

		keyType := key.Type()
		keyFingerprint := ssh.FingerprintSHA256(key)

		// Log the connection with key information for security monitoring
		log.Printf("SSH: Accepting connection to %s with %s key (fingerprint: %s)", host, keyType, keyFingerprint)

		// Always accept the key (stateless TOFU)
		return nil
	}
}

// SendFileViaSFTP sends file content to a remote path via SFTP
func SendFileViaSFTP(address string, sshConfig *ssh.ClientConfig, remotePath string, content []byte) error {
	return SendFileViaSFTPWithContext(context.Background(), address, sshConfig, remotePath, content)
}

// SendFileViaSFTPWithContext sends file content to a remote path via SFTP with context support
func SendFileViaSFTPWithContext(ctx context.Context, address string, sshConfig *ssh.ClientConfig, remotePath string, content []byte) error {
	// Create a context-aware dialer
	dialer := &net.Dialer{}
	conn, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		return fmt.Errorf("failed to connect to %s: %w", address, err)
	}
	defer conn.Close()

	// Create SSH connection using the established connection
	sshConn, chans, reqs, err := ssh.NewClientConn(conn, address, sshConfig)
	if err != nil {
		return fmt.Errorf("failed to create SSH connection: %w", err)
	}
	defer sshConn.Close()

	// Create SSH client
	client := ssh.NewClient(sshConn, chans, reqs)
	defer client.Close()

	// Create SFTP client
	sftpClient, err := sftp.NewClient(client)
	if err != nil {
		return fmt.Errorf("failed to create SFTP client: %w", err)
	}
	defer sftpClient.Close()

	// Ensure the directory exists
	remoteDir := filepath.Dir(remotePath)
	if err := sftpClient.MkdirAll(remoteDir); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", remoteDir, err)
	}

	// Create and write the file
	file, err := sftpClient.Create(remotePath)
	if err != nil {
		return fmt.Errorf("failed to create file %s: %w", remotePath, err)
	}
	defer file.Close()

	if _, err := file.Write(content); err != nil {
		return fmt.Errorf("failed to write content: %w", err)
	}

	return nil
}
