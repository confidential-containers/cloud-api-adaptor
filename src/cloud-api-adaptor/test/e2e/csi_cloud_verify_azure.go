//go:build azure

// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	armcompute "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v4"

	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/test/e2e/cloudutil"
)

func init() {
	registerCloudDiskVerifier("azure", newAzureDiskVerifier)
}

type azureDiskVerifier struct {
	client        *armcompute.DisksClient
	resourceGroup string
}

func newAzureDiskVerifier() (CloudDiskVerifier, error) {
	subscriptionID := os.Getenv("AZURE_SUBSCRIPTION_ID")
	resourceGroup := os.Getenv("AZURE_RESOURCE_GROUP")
	if subscriptionID == "" || resourceGroup == "" {
		return nil, fmt.Errorf("AZURE_SUBSCRIPTION_ID and AZURE_RESOURCE_GROUP must be set for cloud-side verification")
	}
	cred, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		return nil, fmt.Errorf("creating Azure credential: %w", err)
	}
	client, err := armcompute.NewDisksClient(subscriptionID, cred, nil)
	if err != nil {
		return nil, fmt.Errorf("creating Azure disks client: %w", err)
	}
	return &azureDiskVerifier{
		client:        client,
		resourceGroup: resourceGroup,
	}, nil
}

func isAzureNotFound(err error) bool {
	var respErr *azcore.ResponseError
	if errors.As(err, &respErr) {
		return respErr.StatusCode == http.StatusNotFound
	}
	return false
}

func (v *azureDiskVerifier) resourceGroupForDisk(diskID string) string {
	if rg := cloudutil.ExtractAzureResourceGroup(diskID); rg != "" {
		return rg
	}
	return v.resourceGroup
}

func (v *azureDiskVerifier) DiskExists(ctx context.Context, diskID string) (bool, error) {
	diskName := cloudutil.ExtractAzureDiskName(diskID)
	rg := v.resourceGroupForDisk(diskID)
	_, err := v.client.Get(ctx, rg, diskName, nil)
	if err != nil {
		if isAzureNotFound(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (v *azureDiskVerifier) DiskState(ctx context.Context, diskID string) (string, error) {
	diskName := cloudutil.ExtractAzureDiskName(diskID)
	rg := v.resourceGroupForDisk(diskID)
	disk, err := v.client.Get(ctx, rg, diskName, nil)
	if err != nil {
		if isAzureNotFound(err) {
			return "deleted", nil
		}
		return "", err
	}
	if disk.Properties == nil || disk.Properties.DiskState == nil {
		return "unknown", nil
	}
	return string(*disk.Properties.DiskState), nil
}

