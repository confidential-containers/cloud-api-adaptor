// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package k8sops

import (
	"fmt"
	"os"
)

const (
	// ServiceAccountNamespaceFile is the default path to the namespace file in a pod
	ServiceAccountNamespaceFile = "/var/run/secrets/kubernetes.io/serviceaccount/namespace"

	// DefaultNamespace is used when namespace detection fails
	DefaultNamespace = "confidential-containers-system"
)

// GetCurrentNamespace detects the namespace where the current pod is running
// It reads from the service account namespace file that's mounted in every pod
func GetCurrentNamespace() (string, error) {
	// Try to read namespace from service account file (standard approach)
	if data, err := os.ReadFile(ServiceAccountNamespaceFile); err == nil {
		namespace := string(data)
		if namespace != "" {
			return namespace, nil
		}
	}

	// Fallback: try POD_NAMESPACE environment variable
	if namespace := os.Getenv("POD_NAMESPACE"); namespace != "" {
		return namespace, nil
	}

	return "", fmt.Errorf("unable to detect current namespace: service account file not found and POD_NAMESPACE environment variable not set")
}

// GetCurrentNamespaceWithDefault detects the current namespace or returns the default
func GetCurrentNamespaceWithDefault() string {
	if namespace, err := GetCurrentNamespace(); err == nil {
		return namespace
	}

	return DefaultNamespace
}
