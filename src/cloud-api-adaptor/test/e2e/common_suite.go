// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"bytes"
	"fmt"
	"math/rand"
	"os"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/pkg/util/tlsutil"
	pv "github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/test/provisioner"
	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/test/utils"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/e2e-framework/pkg/env"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
)

var E2eNamespace = envconf.RandomName("coco-pp-e2e-test", 25)

// DoTestCreateSimplePod tests a simple peer-pod can be created.
func DoTestCreateSimplePod(t *testing.T, e env.Environment, assert CloudAssert) {
	pod := NewBusyboxPodWithName(E2eNamespace, "simple-test").GetPodOrFatal(t)
	if isTestOnCrio() {
		t.Log("crio busybox error")
		NewTestCase(t, e, "SimplePeerPod", assert, "PodVM is created").WithPod(pod).Run()
	} else {
		NewTestCase(t, e, "SimplePeerPod", assert, "PodVM is created").WithPod(pod).WithNydusSnapshotter().Run()
	}
}

func DoTestLibvirtCreateSimplePodWithSecureCommsIsValid(t *testing.T, e env.Environment, assert CloudAssert) {
	if os.Getenv("SECURE_COMMS") != "true" {
		t.Skip("Skip - SecureComms is configured to be inactive - no need to test")
	}
	pod := NewBusyboxPodWithName(E2eNamespace, "simple-test-with-security-comms-is-active").GetPodOrFatal(t)
	NewTestCase(t, e, "SimplePeerPodWithSecureComms", assert, "PodVM is created with secure comms").WithPod(pod).WithExpectedCaaPodLogStrings("Using PP SecureComms").Run()
}

func DoTestDeleteSimplePod(t *testing.T, e env.Environment, assert CloudAssert) {
	pod := NewBusyboxPodWithName(E2eNamespace, "deletion-test").GetPodOrFatal(t)
	duration := assert.DefaultTimeout()
	NewTestCase(t, e, "DeletePod", assert, "Deletion complete").WithPod(pod).WithDeleteAssertion(&duration).Run()
}

func DoTestCreatePodWithConfigMap(t *testing.T, e env.Environment, assert CloudAssert) {
	podName := "busybox-configmap-pod"
	containerName := "busybox-configmap-container"
	imageName := getBusyboxTestImage(t)
	configMapName := "busybox-configmap"
	configMapFileName := "example.txt"
	podKubeConfigmapDir := "/etc/config/"
	configMapPath := podKubeConfigmapDir + configMapFileName
	configMapContents := "Hello, world"
	configMapData := map[string]string{configMapFileName: configMapContents}
	pod := NewPod(E2eNamespace, podName, containerName, imageName, WithConfigMapBinding(podKubeConfigmapDir, configMapName), WithCommand([]string{"/bin/sh", "-c", "sleep 3600"}))
	configMap := NewConfigMap(E2eNamespace, configMapName, configMapData)
	testCommands := []TestCommand{
		{
			Command:       []string{"cat", configMapPath},
			ContainerName: pod.Spec.Containers[0].Name,
			TestCommandStdoutFn: func(stdout bytes.Buffer) bool {
				if stdout.String() == configMapContents {
					t.Logf("Data Inside Configmap: %s", stdout.String())
					return true
				} else {
					t.Errorf("Configmap has invalid Data: %s", stdout.String())
					return false
				}
			},
			TestCommandStderrFn: IsBufferEmpty,
		},
	}

	NewTestCase(t, e, "ConfigMapPeerPod", assert, "Configmap is created and contains data").WithPod(pod).WithConfigMap(configMap).WithTestCommands(testCommands).Run()
}

func DoTestCreatePodWithSecret(t *testing.T, e env.Environment, assert CloudAssert) {
	//DoTestCreatePod(t, assert, "Secret is created and contains data", pod)
	podName := "busybox-secret-pod"
	containerName := "busybox-secret-container"
	imageName := getBusyboxTestImage(t)
	secretName := "busybox-secret"
	podKubeSecretsDir := "/etc/secret/"
	usernameFileName := "username"
	username := "admin"
	usernamePath := podKubeSecretsDir + usernameFileName
	passwordFileName := "password"
	password := "password"
	passwordPath := podKubeSecretsDir + passwordFileName
	secretData := map[string][]byte{passwordFileName: []byte(password), usernameFileName: []byte(username)}
	pod := NewPod(E2eNamespace, podName, containerName, imageName, WithSecretBinding(t, podKubeSecretsDir, secretName, containerName), WithCommand([]string{"/bin/sh", "-c", "sleep 3600"}))
	secret := NewSecret(E2eNamespace, secretName, secretData, v1.SecretTypeOpaque)

	testCommands := []TestCommand{
		{
			Command:       []string{"cat", usernamePath},
			ContainerName: pod.Spec.Containers[0].Name,
			TestCommandStdoutFn: func(stdout bytes.Buffer) bool {
				if stdout.String() == username {
					t.Logf("Username from secret inside pod: %s", stdout.String())
					return true
				} else {
					t.Errorf("Username value from secret inside pod unexpected. Expected %s, got %s", username, stdout.String())
					return false
				}
			},
			TestCommandStderrFn: IsBufferEmpty,
		},
		{
			Command:       []string{"cat", passwordPath},
			ContainerName: pod.Spec.Containers[0].Name,
			TestCommandStdoutFn: func(stdout bytes.Buffer) bool {
				if stdout.String() == password {
					t.Logf("Password from secret inside pod: %s", stdout.String())
					return true
				} else {
					t.Errorf("Password value from secret inside pod unexpected. Expected %s, got %s", password, stdout.String())
					return false
				}
			},
			TestCommandStderrFn: IsBufferEmpty,
		},
	}

	NewTestCase(t, e, "SecretPeerPod", assert, "Secret has been created and contains data").WithPod(pod).WithSecret(secret).WithTestCommands(testCommands).Run()
}

func DoTestCreatePeerPodContainerWithExternalIPAccess(t *testing.T, e env.Environment, assert CloudAssert) {
	// This test requires a container with the right capability otherwise the following error will be thrown:
	// / # ping 8.8.8.8
	// PING 8.8.8.8 (8.8.8.8): 56 data bytes
	// ping: permission denied (are you root?)
	pod := NewPrivPod(E2eNamespace, "busybox-priv").GetPodOrFatal(t)
	testCommands := []TestCommand{
		{
			Command:       []string{"ping", "-c", "1", "www.google.com"},
			ContainerName: pod.Spec.Containers[0].Name,
			TestCommandStdoutFn: func(stdout bytes.Buffer) bool {
				if stdout.String() != "" {
					t.Logf("Output of ping command in busybox : %s", stdout.String())
					return true
				} else {
					t.Log("No output from ping command")
					return false
				}
			},
			TestCommandStderrFn: IsBufferEmpty,
		},
	}

	NewTestCase(t, e, "IPAccessPeerPod", assert, "Peer Pod Container Connected to External IP").WithPod(pod).WithTestCommands(testCommands).Run()
}

func DoTestCreatePeerPodWithJob(t *testing.T, e env.Environment, assert CloudAssert) {
	jobName := "job-pi"
	image := "quay.io/prometheus/busybox:latest"
	job := NewJob(E2eNamespace, jobName, 8, image)
	expectedPodLogString := "3.14"
	NewTestCase(t, e, "JobPeerPod", assert, "Job has been created").WithJob(job).WithExpectedPodLogString(expectedPodLogString).Run()
}

func DoTestCreatePeerPodAndCheckUserLogs(t *testing.T, e env.Environment, assert CloudAssert) {
	// podName := "user-pod"
	// imageName := "quay.io/confidential-containers/test-images:testuser"
	// pod := NewPod(E2eNamespace, podName, podName, imageName, WithRestartPolicy(v1.RestartPolicyOnFailure))
	// expectedPodLogString := "otheruser"
	// NewTestCase(t, e, "UserPeerPod", assert, "Peer pod with user has been created").WithPod(pod).WithExpectedPodLogString(expectedPodLogString).WithCustomPodState(v1.PodSucceeded).Run()
	t.Skip("Skipping Test until issue kata-containers/kata-containers#5732 is Fixed")
	//Reference - https://github.com/kata-containers/kata-containers/issues/5732
}

// DoTestCreateConfidentialPod verify a confidential peer-pod can be created.
func DoTestCreateConfidentialPod(t *testing.T, e env.Environment, assert CloudAssert, testCommands []TestCommand) {
	pod := NewBusyboxPodWithName(E2eNamespace, "confidential-pod-busybox").GetPodOrFatal(t)
	for i := 0; i < len(testCommands); i++ {
		testCommands[i].ContainerName = pod.Spec.Containers[0].Name
	}

	NewTestCase(t, e, "ConfidentialPodVM", assert, "Confidential PodVM is created").WithPod(pod).WithTestCommands(testCommands).Run()
}

func DoTestCreatePeerPodAndCheckWorkDirLogs(t *testing.T, e env.Environment, assert CloudAssert) {
	podName := "workdirpod"
	imageName := "quay.io/confidential-containers/test-images:testworkdir"
	pod := NewPod(E2eNamespace, podName, podName, imageName, WithRestartPolicy(v1.RestartPolicyOnFailure))
	expectedPodLogString := "/other"
	NewTestCase(t, e, "WorkDirPeerPod", assert, "Peer pod with work directory has been created").WithPod(pod).WithExpectedPodLogString(expectedPodLogString).WithCustomPodState(v1.PodSucceeded).Run()
}

func DoTestCreatePeerPodAndCheckEnvVariableLogsWithImageOnly(t *testing.T, e env.Environment, assert CloudAssert) {
	podName := "env-variable-in-image"
	imageName := "quay.io/confidential-containers/test-images:testenv"
	pod := NewPod(E2eNamespace, podName, podName, imageName, WithRestartPolicy(v1.RestartPolicyOnFailure))
	expectedPodLogString := "ISPRODUCTION=false"
	NewTestCase(t, e, "EnvVariablePeerPodWithImageOnly", assert, "Peer pod with environmental variables has been created").WithPod(pod).WithExpectedPodLogString(expectedPodLogString).WithCustomPodState(v1.PodSucceeded).Run()
}

func DoTestCreatePeerPodAndCheckEnvVariableLogsWithDeploymentOnly(t *testing.T, e env.Environment, assert CloudAssert) {
	podName := "env-variable-in-config"
	imageName := getBusyboxTestImage(t)
	pod := NewPod(E2eNamespace, podName, podName, imageName, WithRestartPolicy(v1.RestartPolicyOnFailure), WithEnvironmentVariables([]v1.EnvVar{{Name: "ISPRODUCTION", Value: "true"}}), WithCommand([]string{"/bin/sh", "-c", "env"}))
	expectedPodLogString := "ISPRODUCTION=true"
	NewTestCase(t, e, "EnvVariablePeerPodWithDeploymentOnly", assert, "Peer pod with environmental variables has been created").WithPod(pod).WithExpectedPodLogString(expectedPodLogString).WithCustomPodState(v1.PodSucceeded).Run()
}

func DoTestCreatePeerPodAndCheckEnvVariableLogsWithImageAndDeployment(t *testing.T, e env.Environment, assert CloudAssert) {
	podName := "env-variable-in-both"
	imageName := "quay.io/confidential-containers/test-images:testenv"
	pod := NewPod(E2eNamespace, podName, podName, imageName, WithRestartPolicy(v1.RestartPolicyOnFailure), WithEnvironmentVariables([]v1.EnvVar{{Name: "ISPRODUCTION", Value: "true"}}))
	expectedPodLogString := "ISPRODUCTION=true"
	NewTestCase(t, e, "EnvVariablePeerPodWithBoth", assert, "Peer pod with environmental variables has been created").WithPod(pod).WithExpectedPodLogString(expectedPodLogString).WithCustomPodState(v1.PodSucceeded).Run()
}

func DoTestCreatePeerPodWithLargeImage(t *testing.T, e env.Environment, assert CloudAssert) {
	podName := "largeimage-pod"
	imageName := "quay.io/confidential-containers/test-images:largeimage"
	// Need more timeout to pull large image data
	timeout := "300"
	annotationData := map[string]string{
		"io.katacontainers.config.runtime.create_container_timeout": timeout,
	}
	pod := NewPod(E2eNamespace, podName, podName, imageName, WithRestartPolicy(v1.RestartPolicyOnFailure), WithAnnotations(annotationData))
	NewTestCase(t, e, "LargeImagePeerPod", assert, "Peer pod with Large Image has been created").WithPod(pod).WithPodWatcher().Run()
}

func DoTestCreatePeerPodWithPVCAndCSIWrapper(t *testing.T, e env.Environment, assert CloudAssert, myPVC *v1.PersistentVolumeClaim, pod *v1.Pod, mountPath string) {
	testCommands := []TestCommand{
		{
			Command:       []string{"lsblk"},
			ContainerName: pod.Spec.Containers[2].Name,
			TestCommandStdoutFn: func(stdout bytes.Buffer) bool {
				if strings.Contains(stdout.String(), mountPath) {
					t.Logf("PVC volume is mounted correctly: %s", stdout.String())
					return true
				} else {
					t.Errorf("PVC volume failed to be mounted at target path: %s", stdout.String())
					return false
				}
			},
			TestCommandStderrFn: IsBufferEmpty,
		},
	}
	NewTestCase(t, e, "PeerPodWithPVCAndCSIWrapper", assert, "PVC is created and mounted as expected").WithPod(pod).WithPVC(myPVC).WithTestCommands(testCommands).Run()
}

func DoTestCreatePeerPodWithAuthenticatedImageWithImagePullSecretInServiceAccount(t *testing.T, e env.Environment, assert CloudAssert) {
	randseed := rand.New(rand.NewSource(time.Now().UnixNano()))
	podName := "authenticated-image-with-creds-" + strconv.Itoa(int(randseed.Uint32())) + "-pod"

	imageName := os.Getenv("AUTHENTICATED_REGISTRY_IMAGE")
	cred := os.Getenv("REGISTRY_CREDENTIAL_ENCODED")
	secretName := "regcred"
	regcredSecret := NewImagePullSecret(E2eNamespace, secretName, imageName, cred)

	pod := NewPod(E2eNamespace, podName, podName, imageName, WithRestartPolicy(v1.RestartPolicyNever))
	NewTestCase(t, e, "ValidAuthImagePeerPod", assert, "Peer pod with Authenticated Image with imagePullSecret in service account has been created").WithPod(pod).WithSecret(regcredSecret).WithSAImagePullSecret(secretName).WithCustomPodState(v1.PodRunning).Run()
}

func DoTestCreatePeerPodWithAuthenticatedImageWithImagePullSecretOnPod(t *testing.T, e env.Environment, assert CloudAssert) {
	randseed := rand.New(rand.NewSource(time.Now().UnixNano()))
	podName := "authenticated-image-with-creds-" + strconv.Itoa(int(randseed.Uint32())) + "-pod"

	imageName := os.Getenv("AUTHENTICATED_REGISTRY_IMAGE")
	cred := os.Getenv("REGISTRY_CREDENTIAL_ENCODED")
	secretName := "regcred"
	regcredSecret := NewImagePullSecret(E2eNamespace, secretName, imageName, cred)

	pod := NewPod(E2eNamespace, podName, podName, imageName, WithRestartPolicy(v1.RestartPolicyNever), WithImagePullSecrets(secretName))
	NewTestCase(t, e, "ValidAuthImagePeerPod", assert, "Peer pod with Authenticated Image with imagePullSecret in pod spec has been created").WithPod(pod).WithSecret(regcredSecret).WithCustomPodState(v1.PodRunning).Run()
}

// Check that without creds the image can't be pulled to ensure we don't have a false positive in our auth test
func DoTestCreatePeerPodWithAuthenticatedImageWithoutCredentials(t *testing.T, e env.Environment, assert CloudAssert) {
	randseed := rand.New(rand.NewSource(time.Now().UnixNano()))
	podName := "authenticated-image-without-creds-" + strconv.Itoa(int(randseed.Uint32())) + "-pod"
	imageName := os.Getenv("AUTHENTICATED_REGISTRY_IMAGE")
	pod := NewPod(E2eNamespace, podName, podName, imageName, WithRestartPolicy(v1.RestartPolicyNever))
	expectedErrorString := "401 UNAUTHORIZED"
	if isTestOnCrio() {
		expectedErrorString = "access to the requested resource is not authorized"
	}
	NewTestCase(t, e, "InvalidAuthImagePeerPod", assert, "Peer pod with Authenticated Image without Credentials has been created").WithPod(pod).WithExpectedPodEventError(expectedErrorString).WithCustomPodState(v1.PodPending).Run()
}

func DoTestPodVMwithNoAnnotations(t *testing.T, e env.Environment, assert CloudAssert, expectedType string) {

	podName := "no-annotations"
	containerName := "busybox"
	imageName := getBusyboxTestImage(t)
	pod := NewPod(E2eNamespace, podName, containerName, imageName, WithCommand([]string{"/bin/sh", "-c", "sleep 3600"}))
	NewTestCase(t, e, "PodVMWithNoAnnotations", assert, "PodVM with No Annotation is created").WithPod(pod).WithExpectedInstanceType(expectedType).Run()
}

func DoTestPodVMwithAnnotationsInstanceType(t *testing.T, e env.Environment, assert CloudAssert, expectedType string) {
	podName := "annotations-instance-type"
	containerName := "busybox"
	imageName := getBusyboxTestImage(t)
	annotationData := map[string]string{
		"io.katacontainers.config.hypervisor.machine_type": expectedType,
	}
	pod := NewPod(E2eNamespace, podName, containerName, imageName, WithCommand([]string{"/bin/sh", "-c", "sleep 3600"}), WithAnnotations(annotationData))
	NewTestCase(t, e, "PodVMwithAnnotationsInstanceType", assert, "PodVM with Annotation is created").WithPod(pod).WithExpectedInstanceType(expectedType).Run()
}

func DoTestPodVMwithAnnotationsCPUMemory(t *testing.T, e env.Environment, assert CloudAssert, expectedType string) {
	podName := "annotations-cpu-mem"
	containerName := "busybox"
	imageName := getBusyboxTestImage(t)
	annotationData := map[string]string{
		"io.katacontainers.config.hypervisor.default_vcpus":  "2",
		"io.katacontainers.config.hypervisor.default_memory": "12288",
	}
	pod := NewPod(E2eNamespace, podName, containerName, imageName, WithCommand([]string{"/bin/sh", "-c", "sleep 3600"}), WithAnnotations(annotationData))
	NewTestCase(t, e, "PodVMwithAnnotationsCPUMemory", assert, "PodVM with Annotations CPU Memory is created").WithPod(pod).WithExpectedInstanceType(expectedType).Run()
}

func DoTestPodVMwithAnnotationsInvalidInstanceType(t *testing.T, e env.Environment, assert CloudAssert, expectedType string) {
	podName := "annotations-invalid-instance-type"
	containerName := "busybox"
	imageName := getBusyboxTestImage(t)
	annotationData := map[string]string{
		"io.katacontainers.config.hypervisor.machine_type": expectedType,
	}
	pod := NewPod(E2eNamespace, podName, containerName, imageName, WithCommand([]string{"/bin/sh", "-c", "sleep 3600"}), WithAnnotations(annotationData))
	expectedErrorMessage := `requested instance type ("` + expectedType + `") is not part of supported instance types list`
	NewTestCase(t, e, "PodVMwithAnnotationsInvalidInstanceType", assert, "Failed to Create PodVM with Annotations Invalid InstanceType").WithPod(pod).WithExpectedPodEventError(expectedErrorMessage).WithCustomPodState(v1.PodFailed).Run()
}

func DoTestPodVMwithAnnotationsLargerMemory(t *testing.T, e env.Environment, assert CloudAssert) {
	podName := "annotations-too-big-mem"
	containerName := "busybox"
	imageName := getBusyboxTestImage(t)
	annotationData := map[string]string{
		"io.katacontainers.config.hypervisor.default_vcpus":  "2",
		"io.katacontainers.config.hypervisor.default_memory": "18432",
	}
	pod := NewPod(E2eNamespace, podName, containerName, imageName, WithCommand([]string{"/bin/sh", "-c", "sleep 3600"}), WithAnnotations(annotationData))
	expectedErrorMessage := "failed to get instance type based on vCPU and memory annotations: no instance type found for the given vcpus (2) and memory (18432)"
	NewTestCase(t, e, "PodVMwithAnnotationsLargerMemory", assert, "Failed to Create PodVM with Annotations Larger Memory").WithPod(pod).WithExpectedPodEventError(expectedErrorMessage).WithCustomPodState(v1.PodFailed).Run()
}

func DoTestPodVMwithAnnotationsLargerCPU(t *testing.T, e env.Environment, assert CloudAssert) {
	podName := "annotations-too-big-cpu"
	containerName := "busybox"
	imageName := getBusyboxTestImage(t)
	annotationData := map[string]string{
		"io.katacontainers.config.hypervisor.default_vcpus":  "3",
		"io.katacontainers.config.hypervisor.default_memory": "12288",
	}
	pod := NewPod(E2eNamespace, podName, containerName, imageName, WithCommand([]string{"/bin/sh", "-c", "sleep 3600"}), WithAnnotations(annotationData))
	expectedErrorMessage := "no instance type found for the given vcpus (3) and memory (12288)"
	NewTestCase(t, e, "PodVMwithAnnotationsLargerCPU", assert, "Failed to Create PodVM with Annotations Larger CPU").WithPod(pod).WithExpectedPodEventError(expectedErrorMessage).WithCustomPodState(v1.PodFailed).Run()
}

func DoTestCreatePeerPodContainerWithValidAlternateImage(t *testing.T, e env.Environment, assert CloudAssert, alternateImageName string) {
	podName := "annotations-valid-alternate-image"
	containerName := "busybox"
	imageName := getBusyboxTestImage(t)
	annotationData := map[string]string{
		"io.katacontainers.config.hypervisor.image": alternateImageName,
	}
	pod := NewPod(E2eNamespace, podName, containerName, imageName, WithCommand([]string{"/bin/sh", "-c", "sleep 3600"}), WithAnnotations(annotationData))

	NewTestCase(t, e, "PodVMwithAnnotationsValidAlternateImage", assert, "PodVM created with an alternate image").WithPod(pod).WithExpectedCaaPodLogStrings("Choosing " + alternateImageName).Run()
}

func DoTestCreatePeerPodContainerWithInvalidAlternateImage(t *testing.T, e env.Environment, assert CloudAssert,
	nonExistingImageName, expectedErrorMessage string) {

	podName := "annotations-invalid-alternate-image"
	containerName := "busybox"
	imageName := getBusyboxTestImage(t)
	annotationData := map[string]string{
		"io.katacontainers.config.hypervisor.image": nonExistingImageName,
	}
	pod := NewPod(E2eNamespace, podName, containerName, imageName, WithCommand([]string{"/bin/sh", "-c", "sleep 3600"}), WithAnnotations(annotationData))
	NewTestCase(t, e, "PodVMwithAnnotationsInvalidAlternateImage", assert, "Failed to Create PodVM with a non-existent image").WithPod(pod).WithExpectedPodEventError(expectedErrorMessage).WithCustomPodState(v1.PodPending).Run()
}

func DoTestPodToServiceCommunication(t *testing.T, e env.Environment, assert CloudAssert) {
	clientPodName := "test-client"
	clientContainerName := "busybox"
	clientImageName := getBusyboxTestImage(t)
	serverPodName := "test-server"
	serverContainerName := "nginx"
	serverImageName, err := utils.GetImage("nginx")
	if err != nil {
		t.Fatal(err)
	}
	serviceName := "nginx-server"
	labels := map[string]string{
		"app": "nginx-server",
	}
	clientPod := NewExtraPod(E2eNamespace, clientPodName, clientContainerName, clientImageName, WithCommand([]string{"/bin/sh", "-c", "sleep 3600"}), WithRestartPolicy(v1.RestartPolicyNever))
	serverPod := NewPod(E2eNamespace, serverPodName, serverContainerName, serverImageName, WithContainerPort(80), WithRestartPolicy(v1.RestartPolicyNever), WithLabel(labels))
	testCommands := []TestCommand{
		{
			Command:       []string{"wget", "-O-", "nginx-server"},
			ContainerName: clientPod.pod.Spec.Containers[0].Name,
			TestCommandStdoutFn: func(stdout bytes.Buffer) bool {
				if strings.Contains(stdout.String(), "Thank you for using nginx") {
					t.Logf("Success to access nginx service. %s", stdout.String())
					return true
				} else {
					t.Errorf("Failed to access nginx service: %s", stdout.String())
					return false
				}
			},
		},
	}
	clientPod.WithTestCommands(testCommands)
	httpPort := v1.ServicePort{
		Name:       "http",
		Port:       80,
		TargetPort: intstr.FromInt(int(80)),
		Protocol:   v1.ProtocolTCP,
	}
	servicePorts := []v1.ServicePort{httpPort}
	nginxSvc := NewService(E2eNamespace, serviceName, servicePorts, labels)
	extraPods := []*ExtraPod{clientPod}
	NewTestCase(t, e, "TestExtraPods", assert, "Failed to test extra pod.").WithPod(serverPod).WithExtraPods(extraPods).WithService(nginxSvc).Run()
}

func DoTestPodsMTLSCommunication(t *testing.T, e env.Environment, assert CloudAssert) {
	clientPodName := "mtls-client"
	clientContainerName := "curl"
	clientImageName, err := utils.GetImage("curl")
	if err != nil {
		t.Fatal(err)
	}
	serverPodName := "mtls-server"
	serverContainerName := "nginx"
	serverImageName, err := utils.GetImage("nginx")
	if err != nil {
		t.Fatal(err)
	}
	caService, _ := tlsutil.NewCAService("nginx")
	serverCACertPEM := caService.RootCertificate()
	serviceName := "nginx-mtls"
	serverCertPEM, serverKeyPEM, _ := caService.Issue(serviceName)
	clientCertPEM, clientKeyPEM, _ := tlsutil.NewClientCertificate("curl")
	clientSecretDir := "/etc/certs"
	serverSecretDir := "/etc/nginx/certs"
	clientSecretValue := string(clientKeyPEM)
	serverSecretValue := string(serverKeyPEM)
	clientSecretName := "curl-certs"
	serverSecretName := "server-certs"
	serverSecretData := map[string][]byte{"tls.key": []byte(serverSecretValue), "tls.crt": []byte(serverCertPEM), "ca.crt": []byte(serverCACertPEM)}
	clientSecretData := map[string][]byte{"tls.key": []byte(clientSecretValue), "tls.crt": []byte(clientCertPEM), "ca.crt": []byte(serverCACertPEM)}
	serverSecret := NewSecret(E2eNamespace, serverSecretName, serverSecretData, v1.SecretTypeOpaque)
	clientSecret := NewSecret(E2eNamespace, clientSecretName, clientSecretData, v1.SecretTypeOpaque)
	clientPod := NewExtraPod(
		E2eNamespace, clientPodName, clientContainerName, clientImageName,
		WithSecretBinding(t, clientSecretDir, clientSecretName, clientContainerName),
		WithRestartPolicy(v1.RestartPolicyNever),
		WithCommand([]string{"/bin/sh", "-c", "sleep 3600"}),
	)

	configMapName := "nginx-conf"
	configMapFileName := "nginx.conf"
	podKubeConfigmapDir := "/etc/nginx"
	configMapData := map[string]string{
		configMapFileName: `
			worker_processes auto;
			events {
			}
			http{
			  server {
				listen                 80;
				return 301 https://$host$request_uri;
			  }
			  server {
				listen 443 ssl;

				root /usr/share/nginx/html;
				index index.html;

				server_name nginx.default.svc.cluster.local;
				ssl_certificate /etc/nginx/certs/tls.crt;
				ssl_certificate_key /etc/nginx/certs/tls.key;

				location / {
					try_files $uri $uri/ =404;
				}
			  }
		    }
			`,
	}
	labels := map[string]string{
		"app": "mtls-server",
	}
	serverPod := NewPod(E2eNamespace, serverPodName, serverContainerName, serverImageName, WithSecureContainerPort(443), WithSecretBinding(t, serverSecretDir, serverSecretName, serverContainerName), WithLabel(labels), WithConfigMapBinding(podKubeConfigmapDir, configMapName))
	configMap := NewConfigMap(E2eNamespace, configMapName, configMapData)

	serviceUrl := fmt.Sprintf("https://%s", serviceName)
	testCommands := []TestCommand{
		{
			Command:       []string{"curl", "--key", "/etc/certs/tls.key", "--cert", "/etc/certs/tls.crt", "--cacert", "/etc/certs/ca.crt", serviceUrl},
			ContainerName: clientPod.pod.Spec.Containers[0].Name,
			TestCommandStdoutFn: func(stdout bytes.Buffer) bool {
				if strings.Contains(stdout.String(), "Thank you for using nginx") {
					t.Logf("Success to access nginx service. %s", stdout.String())
					return true
				} else {
					t.Errorf("Failed to access nginx service: %s", stdout.String())
					return false
				}
			},
		},
	}
	clientPod.WithTestCommands(testCommands)
	httpsPort := v1.ServicePort{
		Name:       "https",
		Port:       443,
		TargetPort: intstr.FromInt(int(443)),
		Protocol:   v1.ProtocolTCP,
	}
	servicePorts := []v1.ServicePort{httpsPort}
	nginxSvc := NewService(E2eNamespace, serviceName, servicePorts, labels)
	extraPods := []*ExtraPod{clientPod}
	extraSecrets := []*v1.Secret{clientSecret}
	NewTestCase(t, e, "TestPodsMTLSCommunication", assert, "Pods communication with mTLS").WithPod(serverPod).WithExtraPods(extraPods).WithConfigMap(configMap).WithService(nginxSvc).WithSecret(serverSecret).WithExtraSecrets(extraSecrets).Run()

}

func DoTestImageDecryption(t *testing.T, e env.Environment, assert CloudAssert, kbs *pv.KeyBrokerService) {
	// TODO create a multi-arch encrypted image. Note the Kata CI version doesn't work as the key length is 44, not 32 which is wanted
	if runtime.GOARCH == "s390x" {
		t.Skip("Encrypted image test not currently support on s390x")
	}

	image := "ghcr.io/confidential-containers/cloud-api-adaptor/nginx-encrypted:20240123"
	var kbsEndpoint string
	if ep := os.Getenv("KBS_ENDPOINT"); ep != "" {
		kbsEndpoint = ep
	} else if kbs == nil {
		t.Skip("Skipping because KBS config is missing")
	} else {
		// skopeo inspect \
		//   docker://ghcr.io/confidential-containers/cloud-api-adaptor/nginx-encrypted:20240123 \
		//   | jq .Labels
		// {
		//   "coco-key-b64": "pHSE5N+T/3GGfb/umaWgB8bfHc/dQWvmxdsjoWam0Vs=",
		//   "coco-key-id": "default/key/nginx-encrypted",
		// }
		keyID := "default/key/nginx-encrypted"
		key := []byte{
			164, 116, 132, 228, 223, 147, 255, 113, 134, 125,
			191, 238, 153, 165, 160, 7, 198, 223, 29, 207,
			221, 65, 107, 230, 197, 219, 35, 161, 102, 166,
			209, 91}

		err := kbs.SetImageDecryptionKey(keyID, key)
		if err != nil {
			t.Fatalf("Failed to set image decryption key: %v", err)
		}
		err = kbs.EnableKbsCustomizedResourcePolicy("allow_all.rego")
		if err != nil {
			t.Fatalf("Failed to enable KBS customized resource policy: %v", err)
		}
		kbsEndpoint, err = kbs.GetCachedKbsEndpoint()
		if err != nil {
			t.Fatalf("Failed to get KBS endpoint: %v", err)
		}
	}
	podName := "nginx-encrypted"
	// encrypted images need this for the time being
	annotations := map[string]string{"io.containerd.cri.runtime-handler": "kata-remote"}
	pod := NewPod(E2eNamespace, podName, podName, image, WithAnnotations(annotations), WithInitdata(kbsEndpoint))
	duration := 3 * time.Minute
	NewTestCase(t, e, "TestImageDecryption", assert, "Encrypted image layers have been decrypted").WithPod(pod).WithDeleteAssertion(&duration).Run()
}

func DoTestSealedSecret(t *testing.T, e env.Environment, assert CloudAssert, kbsEndpoint string, resourcePath, expectedSecret string) {
	key := "MY_SECRET"
	value := CreateSealedSecretValue("kbs:///" + resourcePath)
	podName := "sealed-secret"
	imageName := getBusyboxTestImage(t)
	env := []v1.EnvVar{{Name: key, Value: value}}
	cmd := []string{"watch", "-n", "120", "-t", "--", "printenv MY_SECRET"}

	pod := NewPod(E2eNamespace, podName, podName, imageName, WithEnvironmentVariables(env), WithInitdata(kbsEndpoint), WithCommand(cmd))

	NewTestCase(t, e, "TestSealedSecret", assert, "Unsealed secret has been set to ENV").WithPod(pod).WithExpectedPodLogString(expectedSecret).Run()
}

// DoTestKbsKeyRelease and DoTestKbsKeyReleaseForFailure should be run in a single test case if you're chaining opa in kbs
// as test cases might be run in parallel
func DoTestKbsKeyRelease(t *testing.T, e env.Environment, assert CloudAssert, kbsEndpoint, resourcePath, expectedSecret string) {
	t.Log("Do test https kbs key release")
	pod := NewBusyboxPodWithNameWithInitdata(E2eNamespace, "kbs-key-release", kbsEndpoint, testInitdata).GetPodOrFatal(t)
	testCommands := []TestCommand{
		{
			Command:       []string{"wget", "-q", "-O-", "http://127.0.0.1:8006/cdh/resource/" + resourcePath},
			ContainerName: pod.Spec.Containers[0].Name,
			TestCommandStdoutFn: func(stdout bytes.Buffer) bool {
				if strings.Contains(stdout.String(), expectedSecret) {
					t.Logf("Success to get secret key: %s", stdout.String())
					return true
				} else {
					t.Errorf("Failed to access secret key: %s", stdout.String())
					return false
				}
			},
		},
	}

	NewTestCase(t, e, "KbsKeyReleasePod", assert, "Kbs key release is successful").WithPod(pod).WithTestCommands(testCommands).WithExpectedPodvmConsoleLog("error").Run()
}

// DoTestKbsKeyRelease and DoTestKbsKeyReleaseForFailure should be run in a single test case if you're chaining opa in kbs
// as test cases might be run in parallel
func DoTestKbsKeyReleaseForFailure(t *testing.T, e env.Environment, assert CloudAssert, kbsEndpoint, resourcePath, expectedSecret string) {
	t.Log("Do test kbs key release failure case")
	pod := NewBusyboxPodWithNameWithInitdata(E2eNamespace, "kbs-failure", kbsEndpoint, testInitdata).GetPodOrFatal(t)
	testCommands := []TestCommand{
		{
			Command:       []string{"wget", "-q", "-O-", "http://127.0.0.1:8006/cdh/resource/" + resourcePath},
			ContainerName: pod.Spec.Containers[0].Name,
			TestErrorFn: func(err error) bool {
				if strings.Contains(err.Error(), "command terminated with exit code 1") {
					return true
				} else {
					t.Errorf("Got unexpected error: %s", err.Error())
					return false
				}
			},
			TestCommandStdoutFn: func(stdout bytes.Buffer) bool {
				if strings.Contains(stdout.String(), expectedSecret) {
					t.Errorf("FAIL as succeed to get secret key: %s", stdout.String())
					return false
				} else {
					t.Logf("PASS as failed to access secret key: %s", stdout.String())
					return true
				}
			},
		},
	}

	NewTestCase(t, e, "DoTestKbsKeyReleaseForFailure", assert, "Kbs key release is failed").WithPod(pod).WithTestCommands(testCommands).Run()
}

func DoTestRestrictivePolicyBlocksExec(t *testing.T, e env.Environment, assert CloudAssert) {
	allowAllExceptExecPolicyFilePath := "fixtures/policies/allow-all-except-exec-process.rego"
	podName := "policy-exec-rejected"
	pod := NewPodWithPolicy(E2eNamespace, podName, allowAllExceptExecPolicyFilePath).GetPodOrFatal(t)

	testCommands := []TestCommand{
		{
			Command:       []string{"ls"},
			ContainerName: pod.Spec.Containers[0].Name,
			TestErrorFn: func(err error) bool {
				if (strings.Contains(err.Error(), "failed to exec in container") || // containerd
					strings.Contains(err.Error(), "error executing command in container")) && // cri-o
					strings.Contains(err.Error(), "ExecProcessRequest is blocked by policy") {
					t.Logf("Exec process was blocked: %s", err.Error())
					return true
				} else {
					t.Errorf("Exec process was allowed: %s", err.Error())
					return false
				}
			},
		},
	}
	NewTestCase(t, e, "PodVMwithPolicyBlockingExec", assert, "Pod which blocks Exec Process").WithPod(pod).WithTestCommands(testCommands).Run()
}

func DoTestPermissivePolicyAllowsExec(t *testing.T, e env.Environment, assert CloudAssert) {
	allowAllPolicyFilePath := "fixtures/policies/allow-all.rego"
	podName := "policy-all-allowed"
	pod := NewPodWithPolicy(E2eNamespace, podName, allowAllPolicyFilePath).GetPodOrFatal(t)

	// Just check there are no errors and something returned
	testCommands := []TestCommand{
		{
			Command:       []string{"ls"},
			ContainerName: pod.Spec.Containers[0].Name,
			TestCommandStdoutFn: func(stdout bytes.Buffer) bool {
				return stdout.Len() > 0
			},
			TestCommandStderrFn: IsBufferEmpty,
		},
	}
	NewTestCase(t, e, "PodVMwithPermissivePolicy", assert, "Pod which allows all kata agent APIs").WithPod(pod).WithTestCommands(testCommands).Run()
}

// Test to run pod with io.Kubernetes.cri-o.Devices annotation and check the devices are created in the pod
func DoTestPodWithCrioDeviceAnnotation(t *testing.T, e env.Environment, assert CloudAssert) {
	podName := "pod-with-devices"
	containerName := "busybox"
	imageName := getBusyboxTestImage(t)
	devicesAnnotation := map[string]string{
		"io.kubernetes.cri-o.Devices": "/dev/fuse",
	}
	pod := NewPod(E2eNamespace, podName, containerName, imageName, WithRestartPolicy(v1.RestartPolicyNever), WithAnnotations(devicesAnnotation), WithCommand([]string{"/bin/sh", "-c", "sleep 3600"}))

	testCommands := []TestCommand{
		{
			Command:       []string{"ls", "/dev/fuse"},
			ContainerName: pod.Spec.Containers[0].Name,
			TestCommandStdoutFn: func(stdout bytes.Buffer) bool {
				if strings.Contains(stdout.String(), "/dev/fuse") {
					t.Logf("Device /dev/fuse is created in the pod")
					return true
				} else {
					t.Errorf("Device /dev/fuse is not created in the pod")
					return false
				}
			},
			TestCommandStderrFn: IsBufferEmpty,
		},
	}

	NewTestCase(t, e, "PodWithDevicesAnnotation", assert, "Pod with devices annotation").WithPod(pod).WithTestCommands(testCommands).Run()
}

// Test to run pod with incorrect annotation and check the devices are not created in the pod
func DoTestPodWithIncorrectCrioDeviceAnnotation(t *testing.T, e env.Environment, assert CloudAssert) {
	podName := "pod-with-devices"
	containerName := "busybox"
	imageName := getBusyboxTestImage(t)
	devicesAnnotation := map[string]string{
		"io.kubernetes.cri.Dev": "/dev/fuse",
	}
	pod := NewPod(E2eNamespace, podName, containerName, imageName, WithRestartPolicy(v1.RestartPolicyNever), WithAnnotations(devicesAnnotation), WithCommand([]string{"/bin/sh", "-c", "sleep 3600"}))

	testCommands := []TestCommand{
		{
			Command:             []string{"ls", "/dev/fuse"},
			ContainerName:       pod.Spec.Containers[0].Name,
			TestCommandStdoutFn: IsBufferEmpty,
			TestCommandStderrFn: func(stderr bytes.Buffer) bool {
				if strings.Contains(stderr.String(), "No such file or directory") {
					t.Logf("Device /dev/fuse is not created in the pod")
					return true
				} else {
					t.Errorf("Device /dev/fuse is created in the pod")
					return false
				}
			},
			// The command should throw the following error
			// "command terminated with exit code 1"
			TestErrorFn: func(err error) bool {
				if strings.Contains(err.Error(), "command terminated with exit code 1") {
					t.Logf("Command terminated with exit code 1")
					return true
				} else {
					t.Errorf("Command did not terminate with exit code 1")
					return false
				}

			},
		},
	}

	NewTestCase(t, e, "PodWithIncorrectDevicesAnnotation", assert, "Pod with incorrect devices annotation").WithPod(pod).WithTestCommands(testCommands).Run()
}

// Test to run a pod with init container and check the init container is executed successfully
func DoTestPodWithInitContainer(t *testing.T, e env.Environment, assert CloudAssert) {

	pod := NewPodWithInitContainer(E2eNamespace, "pod-with-init-container").GetPodOrFatal(t)

	NewTestCase(t, e, "PodWithInitContainer", assert, "Pod with init container").WithPod(pod).Run()

}

// Test to run specific commands in a pod and check the output
func DoTestPodWithSpecificCommands(t *testing.T, e env.Environment, assert CloudAssert, testCommands []TestCommand) {
	pod := NewBusyboxPodWithName(E2eNamespace, "command-test").GetPodOrFatal(t)

	NewTestCase(t, e, "PodWithSpecificCommands", assert, "Pod with specific commands").WithPod(pod).WithTestCommands(testCommands).Run()
}

// Test to run a pod with cpu and memory limits and requests
func DoTestPodWithCpuMemLimitsAndRequests(t *testing.T, e env.Environment, assert CloudAssert, cpuRequest, memRequest, cpuLimit, memLimit string) {
	imageName := getBusyboxTestImage(t)
	pod := NewPod(E2eNamespace, "pod-with-cpu-mem-limits-requests", "busybox", imageName,
		WithCpuMemRequestAndLimit(cpuRequest, memRequest, cpuLimit, memLimit))

	// Add testCommands to check that request/limit are removed from the spec and following annotations
	// to pod spec is added
	// io.katacontainers.config.hypervisor.default_cpus
	// io.katacontainers.config.hypervisor.default_memory
	// Custom resource added as req/limit - "kata.peerpods.io/vm

	NewTestCase(t, e, "PodWithCpuMemLimitsAndRequests", assert, "Pod with cpu and memory limits and requests").WithPod(pod).Run()
}

// Test to create a peer pod with cpu request as annotation
func DoTestPodVMwithAnnotationCPU(t *testing.T, e env.Environment, assert CloudAssert, expectedType string) {

	podName := "annotations-cpu"
	containerName := "busybox"
	imageName := getBusyboxTestImage(t)
	annotationData := map[string]string{
		"io.katacontainers.config.hypervisor.default_vcpus": "4",
	}
	pod := NewPod(E2eNamespace, podName, containerName, imageName, WithCommand([]string{"/bin/sh", "-c", "sleep 3600"}), WithAnnotations(annotationData))
	NewTestCase(t, e, "PodVMwithAnnotationCPU", assert, "PodVM with Annotation CPU is created").WithPod(pod).WithExpectedInstanceType(expectedType).Run()
}

// Test to create a peer pod with memory request as annotation
func DoTestPodVMwithAnnotationMemory(t *testing.T, e env.Environment, assert CloudAssert, expectedType string) {

	podName := "annotations-mem"
	containerName := "busybox"
	imageName := getBusyboxTestImage(t)
	annotationData := map[string]string{
		"io.katacontainers.config.hypervisor.default_memory": "7168",
	}
	pod := NewPod(E2eNamespace, podName, containerName, imageName, WithCommand([]string{"/bin/sh", "-c", "sleep 3600"}), WithAnnotations(annotationData))
	NewTestCase(t, e, "PodVMwithAnnotationMemory", assert, "PodVM with Annotation Memory is created").WithPod(pod).WithExpectedInstanceType(expectedType).Run()
}
