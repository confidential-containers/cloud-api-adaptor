//go:build docker

// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"os"
	"testing"

	_ "github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/test/provisioner/docker"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
)

func TestBasicDockerCreateSimplePod(t *testing.T) {
	assert := DockerAssert{}
	DoTestCreateSimplePod(t, testEnv, assert)
}

func TestBasicDockerCreatePodWithConfigMap(t *testing.T) {
	SkipTestOnCI(t)
	assert := DockerAssert{}
	DoTestCreatePodWithConfigMap(t, testEnv, assert)
}

func TestBasicDockerCreatePodWithSecret(t *testing.T) {
	assert := DockerAssert{}
	DoTestCreatePodWithSecret(t, testEnv, assert)
}

func TestNetDockerCreatePeerPodContainerWithExternalIPAccess(t *testing.T) {
	SkipTestOnCI(t)
	assert := DockerAssert{}
	DoTestCreatePeerPodContainerWithExternalIPAccess(t, testEnv, assert)

}

func TestBasicDockerCreatePeerPodWithJob(t *testing.T) {
	assert := DockerAssert{}
	DoTestCreatePeerPodWithJob(t, testEnv, assert)
}

func TestResDockerCreatePeerPodAndCheckUserLogs(t *testing.T) {
	assert := DockerAssert{}
	DoTestCreatePeerPodAndCheckUserLogs(t, testEnv, assert)
}

func TestResDockerCreatePeerPodAndCheckWorkDirLogs(t *testing.T) {
	assert := DockerAssert{}
	DoTestCreatePeerPodAndCheckWorkDirLogs(t, testEnv, assert)
}

func TestResDockerCreatePeerPodAndCheckEnvVariableLogsWithImageOnly(t *testing.T) {
	// This test is causing issues on CI with instability, so skip until we can resolve this.
	// See https://github.com/confidential-containers/cloud-api-adaptor/issues/1831
	SkipTestOnCI(t)
	assert := DockerAssert{}
	DoTestCreatePeerPodAndCheckEnvVariableLogsWithImageOnly(t, testEnv, assert)
}

func TestResDockerCreatePeerPodAndCheckEnvVariableLogsWithDeploymentOnly(t *testing.T) {
	assert := DockerAssert{}
	DoTestCreatePeerPodAndCheckEnvVariableLogsWithDeploymentOnly(t, testEnv, assert)
}

func TestResDockerCreatePeerPodAndCheckEnvVariableLogsWithImageAndDeployment(t *testing.T) {
	// This test is causing issues on CI with instability, so skip until we can resolve this.
	// See https://github.com/confidential-containers/cloud-api-adaptor/issues/1831
	assert := DockerAssert{}
	DoTestCreatePeerPodAndCheckEnvVariableLogsWithImageAndDeployment(t, testEnv, assert)
}

func TestBasicDockerCreateNginxDeployment(t *testing.T) {
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

func TestBasicDockerDeletePod(t *testing.T) {
	assert := DockerAssert{}
	DoTestDeleteSimplePod(t, testEnv, assert)
}

func TestNetDockerPodToServiceCommunication(t *testing.T) {
	assert := DockerAssert{}
	DoTestPodToServiceCommunication(t, testEnv, assert)
}

func TestNetDockerPodsMTLSCommunication(t *testing.T) {
	assert := DockerAssert{}
	DoTestPodsMTLSCommunication(t, testEnv, assert)
}

func TestConfDockerKbsKeyRelease(t *testing.T) {
	if !isTestWithKbs() {
		t.Skip("Skipping kbs related test as kbs is not deployed")
	}
	testSecret := envconf.RandomName("coco-pp-e2e-secret", 25)
	resourcePath := "caa/workload_key/test_key.bin"
	err := keyBrokerService.SetSecret(resourcePath, []byte(testSecret))
	if err != nil {
		t.Fatalf("SetSecret failed with: %v", err)
	}
	err = keyBrokerService.EnableKbsCustomizedResourcePolicy("deny_all.rego")
	if err != nil {
		t.Fatalf("EnableKbsCustomizedResourcePolicy failed with: %v", err)
	}
	kbsEndpoint, err := keyBrokerService.GetCachedKbsEndpoint()
	if err != nil {
		t.Fatalf("GetCachedKbsEndpoint failed with: %v", err)
	}
	assert := DockerAssert{}
	t.Parallel()
	DoTestKbsKeyReleaseForFailure(t, testEnv, assert, kbsEndpoint, resourcePath, testSecret)
	err = keyBrokerService.EnableKbsCustomizedResourcePolicy("allow_all.rego")
	if err != nil {
		t.Fatalf("EnableKbsCustomizedResourcePolicy failed with: %v", err)
	}
	DoTestKbsKeyRelease(t, testEnv, assert, kbsEndpoint, resourcePath, testSecret)
}

func TestSecDockerCreatePeerPodWithAuthenticatedImageWithoutCredentials(t *testing.T) {
	assert := DockerAssert{}
	if os.Getenv("AUTHENTICATED_REGISTRY_IMAGE") != "" {
		DoTestCreatePeerPodWithAuthenticatedImageWithoutCredentials(t, testEnv, assert)
	} else {
		t.Skip("Authenticated Image Name not exported")
	}
}

func TestSecDockerCreatePeerPodWithAuthenticatedImageWithImagePullSecretInServiceAccount(t *testing.T) {
	assert := DockerAssert{}
	if os.Getenv("REGISTRY_CREDENTIAL_ENCODED") != "" && os.Getenv("AUTHENTICATED_REGISTRY_IMAGE") != "" {
		DoTestCreatePeerPodWithAuthenticatedImageWithImagePullSecretInServiceAccount(t, testEnv, assert)
	} else {
		t.Skip("Registry Credentials, or authenticated image name not exported")
	}
}

func TestSecDockerCreatePeerPodWithAuthenticatedImageWithImagePullSecretOnPod(t *testing.T) {
	assert := DockerAssert{}
	if os.Getenv("REGISTRY_CREDENTIAL_ENCODED") != "" && os.Getenv("AUTHENTICATED_REGISTRY_IMAGE") != "" {
		DoTestCreatePeerPodWithAuthenticatedImageWithImagePullSecretOnPod(t, testEnv, assert)
	} else {
		t.Skip("Registry Credentials, or authenticated image name not exported")
	}
}

func TestResDockerCreateWithCpuLimit(t *testing.T) {
	// This test is covered as part of unit test and hence skipping to optimise CI time
	SkipTestOnCI(t)
	assert := DockerAssert{}
	DoTestPodWithCpuMemLimitsAndRequests(t, testEnv, assert, "", "", "200m", "")
}

func TestResDockerCreateWithMemLimit(t *testing.T) {
	// This test is covered as part of unit test and hence skipping to optimise CI time
	SkipTestOnCI(t)
	assert := DockerAssert{}
	DoTestPodWithCpuMemLimitsAndRequests(t, testEnv, assert, "", "", "", "200Mi")
}

func TestResDockerCreateWithCpuAndMemLimit(t *testing.T) {
	// This test is covered as part of unit test and hence skipping to optimise CI time
	SkipTestOnCI(t)
	assert := DockerAssert{}
	DoTestPodWithCpuMemLimitsAndRequests(t, testEnv, assert, "", "", "200m", "200Mi")
}

func TestResDockerCreateWithCpuAndMemRequestLimit(t *testing.T) {
	assert := DockerAssert{}
	DoTestPodWithCpuMemLimitsAndRequests(t, testEnv, assert, "100m", "100Mi", "200m", "200Mi")
}
