// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	csiDriverName       = "caa-csi-block-driver"
	csiStorageClassName = "caa-csi-block"
	csiTestNamespace    = "csi-e2e-test"
	podReadyTimeout     = 5 * time.Minute
	podDeleteTimeout    = 3 * time.Minute
	pollInterval        = 5 * time.Second
)

func getClient(t *testing.T) *kubernetes.Clientset {
	t.Helper()
	kubeconfig := os.Getenv("KUBECONFIG")
	if kubeconfig == "" {
		kubeconfig = os.ExpandEnv("$HOME/.kube/config")
	}
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		t.Fatalf("Failed to build kubeconfig: %v", err)
	}
	cs, err := kubernetes.NewForConfig(config)
	if err != nil {
		t.Fatalf("Failed to create k8s client: %v", err)
	}
	return cs
}

func skipIfNoCluster(t *testing.T) {
	t.Helper()
	if os.Getenv("CSI_E2E_ENABLED") != "true" {
		t.Skip("CSI E2E tests disabled — set CSI_E2E_ENABLED=true to run")
	}
}

func ensureNamespace(t *testing.T, cs *kubernetes.Clientset) {
	t.Helper()
	ctx := context.Background()
	_, err := cs.CoreV1().Namespaces().Get(ctx, csiTestNamespace, metav1.GetOptions{})
	if err != nil {
		_, err = cs.CoreV1().Namespaces().Create(ctx, &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{Name: csiTestNamespace},
		}, metav1.CreateOptions{})
		if err != nil {
			t.Fatalf("Failed to create test namespace: %v", err)
		}
	}
}

func waitForPodCompletion(t *testing.T, cs *kubernetes.Clientset, ns, name string, timeout time.Duration) string {
	t.Helper()
	ctx := context.Background()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		pod, err := cs.CoreV1().Pods(ns).Get(ctx, name, metav1.GetOptions{})
		if err == nil && (pod.Status.Phase == corev1.PodSucceeded || pod.Status.Phase == corev1.PodFailed) {
			if pod.Status.Phase == corev1.PodFailed {
				t.Fatalf("Pod %s failed", name)
			}
			logs, _ := cs.CoreV1().Pods(ns).GetLogs(name, &corev1.PodLogOptions{}).Do(ctx).Raw()
			return string(logs)
		}
		time.Sleep(pollInterval)
	}
	t.Fatalf("Pod %s did not complete within %v", name, timeout)
	return ""
}

func deletePodAndWait(t *testing.T, cs *kubernetes.Clientset, ns, name string) {
	t.Helper()
	ctx := context.Background()
	cs.CoreV1().Pods(ns).Delete(ctx, name, metav1.DeleteOptions{}) //nolint:errcheck
	deadline := time.Now().Add(podDeleteTimeout)
	for time.Now().Before(deadline) {
		_, err := cs.CoreV1().Pods(ns).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return
		}
		time.Sleep(pollInterval)
	}
}

func newCSIPVC(namespace, name, size string) *corev1.PersistentVolumeClaim {
	storageClass := csiStorageClassName
	return &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			StorageClassName: &storageClass,
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse(size),
				},
			},
		},
	}
}

func newCSIWriterPod(namespace, podName, pvcName, mountPath, writeData string) *corev1.Pod {
	runtimeClassName := "kata-remote"
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: namespace,
			Labels:    map[string]string{"app": "csi-writer"},
		},
		Spec: corev1.PodSpec{
			RuntimeClassName: &runtimeClassName,
			Containers: []corev1.Container{{
				Name:    "writer",
				Image:   "busybox:1.36",
				Command: []string{"/bin/sh", "-c", fmt.Sprintf("echo '%s' > %s/data.txt && sync && cat %s/data.txt", writeData, mountPath, mountPath)},
				VolumeMounts: []corev1.VolumeMount{
					{Name: "data", MountPath: mountPath},
				},
			}},
			Volumes: []corev1.Volume{{
				Name: "data",
				VolumeSource: corev1.VolumeSource{
					PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: pvcName},
				},
			}},
			RestartPolicy: corev1.RestartPolicyNever,
		},
	}
}

func newCSIReaderPod(namespace, podName, pvcName, mountPath string) *corev1.Pod {
	runtimeClassName := "kata-remote"
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: namespace,
			Labels:    map[string]string{"app": "csi-reader"},
		},
		Spec: corev1.PodSpec{
			RuntimeClassName: &runtimeClassName,
			Containers: []corev1.Container{{
				Name:    "reader",
				Image:   "busybox:1.36",
				Command: []string{"/bin/sh", "-c", fmt.Sprintf("cat %s/data.txt", mountPath)},
				VolumeMounts: []corev1.VolumeMount{
					{Name: "data", MountPath: mountPath},
				},
			}},
			Volumes: []corev1.Volume{{
				Name: "data",
				VolumeSource: corev1.VolumeSource{
					PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: pvcName},
				},
			}},
			RestartPolicy: corev1.RestartPolicyNever,
		},
	}
}

func TestCSISingleDiskPersistence(t *testing.T) {
	skipIfNoCluster(t)
	cs := getClient(t)
	ensureNamespace(t, cs)
	ctx := context.Background()

	pvcName := "e2e-single-pvc"
	testData := "hello-single-disk-e2e"

	t.Log("Creating PVC")
	cs.CoreV1().PersistentVolumeClaims(csiTestNamespace).Delete(ctx, pvcName, metav1.DeleteOptions{}) //nolint:errcheck
	_, err := cs.CoreV1().PersistentVolumeClaims(csiTestNamespace).Create(ctx, newCSIPVC(csiTestNamespace, pvcName, "1Gi"), metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create PVC: %v", err)
	}
	defer cs.CoreV1().PersistentVolumeClaims(csiTestNamespace).Delete(ctx, pvcName, metav1.DeleteOptions{}) //nolint:errcheck

	t.Log("Creating writer pod")
	writerPod := newCSIWriterPod(csiTestNamespace, "e2e-writer", pvcName, "/mnt/data", testData)
	cs.CoreV1().Pods(csiTestNamespace).Delete(ctx, "e2e-writer", metav1.DeleteOptions{}) //nolint:errcheck
	time.Sleep(5 * time.Second)
	_, err = cs.CoreV1().Pods(csiTestNamespace).Create(ctx, writerPod, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create writer pod: %v", err)
	}

	logs := waitForPodCompletion(t, cs, csiTestNamespace, "e2e-writer", podReadyTimeout)
	t.Logf("Writer logs: %s", logs)
	if !strings.Contains(logs, testData) {
		t.Fatalf("Writer output does not contain test data %q", testData)
	}

	t.Log("Deleting writer pod")
	deletePodAndWait(t, cs, csiTestNamespace, "e2e-writer")

	t.Log("Creating reader pod")
	readerPod := newCSIReaderPod(csiTestNamespace, "e2e-reader", pvcName, "/mnt/data")
	_, err = cs.CoreV1().Pods(csiTestNamespace).Create(ctx, readerPod, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create reader pod: %v", err)
	}
	defer deletePodAndWait(t, cs, csiTestNamespace, "e2e-reader")

	logs = waitForPodCompletion(t, cs, csiTestNamespace, "e2e-reader", podReadyTimeout)
	t.Logf("Reader logs: %s", logs)
	if !strings.Contains(logs, testData) {
		t.Fatalf("PERSISTENCE FAILED: reader output %q does not contain %q", strings.TrimSpace(logs), testData)
	}

	t.Log("PASSED: Single disk persistence verified")
}

func TestCSIMultiDiskPersistence(t *testing.T) {
	skipIfNoCluster(t)
	cs := getClient(t)
	ensureNamespace(t, cs)
	ctx := context.Background()

	pvcNames := []string{"e2e-multi-pvc-0", "e2e-multi-pvc-1"}
	testData := []string{"data-on-disk-0", "data-on-disk-1"}

	for _, pvc := range pvcNames {
		cs.CoreV1().PersistentVolumeClaims(csiTestNamespace).Delete(ctx, pvc, metav1.DeleteOptions{}) //nolint:errcheck
		_, err := cs.CoreV1().PersistentVolumeClaims(csiTestNamespace).Create(ctx, newCSIPVC(csiTestNamespace, pvc, "1Gi"), metav1.CreateOptions{})
		if err != nil {
			t.Fatalf("Failed to create PVC %s: %v", pvc, err)
		}
		defer cs.CoreV1().PersistentVolumeClaims(csiTestNamespace).Delete(ctx, pvc, metav1.DeleteOptions{}) //nolint:errcheck
	}

	t.Log("Creating multi-disk writer pod")
	runtimeClassName := "kata-remote"
	cmd := ""
	var mounts []corev1.VolumeMount
	var vols []corev1.Volume
	for i, pvc := range pvcNames {
		mp := fmt.Sprintf("/mnt/%s", pvc)
		mounts = append(mounts, corev1.VolumeMount{Name: pvc, MountPath: mp})
		vols = append(vols, corev1.Volume{Name: pvc, VolumeSource: corev1.VolumeSource{
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: pvc},
		}})
		cmd += fmt.Sprintf("echo '%s' > %s/data.txt && ", testData[i], mp)
	}
	cmd += "sync"
	for _, pvc := range pvcNames {
		cmd += fmt.Sprintf(" && cat /mnt/%s/data.txt", pvc)
	}

	writerPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "e2e-multi-writer", Namespace: csiTestNamespace},
		Spec: corev1.PodSpec{
			RuntimeClassName: &runtimeClassName,
			Containers:       []corev1.Container{{Name: "writer", Image: "busybox:1.36", Command: []string{"/bin/sh", "-c", cmd}, VolumeMounts: mounts}},
			Volumes:          vols,
			RestartPolicy:    corev1.RestartPolicyNever,
		},
	}

	cs.CoreV1().Pods(csiTestNamespace).Delete(ctx, "e2e-multi-writer", metav1.DeleteOptions{}) //nolint:errcheck
	time.Sleep(5 * time.Second)
	_, err := cs.CoreV1().Pods(csiTestNamespace).Create(ctx, writerPod, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create multi-writer pod: %v", err)
	}

	logs := waitForPodCompletion(t, cs, csiTestNamespace, "e2e-multi-writer", podReadyTimeout)
	t.Logf("Multi-writer logs: %s", logs)
	for _, d := range testData {
		if !strings.Contains(logs, d) {
			t.Fatalf("Multi-writer output missing %q", d)
		}
	}

	t.Log("Deleting multi-writer pod")
	deletePodAndWait(t, cs, csiTestNamespace, "e2e-multi-writer")

	t.Log("Creating multi-disk reader pod")
	readCmd := ""
	for _, pvc := range pvcNames {
		readCmd += fmt.Sprintf("cat /mnt/%s/data.txt && ", pvc)
	}
	readCmd += "true"

	readerPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "e2e-multi-reader", Namespace: csiTestNamespace},
		Spec: corev1.PodSpec{
			RuntimeClassName: &runtimeClassName,
			Containers:       []corev1.Container{{Name: "reader", Image: "busybox:1.36", Command: []string{"/bin/sh", "-c", readCmd}, VolumeMounts: mounts}},
			Volumes:          vols,
			RestartPolicy:    corev1.RestartPolicyNever,
		},
	}

	_, err = cs.CoreV1().Pods(csiTestNamespace).Create(ctx, readerPod, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create multi-reader pod: %v", err)
	}
	defer deletePodAndWait(t, cs, csiTestNamespace, "e2e-multi-reader")

	logs = waitForPodCompletion(t, cs, csiTestNamespace, "e2e-multi-reader", podReadyTimeout)
	t.Logf("Multi-reader logs: %s", logs)
	for _, d := range testData {
		if !strings.Contains(logs, d) {
			t.Fatalf("PERSISTENCE FAILED: multi-reader output missing %q", d)
		}
	}

	t.Log("PASSED: Multi-disk persistence verified")
}

func TestCSIVolumeExpansion(t *testing.T) {
	skipIfNoCluster(t)
	cs := getClient(t)
	ensureNamespace(t, cs)
	ctx := context.Background()

	pvcName := "e2e-expand-pvc"

	cs.CoreV1().PersistentVolumeClaims(csiTestNamespace).Delete(ctx, pvcName, metav1.DeleteOptions{}) //nolint:errcheck
	_, err := cs.CoreV1().PersistentVolumeClaims(csiTestNamespace).Create(ctx, newCSIPVC(csiTestNamespace, pvcName, "1Gi"), metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create PVC: %v", err)
	}
	defer cs.CoreV1().PersistentVolumeClaims(csiTestNamespace).Delete(ctx, pvcName, metav1.DeleteOptions{}) //nolint:errcheck

	t.Log("Writing data to volume")
	writerPod := newCSIWriterPod(csiTestNamespace, "e2e-expand-writer", pvcName, "/mnt/data", "expand-test")
	cs.CoreV1().Pods(csiTestNamespace).Delete(ctx, "e2e-expand-writer", metav1.DeleteOptions{}) //nolint:errcheck
	time.Sleep(5 * time.Second)
	_, err = cs.CoreV1().Pods(csiTestNamespace).Create(ctx, writerPod, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create writer pod: %v", err)
	}
	waitForPodCompletion(t, cs, csiTestNamespace, "e2e-expand-writer", podReadyTimeout)
	deletePodAndWait(t, cs, csiTestNamespace, "e2e-expand-writer")

	t.Log("Expanding PVC to 2Gi")
	pvc, err := cs.CoreV1().PersistentVolumeClaims(csiTestNamespace).Get(ctx, pvcName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Failed to get PVC: %v", err)
	}
	pvc.Spec.Resources.Requests[corev1.ResourceStorage] = resource.MustParse("2Gi")
	_, err = cs.CoreV1().PersistentVolumeClaims(csiTestNamespace).Update(ctx, pvc, metav1.UpdateOptions{})
	if err != nil {
		t.Fatalf("Failed to expand PVC: %v", err)
	}

	time.Sleep(30 * time.Second)

	t.Log("Verifying data persists after expansion")
	readerPod := newCSIReaderPod(csiTestNamespace, "e2e-expand-reader", pvcName, "/mnt/data")
	_, err = cs.CoreV1().Pods(csiTestNamespace).Create(ctx, readerPod, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create reader pod: %v", err)
	}
	defer deletePodAndWait(t, cs, csiTestNamespace, "e2e-expand-reader")

	logs := waitForPodCompletion(t, cs, csiTestNamespace, "e2e-expand-reader", podReadyTimeout)
	if !strings.Contains(logs, "expand-test") {
		t.Fatalf("EXPANSION FAILED: data lost after resize — got: %q", strings.TrimSpace(logs))
	}

	t.Log("PASSED: Volume expansion with data persistence verified")
}
