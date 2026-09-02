// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package ibmcloudpowervs

import (
	"fmt"
	"os"
	"strings"
)

func newIBMCloudPowerVSProvisioner(properties map[string]string) (*IBMCloudPowerVSProvisioner, error) {
	needProvisionStr := os.Getenv("TEST_PROVISION")
	if strings.EqualFold(needProvisionStr, "yes") || strings.EqualFold(needProvisionStr, "true") {
		required := []string{
			"IBMCLOUD_API_KEY",
			"POWERVS_ZONE",
			"POWERVS_SERVICE_INSTANCE_ID",
			"POWERVS_IMAGE_ID",
			"POWERVS_NETWORK_ID",
			"POWERVS_SSH_KEY_NAME",
		}
		for _, key := range required {
			if properties[key] == "" {
				return nil, fmt.Errorf("required property %s is not set", key)
			}
		}
	}

	return &IBMCloudPowerVSProvisioner{
		IBMCloudPowerVSAPIKey:    properties["IBMCLOUD_API_KEY"],
		PowerVSZone:              properties["POWERVS_ZONE"],
		PowerVSServiceInstanceID: properties["POWERVS_SERVICE_INSTANCE_ID"],
		PowerVSImageID:           properties["POWERVS_IMAGE_ID"],
		PowerVSNetworkID:         properties["POWERVS_NETWORK_ID"],
		PowerVSSSHKeyName:        properties["POWERVS_SSH_KEY_NAME"],
		PowerVSProcessorType:     properties["POWERVS_PROCESSOR_TYPE"],
		PowerVSSystemType:        properties["POWERVS_SYSTEM_TYPE"],
		PowerVSMemory:            properties["POWERVS_MEMORY"],
		PowerVSProcessors:        properties["POWERVS_PROCESSORS"],
		ForwarderPort:            properties["FORWARDER_PORT"],
		ProxyTimeout:             properties["PROXY_TIMEOUT"],
		UsePublicIP:              properties["USE_PUBLIC_IP"],
	}, nil
}
