// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	yaml "gopkg.in/yaml.v2"
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
const WAIT_PVC_RUNNING_TIMEOUT = time.Second * 30

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
	podState             v1.PodPhase
	testCommands         []TestCommand
}

type TestCase struct {
	testing                     *testing.T
	testEnv                     env.Environment
	testName                    string
	assert                      CloudAssert
	assessMessage               string
	pod                         *v1.Pod
	extraPods                   []*ExtraPod
	configMap                   *v1.ConfigMap
	secret                      *v1.Secret
	extraSecrets                []*v1.Secret
	pvc                         *v1.PersistentVolumeClaim
	job                         *batchv1.Job
	service                     *v1.Service
	testCommands                []TestCommand
	expectedCaaPodLogStrings    []string
	expectedPodLogString        string
	expectedPodEventErrorString string
	expectedPodvmConsoleLog     string
	podState                    v1.PodPhase
	imagePullTimer              bool
	saImagePullSecret           string
	deletionWithin              time.Duration
	expectedInstanceType        string
	isNydusSnapshotter          bool
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

func (tc *TestCase) WithExpectedInstanceType(expectedInstanceType string) *TestCase {
	tc.expectedInstanceType = expectedInstanceType
	return tc
}

func (pod *ExtraPod) WithTestCommands(TestCommands []TestCommand) *ExtraPod {
	pod.testCommands = TestCommands
	return pod
}

func (tc *TestCase) WithExpectedPodvmConsoleLog(expectedPodvmConsoleLog string) *TestCase {
	tc.expectedPodvmConsoleLog = expectedPodvmConsoleLog
	return tc
}

func (tc *TestCase) WithExpectedCaaPodLogStrings(expectedCaaPodLogStrings ...string) *TestCase {
	tc.expectedCaaPodLogStrings = expectedCaaPodLogStrings
	return tc
}

func (tc *TestCase) WithExpectedPodLogString(expectedPodLogString string) *TestCase {
	tc.expectedPodLogString = expectedPodLogString
	return tc
}

func (tc *TestCase) WithExpectedPodEventError(expectedPodEventMessage string) *TestCase {
	tc.expectedPodEventErrorString = expectedPodEventMessage
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

func (tc *TestCase) WithSAImagePullSecret(secretName string) *TestCase {
	tc.saImagePullSecret = secretName
	return tc
}

func (tc *TestCase) WithNydusSnapshotter() *TestCase {
	tc.isNydusSnapshotter = true
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
				if err = WaitForPVCBound(client, tc.pvc, WAIT_PVC_RUNNING_TIMEOUT); err != nil {
					t.Log(err)
				}
			}

			if tc.saImagePullSecret != "" {
				if err = AddImagePullSecretToDefaultServiceAccount(ctx, client, tc.saImagePullSecret); err != nil {
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
				var podvmName string
				if tc.expectedPodvmConsoleLog != "" {
					for i := range 10 {
						podvmName, err = getPodvmName(ctx, client, tc.pod)
						if err != nil {
							t.Logf("%d attempt: to getPodvmName failed: %v", i, err)
							time.Sleep(2 * time.Second)
						} else {
							break
						}
					}
					if podvmName != "" {
						t.Log("Verifying PodVM console log")
						tc.assert.VerifyPodvmConsole(t, podvmName, tc.expectedPodvmConsoleLog)
					} else {
						t.Logf("Warning: Failed to validated as podvmName is failed")
					}
				}
				if err = wait.For(conditions.New(client.Resources()).PodPhaseMatch(tc.pod, tc.podState), wait.WithTimeout(WAIT_POD_RUNNING_TIMEOUT)); err != nil {
					t.Error(err)
				}
				if tc.podState == v1.PodRunning || len(tc.testCommands) > 0 {
					t.Logf("Waiting for containers in pod: %v are ready", tc.pod.Name)
					if err = wait.For(conditions.New(client.Resources()).ContainersReady(tc.pod), wait.WithTimeout(WAIT_POD_RUNNING_TIMEOUT)); err != nil {
						//Added logs for debugging nightly tests
						clientset, err := kubernetes.NewForConfig(client.RESTConfig())
						if err != nil {
							t.Error(err)
						}
						pod, err := clientset.CoreV1().Pods(tc.pod.Namespace).Get(ctx, tc.pod.Name, metav1.GetOptions{})
						if err != nil {
							t.Error(err)
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
							podLogString, _ := GetPodLog(ctx, client, pod)
							t.Log(podLogString)
							t.Logf("===================\n")
						}
						t.Error(err)
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
					err := VerifyImagePullTimer(ctx, t, client, tc.pod)
					if err != nil {
						t.Error(err)
					}
				}

				if tc.expectedPodLogString != "" {
					logString, err := ComparePodLogString(ctx, client, tc.pod, tc.expectedPodLogString)
					if err != nil {
						t.Errorf("Looking for %s, in pod log: %s, failed with: %v", tc.expectedPodLogString, logString, err)
					}
				}

				if len(tc.expectedCaaPodLogStrings) > 0 {
					err := CompareCaaPodLogStrings(ctx, t, client, tc.pod, tc.expectedCaaPodLogStrings)
					if err != nil {
						t.Errorf("CompareCaaPodLogStrings, failed with: %v", err)
					}
				}

				if tc.expectedPodEventErrorString != "" {
					err := ComparePodEventWarningDescriptions(ctx, t, client, tc.pod, tc.expectedPodEventErrorString)
					if err != nil {
						t.Errorf("Looking for %s, in pod events log, failed with: %v", tc.expectedPodEventErrorString, err)
					}
				} else {
					// There shouldn't have been any pod event warnings/errors
					warnings, err := GetPodEventWarningDescriptions(ctx, client, tc.pod)
					if err != nil {
						t.Errorf("We hit an error trying to get the event log of %s", tc.pod.Name)
					}
					if warnings != "" {
						t.Errorf("unexpected warning/error event(s): %s", warnings)
					}
				}

				if tc.expectedInstanceType != "" {
					err := CompareInstanceType(ctx, t, client, *tc.pod, tc.expectedInstanceType, tc.assert.GetInstanceType)
					if err != nil {
						t.Errorf("CompareInstanceType failed: %v", err)
					}
				}

				if tc.podState == v1.PodRunning {
					if len(tc.testCommands) > 0 {
						err := AssessPodTestCommands(t, ctx, client, tc.pod, tc.testCommands)
						if err != nil {
							t.Errorf("AssessPodTestCommands failed with error: %v", err)
						}
					}

					err := AssessPodRequestAndLimit(ctx, client, tc.pod)
					if err != nil {
						t.Errorf("request and limit for podvm extended resource are not set to 1: %v", err)
					}
					podvmName, err := getPodvmName(ctx, client, tc.pod)
					if err != nil {
						t.Errorf("getPodvmName failed: %v", err)
					}
					tc.assert.HasPodVM(t, podvmName)
				}

				if tc.isNydusSnapshotter {
					err := VerifyNydusSnapshotter(ctx, t, client, tc.pod)
					if err != nil {
						t.Errorf("VerifyNydusSnapshotter failed: %v", err)
					}
				}
			}

			if tc.extraPods != nil {
				for _, extraPod := range tc.extraPods {
					if extraPod.imagePullTimer {
						// TBD
						t.Error("Please implement assess logic for imagePullTimer")
					}
					if extraPod.expectedPodLogString != "" {
						LogString, err := ComparePodLogString(ctx, client, extraPod.pod, extraPod.expectedPodLogString)
						if err != nil {
							t.Logf("Output:%s", LogString)
							t.Error(err)
						}
						t.Logf("Log output of peer pod:%s", LogString)
					}

					if extraPod.podState == v1.PodRunning {
						if len(extraPod.testCommands) > 0 {
							err := AssessPodTestCommands(t, ctx, client, extraPod.pod, extraPod.testCommands)
							if err != nil {
								t.Errorf("AssessPodTestCommands failed with error: %v", err)
							}

						}

						podvmName, err := getPodvmName(ctx, client, extraPod.pod)
						if err != nil {
							t.Errorf("getPodvmName failed: %v", err)
						}
						tc.assert.HasPodVM(t, podvmName)
					}

					if tc.isNydusSnapshotter {
						// TBD
						t.Error("Error: isNydusSnapshotter hasn't been implemented in extraPods. Please implement assess function for isNydusSnapshotter.")
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
					t.Error(err)
				}

				t.Logf("Deleting Configmap... %s", tc.configMap.Name)
			}

			if tc.secret != nil {
				if err = client.Resources().Delete(ctx, tc.secret); err != nil {
					t.Error(err)
				} else {
					t.Logf("Deleting Secret... %s", tc.secret.Name)
				}
			}

			if tc.saImagePullSecret != "" {
				if err = RemoveImagePullSecretFromDefaultServiceAccount(ctx, client, tc.saImagePullSecret); err != nil {
					t.Fatal(err)
				}
			}

			if tc.extraSecrets != nil {
				for _, extraSecret := range tc.extraSecrets {
					if err = client.Resources().Delete(ctx, extraSecret); err != nil {
						t.Error(err)
					} else {
						t.Logf("Deleting extra Secret... %s", extraSecret.Name)
					}
				}

			}

			if tc.job != nil {
				podList, err := GetPodsFromJob(ctx, t, client, tc.job)
				if err != nil {
					t.Error(err)
				}

				if t.Failed() {
					if len(podList.Items) > 0 {
						jobPod := podList.Items[0]
						LogPodDebugInfo(ctx, t, client, &jobPod)
					}
				}

				if err = client.Resources().Delete(ctx, tc.job); err != nil {
					t.Error(err)
				} else {
					t.Logf("Deleting Job... %s", tc.job.Name)
				}
				for _, pod := range podList.Items {
					if err = client.Resources().Delete(ctx, &pod); err != nil {
						t.Error(err)
					}
					t.Logf("Deleting pods created by job... %s", pod.ObjectMeta.Name)
				}
			}

			if tc.pod != nil {

				if t.Failed() {
					LogPodDebugInfo(ctx, t, client, tc.pod)
				}

				if err = client.Resources().Delete(ctx, tc.pod); err != nil {
					t.Error(err)
				}
				t.Logf("Deleting pod %s...", tc.pod.Name)
				if err = wait.For(conditions.New(
					client.Resources()).ResourceDeleted(tc.pod),
					wait.WithInterval(5*time.Second),
					wait.WithTimeout(tc.deletionWithin)); err != nil {
					t.Error(err)
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
						t.Error(err)
					}
					t.Logf("Pod %s has been successfully deleted within %.0fs", pod.Name, tc.deletionWithin.Seconds())
				}
			}

			if tc.pvc != nil {
				if err = client.Resources().Delete(ctx, tc.pvc); err != nil {
					t.Error(err)
				} else {
					t.Logf("Deleting PVC... %s", tc.pvc.Name)
				}
			}

			if tc.service != nil {
				if err = client.Resources().Delete(ctx, tc.service); err != nil {
					t.Error(err)
				} else {
					t.Logf("Deleting Service... %s", tc.service.Name)
				}
			}

			return ctx
		}).Feature()

	tc.testEnv.Test(tc.testing, testCaseFeature)
}
