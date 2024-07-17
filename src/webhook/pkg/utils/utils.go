package utils

import (
	"strconv"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/cloud-provider/volume/helpers"
)

// GetResourceRequestQuantity finds and returns the request quantity for a specific resource.
func GetResourceRequestQuantity(pod *corev1.Pod, resourceName corev1.ResourceName) resource.Quantity {

	requestQuantity := getResourceQuantity(resourceName)
	// Add the request quantity for each container
	for _, container := range pod.Spec.Containers {
		if rQuantity, ok := container.Resources.Requests[resourceName]; ok {
			requestQuantity.Add(rQuantity)
		}
	}

	// Add the request quantity for each init container
	for _, container := range pod.Spec.InitContainers {
		if rQuantity, ok := container.Resources.Requests[resourceName]; ok {
			requestQuantity.Add(rQuantity)
		}
	}

	// Don't add PodOverhead to the total requests
	// as its not needed for peer-pod since the VM is external to the worker

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

// GetResourceLimitQuantity finds and returns the limit quantity for a specific resource.
func GetResourceLimitQuantity(pod *corev1.Pod, resourceName corev1.ResourceName) resource.Quantity {

	limitQuantity := getResourceQuantity(resourceName)

	for _, container := range pod.Spec.Containers {
		if lQuantity, ok := container.Resources.Limits[resourceName]; ok {
			limitQuantity.Add(lQuantity)
		}
	}

	for _, container := range pod.Spec.InitContainers {
		if lQuantity, ok := container.Resources.Limits[resourceName]; ok {
			limitQuantity.Add(lQuantity)

		}
	}
	return limitQuantity
}

// Method to get the resource Quantity from the resource name
func getResourceQuantity(resourceName corev1.ResourceName) resource.Quantity {

	switch resourceName {
	case corev1.ResourceMemory:
		return resource.Quantity{Format: resource.BinarySI}
	default:
		return resource.Quantity{Format: resource.DecimalSI}
	}
}

// Method to convert memory quantity to MiB string
func ConvertMemoryQuantityToMib(memoryQuantity resource.Quantity) (string, error) {

	memoryQuantityMib, err := helpers.RoundUpToMiB(memoryQuantity)
	if err != nil {
		return "0", err
	}
	return strconv.FormatInt(memoryQuantityMib, 10), nil
}
