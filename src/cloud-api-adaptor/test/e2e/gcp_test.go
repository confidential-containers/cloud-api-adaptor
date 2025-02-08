//go:build gcp

// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	_ "github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/test/provisioner/gcp"
	"testing"
)

func TestDeletePodGCP(t *testing.T) {
	assert := GCPCloudAssert{}
	DoTestDeleteSimplePod(t, testEnv, assert)
}

func TestCreateSimplePodGCP(t *testing.T) {
	assert := GCPCloudAssert{}
	DoTestCreateSimplePod(t, testEnv, assert)
}

func TestCreatePodWithConfigMapGCP(t *testing.T) {
	assert := GCPCloudAssert{}
	DoTestCreatePodWithConfigMap(t, testEnv, assert)
}

func TestCreatePodWithSecretGCP(t *testing.T) {
	assert := GCPCloudAssert{}
	DoTestCreatePodWithSecret(t, testEnv, assert)
}
