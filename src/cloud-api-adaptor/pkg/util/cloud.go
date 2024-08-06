package util

import (
	"fmt"
	"strconv"
	"strings"

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

// Method to get vCPU and memory from annotations
func GetCPUAndMemoryFromAnnotation(annotations map[string]string) (int64, int64) {

	var vcpuInt, memoryInt int64
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

	// Return vCPU and memory
	return vcpuInt, memoryInt
}

// Method to get initdata from annotation
func GetInitdataFromAnnotation(annotations map[string]string) string {
	return annotations["io.katacontainers.config.runtime.cc_init_data"]
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
