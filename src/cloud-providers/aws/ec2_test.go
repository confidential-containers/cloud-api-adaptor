// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package aws

import (
	"os"
	"testing"
)

// TestNewEC2Client_StaticCredentials verifies that NewEC2Client uses static credentials when provided
func TestNewEC2Client_StaticCredentials(t *testing.T) {
	// Clear any environment variables that might interfere
	os.Unsetenv("AWS_WEB_IDENTITY_TOKEN_FILE")
	os.Unsetenv("AWS_ROLE_ARN")
	os.Unsetenv("AWS_ACCESS_KEY_ID")
	os.Unsetenv("AWS_SECRET_ACCESS_KEY")

	cfg := Config{
		AccessKeyID: "AKIAIOSFODNN7EXAMPLE",
		SecretKey:   "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
		Region:      "us-east-1",
	}

	// This should not panic or return error for invalid credentials (just test config creation)
	// We can't actually test AWS connectivity without real credentials
	client, err := NewEC2Client(cfg)
	if err != nil {
		t.Errorf("NewEC2Client with static credentials failed: %v", err)
	}
	if client == nil {
		t.Error("Expected non-nil client")
	}
}

// TestNewEC2Client_DefaultCredentialChain verifies that NewEC2Client uses default credential chain when no static credentials
func TestNewEC2Client_DefaultCredentialChain(t *testing.T) {
	// Clear any environment variables
	os.Unsetenv("AWS_WEB_IDENTITY_TOKEN_FILE")
	os.Unsetenv("AWS_ROLE_ARN")
	os.Unsetenv("AWS_ACCESS_KEY_ID")
	os.Unsetenv("AWS_SECRET_ACCESS_KEY")

	cfg := Config{
		AccessKeyID: "",
		SecretKey:   "",
		Region:      "us-east-1",
	}

	// This should use default credential chain
	// It will fail to find credentials in test environment, but the SDK initialization should work
	client, err := NewEC2Client(cfg)
	if err != nil {
		t.Errorf("NewEC2Client with default credential chain failed: %v", err)
	}
	if client == nil {
		t.Error("Expected non-nil client")
	}
}

// TestNewEC2Client_MissingSecretKey verifies that having only AccessKeyID without SecretKey still works
// (it will be ignored and fall through to default credential chain)
func TestNewEC2Client_PartialStaticCredentials(t *testing.T) {
	// Clear environment
	os.Unsetenv("AWS_WEB_IDENTITY_TOKEN_FILE")
	os.Unsetenv("AWS_ROLE_ARN")
	os.Unsetenv("AWS_ACCESS_KEY_ID")
	os.Unsetenv("AWS_SECRET_ACCESS_KEY")

	// Only AccessKeyID, no SecretKey
	cfg := Config{
		AccessKeyID: "AKIAIOSFODNN7EXAMPLE",
		SecretKey:   "", // Missing
		Region:      "us-east-1",
	}

	// Should fall through to default credential chain since both are required for static credentials
	client, err := NewEC2Client(cfg)
	if err != nil {
		t.Errorf("NewEC2Client with partial credentials failed: %v", err)
	}
	if client == nil {
		t.Error("Expected non-nil client")
	}
}
