// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package provider

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-providers/util"
	"golang.org/x/crypto/ssh"
)

var logger = log.New(log.Writer(), "[adaptor/cloud] ", log.LstdFlags|log.Lmsgprefix)

// Method to verify the correct instanceType to be used for Pod VM
func VerifyCloudInstanceType(instanceType string, validInstanceTypes []string, defaultInstanceType string) (string, error) {
	// If instanceType is empty, set instanceType to default.
	if instanceType == "" {
		instanceType = defaultInstanceType
		logger.Printf("Using default instance type (%q)", defaultInstanceType)
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

// Method to sort InstanceTypeSpec into ascending order based on gpu, then memory, followed by cpu
func SortInstanceTypesOnResources(instanceTypeSpecList []InstanceTypeSpec) []InstanceTypeSpec {
	sort.Slice(instanceTypeSpecList, func(i, j int) bool {
		// First, sort by GPU count
		if instanceTypeSpecList[i].GPUs != instanceTypeSpecList[j].GPUs {
			return instanceTypeSpecList[i].GPUs < instanceTypeSpecList[j].GPUs
		}
		// If GPU count is the same, sort by memory
		if instanceTypeSpecList[i].Memory != instanceTypeSpecList[j].Memory {
			return instanceTypeSpecList[i].Memory < instanceTypeSpecList[j].Memory
		}
		// If memory is the same, sort by vCPUs
		return instanceTypeSpecList[i].VCPUs < instanceTypeSpecList[j].VCPUs
	})

	return instanceTypeSpecList
}

func SelectInstanceTypeToUse(spec InstanceTypeSpec, specList []InstanceTypeSpec, validInstanceTypes []string, defaultInstanceType string) (string, error) {

	var instanceType string
	var err error

	// spec.InstanceType gets the highest priority
	if spec.InstanceType != "" {
		instanceType = spec.InstanceType
		logger.Printf("Instance type selected by the cloud provider based on instance type annotation: %s", instanceType)
	} else if spec.GPUs > 0 {
		// If no explicit instance type, GPU gets the next priority
		instanceType, err = GetBestFitInstanceTypeWithGPU(specList, spec.GPUs, spec.VCPUs, spec.Memory)
		if err != nil {
			return "", fmt.Errorf("failed to get instance type based on GPU, vCPU, and memory annotations: %w", err)
		}
		logger.Printf("Instance type selected by the cloud provider based on GPU annotation: %s", instanceType)
	} else if spec.VCPUs != 0 && spec.Memory != 0 {
		// If no GPU is required, fall back to vCPU and memory selection
		instanceType, err = GetBestFitInstanceType(specList, spec.VCPUs, spec.Memory)
		if err != nil {
			return "", fmt.Errorf("failed to get instance type based on vCPU and memory annotations: %w", err)
		}
		logger.Printf("Instance type selected by the cloud provider based on vCPU and memory annotations: %s", instanceType)
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

	// Filter the out GPU instances from the list
	sortedInstanceTypeSpecList = FilterOutGPUInstances(sortedInstanceTypeSpecList)

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

// Filter out GPU instances from the instance type spec list
func FilterOutGPUInstances(instanceTypeSpecList []InstanceTypeSpec) []InstanceTypeSpec {
	var filteredList []InstanceTypeSpec
	for _, spec := range instanceTypeSpecList {
		if spec.GPUs == 0 {
			filteredList = append(filteredList, spec)
		}
	}
	return filteredList
}

// Implement the GetBestFitInstanceTypeWithGPU function
// TBD: Incorporate GPU model based selection as well
func GetBestFitInstanceTypeWithGPU(sortedInstanceTypeSpecList []InstanceTypeSpec, gpus, vcpus, memory int64) (string, error) {
	index := sort.Search(len(sortedInstanceTypeSpecList), func(i int) bool {
		return sortedInstanceTypeSpecList[i].GPUs >= gpus &&
			sortedInstanceTypeSpecList[i].VCPUs >= vcpus &&
			sortedInstanceTypeSpecList[i].Memory >= memory
	})

	if index == len(sortedInstanceTypeSpecList) {
		return "", fmt.Errorf("no instance type found for the given GPUs (%d), vCPUs (%d), and memory (%d)", gpus, vcpus, memory)
	}

	return sortedInstanceTypeSpecList[index].InstanceType, nil
}

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

// FlagRegistrar wraps a FlagSet to provide registration methods with environment variable support.
type FlagRegistrar struct {
	flags *flag.FlagSet
}

// NewFlagRegistrar creates a new FlagRegistrar for the given FlagSet.
func NewFlagRegistrar(flags *flag.FlagSet) *FlagRegistrar {
	return &FlagRegistrar{flags: flags}
}

// StringWithEnv registers a string flag with environment variable support.
func (r *FlagRegistrar) StringWithEnv(field *string, flagName, hardcodedDefault, envVarName, usage string) {
	*field = hardcodedDefault

	if envVarName != "" {
		if envValue, exists := os.LookupEnv(envVarName); exists {
			*field = envValue
		}
	}

	r.flags.StringVar(field, flagName, *field, usage)
}

// IntWithEnv registers an int flag with environment variable support.
func (r *FlagRegistrar) IntWithEnv(field *int, flagName string, hardcodedDefault int, envVarName, usage string) {
	*field = hardcodedDefault

	if envVarName != "" {
		if envValue, exists := os.LookupEnv(envVarName); exists {
			var intVal int
			if n, err := fmt.Sscanf(envValue, "%d", &intVal); err == nil && n == 1 {
				*field = intVal
			}
		}
	}

	r.flags.IntVar(field, flagName, *field, usage)
}

// UintWithEnv registers a uint flag with environment variable support.
func (r *FlagRegistrar) UintWithEnv(field *uint, flagName string, hardcodedDefault uint, envVarName, usage string) {
	*field = hardcodedDefault

	if envVarName != "" {
		if envValue, exists := os.LookupEnv(envVarName); exists {
			if uintVal, err := strconv.ParseUint(envValue, 10, 32); err == nil {
				*field = uint(uintVal)
			}
		}
	}

	r.flags.UintVar(field, flagName, *field, usage)
}

// BoolWithEnv registers a bool flag with environment variable support.
// Accepts: "1" or "true" (case-insensitive) for true, anything else for false.
func (r *FlagRegistrar) BoolWithEnv(field *bool, flagName string, hardcodedDefault bool, envVarName, usage string) {
	*field = hardcodedDefault

	if envVarName != "" {
		if envValue, exists := os.LookupEnv(envVarName); exists {
			lowerValue := strings.ToLower(envValue)
			*field = (lowerValue == "1" || lowerValue == "true")
		}
	}

	r.flags.BoolVar(field, flagName, *field, usage)
}

// CustomTypeWithEnv registers a custom flag type (like comma-separated lists or key-value maps) with environment variable support.
// The field must implement flag.Value interface.
func (r *FlagRegistrar) CustomTypeWithEnv(field flag.Value, flagName, hardcodedDefault, envVarName, usage string) {
	// Check environment variable first
	if envVarName != "" {
		if envValue, exists := os.LookupEnv(envVarName); exists {
			_ = field.Set(envValue)
			r.flags.Var(field, flagName, usage)
			return
		}
	}

	// Use hardcoded default if env var doesn't exist
	if hardcodedDefault != "" {
		_ = field.Set(hardcodedDefault)
	}

	r.flags.Var(field, flagName, usage)
}

// Method to write userdata to a file

func WriteUserData(instanceName string, userData string, dataDir string) (string, error) {
	// Write userdata to a file named after the instance name in the dataDir directory
	// File name: $dataDir/${instanceName}-userdata
	// File content: userdata

	// Check if the dataDir directory exists
	// If it does not exist, create it
	err := os.MkdirAll(dataDir, 0755)
	if err != nil {
		return "", err
	}

	// Create file path
	filePath := filepath.Join(dataDir, instanceName+"-userdata")

	// Write userdata to a file in the temp directory
	err = os.WriteFile(filePath, []byte(userData), 0644)
	if err != nil {
		return "", err
	}

	// Write userdata to a file in the temp directory
	// Return the file path
	return filePath, nil
}

// Verify SSH public key file
// Check the permissions and the content of the file to ensure it is a valid SSH public key
func VerifySSHKeyFile(sshKeyFile string) error {

	// Check if the file exists
	_, err := os.Stat(sshKeyFile)
	if err != nil {
		return fmt.Errorf("failed to verify SSH key file: %w", err)
	}

	// Check the permissions of the file
	fileInfo, err := os.Stat(sshKeyFile)
	if err != nil {
		return fmt.Errorf("failed to verify SSH key file: %w", err)
	}

	// Check if the file permissions are exactly 600
	if fileInfo.Mode().Perm() != 0600 {
		return fmt.Errorf("SSH key file permissions are not 600: %s", sshKeyFile)
	}

	// Read the content of the file
	content, err := os.ReadFile(sshKeyFile)
	if err != nil {
		return fmt.Errorf("failed to read SSH key file: %w", err)
	}

	// Check if the content is a valid SSH public key
	_, _, _, _, err = ssh.ParseAuthorizedKey(content)
	if err != nil {
		return fmt.Errorf("invalid SSH public key: %w", err)
	}

	return nil
}
