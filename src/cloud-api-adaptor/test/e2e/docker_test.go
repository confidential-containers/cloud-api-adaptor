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

func TestDockerCreatePeerPodWithAuthenticatedImageWithoutCredentials(t *testing.T) {
	assert := DockerAssert{}
	if os.Getenv("AUTHENTICATED_REGISTRY_IMAGE") != "" {
		DoTestCreatePeerPodWithAuthenticatedImageWithoutCredentials(t, testEnv, assert)
	} else {
		t.Skip("Authenticated Image Name not exported")
	}
}

func TestDockerCreatePeerPodWithAuthenticatedImageWithImagePullSecretInServiceAccount(t *testing.T) {
	assert := DockerAssert{}
	if os.Getenv("REGISTRY_CREDENTIAL_ENCODED") != "" && os.Getenv("AUTHENTICATED_REGISTRY_IMAGE") != "" {
		DoTestCreatePeerPodWithAuthenticatedImageWithImagePullSecretInServiceAccount(t, testEnv, assert)
	} else {
		t.Skip("Registry Credentials, or authenticated image name not exported")
	}
}

func TestDockerCreatePeerPodWithAuthenticatedImageWithImagePullSecretOnPod(t *testing.T) {
	assert := DockerAssert{}
	if os.Getenv("REGISTRY_CREDENTIAL_ENCODED") != "" && os.Getenv("AUTHENTICATED_REGISTRY_IMAGE") != "" {
		DoTestCreatePeerPodWithAuthenticatedImageWithImagePullSecretOnPod(t, testEnv, assert)
	} else {
		t.Skip("Registry Credentials, or authenticated image name not exported")
	}
}

func TestDockerCreateWithCpuLimit(t *testing.T) {
	// This test is covered as part of unit test and hence skipping to optimise CI time
	SkipTestOnCI(t)
	assert := DockerAssert{}
	DoTestPodWithCPUMemLimitsAndRequests(t, testEnv, assert, "", "", "200m", "")
}

func TestDockerCreateWithMemLimit(t *testing.T) {
	// This test is covered as part of unit test and hence skipping to optimise CI time
	SkipTestOnCI(t)
	assert := DockerAssert{}
	DoTestPodWithCPUMemLimitsAndRequests(t, testEnv, assert, "", "", "", "200Mi")
}

func TestDockerCreateWithCpuAndMemLimit(t *testing.T) {
	// This test is covered as part of unit test and hence skipping to optimise CI time
	SkipTestOnCI(t)
	assert := DockerAssert{}
	DoTestPodWithCPUMemLimitsAndRequests(t, testEnv, assert, "", "", "200m", "200Mi")
}

func TestDockerCreateWithCpuAndMemRequestLimit(t *testing.T) {
	assert := DockerAssert{}
	DoTestPodWithCPUMemLimitsAndRequests(t, testEnv, assert, "100m", "100Mi", "200m", "200Mi")
}
