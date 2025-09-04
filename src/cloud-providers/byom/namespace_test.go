// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package byom

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetectCurrentNamespaceWithFile(t *testing.T) {
	// Test with service account file
	t.Run("ServiceAccountFile", func(t *testing.T) {
		// Create temporary file for testing
		tmpDir := t.TempDir()
		tmpFile := filepath.Join(tmpDir, "namespace")

		// Write test namespace to file
		testNamespace := "test-namespace"
		err := os.WriteFile(tmpFile, []byte(testNamespace), 0644)
		if err != nil {
			t.Fatalf("Failed to write test file: %v", err)
		}

		// Test detection with custom file
		namespace, err := detectCurrentNamespaceWithFile(tmpFile)
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}

		if namespace != testNamespace {
			t.Errorf("Expected namespace %s, got %s", testNamespace, namespace)
		}
	})

	// Test with POD_NAMESPACE environment variable
	t.Run("POD_NAMESPACE", func(t *testing.T) {
		// Set environment variable
		testNamespace := "pod-namespace"
		os.Setenv("POD_NAMESPACE", testNamespace)
		defer os.Unsetenv("POD_NAMESPACE")

		// Test detection with nonexistent file (falls back to env var)
		namespace, err := detectCurrentNamespaceWithFile("/nonexistent/path")
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}

		if namespace != testNamespace {
			t.Errorf("Expected namespace %s, got %s", testNamespace, namespace)
		}
	})

	// Test failure case
	t.Run("NoNamespaceFound", func(t *testing.T) {
		// Clear POD_NAMESPACE environment variable
		os.Unsetenv("POD_NAMESPACE")

		// Test detection with nonexistent file and no env vars
		_, err := detectCurrentNamespaceWithFile("/nonexistent/path")
		if err == nil {
			t.Error("Expected error when no namespace sources available")
		}
	})
}

func TestGetCurrentNamespaceWithDefault(t *testing.T) {
	// Test with successful detection using real files and environment
	t.Run("SuccessfulDetection", func(t *testing.T) {
		// Create temporary file for testing
		tmpDir := t.TempDir()
		tmpFile := filepath.Join(tmpDir, "namespace")

		// Write test namespace to file
		testNamespace := "detected-namespace"
		err := os.WriteFile(tmpFile, []byte(testNamespace), 0644)
		if err != nil {
			t.Fatalf("Failed to write test file: %v", err)
		}

		// Test detection with custom file - this simulates successful detection
		namespace, err := detectCurrentNamespaceWithFile(tmpFile)
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}

		if namespace != testNamespace {
			t.Errorf("Expected detected namespace %s, got %s", testNamespace, namespace)
		}
	})

	// Test with detection failure falls back to default
	t.Run("DefaultFallback", func(t *testing.T) {
		// Clear POD_NAMESPACE environment variable
		os.Unsetenv("POD_NAMESPACE")

		// Test with nonexistent file (simulates detection failure)
		namespace := getCurrentNamespaceWithDefault()
		if namespace != DefaultNamespace {
			t.Errorf("Expected default namespace %s, got %s", DefaultNamespace, namespace)
		}
	})
}

func TestNamespaceWithWhitespace(t *testing.T) {
	// Test that whitespace is properly trimmed
	t.Run("TrimWhitespace", func(t *testing.T) {
		// Create temporary file with whitespace
		tmpDir := t.TempDir()
		tmpFile := filepath.Join(tmpDir, "namespace")

		// Write test namespace with whitespace to file
		testNamespace := "test-namespace"
		namespaceWithWhitespace := "  " + testNamespace + "\n\t  "
		err := os.WriteFile(tmpFile, []byte(namespaceWithWhitespace), 0644)
		if err != nil {
			t.Fatalf("Failed to write test file: %v", err)
		}

		// Test detection with custom file
		namespace, err := detectCurrentNamespaceWithFile(tmpFile)
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}

		if namespace != testNamespace {
			t.Errorf("Expected trimmed namespace %s, got %s", testNamespace, namespace)
		}
	})
}

func TestConfigurationOptions(t *testing.T) {
	// Test namespace configuration in Config struct
	t.Run("ConfigNamespaceOptions", func(t *testing.T) {
		config := &Config{
			PoolNamespace:     "custom-namespace",
			PoolConfigMapName: "custom-configmap",
		}

		if config.PoolNamespace != "custom-namespace" {
			t.Errorf("Expected PoolNamespace 'custom-namespace', got %s", config.PoolNamespace)
		}

		if config.PoolConfigMapName != "custom-configmap" {
			t.Errorf("Expected PoolConfigMapName 'custom-configmap', got %s", config.PoolConfigMapName)
		}
	})

	// Test default values
	t.Run("DefaultConfigValues", func(t *testing.T) {
		config := &Config{}

		if config.PoolNamespace != "" {
			t.Errorf("Expected empty PoolNamespace by default, got %s", config.PoolNamespace)
		}

		if config.PoolConfigMapName != "" {
			t.Errorf("Expected empty PoolConfigMapName by default, got %s", config.PoolConfigMapName)
		}
	})
}
