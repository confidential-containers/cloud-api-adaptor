// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package byom

import (
	"fmt"
	"os"
	"strings"
)

const (
	// ServiceAccountNamespaceFile is the default path to the namespace file in a pod
	ServiceAccountNamespaceFile = "/var/run/secrets/kubernetes.io/serviceaccount/namespace"

	// DefaultNamespace is used when namespace detection fails
	DefaultNamespace = "confidential-containers-system"
)

// detectCurrentNamespace detects the namespace where the current pod is running
// It reads from the service account namespace file that's mounted in every pod
func detectCurrentNamespace() (string, error) {
	return detectCurrentNamespaceWithFile(ServiceAccountNamespaceFile)
}

// detectCurrentNamespaceWithFile allows testing with custom file path
func detectCurrentNamespaceWithFile(namespaceFile string) (string, error) {
	// Try to read namespace from service account file (standard approach)
	if data, err := os.ReadFile(namespaceFile); err == nil {
		namespace := strings.TrimSpace(string(data))
		if namespace != "" {
			return namespace, nil
		}
	}

	// Fallback: try POD_NAMESPACE environment variable
	if namespace := strings.TrimSpace(os.Getenv("POD_NAMESPACE")); namespace != "" {
		return namespace, nil
	}

	return "", fmt.Errorf("unable to detect current namespace: service account file not found and POD_NAMESPACE environment variable not set")
}

// getCurrentNamespaceWithDefault detects the current namespace or returns the default
func getCurrentNamespaceWithDefault() string {
	if namespace, err := detectCurrentNamespace(); err == nil {
		logger.Printf("Detected current namespace: %s", namespace)
		return namespace
	}

	logger.Printf("Using default namespace: %s", DefaultNamespace)
	return DefaultNamespace
}
