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

func TestCreateSimplePod(t *testing.T) {
	assert := IBMCloudAssert{
		VPC: pv.IBMCloudProps.VPC,
	}
	DoTestCreateSimplePod(t, testEnv, assert)
}

func TestCaaDaemonsetRollingUpdate(t *testing.T) {
	if os.Getenv("TEST_CAA_ROLLING_UPDATE") == "yes" {
		assert := IBMRollingUpdateAssert{
			VPC: pv.IBMCloudProps.VPC,
		}
		DoTestCaaDaemonsetRollingUpdate(t, testEnv, &assert)
	} else {
		t.Skip("Ignore CAA DaemonSet upgrade  test")
	}
}

func TestCreateConfidentialPod(t *testing.T) {
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

func TestCreatePodWithConfigMap(t *testing.T) {
	assert := IBMCloudAssert{
		VPC: pv.IBMCloudProps.VPC,
	}
	DoTestCreatePodWithConfigMap(t, testEnv, assert)
}

func TestCreatePodWithSecret(t *testing.T) {
	assert := IBMCloudAssert{
		VPC: pv.IBMCloudProps.VPC,
	}
	DoTestCreatePodWithSecret(t, testEnv, assert)
}

func TestCreatePeerPodContainerWithExternalIPAccess(t *testing.T) {
	assert := IBMCloudAssert{
		VPC: pv.IBMCloudProps.VPC,
	}
	DoTestCreatePeerPodContainerWithExternalIPAccess(t, testEnv, assert)
}

func TestCreatePeerPodWithJob(t *testing.T) {
	assert := IBMCloudAssert{
		VPC: pv.IBMCloudProps.VPC,
	}
	DoTestCreatePeerPodWithJob(t, testEnv, assert)
}

func TestCreatePeerPodAndCheckUserLogs(t *testing.T) {
	assert := IBMCloudAssert{
		VPC: pv.IBMCloudProps.VPC,
	}
	DoTestCreatePeerPodAndCheckUserLogs(t, testEnv, assert)
}

func TestCreatePeerPodAndCheckWorkDirLogs(t *testing.T) {
	assert := IBMCloudAssert{
		VPC: pv.IBMCloudProps.VPC,
	}
	DoTestCreatePeerPodAndCheckWorkDirLogs(t, testEnv, assert)
}

func TestCreatePeerPodAndCheckEnvVariableLogsWithImageOnly(t *testing.T) {
	assert := IBMCloudAssert{
		VPC: pv.IBMCloudProps.VPC,
	}
	DoTestCreatePeerPodAndCheckEnvVariableLogsWithImageOnly(t, testEnv, assert)
}

func TestCreatePeerPodAndCheckEnvVariableLogsWithDeploymentOnly(t *testing.T) {
	assert := IBMCloudAssert{
		VPC: pv.IBMCloudProps.VPC,
	}
	DoTestCreatePeerPodAndCheckEnvVariableLogsWithDeploymentOnly(t, testEnv, assert)
}

func TestCreatePeerPodAndCheckEnvVariableLogsWithImageAndDeployment(t *testing.T) {
	assert := IBMCloudAssert{
		VPC: pv.IBMCloudProps.VPC,
	}
	DoTestCreatePeerPodAndCheckEnvVariableLogsWithImageAndDeployment(t, testEnv, assert)
}

func TestCreatePeerPodWithLargeImage(t *testing.T) {
	assert := IBMCloudAssert{
		VPC: pv.IBMCloudProps.VPC,
	}
	DoTestCreatePeerPodWithLargeImage(t, testEnv, assert)
}

func TestCreatePeerPodWithPVC(t *testing.T) {
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

		myPVC := NewPVC(nameSpace, pvcName, storageSize, corev1.ReadWriteOnce, WithStorageClass(storageClassName))
		myPodwithPVC := NewPodWithPVCFromIBMVPCBlockDriver(nameSpace, podName, containerName, imageName, csiContainerName, csiImageName, WithPVCBinding(t, mountPath, pvcName, containerName))
		DoTestCreatePeerPodWithPVCAndCSIWrapper(t, testEnv, assert, myPVC, myPodwithPVC, mountPath)
	} else {
		t.Skip("Ignore PeerPod with PVC (CSI wrapper) test")
	}
}

func TestIBMCloudCreatePeerPodWithAuthenticatedImageWithImagePullSecretOnPod(t *testing.T) {
	assert := IBMCloudAssert{
		VPC: pv.IBMCloudProps.VPC,
	}
	if os.Getenv("REGISTRY_CREDENTIAL_ENCODED") != "" && os.Getenv("AUTHENTICATED_REGISTRY_IMAGE") != "" {
		DoTestCreatePeerPodWithAuthenticatedImageWithImagePullSecretOnPod(t, testEnv, assert)
	} else {
		t.Skip("Registry Credentials not exported")
	}
}

func TestIBMCloudCreatePeerPodWithAuthenticatedImageWithImagePullSecretInServiceAccount(t *testing.T) {
	assert := IBMCloudAssert{
		VPC: pv.IBMCloudProps.VPC,
	}
	if os.Getenv("REGISTRY_CREDENTIAL_ENCODED") != "" && os.Getenv("AUTHENTICATED_REGISTRY_IMAGE") != "" {
		DoTestCreatePeerPodWithAuthenticatedImageWithImagePullSecretInServiceAccount(t, testEnv, assert)
	} else {
		t.Skip("Registry Credentials not exported")
	}
}

func TestCreatePeerPodWithAuthenticatedImageWithoutCredentials(t *testing.T) {
	assert := IBMCloudAssert{
		VPC: pv.IBMCloudProps.VPC,
	}
	if os.Getenv("AUTHENTICATED_REGISTRY_IMAGE") != "" {
		DoTestCreatePeerPodWithAuthenticatedImageWithoutCredentials(t, testEnv, assert)
	} else {
		t.Skip("Image Name not exported")
	}
}

func TestDeletePod(t *testing.T) {
	assert := IBMCloudAssert{
		VPC: pv.IBMCloudProps.VPC,
	}
	DoTestDeleteSimplePod(t, testEnv, assert)
}

func TestPodVMwithNoAnnotations(t *testing.T) {
	assert := IBMCloudAssert{
		VPC: pv.IBMCloudProps.VPC,
	}
	DoTestPodVMwithNoAnnotations(t, testEnv, assert, GetIBMInstanceProfileType("b", "2x8"))
}

func TestPodVMwithAnnotationsInstanceType(t *testing.T) {
	assert := IBMCloudAssert{
		VPC: pv.IBMCloudProps.VPC,
	}
	DoTestPodVMwithAnnotationsInstanceType(t, testEnv, assert, GetIBMInstanceProfileType("c", "2x4"))
}

func TestPodVMwithAnnotationsCPUMemory(t *testing.T) {
	assert := IBMCloudAssert{
		VPC: pv.IBMCloudProps.VPC,
	}
	DoTestPodVMwithAnnotationsCPUMemory(t, testEnv, assert, GetIBMInstanceProfileType("m", "2x16"))
}

func TestPodVMwithAnnotationsInvalidInstanceType(t *testing.T) {
	assert := IBMCloudAssert{
		VPC: pv.IBMCloudProps.VPC,
	}
	DoTestPodVMwithAnnotationsInvalidInstanceType(t, testEnv, assert, GetIBMInstanceProfileType("b", "2x4"))
}
func TestPodVMwithAnnotationsLargerMemory(t *testing.T) {
	assert := IBMCloudAssert{
		VPC: pv.IBMCloudProps.VPC,
	}
	DoTestPodVMwithAnnotationsLargerMemory(t, testEnv, assert)
}
func TestPodVMwithAnnotationsLargerCPU(t *testing.T) {
	assert := IBMCloudAssert{
		VPC: pv.IBMCloudProps.VPC,
	}
	DoTestPodVMwithAnnotationsLargerCPU(t, testEnv, assert)
}

func TestIBMCreateNginxDeployment(t *testing.T) {
	assert := IBMCloudAssert{
		VPC: pv.IBMCloudProps.VPC,
	}
	DoTestNginxDeployment(t, testEnv, assert)
}

func TestPodToServiceCommunication(t *testing.T) {
	assert := IBMCloudAssert{
		VPC: pv.IBMCloudProps.VPC,
	}
	DoTestPodToServiceCommunication(t, testEnv, assert)
}

func TestPodsMTLSCommunication(t *testing.T) {
	assert := IBMCloudAssert{
		VPC: pv.IBMCloudProps.VPC,
	}
	DoTestPodsMTLSCommunication(t, testEnv, assert)
}
