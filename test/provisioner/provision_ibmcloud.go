//go:build ibmcloud

// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package provisioner

import (
	"context"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
)

// IBMCloudProvisioner implements the CloudProvisioner interface for ibmcloud.
type IBMCloudProvisioner struct {
}

func NewIBMCloudProvisioner(network string, storage string) (*IBMCloudProvisioner, error) {
	return &IBMCloudProvisioner{}, nil
}

func (l *IBMCloudProvisioner) CreateCluster(ctx context.Context, cfg *envconf.Config) error {
	return nil
}

func (l *IBMCloudProvisioner) CreateVPC(ctx context.Context, cfg *envconf.Config) error {
	return nil
}

func (l *IBMCloudProvisioner) DeleteCluster(ctx context.Context, cfg *envconf.Config) error {
	return nil
}

func (l *IBMCloudProvisioner) DeleteVPC(ctx context.Context, cfg *envconf.Config) error {
	return nil
}

func (l *IBMCloudProvisioner) UploadPodvm(imagePath string, ctx context.Context, cfg *envconf.Config) error {
	return nil
}
