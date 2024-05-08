//go:build libvirt && cgo

// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"testing"

	_ "github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/test/provisioner/libvirt"
)

func TestLibvirtCreateSimplePod(t *testing.T) {
	assert := LibvirtAssert{}
	DoTestCreateSimplePod(t, testEnv, assert)
}

func TestLibvirtCreateSimplePodWithNydusAnnotation(t *testing.T) {
	assert := LibvirtAssert{}
	DoTestCreateSimplePodWithNydusAnnotation(t, testEnv, assert)
}

func TestLibvirtCreatePodWithConfigMap(t *testing.T) {
	SkipTestOnCI(t)
	assert := LibvirtAssert{}
	DoTestCreatePodWithConfigMap(t, testEnv, assert)
}

func TestLibvirtCreatePodWithSecret(t *testing.T) {
	assert := LibvirtAssert{}
	DoTestCreatePodWithSecret(t, testEnv, assert)
}

func TestLibvirtCreatePeerPodContainerWithExternalIPAccess(t *testing.T) {
	SkipTestOnCI(t)
	assert := LibvirtAssert{}
	DoTestCreatePeerPodContainerWithExternalIPAccess(t, testEnv, assert)

}

func TestLibvirtCreatePeerPodWithJob(t *testing.T) {
	assert := LibvirtAssert{}
	DoTestCreatePeerPodWithJob(t, testEnv, assert)
}

func TestLibvirtCreatePeerPodAndCheckUserLogs(t *testing.T) {
	assert := LibvirtAssert{}
	DoTestCreatePeerPodAndCheckUserLogs(t, testEnv, assert)
}

func TestLibvirtCreatePeerPodAndCheckWorkDirLogs(t *testing.T) {
	assert := LibvirtAssert{}
	DoTestCreatePeerPodAndCheckWorkDirLogs(t, testEnv, assert)
}

func TestLibvirtCreatePeerPodAndCheckEnvVariableLogsWithImageOnly(t *testing.T) {
	assert := LibvirtAssert{}
	DoTestCreatePeerPodAndCheckEnvVariableLogsWithImageOnly(t, testEnv, assert)
}

func TestLibvirtCreatePeerPodAndCheckEnvVariableLogsWithDeploymentOnly(t *testing.T) {
	assert := LibvirtAssert{}
	DoTestCreatePeerPodAndCheckEnvVariableLogsWithDeploymentOnly(t, testEnv, assert)
}

func TestLibvirtCreatePeerPodAndCheckEnvVariableLogsWithImageAndDeployment(t *testing.T) {
	assert := LibvirtAssert{}
	DoTestCreatePeerPodAndCheckEnvVariableLogsWithImageAndDeployment(t, testEnv, assert)
}

func TestLibvirtCreateNginxDeployment(t *testing.T) {
	assert := LibvirtAssert{}
	DoTestNginxDeployment(t, testEnv, assert)
}

/*
Failing due to issues will pulling image (ErrImagePull)
func TestLibvirtCreatePeerPodWithLargeImage(t *testing.T) {
	assert := LibvirtAssert{}
	DoTestCreatePeerPodWithLargeImage(t, testEnv, assert)
}
*/

func TestLibvirtDeletePod(t *testing.T) {
	assert := LibvirtAssert{}
	DoTestDeleteSimplePod(t, testEnv, assert)
}

func TestLibvirtPodToServiceCommunication(t *testing.T) {
	assert := LibvirtAssert{}
	DoTestPodToServiceCommunication(t, testEnv, assert)
}

func TestLibvirtPodsMTLSCommunication(t *testing.T) {
	assert := LibvirtAssert{}
	DoTestPodsMTLSCommunication(t, testEnv, assert)
}

func TestLibvirtKbsKeyReleaseWithDefaultOpa(t *testing.T) {
	if !isTestWithKbs() {
		t.Skip("Skipping kbs related test as kbs is not deployed")
	}
	if !isTestWithCustomizedOpa() {
		t.Skip("Skipping TestLibvirtKbsKeyReleaseWithCustomizedOpa as default opa is used")
	}
	assert := LibvirtAssert{}
	t.Parallel()
	DoTestKbsKeyRelease(t, testEnv, assert)
}

func TestLibvirtKbsKeyReleaseWithCustomizedOpa(t *testing.T) {
	if !isTestWithKbs() {
		t.Skip("Skipping kbs related test as kbs is not deployed")
	}
	if isTestWithCustomizedOpa() {
		t.Skip("Skipping TestLibvirtKbsKeyReleaseWithDefaultOpa as customized opa is used")
	}
	assert := LibvirtAssert{}
	t.Parallel()
	DoTestKbsKeyReleaseForFailure(t, testEnv, assert)
}
