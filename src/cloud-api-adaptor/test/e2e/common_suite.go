// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"bytes"
	"encoding/json"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/pkg/util/tlsutil"
	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/e2e-framework/pkg/env"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
)

var E2eNamespace = envconf.RandomName("coco-pp-e2e-test", 25)

// DoTestCreateSimplePod tests a simple peer-pod can be created.
func DoTestCreateSimplePod(t *testing.T, e env.Environment, assert CloudAssert) {
	pod := NewBusyboxPodWithName(E2eNamespace, "simple-test")
	NewTestCase(t, e, "SimplePeerPod", assert, "PodVM is created").WithPod(pod).Run()
}

func DoTestCreateSimplePodWithNydusAnnotation(t *testing.T, e env.Environment, assert CloudAssert) {
	annotationData := map[string]string{
		"io.containerd.cri.runtime-handler": "kata-remote",
	}
	pod := NewPod(E2eNamespace, "alpine", "alpine", "alpine", WithRestartPolicy(v1.RestartPolicyNever), WithAnnotations(annotationData))
	NewTestCase(t, e, "SimplePeerPod", assert, "PodVM is created").WithPod(pod).WithNydusSnapshotter().Run()
}

func DoTestDeleteSimplePod(t *testing.T, e env.Environment, assert CloudAssert) {
	pod := NewBusyboxPodWithName(E2eNamespace, "deletion-test")
	duration := assert.DefaultTimeout()
	NewTestCase(t, e, "DeletePod", assert, "Deletion complete").WithPod(pod).WithDeleteAssertion(&duration).Run()
}

func DoTestCreatePodWithConfigMap(t *testing.T, e env.Environment, assert CloudAssert) {
	podName := "busybox-configmap-pod"
	containerName := "busybox-configmap-container"
	imageName := BUSYBOX_IMAGE
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
					log.Infof("Data Inside Configmap: %s", stdout.String())
					return true
				} else {
					log.Errorf("Configmap has invalid Data: %s", stdout.String())
					return false
				}
			},
			TestCommandStderrFn: IsBufferEmpty,
			TestErrorFn:         IsErrorEmpty,
		},
	}

	NewTestCase(t, e, "ConfigMapPeerPod", assert, "Configmap is created and contains data").WithPod(pod).WithConfigMap(configMap).WithTestCommands(testCommands).Run()
}

func DoTestCreatePodWithSecret(t *testing.T, e env.Environment, assert CloudAssert) {
	//DoTestCreatePod(t, assert, "Secret is created and contains data", pod)
	podName := "busybox-secret-pod"
	containerName := "busybox-secret-container"
	imageName := BUSYBOX_IMAGE
	secretName := "busybox-secret"
	podKubeSecretsDir := "/etc/secret/"
	usernameFileName := "username"
	username := "admin"
	usernamePath := podKubeSecretsDir + usernameFileName
	passwordFileName := "password"
	password := "password"
	passwordPath := podKubeSecretsDir + passwordFileName
	secretData := map[string][]byte{passwordFileName: []byte(password), usernameFileName: []byte(username)}
	pod := NewPod(E2eNamespace, podName, containerName, imageName, WithSecretBinding(podKubeSecretsDir, secretName), WithCommand([]string{"/bin/sh", "-c", "sleep 3600"}))
	secret := NewSecret(E2eNamespace, secretName, secretData, v1.SecretTypeOpaque)

	testCommands := []TestCommand{
		{
			Command:       []string{"cat", usernamePath},
			ContainerName: pod.Spec.Containers[0].Name,
			TestCommandStdoutFn: func(stdout bytes.Buffer) bool {
				if stdout.String() == username {
					log.Infof("Username from secret inside pod: %s", stdout.String())
					return true
				} else {
					log.Errorf("Username value from secret inside pod unexpected. Expected %s, got %s", username, stdout.String())
					return false
				}
			},
			TestCommandStderrFn: IsBufferEmpty,
			TestErrorFn:         IsErrorEmpty,
		},
		{
			Command:       []string{"cat", passwordPath},
			ContainerName: pod.Spec.Containers[0].Name,
			TestCommandStdoutFn: func(stdout bytes.Buffer) bool {
				if stdout.String() == password {
					log.Infof("Password from secret inside pod: %s", stdout.String())
					return true
				} else {
					log.Errorf("Password value from secret inside pod unexpected. Expected %s, got %s", password, stdout.String())
					return false
				}
			},
			TestCommandStderrFn: IsBufferEmpty,
			TestErrorFn:         IsErrorEmpty,
		},
	}

	NewTestCase(t, e, "SecretPeerPod", assert, "Secret has been created and contains data").WithPod(pod).WithSecret(secret).WithTestCommands(testCommands).Run()
}

func DoTestCreatePeerPodContainerWithExternalIPAccess(t *testing.T, e env.Environment, assert CloudAssert) {
	pod := NewBusyboxPod(E2eNamespace)
	testCommands := []TestCommand{
		{
			Command:       []string{"ping", "-c", "1", "www.google.com"},
			ContainerName: pod.Spec.Containers[0].Name,
			TestCommandStdoutFn: func(stdout bytes.Buffer) bool {
				if stdout.String() != "" {
					log.Infof("Output of ping command in busybox : %s", stdout.String())
					return true
				} else {
					log.Info("No output from ping command")
					return false
				}
			},
			TestCommandStderrFn: IsBufferEmpty,
			TestErrorFn:         IsErrorEmpty,
		},
	}

	NewTestCase(t, e, "IPAccessPeerPod", assert, "Peer Pod Container Connected to External IP").WithPod(pod).WithTestCommands(testCommands).Run()
}

func DoTestCreatePeerPodWithJob(t *testing.T, e env.Environment, assert CloudAssert) {
	jobName := "job-pi"
	job := NewJob(E2eNamespace, jobName)
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
	pod := NewBusyboxPodWithName(E2eNamespace, "confidential-pod-busybox")
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
	imageName := BUSYBOX_IMAGE
	pod := NewPod(E2eNamespace, podName, podName, imageName, WithRestartPolicy(v1.RestartPolicyOnFailure), WithEnvironmentalVariables([]v1.EnvVar{{Name: "ISPRODUCTION", Value: "true"}}), WithCommand([]string{"/bin/sh", "-c", "env"}))
	expectedPodLogString := "ISPRODUCTION=true"
	NewTestCase(t, e, "EnvVariablePeerPodWithDeploymentOnly", assert, "Peer pod with environmental variables has been created").WithPod(pod).WithExpectedPodLogString(expectedPodLogString).WithCustomPodState(v1.PodSucceeded).Run()
}

func DoTestCreatePeerPodAndCheckEnvVariableLogsWithImageAndDeployment(t *testing.T, e env.Environment, assert CloudAssert) {
	podName := "env-variable-in-both"
	imageName := "quay.io/confidential-containers/test-images:testenv"
	pod := NewPod(E2eNamespace, podName, podName, imageName, WithRestartPolicy(v1.RestartPolicyOnFailure), WithEnvironmentalVariables([]v1.EnvVar{{Name: "ISPRODUCTION", Value: "true"}}))
	expectedPodLogString := "ISPRODUCTION=true"
	NewTestCase(t, e, "EnvVariablePeerPodWithBoth", assert, "Peer pod with environmental variables has been created").WithPod(pod).WithExpectedPodLogString(expectedPodLogString).WithCustomPodState(v1.PodSucceeded).Run()
}

func DoTestCreatePeerPodWithLargeImage(t *testing.T, e env.Environment, assert CloudAssert) {
	podName := "largeimage-pod"
	imageName := "quay.io/confidential-containers/test-images:largeimage"
	pod := NewPod(E2eNamespace, podName, podName, imageName, WithRestartPolicy(v1.RestartPolicyOnFailure))
	NewTestCase(t, e, "LargeImagePeerPod", assert, "Peer pod with Large Image has been created").WithPod(pod).WithPodWatcher().Run()
}

func DoTestCreatePeerPodWithPVCAndCSIWrapper(t *testing.T, e env.Environment, assert CloudAssert, myPVC *v1.PersistentVolumeClaim, pod *v1.Pod, mountPath string) {
	testCommands := []TestCommand{
		{
			Command:       []string{"lsblk"},
			ContainerName: pod.Spec.Containers[2].Name,
			TestCommandStdoutFn: func(stdout bytes.Buffer) bool {
				if strings.Contains(stdout.String(), mountPath) {
					log.Infof("PVC volume is mounted correctly: %s", stdout.String())
					return true
				} else {
					log.Errorf("PVC volume failed to be mounted at target path: %s", stdout.String())
					return false
				}
			},
			TestCommandStderrFn: IsBufferEmpty,
			TestErrorFn:         IsErrorEmpty,
		},
	}
	NewTestCase(t, e, "PeerPodWithPVCAndCSIWrapper", assert, "PVC is created and mounted as expected").WithPod(pod).WithPVC(myPVC).WithTestCommands(testCommands).Run()
}

func DoTestCreatePeerPodWithAuthenticatedImagewithValidCredentials(t *testing.T, e env.Environment, assert CloudAssert) {
	randseed := rand.New(rand.NewSource(time.Now().UnixNano()))
	podName := "authenticated-image-valid-" + strconv.Itoa(int(randseed.Uint32())) + "-pod"
	expectedAuthStatus := "Completed"
	imageName := os.Getenv("AUTHENTICATED_REGISTRY_IMAGE")
	pod := NewPod(E2eNamespace, podName, podName, imageName, WithRestartPolicy(v1.RestartPolicyNever))
	NewTestCase(t, e, "ValidAuthImagePeerPod", assert, "Peer pod with Authenticated Image with Valid Credentials(Default service account) has been created").WithPod(pod).WithAuthenticatedImage().WithAuthImageStatus(expectedAuthStatus).WithCustomPodState(v1.PodPending).Run()
}

func DoTestCreatePeerPodWithAuthenticatedImageWithInvalidCredentials(t *testing.T, e env.Environment, assert CloudAssert) {
	registryName := "quay.io"
	if os.Getenv("AUTHENTICATED_REGISTRY_IMAGE") != "" {
		registryName = strings.Split(os.Getenv("AUTHENTICATED_REGISTRY_IMAGE"), "/")[0]
	}
	randseed := rand.New(rand.NewSource(time.Now().UnixNano()))
	podName := "authenticated-image-invalid-" + strconv.Itoa(int(randseed.Uint32())) + "-pod"
	secretName := "auth-json-secret-invalid"
	data := map[string]interface{}{
		"auths": map[string]interface{}{
			registryName: map[string]interface{}{
				"auth": "aW52YWxpZHVzZXJuYW1lOmludmFsaWRwYXNzd29yZAo=",
			},
		},
	}
	jsondata, err := json.MarshalIndent(data, "", " ")
	if err != nil {
		t.Fatal(err)
	}
	if err != nil {
		t.Fatal(err)
	}
	expectedAuthStatus := "ImagePullBackOff"
	secretData := map[string][]byte{v1.DockerConfigJsonKey: jsondata}
	secret := NewSecret(E2eNamespace, secretName, secretData, v1.SecretTypeDockerConfigJson)
	imageName := os.Getenv("AUTHENTICATED_REGISTRY_IMAGE")
	pod := NewPod(E2eNamespace, podName, podName, imageName, WithRestartPolicy(v1.RestartPolicyNever), WithImagePullSecrets(secretName))
	NewTestCase(t, e, "InvalidAuthImagePeerPod", assert, "Peer pod with Authenticated Image with Invalid Credentials has been created").WithSecret(secret).WithPod(pod).WithAuthenticatedImage().WithAuthImageStatus(expectedAuthStatus).WithCustomPodState(v1.PodPending).Run()
}

func DoTestCreatePeerPodWithAuthenticatedImageWithoutCredentials(t *testing.T, e env.Environment, assert CloudAssert) {
	randseed := rand.New(rand.NewSource(time.Now().UnixNano()))
	podName := "authenticated-image-without-creds-" + strconv.Itoa(int(randseed.Uint32())) + "-pod"
	expectedAuthStatus := "WithoutCredentials"
	imageName := os.Getenv("AUTHENTICATED_REGISTRY_IMAGE")
	pod := NewPod(E2eNamespace, podName, podName, imageName, WithRestartPolicy(v1.RestartPolicyNever))
	NewTestCase(t, e, "InvalidAuthImagePeerPod", assert, "Peer pod with Authenticated Image without Credentials has been created").WithPod(pod).WithAuthenticatedImage().WithAuthImageStatus(expectedAuthStatus).WithCustomPodState(v1.PodPending).Run()
}

func DoTestPodVMwithNoAnnotations(t *testing.T, e env.Environment, assert CloudAssert, expectedType string) {

	podName := "no-annotations"
	containerName := "busybox"
	imageName := BUSYBOX_IMAGE
	pod := NewPod(E2eNamespace, podName, containerName, imageName, WithCommand([]string{"/bin/sh", "-c", "sleep 3600"}))
	testInstanceTypes := InstanceValidatorFunctions{
		testSuccessfn: func(instance string) bool {
			if instance == expectedType {
				log.Infof("PodVM Created with %s Instance type successfully...", instance)
				return true
			} else {
				log.Infof("Failed to Create PodVM with %s Instance type", expectedType)
				return false
			}
		},
		testFailurefn: IsErrorEmpty,
	}
	NewTestCase(t, e, "PodVMWithNoAnnotations", assert, "PodVM with No Annotation is created").WithPod(pod).WithInstanceTypes(testInstanceTypes).Run()
}

func DoTestPodVMwithAnnotationsInstanceType(t *testing.T, e env.Environment, assert CloudAssert, expectedType string) {
	podName := "annotations-instance-type"
	containerName := "busybox"
	imageName := BUSYBOX_IMAGE
	annotationData := map[string]string{
		"io.katacontainers.config.hypervisor.machine_type": expectedType,
	}
	pod := NewPod(E2eNamespace, podName, containerName, imageName, WithCommand([]string{"/bin/sh", "-c", "sleep 3600"}), WithAnnotations(annotationData))

	testInstanceTypes := InstanceValidatorFunctions{
		testSuccessfn: func(instance string) bool {
			if instance == expectedType {
				log.Infof("PodVM Created with %s Instance type successfully...", instance)
				return true
			} else {
				log.Infof("Failed to Create PodVM with %s Instance type", expectedType)
				return false
			}
		},
		testFailurefn: IsErrorEmpty,
	}
	NewTestCase(t, e, "PodVMwithAnnotationsInstanceType", assert, "PodVM with Annotation is created").WithPod(pod).WithInstanceTypes(testInstanceTypes).Run()
}

func DoTestPodVMwithAnnotationsCPUMemory(t *testing.T, e env.Environment, assert CloudAssert, expectedType string) {
	podName := "annotations-cpu-mem"
	containerName := "busybox"
	imageName := BUSYBOX_IMAGE
	annotationData := map[string]string{
		"io.katacontainers.config.hypervisor.default_vcpus":  "2",
		"io.katacontainers.config.hypervisor.default_memory": "12288",
	}
	pod := NewPod(E2eNamespace, podName, containerName, imageName, WithCommand([]string{"/bin/sh", "-c", "sleep 3600"}), WithAnnotations(annotationData))

	testInstanceTypes := InstanceValidatorFunctions{
		testSuccessfn: func(instance string) bool {
			if instance == expectedType {
				log.Infof("PodVM Created with %s Instance type successfully...", instance)
				return true
			} else {
				log.Infof("Failed to Create PodVM with %s Instance type", expectedType)
				return false
			}
		},
		testFailurefn: IsErrorEmpty,
	}
	NewTestCase(t, e, "PodVMwithAnnotationsCPUMemory", assert, "PodVM with Annotations CPU Memory is created").WithPod(pod).WithInstanceTypes(testInstanceTypes).Run()
}

func DoTestPodVMwithAnnotationsInvalidInstanceType(t *testing.T, e env.Environment, assert CloudAssert, expectedType string) {
	podName := "annotations-invalid-instance-type"
	containerName := "busybox"
	imageName := BUSYBOX_IMAGE
	expectedErrorMessage := `requested instance type ("` + expectedType + `") is not part of supported instance types list`
	annotationData := map[string]string{
		"io.katacontainers.config.hypervisor.machine_type": expectedType,
	}
	pod := NewPod(E2eNamespace, podName, containerName, imageName, WithCommand([]string{"/bin/sh", "-c", "sleep 3600"}), WithAnnotations(annotationData))
	testInstanceTypes := InstanceValidatorFunctions{
		testSuccessfn: IsStringEmpty,
		testFailurefn: func(errorMsg error) bool {
			if strings.Contains(errorMsg.Error(), expectedErrorMessage) {
				log.Infof("Got Expected Error: %v", errorMsg.Error())
				return true
			} else {
				log.Infof("Failed to Get Expected Error: %v", errorMsg.Error())
				return false
			}
		},
	}
	NewTestCase(t, e, "PodVMwithAnnotationsInvalidInstanceType", assert, "Failed to Create PodVM with Annotations Invalid InstanceType").WithPod(pod).WithInstanceTypes(testInstanceTypes).WithCustomPodState(v1.PodPending).Run()
}

func DoTestPodVMwithAnnotationsLargerMemory(t *testing.T, e env.Environment, assert CloudAssert) {
	podName := "annotations-too-big-mem"
	containerName := "busybox"
	imageName := BUSYBOX_IMAGE
	expectedErrorMessage := "failed to get instance type based on vCPU and memory annotations: no instance type found for the given vcpus (2) and memory (18432)"
	annotationData := map[string]string{
		"io.katacontainers.config.hypervisor.default_vcpus":  "2",
		"io.katacontainers.config.hypervisor.default_memory": "18432",
	}
	pod := NewPod(E2eNamespace, podName, containerName, imageName, WithCommand([]string{"/bin/sh", "-c", "sleep 3600"}), WithAnnotations(annotationData))
	testInstanceTypes := InstanceValidatorFunctions{
		testSuccessfn: IsStringEmpty,
		testFailurefn: func(errorMsg error) bool {
			if strings.Contains(errorMsg.Error(), expectedErrorMessage) {
				log.Infof("Got Expected Error: %v", errorMsg.Error())
				return true
			} else {
				log.Infof("Failed to Get Expected Error: %v", errorMsg.Error())
				return false
			}
		},
	}
	NewTestCase(t, e, "PodVMwithAnnotationsLargerMemory", assert, "Failed to Create PodVM with Annotations Larger Memory").WithPod(pod).WithInstanceTypes(testInstanceTypes).WithCustomPodState(v1.PodPending).Run()
}

func DoTestPodVMwithAnnotationsLargerCPU(t *testing.T, e env.Environment, assert CloudAssert) {
	podName := "annotations-too-big-cpu"
	containerName := "busybox"
	imageName := BUSYBOX_IMAGE
	expectedErrorMessage := []string{
		"no instance type found for the given vcpus (3) and memory (12288)",
		"Number of cpus 3 specified in annotation default_vcpus is greater than the number of CPUs 2 on the system",
	}
	annotationData := map[string]string{
		"io.katacontainers.config.hypervisor.default_vcpus":  "3",
		"io.katacontainers.config.hypervisor.default_memory": "12288",
	}
	pod := NewPod(E2eNamespace, podName, containerName, imageName, WithCommand([]string{"/bin/sh", "-c", "sleep 3600"}), WithAnnotations(annotationData))
	testInstanceTypes := InstanceValidatorFunctions{
		testSuccessfn: IsStringEmpty,
		testFailurefn: func(errorMsg error) bool {
			for _, i := range expectedErrorMessage {
				if strings.Contains(errorMsg.Error(), i) {
					log.Infof("Got Expected Error: %v", errorMsg.Error())
					return true
				}
			}
			log.Infof("Failed to Get Expected Error: %v", errorMsg.Error())
			return false
		},
	}
	NewTestCase(t, e, "PodVMwithAnnotationsLargerCPU", assert, "Failed to Create PodVM with Annotations Larger CPU").WithPod(pod).WithInstanceTypes(testInstanceTypes).WithCustomPodState(v1.PodPending).Run()
}

func DoTestPodToServiceCommunication(t *testing.T, e env.Environment, assert CloudAssert) {
	clientPodName := "busybox"
	clientContainerName := "busybox"
	clientImageName := BUSYBOX_IMAGE
	serverPodName := "nginx"
	serverContainerName := "nginx"
	serverImageName := "nginx:latest"
	serviceName := "nginx"
	labels := map[string]string{
		"app": "nginx",
	}
	clientPod := NewExtraPod(E2eNamespace, clientPodName, clientContainerName, clientImageName, WithCommand([]string{"/bin/sh", "-c", "sleep 3600"}), WithRestartPolicy(v1.RestartPolicyNever))
	serverPod := NewPod(E2eNamespace, serverPodName, serverContainerName, serverImageName, WithContainerPort(80), WithRestartPolicy(v1.RestartPolicyNever), WithLabel(labels))
	testCommands := []TestCommand{
		{
			Command:       []string{"wget", "-O-", "nginx"},
			ContainerName: clientPod.pod.Spec.Containers[0].Name,
			TestCommandStdoutFn: func(stdout bytes.Buffer) bool {
				if strings.Contains(stdout.String(), "Thank you for using nginx") {
					log.Infof("Success to access nginx service. %s", stdout.String())
					return true
				} else {
					log.Errorf("Failed to access nginx service: %s", stdout.String())
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
	clientPodName := "curl"
	clientContainerName := "curl"
	clientImageName := "docker.io/curlimages/curl:8.4.0"
	serverPodName := "nginx"
	serverContainerName := "nginx"
	serverImageName := "nginx:latest"
	caService, _ := tlsutil.NewCAService("nginx")
	serverCACertPEM := caService.RootCertificate()
	serverName := "nginx"
	serverCertPEM, serverKeyPEM, _ := caService.Issue(serverName)
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
		WithSecretBinding(clientSecretDir, clientSecretName),
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
		"app": "nginx",
	}
	serverPod := NewPod(E2eNamespace, serverPodName, serverContainerName, serverImageName, WithSecureContainerPort(443), WithSecretBinding(serverSecretDir, serverSecretName), WithLabel(labels), WithConfigMapBinding(podKubeConfigmapDir, configMapName))
	configMap := NewConfigMap(E2eNamespace, configMapName, configMapData)

	testCommands := []TestCommand{
		{
			Command:       []string{"curl", "--key", "/etc/certs/tls.key", "--cert", "/etc/certs/tls.crt", "--cacert", "/etc/certs/ca.crt", "https://nginx"},
			ContainerName: clientPod.pod.Spec.Containers[0].Name,
			TestCommandStdoutFn: func(stdout bytes.Buffer) bool {
				if strings.Contains(stdout.String(), "Thank you for using nginx") {
					log.Infof("Success to access nginx service. %s", stdout.String())
					return true
				} else {
					log.Errorf("Failed to access nginx service: %s", stdout.String())
					return false
				}
			},
		},
	}
	serviceName := "nginx"
	clientPod.WithTestCommands(testCommands)
	httpsPort := corev1.ServicePort{
		Name:       "https",
		Port:       443,
		TargetPort: intstr.FromInt(int(443)),
		Protocol:   corev1.ProtocolTCP,
	}
	servicePorts := []corev1.ServicePort{httpsPort}
	nginxSvc := NewService(E2eNamespace, serviceName, servicePorts, labels)
	extraPods := []*ExtraPod{clientPod}
	extraSecrets := []*v1.Secret{clientSecret}
	NewTestCase(t, e, "TestPodsMTLSCommunication", assert, "Pods communication with mTLS").WithPod(serverPod).WithExtraPods(extraPods).WithConfigMap(configMap).WithService(nginxSvc).WithSecret(serverSecret).WithExtraSecrets(extraSecrets).Run()

}
