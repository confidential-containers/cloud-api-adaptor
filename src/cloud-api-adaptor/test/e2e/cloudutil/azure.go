// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package cloudutil

import "strings"

// ExtractAzureResourceGroup extracts the resource group from a full ARM resource ID.
// Returns empty string if the input is not a full ARM path.
// Example: /subscriptions/.../resourceGroups/MC_mygroup/providers/Microsoft.Compute/disks/mydisk
func ExtractAzureResourceGroup(diskID string) string {
	const rgSegment = "/resourcegroups/"
	lower := strings.ToLower(diskID)
	idx := strings.Index(lower, rgSegment)
	if idx == -1 {
		return ""
	}
	rest := diskID[idx+len(rgSegment):]
	if slashIdx := strings.Index(rest, "/"); slashIdx != -1 {
		return rest[:slashIdx]
	}
	return rest
}

// ExtractAzureDiskName extracts the disk name from a full Azure resource ID
// or returns the input as-is if it's already just a name.
func ExtractAzureDiskName(diskID string) string {
	parts := strings.Split(diskID, "/")
	if len(parts) > 1 {
		return parts[len(parts)-1]
	}
	return diskID
}
