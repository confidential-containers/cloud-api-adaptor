//go:build aws

// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"fmt"
	"testing"

	_ "github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/test/provisioner/aws"
)

func TestBasicAwsCreateSimplePod(t *testing.T) {
	assert := NewAWSAssert()
	DoTestCreateSimplePod(t, testEnv, assert)
}

func TestBasicAwsCreatePodWithConfigMap(t *testing.T) {
	assert := NewAWSAssert()

	DoTestCreatePodWithConfigMap(t, testEnv, assert)
}

func TestBasicAwsCreatePodWithSecret(t *testing.T) {
	t.Skip("Test not passing")
	assert := NewAWSAssert()

	DoTestCreatePodWithSecret(t, testEnv, assert)
}

func TestNetAwsCreatePeerPodContainerWithExternalIPAccess(t *testing.T) {
	t.Skip("Test not passing")
	assert := NewAWSAssert()

	DoTestCreatePeerPodContainerWithExternalIPAccess(t, testEnv, assert)
}

func TestBasicAwsCreatePeerPodWithJob(t *testing.T) {
	assert := NewAWSAssert()

	DoTestCreatePeerPodWithJob(t, testEnv, assert)
}

func TestResAwsCreatePeerPodAndCheckUserLogs(t *testing.T) {
	assert := NewAWSAssert()

	DoTestCreatePeerPodAndCheckUserLogs(t, testEnv, assert)
}

func TestResAwsCreatePeerPodAndCheckWorkDirLogs(t *testing.T) {
	assert := NewAWSAssert()

	DoTestCreatePeerPodAndCheckWorkDirLogs(t, testEnv, assert)
}

func TestResAwsCreatePeerPodAndCheckEnvVariableLogsWithImageOnly(t *testing.T) {
	assert := NewAWSAssert()

	DoTestCreatePeerPodAndCheckEnvVariableLogsWithImageOnly(t, testEnv, assert)
}

func TestResAwsCreatePeerPodAndCheckEnvVariableLogsWithDeploymentOnly(t *testing.T) {
	assert := NewAWSAssert()

	DoTestCreatePeerPodAndCheckEnvVariableLogsWithDeploymentOnly(t, testEnv, assert)
}

func TestResAwsCreatePeerPodAndCheckEnvVariableLogsWithImageAndDeployment(t *testing.T) {
	assert := NewAWSAssert()

	DoTestCreatePeerPodAndCheckEnvVariableLogsWithImageAndDeployment(t, testEnv, assert)
}

func TestImgAwsCreatePeerPodWithLargeImage(t *testing.T) {
	assert := NewAWSAssert()

	DoTestCreatePeerPodWithLargeImage(t, testEnv, assert)
}

func TestStoreAwsCreatePeerPodWithPVC(t *testing.T) {
	t.Skip("To be implemented")
}

func TestSecAwsCreatePeerPodWithAuthenticatedImageValidCredentials(t *testing.T) {
	t.Skip("To be implemented")
}

func TestSecAwsCreatePeerPodWithAuthenticatedImageInvalidCredentials(t *testing.T) {
	t.Skip("To be implemented")
}

func TestSecAwsCreatePeerPodWithAuthenticatedImageWithoutCredentials(t *testing.T) {
	t.Skip("To be implemented")
}

func TestBasicAwsDeletePod(t *testing.T) {
	assert := NewAWSAssert()
	DoTestDeleteSimplePod(t, testEnv, assert)
}

func TestBasicAwsCreateNginxDeployment(t *testing.T) {
	t.Skip("Test not passing")
	assert := NewAWSAssert()
	DoTestNginxDeployment(t, testEnv, assert)
}

func TestImgAwsCreatePeerPodContainerWithInvalidAlternateImage(t *testing.T) {
	assert := NewAWSAssert()
	nonExistingImageName := "ami-123456"
	expectedErrorMessage := fmt.Sprintf("InvalidAMIID.NotFound: The image id '[%s]' does not exist: not found", nonExistingImageName)
	DoTestCreatePeerPodContainerWithInvalidAlternateImage(t, testEnv, assert, nonExistingImageName, expectedErrorMessage)
}

func TestBasicAwsPodWithInitContainer(t *testing.T) {
	assert := NewAWSAssert()
	DoTestPodWithInitContainer(t, testEnv, assert)
}
