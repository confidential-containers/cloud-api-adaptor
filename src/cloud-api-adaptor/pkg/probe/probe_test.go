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
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
)

var timeBefore time.Time = time.Now()
var timeStart time.Time = timeBefore.Add(time.Second * 10)
var timeAfter time.Time = timeStart.Add(time.Second * 10)

// Mock a successful clientset
func getFakeClientSetWithParas(podnamePrefix, namespace, nodeName, runtimeClass string, status corev1.ConditionStatus, transitionTime time.Time) *fake.Clientset {
	clientset := fake.NewClientset(&corev1.Pod{
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

const closedSocket = "/tmp/caa-probe-test-no-such-socket.sock"

// listenOnSocket returns the path of a socket that accepts connections until
// the test ends.
func listenOnSocket(t *testing.T) string {
	t.Helper()
	socketPath := "/tmp/caa-probe-test-socket.sock"
	socket, err := net.Listen("unix", socketPath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = socket.Close() })
	return socketPath
}

// clientsetWithPeerPods returns a clientset with two peer pods on node-name-1
// whose Ready condition has the given status.
func clientsetWithPeerPods(ready corev1.ConditionStatus) *fake.Clientset {
	return getFakeClientSetWithParas("pod", "default", "node-name-1", DefaultCCRuntimeClassName, ready, timeAfter)
}

func statusOf(handler http.HandlerFunc, path string) int {
	rr := httptest.NewRecorder()
	handler(rr, httptest.NewRequest(http.MethodGet, path, nil))
	return rr.Code
}

func TestStartupHandler(t *testing.T) {
	t.Setenv("NODE_NAME", "node-name-1")

	t.Run("fails while the socket does not accept connections", func(t *testing.T) {
		checker = Checker{
			Clientset:        clientsetWithPeerPods(corev1.ConditionTrue),
			RuntimeclassName: DefaultCCRuntimeClassName,
			SocketPath:       closedSocket,
		}

		assert.Equal(t, http.StatusInternalServerError, statusOf(StartupHandler, "/startup"))
	})

	t.Run("succeeds while peer pods are not ready", func(t *testing.T) {
		checker = Checker{
			Clientset:        clientsetWithPeerPods(corev1.ConditionFalse),
			RuntimeclassName: DefaultCCRuntimeClassName,
			SocketPath:       listenOnSocket(t),
		}

		assert.Equal(t, http.StatusOK, statusOf(StartupHandler, "/startup"))
	})
}

func TestReadinessHandler(t *testing.T) {
	t.Setenv("NODE_NAME", "node-name-1")

	t.Run("fails while the socket does not accept connections", func(t *testing.T) {
		podsReadizProbesDone = false
		checker = Checker{
			Clientset:        clientsetWithPeerPods(corev1.ConditionTrue),
			RuntimeclassName: DefaultCCRuntimeClassName,
			SocketPath:       closedSocket,
		}

		assert.Equal(t, http.StatusInternalServerError, statusOf(ReadinessHandler, "/ready"))
	})

	t.Run("fails while a peer pod is not ready", func(t *testing.T) {
		podsReadizProbesDone = false
		checker = Checker{
			Clientset:        clientsetWithPeerPods(corev1.ConditionFalse),
			RuntimeclassName: DefaultCCRuntimeClassName,
			SocketPath:       listenOnSocket(t),
		}

		assert.Equal(t, http.StatusInternalServerError, statusOf(ReadinessHandler, "/ready"))
	})

	t.Run("succeeds when every peer pod is ready", func(t *testing.T) {
		podsReadizProbesDone = false
		checker = Checker{
			Clientset:        clientsetWithPeerPods(corev1.ConditionTrue),
			RuntimeclassName: DefaultCCRuntimeClassName,
			SocketPath:       listenOnSocket(t),
		}

		assert.Equal(t, http.StatusOK, statusOf(ReadinessHandler, "/ready"))
	})

	t.Run("stays ready when a peer pod later stops being ready", func(t *testing.T) {
		podsReadizProbesDone = false
		checker = Checker{
			Clientset:        clientsetWithPeerPods(corev1.ConditionTrue),
			RuntimeclassName: DefaultCCRuntimeClassName,
			SocketPath:       listenOnSocket(t),
		}
		require.Equal(t, http.StatusOK, statusOf(ReadinessHandler, "/ready"))

		checker.Clientset = clientsetWithPeerPods(corev1.ConditionFalse)

		assert.Equal(t, http.StatusOK, statusOf(ReadinessHandler, "/ready"))
	})

	t.Run("fails when the pods cannot be listed", func(t *testing.T) {
		clientset := fake.NewClientset()
		clientset.CoreV1().(*fakecorev1.FakeCoreV1).PrependReactor("list", "pods", func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
			return true, nil, errors.New("cannot list pods")
		})
		podsReadizProbesDone = false
		checker = Checker{
			Clientset:        clientset,
			RuntimeclassName: DefaultCCRuntimeClassName,
			SocketPath:       listenOnSocket(t),
		}

		assert.Equal(t, http.StatusInternalServerError, statusOf(ReadinessHandler, "/ready"))
	})
}
