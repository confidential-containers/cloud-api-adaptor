// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package azure

import (
	"context"

	"sigs.k8s.io/e2e-framework/pkg/envconf"
)

// AzureSelfManagedClusterProvisioner implements the CloudProvisioner interface for self-managed k8s cluster in azure cloud.
type AzureSelfManagedClusterProvisioner struct {
}

func (p *AzureSelfManagedClusterProvisioner) CreateCluster(ctx context.Context, cfg *envconf.Config) error {
	return nil
}

func (p *AzureSelfManagedClusterProvisioner) CreateVPC(ctx context.Context, cfg *envconf.Config) error {
	return createResourceImpl()
}

func (p *AzureSelfManagedClusterProvisioner) DeleteCluster(ctx context.Context, cfg *envconf.Config) error {
	return nil
}

func (p *AzureSelfManagedClusterProvisioner) DeleteVPC(ctx context.Context, cfg *envconf.Config) error {
	return deleteResourceImpl()
}

func (p *AzureSelfManagedClusterProvisioner) UploadPodvm(imagePath string, ctx context.Context, cfg *envconf.Config) error {
	return nil
}

func (p *AzureSelfManagedClusterProvisioner) GetProvisionValues() map[string]interface{} {
	// TODO: implement properly
	return nil
}

func (p *AzureSelfManagedClusterProvisioner) GetProperties(ctx context.Context, cfg *envconf.Config) map[string]string {
	return getPropertiesImpl()
}
