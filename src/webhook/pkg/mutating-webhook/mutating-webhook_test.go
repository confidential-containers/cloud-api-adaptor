package mutatingwebhook

import (
	"os"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

func TestMutatePod_CpuMemReqLimit(t *testing.T) {
	// Mock environment variable
	os.Setenv("TARGET_RUNTIMECLASS", "kata-remote")
	os.Setenv("POD_VM_EXTENDED_RESOURCE", "kata.peerpods.io/vm")

	// Create a sample pod spec
	runtimeClassName := "kata-remote"
	pod := &corev1.Pod{
		Spec: corev1.PodSpec{
			RuntimeClassName: &runtimeClassName,
			Containers: []corev1.Container{
				{
					Name:  "container1",
					Image: "busybox",
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("2"),
							corev1.ResourceMemory: resource.MustParse("4Gi"),
						},
						Limits: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("3"),
							corev1.ResourceMemory: resource.MustParse("5Gi"),
						},
					},
				},
			},
		},
	}

	podMutator := &PodMutator{}
	mutatedPod, err := podMutator.mutatePod(pod)
	if err != nil {
		t.Fatalf("mutatePod() error = %v", err)
	}

	if mutatedPod.Annotations[PeerpodsCPUAnnotation] != "3" {
		t.Errorf("Expected CPU annotation to be 3, got %s", mutatedPod.Annotations[PeerpodsCPUAnnotation])
	}

	if mutatedPod.Annotations[PeerpodsMemoryAnnotation] != "5120" {
		t.Errorf("Expected Memory annotation to be 5120, got %s", mutatedPod.Annotations[PeerpodsMemoryAnnotation])
	}

	if _, exists := mutatedPod.Annotations[PeerpodsGPUAnnotation]; exists {
		t.Errorf("Expected no GPU annotation, got %s", mutatedPod.Annotations[PeerpodsGPUAnnotation])
	}
	// Check resource requirements for all containers are cleared
	// Except for the first container which has the peer-pod resource added
	for idx, container := range mutatedPod.Spec.Containers {
		// Skip the first container
		if idx == 0 {
			continue
		}

		if len(container.Resources.Requests) != 0 || len(container.Resources.Limits) != 0 {
			t.Errorf("Expected resources to be cleared for container %d, got Requests: %v, Limits: %v", idx, container.Resources.Requests, container.Resources.Limits)
		}
	}

	expectedResource := resource.MustParse("1")
	if !mutatedPod.Spec.Containers[0].Resources.Requests[corev1.ResourceName(PodVMExtendedResourceDefault)].Equal(expectedResource) {
		t.Errorf("Expected peer-pod VM resource request to be 1, got %v", mutatedPod.Spec.Containers[0].Resources.Requests[corev1.ResourceName(PodVMExtendedResourceDefault)])
	}
}

// Add test case with only CPU requests
func TestMutatePod_CpuReq(t *testing.T) {
	// Mock environment variable
	os.Setenv("TARGET_RUNTIMECLASS", "kata-remote")
	os.Setenv("POD_VM_EXTENDED_RESOURCE", "kata.peerpods.io/vm")

	// Create a sample pod spec
	runtimeClassName := "kata-remote"
	pod := &corev1.Pod{
		Spec: corev1.PodSpec{
			RuntimeClassName: &runtimeClassName,
			Containers: []corev1.Container{
				{
					Name:  "container1",
					Image: "busybox",
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU: resource.MustParse("2"),
						},
					},
				},
			},
		},
	}

	podMutator := &PodMutator{}
	mutatedPod, err := podMutator.mutatePod(pod)
	if err != nil {
		t.Fatalf("mutatePod() error = %v", err)
	}

	if mutatedPod.Annotations[PeerpodsCPUAnnotation] != "2" {
		t.Errorf("Expected CPU annotation to be 2, got %s", mutatedPod.Annotations[PeerpodsCPUAnnotation])
	}

	if _, exists := mutatedPod.Annotations[PeerpodsMemoryAnnotation]; exists {
		t.Errorf("Expected no Memory annotation, got %s", mutatedPod.Annotations[PeerpodsMemoryAnnotation])
	}

	if _, exists := mutatedPod.Annotations[PeerpodsGPUAnnotation]; exists {
		t.Errorf("Expected no GPU annotation, got %s", mutatedPod.Annotations[PeerpodsGPUAnnotation])
	}
	// Check resource requirements for all containers are cleared
	// Except for the first container which has the peer-pod resource added
	for idx, container := range mutatedPod.Spec.Containers {
		// Skip the first container
		if idx == 0 {
			continue
		}

		if len(container.Resources.Requests) != 0 || len(container.Resources.Limits) != 0 {
			t.Errorf("Expected resources to be cleared for container %d, got Requests: %v, Limits: %v", idx, container.Resources.Requests, container.Resources.Limits)
		}
	}

	expectedResource := resource.MustParse("1")
	if !mutatedPod.Spec.Containers[0].Resources.Requests[corev1.ResourceName(PodVMExtendedResourceDefault)].Equal(expectedResource) {
		t.Errorf("Expected peer-pod VM resource request to be 1, got %v", mutatedPod.Spec.Containers[0].Resources.Requests[corev1.ResourceName(PodVMExtendedResourceDefault)])
	}
}

// Add test case with only Memory requests
func TestMutatePod_MemReq(t *testing.T) {
	// Mock environment variable
	os.Setenv("TARGET_RUNTIMECLASS", "kata-remote")
	os.Setenv("POD_VM_EXTENDED_RESOURCE", "kata.peerpods.io/vm")

	// Create a sample pod spec
	runtimeClassName := "kata-remote"
	pod := &corev1.Pod{
		Spec: corev1.PodSpec{
			RuntimeClassName: &runtimeClassName,
			Containers: []corev1.Container{
				{
					Name:  "container1",
					Image: "busybox",
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceMemory: resource.MustParse("4Gi"),
						},
					},
				},
			},
		},
	}

	podMutator := &PodMutator{}
	mutatedPod, err := podMutator.mutatePod(pod)
	if err != nil {
		t.Fatalf("mutatePod() error = %v", err)
	}

	if _, exists := mutatedPod.Annotations[PeerpodsCPUAnnotation]; exists {
		t.Errorf("Expected no CPU annotation, got %s", mutatedPod.Annotations[PeerpodsCPUAnnotation])
	}

	if mutatedPod.Annotations[PeerpodsMemoryAnnotation] != "4096" {
		t.Errorf("Expected Memory annotation to be 4096, got %s", mutatedPod.Annotations[PeerpodsMemoryAnnotation])
	}

	if _, exists := mutatedPod.Annotations[PeerpodsGPUAnnotation]; exists {
		t.Errorf("Expected no GPU annotation, got %s", mutatedPod.Annotations[PeerpodsGPUAnnotation])
	}

	// Check resource requirements for all containers are cleared
	// Except for the first container which has the peer-pod resource added
	for idx, container := range mutatedPod.Spec.Containers {
		// Skip the first container
		if idx == 0 {
			continue
		}

		if len(container.Resources.Requests) != 0 || len(container.Resources.Limits) != 0 {
			t.Errorf("Expected resources to be cleared for container %d, got Requests: %v, Limits: %v", idx, container.Resources.Requests, container.Resources.Limits)
		}
	}

	expectedResource := resource.MustParse("1")
	if !mutatedPod.Spec.Containers[0].Resources.Requests[corev1.ResourceName(PodVMExtendedResourceDefault)].Equal(expectedResource) {
		t.Errorf("Expected peer-pod VM resource request to be 1, got %v", mutatedPod.Spec.Containers[0].Resources.Requests[corev1.ResourceName(PodVMExtendedResourceDefault)])
	}
}

// Add test case with only GPU requests
func TestMutatePod_GpuReq(t *testing.T) {
	// Mock environment variable
	os.Setenv("TARGET_RUNTIMECLASS", "kata-remote")
	os.Setenv("POD_VM_EXTENDED_RESOURCE", "kata.peerpods.io/vm")

	// Create a sample pod spec
	runtimeClassName := "kata-remote"
	pod := &corev1.Pod{
		Spec: corev1.PodSpec{
			RuntimeClassName: &runtimeClassName,
			Containers: []corev1.Container{
				{
					Name:  "container1",
					Image: "busybox",
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceName(GPUResourceName): resource.MustParse("1"),
						},
					},
				},
			},
		},
	}

	podMutator := &PodMutator{}
	mutatedPod, err := podMutator.mutatePod(pod)
	if err != nil {
		t.Fatalf("mutatePod() error = %v", err)
	}

	if _, exists := mutatedPod.Annotations[PeerpodsCPUAnnotation]; exists {
		t.Errorf("Expected no CPU annotation, got %s", mutatedPod.Annotations[PeerpodsCPUAnnotation])
	}

	if _, exists := mutatedPod.Annotations[PeerpodsMemoryAnnotation]; exists {
		t.Errorf("Expected no Memory annotation, got %s", mutatedPod.Annotations[PeerpodsMemoryAnnotation])
	}

	if mutatedPod.Annotations[PeerpodsGPUAnnotation] != "1" {
		t.Errorf("Expected GPU annotation to be 1, got %s", mutatedPod.Annotations[PeerpodsGPUAnnotation])
	}

	// Check resource requirements for all containers are cleared
	// Except for the first container which has the peer-pod resource added
	for idx, container := range mutatedPod.Spec.Containers {
		// Skip the first container
		if idx == 0 {
			continue
		}

		if len(container.Resources.Requests) != 0 || len(container.Resources.Limits) != 0 {
			t.Errorf("Expected resources to be cleared for container %d, got Requests: %v, Limits: %v",
				idx, container.Resources.Requests, container.Resources.Limits)
		}
	}

	expectedResource := resource.MustParse("1")
	if !mutatedPod.Spec.Containers[0].Resources.Requests[corev1.ResourceName(PodVMExtendedResourceDefault)].Equal(expectedResource) {
		t.Errorf("Expected peer-pod VM resource request to be 1, got %v",
			mutatedPod.Spec.Containers[0].Resources.Requests[corev1.ResourceName(PodVMExtendedResourceDefault)])
	}
}

// Add test case with no resource requests
func TestMutatePod_NoReq(t *testing.T) {
	// Mock environment variable
	os.Setenv("TARGET_RUNTIMECLASS", "kata-remote")
	os.Setenv("POD_VM_EXTENDED_RESOURCE", "kata.peerpods.io/vm")

	// Create a sample pod spec
	runtimeClassName := "kata-remote"
	pod := &corev1.Pod{
		Spec: corev1.PodSpec{
			RuntimeClassName: &runtimeClassName,
			Containers: []corev1.Container{
				{
					Name:  "container1",
					Image: "busybox",
				},
			},
		},
	}

	podMutator := &PodMutator{}
	mutatedPod, err := podMutator.mutatePod(pod)
	if err != nil {
		t.Fatalf("mutatePod() error = %v", err)
	}

	if _, exists := mutatedPod.Annotations[PeerpodsCPUAnnotation]; exists {
		t.Errorf("Expected no CPU annotation, got %s", mutatedPod.Annotations[PeerpodsCPUAnnotation])
	}

	if _, exists := mutatedPod.Annotations[PeerpodsMemoryAnnotation]; exists {
		t.Errorf("Expected no Memory annotation, got %s", mutatedPod.Annotations[PeerpodsMemoryAnnotation])
	}

	if _, exists := mutatedPod.Annotations[PeerpodsGPUAnnotation]; exists {
		t.Errorf("Expected no GPU annotation, got %s", mutatedPod.Annotations[PeerpodsGPUAnnotation])
	}

	// Check resource requirements for all containers are cleared
	// Except for the first container which has the peer-pod resource added
	for idx, container := range mutatedPod.Spec.Containers {
		// Skip the first container
		if idx == 0 {
			continue
		}

		if len(container.Resources.Requests) != 0 || len(container.Resources.Limits) != 0 {
			t.Errorf("Expected resources to be cleared for container %d, got Requests: %v, Limits: %v",
				idx, container.Resources.Requests, container.Resources.Limits)
		}
	}

	expectedResource := resource.MustParse("1")
	if !mutatedPod.Spec.Containers[0].Resources.Requests[corev1.ResourceName(PodVMExtendedResourceDefault)].Equal(expectedResource) {
		t.Errorf("Expected peer-pod VM resource request to be 1, got %v",
			mutatedPod.Spec.Containers[0].Resources.Requests[corev1.ResourceName(PodVMExtendedResourceDefault)])
	}
}

// Add test case with same request limit values
func TestMutatePod_CpuMemReqLimitSame(t *testing.T) {
	// Mock environment variable
	os.Setenv("TARGET_RUNTIMECLASS", "kata-remote")
	os.Setenv("POD_VM_EXTENDED_RESOURCE", "kata.peerpods.io/vm")

	// Create a sample pod spec
	runtimeClassName := "kata-remote"
	pod := &corev1.Pod{
		Spec: corev1.PodSpec{
			RuntimeClassName: &runtimeClassName,
			Containers: []corev1.Container{
				{
					Name:  "container1",
					Image: "busybox",
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("2"),
							corev1.ResourceMemory: resource.MustParse("4Gi"),
						},
						Limits: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("2"),
							corev1.ResourceMemory: resource.MustParse("4Gi"),
						},
					},
				},
			},
		},
	}

	podMutator := &PodMutator{}
	mutatedPod, err := podMutator.mutatePod(pod)
	if err != nil {
		t.Fatalf("mutatePod() error = %v", err)
	}

	if mutatedPod.Annotations[PeerpodsCPUAnnotation] != "2" {
		t.Errorf("Expected CPU annotation to be 3, got %s", mutatedPod.Annotations[PeerpodsCPUAnnotation])
	}

	if mutatedPod.Annotations[PeerpodsMemoryAnnotation] != "4096" {
		t.Errorf("Expected Memory annotation to be 4096, got %s", mutatedPod.Annotations[PeerpodsMemoryAnnotation])
	}

	if _, exists := mutatedPod.Annotations[PeerpodsGPUAnnotation]; exists {
		t.Errorf("Expected no GPU annotation, got %s", mutatedPod.Annotations[PeerpodsGPUAnnotation])
	}

	// Check resource requirements for all containers are cleared
	// Except for the first container which has the peer-pod resource added
	for idx, container := range mutatedPod.Spec.Containers {
		// Skip the first container
		if idx == 0 {
			continue
		}

		if len(container.Resources.Requests) != 0 || len(container.Resources.Limits) != 0 {
			t.Errorf("Expected resources to be cleared for container %d, got Requests: %v, Limits: %v",
				idx, container.Resources.Requests, container.Resources.Limits)
		}
	}

	expectedResource := resource.MustParse("1")
	if !mutatedPod.Spec.Containers[0].Resources.Requests[corev1.ResourceName(PodVMExtendedResourceDefault)].Equal(expectedResource) {
		t.Errorf("Expected peer-pod VM resource request to be 1, got %v",
			mutatedPod.Spec.Containers[0].Resources.Requests[corev1.ResourceName(PodVMExtendedResourceDefault)])
	}
}

// Add test case with initContainers and multiple containers requests and limit values
func TestMutatePod_InitContainers(t *testing.T) {
	// Mock environment variable
	os.Setenv("TARGET_RUNTIMECLASS", "kata-remote")
	os.Setenv("POD_VM_EXTENDED_RESOURCE", "kata.peerpods.io/vm")

	// Create a sample pod spec
	runtimeClassName := "kata-remote"
	pod := &corev1.Pod{
		Spec: corev1.PodSpec{
			RuntimeClassName: &runtimeClassName,
			InitContainers: []corev1.Container{
				{
					Name:  "init-container1",
					Image: "busybox",
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("1"),
							corev1.ResourceMemory: resource.MustParse("2Gi"),
						},
						Limits: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("1"),
							corev1.ResourceMemory: resource.MustParse("2Gi"),
						},
					},
				},
			},
			Containers: []corev1.Container{
				{
					Name:  "container1",
					Image: "busybox",
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("2"),
							corev1.ResourceMemory: resource.MustParse("4Gi"),
						},
						Limits: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("3"),
							corev1.ResourceMemory: resource.MustParse("5Gi"),
						},
					},
				},
				{
					Name:  "container2",
					Image: "nginx",
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("1"),
							corev1.ResourceMemory: resource.MustParse("1Gi"),
						},
						Limits: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("1"),
							corev1.ResourceMemory: resource.MustParse("1Gi"),
						},
					},
				},
				{
					// GPU container
					Name: "container3",
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:                   resource.MustParse("1"),
							corev1.ResourceName(GPUResourceName): resource.MustParse("1"),
						},
						Limits: corev1.ResourceList{
							corev1.ResourceCPU:                   resource.MustParse("1"),
							corev1.ResourceName(GPUResourceName): resource.MustParse("1"),
						},
					},
				},
			},
		},
	}

	podMutator := &PodMutator{}
	mutatedPod, err := podMutator.mutatePod(pod)
	if err != nil {
		t.Fatalf("mutatePod() error = %v", err)
	}

	// Check annotations
	if mutatedPod.Annotations[PeerpodsCPUAnnotation] != "6" {
		t.Errorf("Expected CPU annotation to be 6, got %s", mutatedPod.Annotations[PeerpodsCPUAnnotation])
	}

	if mutatedPod.Annotations[PeerpodsMemoryAnnotation] != "8192" {
		t.Errorf("Expected Memory annotation to be 8192, got %s", mutatedPod.Annotations[PeerpodsMemoryAnnotation])
	}

	if mutatedPod.Annotations[PeerpodsGPUAnnotation] != "1" {
		t.Errorf("Expected GPU annotation to be 1, got %s", mutatedPod.Annotations[PeerpodsGPUAnnotation])
	}

	// Check resource requirements for all containers are cleared
	// Except for the first container which has the peer-pod resource added
	for idx, container := range mutatedPod.Spec.Containers {
		// Skip the first container
		if idx == 0 {
			continue
		}

		if len(container.Resources.Requests) != 0 || len(container.Resources.Limits) != 0 {
			t.Errorf("Expected resources to be cleared for container %d, got Requests: %v, Limits: %v",
				idx, container.Resources.Requests, container.Resources.Limits)
		}
	}

	// Check resource requirements for all init containers are cleared
	for idx, container := range mutatedPod.Spec.InitContainers {
		if len(container.Resources.Requests) != 0 || len(container.Resources.Limits) != 0 {
			t.Errorf("Expected resources to be cleared for init container %d, got Requests: %v, Limits: %v",
				idx, container.Resources.Requests, container.Resources.Limits)
		}
	}

	// Check that the first container has the peer-pod resource added
	expectedResource := resource.MustParse("1")
	if !mutatedPod.Spec.Containers[0].Resources.Requests[corev1.ResourceName(PodVMExtendedResourceDefault)].Equal(expectedResource) {
		t.Errorf("Expected VM resource request to be 1, got %v",
			mutatedPod.Spec.Containers[0].Resources.Requests[corev1.ResourceName(PodVMExtendedResourceDefault)])
	}
}

func TestMutatePod_NoChangeForDifferentRuntimeClass(t *testing.T) {
	// Mock environment variable
	os.Setenv("TARGET_RUNTIMECLASS", "kata-remote")

	// Create a sample pod spec with different runtime class
	runtimeClassName := "different-runtime"
	pod := &corev1.Pod{
		Spec: corev1.PodSpec{
			RuntimeClassName: &runtimeClassName,
			Containers: []corev1.Container{
				{
					Name:  "container1",
					Image: "busybox",
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("2"),
							corev1.ResourceMemory: resource.MustParse("4Gi"),
						},
						Limits: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("3"),
							corev1.ResourceMemory: resource.MustParse("5Gi"),
						},
					},
				},
			},
		},
	}

	podMutator := &PodMutator{}
	mutatedPod, err := podMutator.mutatePod(pod)
	if err != nil {
		t.Fatalf("mutatePod() error = %v", err)
	}

	// Check that the pod spec has not been mutated
	if _, exists := mutatedPod.Annotations[PeerpodsCPUAnnotation]; exists {
		t.Errorf("Expected no CPU annotation, got %s", mutatedPod.Annotations[PeerpodsCPUAnnotation])
	}

	if _, exists := mutatedPod.Annotations[PeerpodsMemoryAnnotation]; exists {
		t.Errorf("Expected no Memory annotation, got %s", mutatedPod.Annotations[PeerpodsMemoryAnnotation])
	}

	if _, exists := mutatedPod.Annotations[PeerpodsGPUAnnotation]; exists {
		t.Errorf("Expected no GPU annotation, got %s", mutatedPod.Annotations[PeerpodsGPUAnnotation])
	}

	// Check resource requirements for all containers; it shouldn't be cleared
	for idx, container := range mutatedPod.Spec.Containers {
		if len(container.Resources.Requests) == 0 || len(container.Resources.Limits) == 0 {
			t.Errorf("Expected resources to be present for container %d, got Requests: %v, Limits: %v",
				idx, container.Resources.Requests, container.Resources.Limits)
		}
	}
}

// Add test case with fractional CPU requests (eg 200m, 0.5) and limits (eg 500m, 0.5)
func TestMutatePod_FractionalCpu(t *testing.T) {
	// Mock environment variable
	os.Setenv("TARGET_RUNTIMECLASS", "kata-remote")
	os.Setenv("POD_VM_EXTENDED_RESOURCE", "kata.peerpods.io/vm")

	// Create a sample pod spec with fractional CPU requests and limits
	runtimeClassName := "kata-remote"
	pod := &corev1.Pod{
		Spec: corev1.PodSpec{
			RuntimeClassName: &runtimeClassName,
			Containers: []corev1.Container{
				{
					Name:  "container1",
					Image: "busybox",
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("200m"),
							corev1.ResourceMemory: resource.MustParse("4Gi"),
						},
						Limits: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("500m"),
							corev1.ResourceMemory: resource.MustParse("4Gi"),
						},
					},
				},
			},
		},
	}

	podMutator := &PodMutator{}
	mutatedPod, err := podMutator.mutatePod(pod)
	if err != nil {
		t.Fatalf("mutatePod() error = %v", err)
	}

	// Check annotations
	if mutatedPod.Annotations[PeerpodsCPUAnnotation] != "1" {
		t.Errorf("Expected CPU annotation to be 1, got %s", mutatedPod.Annotations[PeerpodsCPUAnnotation])
	}

	if mutatedPod.Annotations[PeerpodsMemoryAnnotation] != "4096" {
		t.Errorf("Expected Memory annotation to be 4096, got %s", mutatedPod.Annotations[PeerpodsMemoryAnnotation])
	}

	if _, exists := mutatedPod.Annotations[PeerpodsGPUAnnotation]; exists {
		t.Errorf("Expected no GPU annotation, got %s", mutatedPod.Annotations[PeerpodsGPUAnnotation])
	}

	// Check resource requirements for all containers are cleared
	// Except for the first container which has the peer-pod resource added
	for idx, container := range mutatedPod.Spec.Containers {
		// Skip the first container
		if idx == 0 {
			continue
		}

		if len(container.Resources.Requests) != 0 || len(container.Resources.Limits) != 0 {
			t.Errorf("Expected resources to be cleared for container %d, got Requests: %v, Limits: %v",
				idx, container.Resources.Requests, container.Resources.Limits)
		}
	}

	expectedResource := resource.MustParse("1")
	if !mutatedPod.Spec.Containers[0].Resources.Requests[corev1.ResourceName(PodVMExtendedResourceDefault)].Equal(expectedResource) {
		t.Errorf("Expected peer-pod VM resource request to be 1, got %v",
			mutatedPod.Spec.Containers[0].Resources.Requests[corev1.ResourceName(PodVMExtendedResourceDefault)])
	}
}

func TestMutatePod_FractionalCpuMoreThanOne(t *testing.T) {
	// Mock environment variable
	os.Setenv("TARGET_RUNTIMECLASS", "kata-remote")
	os.Setenv("POD_VM_EXTENDED_RESOURCE", "kata.peerpods.io/vm")

	// Create a sample pod spec with fractional CPU requests and limits
	runtimeClassName := "kata-remote"
	pod := &corev1.Pod{
		Spec: corev1.PodSpec{
			RuntimeClassName: &runtimeClassName,
			Containers: []corev1.Container{
				{
					Name:  "container1",
					Image: "busybox",
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("200m"),
							corev1.ResourceMemory: resource.MustParse("4Gi"),
						},
						Limits: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("1500m"),
							corev1.ResourceMemory: resource.MustParse("4Gi"),
						},
					},
				},
			},
		},
	}

	podMutator := &PodMutator{}
	mutatedPod, err := podMutator.mutatePod(pod)
	if err != nil {
		t.Fatalf("mutatePod() error = %v", err)
	}

	// Check annotations
	if mutatedPod.Annotations[PeerpodsCPUAnnotation] != "2" {
		t.Errorf("Expected CPU annotation to be 1, got %s", mutatedPod.Annotations[PeerpodsCPUAnnotation])
	}

	if mutatedPod.Annotations[PeerpodsMemoryAnnotation] != "4096" {
		t.Errorf("Expected Memory annotation to be 4096, got %s", mutatedPod.Annotations[PeerpodsMemoryAnnotation])
	}

	if _, exists := mutatedPod.Annotations[PeerpodsGPUAnnotation]; exists {
		t.Errorf("Expected no GPU annotation, got %s", mutatedPod.Annotations[PeerpodsGPUAnnotation])
	}

	// Check resource requirements for all containers are cleared
	// Except for the first container which has the peer-pod resource added
	for idx, container := range mutatedPod.Spec.Containers {
		// Skip the first container
		if idx == 0 {
			continue
		}

		if len(container.Resources.Requests) != 0 || len(container.Resources.Limits) != 0 {
			t.Errorf("Expected resources to be cleared for container %d, got Requests: %v, Limits: %v",
				idx, container.Resources.Requests, container.Resources.Limits)
		}
	}

	expectedResource := resource.MustParse("1")
	if !mutatedPod.Spec.Containers[0].Resources.Requests[corev1.ResourceName(PodVMExtendedResourceDefault)].Equal(expectedResource) {
		t.Errorf("Expected peer-pod VM resource request to be 1, got %v",
			mutatedPod.Spec.Containers[0].Resources.Requests[corev1.ResourceName(PodVMExtendedResourceDefault)])
	}
}

// Add test case with fractional CPU requests (eg 200m, 0.5) and limits (eg 500m, 0.5)
// Add multiple containers and init containers

func TestMutatePod_FractionalCpu_MultipleContainers(t *testing.T) {
	// Mock environment variable
	os.Setenv("TARGET_RUNTIMECLASS", "kata-remote")
	os.Setenv("POD_VM_EXTENDED_RESOURCE", "kata.peerpods.io/vm")

	// Create a sample pod spec with fractional CPU requests and limits
	runtimeClassName := "kata-remote"
	pod := &corev1.Pod{
		Spec: corev1.PodSpec{
			RuntimeClassName: &runtimeClassName,
			InitContainers: []corev1.Container{
				{
					Name:  "init-container1",
					Image: "busybox",
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("200m"),
							corev1.ResourceMemory: resource.MustParse("4Gi"),
						},
						Limits: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("0.7"),
							corev1.ResourceMemory: resource.MustParse("4Gi"),
						},
					},
				},
			},
			Containers: []corev1.Container{
				{
					Name:  "container1",
					Image: "busybox",
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("200m"),
							corev1.ResourceMemory: resource.MustParse("4Gi"),
						},
						Limits: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("0.5"),
							corev1.ResourceMemory: resource.MustParse("4Gi"),
						},
					},
				},
			},
		},
	}

	podMutator := &PodMutator{}
	mutatedPod, err := podMutator.mutatePod(pod)
	if err != nil {
		t.Fatalf("mutatePod() error = %v", err)
	}

	// Check annotations
	if mutatedPod.Annotations[PeerpodsCPUAnnotation] != "2" {
		t.Errorf("Expected CPU annotation to be 2, got %s", mutatedPod.Annotations[PeerpodsCPUAnnotation])
	}

	if mutatedPod.Annotations[PeerpodsMemoryAnnotation] != "8192" {
		t.Errorf("Expected Memory annotation to be 4096, got %s", mutatedPod.Annotations[PeerpodsMemoryAnnotation])
	}

	// Check resource requirements for all containers are cleared
	// Except for the first container which has the peer-pod resource added
	for idx, container := range mutatedPod.Spec.Containers {
		// Skip the first container
		if idx == 0 {
			continue
		}

		if len(container.Resources.Requests) != 0 || len(container.Resources.Limits) != 0 {
			t.Errorf("Expected resources to be cleared for container %d, got Requests: %v, Limits: %v",
				idx, container.Resources.Requests, container.Resources.Limits)
		}
	}

	expectedResource := resource.MustParse("1")
	if !mutatedPod.Spec.Containers[0].Resources.Requests[corev1.ResourceName(PodVMExtendedResourceDefault)].Equal(expectedResource) {
		t.Errorf("Expected peer-pod VM resource request to be 1, got %v",
			mutatedPod.Spec.Containers[0].Resources.Requests[corev1.ResourceName(PodVMExtendedResourceDefault)])
	}
}
