//go:build libvirt && cgo

// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"os"
	"testing"

	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/test/provisioner/libvirt"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
)

func TestLibvirtCreateSimplePod(t *testing.T) {
	assert := LibvirtAssert{}
	DoTestCreateSimplePod(t, testEnv, assert)
}

func TestLibvirtCreateSimplePodWithSecureCommsIsValid(t *testing.T) {
	assert := LibvirtAssert{}
	DoTestLibvirtCreateSimplePodWithSecureCommsIsValid(t, testEnv, assert)
}

func TestLibvirtCreatePodWithConfigMap(t *testing.T) {
	assert := LibvirtAssert{}
	DoTestCreatePodWithConfigMap(t, testEnv, assert)
}

func TestLibvirtCreatePodWithSecret(t *testing.T) {
	assert := LibvirtAssert{}
	DoTestCreatePodWithSecret(t, testEnv, assert)
}

func TestLibvirtCreatePeerPodContainerWithExternalIPAccess(t *testing.T) {
	SkipTestOnCI(t)
	if isTestOnCrio() {
		t.Skip("Fails with CRI-O (confidential-containers/cloud-api-adaptor#2100)")
	}
	assert := LibvirtAssert{}
	DoTestCreatePeerPodContainerWithExternalIPAccess(t, testEnv, assert)

}

func TestLibvirtCreatePeerPodContainerWithValidAlternateImage(t *testing.T) {
	assert := LibvirtAssert{}
	DoTestCreatePeerPodContainerWithValidAlternateImage(t, testEnv, assert, libvirt.AlternateVolumeName)
}

func TestLibvirtCreatePeerPodContainerWithInvalidAlternateImage(t *testing.T) {
	assert := LibvirtAssert{}
	nonExistingImageName := "non-existing-image"
	expectedErrorMessage := "Error in creating volume: Can't retrieve volume " + nonExistingImageName
	DoTestCreatePeerPodContainerWithInvalidAlternateImage(t, testEnv, assert, nonExistingImageName, expectedErrorMessage)
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
	// This test is causing issues on CI with instability, so skip until we can resolve this.
	// See https://github.com/confidential-containers/cloud-api-adaptor/issues/1831
	SkipTestOnCI(t)
	assert := LibvirtAssert{}
	DoTestCreatePeerPodAndCheckWorkDirLogs(t, testEnv, assert)
}

func TestLibvirtCreatePeerPodAndCheckEnvVariableLogsWithImageOnly(t *testing.T) {
	// This test is causing issues on CI with instability, so skip until we can resolve this.
	// See https://github.com/confidential-containers/cloud-api-adaptor/issues/1831
	SkipTestOnCI(t)
	assert := LibvirtAssert{}
	DoTestCreatePeerPodAndCheckEnvVariableLogsWithImageOnly(t, testEnv, assert)
}

func TestLibvirtCreatePeerPodAndCheckEnvVariableLogsWithDeploymentOnly(t *testing.T) {
	// This test is causing issues on CI with instability, so skip until we can resolve this.
	// See https://github.com/confidential-containers/cloud-api-adaptor/issues/1831
	SkipTestOnCI(t)
	assert := LibvirtAssert{}
	DoTestCreatePeerPodAndCheckEnvVariableLogsWithDeploymentOnly(t, testEnv, assert)
}

func TestLibvirtCreatePeerPodAndCheckEnvVariableLogsWithImageAndDeployment(t *testing.T) {
	// This test is causing issues on CI with instability, so skip until we can resolve this.
	// See https://github.com/confidential-containers/cloud-api-adaptor/issues/1831
	SkipTestOnCI(t)
	assert := LibvirtAssert{}
	DoTestCreatePeerPodAndCheckEnvVariableLogsWithImageAndDeployment(t, testEnv, assert)
}

func TestLibvirtCreateNginxDeployment(t *testing.T) {
	assert := LibvirtAssert{}
	DoTestNginxDeployment(t, testEnv, assert)
}

func TestLibvirtCreatePeerPodWithLargeImage(t *testing.T) {
	SkipTestOnCI(t)
	assert := LibvirtAssert{}
	DoTestCreatePeerPodWithLargeImage(t, testEnv, assert)
}

func TestLibvirtDeletePod(t *testing.T) {
	assert := LibvirtAssert{}
	DoTestDeleteSimplePod(t, testEnv, assert)
}

func TestLibvirtPodToServiceCommunication(t *testing.T) {
	// This test is causing issues on CI with instability, so skip until we can resolve this.
	if isTestOnCrio() {
		t.Skip("Fails with CRI-O (confidential-containers/cloud-api-adaptor#2100)")
	}
	assert := LibvirtAssert{}
	DoTestPodToServiceCommunication(t, testEnv, assert)
}

func TestLibvirtPodsMTLSCommunication(t *testing.T) {
	// This test is causing issues on CI with instability, so skip until we can resolve this.
	if isTestOnCrio() {
		t.Skip("Fails with CRI-O (confidential-containers/cloud-api-adaptor#2100)")
	}
	assert := LibvirtAssert{}
	DoTestPodsMTLSCommunication(t, testEnv, assert)
}

func TestLibvirtImageDecryption(t *testing.T) {
	if !isTestWithKbs() {
		t.Skip("Skipping kbs related test as kbs is not deployed")
	}

	if isTestOnCrio() {
		t.Skip("Image decryption not supported with CRI-O")
	}

	assert := LibvirtAssert{}
	DoTestImageDecryption(t, testEnv, assert, keyBrokerService)
}

func TestLibvirtSealedSecret(t *testing.T) {
	if !isTestWithKbs() {
		t.Skip("Skipping kbs related test as kbs is not deployed")
	}

	testSecret := envconf.RandomName("coco-pp-e2e-secret", 25)
	resourcePath := "caa/workload_key/test_key.bin"
	err := keyBrokerService.SetSecret(resourcePath, []byte(testSecret))
	if err != nil {
		t.Fatalf("SetSecret failed with: %v", err)
	}
	err = keyBrokerService.EnableKbsCustomizedResourcePolicy("allow_all.rego")
	if err != nil {
		t.Fatalf("EnableKbsCustomizedResourcePolicy failed with: %v", err)
	}
	kbsEndpoint, err := keyBrokerService.GetCachedKbsEndpoint()
	if err != nil {
		t.Fatalf("GetCachedKbsEndpoint failed with: %v", err)
	}
	assert := LibvirtAssert{}
	DoTestSealedSecret(t, testEnv, assert, kbsEndpoint, resourcePath, testSecret)
}

func TestLibvirtKbsKeyRelease(t *testing.T) {
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
	assert := LibvirtAssert{}
	t.Parallel()
	DoTestKbsKeyReleaseForFailure(t, testEnv, assert, kbsEndpoint, resourcePath, testSecret)
	if isTestWithKbsIBMSE() {
		t.Log("KBS with ibmse cases")
		// the allow_*_.rego file is created by follow document
		// https://github.com/confidential-containers/trustee/blob/main/deps/verifier/src/se/README.md#set-attestation-policy
		err = keyBrokerService.EnableKbsCustomizedAttestationPolicy("allow_with_wrong_image_tag.rego")
		if err != nil {
			t.Fatalf("EnableKbsCustomizedAttestationPolicy failed with: %v", err)
		}
		DoTestKbsKeyReleaseForFailure(t, testEnv, assert, kbsEndpoint, resourcePath, testSecret)
		err = keyBrokerService.EnableKbsCustomizedAttestationPolicy("allow_with_correct_claims.rego")
		if err != nil {
			t.Fatalf("EnableKbsCustomizedAttestationPolicy failed with: %v", err)
		}
		DoTestKbsKeyRelease(t, testEnv, assert, kbsEndpoint, resourcePath, testSecret)
	} else {
		t.Log("KBS normal cases")
		err = keyBrokerService.EnableKbsCustomizedResourcePolicy("allow_all.rego")
		if err != nil {
			t.Fatalf("EnableKbsCustomizedResourcePolicy failed with: %v", err)
		}
		DoTestKbsKeyRelease(t, testEnv, assert, kbsEndpoint, resourcePath, testSecret)
	}
}

func TestLibvirtRestrictivePolicyBlocksExec(t *testing.T) {
	assert := LibvirtAssert{}
	DoTestRestrictivePolicyBlocksExec(t, testEnv, assert)
}

func TestLibvirtPermissivePolicyAllowsExec(t *testing.T) {
	assert := LibvirtAssert{}
	DoTestPermissivePolicyAllowsExec(t, testEnv, assert)
}

func TestLibvirtCreatePeerPodWithAuthenticatedImageWithoutCredentials(t *testing.T) {
	assert := LibvirtAssert{}
	if os.Getenv("AUTHENTICATED_REGISTRY_IMAGE") != "" {
		DoTestCreatePeerPodWithAuthenticatedImageWithoutCredentials(t, testEnv, assert)
	} else {
		t.Skip("Authenticated Image Name not exported")
	}
}

func TestLibvirtCreatePeerPodWithAuthenticatedImageWithImagePullSecretInServiceAccount(t *testing.T) {
	assert := LibvirtAssert{}
	if os.Getenv("REGISTRY_CREDENTIAL_ENCODED") != "" && os.Getenv("AUTHENTICATED_REGISTRY_IMAGE") != "" {
		DoTestCreatePeerPodWithAuthenticatedImageWithImagePullSecretInServiceAccount(t, testEnv, assert)
	} else {
		t.Skip("Registry Credentials, or authenticated image name not exported")
	}
}

func TestLibvirtCreatePeerPodWithAuthenticatedImageWithImagePullSecretOnPod(t *testing.T) {
	assert := LibvirtAssert{}
	if os.Getenv("REGISTRY_CREDENTIAL_ENCODED") != "" && os.Getenv("AUTHENTICATED_REGISTRY_IMAGE") != "" {
		DoTestCreatePeerPodWithAuthenticatedImageWithImagePullSecretOnPod(t, testEnv, assert)
	} else {
		t.Skip("Registry Credentials, or authenticated image name not exported")
	}
}

func TestLibvirtCreateWithCpuAndMemRequestLimit(t *testing.T) {
	assert := LibvirtAssert{}
	DoTestPodWithCpuMemLimitsAndRequests(t, testEnv, assert, "100m", "100Mi", "200m", "1200Mi")
}

func TestLibvirtPodVMwithAnnotationsCPUMemory(t *testing.T) {
	assert := LibvirtAssert{}
	DoTestPodVMwithAnnotationsCPUMemory(t, testEnv, assert, CreateInstanceProfileFromCPUMemory(2, 12288))
}

func TestLibvirtPodVMwithAnnotationCPU(t *testing.T) {
	assert := LibvirtAssert{}
	DoTestPodVMwithAnnotationCPU(t, testEnv, assert, CreateInstanceProfileFromCPUMemory(4, libvirt.DefaultMemory))
}

func TestLibvirtPodVMwithAnnotationMemory(t *testing.T) {
	assert := LibvirtAssert{}
	DoTestPodVMwithAnnotationMemory(t, testEnv, assert, CreateInstanceProfileFromCPUMemory(libvirt.DefaultCPU, 7168))
}

func TestLibvirtPodVMwithNoAnnotations(t *testing.T) {
	assert := LibvirtAssert{}
	DoTestPodVMwithNoAnnotations(t, testEnv, assert, CreateInstanceProfileFromCPUMemory(libvirt.DefaultCPU, libvirt.DefaultMemory))
}
