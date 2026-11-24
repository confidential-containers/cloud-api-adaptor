//go:build ecloud

// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"testing"

	_ "github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/test/provisioner/ecloud"
)

func TestEcloudCreateSimplePod(t *testing.T) {
	assert := NewEcloudAssert()
	DoTestCreateSimplePod(t, testEnv, assert)
}

func TestEcloudCreatePodWithConfigMap(t *testing.T) {
	assert := NewEcloudAssert()
	DoTestCreatePodWithConfigMap(t, testEnv, assert)
}

func TestEcloudCreatePodWithSecret(t *testing.T) {
	assert := NewEcloudAssert()
	DoTestCreatePodWithSecret(t, testEnv, assert)
}

func TestEcloudCreatePeerPodWithJob(t *testing.T) {
	assert := NewEcloudAssert()
	DoTestCreatePeerPodWithJob(t, testEnv, assert)
}

func TestEcloudCreatePeerPodAndCheckUserLogs(t *testing.T) {
	assert := NewEcloudAssert()
	DoTestCreatePeerPodAndCheckUserLogs(t, testEnv, assert)
}

func TestEcloudCreatePeerPodAndCheckWorkDirLogs(t *testing.T) {
	assert := NewEcloudAssert()
	DoTestCreatePeerPodAndCheckWorkDirLogs(t, testEnv, assert)
}

func TestEcloudCreatePeerPodAndCheckEnvVariableLogsWithImageOnly(t *testing.T) {
	assert := NewEcloudAssert()
	DoTestCreatePeerPodAndCheckEnvVariableLogsWithImageOnly(t, testEnv, assert)
}

func TestEcloudCreatePeerPodAndCheckEnvVariableLogsWithDeploymentOnly(t *testing.T) {
	assert := NewEcloudAssert()
	DoTestCreatePeerPodAndCheckEnvVariableLogsWithDeploymentOnly(t, testEnv, assert)
}

func TestEcloudCreatePeerPodAndCheckEnvVariableLogsWithImageAndDeployment(t *testing.T) {
	assert := NewEcloudAssert()
	DoTestCreatePeerPodAndCheckEnvVariableLogsWithImageAndDeployment(t, testEnv, assert)
}

func TestEcloudDeletePod(t *testing.T) {
	assert := NewEcloudAssert()
	DoTestDeleteSimplePod(t, testEnv, assert)
}

func TestEcloudPodWithInitContainer(t *testing.T) {
	assert := NewEcloudAssert()
	DoTestPodWithInitContainer(t, testEnv, assert)
}
