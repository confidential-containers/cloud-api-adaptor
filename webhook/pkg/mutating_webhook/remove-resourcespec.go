package mutating_webhook

import (
	"os"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

const (
	RUNTIME_CLASS_NAME_DEFAULT       = "kata-remote-cc"
	POD_VM_ANNOTATION_INSTANCE_TYPE  = "kata.peerpods.io/instance_type"
	POD_VM_INSTANCE_TYPE_DEFAULT     = "t2.small"
	POD_VM_EXTENDED_RESOURCE_DEFAULT = "kata.peerpods.io/vm"
)

// remove the POD resource spec
func removePodResourceSpec(pod *corev1.Pod) (*corev1.Pod, error) {
	var runtimeClassName string
	mpod := pod.DeepCopy()

	if runtimeClassName = os.Getenv("TARGET_RUNTIMECLASS"); runtimeClassName == "" {
		runtimeClassName = RUNTIME_CLASS_NAME_DEFAULT
	}
	// Mutate only if the POD is using specific runtimeClass
	if mpod.Spec.RuntimeClassName == nil || *mpod.Spec.RuntimeClassName != runtimeClassName {
		return mpod, nil
	}

	var podVmInstanceType string
	if podVmInstanceType = os.Getenv("POD_VM_INSTANCE_TYPE"); podVmInstanceType == "" {
		podVmInstanceType = POD_VM_INSTANCE_TYPE_DEFAULT
	}

	if mpod.Annotations == nil {
		mpod.Annotations = map[string]string{}
	}

	mpod.Annotations[POD_VM_ANNOTATION_INSTANCE_TYPE] = podVmInstanceType

	// Remove all resource specs
	for idx := range mpod.Spec.Containers {
		mpod.Spec.Containers[idx].Resources = corev1.ResourceRequirements{}
	}

	for idx := range mpod.Spec.InitContainers {
		mpod.Spec.InitContainers[idx].Resources = corev1.ResourceRequirements{}
	}

	// Add peer-pod resource to one container
	mpod.Spec.Containers[0].Resources = defaultContainerResourceRequirements()
	return mpod, nil
}

// defaultContainerResourceRequirements returns the default requirements for a container
func defaultContainerResourceRequirements() corev1.ResourceRequirements {
	requirements := corev1.ResourceRequirements{}
	requirements.Requests = corev1.ResourceList{}
	requirements.Limits = corev1.ResourceList{}

	var podVmExtResource string
	if podVmExtResource = os.Getenv("POD_VM_EXTENDED_RESOURCE"); podVmExtResource == "" {
		podVmExtResource = POD_VM_EXTENDED_RESOURCE_DEFAULT
	}

	requirements.Requests[corev1.ResourceName(podVmExtResource)] = resource.MustParse("1")
	requirements.Limits[corev1.ResourceName(podVmExtResource)] = resource.MustParse("1")
	return requirements
}
