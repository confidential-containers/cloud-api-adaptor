//go:build aws

// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"fmt"
	"testing"

	_ "github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/test/provisioner/aws"
)

func TestAwsCreateSimplePod(t *testing.T) {
	assert := NewAWSAssert()
	DoTestCreateSimplePod(t, testEnv, assert)
}

func TestAwsCreatePodWithConfigMap(t *testing.T) {
	assert := NewAWSAssert()

	DoTestCreatePodWithConfigMap(t, testEnv, assert)
}

func TestAwsCreatePodWithSecret(t *testing.T) {
	t.Skip("Test not passing")
	assert := NewAWSAssert()

	DoTestCreatePodWithSecret(t, testEnv, assert)
}

func TestAwsCreatePeerPodContainerWithExternalIPAccess(t *testing.T) {
	t.Skip("Test not passing")
	assert := NewAWSAssert()

	DoTestCreatePeerPodContainerWithExternalIPAccess(t, testEnv, assert)
}

func TestAwsCreatePeerPodWithJob(t *testing.T) {
	assert := NewAWSAssert()

	DoTestCreatePeerPodWithJob(t, testEnv, assert)
}

func TestAwsCreatePeerPodAndCheckUserLogs(t *testing.T) {
	assert := NewAWSAssert()

	DoTestCreatePeerPodAndCheckUserLogs(t, testEnv, assert)
}

func TestAwsCreatePeerPodAndCheckWorkDirLogs(t *testing.T) {
	assert := NewAWSAssert()

	DoTestCreatePeerPodAndCheckWorkDirLogs(t, testEnv, assert)
}

func TestAwsCreatePeerPodAndCheckEnvVariableLogsWithImageOnly(t *testing.T) {
	assert := NewAWSAssert()

	DoTestCreatePeerPodAndCheckEnvVariableLogsWithImageOnly(t, testEnv, assert)
}

func TestAwsCreatePeerPodAndCheckEnvVariableLogsWithDeploymentOnly(t *testing.T) {
	assert := NewAWSAssert()

	DoTestCreatePeerPodAndCheckEnvVariableLogsWithDeploymentOnly(t, testEnv, assert)
}

func TestAwsCreatePeerPodAndCheckEnvVariableLogsWithImageAndDeployment(t *testing.T) {
	assert := NewAWSAssert()

	DoTestCreatePeerPodAndCheckEnvVariableLogsWithImageAndDeployment(t, testEnv, assert)
}

func TestAwsCreatePeerPodWithLargeImage(t *testing.T) {
	// This test running on default Github runner makes the disk full
	// (`System.IO.IOException: No space left on device`) to the point
	// the job gets aborted.
	SkipTestOnCI(t)
	assert := NewAWSAssert()

	DoTestCreatePeerPodWithLargeImage(t, testEnv, assert)
}

func TestAwsCreatePeerPodWithPVC(t *testing.T) {
	t.Skip("To be implemented")
}

func TestAwsCreatePeerPodWithAuthenticatedImagewithValidCredentials(t *testing.T) {
	t.Skip("To be implemented")
}

func TestAwsCreatePeerPodWithAuthenticatedImageWithInvalidCredentials(t *testing.T) {
	t.Skip("To be implemented")
}

func TestAwsCreatePeerPodWithAuthenticatedImageWithoutCredentials(t *testing.T) {
	t.Skip("To be implemented")
}

func TestAwsDeletePod(t *testing.T) {
	assert := NewAWSAssert()
	DoTestDeleteSimplePod(t, testEnv, assert)
}

func TestAwsCreateNginxDeployment(t *testing.T) {
	t.Skip("Test not passing")
	assert := NewAWSAssert()
	DoTestNginxDeployment(t, testEnv, assert)
}

func TestAwsCreatePeerPodContainerWithInvalidAlternateImage(t *testing.T) {
	assert := NewAWSAssert()
	nonExistingImageName := "ami-123456"
	expectedErrorMessage := fmt.Sprintf("InvalidAMIID.NotFound: The image id '[%s]' does not exist: not found", nonExistingImageName)
	DoTestCreatePeerPodContainerWithInvalidAlternateImage(t, testEnv, assert, nonExistingImageName, expectedErrorMessage)
}

func TestAwsPodWithInitContainer(t *testing.T) {
	assert := NewAWSAssert()
	DoTestPodWithInitContainer(t, testEnv, assert)
}
