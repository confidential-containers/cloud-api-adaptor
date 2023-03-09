//go:build ibmcloud

// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package provisioner

import (
	"context"

	"sigs.k8s.io/e2e-framework/pkg/envconf"
)

// SelfManagedClusterProvisioner implements the CloudProvisioner interface for self-managed k8s cluster in ibmcloud VSI.
type SelfManagedClusterProvisioner struct {
}

func (p *SelfManagedClusterProvisioner) CreateCluster(ctx context.Context, cfg *envconf.Config) error {
	return nil
}

func (p *SelfManagedClusterProvisioner) CreateVPC(ctx context.Context, cfg *envconf.Config) error {
	return createVpcImpl()
}

func (p *SelfManagedClusterProvisioner) DeleteCluster(ctx context.Context, cfg *envconf.Config) error {
	return nil
}

func (p *SelfManagedClusterProvisioner) DeleteVPC(ctx context.Context, cfg *envconf.Config) error {
	return deleteVpcImpl()
}

func (p *SelfManagedClusterProvisioner) UploadPodvm(imagePath string, ctx context.Context, cfg *envconf.Config) error {
	return nil
}
