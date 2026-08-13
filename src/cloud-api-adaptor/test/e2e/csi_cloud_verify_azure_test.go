//go:build azure

// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package e2e

import "testing"

func TestExtractAzureResourceGroup(t *testing.T) {
	tests := []struct {
		name   string
		diskID string
		want   string
	}{
		{
			name:   "full ARM path with MC resource group",
			diskID: "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/MC_mygroup/providers/Microsoft.Compute/disks/mydisk",
			want:   "MC_mygroup",
		},
		{
			name:   "lowercase resourcegroups in path",
			diskID: "/subscriptions/00000000-0000-0000-0000-000000000000/resourcegroups/lowercase-rg/providers/Microsoft.Compute/disks/mydisk",
			want:   "lowercase-rg",
		},
		{
			name:   "mixed case ResourceGroups",
			diskID: "/subscriptions/00000000-0000-0000-0000-000000000000/ResourceGroups/MixedCase-RG/providers/Microsoft.Compute/disks/mydisk",
			want:   "MixedCase-RG",
		},
		{
			name:   "plain disk name returns empty",
			diskID: "mydisk",
			want:   "",
		},
		{
			name:   "empty string returns empty",
			diskID: "",
			want:   "",
		},
		{
			name:   "path without resourceGroups segment",
			diskID: "/subscriptions/00000000-0000-0000-0000-000000000000/providers/Microsoft.Compute/disks/mydisk",
			want:   "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractAzureResourceGroup(tt.diskID)
			if got != tt.want {
				t.Errorf("extractAzureResourceGroup(%q) = %q, want %q", tt.diskID, got, tt.want)
			}
		})
	}
}

func TestExtractAzureDiskName(t *testing.T) {
	tests := []struct {
		name   string
		diskID string
		want   string
	}{
		{
			name:   "full ARM resource ID",
			diskID: "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/MC_mygroup/providers/Microsoft.Compute/disks/mydisk",
			want:   "mydisk",
		},
		{
			name:   "plain disk name",
			diskID: "mydisk",
			want:   "mydisk",
		},
		{
			name:   "empty string",
			diskID: "",
			want:   "",
		},
		{
			name:   "disk name with hyphens",
			diskID: "/subscriptions/sub-id/resourceGroups/rg/providers/Microsoft.Compute/disks/pvc-abc-123-def",
			want:   "pvc-abc-123-def",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractAzureDiskName(tt.diskID)
			if got != tt.want {
				t.Errorf("extractAzureDiskName(%q) = %q, want %q", tt.diskID, got, tt.want)
			}
		})
	}
}
