// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package probe

import (
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	fakecorev1 "k8s.io/client-go/kubernetes/typed/core/v1/fake"
	k8stesting "k8s.io/client-go/testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
)

var timeBefore time.Time = time.Now()
var timeStart time.Time = timeBefore.Add(time.Second * 10)
var timeAfter time.Time = timeStart.Add(time.Second * 10)

// Mock a successful clientset
func getFakeClientSetWithParas(podnamePrefix, namespace, nodeName, runtimeClass string, status corev1.ConditionStatus, transitionTime time.Time) *fake.Clientset {
	clientset := fake.NewSimpleClientset(&corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podnamePrefix + "-1",
			Namespace: namespace,
		},
		Spec: corev1.PodSpec{
			NodeName:         nodeName,
			RuntimeClassName: &runtimeClass,
		},
		Status: corev1.PodStatus{
			Conditions: []corev1.PodCondition{
				{
					Type:               corev1.PodReady,
					Status:             status,
					LastTransitionTime: metav1.Time{Time: transitionTime},
				},
			},
		},
	}, &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podnamePrefix + "-2",
			Namespace: namespace,
		},
		Spec: corev1.PodSpec{
			NodeName:         nodeName,
			RuntimeClassName: &runtimeClass,
		},
		Status: corev1.PodStatus{
			Conditions: []corev1.PodCondition{
				{
					Type:               corev1.PodReady,
					Status:             status,
					LastTransitionTime: metav1.Time{Time: transitionTime},
				},
			},
		},
	})

	return clientset
}

func Test_GetRuntimeclassName_Default(t *testing.T) {
	os.Setenv("RUNTIMECLASS_NAME", "")
	ret := GetRuntimeclassName()
	assert.Equal(t, ret, DefaultCCRuntimeClassName)
}

func Test_GetRuntimeclassName_Env(t *testing.T) {
	os.Setenv("RUNTIMECLASS_NAME", "runtimeclass-customized")
	ret := GetRuntimeclassName()
	assert.Equal(t, ret, "runtimeclass-customized")
}

func Test_GetAllPeerPods_BeTrue(t *testing.T) {
	os.Setenv("NODE_NAME", "node-name-1")

	clientset := getFakeClientSetWithParas("pod", "default", "node-name-1", DefaultCCRuntimeClassName, corev1.ConditionTrue, timeAfter)
	checker = Checker{
		Clientset:        clientset,
		RuntimeclassName: DefaultCCRuntimeClassName,
		SocketPath:       "",
	}
	result, err := checker.GetAllPeerPods(timeStart)

	assert.NoError(t, err)
	assert.True(t, result)

	clientset = getFakeClientSetWithParas("pod", "default", "node-name-1", "", corev1.ConditionTrue, timeAfter)
	checker = Checker{
		Clientset:        clientset,
		RuntimeclassName: DefaultCCRuntimeClassName,
		SocketPath:       "",
	}
	result, err = checker.GetAllPeerPods(timeStart)

	assert.NoError(t, err)
	assert.True(t, result)
}

func Test_GetAllPeerPods_BeFalse(t *testing.T) {
	os.Setenv("NODE_NAME", "node-name-1")

	clientset := getFakeClientSetWithParas("pod", "default", "node-name-1", DefaultCCRuntimeClassName, corev1.ConditionFalse, timeAfter)
	checker = Checker{
		Clientset:        clientset,
		RuntimeclassName: DefaultCCRuntimeClassName,
		SocketPath:       "",
	}
	result, err := checker.GetAllPeerPods(timeStart)

	assert.NotNil(t, err)
	assert.False(t, result)
}

func Test_GetAllPeerPods_BeFalse_time_before(t *testing.T) {
	os.Setenv("NODE_NAME", "node-name-1")

	clientset := getFakeClientSetWithParas("pod", "default", "node-name-1", DefaultCCRuntimeClassName, corev1.ConditionTrue, timeBefore)
	checker = Checker{
		Clientset:        clientset,
		RuntimeclassName: DefaultCCRuntimeClassName,
		SocketPath:       "",
	}
	result, err := checker.GetAllPeerPods(timeStart)

	assert.NotNil(t, err)
	assert.False(t, result)
}

func Test_GetAllPeerPods_BeError(t *testing.T) {
	os.Setenv("NODE_NAME", "")

	clientset := getFakeClientSetWithParas("pod", "default", "node-name-1", DefaultCCRuntimeClassName, corev1.ConditionFalse, timeAfter)
	checker = Checker{
		Clientset:        clientset,
		RuntimeclassName: DefaultCCRuntimeClassName,
		SocketPath:       "",
	}
	result, err := checker.GetAllPeerPods(timeStart)

	assert.NotNil(t, err)
	assert.False(t, result)
}

func Test_IsSocketOpen_BeOpen(t *testing.T) {
	socketPath := "/tmp/caa-probe-test-socket.sock"
	socket, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatal(err)
	}
	defer socket.Close()

	checker = Checker{
		Clientset:        nil,
		RuntimeclassName: DefaultCCRuntimeClassName,
		SocketPath:       socketPath,
	}

	assert.Nil(t, err)

	opened, err := checker.IsSocketOpen()
	assert.Nil(t, err)
	assert.True(t, opened)
}

func Test_IsSocketOpen_BeNotOpen(t *testing.T) {
	socketPath := "/tmp/caa-probe-test-socket.sock"
	checker = Checker{
		Clientset:        nil,
		RuntimeclassName: DefaultCCRuntimeClassName,
		SocketPath:       socketPath,
	}

	opened, err := checker.IsSocketOpen()
	assert.NotNil(t, err)
	assert.False(t, opened)
}

func Test_StartupHandler_BeReady(t *testing.T) {
	os.Setenv("NODE_NAME", "node-name-1")

	socketPath := "/tmp/caa-probe-test-socket.sock"
	socket, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatal(err)
	}
	defer socket.Close()

	clientset := getFakeClientSetWithParas("pod", "default", "node-name-1", DefaultCCRuntimeClassName, corev1.ConditionFalse, timeAfter)
	checker = Checker{
		Clientset:        clientset,
		RuntimeclassName: DefaultCCRuntimeClassName,
		SocketPath:       socketPath,
	}

	req, err := http.NewRequest("GET", "/startup", nil)
	if err != nil {
		t.Fatal(err)
	}

	podsReadizProbesDone = true
	rr := httptest.NewRecorder()
	http.HandlerFunc(StartupHandler).ServeHTTP(rr, req)

	assert.Equal(t, rr.Code, http.StatusOK)
}

func Test_StartupHandler_NotBeAllPodsReady(t *testing.T) {
	os.Setenv("NODE_NAME", "node-name-1")

	socketPath := "/tmp/caa-probe-test-socket.sock"
	socket, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatal(err)
	}
	defer socket.Close()

	clientset := getFakeClientSetWithParas("pod", "default", "node-name-1", DefaultCCRuntimeClassName, corev1.ConditionFalse, timeAfter)
	checker = Checker{
		Clientset:        clientset,
		RuntimeclassName: DefaultCCRuntimeClassName,
		SocketPath:       socketPath,
	}

	req, err := http.NewRequest("GET", "/startup", nil)
	if err != nil {
		t.Fatal(err)
	}

	podsReadizProbesDone = false
	rr := httptest.NewRecorder()
	http.HandlerFunc(StartupHandler).ServeHTTP(rr, req)

	assert.Equal(t, rr.Code, http.StatusInternalServerError)
}

func Test_StartupHandler_BeAllPodsReady(t *testing.T) {
	os.Setenv("NODE_NAME", "node-name-1")

	socketPath := "/tmp/caa-probe-test-socket.sock"
	socket, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatal(err)
	}
	defer socket.Close()

	clientset := getFakeClientSetWithParas("pod", "default", "node-name-1", DefaultCCRuntimeClassName, corev1.ConditionTrue, timeAfter)
	checker = Checker{
		Clientset:        clientset,
		RuntimeclassName: DefaultCCRuntimeClassName,
		SocketPath:       socketPath,
	}

	req, err := http.NewRequest("GET", "/startup", nil)
	if err != nil {
		t.Fatal(err)
	}

	podsReadizProbesDone = false
	rr := httptest.NewRecorder()
	http.HandlerFunc(StartupHandler).ServeHTTP(rr, req)

	assert.Equal(t, rr.Code, http.StatusOK)
}

func Test_StartupHandler_BeErrorListPods(t *testing.T) {
	os.Setenv("NODE_NAME", "node-name-1")

	socketPath := "/tmp/caa-probe-test-socket.sock"
	socket, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatal(err)
	}
	defer socket.Close()

	clientset := fake.NewSimpleClientset()
	clientset.CoreV1().(*fakecorev1.FakeCoreV1).PrependReactor("list", "pods", func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, nil, errors.New("Error creating secret")
	})

	checker = Checker{
		Clientset:        clientset,
		RuntimeclassName: DefaultCCRuntimeClassName,
		SocketPath:       socketPath,
	}

	req, err := http.NewRequest("GET", "/startup", nil)
	if err != nil {
		t.Fatal(err)
	}

	podsReadizProbesDone = false
	rr := httptest.NewRecorder()
	http.HandlerFunc(StartupHandler).ServeHTTP(rr, req)

	assert.Equal(t, rr.Code, http.StatusInternalServerError)
}
