// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package azure

import (
	"math"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	armcompute "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v4"
	provider "github.com/confidential-containers/cloud-api-adaptor/src/cloud-providers"
)

func buildDataDisks(volumes []provider.CloudVolume) ([]*armcompute.DataDisk, error) {
	var dataDisks []*armcompute.DataDisk
	for i, vol := range volumes {
		if i > math.MaxInt32 {
			return nil, nil
		}
		dataDisks = append(dataDisks, &armcompute.DataDisk{
			Lun:          to.Ptr(int32(i)),
			CreateOption: to.Ptr(armcompute.DiskCreateOptionTypesAttach),
			DeleteOption: to.Ptr(armcompute.DiskDeleteOptionTypesDetach),
			ManagedDisk: &armcompute.ManagedDiskParameters{
				ID: to.Ptr(vol.DiskID),
			},
		})
	}
	return dataDisks, nil
}

func TestBuildDataDisks(t *testing.T) {
	t.Run("empty volumes produces nil slice", func(t *testing.T) {
		disks, err := buildDataDisks(nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(disks) != 0 {
			t.Errorf("expected 0 disks, got %d", len(disks))
		}
	})

	t.Run("single volume produces one data disk at LUN 0", func(t *testing.T) {
		vols := []provider.CloudVolume{
			{DiskID: "/subscriptions/sub-1/resourceGroups/rg/providers/Microsoft.Compute/disks/disk-1"},
		}
		disks, err := buildDataDisks(vols)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(disks) != 1 {
			t.Fatalf("expected 1 disk, got %d", len(disks))
		}
		if *disks[0].Lun != 0 {
			t.Errorf("expected LUN 0, got %d", *disks[0].Lun)
		}
		if *disks[0].ManagedDisk.ID != vols[0].DiskID {
			t.Errorf("expected disk ID %q, got %q", vols[0].DiskID, *disks[0].ManagedDisk.ID)
		}
		if *disks[0].CreateOption != armcompute.DiskCreateOptionTypesAttach {
			t.Errorf("expected Attach create option")
		}
		if *disks[0].DeleteOption != armcompute.DiskDeleteOptionTypesDetach {
			t.Errorf("expected Detach delete option")
		}
	})

	t.Run("multiple volumes get sequential LUNs", func(t *testing.T) {
		vols := []provider.CloudVolume{
			{DiskID: "/subscriptions/sub-1/resourceGroups/rg/providers/Microsoft.Compute/disks/disk-1"},
			{DiskID: "/subscriptions/sub-1/resourceGroups/rg/providers/Microsoft.Compute/disks/disk-2"},
			{DiskID: "/subscriptions/sub-1/resourceGroups/rg/providers/Microsoft.Compute/disks/disk-3"},
		}
		disks, err := buildDataDisks(vols)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(disks) != 3 {
			t.Fatalf("expected 3 disks, got %d", len(disks))
		}
		for i, d := range disks {
			if *d.Lun != int32(i) {
				t.Errorf("disk %d: expected LUN %d, got %d", i, i, *d.Lun)
			}
			if *d.ManagedDisk.ID != vols[i].DiskID {
				t.Errorf("disk %d: expected ID %q, got %q", i, vols[i].DiskID, *d.ManagedDisk.ID)
			}
		}
	})
}
