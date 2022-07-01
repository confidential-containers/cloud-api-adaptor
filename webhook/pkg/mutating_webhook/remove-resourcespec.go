package mutating_webhook

import (
	"os"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

const (
	DEFAULT_RUNTIME_CLASS_NAME  = "kata-remote-cc"
	VM_ANNOTATION_INSTANCE_TYPE = "kata.peerpods.io/instance_type"
	VM_INSTANCE_TYPE_DEFAULT    = "t2.small"
	VM_EXTENDED_RESOURCE        = "kata.peerpods.io/vm"
)

// remove the POD resource spec
func removePodResourceSpec(pod *corev1.Pod) (*corev1.Pod, error) {
	var runtimeClassName string
	mpod := pod.DeepCopy()

	if runtimeClassName = os.Getenv("TARGET_RUNTIMECLASS"); runtimeClassName == "" {
		runtimeClassName = DEFAULT_RUNTIME_CLASS_NAME
	}
	// Mutate only if the POD is using specific runtimeClass
	if mpod.Spec.RuntimeClassName == nil || *mpod.Spec.RuntimeClassName != runtimeClassName {
		return mpod, nil
	}

	if mpod.Annotations == nil {
		mpod.Annotations = map[string]string{}
	}

	mpod.Annotations[VM_ANNOTATION_INSTANCE_TYPE] = VM_INSTANCE_TYPE_DEFAULT

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

	requirements.Requests[VM_EXTENDED_RESOURCE] = resource.MustParse("1")
	requirements.Limits[VM_EXTENDED_RESOURCE] = resource.MustParse("1")
	return requirements
}
