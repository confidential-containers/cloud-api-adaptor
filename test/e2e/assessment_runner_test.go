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
	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/klient/wait/conditions"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
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
type instanceValidatorFunctions struct {
	testSuccessfn func(instance string) bool
	testFailurefn func(error error) bool
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
	isAuth               bool
	AuthImageStatus      string
	deletionWithin       *time.Duration
	testInstanceTypes    instanceValidatorFunctions
	isNydusSnapshotter   bool
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

func (tc *testCase) withDeleteAssertion(duration *time.Duration) *testCase {
	tc.deletionWithin = duration
	return tc
}

func (tc *testCase) withTestCommands(testCommands []testCommand) *testCase {
	tc.testCommands = testCommands
	return tc
}

func (tc *testCase) withInstanceTypes(testInstanceTypes instanceValidatorFunctions) *testCase {
	tc.testInstanceTypes = testInstanceTypes
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

func (tc *testCase) withAuthenticatedImage() *testCase {
	tc.isAuth = true
	return tc
}

func (tc *testCase) withAuthImageStatus(status string) *testCase {
	tc.AuthImageStatus = status
	return tc
}

func (tc *testCase) withNydusSnapshotter() *testCase {
	tc.isNydusSnapshotter = true
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

			if tc.AuthImageStatus == "WithoutCredentials" {
				clientSet, err := kubernetes.NewForConfig(client.RESTConfig())
				if err != nil {
					t.Fatal(err)
				}
				_, err = clientSet.CoreV1().Secrets("confidential-containers-system").Get(ctx, "auth-json-secret", metav1.GetOptions{})
				if err == nil {
					log.Info("Deleting pre-existing auth-json-secret...")
					if err = clientSet.CoreV1().Secrets("confidential-containers-system").Delete(ctx, "auth-json-secret", metav1.DeleteOptions{}); err != nil {
						t.Fatal(err)
					}
					log.Info("Creating empty auth-json-secret...")
					if err = client.Resources().Create(ctx, &v1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "auth-json-secret", Namespace: "confidential-containers-system"}, Type: v1.SecretTypeOpaque}); err != nil {
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
				if tc.podState == v1.PodRunning || tc.pod.Spec.Containers[0].ReadinessProbe != nil {
					clientset, err := kubernetes.NewForConfig(client.RESTConfig())
					if err != nil {
						t.Fatal(err)
					}
					pod, err := clientset.CoreV1().Pods(tc.pod.Namespace).Get(ctx, tc.pod.Name, metav1.GetOptions{})
					if err != nil {
						t.Fatal(err)
					}
					//Added logs for debugging nightly tests
					t.Logf("Expected Pod State: %v", tc.podState)
					t.Logf("Current Pod State: %v", pod.Status.Phase)
					//Getting Readiness probe of a container
					for i, condition := range pod.Status.Conditions {
						fmt.Printf("===================\n")
						fmt.Printf("Checking Conditons - %v....\n", i+1)
						fmt.Printf("===================\n")
						fmt.Printf("*.Condition Type: %v\n", condition.Type)
						fmt.Printf("*.Condition Status: %v\n", condition.Status)
						fmt.Printf("*.Condition Last Probe Time: %v\n", condition.LastProbeTime)
						fmt.Printf("*.Condition Last Transition Time: %v\n", condition.LastTransitionTime)
						fmt.Printf("*.Condition Last Message: %v\n", condition.Message)
						fmt.Printf("*.Condition Last Reason: %v\n", condition.Reason)
					}

					readinessProbe := pod.Spec.Containers[0].ReadinessProbe
					if readinessProbe != nil {
						fmt.Printf("===================\n")
						fmt.Printf("Checking Readiness Probe....\n")
						fmt.Printf("===================\n")
						fmt.Printf("*.Initial Delay Seconds: %v\n", readinessProbe.InitialDelaySeconds)
						fmt.Printf("*.Timeout Seconds: %v\n", readinessProbe.TimeoutSeconds)
						fmt.Printf("*.Success Threshold: %v\n", readinessProbe.SuccessThreshold)
						fmt.Printf("*.Failure Threshold: %v\n", readinessProbe.FailureThreshold)
						fmt.Printf("*.Period Seconds: %v\n", readinessProbe.PeriodSeconds)
						fmt.Printf("*.Probe Handler: %v\n", readinessProbe.ProbeHandler)
						fmt.Printf("*.Probe Handler Port: %v\n", readinessProbe.ProbeHandler.HTTPGet.Port)
						fmt.Printf("===================\n")
					}
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

				if tc.isAuth {
					if err := getAuthenticatedImageStatus(ctx, client, tc.AuthImageStatus, *tc.pod); err != nil {
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
							profile, error := tc.assert.getInstanceType(t, tc.pod.Name)
							if error != nil {
								if error.Error() == "Failed to Create PodVM Instance" {
									podEvent, err := podEventExtractor(ctx, client, *tc.pod)
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
								if !tc.testInstanceTypes.testSuccessfn(profile) {
									t.Fatal(fmt.Errorf("PodVM Created with Differenct Instance Type %v", profile))
								}
							}

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

				if tc.isNydusSnapshotter {
					nodeName, err := getNodeNameFromPod(ctx, client, *tc.pod)
					if err != nil {
						t.Fatal(err)
					}
					log.Tracef("Test pod running on node %s", nodeName)
					usedNydusSnapshotter, err := IsPulledWithNydusSnapshotter(ctx, t, client, nodeName)
					if err != nil {
						t.Fatal(err)
					}
					if !usedNydusSnapshotter {
						t.Fatal("Expected to pull with nydus, but that didn't happen")
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
				duration := 1 * time.Minute
				if tc.deletionWithin == nil {
					tc.deletionWithin = &duration
				}
				if err = client.Resources().Delete(ctx, tc.pod); err != nil {
					t.Fatal(err)
				}
				log.Infof("Deleting pod %s...", tc.pod.Name)
				if err = wait.For(conditions.New(
					client.Resources()).ResourceDeleted(tc.pod),
					wait.WithInterval(5*time.Second),
					wait.WithTimeout(*tc.deletionWithin)); err != nil {
					t.Fatal(err)
				}
				log.Infof("Pod %s has been successfully deleted within %.0fs", tc.pod.Name, tc.deletionWithin.Seconds())
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
