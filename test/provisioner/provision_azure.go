//go:build azure

// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package provisioner

import (
	"context"

	"sigs.k8s.io/e2e-framework/pkg/envconf"
)

func init() {
	newProvisionerFunctions["azure"] = NewAzureCloudProvisioner
}

// AzureCloudProvisioner implements the CloudProvisioner interface for Azure.
type AzureCloudProvisioner struct {
}

func NewAzureCloudProvisioner(properties map[string]string) (CloudProvisioner, error) {
	return &AzureCloudProvisioner{}, nil
}

func (l *AzureCloudProvisioner) CreateCluster(ctx context.Context, cfg *envconf.Config) error {
	return nil
}

func (l *AzureCloudProvisioner) CreateVPC(ctx context.Context, cfg *envconf.Config) error {
	return nil
}

func (l *AzureCloudProvisioner) DeleteCluster(ctx context.Context, cfg *envconf.Config) error {
	return nil
}

func (l *AzureCloudProvisioner) DeleteVPC(ctx context.Context, cfg *envconf.Config) error {
	return nil
}

func (l *AzureCloudProvisioner) UploadPodvm(imagePath string, ctx context.Context, cfg *envconf.Config) error {
	return nil
}
