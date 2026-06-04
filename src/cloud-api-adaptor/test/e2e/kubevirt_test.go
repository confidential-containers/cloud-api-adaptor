//go:build kubevirt

// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	_ "github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/test/provisioner/kubevirt"
	"testing"
)

func TestDeletePodKubeVirt(t *testing.T) {
	assert := KubeVirtCloudAssert{}
	DoTestDeleteSimplePod(t, testEnv, assert)
}

func TestCreateSimplePodKubeVirt(t *testing.T) {
	assert := KubeVirtCloudAssert{}
	DoTestCreateSimplePod(t, testEnv, assert)
}

func TestCreatePodWithConfigMapKubeVirt(t *testing.T) {
	assert := KubeVirtCloudAssert{}
	DoTestCreatePodWithConfigMap(t, testEnv, assert)
}

func TestCreatePodWithSecretKubeVirt(t *testing.T) {
	assert := KubeVirtCloudAssert{}
	DoTestCreatePodWithSecret(t, testEnv, assert)
}
