// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package proxy

import (
	"context"
	b64 "encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/pkg/util"
	pb "github.com/kata-containers/kata-containers/src/runtime/virtcontainers/pkg/agent/protocols/grpc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeTestMountInfo(t *testing.T, dir, volumePath string, info map[string]interface{}) {
	t.Helper()
	encoded := b64.URLEncoding.EncodeToString([]byte(volumePath))
	volDir := filepath.Join(dir, encoded)
	require.NoError(t, os.MkdirAll(volDir, 0o755))

	data, err := json.Marshal(info)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(volDir, "mountInfo.json"), data, 0o644))
}

func overrideKataDirectVolumesDir(t *testing.T, dir string) {
	t.Helper()
	origDir := util.KataDirectVolumesDir
	util.KataDirectVolumesDir = dir
	t.Cleanup(func() { util.KataDirectVolumesDir = origDir })
}

func TestCloudVolumes_SingleVolumeAnnotation(t *testing.T) {
	dir := t.TempDir()
	overrideKataDirectVolumesDir(t, dir)

	service, cleanup := setupMockAgentAndService(t)
	defer cleanup()

	podUID := "pod-uid-111"
	volPath := "/var/lib/kubelet/pods/" + podUID + "/volumes/kubernetes.io~csi/pvc-test/mount"

	writeTestMountInfo(t, dir, volPath, map[string]interface{}{
		"device": "/subscriptions/sub/disks/csi-vol-pvc-test",
		"fstype": "ext4",
		"metadata": map[string]interface{}{
			"cloud-volume-path": "/subscriptions/sub/disks/csi-vol-pvc-test",
		},
	})

	req := newCreateContainerRequest("test-cloud-vol").
		withAnnotations(map[string]string{
			"io.kubernetes.cri.sandbox-uid": podUID,
		}).
		withMounts(&pb.Mount{
			Destination: "/mnt/data",
			Source:      volPath,
			Type:        "bind",
		}).
		build()

	_, err := service.CreateContainer(context.Background(), req)
	require.NoError(t, err)

	cvJSON, ok := req.OCI.Annotations["io.confidentialcontainers.org.cloud_volumes"]
	require.True(t, ok, "cloud_volumes annotation should be set")

	var cloudVolumes map[string]map[string]string
	require.NoError(t, json.Unmarshal([]byte(cvJSON), &cloudVolumes))

	require.Contains(t, cloudVolumes, "vol-0")
	assert.Equal(t, "/mnt/data", cloudVolumes["vol-0"]["mount_point"])
	assert.Equal(t, "ext4", cloudVolumes["vol-0"]["fs_type"])
	assert.Equal(t, "0", cloudVolumes["vol-0"]["lun"])
	assert.Equal(t, "/subscriptions/sub/disks/csi-vol-pvc-test", cloudVolumes["vol-0"]["disk_id"])
}

func TestCloudVolumes_MultipleVolumes(t *testing.T) {
	dir := t.TempDir()
	overrideKataDirectVolumesDir(t, dir)

	service, cleanup := setupMockAgentAndService(t)
	defer cleanup()

	podUID := "pod-uid-222"
	volPathA := "/var/lib/kubelet/pods/" + podUID + "/volumes/kubernetes.io~csi/pvc-alpha/mount"
	volPathB := "/var/lib/kubelet/pods/" + podUID + "/volumes/kubernetes.io~csi/pvc-bravo/mount"

	writeTestMountInfo(t, dir, volPathA, map[string]interface{}{
		"device": "disk-alpha",
		"fstype": "ext4",
	})
	writeTestMountInfo(t, dir, volPathB, map[string]interface{}{
		"device": "disk-bravo",
		"fstype": "xfs",
	})

	req := newCreateContainerRequest("test-multi-vol").
		withAnnotations(map[string]string{
			"io.kubernetes.cri.sandbox-uid": podUID,
		}).
		withMounts(
			&pb.Mount{Destination: "/data/a", Source: volPathA, Type: "bind"},
			&pb.Mount{Destination: "/data/b", Source: volPathB, Type: "bind"},
		).
		build()

	_, err := service.CreateContainer(context.Background(), req)
	require.NoError(t, err)

	cvJSON := req.OCI.Annotations["io.confidentialcontainers.org.cloud_volumes"]
	require.NotEmpty(t, cvJSON)

	var cloudVolumes map[string]map[string]string
	require.NoError(t, json.Unmarshal([]byte(cvJSON), &cloudVolumes))
	assert.Len(t, cloudVolumes, 2)
}

func TestCloudVolumes_FsTypeFromMountInfo(t *testing.T) {
	dir := t.TempDir()
	overrideKataDirectVolumesDir(t, dir)

	service, cleanup := setupMockAgentAndService(t)
	defer cleanup()

	podUID := "pod-uid-333"
	volPath := "/var/lib/kubelet/pods/" + podUID + "/volumes/kubernetes.io~csi/pvc-xfs/mount"

	writeTestMountInfo(t, dir, volPath, map[string]interface{}{
		"device": "disk-xfs",
		"fstype": "xfs",
	})

	req := newCreateContainerRequest("test-fstype").
		withAnnotations(map[string]string{
			"io.kubernetes.cri.sandbox-uid": podUID,
		}).
		withMounts(&pb.Mount{
			Destination: "/mnt/xfs",
			Source:      volPath,
			Type:        "bind",
		}).
		build()

	_, err := service.CreateContainer(context.Background(), req)
	require.NoError(t, err)

	var cloudVolumes map[string]map[string]string
	require.NoError(t, json.Unmarshal([]byte(req.OCI.Annotations["io.confidentialcontainers.org.cloud_volumes"]), &cloudVolumes))
	assert.Equal(t, "xfs", cloudVolumes["vol-0"]["fs_type"])
}

func TestCloudVolumes_FsTypeFallsBackToExt4(t *testing.T) {
	dir := t.TempDir()
	overrideKataDirectVolumesDir(t, dir)

	service, cleanup := setupMockAgentAndService(t)
	defer cleanup()

	podUID := "pod-uid-444"
	volPath := "/var/lib/kubelet/pods/" + podUID + "/volumes/kubernetes.io~csi/pvc-nofs/mount"

	writeTestMountInfo(t, dir, volPath, map[string]interface{}{
		"device": "disk-nofs",
	})

	req := newCreateContainerRequest("test-fstype-default").
		withAnnotations(map[string]string{
			"io.kubernetes.cri.sandbox-uid": podUID,
		}).
		withMounts(&pb.Mount{
			Destination: "/mnt/default",
			Source:      volPath,
			Type:        "bind",
		}).
		build()

	_, err := service.CreateContainer(context.Background(), req)
	require.NoError(t, err)

	var cloudVolumes map[string]map[string]string
	require.NoError(t, json.Unmarshal([]byte(req.OCI.Annotations["io.confidentialcontainers.org.cloud_volumes"]), &cloudVolumes))
	assert.Equal(t, "ext4", cloudVolumes["vol-0"]["fs_type"])
}

func TestCloudVolumes_NoAnnotationWhenNoCSIVolumes(t *testing.T) {
	dir := t.TempDir()
	overrideKataDirectVolumesDir(t, dir)

	service, cleanup := setupMockAgentAndService(t)
	defer cleanup()

	req := newCreateContainerRequest("test-no-vol").
		withMounts(&pb.Mount{
			Destination: "/mnt/regular",
			Source:      "/some/regular/path",
			Type:        "bind",
		}).
		build()

	_, err := service.CreateContainer(context.Background(), req)
	require.NoError(t, err)

	_, ok := req.OCI.Annotations["io.confidentialcontainers.org.cloud_volumes"]
	assert.False(t, ok, "cloud_volumes annotation should not be set for non-CSI volumes")
}

func TestCloudVolumes_SkipsVolumesFromOtherPods(t *testing.T) {
	dir := t.TempDir()
	overrideKataDirectVolumesDir(t, dir)

	service, cleanup := setupMockAgentAndService(t)
	defer cleanup()

	writeTestMountInfo(t, dir,
		"/var/lib/kubelet/pods/other-pod-uid/volumes/kubernetes.io~csi/pvc-other/mount",
		map[string]interface{}{"device": "other-disk", "fstype": "ext4"})

	req := newCreateContainerRequest("test-other-pod").
		withAnnotations(map[string]string{
			"io.kubernetes.cri.sandbox-uid": "my-pod-uid",
		}).
		build()

	_, err := service.CreateContainer(context.Background(), req)
	require.NoError(t, err)

	_, ok := req.OCI.Annotations["io.confidentialcontainers.org.cloud_volumes"]
	assert.False(t, ok, "should not include volumes from other pods")
}

func TestCloudVolumes_LUNIndexSkippedVolumeConsistency(t *testing.T) {
	dir := t.TempDir()
	overrideKataDirectVolumesDir(t, dir)

	service, cleanup := setupMockAgentAndService(t)
	defer cleanup()

	podUID := "pod-uid-555"

	// vol-a has matching OCI mount
	volPathA := "/var/lib/kubelet/pods/" + podUID + "/volumes/kubernetes.io~csi/pvc-alpha/mount"
	writeTestMountInfo(t, dir, volPathA, map[string]interface{}{
		"device": "disk-alpha", "fstype": "ext4",
	})

	// vol-b has NO matching OCI mount (will be skipped in annotation but still counted)
	volPathB := "/var/lib/kubelet/pods/" + podUID + "/volumes/kubernetes.io~csi/pvc-bravo/mount"
	writeTestMountInfo(t, dir, volPathB, map[string]interface{}{
		"device": "disk-bravo", "fstype": "ext4",
	})

	// vol-c has matching OCI mount
	volPathC := "/var/lib/kubelet/pods/" + podUID + "/volumes/kubernetes.io~csi/pvc-charlie/mount"
	writeTestMountInfo(t, dir, volPathC, map[string]interface{}{
		"device": "disk-charlie", "fstype": "ext4",
	})

	req := newCreateContainerRequest("test-lun-skip").
		withAnnotations(map[string]string{
			"io.kubernetes.cri.sandbox-uid": podUID,
		}).
		withMounts(
			&pb.Mount{Destination: "/data/a", Source: volPathA, Type: "bind"},
			// No mount for volPathB
			&pb.Mount{Destination: "/data/c", Source: volPathC, Type: "bind"},
		).
		build()

	_, err := service.CreateContainer(context.Background(), req)
	require.NoError(t, err)

	cvJSON := req.OCI.Annotations["io.confidentialcontainers.org.cloud_volumes"]
	require.NotEmpty(t, cvJSON)

	var cloudVolumes map[string]map[string]string
	require.NoError(t, json.Unmarshal([]byte(cvJSON), &cloudVolumes))

	// Should only have 2 entries (vol-b is skipped because no OCI mount)
	assert.Len(t, cloudVolumes, 2)

	// Collect LUN values
	luns := make(map[string]string)
	for _, vol := range cloudVolumes {
		luns[vol["lun"]] = vol["disk_id"]
	}

	// vol-a should get LUN 0 (canonical index 0)
	// vol-b gets canonical index 1 (skipped in annotation but index still incremented)
	// vol-c should get LUN 2 (canonical index 2)
	assert.Contains(t, luns, "0", "vol-a should be at LUN 0")
	assert.Contains(t, luns, "2", "vol-c should be at LUN 2, not LUN 1")
	assert.NotContains(t, luns, "1", "LUN 1 should be skipped (vol-b had no OCI mount)")

	assert.Equal(t, "disk-alpha", luns["0"])
	assert.Equal(t, "disk-charlie", luns["2"])
}

func TestCloudVolumes_SkipsVolumeWithNoDiskID(t *testing.T) {
	dir := t.TempDir()
	overrideKataDirectVolumesDir(t, dir)

	service, cleanup := setupMockAgentAndService(t)
	defer cleanup()

	podUID := "pod-uid-666"
	volPath := "/var/lib/kubelet/pods/" + podUID + "/volumes/kubernetes.io~csi/pvc-nodisk/mount"

	writeTestMountInfo(t, dir, volPath, map[string]interface{}{
		"fstype": "ext4",
	})

	req := newCreateContainerRequest("test-no-disk").
		withAnnotations(map[string]string{
			"io.kubernetes.cri.sandbox-uid": podUID,
		}).
		withMounts(&pb.Mount{
			Destination: "/mnt/nodisk",
			Source:      volPath,
			Type:        "bind",
		}).
		build()

	_, err := service.CreateContainer(context.Background(), req)
	require.NoError(t, err)

	_, ok := req.OCI.Annotations["io.confidentialcontainers.org.cloud_volumes"]
	assert.False(t, ok, "annotation should not be set when volume has no disk ID")
}

func TestCloudVolumes_SkipsInvalidMountInfoJSON(t *testing.T) {
	dir := t.TempDir()
	overrideKataDirectVolumesDir(t, dir)

	service, cleanup := setupMockAgentAndService(t)
	defer cleanup()

	podUID := "pod-uid-777"
	volPath := "/var/lib/kubelet/pods/" + podUID + "/volumes/kubernetes.io~csi/pvc-badjson/mount"

	encoded := b64.URLEncoding.EncodeToString([]byte(volPath))
	volDir := filepath.Join(dir, encoded)
	require.NoError(t, os.MkdirAll(volDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(volDir, "mountInfo.json"), []byte("not json"), 0o644))

	req := newCreateContainerRequest("test-bad-json").
		withAnnotations(map[string]string{
			"io.kubernetes.cri.sandbox-uid": podUID,
		}).
		withMounts(&pb.Mount{
			Destination: "/mnt/badjson",
			Source:      volPath,
			Type:        "bind",
		}).
		build()

	_, err := service.CreateContainer(context.Background(), req)
	require.NoError(t, err)

	_, ok := req.OCI.Annotations["io.confidentialcontainers.org.cloud_volumes"]
	assert.False(t, ok, "annotation should not be set with invalid JSON")
}

func TestCloudVolumes_EncryptionAnnotation(t *testing.T) {
	dir := t.TempDir()
	overrideKataDirectVolumesDir(t, dir)

	service, cleanup := setupMockAgentAndService(t)
	defer cleanup()

	podUID := "pod-uid-enc-333"
	volPath := "/var/lib/kubelet/pods/" + podUID + "/volumes/kubernetes.io~csi/pvc-encrypted/mount"

	writeTestMountInfo(t, dir, volPath, map[string]interface{}{
		"device": "/subscriptions/sub/disks/csi-vol-pvc-encrypted",
		"fstype": "ext4",
		"metadata": map[string]interface{}{
			"cloud-volume-path": "/subscriptions/sub/disks/csi-vol-pvc-encrypted",
			"encrypt-type":      "LUKS",
			"kbs-key-id":        "default/key/volume-enc-key",
		},
	})

	req := newCreateContainerRequest("test-encrypted-vol").
		withAnnotations(map[string]string{
			"io.kubernetes.cri.sandbox-uid": podUID,
		}).
		withMounts(&pb.Mount{
			Destination: "/mnt/secret",
			Source:      volPath,
			Type:        "bind",
		}).
		build()

	_, err := service.CreateContainer(context.Background(), req)
	require.NoError(t, err)

	cvJSON, ok := req.OCI.Annotations[util.CloudVolumesAnnotationKey]
	require.True(t, ok, "cloud_volumes annotation should be set")

	var cloudVolumes map[string]util.CloudVolumeAnnotation
	require.NoError(t, json.Unmarshal([]byte(cvJSON), &cloudVolumes))

	require.Contains(t, cloudVolumes, "vol-0")
	vol := cloudVolumes["vol-0"]
	assert.Equal(t, "/mnt/secret", vol.MountPoint)
	assert.Equal(t, "ext4", vol.FSType)
	assert.Equal(t, "0", vol.LUN)
	assert.Equal(t, "/subscriptions/sub/disks/csi-vol-pvc-encrypted", vol.DiskID)
	assert.Equal(t, "LUKS", vol.EncryptType)
	assert.Equal(t, "default/key/volume-enc-key", vol.KeyID)
}

func TestCloudVolumes_NoEncryptionParamsWhenAbsent(t *testing.T) {
	dir := t.TempDir()
	overrideKataDirectVolumesDir(t, dir)

	service, cleanup := setupMockAgentAndService(t)
	defer cleanup()

	podUID := "pod-uid-plain-444"
	volPath := "/var/lib/kubelet/pods/" + podUID + "/volumes/kubernetes.io~csi/pvc-plain/mount"

	writeTestMountInfo(t, dir, volPath, map[string]interface{}{
		"device": "vol-abc123def",
		"fstype": "xfs",
	})

	req := newCreateContainerRequest("test-plain-vol").
		withAnnotations(map[string]string{
			"io.kubernetes.cri.sandbox-uid": podUID,
		}).
		withMounts(&pb.Mount{
			Destination: "/mnt/data",
			Source:      volPath,
			Type:        "bind",
		}).
		build()

	_, err := service.CreateContainer(context.Background(), req)
	require.NoError(t, err)

	cvJSON, ok := req.OCI.Annotations[util.CloudVolumesAnnotationKey]
	require.True(t, ok, "cloud_volumes annotation should be set")

	var cloudVolumes map[string]util.CloudVolumeAnnotation
	require.NoError(t, json.Unmarshal([]byte(cvJSON), &cloudVolumes))

	require.Contains(t, cloudVolumes, "vol-0")
	vol := cloudVolumes["vol-0"]
	assert.Equal(t, "", vol.EncryptType)
	assert.Equal(t, "", vol.KeyID)
}
