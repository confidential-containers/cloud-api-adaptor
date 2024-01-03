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

	log "github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/e2e-framework/pkg/env"
	envconf "sigs.k8s.io/e2e-framework/pkg/envconf"
)

// DoTestCreateSimplePod tests a simple peer-pod can be created.
func DoTestCreateSimplePod(t *testing.T, e env.Environment, assert CloudAssert) {
	namespace := envconf.RandomName("default", 7)
	pod := NewBusyboxPodWithName(namespace, "simple-test")
	NewTestCase(t, e, "SimplePeerPod", assert, "PodVM is created").WithPod(pod).Run()
}

func DoTestCreateSimplePodWithNydusAnnotation(t *testing.T, e env.Environment, assert CloudAssert) {
	namespace := envconf.RandomName("default", 7)
	annotationData := map[string]string{
		"io.containerd.cri.runtime-handler": "kata-remote",
	}
	pod := NewPod(namespace, "alpine", "alpine", "alpine", WithRestartPolicy(v1.RestartPolicyNever), WithAnnotations(annotationData))
	NewTestCase(t, e, "SimplePeerPod", assert, "PodVM is created").WithPod(pod).WithNydusSnapshotter().Run()
}

func DoTestDeleteSimplePod(t *testing.T, e env.Environment, assert CloudAssert) {
	namespace := envconf.RandomName("default", 7)
	pod := NewBusyboxPodWithName(namespace, "deletion-test")
	duration := 1 * time.Minute
	NewTestCase(t, e, "DeletePod", assert, "Deletion complete").WithPod(pod).WithDeleteAssertion(&duration).Run()
}

func DoTestCreatePodWithConfigMap(t *testing.T, e env.Environment, assert CloudAssert) {
	namespace := envconf.RandomName("default", 7)
	podName := "busybox-configmap-pod"
	containerName := "busybox-configmap-container"
	imageName := BUSYBOX_IMAGE
	configMapName := "busybox-configmap"
	configMapFileName := "example.txt"
	podKubeConfigmapDir := "/etc/config/"
	configMapPath := podKubeConfigmapDir + configMapFileName
	configMapContents := "Hello, world"
	configMapData := map[string]string{configMapFileName: configMapContents}
	pod := NewPod(namespace, podName, containerName, imageName, WithConfigMapBinding(podKubeConfigmapDir, configMapName), WithCommand([]string{"/bin/sh", "-c", "sleep 3600"}))
	configMap := NewConfigMap(namespace, configMapName, configMapData)
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
	namespace := envconf.RandomName("default", 7)
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
	pod := NewPod(namespace, podName, containerName, imageName, WithSecretBinding(podKubeSecretsDir, secretName), WithCommand([]string{"/bin/sh", "-c", "sleep 3600"}))
	secret := NewSecret(namespace, secretName, secretData, v1.SecretTypeOpaque)

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
	namespace := envconf.RandomName("default", 7)
	pod := NewBusyboxPod(namespace)
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
	namespace := envconf.RandomName("default", 7)
	jobName := "job-pi"
	job := NewJob(namespace, jobName)
	expectedPodLogString := "3.14"
	NewTestCase(t, e, "JobPeerPod", assert, "Job has been created").WithJob(job).WithExpectedPodLogString(expectedPodLogString).Run()
}

func DoTestCreatePeerPodAndCheckUserLogs(t *testing.T, e env.Environment, assert CloudAssert) {
	// namespace := envconf.RandomName("default", 7)
	// podName := "user-pod"
	// imageName := "quay.io/confidential-containers/test-images:testuser"
	// pod := NewPod(namespace, podName, podName, imageName, WithRestartPolicy(v1.RestartPolicyOnFailure))
	// expectedPodLogString := "otheruser"
	// NewTestCase(t, e, "UserPeerPod", assert, "Peer pod with user has been created").WithPod(pod).WithExpectedPodLogString(expectedPodLogString).WithCustomPodState(v1.PodSucceeded).Run()
	t.Skip("Skipping Test until issue kata-containers/kata-containers#5732 is Fixed")
	//Reference - https://github.com/kata-containers/kata-containers/issues/5732
}

// DoTestCreateConfidentialPod verify a confidential peer-pod can be created.
func DoTestCreateConfidentialPod(t *testing.T, e env.Environment, assert CloudAssert, testCommands []TestCommand) {
	namespace := envconf.RandomName("default", 7)
	pod := NewBusyboxPodWithName(namespace, "confidential-pod-busybox")
	for i := 0; i < len(testCommands); i++ {
		testCommands[i].ContainerName = pod.Spec.Containers[0].Name
	}

	NewTestCase(t, e, "ConfidentialPodVM", assert, "Confidential PodVM is created").WithPod(pod).WithTestCommands(testCommands).Run()
}

func DoTestCreatePeerPodAndCheckWorkDirLogs(t *testing.T, e env.Environment, assert CloudAssert) {
	namespace := envconf.RandomName("default", 7)
	podName := "workdirpod"
	imageName := "quay.io/confidential-containers/test-images:testworkdir"
	pod := NewPod(namespace, podName, podName, imageName, WithRestartPolicy(v1.RestartPolicyOnFailure))
	expectedPodLogString := "/other"
	NewTestCase(t, e, "WorkDirPeerPod", assert, "Peer pod with work directory has been created").WithPod(pod).WithExpectedPodLogString(expectedPodLogString).WithCustomPodState(v1.PodSucceeded).Run()
}

func DoTestCreatePeerPodAndCheckEnvVariableLogsWithImageOnly(t *testing.T, e env.Environment, assert CloudAssert) {
	namespace := envconf.RandomName("default", 7)
	podName := "env-variable-in-image"
	imageName := "quay.io/confidential-containers/test-images:testenv"
	pod := NewPod(namespace, podName, podName, imageName, WithRestartPolicy(v1.RestartPolicyOnFailure))
	expectedPodLogString := "ISPRODUCTION=false"
	NewTestCase(t, e, "EnvVariablePeerPodWithImageOnly", assert, "Peer pod with environmental variables has been created").WithPod(pod).WithExpectedPodLogString(expectedPodLogString).WithCustomPodState(v1.PodSucceeded).Run()
}

func DoTestCreatePeerPodAndCheckEnvVariableLogsWithDeploymentOnly(t *testing.T, e env.Environment, assert CloudAssert) {
	namespace := envconf.RandomName("default", 7)
	podName := "env-variable-in-config"
	imageName := BUSYBOX_IMAGE
	pod := NewPod(namespace, podName, podName, imageName, WithRestartPolicy(v1.RestartPolicyOnFailure), WithEnvironmentalVariables([]v1.EnvVar{{Name: "ISPRODUCTION", Value: "true"}}), WithCommand([]string{"/bin/sh", "-c", "env"}))
	expectedPodLogString := "ISPRODUCTION=true"
	NewTestCase(t, e, "EnvVariablePeerPodWithDeploymentOnly", assert, "Peer pod with environmental variables has been created").WithPod(pod).WithExpectedPodLogString(expectedPodLogString).WithCustomPodState(v1.PodSucceeded).Run()
}

func DoTestCreatePeerPodAndCheckEnvVariableLogsWithImageAndDeployment(t *testing.T, e env.Environment, assert CloudAssert) {
	namespace := envconf.RandomName("default", 7)
	podName := "env-variable-in-both"
	imageName := "quay.io/confidential-containers/test-images:testenv"
	pod := NewPod(namespace, podName, podName, imageName, WithRestartPolicy(v1.RestartPolicyOnFailure), WithEnvironmentalVariables([]v1.EnvVar{{Name: "ISPRODUCTION", Value: "true"}}))
	expectedPodLogString := "ISPRODUCTION=true"
	NewTestCase(t, e, "EnvVariablePeerPodWithBoth", assert, "Peer pod with environmental variables has been created").WithPod(pod).WithExpectedPodLogString(expectedPodLogString).WithCustomPodState(v1.PodSucceeded).Run()
}

func DoTestCreatePeerPodWithLargeImage(t *testing.T, e env.Environment, assert CloudAssert) {
	namespace := envconf.RandomName("default", 7)
	podName := "largeimage-pod"
	imageName := "quay.io/confidential-containers/test-images:largeimage"
	pod := NewPod(namespace, podName, podName, imageName, WithRestartPolicy(v1.RestartPolicyOnFailure))
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
	namespace := envconf.RandomName("default", 7)
	randseed := rand.New(rand.NewSource(time.Now().UnixNano()))
	podName := "authenticated-image-valid-" + strconv.Itoa(int(randseed.Uint32())) + "-pod"
	secretName := "auth-json-secret"
	authfile, err := os.ReadFile("../../install/overlays/ibmcloud/auth.json")
	if err != nil {
		t.Fatal(err)
	}
	expectedAuthStatus := "Completed"
	secretData := map[string][]byte{v1.DockerConfigJsonKey: authfile}
	secret := NewSecret(namespace, secretName, secretData, v1.SecretTypeDockerConfigJson)
	imageName := os.Getenv("AUTHENTICATED_REGISTRY_IMAGE")
	pod := NewPod(namespace, podName, podName, imageName, WithRestartPolicy(v1.RestartPolicyNever), WithImagePullSecrets(secretName))
	NewTestCase(t, e, "ValidAuthImagePeerPod", assert, "Peer pod with Authenticated Image with Valid Credentials has been created").WithSecret(secret).WithPod(pod).WithAuthenticatedImage().WithAuthImageStatus(expectedAuthStatus).WithCustomPodState(v1.PodPending).Run()
}

func DoTestCreatePeerPodWithAuthenticatedImageWithInvalidCredentials(t *testing.T, e env.Environment, assert CloudAssert) {
	namespace := envconf.RandomName("default", 7)
	randseed := rand.New(rand.NewSource(time.Now().UnixNano()))
	podName := "authenticated-image-invalid-" + strconv.Itoa(int(randseed.Uint32())) + "-pod"
	secretName := "auth-json-secret"
	data := map[string]interface{}{
		"auths": map[string]interface{}{
			"quay.io": map[string]interface{}{
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
	secret := NewSecret(namespace, secretName, secretData, v1.SecretTypeDockerConfigJson)
	imageName := os.Getenv("AUTHENTICATED_REGISTRY_IMAGE")
	pod := NewPod(namespace, podName, podName, imageName, WithRestartPolicy(v1.RestartPolicyNever), WithImagePullSecrets(secretName))
	NewTestCase(t, e, "InvalidAuthImagePeerPod", assert, "Peer pod with Authenticated Image with Invalid Credentials has been created").WithSecret(secret).WithPod(pod).WithAuthenticatedImage().WithAuthImageStatus(expectedAuthStatus).WithCustomPodState(v1.PodPending).Run()
}

func DoTestCreatePeerPodWithAuthenticatedImageWithoutCredentials(t *testing.T, e env.Environment, assert CloudAssert) {
	namespace := envconf.RandomName("default", 7)
	randseed := rand.New(rand.NewSource(time.Now().UnixNano()))
	podName := "authenticated-image-without-creds-" + strconv.Itoa(int(randseed.Uint32())) + "-pod"
	expectedAuthStatus := "WithoutCredentials"
	imageName := os.Getenv("AUTHENTICATED_REGISTRY_IMAGE")
	pod := NewPod(namespace, podName, podName, imageName, WithRestartPolicy(v1.RestartPolicyNever))
	NewTestCase(t, e, "InvalidAuthImagePeerPod", assert, "Peer pod with Authenticated Image with Invalid Credentials has been created").WithPod(pod).WithAuthenticatedImage().WithAuthImageStatus(expectedAuthStatus).WithCustomPodState(v1.PodPending).Run()
}

func DoTestPodVMwithNoAnnotations(t *testing.T, e env.Environment, assert CloudAssert, expectedType string) {

	namespace := envconf.RandomName("default", 7)
	podName := "no-annotations"
	containerName := "busybox"
	imageName := BUSYBOX_IMAGE
	pod := NewPod(namespace, podName, containerName, imageName, WithCommand([]string{"/bin/sh", "-c", "sleep 3600"}))
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
	namespace := envconf.RandomName("default", 7)
	podName := "annotations-instance-type"
	containerName := "busybox"
	imageName := BUSYBOX_IMAGE
	annotationData := map[string]string{
		"io.katacontainers.config.hypervisor.machine_type": expectedType,
	}
	pod := NewPod(namespace, podName, containerName, imageName, WithCommand([]string{"/bin/sh", "-c", "sleep 3600"}), WithAnnotations(annotationData))

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
	namespace := envconf.RandomName("default", 7)
	podName := "annotations-cpu-mem"
	containerName := "busybox"
	imageName := BUSYBOX_IMAGE
	annotationData := map[string]string{
		"io.katacontainers.config.hypervisor.default_vcpus":  "2",
		"io.katacontainers.config.hypervisor.default_memory": "12288",
	}
	pod := NewPod(namespace, podName, containerName, imageName, WithCommand([]string{"/bin/sh", "-c", "sleep 3600"}), WithAnnotations(annotationData))

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
	namespace := envconf.RandomName("default", 7)
	podName := "annotations-invalid-instance-type"
	containerName := "busybox"
	imageName := BUSYBOX_IMAGE
	expectedErrorMessage := `requested instance type ("` + expectedType + `") is not part of supported instance types list`
	annotationData := map[string]string{
		"io.katacontainers.config.hypervisor.machine_type": expectedType,
	}
	pod := NewPod(namespace, podName, containerName, imageName, WithCommand([]string{"/bin/sh", "-c", "sleep 3600"}), WithAnnotations(annotationData))
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
	namespace := envconf.RandomName("default", 7)
	podName := "annotations-too-big-mem"
	containerName := "busybox"
	imageName := BUSYBOX_IMAGE
	expectedErrorMessage := "failed to get instance type based on vCPU and memory annotations: no instance type found for the given vcpus (2) and memory (18432)"
	annotationData := map[string]string{
		"io.katacontainers.config.hypervisor.default_vcpus":  "2",
		"io.katacontainers.config.hypervisor.default_memory": "18432",
	}
	pod := NewPod(namespace, podName, containerName, imageName, WithCommand([]string{"/bin/sh", "-c", "sleep 3600"}), WithAnnotations(annotationData))
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
	namespace := envconf.RandomName("default", 7)
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
	pod := NewPod(namespace, podName, containerName, imageName, WithCommand([]string{"/bin/sh", "-c", "sleep 3600"}), WithAnnotations(annotationData))
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
	namespace := envconf.RandomName("default", 7)
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
	clientPod := NewExtraPod(namespace, clientPodName, clientContainerName, clientImageName, WithCommand([]string{"/bin/sh", "-c", "sleep 3600"}), WithRestartPolicy(v1.RestartPolicyNever))
	serverPod := NewPod(namespace, serverPodName, serverContainerName, serverImageName, WithContainerPort(80), WithRestartPolicy(v1.RestartPolicyNever), WithLabel(labels))
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
	nginxSvc := NewService(namespace, serviceName, "http", 80, 80, labels)
	extraPods := []*ExtraPod{clientPod}
	NewTestCase(t, e, "TestExtraPods", assert, "Failed to test extra pod.").WithPod(serverPod).WithExtraPods(extraPods).WithService(nginxSvc).Run()
}
