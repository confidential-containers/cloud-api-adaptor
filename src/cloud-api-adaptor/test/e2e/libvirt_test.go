//go:build libvirt && cgo

// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"encoding/base64"
	"os"
	"testing"

	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/test/provisioner/libvirt"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
)

func TestLibvirtCreateSimplePod(t *testing.T) {
	assert := LibvirtAssert{}
	DoTestCreateSimplePod(t, testEnv, assert)
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
	// This test is causing issues on CI with instability, so skip until we can resolve this.
	// See https://github.com/confidential-containers/cloud-api-adaptor/issues/2046
	SkipTestOnCI(t)
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
	// This test is causing issues on CI with instability, so skip until we can resolve this.
	SkipTestOnCI(t)
	if isTestOnCrio() {
		t.Skip("Fails with CRI-O (confidential-containers/cloud-api-adaptor#2100)")
	}
	assert := LibvirtAssert{}
	DoTestPodToServiceCommunication(t, testEnv, assert)
}

func TestLibvirtPodsMTLSCommunication(t *testing.T) {
	// This test is causing issues on CI with instability, so skip until we can resolve this.
	SkipTestOnCI(t)
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
	err = keyBrokerService.EnableKbsCustomizedAttestationPolicy("allow_all.rego")
	if err != nil {
		t.Fatalf("EnableKbsCustomizedAttestationPolicy failed with: %v", err)
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
	err = keyBrokerService.EnableKbsCustomizedResourcePolicy("allow_all.rego")
	if err != nil {
		t.Fatalf("EnableKbsCustomizedResourcePolicy failed with: %v", err)
	}
	err = keyBrokerService.EnableKbsCustomizedAttestationPolicy("deny_all.rego")
	if err != nil {
		t.Fatalf("EnableKbsCustomizedAttestationPolicy failed with: %v", err)
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
		err = keyBrokerService.EnableKbsCustomizedAttestationPolicy("allow_all.rego")
		if err != nil {
			t.Fatalf("EnableKbsCustomizedAttestationPolicy failed with: %v", err)
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

func TestLibvirtCreatePeerPodWithAuthenticatedImageWithValidCredentials(t *testing.T) {
	assert := LibvirtAssert{}
	if os.Getenv("REGISTRY_CREDENTIAL_ENCODED") != "" && os.Getenv("AUTHENTICATED_REGISTRY_IMAGE") != "" {
		DoTestCreatePeerPodWithAuthenticatedImageWithValidCredentials(t, testEnv, assert)
	} else {
		t.Skip("Registry Credentials, or authenticated image name not exported")
	}
}

func TestLibvirtCreateWithCpuAndMemRequestLimit(t *testing.T) {
	assert := LibvirtAssert{}
	DoTestPodWithCpuMemLimitsAndRequests(t, testEnv, assert, "100m", "100Mi", "200m", "200Mi")
}

func TestLibvirtCreatePodVMwithInitdataAnnotations(t *testing.T) {
	assert := LibvirtAssert{}
	// Input data
	data := `algorithm = "sha384"
version = "0.1.0"

[data]
		`
	// "contract.yaml" = ''' ZW52OiBoeXBlci1wcm90ZWN0LWJhc2ljLlRYRSs5cFBvMTNHVEhVbW5jNTVoaFRmMDd0cFlSYWRjeGxpVCtmR0F1cnlYTUFHVWM4eGVTNHFNMVNiZ2grNWplMVFBa1lvLzVieHNPYy90VmIxbUxoNTFqL2l4NmVrcXViKzdyQ281L3c5M0djUzQ5aFc5YXJabzV1cVRieVFuV0RjY0Y4ZUdZRFhHN0NBZ2RJbkVoNW5FNE14VnVKVzJUY0lJNUJLQTZHelM4cm5FWGgzS0Uwc2pZZVcyWnY2TGo0cVBOd0dZNSs2TzhDUm1nc3pkV05NeFUvUHpzVEtoU0k3MFhpRnVKT1F5ZGgvVFJIYTM3RENWaFhGQk5jK1dQYlIrT0Z2R25KM3k0c2RXczYxYXlRbm9PTHJEN0h2eVlvSUJFZlZWSndvUEhtRjd1b1hLQkd1MFVhTFpselBERk95bHB0enJrMTNpMXVUL0NJS3hpd0lkMU1wYnFBZjdrRnJKOGtQYkppdTR5ZFlUbWtOU1hBNEk5M2lZcXFqSWlMMGxLa2l5N0ZQRmRKN3BwaFFoN2gxeU1mYnZPMWJiUXpqYVBXOWtvZk16UUs3QTZ5QkVZWllDMTJQYTV2UHJZbE52OGdYMTFQTlJ1VEdqQUZ5dytRbERqMmp3VkkzRVBTbTFnK3Q5TzhLVG1QK3J4TXBlNngyV3lnbGt4UXMyRDcwaC81amRxei90S3MxSFVkdHJmdmVocVdBMFJLeHY4Y0s3bmQ5cG1LVmtLT2FTZlhOOGQvZnJaSkVWVzBJeG5BMUpzTGViRjFRR2lLUmFlMVRNTlJuTmRLa2NqK25ERmI3a0h2dGpCa1FQNFBzOTlrckFCV2t3Z01NMWRSVWE0eWNoWEJJS2tkcmp1a0FLdUtRdit1SWhOcC9sTlpZNkNBVkFUckNyQmZjPS5VMkZzZEdWa1gxLy9LSko5OVhubzFEakMvWElvN2xnUVpFeFJEdnhMMCtxMGdMdldpOWZrWDhuUXNKN2FraFRLbUwwbTVoUksyVXVocUFPV2M2MVh1eWhRYmpydVQwTDE3UVo1eWxKZ0VWQT0Kd29ya2xvYWQ6IGh5cGVyLXByb3RlY3QtYmFzaWMuQ2lUL2VzUkgrajRRTzl0WmJhR3NWR2FYQ3pORFBvQ1NmZmxXK0M2OVljVDBvUHZjRW82cXBwMVhVY0Q1clZzZHg1ejEzNHZBZlRTMFRNaUlVOEFZNktOVjE3TFpoK2xHQm5jTVJSRUJsOHdlWEV4Y2xxRlc0bS9LSVJzckx2WlYzWHpPTDVTOHhWZExxNmx2Q1dxT0VIL2VmSFVEcnlRZTdlNEhOVzlSR0kwVEp2a3BiQyt1YUZQMWZEcFZsZHB5bFgyZDNEcXk4QmdaRE1ESUxQdmJtdTRET0p4NC96bUY5SGVCbGx6K2JxbGdOdXVwdGtxRnFMcTljdVpwVk5FNDQ3czBLaWFtd0NTSTJqV2syRjlPSjNqUVJmM29nMlg3YXVNZmRFenlDcXRGeEU5VDJTWTd0czMwV0I2c05iNEYzOHpBK0hLYkxXcjBxVUJ1MDY2cWxVZnRBNHZMdmszQzBPaTBqbVF4ZE93OVpINkJQU0VpQ3QzVys0ZUxYdzZ4Z3FkS1R4SkhMUkhvMGR1VXBDbXhqeGFsUHowcTJGc0U1U0hsdHpESkY0LzRaZlBuODZraDN5bW5WVDZaTzVjRVVjWUVOVUdkaTJWZEtod3ZOTW9BQ2ZDWWplaDJCaUlTa3pUYjNOQlFPTUR6d2U5aEpIcDAvcGJPOG5RbUo3eDROOEtvdHJqOVFtN3BGbG1DY2ZvYWFocll2SHUrSyt1R0dObHJIR0JtREF5emNzaDcrRGFkOUZqYkdFVWtsbmc4V0c1NzlUc1dHREJYNHpJTE9iT1MvbUJlR3ZOc2RKMkViSmIyV1FoRnB2bWhvMTFHdXVJU1pDTDNzTzU3UzFIRGFaWVhxbHdjMjJ0VW5LbjE1K24yaldwdWFpMFN0QkJHUkhBdVpoQm01b2M9LlUyRnNkR1ZrWDE5VHRvS3hRSU1MbnhXd3JjTVU4WCtxNmNnQ05WR3F3SE50YUV1K292ZDByOXNhaS9YTXByalBBcURaY0RudmNPMGYycHRuVy9LMjNZNDVxSDNFbUJqSVVtNmEwQXlMb1NkZ1A2MnEwYzRuYk9HRUlvR0NwQnNxdmN5T2MrSVdwRTlsVFhIZm5SR0NOdz09Cg== '''
	// Perform Base64 encoding
	encodedData := base64.StdEncoding.EncodeToString([]byte(data))

	DoTestPodVMwithInitdataAnnotations(t, testEnv, assert, encodedData)
}

func TestLibvirtCreatePodwithInitdataAnnotations(t *testing.T) {
	assert := LibvirtAssert{}
	// Input data
	data := `
		`

	encodedData := base64.StdEncoding.EncodeToString([]byte(data))

	DoTestPodwithInitdataAnnotations(t, testEnv, assert, encodedData)
}
