package e2e

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	pv "github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/test/provisioner"
	log "github.com/sirupsen/logrus"
	yaml "gopkg.in/yaml.v2"
	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/e2e-framework/klient"
	"sigs.k8s.io/e2e-framework/klient/k8s"
	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/klient/wait/conditions"
	"sigs.k8s.io/e2e-framework/pkg/env"
)

const waitNamespaceAvailableTimeout = time.Second * 120

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
		return "", errors.New("invalid time data")
	}
	return matchString[0], nil
}

func NewTestCase(t *testing.T, e env.Environment, testName string, assert CloudAssert, assessMessage string) *TestCase {
	testCase := &TestCase{
		testing:        t,
		testEnv:        e,
		testName:       testName,
		assert:         assert,
		assessMessage:  assessMessage,
		podState:       v1.PodRunning,
		imagePullTimer: false,
		deletionWithin: assert.DefaultTimeout(),
	}

	return testCase
}

func NewExtraPod(namespace string, podName string, containerName string, imageName string, options ...PodOption) *ExtraPod {
	basicPod := NewPod(namespace, podName, containerName, imageName)
	for _, option := range options {
		option(basicPod)
	}
	extPod := &ExtraPod{
		pod:      basicPod,
		podState: v1.PodRunning,
	}
	return extPod
}

func WatchImagePullTime(ctx context.Context, client klient.Client, caaPod *v1.Pod, pod *v1.Pod) (string, error) {
	pullingtime := ""
	var startTime, endTime time.Time

	if pod.Status.Phase == v1.PodRunning {
		podLogString, err := GetPodLog(ctx, client, caaPod)
		if err != nil {
			return "", err
		}

		if podLogString != "" {
			podLogSlice := reverseSlice(strings.Split(podLogString, "\n"))
			for _, i := range podLogSlice {
				if strings.Contains(i, "calling PullImage for \""+pod.Spec.Containers[0].Image+"\"") {
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
				if strings.Contains(i, "successfully pulled image \""+pod.Spec.Containers[0].Image+"\"") {
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
			return "", errors.New("pod Failed to Log expected Output")
		}
	} else {
		return "", errors.New("pod Failed to Start")
	}

	pullingtime = endTime.Sub(startTime).String()
	return pullingtime, nil
}

func getCaaPod(ctx context.Context, client klient.Client, t *testing.T, nodeName string) (*v1.Pod, error) {
	caaPod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: pv.GetCAANamespace(),
		},
	}
	pods, err := GetPodNamesByLabel(ctx, client, t, caaPod.GetObjectMeta().GetNamespace(), "app", "cloud-api-adaptor", nodeName)
	if err != nil {
		return nil, fmt.Errorf("getCaaPod: GetPodNamesByLabel failed: %v", err)
	}

	if len(pods.Items) == 0 {
		return nil, fmt.Errorf("getCaaPod: We didn't find the CAA pod")
	} else if len(pods.Items) > 1 {
		return nil, fmt.Errorf("getCaaPod: We found multiple CAA pods: %v", pods.Items)
	}

	caaPod.Name = pods.Items[0].Name
	return caaPod, nil
}

// Check cloud-api-adaptor daemonset pod logs to ensure that something like:
// <date time> [adaptor/proxy]         mount_point:/run/kata-containers/<id>/rootfs source:<image> fstype:overlay driver:image_guest_pull
// <date time> 11:47:42 [adaptor/proxy] CreateContainer: Ignoring PullImage before CreateContainer (cid: "<cid>")
// was output
func IsPulledWithNydusSnapshotter(ctx context.Context, t *testing.T, client klient.Client, nodeName string, containerID string) (bool, error) {
	nydusSnapshotterPullRegex, err := regexp.Compile(`.*mount_point:/run/kata-containers.*` + containerID + `.*driver:image_guest_pull.*$`)
	if err != nil {
		return false, err
	}

	caaPod, err := getCaaPod(ctx, client, t, nodeName)
	if err != nil {
		return false, fmt.Errorf("IsPulledWithNydusSnapshotter: failed to get CAA pod: %v", err)
	}

	podLogString, err := GetPodLog(ctx, client, caaPod)
	if err != nil {
		return false, fmt.Errorf("IsPulledWithNydusSnapshotter: failed to list pods: %v", err)
	}
	podLogSlice := reverseSlice(strings.Split(podLogString, "\n"))
	for _, line := range podLogSlice {
		if nydusSnapshotterPullRegex.MatchString(line) {
			return true, nil
		}
	}
	return false, fmt.Errorf("didn't find pull image for snapshotter")
}

func GetPodLog(ctx context.Context, client klient.Client, pod *v1.Pod) (string, error) {
	clientset, err := kubernetes.NewForConfig(client.RESTConfig())
	if err != nil {
		return "", err
	}

	req := clientset.CoreV1().Pods(pod.ObjectMeta.Namespace).GetLogs(pod.Name, &v1.PodLogOptions{})
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
	return strings.TrimSpace(buf.String()), nil
}

func CompareCaaPodLogStrings(ctx context.Context, t *testing.T, client klient.Client, customPod *v1.Pod, expectedCaaPodLogStrings []string) error {
	nodeName, err := GetNodeNameFromPod(ctx, client, customPod)
	if err != nil {
		return fmt.Errorf("CompareCaaPodLogStrings: GetNodeNameFromPod failed with %v", err)
	}
	for _, expectedCaaPodLogString := range expectedCaaPodLogStrings {
		err = VerifyCaaPodLogContains(ctx, t, client, nodeName, expectedCaaPodLogString)
		if err != nil {
			return fmt.Errorf("looking for '%s' in caa pod logs : failed: %v", expectedCaaPodLogString, err)
		}
	}
	return nil
}

func ComparePodLogString(ctx context.Context, client klient.Client, customPod *v1.Pod, expectedPodLogString string) (string, error) {
	//adding sleep time to initialize container and ready for logging
	time.Sleep(5 * time.Second)

	podLogString, err := getStringFromPod(ctx, client, customPod, GetPodLog)
	if err != nil {
		return "", err
	}

	if !strings.Contains(podLogString, expectedPodLogString) {
		return podLogString, errors.New("error: Pod Log doesn't contain Expected String")
	}

	return podLogString, nil
}

// Note: there are currently two event types: Normal and Warning, so Warning includes errors
func GetPodEventWarningDescriptions(ctx context.Context, client klient.Client, pod *v1.Pod) (string, error) {
	clientset, err := kubernetes.NewForConfig(client.RESTConfig())
	if err != nil {
		return "", err
	}

	events, err := clientset.CoreV1().Events(pod.Namespace).List(ctx, metav1.ListOptions{FieldSelector: fmt.Sprintf("involvedObject.name=%s", pod.Name)})
	if err != nil {
		return "", err
	}

	var descriptionsBuilder strings.Builder
	for _, event := range events.Items {
		if event.Type == v1.EventTypeWarning {
			descriptionsBuilder.WriteString(event.Message)
		}
	}
	return descriptionsBuilder.String(), nil
}

func LogPodDebugInfo(ctx context.Context, t *testing.T, client klient.Client, pod *v1.Pod) {
	podLogString, err := GetPodLog(ctx, client, pod)
	if err != nil {
		t.Error(err)
	}
	t.Logf("Pod log: %s\n", podLogString)

	events, err := GetPodEventWarningDescriptions(ctx, client, pod)
	if err != nil {
		t.Error(err)
	}
	t.Logf("Pod error events: %s\n", events)

	caaPodLog, err := getCaaPodLogForPod(ctx, t, client, pod)
	if err != nil {
		t.Error(err)
	}
	t.Logf("CAA log: %s\n", caaPodLog)
}

// This function takes an expected pod event "warning" string (note warning also covers errors) and checks to see if it
// shows up in the event log of the pod. Some pods error in failed state, so can be immediately checks, others fail
// in waiting state (e.g. ImagePullBackoff errors), so we need to poll for errors showing up on these pods
func ComparePodEventWarningDescriptions(ctx context.Context, t *testing.T, client klient.Client, pod *v1.Pod, expectedPodEvent string) error {
	retries := 1
	delay := 10 * time.Second

	if pod.Status.Phase != v1.PodFailed {
		// If not failed state we might have to wait/retry until the error happens
		retries = int(waitPodRunningTimeout / delay)
	}

	var err error = nil
	for retries > 0 {
		podEventsDescriptions, podErr := getStringFromPod(ctx, client, pod, GetPodEventWarningDescriptions)
		if podErr != nil {
			return podErr
		}

		t.Logf("podEvents: %s\n", podEventsDescriptions)
		if !strings.Contains(podEventsDescriptions, expectedPodEvent) {
			err = fmt.Errorf("error: Pod Events don't contain Expected String %s", expectedPodEvent)
		} else {
			return nil
		}
		retries--
		time.Sleep(delay)
	}
	return err
}

func CompareInstanceType(ctx context.Context, t *testing.T, client klient.Client, pod v1.Pod, expectedInstanceType string, getInstanceTypeFn func(t *testing.T, podName string) (string, error)) error {
	var podlist v1.PodList
	if err := client.Resources(pod.Namespace).List(ctx, &podlist); err != nil {
		return err
	}
	for _, podItem := range podlist.Items {
		if podItem.Name == pod.Name {
			instanceType, err := getInstanceTypeFn(t, pod.Name)
			if err != nil {
				return fmt.Errorf("CompareInstanceType: failed to getCaaPod: %v", err)
			}
			if instanceType == expectedInstanceType {
				return nil
			} else {
				return fmt.Errorf("CompareInstanceType: instance type was %s, but we expected %s ", instanceType, expectedInstanceType)
			}
		}
	}
	return fmt.Errorf("no pod matching %v, was found", pod)
}

func VerifyCaaPodLogContains(ctx context.Context, t *testing.T, client klient.Client, nodeName, expected string) error {
	caaPod, err := getCaaPod(ctx, client, t, nodeName)
	if err != nil {
		return fmt.Errorf("VerifyCaaPodLogContains: failed to getCaaPod: %v", err)
	}

	LogString, err := ComparePodLogString(ctx, client, caaPod, expected)
	if err != nil {
		return fmt.Errorf("VerifyCaaPodLogContains: failed to ComparePodLogString: logString: %s error %v", LogString, err)
	}
	t.Logf("CAA pod log contained the expected string %s", expected)
	return nil
}

func VerifyNydusSnapshotter(ctx context.Context, t *testing.T, client klient.Client, pod *v1.Pod) error {
	nodeName, err := GetNodeNameFromPod(ctx, client, pod)
	if err != nil {
		return fmt.Errorf("VerifyNydusSnapshotter: GetNodeNameFromPod failed with %v", err)
	}
	log.Tracef("Test pod running on node %s", nodeName)

	containerID := pod.Status.ContainerStatuses[0].ContainerID
	containerID, found := strings.CutPrefix(containerID, "containerd://")
	if !found {
		return fmt.Errorf("VerifyNydusSnapshotter: unexpected container id format: %s", containerID)
	}

	usedNydusSnapshotter, err := IsPulledWithNydusSnapshotter(ctx, t, client, nodeName, containerID)
	if err != nil {
		return fmt.Errorf("isPulledWithNydusSnapshotter:  failed with %v", err)
	}
	if !usedNydusSnapshotter {
		return fmt.Errorf("expected to pull with nydus, but that didn't happen")
	}
	return nil
}

func VerifyImagePullTimer(ctx context.Context, t *testing.T, client klient.Client, pod *v1.Pod) error {
	nodeName, err := GetNodeNameFromPod(ctx, client, pod)
	if err != nil {
		return fmt.Errorf("VerifyImagePullTimer: GetNodeNameFromPod failed with %v", err)
	}

	caaPod, err := getCaaPod(ctx, client, t, nodeName)
	if err != nil {
		return fmt.Errorf("VerifyImagePullTimer: failed to getCaaPod: %v", err)
	}

	imagePullTime, err := WatchImagePullTime(ctx, client, caaPod, pod)
	if err != nil {
		return fmt.Errorf("VerifyImagePullTimer: WatchImagePullTime failed with %v", err)
	}
	t.Logf("Time Taken to pull 4GB Image: %s", imagePullTime)
	return nil
}

func GetNodeNameFromPod(ctx context.Context, client klient.Client, customPod *v1.Pod) (string, error) {
	var getNodeName = func(ctx context.Context, client klient.Client, pod *v1.Pod) (string, error) {
		return pod.Spec.NodeName, nil
	}
	return getStringFromPod(ctx, client, customPod, getNodeName)
}

func GetPodsFromJob(ctx context.Context, t *testing.T, client klient.Client, job *batchv1.Job) (*v1.PodList, error) {
	clientset, err := kubernetes.NewForConfig(client.RESTConfig())
	if err != nil {
		return nil, fmt.Errorf("GetPodsFromJob: get Kubernetes clientSet failed: %v", err)
	}

	pods, err := clientset.CoreV1().Pods(job.Namespace).List(context.TODO(), metav1.ListOptions{LabelSelector: "job-name=" + job.Name})
	if err != nil {
		return nil, fmt.Errorf("GetPodsFromJob: get pod list failed: %v", err)
	}

	return pods, nil
}

func GetSuccessfulAndErroredPods(ctx context.Context, t *testing.T, client klient.Client, job batchv1.Job) (int, int, string, error) {
	podLogString := ""
	errorPod := 0
	successPod := 0
	clientset, err := kubernetes.NewForConfig(client.RESTConfig())
	if err != nil {
		return 0, 0, "", err
	}
	podList, err := GetPodsFromJob(ctx, t, client, &job)
	if err != nil {
		return 0, 0, "", err
	}
	for _, pod := range podList.Items {
		if pod.Status.Phase == v1.PodPending {
			if pod.Status.ContainerStatuses[0].State.Waiting.Reason == "ContainerCreating" {
				return 0, 0, "", errors.New("failed to Create PodVM")
			}
		}
		if pod.Status.ContainerStatuses[0].State.Terminated.Reason == "StartError" {
			errorPod++
			t.Log("WARNING:", pod.Name, "-", pod.Status.ContainerStatuses[0].State.Terminated.Reason)
		}
		if pod.Status.ContainerStatuses[0].State.Terminated.Reason == "Completed" {
			successPod++
			watcher, err := clientset.CoreV1().Events(job.Namespace).Watch(ctx, metav1.ListOptions{})
			if err != nil {
				return 0, 0, "", err
			}
			defer watcher.Stop()
			for event := range watcher.ResultChan() {
				if event.Object.(*v1.Event).Reason == "Started" && pod.Status.ContainerStatuses[0].State.Terminated.Reason == "Completed" {
					func() {
						req := clientset.CoreV1().Pods(job.Namespace).GetLogs(pod.Name, &v1.PodLogOptions{})
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
					t.Log("SUCCESS:", pod.Name, "-", pod.Status.ContainerStatuses[0].State.Terminated.Reason, "- LOG:", podLogString)
					break
				}
			}
		}
	}

	return successPod, errorPod, podLogString, nil
}

func getCaaPodLogForPod(ctx context.Context, t *testing.T, client klient.Client, pod *v1.Pod) (string, error) {
	nodeName, err := GetNodeNameFromPod(ctx, client, pod)
	if err != nil {
		return "", fmt.Errorf("GetCaaPodLog: GetNodeNameFromPod failed with %v", err)
	}
	caaPod, err := getCaaPod(ctx, client, t, nodeName)
	if err != nil {
		return "", fmt.Errorf("GetCaaPodLog: failed to getCaaPod: %v", err)
	}
	podLogString, err := getStringFromPod(ctx, client, caaPod, GetPodLog)
	if err != nil {
		return "", fmt.Errorf("GetCaaPodLog: failed to getStringFromPod: %v", err)
	}

	// Find the logs starting with the pod
	// e.g. 2024/12/19 17:18:52 [adaptor/cloud] create a sandbox 27e11ff35fd1284b45d3be30b42f435a9a597c322bb66e965785c003338d792a for pod job-pi-fgr78 in namespace coco-pp-e2e-test-9a6697df
	dateMatcher := "[0-9]{4}/[0-9]{2}/[0-9]{2}"
	timeMatcher := "([0-1]?[0-9]|2[0-3]):[0-5][0-9]:[0-5][0-9]"
	podMatcher := regexp.MustCompile(dateMatcher + " " + timeMatcher + ` \[adaptor\/cloud\] create a sandbox [0-9|a-f]* for pod ` + pod.Name)
	index := podMatcher.FindStringIndex(podLogString)[0]
	if index < 0 {
		return "", fmt.Errorf("GetCaaPodLog: couldn't find pod log matcher: %s in CAA log %s", podMatcher, podLogString)
	} else {
		podLogString = podLogString[index:]
	}

	return podLogString, nil
}

// SkipTestOnCI skips the test if running on CI
func SkipTestOnCI(t *testing.T) {
	ci := os.Getenv("CI")

	if ci == "true" {
		t.Skip("Failing on CI")
	}
}

func IsStringEmpty(data string) bool {
	if data == "" {
		return true
	} else {
		return false
	}
}

func IsErrorEmpty(err error) bool {
	if err == nil {
		return true
	} else {
		return false
	}
}

func IsBufferEmpty(buffer bytes.Buffer) bool {
	if buffer.String() == "" {
		return true
	} else {
		return false
	}
}

func AssessPodRequestAndLimit(ctx context.Context, client klient.Client, pod *v1.Pod) error {
	// Check if the pod has the "kata.peerpods.io/vm request and limit with value "1"

	podVMExtResource := "kata.peerpods.io/vm"

	request := pod.Spec.Containers[0].Resources.Requests[v1.ResourceName(podVMExtResource)]
	limit := pod.Spec.Containers[0].Resources.Limits[v1.ResourceName(podVMExtResource)]

	// Check if the request and limit are set to "1"
	if request.Cmp(resource.MustParse("1")) != 0 {
		return fmt.Errorf("request for podvm extended resource is not set to 1")
	}
	if limit.Cmp(resource.MustParse("1")) != 0 {
		return fmt.Errorf("limit for podvm extended resource is not set to 1")
	}

	return nil

}

func findPod(ctx context.Context, client klient.Client, pod *v1.Pod) (*v1.Pod, error) {
	var podList v1.PodList
	if err := client.Resources(pod.Namespace).List(ctx, &podList); err != nil {
		return nil, fmt.Errorf("failed to list pod, error : %s", err.Error())
	}
	for _, podItem := range podList.Items {
		if podItem.Name == pod.Name {
			return &podItem, nil
		}
	}
	return nil, fmt.Errorf("pod not found with name %s in namespace %s", pod.Name, pod.Namespace)
}

func AssessPodTestCommands(t *testing.T, ctx context.Context, client klient.Client, pod *v1.Pod, testCommands []TestCommand) error {
	pod, err := findPod(ctx, client, pod)
	if err != nil {
		return err
	}
	//adding sleep time to intialize container and ready for Executing commands
	time.Sleep(5 * time.Second)
	for _, testCommand := range testCommands {
		err := assessPodTestCommand(t, ctx, client, pod, testCommand)
		if err != nil {
			return err
		}
	}
	return nil
}

func assessPodTestCommand(t *testing.T, ctx context.Context, client klient.Client, pod *v1.Pod, testCommand TestCommand) error {
	log.Tracef("Running test command: %v", testCommand)
	var stdout, stderr bytes.Buffer
	if err := client.Resources(pod.Namespace).ExecInPod(ctx, pod.Namespace, pod.Name, testCommand.ContainerName, testCommand.Command, &stdout, &stderr); err != nil {
		if testCommand.TestErrorFn != nil {
			if !testCommand.TestErrorFn(err) {
				t.Logf("Output when execute test command : %s", err.Error())
				return fmt.Errorf("command %v running in container %s produced unexpected output on error: %s, stderr: %s", testCommand.Command, testCommand.ContainerName, err.Error(), stderr.String())
			}
		} else {
			t.Logf("Output when execute test command : %s", err.Error())
			return fmt.Errorf("command %v running in container %s produced unexpected output on error: %s, stderr: %s", testCommand.Command, testCommand.ContainerName, err.Error(), stderr.String())
		}
	} else if testCommand.TestErrorFn != nil {
		return fmt.Errorf("we expected an error from Pod %s, but it was not found", pod.Name)
	}
	if testCommand.TestCommandStderrFn != nil {
		if !testCommand.TestCommandStderrFn(stderr) {
			t.Logf("Output when execute test command : %s", stderr.String())
			return fmt.Errorf("command %v running in container %s produced unexpected output on stderr: %s, stdout: %s", testCommand.Command, testCommand.ContainerName, stderr.String(), stdout.String())
		} else {
			t.Logf("Output when execute test command : %s", stderr.String())
		}
	}
	if testCommand.TestCommandStdoutFn != nil {
		if !testCommand.TestCommandStdoutFn(stdout) {
			t.Logf("Output when execute test command : %s", stdout.String())
			return fmt.Errorf("command %v running in container %s produced unexpected output on stdout: %s, stderr: %s", testCommand.Command, testCommand.ContainerName, stdout.String(), stderr.String())
		} else {
			t.Logf("Output when execute test command : %s", stdout.String())
		}
	}
	return nil
}

func ProvisionPod(ctx context.Context, client klient.Client, t *testing.T, pod *v1.Pod, podState v1.PodPhase, testCommands []TestCommand) error {
	if err := client.Resources().Create(ctx, pod); err != nil {
		t.Fatal(err)
	}
	if err := wait.For(conditions.New(client.Resources()).PodPhaseMatch(pod, podState), wait.WithTimeout(waitPodRunningTimeout)); err != nil {
		t.Fatal(err)
	}
	if podState == v1.PodRunning || len(testCommands) > 0 {
		t.Logf("Waiting for containers in pod: %v are ready", pod.Name)
		if err := wait.For(conditions.New(client.Resources()).ContainersReady(pod), wait.WithTimeout(waitPodRunningTimeout)); err != nil {
			//Added logs for debugging nightly tests
			clientset, err := kubernetes.NewForConfig(client.RESTConfig())
			if err != nil {
				t.Fatal(err)
			}
			actualPod, err := clientset.CoreV1().Pods(pod.Namespace).Get(ctx, pod.Name, metav1.GetOptions{})
			if err != nil {
				t.Fatal(err)
			}
			t.Logf("Expected Pod State: %v", podState)
			yamlData, err := yaml.Marshal(actualPod.Status)
			if err != nil {
				fmt.Println("Error marshaling pod.Status to YAML: ", err.Error())
			} else {
				t.Logf("Current Pod State: %v", string(yamlData))
			}
			if actualPod.Status.Phase == v1.PodRunning {
				fmt.Printf("Log of the pod %.v \n===================\n", actualPod.Name)
				podLogString, _ := GetPodLog(ctx, client, actualPod)
				fmt.Println(podLogString)
				fmt.Printf("===================\n")
			}
			t.Fatal(err)
		}
	}
	return nil
}

func DeletePod(ctx context.Context, client klient.Client, pod *v1.Pod, tcDelDuration *time.Duration) error {
	duration := 1 * time.Minute
	if tcDelDuration == nil {
		tcDelDuration = &duration
	}
	if err := client.Resources().Delete(ctx, pod); err != nil {
		return err
	}
	if err := wait.For(conditions.New(
		client.Resources()).ResourceDeleted(pod),
		wait.WithInterval(5*time.Second),
		wait.WithTimeout(*tcDelDuration)); err != nil {
		return err
	}
	return nil
}

func CreateAndWaitForNamespace(ctx context.Context, client klient.Client, namespaceName string) error {
	log.Infof("Creating namespace '%s'...", namespaceName)
	nsObj := v1.Namespace{}
	nsObj.Name = namespaceName
	if err := client.Resources().Create(ctx, &nsObj); err != nil {
		return err
	}

	if err := waitForNamespaceToBeUseable(ctx, client, namespaceName); err != nil {
		return err
	}
	return nil
}

func waitForNamespaceToBeUseable(ctx context.Context, client klient.Client, namespaceName string) error {
	log.Infof("Wait for namespace '%s' be ready...", namespaceName)
	nsObj := v1.Namespace{}
	nsObj.Name = namespaceName
	if err := wait.For(conditions.New(client.Resources()).ResourceMatch(&nsObj, func(object k8s.Object) bool {
		ns, ok := object.(*v1.Namespace)
		if !ok {
			log.Printf("Not a namespace object: %v", object)
			return false
		}
		return ns.Status.Phase == v1.NamespaceActive
	}), wait.WithTimeout(waitNamespaceAvailableTimeout)); err != nil {
		return err
	}

	// SH: There is a race condition where the default service account isn't ready when we
	// try and use it #1657, so we want to ensure that it is available before continuing.
	// As the serviceAccount doesn't have a status I can't seem to use the wait condition to
	// detect if it is ready, so do things the old-fashioned way
	log.Infof("Wait for default serviceaccount in namespace '%s'...", namespaceName)
	var saList v1.ServiceAccountList
	for start := time.Now(); time.Since(start) < waitNamespaceAvailableTimeout; {
		if err := client.Resources(namespaceName).List(ctx, &saList); err != nil {
			return err
		}
		for _, sa := range saList.Items {
			if sa.Name == "default" {

				log.Infof("default serviceAccount exists, namespace '%s' is ready for use", namespaceName)
				return nil
			}
		}
		log.Tracef("default serviceAccount not found after %.0f seconds", time.Since(start).Seconds())
		time.Sleep(5 * time.Second)
	}
	return fmt.Errorf("default service account not found in namespace '%s' after %.0f seconds wait", namespaceName, waitNamespaceAvailableTimeout.Seconds())
}

func DeleteAndWaitForNamespace(ctx context.Context, client klient.Client, namespaceName string) error {
	nsObj := v1.Namespace{}
	nsObj.Name = namespaceName
	if err := client.Resources().Delete(ctx, &nsObj); err != nil {
		return err
	}
	log.Infof("Deleting namespace '%s'...", nsObj.Name)
	if err := wait.For(conditions.New(
		client.Resources()).ResourceDeleted(&nsObj),
		wait.WithInterval(5*time.Second),
		wait.WithTimeout(60*time.Second)); err != nil {
		return err
	}
	log.Infof("Namespace '%s' has been successfully deleted within 60s", nsObj.Name)
	return nil
}

func getDefaultServiceAccount(ctx context.Context, client klient.Client) (*v1.ServiceAccount, error) {
	clientSet, err := kubernetes.NewForConfig(client.RESTConfig())
	if err != nil {
		return nil, err
	}
	serviceAccount, err := clientSet.CoreV1().ServiceAccounts(E2eNamespace).Get(context.TODO(), "default", metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	return serviceAccount, nil
}

func setImagePullSecretsOnServiceAccount(ctx context.Context, client klient.Client, serviceAccount *v1.ServiceAccount, imagePullSecrets []v1.LocalObjectReference) error {
	clientSet, err := kubernetes.NewForConfig(client.RESTConfig())
	if err != nil {
		return err
	}
	serviceAccount.ImagePullSecrets = imagePullSecrets
	_, err = clientSet.CoreV1().ServiceAccounts(E2eNamespace).Update(context.TODO(), serviceAccount, metav1.UpdateOptions{})
	if err != nil {
		return err
	}
	return nil
}

func AddImagePullSecretToDefaultServiceAccount(ctx context.Context, client klient.Client, secretName string) error {
	serviceAccount, err := getDefaultServiceAccount(ctx, client)
	if err != nil {
		return err
	}
	secretExists := false
	for _, secret := range serviceAccount.ImagePullSecrets {
		if secret.Name == secretName {
			secretExists = true
			break
		}
	}
	if !secretExists {
		imagePullSecrets := append(serviceAccount.ImagePullSecrets, v1.LocalObjectReference{Name: secretName})
		err := setImagePullSecretsOnServiceAccount(ctx, client, serviceAccount, imagePullSecrets)
		if err != nil {
			return err
		}
	}
	return nil
}

func RemoveImagePullSecretFromDefaultServiceAccount(ctx context.Context, client klient.Client, secretName string) error {
	serviceAccount, err := getDefaultServiceAccount(ctx, client)
	if err != nil {
		return err
	}
	newSecrets := []v1.LocalObjectReference{}
	for _, secret := range serviceAccount.ImagePullSecrets {
		if secret.Name != secretName {
			newSecrets = append(newSecrets, secret)
		}
	}
	err = setImagePullSecretsOnServiceAccount(ctx, client, serviceAccount, newSecrets)
	if err != nil {
		return err
	}
	return nil
}

func GetPodNamesByLabel(ctx context.Context, client klient.Client, t *testing.T, namespace string, labelName string, labelValue string, nodeName string) (*v1.PodList, error) {

	clientset, err := kubernetes.NewForConfig(client.RESTConfig())
	if err != nil {
		return nil, fmt.Errorf("GetPodNamesByLabel: get Kubernetes clientSet failed: %v", err)
	}

	nodeSelector := fmt.Sprintf("spec.nodeName=%s", nodeName)
	pods, err := clientset.CoreV1().Pods(namespace).List(context.TODO(), metav1.ListOptions{LabelSelector: labelName + "=" + labelValue, FieldSelector: nodeSelector})
	if err != nil {
		return nil, fmt.Errorf("GetPodNamesByLabel: get pod list failed: %v", err)
	}

	return pods, nil
}

type podToString func(context.Context, klient.Client, *v1.Pod) (string, error)

func getStringFromPod(ctx context.Context, client klient.Client, pod *v1.Pod, fn podToString) (string, error) {
	var podlist v1.PodList
	if err := client.Resources(pod.Namespace).List(ctx, &podlist); err != nil {
		return "", err
	}
	for _, podItem := range podlist.Items {
		if podItem.Name == pod.Name {
			return fn(ctx, client, &podItem)
		}
	}
	return "", fmt.Errorf("no pod matching %v, was found", pod)
}
