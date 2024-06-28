// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package gcp

import (
	"context"
	"fmt"
	"google.golang.org/api/option"

	pv "github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/test/provisioner"
	// log "github.com/sirupsen/logrus"
	"google.golang.org/api/compute/v1"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
)

var GCPProps = &GCPProvisioner{}

// GCPProvisioner implements the CloudProvisioner interface.
type GCPProvisioner struct {
	GkeCluster   *GKECluster
	GcpVPC       *GCPVPC
	PodvmImage   *GCPImage
	CaaImageName string
}

// NewGCPProvisioner creates a new GCPProvisioner with the given properties.
func NewGCPProvisioner(properties map[string]string) (pv.CloudProvisioner, error) {
	ctx := context.Background()

	srv, err := compute.NewService(
		ctx, option.WithCredentialsFile(properties["credentials"]))
	if err != nil {
		return nil, fmt.Errorf("GCP: compute.NewService: %v", err)
	}

	gkeCluster, err := NewGKECluster(properties)
	if err != nil {
		return nil, err
	}

	gcpVPC, err := NewGCPVPC(properties)
	if err != nil {
		return nil, err
	}

	// TODO: Move to Overlay?
	image, err := NewGCPImage(srv, properties["podvm_image_name"])
	if err != nil {
		return nil, err
	}

	GCPProps = &GCPProvisioner{
		GkeCluster:   gkeCluster,
		GcpVPC:       gcpVPC,
		PodvmImage:   image,
		CaaImageName: properties["caa_image_name"],
	}
	return GCPProps, nil
}

// CreateCluster creates a new GKE cluster.
func (p *GCPProvisioner) CreateCluster(ctx context.Context, cfg *envconf.Config) error {
	err := p.GkeCluster.CreateCluster(ctx)
	if err != nil {
		return err
	}

	kubeconfigPath, err := p.GkeCluster.GetKubeconfigFile(ctx)
	if err != nil {
		return err
	}
	*cfg = *envconf.NewWithKubeConfig(kubeconfigPath)

	return nil
}

// CreateVPC creates a new VPC in Google Cloud.
func (p *GCPProvisioner) CreateVPC(ctx context.Context, cfg *envconf.Config) error {
	return p.GcpVPC.CreateVPC(ctx, cfg)
}

// DeleteCluster deletes the GKE cluster.
func (p *GCPProvisioner) DeleteCluster(ctx context.Context, cfg *envconf.Config) error {
	return p.GkeCluster.DeleteCluster(ctx)
}

// DeleteVPC deletes the VPC in Google Cloud.
func (p *GCPProvisioner) DeleteVPC(ctx context.Context, cfg *envconf.Config) error {
	return p.GcpVPC.DeleteVPC(ctx, cfg)
}

func (p *GCPProvisioner) GetProperties(ctx context.Context, cfg *envconf.Config) map[string]string {
	return map[string]string{
		"podvm_image_name": p.PodvmImage.Name,
		"machine_type":     p.GkeCluster.machineType,
		"project_id":       p.GkeCluster.projectID,
		"zone":             p.GkeCluster.zone,
		"network":          p.GcpVPC.vpcName,
		"caa_image_name":   p.CaaImageName,
	}
}

func (p *GCPProvisioner) UploadPodvm(imagePath string, ctx context.Context, cfg *envconf.Config) error {
	return nil
}
