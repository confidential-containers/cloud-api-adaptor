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

func TestLibvirtKbsKeyRelease(t *testing.T) {
	if !isTestWithKbs() {
		t.Skip("Skipping kbs related test as kbs is not deployed")
	}
	_ = keyBrokerService.SetSampleSecretKey()
	_ = keyBrokerService.EnableKbsCustomizedResourcePolicy("allow_all.rego")
	_ = keyBrokerService.EnableKbsCustomizedAttestationPolicy("deny_all.rego")
	assert := LibvirtAssert{}
	t.Parallel()
	DoTestKbsKeyReleaseForFailure(t, testEnv, assert)
	if isTestWithKbsIBMSE() {
		t.Log("KBS with ibmse cases")
		// the allow_*_.rego file is created by follow document
		// https://github.com/confidential-containers/trustee/blob/main/deps/verifier/src/se/README.md#set-attestation-policy
		_ = keyBrokerService.EnableKbsCustomizedAttestationPolicy("allow_with_wrong_image_tag.rego")
		DoTestKbsKeyReleaseForFailure(t, testEnv, assert)
		_ = keyBrokerService.EnableKbsCustomizedAttestationPolicy("allow_with_correct_claims.rego")
		DoTestKbsKeyRelease(t, testEnv, assert)
	} else {
		t.Log("KBS normal cases")
		_ = keyBrokerService.EnableKbsCustomizedAttestationPolicy("allow_all.rego")
		DoTestKbsKeyRelease(t, testEnv, assert)
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
