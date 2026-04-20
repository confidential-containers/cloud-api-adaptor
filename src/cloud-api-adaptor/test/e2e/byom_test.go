//go:build byom

// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"testing"

	_ "github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/test/provisioner/byom"
)

func TestByomCreateSimplePod(t *testing.T) {
	assert := ByomAssert{}
	DoTestCreateSimplePod(t, testEnv, assert)
}

func TestByomCreatePodWithConfigMap(t *testing.T) {
	assert := ByomAssert{}
	DoTestCreatePodWithConfigMap(t, testEnv, assert)
}

func TestByomCreatePodWithSecret(t *testing.T) {
	assert := ByomAssert{}
	DoTestCreatePodWithSecret(t, testEnv, assert)
}

func TestByomCreatePeerPodContainerWithExternalIPAccess(t *testing.T) {
	SkipTestOnCI(t)
	assert := ByomAssert{}
	DoTestCreatePeerPodContainerWithExternalIPAccess(t, testEnv, assert)
}

func TestByomCreatePeerPodWithJob(t *testing.T) {
	assert := ByomAssert{}
	DoTestCreatePeerPodWithJob(t, testEnv, assert)
}

func TestByomCreatePeerPodAndCheckUserLogs(t *testing.T) {
	assert := ByomAssert{}
	DoTestCreatePeerPodAndCheckUserLogs(t, testEnv, assert)
}

func TestByomCreatePeerPodAndCheckWorkDirLogs(t *testing.T) {
	assert := ByomAssert{}
	DoTestCreatePeerPodAndCheckWorkDirLogs(t, testEnv, assert)
}

func TestByomCreatePeerPodAndCheckEnvVariableLogsWithImageOnly(t *testing.T) {
	assert := ByomAssert{}
	DoTestCreatePeerPodAndCheckEnvVariableLogsWithImageOnly(t, testEnv, assert)
}

func TestByomCreatePeerPodAndCheckEnvVariableLogsWithDeploymentOnly(t *testing.T) {
	assert := ByomAssert{}
	DoTestCreatePeerPodAndCheckEnvVariableLogsWithDeploymentOnly(t, testEnv, assert)
}

func TestByomCreatePeerPodAndCheckEnvVariableLogsWithImageAndDeployment(t *testing.T) {
	assert := ByomAssert{}
	DoTestCreatePeerPodAndCheckEnvVariableLogsWithImageAndDeployment(t, testEnv, assert)
}

func TestByomCreateNginxDeployment(t *testing.T) {
	assert := ByomAssert{}
	DoTestNginxDeployment(t, testEnv, assert)
}

func TestByomDeletePod(t *testing.T) {
	assert := ByomAssert{}
	DoTestDeleteSimplePod(t, testEnv, assert)
}

func TestByomPodToServiceCommunication(t *testing.T) {
	assert := ByomAssert{}
	DoTestPodToServiceCommunication(t, testEnv, assert)
}

func TestByomPodsMTLSCommunication(t *testing.T) {
	assert := ByomAssert{}
	DoTestPodsMTLSCommunication(t, testEnv, assert)
}

func TestByomCreateWithCpuAndMemRequestLimit(t *testing.T) {
	assert := ByomAssert{}
	DoTestPodWithCPUMemLimitsAndRequests(t, testEnv, assert, "100m", "100Mi", "200m", "200Mi")
}
