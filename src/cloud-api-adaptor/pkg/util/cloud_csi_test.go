package util

import (
	b64 "encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	provider "github.com/confidential-containers/cloud-api-adaptor/src/cloud-providers"
	cri "github.com/containerd/containerd/pkg/cri/annotations"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupDirectVolumesDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	origDir := KataDirectVolumesDir

	t.Cleanup(func() {
		KataDirectVolumesDir = origDir
	})
	KataDirectVolumesDir = dir
	return dir
}

func writeMountInfo(t *testing.T, dir, volumePath string, info map[string]interface{}) {
	t.Helper()
	encoded := b64.URLEncoding.EncodeToString([]byte(volumePath))
	volDir := filepath.Join(dir, encoded)
	require.NoError(t, os.MkdirAll(volDir, 0o755))

	data, err := json.Marshal(info)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(volDir, "mountInfo.json"), data, 0o644))
}

func TestGetCSIVolumesForPod_EmptyDirectory(t *testing.T) {
	setupDirectVolumesDir(t)
	volumes := GetCSIVolumesForPod(map[string]string{})
	assert.Nil(t, volumes)
}

func TestGetCSIVolumesForPod_NonExistentDirectory(t *testing.T) {
	origDir := KataDirectVolumesDir
	KataDirectVolumesDir = "/nonexistent/path"
	defer func() { KataDirectVolumesDir = origDir }()

	volumes := GetCSIVolumesForPod(map[string]string{})
	assert.Nil(t, volumes)
}

func TestGetCSIVolumesForPod_SingleVolume(t *testing.T) {
	dir := setupDirectVolumesDir(t)
	volPath := "/var/lib/kubelet/pods/pod-uid-123/volumes/kubernetes.io~csi/pvc-abc/mount"

	writeMountInfo(t, dir, volPath, map[string]interface{}{
		"device": "/subscriptions/sub1/resourceGroups/rg1/providers/Microsoft.Compute/disks/csi-vol-pvc-abc",
		"fstype": "ext4",
	})

	volumes := GetCSIVolumesForPod(map[string]string{})
	require.Len(t, volumes, 1)
	assert.Equal(t, "/subscriptions/sub1/resourceGroups/rg1/providers/Microsoft.Compute/disks/csi-vol-pvc-abc", volumes[0].DiskID)
}

func TestGetCSIVolumesForPod_CloudVolumePathTakesPrecedence(t *testing.T) {
	dir := setupDirectVolumesDir(t)
	volPath := "/var/lib/kubelet/pods/pod-uid-123/volumes/kubernetes.io~csi/pvc-abc/mount"

	writeMountInfo(t, dir, volPath, map[string]interface{}{
		"device": "fallback-device",
		"fstype": "ext4",
		"metadata": map[string]interface{}{
			"cloud-volume-path": "/subscriptions/sub1/disks/preferred-disk",
		},
	})

	volumes := GetCSIVolumesForPod(map[string]string{})
	require.Len(t, volumes, 1)
	assert.Equal(t, "/subscriptions/sub1/disks/preferred-disk", volumes[0].DiskID)
}

func TestGetCSIVolumesForPod_PodUIDFiltering(t *testing.T) {
	dir := setupDirectVolumesDir(t)

	writeMountInfo(t, dir,
		"/var/lib/kubelet/pods/pod-uid-AAA/volumes/kubernetes.io~csi/pvc-1/mount",
		map[string]interface{}{"device": "disk-A", "fstype": "ext4"})

	writeMountInfo(t, dir,
		"/var/lib/kubelet/pods/pod-uid-BBB/volumes/kubernetes.io~csi/pvc-2/mount",
		map[string]interface{}{"device": "disk-B", "fstype": "ext4"})

	writeMountInfo(t, dir,
		"/var/lib/kubelet/pods/pod-uid-AAA/volumes/kubernetes.io~csi/pvc-3/mount",
		map[string]interface{}{"device": "disk-C", "fstype": "ext4"})

	annotations := map[string]string{
		cri.SandboxUID: "pod-uid-AAA",
	}
	volumes := GetCSIVolumesForPod(annotations)
	require.Len(t, volumes, 2)

	diskIDs := []string{volumes[0].DiskID, volumes[1].DiskID}
	assert.Contains(t, diskIDs, "disk-A")
	assert.Contains(t, diskIDs, "disk-C")
	assert.NotContains(t, diskIDs, "disk-B")
}

func TestGetCSIVolumesForPod_EmptyPodUIDReturnsAll(t *testing.T) {
	dir := setupDirectVolumesDir(t)

	writeMountInfo(t, dir,
		"/var/lib/kubelet/pods/pod-uid-AAA/volumes/kubernetes.io~csi/pvc-1/mount",
		map[string]interface{}{"device": "disk-A", "fstype": "ext4"})

	writeMountInfo(t, dir,
		"/var/lib/kubelet/pods/pod-uid-BBB/volumes/kubernetes.io~csi/pvc-2/mount",
		map[string]interface{}{"device": "disk-B", "fstype": "ext4"})

	volumes := GetCSIVolumesForPod(map[string]string{})
	assert.Len(t, volumes, 2)
}

func TestGetCSIVolumesForPod_SkipsNonCSIVolumes(t *testing.T) {
	dir := setupDirectVolumesDir(t)

	writeMountInfo(t, dir,
		"/var/lib/kubelet/pods/pod-uid-123/volumes/kubernetes.io~csi/pvc-1/mount",
		map[string]interface{}{"device": "csi-disk", "fstype": "ext4"})

	writeMountInfo(t, dir,
		"/var/lib/kubelet/pods/pod-uid-123/volumes/kubernetes.io~configmap/config/mount",
		map[string]interface{}{"device": "configmap-device", "fstype": "ext4"})

	volumes := GetCSIVolumesForPod(map[string]string{})
	require.Len(t, volumes, 1)
	assert.Equal(t, "csi-disk", volumes[0].DiskID)
}

func TestGetCSIVolumesForPod_SkipsMissingDiskID(t *testing.T) {
	dir := setupDirectVolumesDir(t)

	writeMountInfo(t, dir,
		"/var/lib/kubelet/pods/pod-uid-123/volumes/kubernetes.io~csi/pvc-no-disk/mount",
		map[string]interface{}{"fstype": "ext4"})

	volumes := GetCSIVolumesForPod(map[string]string{})
	assert.Empty(t, volumes)
}

func TestGetCSIVolumesForPod_SkipsInvalidJSON(t *testing.T) {
	dir := setupDirectVolumesDir(t)
	volPath := "/var/lib/kubelet/pods/pod-uid-123/volumes/kubernetes.io~csi/pvc-bad/mount"
	encoded := b64.URLEncoding.EncodeToString([]byte(volPath))
	volDir := filepath.Join(dir, encoded)
	require.NoError(t, os.MkdirAll(volDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(volDir, "mountInfo.json"), []byte("{invalid json"), 0o644))

	volumes := GetCSIVolumesForPod(map[string]string{})
	assert.Empty(t, volumes)
}

func TestGetCSIVolumesForPod_SkipsNonBase64Directories(t *testing.T) {
	dir := setupDirectVolumesDir(t)

	require.NoError(t, os.MkdirAll(filepath.Join(dir, "not-base64-encoded"), 0o755))

	writeMountInfo(t, dir,
		"/var/lib/kubelet/pods/pod-uid-123/volumes/kubernetes.io~csi/pvc-ok/mount",
		map[string]interface{}{"device": "good-disk", "fstype": "ext4"})

	volumes := GetCSIVolumesForPod(map[string]string{})
	require.Len(t, volumes, 1)
	assert.Equal(t, "good-disk", volumes[0].DiskID)
}

func TestGetCSIVolumesForPod_SkipsRegularFiles(t *testing.T) {
	dir := setupDirectVolumesDir(t)

	volPath := "/var/lib/kubelet/pods/pod-uid-123/volumes/kubernetes.io~csi/pvc-file/mount"
	encoded := b64.URLEncoding.EncodeToString([]byte(volPath))
	require.NoError(t, os.WriteFile(filepath.Join(dir, encoded), []byte("not a dir"), 0o644))

	volumes := GetCSIVolumesForPod(map[string]string{})
	assert.Empty(t, volumes)
}

func TestGetCSIVolumesForPod_MultipleVolumesCanonicalOrder(t *testing.T) {
	dir := setupDirectVolumesDir(t)

	paths := []string{
		"/var/lib/kubelet/pods/pod-uid-123/volumes/kubernetes.io~csi/pvc-charlie/mount",
		"/var/lib/kubelet/pods/pod-uid-123/volumes/kubernetes.io~csi/pvc-alpha/mount",
		"/var/lib/kubelet/pods/pod-uid-123/volumes/kubernetes.io~csi/pvc-bravo/mount",
	}

	for i, p := range paths {
		writeMountInfo(t, dir, p, map[string]interface{}{
			"device": fmt.Sprintf("disk-%d", i),
			"fstype": "ext4",
		})
	}

	volumes := GetCSIVolumesForPod(map[string]string{})
	require.Len(t, volumes, 3)

	for i := 0; i < len(volumes)-1; i++ {
		assert.NotEqual(t, volumes[i].DiskID, volumes[i+1].DiskID,
			"volumes should be distinct")
	}
}

func TestGetCSIVolumesForPod_ReturnType(t *testing.T) {
	dir := setupDirectVolumesDir(t)

	writeMountInfo(t, dir,
		"/var/lib/kubelet/pods/pod-uid-123/volumes/kubernetes.io~csi/pvc-test/mount",
		map[string]interface{}{"device": "test-disk", "fstype": "ext4"})

	volumes := GetCSIVolumesForPod(map[string]string{})
	require.Len(t, volumes, 1)
	assert.IsType(t, provider.CloudVolume{}, volumes[0])
}
