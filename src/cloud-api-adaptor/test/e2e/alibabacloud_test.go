//go:build alibabacloud

// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"fmt"
	"testing"

	_ "github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/test/provisioner/alibabacloud"
)

func TestAlibabaCloudCreateSimplePod(t *testing.T) {
	assert := NewAlibabaCloudAssert()
	DoTestCreateSimplePod(t, testEnv, assert)
}

func TestAlibabaCloudCreatePodWithConfigMap(t *testing.T) {
	assert := NewAlibabaCloudAssert()
	DoTestCreatePodWithConfigMap(t, testEnv, assert)
}

func TestAlibabaCloudCreatePodWithSecret(t *testing.T) {
	t.Skip("Test not passing")
	assert := NewAlibabaCloudAssert()
	DoTestCreatePodWithSecret(t, testEnv, assert)
}

func TestAlibabaCloudCreatePeerPodContainerWithExternalIPAccess(t *testing.T) {
	t.Skip("Test not passing")
	assert := NewAlibabaCloudAssert()
	DoTestCreatePeerPodContainerWithExternalIPAccess(t, testEnv, assert)
}

func TestAlibabaCloudCreatePeerPodWithJob(t *testing.T) {
	assert := NewAlibabaCloudAssert()
	DoTestCreatePeerPodWithJob(t, testEnv, assert)
}

func TestAlibabaCloudCreatePeerPodAndCheckUserLogs(t *testing.T) {
	assert := NewAlibabaCloudAssert()
	DoTestCreatePeerPodAndCheckUserLogs(t, testEnv, assert)
}

func TestAlibabaCloudCreatePeerPodAndCheckWorkDirLogs(t *testing.T) {
	assert := NewAlibabaCloudAssert()
	DoTestCreatePeerPodAndCheckWorkDirLogs(t, testEnv, assert)
}

func TestAlibabaCloudCreatePeerPodAndCheckEnvVariableLogsWithImageOnly(t *testing.T) {
	assert := NewAlibabaCloudAssert()
	DoTestCreatePeerPodAndCheckEnvVariableLogsWithImageOnly(t, testEnv, assert)
}

func TestAlibabaCloudCreatePeerPodAndCheckEnvVariableLogsWithDeploymentOnly(t *testing.T) {
	assert := NewAlibabaCloudAssert()
	DoTestCreatePeerPodAndCheckEnvVariableLogsWithDeploymentOnly(t, testEnv, assert)
}

func TestAlibabaCloudCreatePeerPodAndCheckEnvVariableLogsWithImageAndDeployment(t *testing.T) {
	assert := NewAlibabaCloudAssert()
	DoTestCreatePeerPodAndCheckEnvVariableLogsWithImageAndDeployment(t, testEnv, assert)
}

func TestAlibabaCloudCreatePeerPodWithLargeImage(t *testing.T) {
	SkipTestOnCI(t)
	assert := NewAlibabaCloudAssert()
	DoTestCreatePeerPodWithLargeImage(t, testEnv, assert)
}

func TestAlibabaCloudCreatePeerPodWithPVC(t *testing.T) {
	t.Skip("To be implemented")
}

func TestAlibabaCloudCreatePeerPodWithAuthenticatedImagewithValidCredentials(t *testing.T) {
	t.Skip("To be implemented")
}

func TestAlibabaCloudCreatePeerPodWithAuthenticatedImageWithInvalidCredentials(t *testing.T) {
	t.Skip("To be implemented")
}

func TestAlibabaCloudCreatePeerPodWithAuthenticatedImageWithoutCredentials(t *testing.T) {
	t.Skip("To be implemented")
}

func TestAlibabaCloudDeletePod(t *testing.T) {
	assert := NewAlibabaCloudAssert()
	DoTestDeleteSimplePod(t, testEnv, assert)
}

func TestAlibabaCloudCreateNginxDeployment(t *testing.T) {
	t.Skip("Test not passing")
	assert := NewAlibabaCloudAssert()
	DoTestNginxDeployment(t, testEnv, assert)
}

func TestAlibabaCloudCreatePeerPodContainerWithInvalidAlternateImage(t *testing.T) {
	SkipTestOnCI(t)
	assert := NewAlibabaCloudAssert()
	nonExistingImageName := "m-nonexistentimageid000000"
	expectedErrorMessage := fmt.Sprintf("InvalidImage.NotFound: The image '%s' does not exist", nonExistingImageName)
	DoTestCreatePeerPodContainerWithInvalidAlternateImage(t, testEnv, assert, nonExistingImageName, expectedErrorMessage)
}

func TestAlibabaCloudPodWithInitContainer(t *testing.T) {
	assert := NewAlibabaCloudAssert()
	DoTestPodWithInitContainer(t, testEnv, assert)
}
