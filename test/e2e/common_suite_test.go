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
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	envconf "sigs.k8s.io/e2e-framework/pkg/envconf"
)

// doTestCreateSimplePod tests a simple peer-pod can be created.
func doTestCreateSimplePod(t *testing.T, assert CloudAssert) {
	namespace := envconf.RandomName("default", 7)
	pod := newNginxPod(namespace)
	newTestCase(t, "SimplePeerPod", assert, "PodVM is created").withPod(pod).run()
}

func doTestCreateSimplePodWithNydusAnnotation(t *testing.T, assert CloudAssert) {
	namespace := envconf.RandomName("default", 7)
	annotationData := map[string]string{
		"io.containerd.cri.runtime-handler": "kata-remote",
	}
	pod := newPod(namespace, "alpine", "alpine", "alpine", withRestartPolicy(corev1.RestartPolicyNever), withAnnotations(annotationData))
	newTestCase(t, "SimplePeerPod", assert, "PodVM is created").withPod(pod).withNydusSnapshotter().run()
}

func doTestDeleteSimplePod(t *testing.T, assert CloudAssert) {
	namespace := envconf.RandomName("default", 7)
	pod := newNginxPodWithName(namespace, "deletion-test")
	duration := 1 * time.Minute
	newTestCase(t, "DeletePod", assert, "Deletion complete").withPod(pod).withDeleteAssertion(&duration).run()
}

func doTestCreatePodWithConfigMap(t *testing.T, assert CloudAssert) {
	namespace := envconf.RandomName("default", 7)
	podName := "nginx-configmap-pod"
	containerName := "nginx-configmap-container"
	imageName := "nginx:latest"
	configMapName := "nginx-configmap"
	configMapFileName := "example.txt"
	podKubeConfigmapDir := "/etc/config/"
	configMapPath := podKubeConfigmapDir + configMapFileName
	configMapContents := "Hello, world"
	configMapData := map[string]string{configMapFileName: configMapContents}
	pod := newPod(namespace, podName, containerName, imageName, withConfigMapBinding(podKubeConfigmapDir, configMapName), withContainerPort(80))
	configMap := newConfigMap(namespace, configMapName, configMapData)
	testCommands := []testCommand{
		{
			command:       []string{"cat", configMapPath},
			containerName: pod.Spec.Containers[0].Name,
			testCommandStdoutFn: func(stdout bytes.Buffer) bool {
				if stdout.String() == configMapContents {
					log.Infof("Data Inside Configmap: %s", stdout.String())
					return true
				} else {
					log.Errorf("Configmap has invalid Data: %s", stdout.String())
					return false
				}
			},
		},
	}

	newTestCase(t, "ConfigMapPeerPod", assert, "Configmap is created and contains data").withPod(pod).withConfigMap(configMap).withTestCommands(testCommands).withCustomPodState(v1.PodRunning).run()
}

func doTestCreatePodWithSecret(t *testing.T, assert CloudAssert) {
	//doTestCreatePod(t, assert, "Secret is created and contains data", pod)
	namespace := envconf.RandomName("default", 7)
	podName := "nginx-secret-pod"
	containerName := "nginx-secret-container"
	imageName := "nginx:latest"
	secretName := "nginx-secret"
	podKubeSecretsDir := "/etc/secret/"
	usernameFileName := "username"
	username := "admin"
	usernamePath := podKubeSecretsDir + usernameFileName
	passwordFileName := "password"
	password := "password"
	passwordPath := podKubeSecretsDir + passwordFileName
	secretData := map[string][]byte{passwordFileName: []byte(password), usernameFileName: []byte(username)}
	pod := newPod(namespace, podName, containerName, imageName, withSecretBinding(podKubeSecretsDir, secretName), withContainerPort(80))
	secret := newSecret(namespace, secretName, secretData, v1.SecretTypeOpaque)

	testCommands := []testCommand{
		{
			command:       []string{"cat", usernamePath},
			containerName: pod.Spec.Containers[0].Name,
			testCommandStdoutFn: func(stdout bytes.Buffer) bool {
				if stdout.String() == username {
					log.Infof("Username from secret inside pod: %s", stdout.String())
					return true
				} else {
					log.Errorf("Username value from secret inside pod unexpected. Expected %s, got %s", username, stdout.String())
					return false
				}
			},
		},
		{
			command:       []string{"cat", passwordPath},
			containerName: pod.Spec.Containers[0].Name,
			testCommandStdoutFn: func(stdout bytes.Buffer) bool {
				if stdout.String() == password {
					log.Infof("Password from secret inside pod: %s", stdout.String())
					return true
				} else {
					log.Errorf("Password value from secret inside pod unexpected. Expected %s, got %s", password, stdout.String())
					return false
				}
			},
		},
	}

	newTestCase(t, "SecretPeerPod", assert, "Secret has been created and contains data").withPod(pod).withSecret(secret).withTestCommands(testCommands).withCustomPodState(v1.PodRunning).run()
}

func doTestCreatePeerPodContainerWithExternalIPAccess(t *testing.T, assert CloudAssert) {
	namespace := envconf.RandomName("default", 7)
	pod := newBusyboxPod(namespace)
	testCommands := []testCommand{
		{
			command:       []string{"ping", "-c", "1", "www.google.com"},
			containerName: pod.Spec.Containers[0].Name,
			testCommandStdoutFn: func(stdout bytes.Buffer) bool {
				if stdout.String() != "" {
					log.Infof("Output of ping command in busybox : %s", stdout.String())
					return true
				} else {
					log.Info("No output from ping command")
					return false
				}
			},
		},
	}

	newTestCase(t, "IPAccessPeerPod", assert, "Peer Pod Container Connected to External IP").withPod(pod).withTestCommands(testCommands).run()
}

func doTestCreatePeerPodWithJob(t *testing.T, assert CloudAssert) {
	namespace := envconf.RandomName("default", 7)
	jobName := "job-pi"
	job := newJob(namespace, jobName)
	expectedPodLogString := "3.14"
	newTestCase(t, "JobPeerPod", assert, "Job has been created").withJob(job).withExpectedPodLogString(expectedPodLogString).run()
}

func doTestCreatePeerPodAndCheckUserLogs(t *testing.T, assert CloudAssert) {
	// namespace := envconf.RandomName("default", 7)
	// podName := "user-pod"
	// imageName := "quay.io/confidential-containers/test-images:testuser"
	// pod := newPod(namespace, podName, podName, imageName, withRestartPolicy(v1.RestartPolicyOnFailure))
	// expectedPodLogString := "otheruser"
	// newTestCase(t, "UserPeerPod", assert, "Peer pod with user has been created").withPod(pod).withExpectedPodLogString(expectedPodLogString).withCustomPodState(v1.PodSucceeded).run()
	t.Skip("Skipping Test until issue kata-containers/kata-containers#5732 is Fixed")
	//Reference - https://github.com/kata-containers/kata-containers/issues/5732
}

// doTestCreateConfidentialPod verify a confidential peer-pod can be created.
func doTestCreateConfidentialPod(t *testing.T, assert CloudAssert, testCommands []testCommand) {
	namespace := envconf.RandomName("default", 7)
	pod := newNginxPodWithName(namespace, "confidential-pod-nginx")
	for i := 0; i < len(testCommands); i++ {
		testCommands[i].containerName = pod.Spec.Containers[0].Name
	}

	newTestCase(t, "ConfidentialPodVM", assert, "Confidential PodVM is created").withPod(pod).withTestCommands(testCommands).run()
}

func doTestCreatePeerPodAndCheckWorkDirLogs(t *testing.T, assert CloudAssert) {
	namespace := envconf.RandomName("default", 7)
	podName := "workdirpod"
	imageName := "quay.io/confidential-containers/test-images:testworkdir"
	pod := newPod(namespace, podName, podName, imageName, withRestartPolicy(v1.RestartPolicyOnFailure))
	expectedPodLogString := "/other"
	newTestCase(t, "WorkDirPeerPod", assert, "Peer pod with work directory has been created").withPod(pod).withExpectedPodLogString(expectedPodLogString).withCustomPodState(v1.PodSucceeded).run()
}

func doTestCreatePeerPodAndCheckEnvVariableLogsWithImageOnly(t *testing.T, assert CloudAssert) {
	namespace := envconf.RandomName("default", 7)
	podName := "env-variable-in-image"
	imageName := "quay.io/confidential-containers/test-images:testenv"
	pod := newPod(namespace, podName, podName, imageName, withRestartPolicy(v1.RestartPolicyOnFailure))
	expectedPodLogString := "ISPRODUCTION=false"
	newTestCase(t, "EnvVariablePeerPodWithImageOnly", assert, "Peer pod with environmental variables has been created").withPod(pod).withExpectedPodLogString(expectedPodLogString).withCustomPodState(v1.PodSucceeded).run()
}

func doTestCreatePeerPodAndCheckEnvVariableLogsWithDeploymentOnly(t *testing.T, assert CloudAssert) {
	namespace := envconf.RandomName("default", 7)
	podName := "env-variable-in-config"
	imageName := "nginx:latest"
	pod := newPod(namespace, podName, podName, imageName, withRestartPolicy(v1.RestartPolicyOnFailure), withEnvironmentalVariables([]v1.EnvVar{{Name: "ISPRODUCTION", Value: "true"}}), withCommand([]string{"/bin/sh", "-c", "env"}), withContainerPort(80))
	expectedPodLogString := "ISPRODUCTION=true"
	newTestCase(t, "EnvVariablePeerPodWithDeploymentOnly", assert, "Peer pod with environmental variables has been created").withPod(pod).withExpectedPodLogString(expectedPodLogString).withCustomPodState(v1.PodSucceeded).run()
}

func doTestCreatePeerPodAndCheckEnvVariableLogsWithImageAndDeployment(t *testing.T, assert CloudAssert) {
	namespace := envconf.RandomName("default", 7)
	podName := "env-variable-in-both"
	imageName := "quay.io/confidential-containers/test-images:testenv"
	pod := newPod(namespace, podName, podName, imageName, withRestartPolicy(v1.RestartPolicyOnFailure), withEnvironmentalVariables([]v1.EnvVar{{Name: "ISPRODUCTION", Value: "true"}}))
	expectedPodLogString := "ISPRODUCTION=true"
	newTestCase(t, "EnvVariablePeerPodWithBoth", assert, "Peer pod with environmental variables has been created").withPod(pod).withExpectedPodLogString(expectedPodLogString).withCustomPodState(v1.PodSucceeded).run()
}

func doTestCreatePeerPodWithLargeImage(t *testing.T, assert CloudAssert) {
	namespace := envconf.RandomName("default", 7)
	podName := "largeimage-pod"
	imageName := "quay.io/confidential-containers/test-images:largeimage"
	pod := newPod(namespace, podName, podName, imageName, withRestartPolicy(v1.RestartPolicyOnFailure))
	newTestCase(t, "LargeImagePeerPod", assert, "Peer pod with Large Image has been created").withPod(pod).withPodWatcher().run()
}

func doTestCreatePeerPodWithPVCAndCSIWrapper(t *testing.T, assert CloudAssert, myPVC *v1.PersistentVolumeClaim, pod *v1.Pod, mountPath string) {
	testCommands := []testCommand{
		{
			command:       []string{"lsblk"},
			containerName: pod.Spec.Containers[2].Name,
			testCommandStdoutFn: func(stdout bytes.Buffer) bool {
				if strings.Contains(stdout.String(), mountPath) {
					log.Infof("PVC volume is mounted correctly: %s", stdout.String())
					return true
				} else {
					log.Errorf("PVC volume failed to be mounted at target path: %s", stdout.String())
					return false
				}
			},
		},
	}
	newTestCase(t, "PeerPodWithPVCAndCSIWrapper", assert, "PVC is created and mounted as expected").withPod(pod).withPVC(myPVC).withTestCommands(testCommands).withCustomPodState(v1.PodRunning).run()
}

func doTestCreatePeerPodWithAuthenticatedImagewithValidCredentials(t *testing.T, assert CloudAssert) {
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
	secret := newSecret(namespace, secretName, secretData, v1.SecretTypeDockerConfigJson)
	imageName := os.Getenv("AUTHENTICATED_REGISTRY_IMAGE")
	pod := newPod(namespace, podName, podName, imageName, withRestartPolicy(v1.RestartPolicyNever), withImagePullSecrets(secretName))
	newTestCase(t, "ValidAuthImagePeerPod", assert, "Peer pod with Authenticated Image with Valid Credentials has been created").withSecret(secret).withPod(pod).withAuthenticatedImage().withAuthImageStatus(expectedAuthStatus).withCustomPodState(v1.PodPending).run()
}

func doTestCreatePeerPodWithAuthenticatedImageWithInvalidCredentials(t *testing.T, assert CloudAssert) {
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
	secret := newSecret(namespace, secretName, secretData, v1.SecretTypeDockerConfigJson)
	imageName := os.Getenv("AUTHENTICATED_REGISTRY_IMAGE")
	pod := newPod(namespace, podName, podName, imageName, withRestartPolicy(v1.RestartPolicyNever), withImagePullSecrets(secretName))
	newTestCase(t, "InvalidAuthImagePeerPod", assert, "Peer pod with Authenticated Image with Invalid Credentials has been created").withSecret(secret).withPod(pod).withAuthenticatedImage().withAuthImageStatus(expectedAuthStatus).withCustomPodState(v1.PodPending).run()
}

func doTestCreatePeerPodWithAuthenticatedImageWithoutCredentials(t *testing.T, assert CloudAssert) {
	namespace := envconf.RandomName("default", 7)
	randseed := rand.New(rand.NewSource(time.Now().UnixNano()))
	podName := "authenticated-image-without-creds-" + strconv.Itoa(int(randseed.Uint32())) + "-pod"
	expectedAuthStatus := "WithoutCredentials"
	imageName := os.Getenv("AUTHENTICATED_REGISTRY_IMAGE")
	pod := newPod(namespace, podName, podName, imageName, withRestartPolicy(v1.RestartPolicyNever))
	newTestCase(t, "InvalidAuthImagePeerPod", assert, "Peer pod with Authenticated Image with Invalid Credentials has been created").withPod(pod).withAuthenticatedImage().withAuthImageStatus(expectedAuthStatus).withCustomPodState(v1.PodPending).run()
}

func doTestPodVMwithNoAnnotations(t *testing.T, assert CloudAssert, expectedType string) {

	namespace := envconf.RandomName("default", 7)
	podName := "no-annotations"
	containerName := "busybox"
	imageName := "busybox:latest"
	pod := newPod(namespace, podName, containerName, imageName, withCommand([]string{"/bin/sh", "-c", "sleep 3600"}))
	testInstanceTypes := instanceValidatorFunctions{
		testSuccessfn: func(instance string) bool {
			if instance == expectedType {
				log.Infof("PodVM Created with %s Instance type successfully...", instance)
				return true
			} else {
				log.Infof("Failed to Create PodVM with %s Instance type", expectedType)
				return false
			}
		},
		testFailurefn: testErrorEmpty,
	}
	newTestCase(t, "PodVMWithNoAnnotations", assert, "PodVM with No Annotation is created").withPod(pod).withInstanceTypes(testInstanceTypes).run()
}

func doTestPodVMwithAnnotationsInstanceType(t *testing.T, assert CloudAssert, expectedType string) {
	namespace := envconf.RandomName("default", 7)
	podName := "annotations-instance-type"
	containerName := "busybox"
	imageName := "busybox:latest"
	annotationData := map[string]string{
		"io.katacontainers.config.hypervisor.machine_type": expectedType,
	}
	pod := newPod(namespace, podName, containerName, imageName, withCommand([]string{"/bin/sh", "-c", "sleep 3600"}), withAnnotations(annotationData))

	testInstanceTypes := instanceValidatorFunctions{
		testSuccessfn: func(instance string) bool {
			if instance == expectedType {
				log.Infof("PodVM Created with %s Instance type successfully...", instance)
				return true
			} else {
				log.Infof("Failed to Create PodVM with %s Instance type", expectedType)
				return false
			}
		},
		testFailurefn: testErrorEmpty,
	}
	newTestCase(t, "PodVMwithAnnotationsInstanceType", assert, "PodVM with Annotation is created").withPod(pod).withInstanceTypes(testInstanceTypes).run()
}

func doTestPodVMwithAnnotationsCPUMemory(t *testing.T, assert CloudAssert, expectedType string) {
	namespace := envconf.RandomName("default", 7)
	podName := "annotations-cpu-mem"
	containerName := "busybox"
	imageName := "busybox:latest"
	annotationData := map[string]string{
		"io.katacontainers.config.hypervisor.default_vcpus":  "2",
		"io.katacontainers.config.hypervisor.default_memory": "12288",
	}
	pod := newPod(namespace, podName, containerName, imageName, withCommand([]string{"/bin/sh", "-c", "sleep 3600"}), withAnnotations(annotationData))

	testInstanceTypes := instanceValidatorFunctions{
		testSuccessfn: func(instance string) bool {
			if instance == expectedType {
				log.Infof("PodVM Created with %s Instance type successfully...", instance)
				return true
			} else {
				log.Infof("Failed to Create PodVM with %s Instance type", expectedType)
				return false
			}
		},
		testFailurefn: testErrorEmpty,
	}
	newTestCase(t, "PodVMwithAnnotationsCPUMemory", assert, "PodVM with Annotations CPU Memory is created").withPod(pod).withInstanceTypes(testInstanceTypes).run()
}

func doTestPodVMwithAnnotationsInvalidInstanceType(t *testing.T, assert CloudAssert, expectedType string) {
	namespace := envconf.RandomName("default", 7)
	podName := "annotations-invalid-instance-type"
	containerName := "busybox"
	imageName := "busybox:latest"
	expectedErrorMessage := `requested instance type ("` + expectedType + `") is not part of supported instance types list`
	annotationData := map[string]string{
		"io.katacontainers.config.hypervisor.machine_type": expectedType,
	}
	pod := newPod(namespace, podName, containerName, imageName, withCommand([]string{"/bin/sh", "-c", "sleep 3600"}), withAnnotations(annotationData))
	testInstanceTypes := instanceValidatorFunctions{
		testSuccessfn: testStringEmpty,
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
	newTestCase(t, "PodVMwithAnnotationsInvalidInstanceType", assert, "Failed to Create PodVM with Annotations Invalid InstanceType").withPod(pod).withInstanceTypes(testInstanceTypes).withCustomPodState(v1.PodPending).run()
}

func doTestPodVMwithAnnotationsLargerMemory(t *testing.T, assert CloudAssert) {
	namespace := envconf.RandomName("default", 7)
	podName := "annotations-too-big-mem"
	containerName := "busybox"
	imageName := "busybox:latest"
	expectedErrorMessage := "failed to get instance type based on vCPU and memory annotations: no instance type found for the given vcpus (2) and memory (18432)"
	annotationData := map[string]string{
		"io.katacontainers.config.hypervisor.default_vcpus":  "2",
		"io.katacontainers.config.hypervisor.default_memory": "18432",
	}
	pod := newPod(namespace, podName, containerName, imageName, withCommand([]string{"/bin/sh", "-c", "sleep 3600"}), withAnnotations(annotationData))
	testInstanceTypes := instanceValidatorFunctions{
		testSuccessfn: testStringEmpty,
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
	newTestCase(t, "PodVMwithAnnotationsLargerMemory", assert, "Failed to Create PodVM with Annotations Larger Memory").withPod(pod).withInstanceTypes(testInstanceTypes).withCustomPodState(v1.PodPending).run()
}

func doTestPodVMwithAnnotationsLargerCPU(t *testing.T, assert CloudAssert) {
	namespace := envconf.RandomName("default", 7)
	podName := "annotations-too-big-cpu"
	containerName := "busybox"
	imageName := "busybox:latest"
	expectedErrorMessage := []string{
		"no instance type found for the given vcpus (3) and memory (12288)",
		"Number of cpus 3 specified in annotation default_vcpus is greater than the number of CPUs 2 on the system",
	}
	annotationData := map[string]string{
		"io.katacontainers.config.hypervisor.default_vcpus":  "3",
		"io.katacontainers.config.hypervisor.default_memory": "12288",
	}
	pod := newPod(namespace, podName, containerName, imageName, withCommand([]string{"/bin/sh", "-c", "sleep 3600"}), withAnnotations(annotationData))
	testInstanceTypes := instanceValidatorFunctions{
		testSuccessfn: testStringEmpty,
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
	newTestCase(t, "PodVMwithAnnotationsLargerCPU", assert, "Failed to Create PodVM with Annotations Larger CPU").withPod(pod).withInstanceTypes(testInstanceTypes).withCustomPodState(v1.PodPending).run()
}
