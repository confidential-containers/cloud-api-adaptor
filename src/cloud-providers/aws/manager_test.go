// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package aws

import (
	"flag"
	"fmt"
	"testing"
)

func TestManager_ParseCmd(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		expected Config
	}{
		{
			name: "AllFlagsSet",
			args: []string{
				"-aws-access-key-id=test-access-key",
				"-aws-secret-key=test-secret-key",
				"-aws-region=test-region",
				"-aws-profile=test-profile",
				"-aws-lt-name=test-lt-name",
				"-use-lt=true",
				"-imageid=test-image-id",
				"-instance-type=test-instance-type",
				"-securitygroupids=sg-1,sg-2",
				"-keyname=test-key-name",
				"-subnetid=test-subnet-id",
				"-use-public-ip=true",
				"-instance-types=t2.micro,t3.small",
				"-tags=key1=value1,key2=value2",
				"-root-volume-size=60",
				"-disable-cvm=false",
			},
			expected: Config{
				AccessKeyID:        "test-access-key",
				SecretKey:          "test-secret-key",
				Region:             "test-region",
				LoginProfile:       "test-profile",
				LaunchTemplateName: "test-lt-name",
				UseLaunchTemplate:  true,
				ImageID:            "test-image-id",
				InstanceType:       "test-instance-type",
				SecurityGroupIds:   []string{"sg-1", "sg-2"},
				KeyName:            "test-key-name",
				SubnetID:           "test-subnet-id",
				InstanceTypes:      []string{"t2.micro", "t3.small"},
				Tags:               map[string]string{"key1": "value1", "key2": "value2"},
				UsePublicIP:        true,
				RootVolumeSize:     60,
				DisableCVM:         false,
			},
		},
		{
			name: "DefaultValues",
			args: []string{},
			expected: Config{
				AccessKeyID:        "",
				SecretKey:          "",
				Region:             "",
				LoginProfile:       "",
				LaunchTemplateName: "kata",
				UseLaunchTemplate:  false,
				ImageID:            "",
				InstanceType:       "m6a.large",
				SecurityGroupIds:   securityGroupIds{},
				KeyName:            "",
				SubnetID:           "",
				InstanceTypes:      instanceTypes{},
				Tags:               nil,
				UsePublicIP:        false,
				RootVolumeSize:     30,
				DisableCVM:         false,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Create a new flag set
			flags := flag.NewFlagSet("test", flag.ContinueOnError)

			// Create a new Manager instance
			manager := &Manager{}

			// Parse the command-line flags
			manager.ParseCmd(flags)

			// Set the command-line arguments
			err := flags.Parse(test.args)
			if err != nil {
				t.Errorf("Failed to parse flags: %v", err)
			}

			// Compare the expected and actual values using comparestructs
			if !comparestructs(test.expected, awscfg) {
				t.Errorf("Expected config: %+v, but got: %+v", test.expected, awscfg)
			}

			// Delete the flag set
			flags = nil

			// Reset the awscfg
			awscfg = Config{}
		})
	}
}

// Add a comparestructs function to compare the expected and actual values of the Config struct
// This is needed because the reflect.DeepEqual() function does not work with maps
// Print the expected and actual values to the console if they do not match
func comparestructs(expected, actual Config) bool {
	if expected.AccessKeyID != actual.AccessKeyID {
		// Print the expected and actual values to the console if they do not match
		fmt.Printf("Expected AccessKeyId: %s, but got: %s\n", expected.AccessKeyID, actual.AccessKeyID)
		return false
	}
	if expected.SecretKey != actual.SecretKey {
		// Print the expected and actual values to the console if they do not match
		fmt.Printf("Expected SecretKey: %s, but got: %s\n", expected.SecretKey, actual.SecretKey)
		return false
	}
	if expected.Region != actual.Region {
		// Print the expected and actual values to the console if they do not match
		fmt.Printf("Expected Region: %s, but got: %s\n", expected.Region, actual.Region)
		return false
	}
	if expected.LoginProfile != actual.LoginProfile {
		// Print the expected and actual values to the console if they do not match
		fmt.Printf("Expected LoginProfile: %s, but got: %s\n", expected.LoginProfile, actual.LoginProfile)
		return false
	}
	if expected.LaunchTemplateName != actual.LaunchTemplateName {
		// Print the expected and actual values to the console if they do not match
		fmt.Printf("Expected LaunchTemplateName: %s, but got: %s\n", expected.LaunchTemplateName, actual.LaunchTemplateName)
		return false
	}
	if expected.UseLaunchTemplate != actual.UseLaunchTemplate {
		// Print the expected and actual values to the console if they do not match
		fmt.Printf("Expected UseLaunchTemplate: %t, but got: %t\n", expected.UseLaunchTemplate, actual.UseLaunchTemplate)
		return false

	}
	if expected.ImageID != actual.ImageID {
		// Print the expected and actual values to the console if they do not match
		fmt.Printf("Expected ImageId: %s, but got: %s\n", expected.ImageID, actual.ImageID)
		return false
	}
	if expected.InstanceType != actual.InstanceType {
		// Print the expected and actual values to the console if they do not match
		fmt.Printf("Expected InstanceType: %s, but got: %s\n", expected.InstanceType, actual.InstanceType)
		return false
	}

	// DeepEqual() does not work with slices
	fmt.Printf("sg %v\n", actual.SecurityGroupIds)

	// Compare the length of the expected and actual values of the SecurityGroupIds slice.
	if len(expected.SecurityGroupIds) != len(actual.SecurityGroupIds) {
		// Print the expected and actual values to the console if they do not match
		fmt.Printf("Expected length of SecurityGroupIds: %s, but got: %s\n", expected.SecurityGroupIds, actual.SecurityGroupIds)
		return false
	}

	// Compare the expected and actual values of the SecurityGroupIds slice.
	for i := range expected.SecurityGroupIds {
		if expected.SecurityGroupIds[i] != actual.SecurityGroupIds[i] {
			// Print the expected and actual values to the console if they do not match
			fmt.Printf("Expected SecurityGroupIds: %s, but got: %s\n", expected.SecurityGroupIds, actual.SecurityGroupIds)
			return false
		}
	}

	if expected.KeyName != actual.KeyName {
		// Print the expected and actual values to the console if they do not match
		fmt.Printf("Expected KeyName: %s, but got: %s\n", expected.KeyName, actual.KeyName)
		return false
	}
	if expected.SubnetID != actual.SubnetID {
		// Print the expected and actual values to the console if they do not match
		fmt.Printf("Expected SubnetId: %s, but got: %s\n", expected.SubnetID, actual.SubnetID)
		return false
	}
	if expected.UsePublicIP != actual.UsePublicIP {
		// Print the expected and actual values to the console if they do not match
		fmt.Printf("Expected UsePublicIP: %t, but got: %t\n", expected.UsePublicIP, actual.UsePublicIP)
		return false
	}

	// DeepEqual() does not work with slices
	// Compare the length of the expected and actual values of the InstanceTypes slice.
	if len(expected.InstanceTypes) != len(actual.InstanceTypes) {
		// Print the expected and actual values to the console if they do not match
		fmt.Printf("Expected length of InstanceTypes: %s, but got: %s\n", expected.InstanceTypes, actual.InstanceTypes)
		return false
	}

	// Compare the expected and actual values of the InstanceTypes slice.
	for i := range expected.InstanceTypes {
		if expected.InstanceTypes[i] != actual.InstanceTypes[i] {
			// Print the expected and actual values to the console if they do not match
			fmt.Printf("Expected InstanceTypes: %s, but got: %s\n", expected.InstanceTypes, actual.InstanceTypes)
			return false
		}
	}

	// DeepEqual() does not work with maps
	// Compare the length of the expected and actual values of the Tags map.
	if len(expected.Tags) != len(actual.Tags) {
		// Print the expected and actual values to the console if they do not match
		fmt.Printf("Expected length of Tags: %s, but got: %s\n", expected.Tags, actual.Tags)
		return false
	}
	for key, value := range expected.Tags {
		if actualValue, ok := actual.Tags[key]; !ok || actualValue != value {
			// Print the expected and actual values to the console if they do not match
			fmt.Printf("Expected Tags: %s, but got: %s\n", expected.Tags, actual.Tags)
			return false
		}
	}

	if expected.RootVolumeSize != actual.RootVolumeSize {
		// Print the expected and actual values to the console if they do not match
		fmt.Printf("Expected RootVolumeSize: %d, but got: %d\n", expected.RootVolumeSize, actual.RootVolumeSize)
		return false
	}

	if expected.DisableCVM != actual.DisableCVM {
		// Print the expected and actual values to the console if they do not match
		fmt.Printf("Expected DisableCVM: %t, but got: %t\n", expected.DisableCVM, actual.DisableCVM)
		return false
	}

	return true
}
