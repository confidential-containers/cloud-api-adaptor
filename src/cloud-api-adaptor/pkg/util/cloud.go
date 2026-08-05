package util

import (
	b64 "encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/pkg/initdata"
	provider "github.com/confidential-containers/cloud-api-adaptor/src/cloud-providers"
	cri "github.com/containerd/containerd/pkg/cri/annotations"
	hypannotations "github.com/kata-containers/kata-containers/src/runtime/virtcontainers/pkg/annotations"
)

func GetPodName(annotations map[string]string) string {

	sandboxName := annotations[cri.SandboxName]

	// cri-o stores the sandbox name in the form of k8s_<pod name>_<namespace>_<uid>_0
	// Extract the pod name from it.
	if tmp := strings.Split(sandboxName, "_"); len(tmp) > 1 && tmp[0] == "k8s" {
		return tmp[1]
	}

	return sandboxName
}

func GetPodNamespace(annotations map[string]string) string {

	return annotations[cri.SandboxNamespace]
}

// Method to get instance type from annotation
func GetInstanceTypeFromAnnotation(annotations map[string]string) string {
	// The machine_type annotation in Kata refers to VM type
	// For example machine_type for Kata/Qemu refers to pc, q35, microvm etc.
	// We use the same annotation for Kata/remote to refer to cloud instance type (flavor)
	return annotations[hypannotations.MachineType]
}

// Method to get image from annotation
func GetImageFromAnnotation(annotations map[string]string) string {
	// The image annotation in Kata refers to image path
	// For example image for Kata/Qemu refers to /hypervisor/image.img etc.
	// We use the same annotation for Kata/remote to refer to image name
	return annotations[hypannotations.ImagePath]
}

// Method to get vCPU, memory and gpus from annotations
func GetPodvmResourcesFromAnnotation(annotations map[string]string) (int64, int64, int64) {

	var vcpuInt, memoryInt, gpuInt int64
	var err error

	vcpu, ok := annotations[hypannotations.DefaultVCPUs]
	if ok {
		vcpuInt, err = strconv.ParseInt(vcpu, 10, 64)
		if err != nil {
			fmt.Printf("Error converting vcpu to int64. Defaulting to 0: %v\n", err)
			vcpuInt = 0
		}
	} else {
		vcpuInt = 0
	}

	memory, ok := annotations[hypannotations.DefaultMemory]
	if ok {
		// Use strconv.ParseInt to convert string to int64
		memoryInt, err = strconv.ParseInt(memory, 10, 64)
		if err != nil {
			fmt.Printf("Error converting memory to int64. Defaulting to 0: %v\n", err)
			memoryInt = 0
		}

	} else {
		memoryInt = 0
	}

	gpu, ok := annotations[hypannotations.DefaultGPUs]
	if ok {
		gpuInt, err = strconv.ParseInt(gpu, 10, 64)
		if err != nil {
			fmt.Printf("Error converting gpu to int64. Defaulting to 0: %v\n", err)
			gpuInt = 0
		}
	} else {
		gpuInt = 0
	}

	// Return vCPU, memory and GPU
	return vcpuInt, memoryInt, gpuInt
}

// Method to get initdata from annotation. Initdata is delivered as raw
// string by kata runtime, so we want to compress and base64 it again.
func GetInitdataFromAnnotation(annotations map[string]string) (string, error) {
	str := annotations["io.katacontainers.config.hypervisor.cc_init_data"]
	if str == "" {
		return "", nil
	}

	initdataEnc, err := initdata.Encode(str)
	if err != nil {
		return "", fmt.Errorf("failed to encode initdata: %w", err)
	}

	return initdataEnc, nil
}

// Method to check if a string exists in a slice
func Contains(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

var logger = log.New(log.Writer(), "[util/cloud] ", log.LstdFlags|log.Lmsgprefix)

type mountInfoJSON struct {
	VolumeType string            `json:"volume-type"`
	Device     string            `json:"device"`
	FsType     string            `json:"fstype"`
	Metadata   map[string]string `json:"metadata"`
	Options    []string          `json:"options"`
}

var KataDirectVolumesDir = "/run/kata-containers/shared/direct-volumes"

const CSIPluginEscapeQualifiedName = "kubernetes.io~csi"

const CloudVolumesAnnotationKey = "io.confidentialcontainers.org.cloud_volumes"

// CloudVolumeAnnotation is the schema for each entry in the cloud_volumes
// annotation. It is serialized by the proxy and deserialized by the interceptor.
type CloudVolumeAnnotation struct {
	MountPoint  string `json:"mount_point"`
	FSType      string `json:"fs_type"`
	LUN         string `json:"lun"`
	DiskID      string `json:"disk_id"`
	FSGroup     string `json:"fs_group,omitempty"`
	EncryptType string `json:"encrypt_type,omitempty"`
	KeyID       string `json:"key_id,omitempty"`
}

// GetCSIVolumesForPod scans the shared direct-volumes directory for
// mountInfo.json files written by the CSI block driver. Each file
// describes a cloud volume that should be attached to the PodVM.
// Volumes are filtered by pod UID (from annotations) to prevent
// cross-pod volume leakage on multi-tenant nodes.
func GetCSIVolumesForPod(annotations map[string]string) []provider.CloudVolume {
	var volumes []provider.CloudVolume

	podUID := annotations[cri.SandboxUID]

	entries, err := os.ReadDir(KataDirectVolumesDir)
	if err != nil {
		return nil
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		decodedPath, err := b64.URLEncoding.DecodeString(entry.Name())
		if err != nil {
			continue
		}
		decodedStr := string(decodedPath)

		if !strings.Contains(decodedStr, "/volumes/"+CSIPluginEscapeQualifiedName+"/") {
			continue
		}

		if podUID != "" && !strings.Contains(decodedStr, "/pods/"+podUID+"/") {
			continue
		}

		mountInfoPath := filepath.Join(KataDirectVolumesDir, entry.Name(), "mountInfo.json")
		data, err := os.ReadFile(mountInfoPath)
		if err != nil {
			continue
		}

		var info mountInfoJSON
		if err := json.Unmarshal(data, &info); err != nil {
			logger.Printf("WARNING: invalid mountInfo.json in %s: %v", entry.Name(), err)
			continue
		}

		volPath := info.Device
		if info.Metadata != nil {
			if cp, ok := info.Metadata["cloud-volume-path"]; ok && cp != "" {
				volPath = cp
			}
		}

		if volPath == "" {
			logger.Printf("WARNING: no disk ID in mountInfo.json for %s", decodedStr)
			continue
		}

		volumes = append(volumes, provider.CloudVolume{
			DiskID: volPath,
		})
	}

	return volumes
}
