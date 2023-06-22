// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package cloud

import (
	"fmt"
	"os"
	"sort"

	"github.com/confidential-containers/cloud-api-adaptor/pkg/util"
)

func DefaultToEnv(field *string, env, fallback string) {

	if *field != "" {
		return
	}

	val := os.Getenv(env)
	if val == "" {
		val = fallback
	}

	*field = val
}

// Method to verify the correct instanceType to be used for Pod VM
func VerifyCloudInstanceType(instanceType string, validInstanceTypes []string, defaultInstanceType string) (string, error) {
	// If instanceType is empty, set instanceType to default.
	if instanceType == "" {
		instanceType = defaultInstanceType
		fmt.Printf("Using default instance type (%q)\n", defaultInstanceType)
		return instanceType, nil
	}

	// If instanceTypes is empty and instanceType is not default, return error
	if len(validInstanceTypes) == 0 && instanceType != defaultInstanceType {
		// Return error if instanceTypes is empty and instanceType is not default
		return "", fmt.Errorf("requested instance type (%q) is not default (%q) and supported instance types list is empty",
			instanceType, defaultInstanceType)

	}

	// If instanceTypes is not empty and instanceType is not among the supported instance types, return error
	if len(validInstanceTypes) > 0 && !util.Contains(validInstanceTypes, instanceType) {
		return "", fmt.Errorf("requested instance type (%q) is not part of supported instance types list", instanceType)
	}

	return instanceType, nil
}

// Method to sort InstanceTypeSpec into ascending order based on memory
func SortInstanceTypesOnMemory(instanceTypeSpecList []InstanceTypeSpec) []InstanceTypeSpec {

	// Use sort.Slice to sort the instanceTypeTupleList slice and return the sorted slice
	sort.Slice(instanceTypeSpecList, func(i, j int) bool {
		return instanceTypeSpecList[i].Memory < instanceTypeSpecList[j].Memory
	})

	return instanceTypeSpecList
}

func SelectInstanceTypeToUse(spec InstanceTypeSpec, specList []InstanceTypeSpec, validInstanceTypes []string, defaultInstanceType string) (string, error) {

	var instanceType string
	var err error

	// If vCPU and memory are set in annotations then find the best fit instance type
	// from the cloud provider
	// vCPU and Memory gets higher priority than instance type from annotation
	if spec.VCPUs != 0 && spec.Memory != 0 {
		instanceType, err = GetBestFitInstanceType(specList, spec.VCPUs, spec.Memory)
		if err != nil {
			return "", fmt.Errorf("failed to get instance type based on vCPU and memory annotations: %w", err)
		}
		logger.Printf("Instance type selected by the cloud provider based on vCPU and memory annotations: %s", instanceType)
	} else if spec.InstanceType != "" {
		instanceType = spec.InstanceType
		logger.Printf("Instance type selected by the cloud provider based on instance type annotation: %s", instanceType)
	}

	// Verify the instance type selected via the annotations
	// If instance type is set in annotations then use that instance type
	instanceTypeToUse, err := VerifyCloudInstanceType(instanceType, validInstanceTypes, defaultInstanceType)
	if err != nil {
		return "", fmt.Errorf("failed to verify instance type: %w", err)
	}

	return instanceTypeToUse, nil

}

// Method to find the best fit instance type for the given memory and vcpus
// The sortedInstanceTypeSpecList slice is a sorted list of instance types based on ascending order of supported memory
func GetBestFitInstanceType(sortedInstanceTypeSpecList []InstanceTypeSpec, vcpus int64, memory int64) (string, error) {

	// Use sort.Search to find the index of the first element in the sortedMachineTypeList slice
	// that is greater than or equal to the given memory and vcpus
	index := sort.Search(len(sortedInstanceTypeSpecList), func(i int) bool {
		return sortedInstanceTypeSpecList[i].Memory >= memory && sortedInstanceTypeSpecList[i].VCPUs >= vcpus
	})

	// If binary search fails to find a match, return error
	if index == len(sortedInstanceTypeSpecList) {
		return "", fmt.Errorf("no instance type found for the given vcpus (%d) and memory (%d)", vcpus, memory)
	}

	// If binary search finds a match, return the instance type
	return sortedInstanceTypeSpecList[index].InstanceType, nil

}
