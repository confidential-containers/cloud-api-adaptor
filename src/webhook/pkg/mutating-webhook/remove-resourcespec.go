package mutatingwebhook

import (
	"log"
	"os"
	"strconv"

	"github.com/confidential-containers/cloud-api-adaptor/src/webhook/pkg/utils"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

const (
	RuntimeClassNameDefault      = "kata-remote"
	PodVMExtendedResourceDefault = "kata.peerpods.io/vm"
	PeerpodsCPUAnnotation        = "io.katacontainers.config.hypervisor.default_vcpus"
	PeerpodsMemoryAnnotation     = "io.katacontainers.config.hypervisor.default_memory"
	GPUResourceName              = "nvidia.com/gpu"
	PeerpodsGPUAnnotation        = "io.katacontainers.config.hypervisor.default_gpus"
)

var logger = log.New(log.Writer(), "[pod-mutator] ", log.LstdFlags|log.Lmsgprefix)

// mutate POD spec
// remove the POD resource spec
func (a *PodMutator) mutatePod(pod *corev1.Pod) (*corev1.Pod, error) {
	var runtimeClassName string
	mpod := pod.DeepCopy()

	if runtimeClassName = os.Getenv("TARGET_RUNTIMECLASS"); runtimeClassName == "" {
		runtimeClassName = RuntimeClassNameDefault
	}
	// Mutate only if the POD is using specific runtimeClass
	if mpod.Spec.RuntimeClassName == nil || *mpod.Spec.RuntimeClassName != runtimeClassName {
		return mpod, nil
	}

	mpod = adjustResourceSpec(mpod)

	return mpod, nil
}

// function to remove resource spec from the pod spec
// add the cumulative resources as annotation to pod spec
// add the peer-pod resource to the first container in the pod spec

func adjustResourceSpec(pod *corev1.Pod) *corev1.Pod {

	// Get total CPU resource requests
	cpuRequest := utils.GetResourceRequestQuantity(pod, corev1.ResourceCPU)

	// Get total CPU resource limits
	cpuLimit := utils.GetResourceLimitQuantity(pod, corev1.ResourceCPU)

	// Get total Memory resource requests
	memoryRequest := utils.GetResourceRequestQuantity(pod, corev1.ResourceMemory)

	// Get total Memory resource limits
	memoryLimit := utils.GetResourceLimitQuantity(pod, corev1.ResourceMemory)

	// Get total GPU resource requests
	// GPU resources are always requested in whole numbers and request and limits are same.
	// So we don't need to check for limits
	gpuRequest := utils.GetResourceRequestQuantity(pod, corev1.ResourceName(GPUResourceName))

	// log the resource values
	logger.Printf("CPU Request: %s, CPU Limit: %s, Memory Request: %s, Memory Limit: %s, GPU Request: %s",
		cpuRequest.String(), cpuLimit.String(), memoryRequest.String(), memoryLimit.String(), gpuRequest.String())

	// Add the cumulative resources as annotation to pod spec
	// Use cpuLimit and memoryLimit if those are greater than cpuResource and memoryResource
	annotations := pod.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}

	// A non-existent request or limit will be 0 and we don't need to add annotation for 0
	// We only add annotation if the value is greater than 0
	// Limit will always be preferred over request

	// Add cpu annotation
	if !cpuRequest.IsZero() && cpuLimit.Cmp(cpuRequest) >= 0 {
		logger.Printf("Adding CPU annotation based on cpuLimit (integer value): %d", cpuLimit.Value())
		// We need the scaled value for the annotation and not the raw value like 1000m, 0.4 etc for CPU
		annotations[PeerpodsCPUAnnotation] = strconv.FormatInt(cpuLimit.Value(), 10)
	} else if cpuRequest.Sign() == 1 {
		logger.Printf("Adding CPU annotation based on cpuRequest (integer value): %d", cpuRequest.Value())
		annotations[PeerpodsCPUAnnotation] = strconv.FormatInt(cpuRequest.Value(), 10)
	}

	// Add memory annotation
	if !memoryRequest.IsZero() && memoryLimit.Cmp(memoryRequest) >= 0 {
		logger.Printf("Adding Memory annotation based on memoryLimit: %s", memoryLimit.String())
		memoryLimitMiBStr, err := utils.ConvertMemoryQuantityToMib(memoryLimit)
		if err != nil {
			logger.Printf("Error converting memory quantity to MiB: %v", err)
		}
		annotations[PeerpodsMemoryAnnotation] = memoryLimitMiBStr

	} else if memoryRequest.Sign() == 1 {
		logger.Printf("Adding Memory annotation based on memoryRequest: %s", memoryRequest.String())
		memoryRequestMiBStr, err := utils.ConvertMemoryQuantityToMib(memoryRequest)
		if err != nil {
			logger.Printf("Error converting memory quantity to MiB: %v", err)
		}
		annotations[PeerpodsMemoryAnnotation] = memoryRequestMiBStr
	}

	// Add GPU annotation
	if gpuRequest.Sign() == 1 {
		logger.Printf("Adding GPU annotation based on gpuRequest: %s", gpuRequest.String())
		annotations[PeerpodsGPUAnnotation] = gpuRequest.String()
	}

	pod.SetAnnotations(annotations)

	// Remove all resource specs
	for idx := range pod.Spec.Containers {
		pod.Spec.Containers[idx].Resources = corev1.ResourceRequirements{}
	}

	for idx := range pod.Spec.InitContainers {
		pod.Spec.InitContainers[idx].Resources = corev1.ResourceRequirements{}
	}

	// Add peer-pod resource to one container
	pod.Spec.Containers[0].Resources = defaultContainerResourceRequirements()
	return pod
}

// defaultContainerResourceRequirements returns the default requirements for a container
func defaultContainerResourceRequirements() corev1.ResourceRequirements {
	requirements := corev1.ResourceRequirements{}
	requirements.Requests = corev1.ResourceList{}
	requirements.Limits = corev1.ResourceList{}

	var podVMExtResource string
	if podVMExtResource = os.Getenv("POD_VM_EXTENDED_RESOURCE"); podVMExtResource == "" {
		podVMExtResource = PodVMExtendedResourceDefault
	}

	requirements.Requests[corev1.ResourceName(podVMExtResource)] = resource.MustParse("1")
	requirements.Limits[corev1.ResourceName(podVMExtResource)] = resource.MustParse("1")
	return requirements
}
