//go:build azure

// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package provisioner

import (
	"context"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
)

// AzureCloudProvisioner implements the CloudProvisioner interface for ibmcloud.
type AzureCloudProvisioner struct {
}

func NewAzureCloudProvisioner(network string, storage string) (*AzureCloudProvisioner, error) {
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
