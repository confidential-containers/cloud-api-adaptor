//go:build openstack

// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	_ "github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/test/provisioner/openstack"
	"testing"
)

func TestDeletePodOpenStack(t *testing.T) {
	assert := OpenStackCloudAssert{}
	DoTestDeleteSimplePod(t, testEnv, assert)
}

func TestCreateSimplePodOpenStack(t *testing.T) {
	assert := OpenStackCloudAssert{}
	DoTestCreateSimplePod(t, testEnv, assert)
}

func TestCreatePodWithConfigMapOpenStack(t *testing.T) {
	assert := OpenStackCloudAssert{}
	DoTestCreatePodWithConfigMap(t, testEnv, assert)
}

func TestCreatePodWithSecretOpenStack(t *testing.T) {
	assert := OpenStackCloudAssert{}
	DoTestCreatePodWithSecret(t, testEnv, assert)
}
