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
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	lifecycleSingleNamespace = "csi-lifecycle-single"
	lifecycleMultiNamespace  = "csi-lifecycle-multi"
	lifecycleStorageClass    = "caa-csi-lifecycle"
	lifecyclePVCName         = "lifecycle-pvc"
	lifecycleWriterPod       = "lifecycle-writer"
	lifecycleReaderPod       = "lifecycle-reader"
	lifecycleTestData        = "lifecycle-e2e-persistence-check"
	lifecycleMountPath       = "/mnt/data"
	lifecyclePVCSize         = "1Gi"
	lifecyclePodTimeout      = 8 * time.Minute
	diskDetachTimeout        = 5 * time.Minute
	diskDeleteTimeout        = 5 * time.Minute
	pvDeleteTimeout          = 3 * time.Minute
	pvcBoundTimeout          = 2 * time.Minute
	singleTestTimeout        = 20 * time.Minute
	multiTestTimeout         = 25 * time.Minute

	busyboxImage = "busybox:1.36"

	// PV volume attribute keys set by the CSI driver
	volAttrEBSVolumeID  = "ebs-volume-id"
	volAttrCloudVolPath = "cloud-volume-path"
	volAttrAzureDiskID  = "azure-disk-id"
)

// pvcMountsAndVolumes builds volume mounts and volumes for multiple PVCs,
// each mounted at /mnt/<pvcName>.
func pvcMountsAndVolumes(pvcNames []string) ([]corev1.VolumeMount, []corev1.Volume) {
	var mounts []corev1.VolumeMount
	var vols []corev1.Volume
	for _, name := range pvcNames {
		mp := fmt.Sprintf("/mnt/%s", name)
		mounts = append(mounts, corev1.VolumeMount{Name: name, MountPath: mp})
		vols = append(vols, corev1.Volume{Name: name, VolumeSource: corev1.VolumeSource{
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: name},
		}})
	}
	return mounts, vols
}

func newMultiVolumeWriterPod(namespace, podName string, pvcNames, testData []string) *corev1.Pod {
	runtimeClassName := "kata-remote"
	mounts, vols := pvcMountsAndVolumes(pvcNames)
	cmd := ""
	for i, name := range pvcNames {
		mp := fmt.Sprintf("/mnt/%s", name)
		cmd += fmt.Sprintf("echo %q > %s/data.txt && ", testData[i], mp)
	}
	cmd += "sync"
	for _, name := range pvcNames {
		cmd += fmt.Sprintf(" && cat /mnt/%s/data.txt", name)
	}
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: podName, Namespace: namespace},
		Spec: corev1.PodSpec{
			RuntimeClassName: &runtimeClassName,
			Containers:       []corev1.Container{{Name: "writer", Image: busyboxImage, Command: []string{"/bin/sh", "-c", cmd}, VolumeMounts: mounts}},
			Volumes:          vols,
			RestartPolicy:    corev1.RestartPolicyNever,
		},
	}
}

func newMultiVolumeReaderPod(namespace, podName string, pvcNames []string) *corev1.Pod {
	runtimeClassName := "kata-remote"
	mounts, vols := pvcMountsAndVolumes(pvcNames)
	cmd := ""
	for _, name := range pvcNames {
		cmd += fmt.Sprintf("cat /mnt/%s/data.txt && ", name)
	}
	cmd += "true"
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: podName, Namespace: namespace},
		Spec: corev1.PodSpec{
			RuntimeClassName: &runtimeClassName,
			Containers:       []corev1.Container{{Name: "reader", Image: busyboxImage, Command: []string{"/bin/sh", "-c", cmd}, VolumeMounts: mounts}},
			Volumes:          vols,
			RestartPolicy:    corev1.RestartPolicyNever,
		},
	}
}

func skipIfLifecycleDisabled(t *testing.T) {
	t.Helper()
	if os.Getenv("CSI_LIFECYCLE_E2E") != "true" {
		t.Skip("CSI lifecycle E2E tests disabled — set CSI_LIFECYCLE_E2E=true to run")
	}
}

func getCloudProvider(t *testing.T) string {
	t.Helper()
	p := os.Getenv("CLOUD_PROVIDER")
	if p == "" {
		t.Fatal("CLOUD_PROVIDER must be set (aws or azure)")
	}
	return strings.ToLower(p)
}

func newLifecycleStorageClass(provider string) *storagev1.StorageClass {
	reclaimDelete := corev1.PersistentVolumeReclaimDelete
	bindImmediate := storagev1.VolumeBindingImmediate

	params := map[string]string{"cloudProvider": provider}

	switch provider {
	case "aws":
		if region := os.Getenv("AWS_REGION"); region != "" {
			params["awsRegion"] = region
		}
		if az := os.Getenv("AWS_AVAILABILITY_ZONE"); az != "" {
			params["awsAvailabilityZone"] = az
		}
		if vt := os.Getenv("AWS_VOLUME_TYPE"); vt != "" {
			params["awsVolumeType"] = vt
		} else {
			params["awsVolumeType"] = "gp3"
		}
	case "azure":
		if sub := os.Getenv("AZURE_SUBSCRIPTION_ID"); sub != "" {
			params["azureSubscriptionId"] = sub
		}
		if rg := os.Getenv("AZURE_RESOURCE_GROUP"); rg != "" {
			params["azureResourceGroup"] = rg
		}
		if tid := os.Getenv("AZURE_TENANT_ID"); tid != "" {
			params["azureTenantId"] = tid
		}
		if cid := os.Getenv("AZURE_CLIENT_ID"); cid != "" {
			params["azureClientId"] = cid
		}
		if loc := os.Getenv("AZURE_LOCATION"); loc != "" {
			params["azureLocation"] = loc
		}
	}

	return &storagev1.StorageClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: lifecycleStorageClass,
		},
		Provisioner:       "caa-csi-block.csi.confidentialcontainers.io",
		ReclaimPolicy:     &reclaimDelete,
		VolumeBindingMode: &bindImmediate,
		Parameters:        params,
	}
}

func newLifecyclePVC(namespace, name, storageClass, size string) *corev1.PersistentVolumeClaim {
	sc := storageClass
	return &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			StorageClassName: &sc,
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse(size),
				},
			},
		},
	}
}

func waitForPVCBound(t *testing.T, ctx context.Context, cs *kubernetes.Clientset, namespace, name string, timeout time.Duration) *corev1.PersistentVolumeClaim {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if err := ctx.Err(); err != nil {
			t.Fatalf("context cancelled while waiting for PVC %s/%s to bind: %v", namespace, name, err)
		}
		pvc, err := cs.CoreV1().PersistentVolumeClaims(namespace).Get(ctx, name, metav1.GetOptions{})
		if err == nil && pvc.Status.Phase == corev1.ClaimBound {
			return pvc
		}
		select {
		case <-time.After(pollInterval):
		case <-ctx.Done():
			t.Fatalf("context cancelled while waiting for PVC %s/%s to bind: %v", namespace, name, ctx.Err())
		}
	}
	t.Fatalf("PVC %s/%s did not become Bound within %v", namespace, name, timeout)
	return nil
}

// getPVDiskID extracts the cloud disk ID from the PV backing a bound PVC.
func getPVDiskID(t *testing.T, ctx context.Context, cs *kubernetes.Clientset, pvc *corev1.PersistentVolumeClaim) string {
	t.Helper()
	pvName := pvc.Spec.VolumeName
	if pvName == "" {
		t.Fatal("PVC has no bound PV name")
	}
	pv, err := cs.CoreV1().PersistentVolumes().Get(ctx, pvName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Failed to get PV %s: %v", pvName, err)
	}
	if pv.Spec.CSI == nil {
		t.Fatalf("PV %s has no CSI spec", pvName)
	}
	attrs := pv.Spec.CSI.VolumeAttributes
	if id, ok := attrs[volAttrEBSVolumeID]; ok && id != "" {
		return id
	}
	if id, ok := attrs[volAttrCloudVolPath]; ok && id != "" {
		return id
	}
	if id, ok := attrs[volAttrAzureDiskID]; ok && id != "" {
		return id
	}
	t.Logf("Warning: no known volume attribute found on PV %s, falling back to VolumeHandle %q", pvName, pv.Spec.CSI.VolumeHandle)
	return pv.Spec.CSI.VolumeHandle
}

func waitForPVDeleted(t *testing.T, ctx context.Context, cs *kubernetes.Clientset, pvName string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if err := ctx.Err(); err != nil {
			t.Fatalf("context cancelled while waiting for PV %s deletion: %v", pvName, err)
		}
		_, err := cs.CoreV1().PersistentVolumes().Get(ctx, pvName, metav1.GetOptions{})
		if errors.IsNotFound(err) {
			return
		}
		select {
		case <-time.After(pollInterval):
		case <-ctx.Done():
			t.Fatalf("context cancelled while waiting for PV %s deletion: %v", pvName, ctx.Err())
		}
	}
	t.Fatalf("PV %s was not deleted within %v", pvName, timeout)
}

func waitForPodGone(t *testing.T, ctx context.Context, cs *kubernetes.Clientset, ns, name string) {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			t.Fatalf("context cancelled while waiting for pod %s/%s to be gone: %v", ns, name, ctx.Err())
		default:
		}
		_, err := cs.CoreV1().Pods(ns).Get(ctx, name, metav1.GetOptions{})
		if errors.IsNotFound(err) {
			return
		}
		time.Sleep(pollInterval)
	}
	t.Fatalf("pod %s/%s was not gone within 30s — stale pod from a previous run may need manual cleanup", ns, name)
}

func waitForPVCGone(t *testing.T, ctx context.Context, cs *kubernetes.Clientset, ns, name string) {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			t.Fatalf("context cancelled while waiting for PVC %s/%s to be gone: %v", ns, name, ctx.Err())
		default:
		}
		_, err := cs.CoreV1().PersistentVolumeClaims(ns).Get(ctx, name, metav1.GetOptions{})
		if errors.IsNotFound(err) {
			return
		}
		time.Sleep(pollInterval)
	}
	t.Logf("Warning: PVC %s/%s was not gone within 30s", ns, name)
}

func waitForStorageClassGone(t *testing.T, ctx context.Context, cs *kubernetes.Clientset, name string) {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			t.Fatalf("context cancelled while waiting for StorageClass %s to be gone: %v", name, ctx.Err())
		default:
		}
		_, err := cs.StorageV1().StorageClasses().Get(ctx, name, metav1.GetOptions{})
		if errors.IsNotFound(err) {
			return
		}
		time.Sleep(pollInterval)
	}
	t.Logf("Warning: StorageClass %s was not gone within 30s", name)
}

// ensureLifecycleNamespace creates a namespace and registers cleanup via t.Cleanup.
func ensureLifecycleNamespace(t *testing.T, cs *kubernetes.Clientset, ns string) {
	t.Helper()
	ctx := context.Background()
	_, err := cs.CoreV1().Namespaces().Get(ctx, ns, metav1.GetOptions{})
	if err != nil {
		if !errors.IsNotFound(err) {
			t.Fatalf("Failed to check namespace %s: %v", ns, err)
		}
		_, err = cs.CoreV1().Namespaces().Create(ctx, &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{Name: ns},
		}, metav1.CreateOptions{})
		if err != nil && !errors.IsAlreadyExists(err) {
			t.Fatalf("Failed to create namespace %s: %v", ns, err)
		}
	}
	t.Cleanup(func() {
		cs.CoreV1().Namespaces().Delete(context.Background(), ns, metav1.DeleteOptions{}) //nolint:errcheck
	})
}

// ensureLifecycleStorageClass creates a StorageClass and registers cleanup via t.Cleanup.
func ensureLifecycleStorageClass(t *testing.T, ctx context.Context, cs *kubernetes.Clientset, provider string) {
	t.Helper()
	scClient := cs.StorageV1().StorageClasses()
	scClient.Delete(ctx, lifecycleStorageClass, metav1.DeleteOptions{}) //nolint:errcheck
	waitForStorageClassGone(t, ctx, cs, lifecycleStorageClass)
	sc := newLifecycleStorageClass(provider)
	_, err := scClient.Create(ctx, sc, metav1.CreateOptions{})
	if err != nil && !errors.IsAlreadyExists(err) {
		t.Fatalf("Failed to create StorageClass: %v", err)
	}
	t.Cleanup(func() {
		scClient.Delete(context.Background(), lifecycleStorageClass, metav1.DeleteOptions{}) //nolint:errcheck
	})
}

// TestCSIFullLifecycle exercises the complete volume passthrough lifecycle:
//
//	Phase 1: Setup — create StorageClass (Delete policy) + PVC, verify cloud disk exists
//	Phase 2: Write — create writer pod, write data, verify success
//	Phase 3: Detach — delete writer pod, verify cloud disk detaches
//	Phase 4: Reattach & Verify — create reader pod with same PVC, verify data persistence
//	Phase 5: Cleanup — delete reader pod, delete PVC, verify PV cleanup
//	Phase 6: Cloud Check — verify cloud disk no longer exists
func TestCSIFullLifecycle(t *testing.T) {
	skipIfLifecycleDisabled(t)

	provider := getCloudProvider(t)
	cs := getClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), singleTestTimeout)
	defer cancel()

	verifier, err := newCloudDiskVerifier(provider)
	if err != nil {
		t.Fatalf("Failed to create cloud disk verifier: %v", err)
	}

	// ========== Phase 1: Setup ==========
	t.Log("Phase 1: Setup — creating namespace, StorageClass, and PVC")

	ensureLifecycleNamespace(t, cs, lifecycleSingleNamespace)
	ensureLifecycleStorageClass(t, ctx, cs, provider)

	cs.CoreV1().PersistentVolumeClaims(lifecycleSingleNamespace).Delete(ctx, lifecyclePVCName, metav1.DeleteOptions{}) //nolint:errcheck
	waitForPVCGone(t, ctx, cs, lifecycleSingleNamespace, lifecyclePVCName)
	pvc := newLifecyclePVC(lifecycleSingleNamespace, lifecyclePVCName, lifecycleStorageClass, lifecyclePVCSize)
	_, err = cs.CoreV1().PersistentVolumeClaims(lifecycleSingleNamespace).Create(ctx, pvc, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create PVC: %v", err)
	}
	t.Cleanup(func() {
		cs.CoreV1().PersistentVolumeClaims(lifecycleSingleNamespace).Delete(context.Background(), lifecyclePVCName, metav1.DeleteOptions{}) //nolint:errcheck
	})

	boundPVC := waitForPVCBound(t, ctx, cs, lifecycleSingleNamespace, lifecyclePVCName, pvcBoundTimeout)
	pvName := boundPVC.Spec.VolumeName
	t.Logf("PVC bound to PV: %s", pvName)

	diskID := getPVDiskID(t, ctx, cs, boundPVC)
	t.Logf("Cloud disk ID: %s", diskID)

	// Safety net: detect orphaned cloud disks if normal cleanup fails
	t.Cleanup(func() {
		if exists, _ := verifier.DiskExists(context.Background(), diskID); exists {
			t.Logf("ORPHAN DETECTED: cloud disk %s still exists after test cleanup — manual deletion required", diskID)
		}
	})

	exists, err := verifier.DiskExists(ctx, diskID)
	if err != nil {
		t.Fatalf("Failed to check disk existence: %v", err)
	}
	if !exists {
		t.Fatalf("Phase 1 FAILED: cloud disk %s does not exist after PVC bind", diskID)
	}
	t.Log("Phase 1 PASSED: PVC bound, cloud disk exists")

	// ========== Phase 2: Write ==========
	t.Log("Phase 2: Write — creating writer pod")

	writerPod := newCSIWriterPod(lifecycleSingleNamespace, lifecycleWriterPod, lifecyclePVCName, lifecycleMountPath, lifecycleTestData)
	cs.CoreV1().Pods(lifecycleSingleNamespace).Delete(ctx, lifecycleWriterPod, metav1.DeleteOptions{}) //nolint:errcheck
	waitForPodGone(t, ctx, cs, lifecycleSingleNamespace, lifecycleWriterPod)
	_, err = cs.CoreV1().Pods(lifecycleSingleNamespace).Create(ctx, writerPod, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create writer pod: %v", err)
	}
	t.Cleanup(func() {
		deletePodAndWait(t, cs, lifecycleSingleNamespace, lifecycleWriterPod)
	})

	logs := waitForPodCompletion(t, cs, lifecycleSingleNamespace, lifecycleWriterPod, lifecyclePodTimeout)
	if !strings.Contains(logs, lifecycleTestData) {
		t.Fatalf("Phase 2 FAILED: writer output %q does not contain test data %q", strings.TrimSpace(logs), lifecycleTestData)
	}
	t.Log("Phase 2 PASSED: data written successfully")

	// ========== Phase 3: Detach ==========
	t.Log("Phase 3: Detach — deleting writer pod, verifying disk detaches")

	deletePodAndWait(t, cs, lifecycleSingleNamespace, lifecycleWriterPod)
	t.Log("Writer pod deleted, waiting for disk to detach...")

	state, err := waitForDiskDetached(ctx, verifier, diskID, diskDetachTimeout)
	if err != nil {
		t.Fatalf("Phase 3 FAILED: %v", err)
	}
	t.Logf("Phase 3 PASSED: disk detached, state=%s", state)

	// ========== Phase 4: Reattach & Verify ==========
	t.Log("Phase 4: Reattach & Verify — creating reader pod, verifying data persistence")

	cs.CoreV1().Pods(lifecycleSingleNamespace).Delete(ctx, lifecycleReaderPod, metav1.DeleteOptions{}) //nolint:errcheck
	waitForPodGone(t, ctx, cs, lifecycleSingleNamespace, lifecycleReaderPod)
	readerPod := newCSIReaderPod(lifecycleSingleNamespace, lifecycleReaderPod, lifecyclePVCName, lifecycleMountPath)
	_, err = cs.CoreV1().Pods(lifecycleSingleNamespace).Create(ctx, readerPod, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create reader pod: %v", err)
	}
	t.Cleanup(func() {
		deletePodAndWait(t, cs, lifecycleSingleNamespace, lifecycleReaderPod)
	})

	logs = waitForPodCompletion(t, cs, lifecycleSingleNamespace, lifecycleReaderPod, lifecyclePodTimeout)
	if !strings.Contains(logs, lifecycleTestData) {
		t.Fatalf("Phase 4 FAILED: persistence broken — reader got %q, expected to contain %q", strings.TrimSpace(logs), lifecycleTestData)
	}
	t.Log("Phase 4 PASSED: data persisted across pod restart")

	// ========== Phase 5: Cleanup ==========
	t.Log("Phase 5: Cleanup — deleting reader pod and PVC")

	deletePodAndWait(t, cs, lifecycleSingleNamespace, lifecycleReaderPod)
	t.Log("Reader pod deleted")

	_, err = waitForDiskDetached(ctx, verifier, diskID, diskDetachTimeout)
	if err != nil {
		t.Logf("Warning: disk did not reach detached state before PVC delete: %v", err)
	}

	err = cs.CoreV1().PersistentVolumeClaims(lifecycleSingleNamespace).Delete(ctx, lifecyclePVCName, metav1.DeleteOptions{})
	if err != nil {
		t.Fatalf("Failed to delete PVC: %v", err)
	}
	t.Log("PVC deleted, waiting for PV cleanup...")

	waitForPVDeleted(t, ctx, cs, pvName, pvDeleteTimeout)
	t.Log("Phase 5 PASSED: PV deleted")

	// ========== Phase 6: Cloud Check ==========
	t.Log("Phase 6: Cloud Check — verifying disk no longer exists in cloud")

	err = waitForDiskDeleted(ctx, verifier, diskID, diskDeleteTimeout)
	if err != nil {
		t.Fatalf("Phase 6 FAILED: %v", err)
	}
	t.Log("Phase 6 PASSED: cloud disk deleted — no orphan resources")

	t.Log("ALL PHASES PASSED: Full lifecycle verified (create → write → detach → reattach+verify → delete → cloud cleanup)")
}

// TestCSIFullLifecycleMultiDisk exercises the same lifecycle with two PVCs on one pod.
func TestCSIFullLifecycleMultiDisk(t *testing.T) {
	skipIfLifecycleDisabled(t)

	provider := getCloudProvider(t)
	cs := getClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), multiTestTimeout)
	defer cancel()

	verifier, err := newCloudDiskVerifier(provider)
	if err != nil {
		t.Fatalf("Failed to create cloud disk verifier: %v", err)
	}

	// Setup
	t.Log("Setup: creating namespace, StorageClass, and 2 PVCs")

	ensureLifecycleNamespace(t, cs, lifecycleMultiNamespace)
	ensureLifecycleStorageClass(t, ctx, cs, provider)

	pvcNames := []string{"lifecycle-multi-pvc-0", "lifecycle-multi-pvc-1"}
	testDataValues := []string{"multi-disk-0-data-lifecycle", "multi-disk-1-data-lifecycle"}
	diskIDs := make([]string, 2)
	pvNames := make([]string, 2)

	for i, name := range pvcNames {
		cs.CoreV1().PersistentVolumeClaims(lifecycleMultiNamespace).Delete(ctx, name, metav1.DeleteOptions{}) //nolint:errcheck
		waitForPVCGone(t, ctx, cs, lifecycleMultiNamespace, name)
		pvc := newLifecyclePVC(lifecycleMultiNamespace, name, lifecycleStorageClass, lifecyclePVCSize)
		_, err = cs.CoreV1().PersistentVolumeClaims(lifecycleMultiNamespace).Create(ctx, pvc, metav1.CreateOptions{})
		if err != nil {
			t.Fatalf("Failed to create PVC %s: %v", name, err)
		}
		pvcName := name
		t.Cleanup(func() {
			cs.CoreV1().PersistentVolumeClaims(lifecycleMultiNamespace).Delete(context.Background(), pvcName, metav1.DeleteOptions{}) //nolint:errcheck
		})
		boundPVC := waitForPVCBound(t, ctx, cs, lifecycleMultiNamespace, name, pvcBoundTimeout)
		pvNames[i] = boundPVC.Spec.VolumeName
		diskIDs[i] = getPVDiskID(t, ctx, cs, boundPVC)
		t.Logf("PVC %s → PV %s → Disk %s", name, pvNames[i], diskIDs[i])
	}

	// Safety net: detect orphaned cloud disks if normal cleanup fails
	t.Cleanup(func() {
		for i, diskID := range diskIDs {
			if diskID == "" {
				continue
			}
			if exists, _ := verifier.DiskExists(context.Background(), diskID); exists {
				t.Logf("ORPHAN DETECTED: cloud disk %d (%s) still exists after test cleanup — manual deletion required", i, diskID)
			}
		}
	})

	for i, diskID := range diskIDs {
		exists, err := verifier.DiskExists(ctx, diskID)
		if err != nil {
			t.Fatalf("Disk %d (%s) existence check failed: %v", i, diskID, err)
		}
		if !exists {
			t.Fatalf("Disk %d (%s) does not exist after PVC bind", i, diskID)
		}
	}
	t.Log("Setup PASSED: both disks provisioned")

	// Write phase
	t.Log("Write: creating multi-volume writer pod")

	writerPod := newMultiVolumeWriterPod(lifecycleMultiNamespace, "lifecycle-multi-writer", pvcNames, testDataValues)
	cs.CoreV1().Pods(lifecycleMultiNamespace).Delete(ctx, "lifecycle-multi-writer", metav1.DeleteOptions{}) //nolint:errcheck
	waitForPodGone(t, ctx, cs, lifecycleMultiNamespace, "lifecycle-multi-writer")
	_, err = cs.CoreV1().Pods(lifecycleMultiNamespace).Create(ctx, writerPod, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create writer pod: %v", err)
	}
	t.Cleanup(func() {
		deletePodAndWait(t, cs, lifecycleMultiNamespace, "lifecycle-multi-writer")
	})

	logs := waitForPodCompletion(t, cs, lifecycleMultiNamespace, "lifecycle-multi-writer", lifecyclePodTimeout)
	for _, d := range testDataValues {
		if !strings.Contains(logs, d) {
			t.Fatalf("Write FAILED: output missing %q", d)
		}
	}
	t.Log("Write PASSED")

	// Detach — assert disks actually detach
	t.Log("Detach: deleting writer pod, verifying disks detach")
	deletePodAndWait(t, cs, lifecycleMultiNamespace, "lifecycle-multi-writer")

	for i, diskID := range diskIDs {
		state, err := waitForDiskDetached(ctx, verifier, diskID, diskDetachTimeout)
		if err != nil {
			t.Fatalf("Detach FAILED for disk %d (%s): %v", i, diskID, err)
		}
		t.Logf("Disk %d detached, state=%s", i, state)
	}
	t.Log("Detach PASSED")

	// Reattach + Verify
	t.Log("Reattach: creating multi-volume reader pod")

	readerPod := newMultiVolumeReaderPod(lifecycleMultiNamespace, "lifecycle-multi-reader", pvcNames)
	cs.CoreV1().Pods(lifecycleMultiNamespace).Delete(ctx, "lifecycle-multi-reader", metav1.DeleteOptions{}) //nolint:errcheck
	waitForPodGone(t, ctx, cs, lifecycleMultiNamespace, "lifecycle-multi-reader")
	_, err = cs.CoreV1().Pods(lifecycleMultiNamespace).Create(ctx, readerPod, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create reader pod: %v", err)
	}
	t.Cleanup(func() {
		deletePodAndWait(t, cs, lifecycleMultiNamespace, "lifecycle-multi-reader")
	})

	logs = waitForPodCompletion(t, cs, lifecycleMultiNamespace, "lifecycle-multi-reader", lifecyclePodTimeout)
	for _, d := range testDataValues {
		if !strings.Contains(logs, d) {
			t.Fatalf("Verify FAILED: reader output missing %q", d)
		}
	}
	t.Log("Verify PASSED: both disks persisted data")

	// Cleanup
	t.Log("Cleanup: deleting reader pod and PVCs")
	deletePodAndWait(t, cs, lifecycleMultiNamespace, "lifecycle-multi-reader")

	for i, diskID := range diskIDs {
		_, err := waitForDiskDetached(ctx, verifier, diskID, diskDetachTimeout)
		if err != nil {
			t.Logf("Warning: disk %d did not reach detached state before PVC delete: %v", i, err)
		}
	}

	for _, name := range pvcNames {
		if err := cs.CoreV1().PersistentVolumeClaims(lifecycleMultiNamespace).Delete(ctx, name, metav1.DeleteOptions{}); err != nil {
			if !errors.IsNotFound(err) {
				t.Logf("Warning: failed to delete PVC %s: %v", name, err)
			}
		}
	}

	for _, pv := range pvNames {
		waitForPVDeleted(t, ctx, cs, pv, pvDeleteTimeout)
	}
	t.Log("Cleanup PASSED: PVs deleted")

	// Cloud check
	t.Log("Cloud Check: verifying disks deleted")
	for i, diskID := range diskIDs {
		err = waitForDiskDeleted(ctx, verifier, diskID, diskDeleteTimeout)
		if err != nil {
			t.Fatalf("Cloud Check FAILED for disk %d: %v", i, err)
		}
	}
	t.Log("Cloud Check PASSED: all disks deleted — no orphans")

	t.Log("ALL PHASES PASSED: Multi-disk full lifecycle verified")
}
