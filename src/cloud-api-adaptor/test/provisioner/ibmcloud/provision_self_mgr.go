// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package ibmcloud

import (
	"context"

	"sigs.k8s.io/e2e-framework/pkg/envconf"
)

// IBMSelfManagedClusterProvisioner implements the CloudProvisioner interface for self-managed k8s cluster in ibmcloud VSI.
type IBMSelfManagedClusterProvisioner struct {
}

func (p *IBMSelfManagedClusterProvisioner) CreateCluster(ctx context.Context, cfg *envconf.Config) error {
	return nil
}

func (p *IBMSelfManagedClusterProvisioner) CreateVPC(ctx context.Context, cfg *envconf.Config) error {
	return createVpcImpl()
}

func (p *IBMSelfManagedClusterProvisioner) DeleteCluster(ctx context.Context, cfg *envconf.Config) error {
	return nil
}

func (p *IBMSelfManagedClusterProvisioner) DeleteVPC(ctx context.Context, cfg *envconf.Config) error {
	return deleteVpcImpl()
}

func (p *IBMSelfManagedClusterProvisioner) UploadPodvm(imagePath string, ctx context.Context, cfg *envconf.Config) error {
	return nil
}

func (p *IBMSelfManagedClusterProvisioner) GetProvisionValues() map[string]interface{} {
	// TODO: implement properly
	return nil
}

func (p *IBMSelfManagedClusterProvisioner) GetProperties(ctx context.Context, cfg *envconf.Config) map[string]string {
	return make(map[string]string)
}
