// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/client-go/kubernetes"

	batchv1 "k8s.io/api/batch/v1"

	log "github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/e2e-framework/klient"
	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/klient/wait/conditions"
	envconf "sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"
)

const WAIT_POD_RUNNING_TIMEOUT = time.Second * 900
const WAIT_JOB_RUNNING_TIMEOUT = time.Second * 600

// testCommand is a list of commands to execute inside the pod container,
// each with a function to test if the command outputs the value the test
// expects it to on the stdout stream
type testCommand struct {
	command             []string
	testCommandStdoutFn func(stdout bytes.Buffer) bool
	containerName       string
}

type testCase struct {
	testing              *testing.T
	testName             string
	assert               CloudAssert
	assessMessage        string
	pod                  *v1.Pod
	configMap            *v1.ConfigMap
	secret               *v1.Secret
	pvc                  *v1.PersistentVolumeClaim
	job                  *batchv1.Job
	testCommands         []testCommand
	expectedPodLogString string
	podState             v1.PodPhase
	imagePullTimer       bool
}

func (tc *testCase) withConfigMap(configMap *v1.ConfigMap) *testCase {
	tc.configMap = configMap
	return tc
}

func (tc *testCase) withSecret(secret *v1.Secret) *testCase {
	tc.secret = secret
	return tc
}

func (tc *testCase) withPVC(pvc *v1.PersistentVolumeClaim) *testCase {
	tc.pvc = pvc
	return tc
}

func (tc *testCase) withJob(job *batchv1.Job) *testCase {
	tc.job = job
	return tc
}

func (tc *testCase) withPod(pod *v1.Pod) *testCase {
	tc.pod = pod
	return tc
}

func (tc *testCase) withTestCommands(testCommands []testCommand) *testCase {
	tc.testCommands = testCommands
	return tc
}

func (tc *testCase) withExpectedPodLogString(expectedPodLogString string) *testCase {
	tc.expectedPodLogString = expectedPodLogString
	return tc
}

func (tc *testCase) withCustomPodState(customPodState v1.PodPhase) *testCase {
	tc.podState = customPodState
	return tc
}

func (tc *testCase) withPodWatcher() *testCase {
	tc.imagePullTimer = true
	return tc
}

func (tc *testCase) run() {
	testCaseFeature := features.New(fmt.Sprintf("%s test", tc.testName)).
		WithSetup("Create testworkload", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			client, err := cfg.NewClient()
			if err != nil {
				t.Fatal(err)
			}

			if tc.configMap != nil {
				if err = client.Resources().Create(ctx, tc.configMap); err != nil {
					t.Fatal(err)
				}
			}

			if tc.secret != nil {
				if err = client.Resources().Create(ctx, tc.secret); err != nil {
					t.Fatal(err)
				}
			}

			if tc.pvc != nil {
				if err = client.Resources().Create(ctx, tc.pvc); err != nil {
					t.Fatal(err)
				}
			}

			if tc.job != nil {
				if err = client.Resources().Create(ctx, tc.job); err != nil {
					t.Fatal(err)
				}
				if err = wait.For(conditions.New(client.Resources()).JobCompleted(tc.job), wait.WithTimeout(WAIT_JOB_RUNNING_TIMEOUT)); err != nil {
					//Using t.log instead of t.Fatal here because we need to assess number of success and failure pods if job fails to complete
					t.Log(err)
				}
			}

			if tc.pod != nil {
				if err = client.Resources().Create(ctx, tc.pod); err != nil {
					t.Fatal(err)
				}

				if err = wait.For(conditions.New(client.Resources()).PodPhaseMatch(tc.pod, tc.podState), wait.WithTimeout(WAIT_POD_RUNNING_TIMEOUT)); err != nil {
					t.Fatal(err)
				}

			}
			return ctx
		}).
		Assess(tc.assessMessage, func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			client := cfg.Client()
			var podlist v1.PodList

			if tc.job != nil {
				if err := client.Resources(tc.job.Namespace).List(ctx, &podlist); err != nil {
					t.Fatal(err)
				}
				successPod, errorPod, podLogString, err := getSuccessfulAndErroredPods(ctx, t, client, *tc.job)
				if err != nil {
					t.Fatal(err)
				}
				if errorPod == len(podlist.Items) {
					t.Errorf("Job Failed to Start pod")
				}
				if successPod == 1 && errorPod >= 1 {
					t.Skip("Expected Completed status on first attempt")
				}
				if podLogString != "" {
					if strings.Contains(podLogString, tc.expectedPodLogString) {
						log.Printf("Output Log from Pod: %s", podLogString)
					} else {
						t.Errorf("Job Created pod with Invalid log")
					}
				}
			}

			if tc.pod != nil {

				if tc.imagePullTimer {
					if err := client.Resources("confidential-containers-system").List(ctx, &podlist); err != nil {
						t.Fatal(err)
					}
					for _, caaPod := range podlist.Items {
						if caaPod.Labels["app"] == "cloud-api-adaptor" {
							imagePullTime, err := watchImagePullTime(ctx, client, caaPod, *tc.pod)
							if err != nil {
								t.Fatal(err)
							}
							t.Logf("Time Taken to pull 4GB Image: %s", imagePullTime)
							break
						}
					}

				}
				if tc.expectedPodLogString != "" {
					LogString, err := comparePodLogString(ctx, client, *tc.pod, tc.expectedPodLogString)
					if err != nil {
						t.Logf("Output:%s", LogString)
						t.Fatal(err)
					}
					t.Logf("Log output of peer pod:%s", LogString)
				}
				if tc.podState == v1.PodRunning {
					if err := client.Resources(tc.pod.Namespace).List(ctx, &podlist); err != nil {
						t.Fatal(err)
					}
					if len(tc.testCommands) > 0 {
						for _, testCommand := range tc.testCommands {
							var stdout, stderr bytes.Buffer

							for _, podItem := range podlist.Items {
								if podItem.ObjectMeta.Name == tc.pod.Name {
									//adding sleep time to intialize container and ready for Executing commands
									time.Sleep(5 * time.Second)
									if err := cfg.Client().Resources(tc.pod.Namespace).ExecInPod(ctx, tc.pod.Namespace, tc.pod.Name, testCommand.containerName, testCommand.command, &stdout, &stderr); err != nil {
										t.Log(stderr.String())
										t.Fatal(err)
									}

									if !testCommand.testCommandStdoutFn(stdout) {
										t.Fatal(fmt.Errorf("Command %v running in container %s produced unexpected output on stdout: %s", testCommand.command, testCommand.containerName, stdout.String()))
									}
								}
							}
						}
					}

					tc.assert.HasPodVM(t, tc.pod.Name)
				}
			}
			return ctx
		}).
		Teardown(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			client, err := cfg.NewClient()
			if err != nil {
				t.Fatal(err)
			}
			if tc.configMap != nil {
				if err = client.Resources().Delete(ctx, tc.configMap); err != nil {
					t.Fatal(err)
				}

				log.Infof("Deleting Configmap... %s", tc.configMap.Name)
			}

			if tc.secret != nil {
				if err = client.Resources().Delete(ctx, tc.secret); err != nil {
					t.Fatal(err)
				} else {
					log.Infof("Deleting Secret... %s", tc.secret.Name)
				}
			}

			if tc.job != nil {
				var podlist v1.PodList
				if err := client.Resources(tc.job.Namespace).List(ctx, &podlist); err != nil {
					t.Fatal(err)
				}
				if err = client.Resources().Delete(ctx, tc.job); err != nil {
					t.Fatal(err)
				} else {
					log.Infof("Deleting Job... %s", tc.job.Name)
				}
				for _, pod := range podlist.Items {
					if pod.ObjectMeta.Labels["job-name"] == tc.job.Name {
						if err = client.Resources().Delete(ctx, &pod); err != nil {
							t.Fatal(err)
						}
						log.Infof("Deleting pods created by job... %s", pod.ObjectMeta.Name)

					}
				}
			}

			if tc.pod != nil {
				if err = client.Resources().Delete(ctx, tc.pod); err != nil {
					t.Fatal(err)
				}
				log.Infof("Deleting pod... %s", tc.pod.Name)

			}

			if tc.pvc != nil {
				if err = client.Resources().Delete(ctx, tc.pvc); err != nil {
					t.Fatal(err)
				} else {
					log.Infof("Deleting PVC... %s", tc.pvc.Name)
				}
			}

			return ctx
		}).Feature()
	testEnv.Test(tc.testing, testCaseFeature)
}

func newTestCase(t *testing.T, testName string, assert CloudAssert, assessMessage string) *testCase {
	testCase := &testCase{
		testing:        t,
		testName:       testName,
		assert:         assert,
		assessMessage:  assessMessage,
		podState:       v1.PodRunning,
		imagePullTimer: false,
	}

	return testCase
}

func reverseSlice(slice []string) []string {
	length := len(slice)
	for i := 0; i < length/2; i++ {
		slice[i], slice[length-i-1] = slice[length-i-1], slice[i]
	}
	return slice
}

// timeExtractor for comparing and extracting time from a Log String
func timeExtractor(log string) (string, error) {
	matchString := regexp.MustCompile(`\b(\d{2}):(\d{2}):(\d{2})\b`).FindStringSubmatch(log)
	if len(matchString) != 4 {
		return "", errors.New("Invalid Time Data")
	}
	return matchString[0], nil
}

func watchImagePullTime(ctx context.Context, client klient.Client, caaPod v1.Pod, Pod v1.Pod) (string, error) {
	pullingtime := ""
	podLogString := ""
	var startTime, endTime time.Time
	clientset, err := kubernetes.NewForConfig(client.RESTConfig())
	if err != nil {
		return "", err
	}

	if Pod.Status.Phase == v1.PodRunning {
		req := clientset.CoreV1().Pods(caaPod.ObjectMeta.Namespace).GetLogs(caaPod.ObjectMeta.Name, &v1.PodLogOptions{})
		podLogs, err := req.Stream(ctx)
		if err != nil {
			return "", err
		}
		defer podLogs.Close()
		buf := new(bytes.Buffer)
		_, err = io.Copy(buf, podLogs)
		if err != nil {
			return "", err
		}
		podLogString = buf.String()

		if podLogString != "" {
			podLogSlice := reverseSlice(strings.Split(podLogString, "\n"))
			for _, i := range podLogSlice {
				if strings.Contains(i, "calling PullImage for \""+Pod.Spec.Containers[0].Image+"\"") {
					timeString, err := timeExtractor(i)
					if err != nil {
						return "", err
					}
					startTime, err = time.Parse("15:04:05", timeString)
					if err != nil {
						return "", err
					}
					break
				}
				if strings.Contains(i, "successfully pulled image \""+Pod.Spec.Containers[0].Image+"\"") {
					timeString, err := timeExtractor(i)
					if err != nil {
						return "", err
					}
					endTime, err = time.Parse("15:04:05", timeString)
					if err != nil {
						return "", err
					}
				}
			}
		} else {
			return "", errors.New("Pod Failed to Log expected Output")
		}
	} else {
		return "", errors.New("Pod Failed to Start")
	}

	pullingtime = endTime.Sub(startTime).String()
	return pullingtime, nil
}

func comparePodLogString(ctx context.Context, client klient.Client, customPod v1.Pod, expectedPodlogString string) (string, error) {
	podLogString := ""
	var podlist v1.PodList
	clientset, err := kubernetes.NewForConfig(client.RESTConfig())
	if err != nil {
		return podLogString, err
	}
	if err := client.Resources(customPod.Namespace).List(ctx, &podlist); err != nil {
		return podLogString, err
	}
	//adding sleep time to intialize container and ready for logging
	time.Sleep(5 * time.Second)
	for _, pod := range podlist.Items {
		if pod.ObjectMeta.Name == customPod.Name {
			func() {
				req := clientset.CoreV1().Pods(customPod.Namespace).GetLogs(pod.ObjectMeta.Name, &v1.PodLogOptions{})
				podLogs, err := req.Stream(ctx)
				if err != nil {
					return
				}
				defer podLogs.Close()
				buf := new(bytes.Buffer)
				_, err = io.Copy(buf, podLogs)
				if err != nil {
					return
				}
				podLogString = strings.TrimSpace(buf.String())
			}()
		}
	}

	if err != nil {
		return podLogString, err
	}

	if !strings.Contains(podLogString, expectedPodlogString) {
		return podLogString, errors.New("Error: Pod Log doesn't contain Expected String")
	}

	return podLogString, nil
}

func getSuccessfulAndErroredPods(ctx context.Context, t *testing.T, client klient.Client, job batchv1.Job) (int, int, string, error) {
	podLogString := ""
	errorPod := 0
	successPod := 0
	var podlist v1.PodList
	clientset, err := kubernetes.NewForConfig(client.RESTConfig())
	if err != nil {
		return 0, 0, "", err
	}
	if err := client.Resources(job.Namespace).List(ctx, &podlist); err != nil {
		return 0, 0, "", err
	}
	for _, pod := range podlist.Items {
		if pod.ObjectMeta.Labels["job-name"] == job.Name && pod.Status.Phase == v1.PodPending {
			if pod.Status.ContainerStatuses[0].State.Waiting.Reason == "ContainerCreating" {
				return 0, 0, "", errors.New("Failed to Create PodVM")
			}
		}
		if pod.ObjectMeta.Labels["job-name"] == job.Name && pod.Status.ContainerStatuses[0].State.Terminated.Reason == "StartError" {
			errorPod++
			t.Log("WARNING:", pod.ObjectMeta.Name, "-", pod.Status.ContainerStatuses[0].State.Terminated.Reason)
		}
		if pod.ObjectMeta.Labels["job-name"] == job.Name && pod.Status.ContainerStatuses[0].State.Terminated.Reason == "Completed" {
			successPod++
			watcher, err := clientset.CoreV1().Events(job.Namespace).Watch(ctx, metav1.ListOptions{})
			if err != nil {
				return 0, 0, "", err
			}
			defer watcher.Stop()
			for event := range watcher.ResultChan() {
				if event.Object.(*v1.Event).Reason == "Started" && pod.Status.ContainerStatuses[0].State.Terminated.Reason == "Completed" {
					func() {
						req := clientset.CoreV1().Pods(job.Namespace).GetLogs(pod.ObjectMeta.Name, &v1.PodLogOptions{})
						podLogs, err := req.Stream(ctx)
						if err != nil {
							return
						}
						defer podLogs.Close()
						buf := new(bytes.Buffer)
						_, err = io.Copy(buf, podLogs)
						if err != nil {
							return
						}
						podLogString = strings.TrimSpace(buf.String())
					}()
					t.Log("SUCCESS:", pod.ObjectMeta.Name, "-", pod.Status.ContainerStatuses[0].State.Terminated.Reason, "- LOG:", podLogString)
					break
				}
			}
		}
	}

	return successPod, errorPod, podLogString, nil
}

// doTestCreateSimplePod tests a simple peer-pod can be created.
func doTestCreateSimplePod(t *testing.T, assert CloudAssert) {
	namespace := envconf.RandomName("default", 7)
	pod := newNginxPod(namespace)
	newTestCase(t, "SimplePeerPod", assert, "PodVM is created").withPod(pod).run()
}

func doTestCreatePodWithConfigMap(t *testing.T, assert CloudAssert) {
	namespace := envconf.RandomName("default", 7)
	configMapName := "nginx-config"
	configMapFileName := "example.txt"
	configMapPath := "/etc/config/" + configMapFileName
	configMapContents := "Hello, world"
	configMapData := map[string]string{configMapFileName: configMapContents}
	pod := newNginxPodWithConfigMap(namespace, configMapName)
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

	newTestCase(t, "ConfigMapPeerPod", assert, "Configmap is created and contains data").withPod(pod).withConfigMap(configMap).withTestCommands(testCommands).run()
}

func doTestCreatePodWithSecret(t *testing.T, assert CloudAssert) {
	//doTestCreatePod(t, assert, "Secret is created and contains data", pod)
	namespace := envconf.RandomName("default", 7)
	secretName := "nginx-secret"
	podKubeSecretsDir := "/etc/secret/"
	usernameFileName := "username"
	username := "admin"
	usernamePath := podKubeSecretsDir + usernameFileName
	passwordFileName := "password"
	password := "password"
	passwordPath := podKubeSecretsDir + passwordFileName
	secretData := map[string][]byte{passwordFileName: []byte(password), usernameFileName: []byte(username)}
	pod := newNginxPodWithSecret(namespace, secretName)
	secret := newSecret(namespace, secretName, secretData)

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

	newTestCase(t, "SecretPeerPod", assert, "Secret has been created and contains data").withPod(pod).withSecret(secret).withTestCommands(testCommands).run()
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
	namespace := envconf.RandomName("default", 7)
	podName := "user-pod"
	imageName := "quay.io/confidential-containers/test-images:testuser"
	pod := newPod(namespace, podName, podName, imageName, withRestartPolicy(v1.RestartPolicyOnFailure))
	expectedPodLogString := "otheruser"
	newTestCase(t, "UserPeerPod", assert, "Peer pod with user has been created").withPod(pod).withExpectedPodLogString(expectedPodLogString).withCustomPodState(v1.PodSucceeded).run()
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
	pod := newPod(namespace, podName, podName, imageName, withRestartPolicy(v1.RestartPolicyOnFailure), withEnvironmentalVariables([]v1.EnvVar{{Name: "ISPRODUCTION", Value: "true"}}), withCommand([]string{"/bin/sh", "-c", "env"}))
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
	newTestCase(t, "PeerPodWithPVCAndCSIWrapper", assert, "PVC is created and mounted as expected").withPod(pod).withPVC(myPVC).withTestCommands(testCommands).run()
}
