//go:build ibmcloud_powervs

// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"context"
	"testing"

	_ "github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/test/provisioner/ibmcloud-powervs"
)

func TestCreateSimplePod(t *testing.T) {
	assert := IBMCloudPowerVSAssert{properties: provisioner.GetProperties(context.Background(), nil)}
	DoTestCreateSimplePod(t, testEnv, assert)
}

func TestCreatePodWithConfigMap(t *testing.T) {
	assert := IBMCloudPowerVSAssert{properties: provisioner.GetProperties(context.Background(), nil)}
	DoTestCreatePodWithConfigMap(t, testEnv, assert)
}

func TestCreatePodWithSecret(t *testing.T) {
	assert := IBMCloudPowerVSAssert{properties: provisioner.GetProperties(context.Background(), nil)}
	DoTestCreatePodWithSecret(t, testEnv, assert)
}

func TestDeletePod(t *testing.T) {
	assert := IBMCloudPowerVSAssert{properties: provisioner.GetProperties(context.Background(), nil)}
	DoTestDeleteSimplePod(t, testEnv, assert)
}
