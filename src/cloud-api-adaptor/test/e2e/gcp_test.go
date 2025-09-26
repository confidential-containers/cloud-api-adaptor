//go:build gcp

// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	_ "github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/test/provisioner/gcp"
	"testing"
)

func TestBasicGcpDeletePod(t *testing.T) {
	assert := GCPCloudAssert{}
	DoTestDeleteSimplePod(t, testEnv, assert)
}

func TestBasicGcpCreateSimplePod(t *testing.T) {
	assert := GCPCloudAssert{}
	DoTestCreateSimplePod(t, testEnv, assert)
}

func TestBasicGcpCreatePodWithConfigMap(t *testing.T) {
	assert := GCPCloudAssert{}
	DoTestCreatePodWithConfigMap(t, testEnv, assert)
}

func TestBasicGcpCreatePodWithSecret(t *testing.T) {
	assert := GCPCloudAssert{}
	DoTestCreatePodWithSecret(t, testEnv, assert)
}
