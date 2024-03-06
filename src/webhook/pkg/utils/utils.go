package utils

import (
	"fmt"
	"sort"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

// GetResourceRequestQuantity finds and returns the request quantity for a specific resource.
func GetResourceRequestQuantity(pod *corev1.Pod, resourceName corev1.ResourceName) resource.Quantity {
	requestQuantity := resource.Quantity{}

	switch resourceName {
	case corev1.ResourceCPU:
		requestQuantity = resource.Quantity{Format: resource.DecimalSI}
	case corev1.ResourceMemory:
		requestQuantity = resource.Quantity{Format: resource.BinarySI}
	default:
		requestQuantity = resource.Quantity{Format: resource.DecimalSI}
	}

	for _, container := range pod.Spec.Containers {
		if rQuantity, ok := container.Resources.Requests[resourceName]; ok {
			requestQuantity.Add(rQuantity)
		}
	}

	for _, container := range pod.Spec.InitContainers {
		if rQuantity, ok := container.Resources.Requests[resourceName]; ok {
			if requestQuantity.Cmp(rQuantity) < 0 {
				requestQuantity = rQuantity.DeepCopy()
			}
		}
	}

	// Don't add PodOverhead to the total requests
	//if pod.Spec.Overhead != nil {
	//	if podOverhead, ok := pod.Spec.Overhead[resourceName]; ok && !requestQuantity.IsZero() {
	//		requestQuantity.Add(podOverhead)
	//	}
	//}

	return requestQuantity
}

// GetResourceRequest finds and returns the request value for a specific resource.
func GetResourceRequest(pod *corev1.Pod, resource corev1.ResourceName) int64 {

	requestQuantity := GetResourceRequestQuantity(pod, resource)

	if resource == corev1.ResourceCPU {
		return requestQuantity.MilliValue()
	}

	return requestQuantity.Value()
}

// MergePodResourceRequirements merges enumerated requirements with default requirements
// it annotates the pod with information about what requirements were modified
func MergePodResourceRequirements(pod *corev1.Pod, defaultRequirements *corev1.ResourceRequirements) {
	annotations := []string{}

	for i := range pod.Spec.Containers {
		annotations = mergeContainerResources(&pod.Spec.Containers[i], defaultRequirements, "container", annotations)
	}

	for i := range pod.Spec.InitContainers {
		annotations = mergeContainerResources(&pod.Spec.InitContainers[i], defaultRequirements, "init container", annotations)
	}

	if len(annotations) > 0 {
		if pod.ObjectMeta.Annotations == nil {
			pod.ObjectMeta.Annotations = make(map[string]string)
		}
		val := strings.Join(annotations, "; ")
		pod.ObjectMeta.Annotations["kata.peerpods.io/changes"] = val
	}
}

// mergeContainerResources handles defaulting all of the resources on a container.
func mergeContainerResources(container *corev1.Container, defaultRequirements *corev1.ResourceRequirements, annotationPrefix string, annotations []string) []string {
	setRequests := []string{}
	setLimits := []string{}
	if container.Resources.Limits == nil {
		container.Resources.Limits = corev1.ResourceList{}
	}
	if container.Resources.Requests == nil {
		container.Resources.Requests = corev1.ResourceList{}
	}

	for k, v := range defaultRequirements.Limits {
		_, found := container.Resources.Limits[k]
		if !found {
			container.Resources.Limits[k] = v.DeepCopy()
			setLimits = append(setLimits, string(k))
		}
	}
	for k, v := range defaultRequirements.Requests {
		_, found := container.Resources.Requests[k]
		if !found {
			container.Resources.Requests[k] = v.DeepCopy()
			setRequests = append(setRequests, string(k))
		}
	}
	if len(setRequests) > 0 {
		sort.Strings(setRequests)
		a := strings.Join(setRequests, ", ") + fmt.Sprintf(" request for %s %s", annotationPrefix, container.Name)
		annotations = append(annotations, a)
	}
	if len(setLimits) > 0 {
		sort.Strings(setLimits)
		a := strings.Join(setLimits, ", ") + fmt.Sprintf(" limit for %s %s", annotationPrefix, container.Name)
		annotations = append(annotations, a)
	}
	return annotations
}
