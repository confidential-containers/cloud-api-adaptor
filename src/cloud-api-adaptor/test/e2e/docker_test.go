//go:build docker

// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"testing"

	_ "github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/test/provisioner/docker"
)

func TestDockerCreateSimplePod(t *testing.T) {
	assert := DockerAssert{}
	DoTestCreateSimplePod(t, testEnv, assert)
}

func TestDockerCreatePodWithConfigMap(t *testing.T) {
	SkipTestOnCI(t)
	assert := DockerAssert{}
	DoTestCreatePodWithConfigMap(t, testEnv, assert)
}

func TestDockerCreatePodWithSecret(t *testing.T) {
	assert := DockerAssert{}
	DoTestCreatePodWithSecret(t, testEnv, assert)
}

func TestDockerCreatePeerPodContainerWithExternalIPAccess(t *testing.T) {
	SkipTestOnCI(t)
	assert := DockerAssert{}
	DoTestCreatePeerPodContainerWithExternalIPAccess(t, testEnv, assert)

}

func TestDockerCreatePeerPodWithJob(t *testing.T) {
	assert := DockerAssert{}
	DoTestCreatePeerPodWithJob(t, testEnv, assert)
}

func TestDockerCreatePeerPodAndCheckUserLogs(t *testing.T) {
	assert := DockerAssert{}
	DoTestCreatePeerPodAndCheckUserLogs(t, testEnv, assert)
}

func TestDockerCreatePeerPodAndCheckWorkDirLogs(t *testing.T) {
	assert := DockerAssert{}
	DoTestCreatePeerPodAndCheckWorkDirLogs(t, testEnv, assert)
}

func TestDockerCreatePeerPodAndCheckEnvVariableLogsWithImageOnly(t *testing.T) {
	// This test is causing issues on CI with instability, so skip until we can resolve this.
	// See https://github.com/confidential-containers/cloud-api-adaptor/issues/1831
	SkipTestOnCI(t)
	assert := DockerAssert{}
	DoTestCreatePeerPodAndCheckEnvVariableLogsWithImageOnly(t, testEnv, assert)
}

func TestDockerCreatePeerPodAndCheckEnvVariableLogsWithDeploymentOnly(t *testing.T) {
	assert := DockerAssert{}
	DoTestCreatePeerPodAndCheckEnvVariableLogsWithDeploymentOnly(t, testEnv, assert)
}

func TestDockerCreatePeerPodAndCheckEnvVariableLogsWithImageAndDeployment(t *testing.T) {
	// This test is causing issues on CI with instability, so skip until we can resolve this.
	// See https://github.com/confidential-containers/cloud-api-adaptor/issues/1831
	assert := DockerAssert{}
	DoTestCreatePeerPodAndCheckEnvVariableLogsWithImageAndDeployment(t, testEnv, assert)
}

func TestDockerCreateNginxDeployment(t *testing.T) {
	assert := DockerAssert{}
	DoTestNginxDeployment(t, testEnv, assert)
}

/*
Failing due to issues will pulling image (ErrImagePull)
func TestDockerCreatePeerPodWithLargeImage(t *testing.T) {
	assert := DockerAssert{}
	DoTestCreatePeerPodWithLargeImage(t, testEnv, assert)
}
*/

func TestDockerDeletePod(t *testing.T) {
	assert := DockerAssert{}
	DoTestDeleteSimplePod(t, testEnv, assert)
}

func TestDockerPodToServiceCommunication(t *testing.T) {
	assert := DockerAssert{}
	DoTestPodToServiceCommunication(t, testEnv, assert)
}

func TestDockerPodsMTLSCommunication(t *testing.T) {
	assert := DockerAssert{}
	DoTestPodsMTLSCommunication(t, testEnv, assert)
}

func TestDockerKbsKeyRelease(t *testing.T) {
	if !isTestWithKbs() {
		t.Skip("Skipping kbs related test as kbs is not deployed")
	}
	_ = keyBrokerService.EnableKbsCustomizedPolicy("deny_all.rego")
	assert := DockerAssert{}
	t.Parallel()
	DoTestKbsKeyReleaseForFailure(t, testEnv, assert)
	_ = keyBrokerService.EnableKbsCustomizedPolicy("allow_all.rego")
	DoTestKbsKeyRelease(t, testEnv, assert)
}
