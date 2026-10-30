// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package probe

import (
	"context"
	"fmt"
	"net"
	"os"
	"time"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Checker struct {
	Clientset        kubernetes.Interface
	RuntimeclassName string
	SocketPath       string
}

func (c *Checker) GetNodeName() string {
	return os.Getenv("NODE_NAME")
}

func (c *Checker) GetAllPods(selector string) (result *corev1.PodList, err error) {
	return c.Clientset.CoreV1().Pods("").List(context.TODO(), metav1.ListOptions{
		FieldSelector: selector,
	})
}

func (c *Checker) GetAllPeerPods(startTime time.Time) (ready bool, err error) {
	nodeName := c.GetNodeName()
	logger.Printf("nodeName: %s", nodeName)

	selector := fmt.Sprintf("spec.nodeName=%s", nodeName)
	pods, err := c.GetAllPods(selector)
	if err != nil {
		return false, err
	}
	logger.Printf("Selected pods count: %d", len(pods.Items))

	for _, pod := range pods.Items {
		if pod.Spec.RuntimeClassName == nil {
			continue
		}
		if *pod.Spec.RuntimeClassName != c.RuntimeclassName {
			continue
		}
		for _, condition := range pod.Status.Conditions {
			if condition.Type != corev1.PodReady {
				continue
			}
			logger.Printf("Dealing with PeerPod: %s, with Ready condition: %v", pod.Name, condition)
			if condition.Status != corev1.ConditionTrue {
				return false, fmt.Errorf("PeerPod %s is not Ready", pod.Name)
			}

			if condition.LastTransitionTime.Time.Before(startTime) {
				return false, fmt.Errorf("PeerPod %s has not been restarted", pod.Name)
			}
		}
	}

	return true, nil
}

func (c *Checker) IsSocketOpen() (open bool, err error) {
	conn, err := net.Dial("unix", c.SocketPath)
	if err != nil {
		return false, err
	}
	defer conn.Close()
	return true, nil
}

func CreateClientset() (kubernetes.Interface, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}

	return kubernetes.NewForConfig(config)
}

func GetRuntimeclassName() string {
	runtimeclassName := os.Getenv("RUNTIMECLASS_NAME")
	if runtimeclassName != "" {
		return runtimeclassName
	}
	return DefaultCCRuntimeClassName
}
