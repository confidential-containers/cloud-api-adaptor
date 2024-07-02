//go:build gcp

// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"testing"
	// pv "github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/test/provisioner/gcp"
)

func TestGCPCreateSimplePod(t *testing.T) {
	assert := GCPAssert{}
	DoTestCreateSimplePod(t, testEnv, assert)
}

func TestGCPCreatePodWithConfigMap(t *testing.T) {
	t.Skip("Test not passing")
	assert := NewGCPAssert()

	DoTestCreatePodWithConfigMap(t, testEnv, assert)
}

func TestGCPCreatePodWithSecret(t *testing.T) {
	t.Skip("Test not passing")
	assert := NewGCPAssert()

	DoTestCreatePodWithSecret(t, testEnv, assert)
}

// func TestAwsCreatePeerPodContainerWithExternalIPAccess(t *testing.T) {
// 	t.Skip("Test not passing")
// 	assert := NewAWSAssert()
//
// 	DoTestCreatePeerPodContainerWithExternalIPAccess(t, testEnv, assert)
// }
//
// func TestAwsCreatePeerPodWithJob(t *testing.T) {
// 	assert := NewAWSAssert()
//
// 	DoTestCreatePeerPodWithJob(t, testEnv, assert)
// }
//
// func TestAwsCreatePeerPodAndCheckUserLogs(t *testing.T) {
// 	assert := NewAWSAssert()
//
// 	DoTestCreatePeerPodAndCheckUserLogs(t, testEnv, assert)
// }
//
// func TestAwsCreatePeerPodAndCheckWorkDirLogs(t *testing.T) {
// 	assert := NewAWSAssert()
//
// 	DoTestCreatePeerPodAndCheckWorkDirLogs(t, testEnv, assert)
// }
//
// func TestAwsCreatePeerPodAndCheckEnvVariableLogsWithImageOnly(t *testing.T) {
// 	assert := NewAWSAssert()
//
// 	DoTestCreatePeerPodAndCheckEnvVariableLogsWithImageOnly(t, testEnv, assert)
// }
//
// func TestAwsCreatePeerPodAndCheckEnvVariableLogsWithDeploymentOnly(t *testing.T) {
// 	assert := NewAWSAssert()
//
// 	DoTestCreatePeerPodAndCheckEnvVariableLogsWithDeploymentOnly(t, testEnv, assert)
// }
//
// func TestAwsCreatePeerPodAndCheckEnvVariableLogsWithImageAndDeployment(t *testing.T) {
// 	assert := NewAWSAssert()
//
// 	DoTestCreatePeerPodAndCheckEnvVariableLogsWithImageAndDeployment(t, testEnv, assert)
// }
//
// func TestAwsCreatePeerPodWithLargeImage(t *testing.T) {
// 	assert := NewAWSAssert()
//
// 	DoTestCreatePeerPodWithLargeImage(t, testEnv, assert)
// }
//
// func TestAwsCreatePeerPodWithPVC(t *testing.T) {
// 	t.Skip("To be implemented")
// }
//
// func TestAwsCreatePeerPodWithAuthenticatedImagewithValidCredentials(t *testing.T) {
// 	t.Skip("To be implemented")
// }
//
// func TestAwsCreatePeerPodWithAuthenticatedImageWithInvalidCredentials(t *testing.T) {
// 	t.Skip("To be implemented")
// }
//
// func TestAwsCreatePeerPodWithAuthenticatedImageWithoutCredentials(t *testing.T) {
// 	t.Skip("To be implemented")
// }
//
// func TestAwsDeletePod(t *testing.T) {
// 	assert := NewAWSAssert()
// 	DoTestDeleteSimplePod(t, testEnv, assert)
// }
//
// func TestAwsCreateNginxDeployment(t *testing.T) {
// 	assert := NewAWSAssert()
// 	DoTestNginxDeployment(t, testEnv, assert)
// }
