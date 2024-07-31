// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/klient/wait/conditions"
	"sigs.k8s.io/e2e-framework/pkg/env"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"
)

const WAIT_POD_RUNNING_TIMEOUT = time.Second * 600
const WAIT_JOB_RUNNING_TIMEOUT = time.Second * 600

// TestCommand is a list of commands to execute inside the pod container,
// each with a function to test if the command outputs the value the test
// expects it to on the stdout stream
type TestCommand struct {
	Command             []string
	TestCommandStdoutFn func(stdout bytes.Buffer) bool
	TestCommandStderrFn func(stderr bytes.Buffer) bool
	TestErrorFn         func(errorMsg error) bool
	ContainerName       string
}

type ExtraPod struct {
	pod                  *v1.Pod
	imagePullTimer       bool
	expectedPodLogString string
	isAuth               bool
	testInstanceTypes    InstanceValidatorFunctions
	podState             v1.PodPhase
	testCommands         []TestCommand
}

type InstanceValidatorFunctions struct {
	testSuccessfn func(instance string) bool
	testFailurefn func(error error) bool
}

type TestCase struct {
	testing              *testing.T
	testEnv              env.Environment
	testName             string
	assert               CloudAssert
	assessMessage        string
	pod                  *v1.Pod
	extraPods            []*ExtraPod
	configMap            *v1.ConfigMap
	secret               *v1.Secret
	extraSecrets         []*v1.Secret
	pvc                  *v1.PersistentVolumeClaim
	job                  *batchv1.Job
	service              *v1.Service
	testCommands         []TestCommand
	expectedPodLogString string
	podState             v1.PodPhase
	imagePullTimer       bool
	isAuth               bool
	AuthImageStatus      string
	deletionWithin       time.Duration
	testInstanceTypes    InstanceValidatorFunctions
	isNydusSnapshotter   bool
	FailReason           string
}

func (tc *TestCase) WithConfigMap(configMap *v1.ConfigMap) *TestCase {
	tc.configMap = configMap
	return tc
}

func (tc *TestCase) WithSecret(secret *v1.Secret) *TestCase {
	tc.secret = secret
	return tc
}

func (tc *TestCase) WithExtraSecrets(secrets []*v1.Secret) *TestCase {
	tc.extraSecrets = secrets
	return tc
}

func (tc *TestCase) WithPVC(pvc *v1.PersistentVolumeClaim) *TestCase {
	tc.pvc = pvc
	return tc
}

func (tc *TestCase) WithJob(job *batchv1.Job) *TestCase {
	tc.job = job
	return tc
}

func (tc *TestCase) WithPod(pod *v1.Pod) *TestCase {
	tc.pod = pod
	return tc
}

func (tc *TestCase) WithExtraPods(pods []*ExtraPod) *TestCase {
	tc.extraPods = pods
	return tc
}

func (tc *TestCase) WithService(service *v1.Service) *TestCase {
	tc.service = service
	return tc
}

func (tc *TestCase) WithDeleteAssertion(duration *time.Duration) *TestCase {
	tc.deletionWithin = *duration
	return tc
}

func (tc *TestCase) WithTestCommands(TestCommands []TestCommand) *TestCase {
	tc.testCommands = TestCommands
	return tc
}

func (tc *TestCase) WithInstanceTypes(testInstanceTypes InstanceValidatorFunctions) *TestCase {
	tc.testInstanceTypes = testInstanceTypes
	return tc
}

func (pod *ExtraPod) WithTestCommands(TestCommands []TestCommand) *ExtraPod {
	pod.testCommands = TestCommands
	return pod
}

func (tc *TestCase) WithExpectedPodLogString(expectedPodLogString string) *TestCase {
	tc.expectedPodLogString = expectedPodLogString
	return tc
}

func (tc *TestCase) WithCustomPodState(customPodState v1.PodPhase) *TestCase {
	tc.podState = customPodState
	return tc
}

func (tc *TestCase) WithPodWatcher() *TestCase {
	tc.imagePullTimer = true
	return tc
}

func (tc *TestCase) WithAuthenticatedImage() *TestCase {
	tc.isAuth = true
	return tc
}

func (tc *TestCase) WithAuthImageStatus(status string) *TestCase {
	tc.AuthImageStatus = status
	return tc
}

func (tc *TestCase) WithNydusSnapshotter() *TestCase {
	tc.isNydusSnapshotter = true
	return tc
}

func (tc *TestCase) WithFailReason(reason string) *TestCase {
	tc.FailReason = reason
	return tc
}

func (tc *TestCase) Run() {
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

			if os.Getenv("REGISTRY_CREDENTIAL_ENCODED") != "" {
				providerName := os.Getenv("CLOUD_PROVIDER")
				authfile, err := os.ReadFile("../../install/overlays/" + providerName + "/auth.json")
				if err != nil {
					t.Fatal(err)
				}
				secretData := map[string][]byte{v1.DockerConfigJsonKey: authfile}
				secret := NewSecret(E2eNamespace, DEFAULT_AUTH_SECRET, secretData, v1.SecretTypeDockerConfigJson)
				if err = client.Resources().Create(ctx, secret); err != nil {
					t.Fatal(err)
				}
				if err = AddImagePullSecretToDefaultServiceAccount(ctx, client, DEFAULT_AUTH_SECRET); err != nil {
					t.Fatal(err)
				}
			}

			if tc.secret != nil {
				if err = client.Resources().Create(ctx, tc.secret); err != nil {
					t.Fatal(err)
				}
			}

			if tc.extraSecrets != nil {
				for _, extraSecret := range tc.extraSecrets {
					if err = client.Resources().Create(ctx, extraSecret); err != nil {
						t.Fatal(err)
					}
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

			if tc.AuthImageStatus == "WithoutCredentials" {
				clientSet, err := kubernetes.NewForConfig(client.RESTConfig())
				if err != nil {
					t.Fatal(err)
				}
				_, err = clientSet.CoreV1().Secrets(E2eNamespace).Get(ctx, DEFAULT_AUTH_SECRET, metav1.GetOptions{})
				if err == nil {
					t.Logf("Deleting pre-existing %v...", DEFAULT_AUTH_SECRET)
					if err = clientSet.CoreV1().Secrets(E2eNamespace).Delete(ctx, DEFAULT_AUTH_SECRET, metav1.DeleteOptions{}); err != nil {
						t.Fatal(err)
					}
					t.Logf("Creating empty %v...", DEFAULT_AUTH_SECRET)
					if err = client.Resources().Create(ctx, &v1.Secret{ObjectMeta: metav1.ObjectMeta{Name: DEFAULT_AUTH_SECRET, Namespace: E2eNamespace}, Type: v1.SecretTypeOpaque}); err != nil {
						t.Fatal(err)
					}
				}
			}

			if tc.pod != nil {
				if err = client.Resources().Create(ctx, tc.pod); err != nil {
					t.Fatal(err)
				}
				if err = wait.For(conditions.New(client.Resources()).PodPhaseMatch(tc.pod, tc.podState), wait.WithTimeout(WAIT_POD_RUNNING_TIMEOUT)); err != nil {
					t.Fatal(err)
				}
				if tc.podState == v1.PodRunning || len(tc.testCommands) > 0 {
					t.Logf("Waiting for containers in pod: %v are ready", tc.pod.Name)
					if err = wait.For(conditions.New(client.Resources()).ContainersReady(tc.pod), wait.WithTimeout(WAIT_POD_RUNNING_TIMEOUT)); err != nil {
						//Added logs for debugging nightly tests
						clientset, err := kubernetes.NewForConfig(client.RESTConfig())
						if err != nil {
							t.Fatal(err)
						}
						pod, err := clientset.CoreV1().Pods(tc.pod.Namespace).Get(ctx, tc.pod.Name, metav1.GetOptions{})
						if err != nil {
							t.Fatal(err)
						}
						t.Logf("Expected Pod State: %v", tc.podState)
						yamlData, err := yaml.Marshal(pod.Status)
						if err != nil {
							t.Logf("Error marshaling pod.Status to YAML: %v", err.Error())
						} else {
							t.Logf("Current Pod State: %v", string(yamlData))
						}
						if pod.Status.Phase == v1.PodRunning {
							t.Logf("Log of the pod %.v \n===================\n", pod.Name)
							podLogString, _ := GetPodLog(ctx, client, *pod)
							t.Log(podLogString)
							t.Logf("===================\n")
						}
						t.Fatal(err)
					}
				}
			}
			if tc.service != nil {
				if err = client.Resources().Create(ctx, tc.service); err != nil {
					t.Fatal(err)
				}
				clusterIP := WaitForClusterIP(t, client, tc.service)
				t.Logf("webserver service is available on cluster IP: %s", clusterIP)
			}
			if tc.extraPods != nil {
				for _, extraPod := range tc.extraPods {
					t.Logf("Provision extra pod %s", extraPod.pod.Name)
					err := ProvisionPod(ctx, client, t, extraPod.pod, extraPod.podState, extraPod.testCommands)
					if err != nil {
						t.Fatal(err)
					}
				}
			}

			return ctx
		}).
		Assess(tc.assessMessage, func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			client := cfg.Client()
			var podlist v1.PodList

			if tc.job != nil {
				conditions := tc.job.Status.Conditions
				if len(conditions) == 1 && conditions[0].Type == batchv1.JobFailed {
					t.Errorf("Job failed")
				}

				if err := client.Resources(tc.job.Namespace).List(ctx, &podlist); err != nil {
					t.Fatal(err)
				}
				successPod, errorPod, podLogString, err := GetSuccessfulAndErroredPods(ctx, t, client, *tc.job)
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
						t.Logf("Output Log from Pod: %s", podLogString)
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
							imagePullTime, err := WatchImagePullTime(ctx, client, caaPod, *tc.pod)
							if err != nil {
								t.Fatal(err)
							}
							t.Logf("Time Taken to pull 4GB Image: %s", imagePullTime)
							break
						}
					}
				}

				if tc.expectedPodLogString != "" {
					LogString, err := ComparePodLogString(ctx, client, *tc.pod, tc.expectedPodLogString)
					if err != nil {
						t.Logf("Output:%s", LogString)
						t.Fatal(err)
					}
					t.Logf("Log output of peer pod:%s", LogString)
				}

				if tc.isAuth {
					if err := GetAuthenticatedImageStatus(ctx, client, tc.AuthImageStatus, *tc.pod); err != nil {
						t.Fatal(err)
					}

					t.Logf("PodVM has successfully reached %v state with authenticated Image - %v", tc.AuthImageStatus, os.Getenv("AUTHENTICATED_REGISTRY_IMAGE"))
				}

				if tc.testInstanceTypes.testSuccessfn != nil && tc.testInstanceTypes.testFailurefn != nil {
					if err := client.Resources(tc.pod.Namespace).List(ctx, &podlist); err != nil {
						t.Fatal(err)
					}

					for _, podItem := range podlist.Items {
						if podItem.ObjectMeta.Name == tc.pod.Name {
							profile, error := tc.assert.GetInstanceType(t, tc.pod.Name)
							if error != nil {
								if error.Error() == "Failed to Create PodVM Instance" {
									podEvent, err := PodEventExtractor(ctx, client, *tc.pod)
									if err != nil {
										t.Fatal(err)
									}
									if !tc.testInstanceTypes.testFailurefn(errors.New(podEvent.EventDescription)) {
										t.Fatal(fmt.Errorf("Pod Failed to execute expected error message %v", error.Error()))
									}
								} else {
									t.Fatal(error)
								}

							}
							if profile != "" {
								t.Logf("PodVM Created with Instance Type: %v", profile)
								if !tc.testInstanceTypes.testSuccessfn(profile) {
									t.Fatal(fmt.Errorf("PodVM Created with Different Instance Type %v", profile))
								}
							}
							break
						} else {
							t.Fatal("Pod Not Found...")
						}
					}

				}

				if tc.podState == v1.PodRunning {
					if err := client.Resources(tc.pod.Namespace).List(ctx, &podlist); err != nil {
						t.Fatal(err)
					}
					if len(tc.testCommands) > 0 {
						if len(tc.testCommands) > 0 {
							logString, err := AssessPodTestCommands(ctx, client, tc.pod, tc.testCommands)
							t.Logf("Output when execute test commands: %s", logString)
							if err != nil {
								t.Fatal(err)
							}
						}
					}

					tc.assert.HasPodVM(t, tc.pod.Name)
				}

				if tc.podState != v1.PodRunning && tc.podState != v1.PodSucceeded {
					profile, error := tc.assert.GetInstanceType(t, tc.pod.Name)
					if error != nil {
						if error.Error() == "Failed to Create PodVM Instance" {
							_, err := PodEventExtractor(ctx, client, *tc.pod)
							if err != nil {
								t.Fatal(err)
							}
						} else {
							t.Fatal(error)
						}
					} else if profile != "" {
						t.Logf("PodVM Created with Instance Type %v", profile)
						if tc.FailReason != "" {
							var podlist v1.PodList
							var podLogString string
							if err := client.Resources("confidential-containers-system").List(ctx, &podlist); err != nil {
								t.Fatal(err)
							}
							for _, pod := range podlist.Items {
								if pod.Labels["app"] == "cloud-api-adaptor" {
									podLogString, _ = GetPodLog(ctx, client, pod)
									break
								}
							}
							if strings.Contains(podLogString, tc.FailReason) {
								t.Logf("failed due to expected reason %s", tc.FailReason)
							} else {
								t.Logf("cloud-api-adaptor pod logs: %s", podLogString)
								yamlData, err := yaml.Marshal(tc.pod.Status)
								if err != nil {
									log.Errorf("Error marshaling pod.Status to JSON: %s", err.Error())
								} else {
									t.Logf("Current Pod State: %v", string(yamlData))
								}
								t.Fatal("failed due to unknown reason")
							}
						} else {
							t.Logf("Pod Failed If you want to cross check please give .WithFailReason(error string)")
						}
					}
				}

				if tc.isNydusSnapshotter {
					nodeName, err := GetNodeNameFromPod(ctx, client, *tc.pod)
					if err != nil {
						t.Fatal(err)
					}
					log.Tracef("Test pod running on node %s", nodeName)

					containerId := tc.pod.Status.ContainerStatuses[0].ContainerID
					containerId, found := strings.CutPrefix(containerId, "containerd://")
					if !found {
						t.Fatal("unexpected container id format")
					}

					usedNydusSnapshotter, err := IsPulledWithNydusSnapshotter(ctx, t, client, nodeName, containerId)
					if err != nil {
						t.Fatal(err)
					}
					if !usedNydusSnapshotter {
						t.Fatal("Expected to pull with nydus, but that didn't happen")
					}
				}
			}

			if tc.extraPods != nil {
				for _, extraPod := range tc.extraPods {
					if extraPod.imagePullTimer {
						// TBD
						t.Fatal("Please implement assess logic for imagePullTimer")
					}
					if extraPod.expectedPodLogString != "" {
						LogString, err := ComparePodLogString(ctx, client, *extraPod.pod, extraPod.expectedPodLogString)
						if err != nil {
							t.Logf("Output:%s", LogString)
							t.Fatal(err)
						}
						t.Logf("Log output of peer pod:%s", LogString)
					}
					if extraPod.isAuth {
						// TBD
						t.Fatal("Error: isAuth hasn't been implemented in extraPods. Please implement assess function for isAuth")
					}
					if extraPod.testInstanceTypes.testSuccessfn != nil && extraPod.testInstanceTypes.testFailurefn != nil {
						// TBD
						t.Fatal("Error: testInstanceTypes hasn't been implemented in extraPods. Please implement assess for function testInstanceTypes.")
					}
					if extraPod.podState == v1.PodRunning {
						if len(extraPod.testCommands) > 0 {
							logString, err := AssessPodTestCommands(ctx, client, extraPod.pod, extraPod.testCommands)
							t.Logf("Output when execute test commands:%s", logString)
							if err != nil {
								t.Fatal(err)
							}
						}
						tc.assert.HasPodVM(t, extraPod.pod.Name)
					}

					if tc.isNydusSnapshotter {
						// TBD
						t.Fatal("Error: isNydusSnapshotter hasn't been implemented in extraPods. Please implement assess function for isNydusSnapshotter.")
					}

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

				t.Logf("Deleting Configmap... %s", tc.configMap.Name)
			}

			if tc.secret != nil {
				if err = client.Resources().Delete(ctx, tc.secret); err != nil {
					t.Fatal(err)
				} else {
					t.Logf("Deleting Secret... %s", tc.secret.Name)
				}
			}

			if os.Getenv("REGISTRY_CREDENTIAL_ENCODED") != "" {
				clientSet, err := kubernetes.NewForConfig(client.RESTConfig())
				if err != nil {
					t.Fatal(err)
				}
				if err = clientSet.CoreV1().Secrets(E2eNamespace).Delete(ctx, DEFAULT_AUTH_SECRET, metav1.DeleteOptions{}); err != nil {
					t.Fatal(err)
				}
			}

			if tc.extraSecrets != nil {
				for _, extraSecret := range tc.extraSecrets {
					if err = client.Resources().Delete(ctx, extraSecret); err != nil {
						t.Fatal(err)
					} else {
						t.Logf("Deleting extra Secret... %s", extraSecret.Name)
					}
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
					t.Logf("Deleting Job... %s", tc.job.Name)
				}
				for _, pod := range podlist.Items {
					if pod.ObjectMeta.Labels["job-name"] == tc.job.Name {
						if err = client.Resources().Delete(ctx, &pod); err != nil {
							t.Fatal(err)
						}
						t.Logf("Deleting pods created by job... %s", pod.ObjectMeta.Name)

					}
				}
			}

			if tc.pod != nil {
				if err = client.Resources().Delete(ctx, tc.pod); err != nil {
					t.Fatal(err)
				}
				t.Logf("Deleting pod %s...", tc.pod.Name)
				if err = wait.For(conditions.New(
					client.Resources()).ResourceDeleted(tc.pod),
					wait.WithInterval(5*time.Second),
					wait.WithTimeout(tc.deletionWithin)); err != nil {
					t.Fatal(err)
				}
				t.Logf("Pod %s has been successfully deleted within %.0fs", tc.pod.Name, tc.deletionWithin.Seconds())
			}

			if tc.extraPods != nil {
				for _, extraPod := range tc.extraPods {
					pod := extraPod.pod
					t.Logf("Deleting pod %s...", pod.Name)
					err := DeletePod(ctx, client, pod, &tc.deletionWithin)
					if err != nil {
						t.Logf("Error occurs when delete pod: %s", extraPod.pod.Name)
						t.Fatal(err)
					}
					t.Logf("Pod %s has been successfully deleted within %.0fs", pod.Name, tc.deletionWithin.Seconds())
				}
			}

			if tc.pvc != nil {
				if err = client.Resources().Delete(ctx, tc.pvc); err != nil {
					t.Fatal(err)
				} else {
					t.Logf("Deleting PVC... %s", tc.pvc.Name)
				}
			}

			if tc.service != nil {
				if err = client.Resources().Delete(ctx, tc.service); err != nil {
					t.Fatal(err)
				} else {
					t.Logf("Deleting Service... %s", tc.service.Name)
				}
			}

			return ctx
		}).Feature()

	tc.testEnv.Test(tc.testing, testCaseFeature)
}
