//go:build ibmcloud

// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"os"
	"strings"
	"testing"

	pv "github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/test/provisioner/ibmcloud"
	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/test/utils"
	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
)

func TestBasicIbmCreateSimplePod(t *testing.T) {
	assert := IBMCloudAssert{
		VPC: pv.IBMCloudProps.VPC,
	}
	DoTestCreateSimplePod(t, testEnv, assert)
}

func TestBasicIbmCaaDaemonsetRollingUpdate(t *testing.T) {
	if os.Getenv("TEST_CAA_ROLLING_UPDATE") == "yes" {
		assert := IBMRollingUpdateAssert{
			VPC: pv.IBMCloudProps.VPC,
		}
		DoTestCaaDaemonsetRollingUpdate(t, testEnv, &assert)
	} else {
		t.Skip("Ignore CAA DaemonSet upgrade  test")
	}
}

func TestConfIbmCreateConfidentialPod(t *testing.T) {
	instanceProfile := pv.IBMCloudProps.InstanceProfile
	if strings.HasPrefix(instanceProfile, "bz2e") {
		log.Infof("Test SE pod")
		assert := IBMCloudAssert{
			VPC: pv.IBMCloudProps.VPC,
		}
		testCommands := CreateConfidentialPodCheckIBMSECommands()
		DoTestCreateConfidentialPod(t, testEnv, assert, testCommands)
	} else {
		t.Skip("Ignore SE test for simple pod")
	}

}

func TestBasicIbmCreatePodWithConfigMap(t *testing.T) {
	assert := IBMCloudAssert{
		VPC: pv.IBMCloudProps.VPC,
	}
	DoTestCreatePodWithConfigMap(t, testEnv, assert)
}

func TestBasicIbmCreatePodWithSecret(t *testing.T) {
	assert := IBMCloudAssert{
		VPC: pv.IBMCloudProps.VPC,
	}
	DoTestCreatePodWithSecret(t, testEnv, assert)
}

func TestNetIbmCreatePeerPodContainerWithExternalIPAccess(t *testing.T) {
	assert := IBMCloudAssert{
		VPC: pv.IBMCloudProps.VPC,
	}
	DoTestCreatePeerPodContainerWithExternalIPAccess(t, testEnv, assert)
}

func TestBasicIbmCreatePeerPodWithJob(t *testing.T) {
	assert := IBMCloudAssert{
		VPC: pv.IBMCloudProps.VPC,
	}
	DoTestCreatePeerPodWithJob(t, testEnv, assert)
}

func TestResIbmCreatePeerPodAndCheckUserLogs(t *testing.T) {
	assert := IBMCloudAssert{
		VPC: pv.IBMCloudProps.VPC,
	}
	DoTestCreatePeerPodAndCheckUserLogs(t, testEnv, assert)
}

func TestResIbmCreatePeerPodAndCheckWorkDirLogs(t *testing.T) {
	assert := IBMCloudAssert{
		VPC: pv.IBMCloudProps.VPC,
	}
	DoTestCreatePeerPodAndCheckWorkDirLogs(t, testEnv, assert)
}

func TestResIbmCreatePeerPodAndCheckEnvVariableLogsWithImageOnly(t *testing.T) {
	assert := IBMCloudAssert{
		VPC: pv.IBMCloudProps.VPC,
	}
	DoTestCreatePeerPodAndCheckEnvVariableLogsWithImageOnly(t, testEnv, assert)
}

func TestResIbmCreatePeerPodAndCheckEnvVariableLogsWithDeploymentOnly(t *testing.T) {
	assert := IBMCloudAssert{
		VPC: pv.IBMCloudProps.VPC,
	}
	DoTestCreatePeerPodAndCheckEnvVariableLogsWithDeploymentOnly(t, testEnv, assert)
}

func TestResIbmCreatePeerPodAndCheckEnvVariableLogsWithImageAndDeployment(t *testing.T) {
	assert := IBMCloudAssert{
		VPC: pv.IBMCloudProps.VPC,
	}
	DoTestCreatePeerPodAndCheckEnvVariableLogsWithImageAndDeployment(t, testEnv, assert)
}

func TestImgIbmCreatePeerPodWithLargeImage(t *testing.T) {
	assert := IBMCloudAssert{
		VPC: pv.IBMCloudProps.VPC,
	}
	DoTestCreatePeerPodWithLargeImage(t, testEnv, assert)
}

func TestStoreIbmCreatePeerPodWithPVC(t *testing.T) {
	if os.Getenv("TEST_CSI_WRAPPER") == "yes" {
		assert := IBMCloudAssert{
			VPC: pv.IBMCloudProps.VPC,
		}
		nameSpace := "kube-system"
		pvcName := "my-pvc"
		mountPath := "/mount-path"
		storageClassName := "ibmc-vpc-block-5iops-tier"
		storageSize := "10Gi"
		podName := "nginx-pvc-pod"
		imageName, err := utils.GetImage("nginx")
		if err != nil {
			t.Fatal(err)
		}
		containerName := "nginx-pvc-container"
		csiContainerName := "ibm-vpc-block-podvm-node-driver"
		csiImageName := "gcr.io/k8s-staging-cloud-provider-ibm/ibm-vpc-block-csi-driver:v5.2.0"

		myPVC := NewPVC(nameSpace, pvcName, storageClassName, storageSize, corev1.ReadWriteOnce)
		myPodwithPVC := NewPodWithPVCFromIBMVPCBlockDriver(nameSpace, podName, containerName, imageName, csiContainerName, csiImageName, WithPVCBinding(mountPath, pvcName))
		DoTestCreatePeerPodWithPVCAndCSIWrapper(t, testEnv, assert, myPVC, myPodwithPVC, mountPath)
	} else {
		t.Skip("Ignore PeerPod with PVC (CSI wrapper) test")
	}
}

func TestSecIbmCreatePeerPodWithAuthenticatedImageWithImagePullSecretOnPod(t *testing.T) {
	assert := IBMCloudAssert{
		VPC: pv.IBMCloudProps.VPC,
	}
	if os.Getenv("REGISTRY_CREDENTIAL_ENCODED") != "" && os.Getenv("AUTHENTICATED_REGISTRY_IMAGE") != "" {
		DoTestCreatePeerPodWithAuthenticatedImageWithImagePullSecretOnPod(t, testEnv, assert)
	} else {
		t.Skip("Registry Credentials not exported")
	}
}

func TestSecIbmCreatePeerPodWithAuthenticatedImageWithImagePullSecretInServiceAccount(t *testing.T) {
	assert := IBMCloudAssert{
		VPC: pv.IBMCloudProps.VPC,
	}
	if os.Getenv("REGISTRY_CREDENTIAL_ENCODED") != "" && os.Getenv("AUTHENTICATED_REGISTRY_IMAGE") != "" {
		DoTestCreatePeerPodWithAuthenticatedImageWithImagePullSecretInServiceAccount(t, testEnv, assert)
	} else {
		t.Skip("Registry Credentials not exported")
	}
}

func TestSecIbmCreatePeerPodWithAuthenticatedImageWithoutCredentials(t *testing.T) {
	assert := IBMCloudAssert{
		VPC: pv.IBMCloudProps.VPC,
	}
	if os.Getenv("AUTHENTICATED_REGISTRY_IMAGE") != "" {
		DoTestCreatePeerPodWithAuthenticatedImageWithoutCredentials(t, testEnv, assert)
	} else {
		t.Skip("Image Name not exported")
	}
}

func TestBasicIbmDeletePod(t *testing.T) {
	assert := IBMCloudAssert{
		VPC: pv.IBMCloudProps.VPC,
	}
	DoTestDeleteSimplePod(t, testEnv, assert)
}

func TestResIbmPodVMwithNoAnnotations(t *testing.T) {
	assert := IBMCloudAssert{
		VPC: pv.IBMCloudProps.VPC,
	}
	DoTestPodVMwithNoAnnotations(t, testEnv, assert, GetIBMInstanceProfileType("b", "2x8"))
}

func TestResIbmPodVMwithAnnotationsInstanceType(t *testing.T) {
	assert := IBMCloudAssert{
		VPC: pv.IBMCloudProps.VPC,
	}
	DoTestPodVMwithAnnotationsInstanceType(t, testEnv, assert, GetIBMInstanceProfileType("c", "2x4"))
}

func TestResIbmPodVMwithAnnotationsCPUMemory(t *testing.T) {
	assert := IBMCloudAssert{
		VPC: pv.IBMCloudProps.VPC,
	}
	DoTestPodVMwithAnnotationsCPUMemory(t, testEnv, assert, GetIBMInstanceProfileType("m", "2x16"))
}

func TestResIbmPodVMwithAnnotationsInvalidInstanceType(t *testing.T) {
	assert := IBMCloudAssert{
		VPC: pv.IBMCloudProps.VPC,
	}
	DoTestPodVMwithAnnotationsInvalidInstanceType(t, testEnv, assert, GetIBMInstanceProfileType("b", "2x4"))
}
func TestResIbmPodVMwithAnnotationsLargerMemory(t *testing.T) {
	assert := IBMCloudAssert{
		VPC: pv.IBMCloudProps.VPC,
	}
	DoTestPodVMwithAnnotationsLargerMemory(t, testEnv, assert)
}
func TestResIbmPodVMwithAnnotationsLargerCPU(t *testing.T) {
	assert := IBMCloudAssert{
		VPC: pv.IBMCloudProps.VPC,
	}
	DoTestPodVMwithAnnotationsLargerCPU(t, testEnv, assert)
}

func TestBasicIbmCreateNginxDeployment(t *testing.T) {
	assert := IBMCloudAssert{
		VPC: pv.IBMCloudProps.VPC,
	}
	DoTestNginxDeployment(t, testEnv, assert)
}

func TestNetIbmPodToServiceCommunication(t *testing.T) {
	assert := IBMCloudAssert{
		VPC: pv.IBMCloudProps.VPC,
	}
	DoTestPodToServiceCommunication(t, testEnv, assert)
}

func TestNetIbmPodsMTLSCommunication(t *testing.T) {
	assert := IBMCloudAssert{
		VPC: pv.IBMCloudProps.VPC,
	}
	DoTestPodsMTLSCommunication(t, testEnv, assert)
}
