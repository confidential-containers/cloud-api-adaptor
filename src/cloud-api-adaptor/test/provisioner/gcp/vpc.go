// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package gcp

import (
	"context"
	"fmt"
	"time"

	log "github.com/sirupsen/logrus"
	"google.golang.org/api/compute/v1"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/option"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
)

// GCPVPC implements the Google Compute VPC interface.
type GCPVPC struct {
	vpcName     string
	credentials string
	projectID   string
}

// NewGCPVPC creates a new GCPVPC object.
func NewGCPVPC(properties map[string]string) (*GCPVPC, error) {
	defaults := map[string]string{
		"vpc_name": "default",
	}

	for key, value := range properties {
		defaults[key] = value
	}

	requiredFields := []string{"project_id", "credentials"}
	for _, field := range requiredFields {
		if _, ok := defaults[field]; !ok {
			return nil, fmt.Errorf("%s is required", field)
		}
	}

	return &GCPVPC{
		vpcName:     defaults["vpc_name"],
		credentials: defaults["credentials"],
		projectID:   defaults["project_id"],
	}, nil
}

// CreateVPC creates a new VPC in Google Cloud.
func (g *GCPVPC) CreateVPC(
	ctx context.Context, cfg *envconf.Config,
) error {
	ctx, cancel := context.WithTimeout(ctx, time.Hour)
	defer cancel()

	srv, err := compute.NewService(ctx, option.WithCredentialsFile(g.credentials))
	if err != nil {
		return fmt.Errorf("GKE: compute.NewService: %v", err)
	}

  _, err = srv.Networks.Get(g.projectID, g.vpcName).Context(ctx).Do()
  if err == nil {
      log.Infof("GKE: Using existing VPC %s.\n", g.vpcName)
      return nil
  }

	network := &compute.Network{
		Name:                  g.vpcName,
		AutoCreateSubnetworks: true,
	}

	op, err := srv.Networks.Insert(g.projectID, network).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("GKE: Networks.Insert: %v", err)
	}

	log.Infof("GKE: VPC creation operation started: %v\n", op.Name)

	err = g.WaitForVPCCreation(ctx, 30*time.Minute)
	if err != nil {
		return fmt.Errorf("GKE: Error waiting for VPC to be created: %v", err)
	}

	// subnetwork := &compute.Subnetwork{
	//     Name:    "peer-pods-subnet",
	//     Network: op.SelfLink,
	//     Region:  "us-west1",
	// }
	//
	// _, err = srv.Subnetworks.Insert(g.projectID, "us-west1", subnetwork).Context(ctx).Do()
	// if err != nil {
	//     return fmt.Errorf("GKE: Subnetworks.Insert: %v", err)
	// }
	return nil
}

// DeleteVPC deletes a VPC in Google Cloud.
func (g *GCPVPC) DeleteVPC(ctx context.Context, cfg *envconf.Config) error {
	srv, err := compute.NewService(ctx, option.WithCredentialsFile(g.credentials))
	if err != nil {
		return fmt.Errorf("GKE: compute.NewService: %v", err)
	}

	op, err := srv.Networks.Delete(g.projectID, g.vpcName).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("GKE: Networks.Delete: %v", err)
	}

	log.Infof("GKE: VPC deletion operation started: %v\n", op.Name)

	err = g.WaitForVPCDeleted(ctx, 30*time.Minute)
	if err != nil {
		return fmt.Errorf("GKE: Error waiting for VPC to be deleted: %v", err)
	}

	return nil
}

// WaitForVPCCreation waits until the VPC is created and available.
func (g *GCPVPC) WaitForVPCCreation(
	ctx context.Context, timeout time.Duration,
) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	srv, err := compute.NewService(ctx, option.WithCredentialsFile(g.credentials))
	if err != nil {
		return fmt.Errorf("compute.NewService: %v", err)
	}

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for VPC creation")
		case <-ticker.C:
			network, err := srv.Networks.Get(g.projectID, g.vpcName).Context(ctx).Do()
			if err != nil {
				if apiErr, ok := err.(*googleapi.Error); ok && apiErr.Code == 404 {
					log.Info("Waiting for VPC to be created...")
					continue
				}
				return fmt.Errorf("Networks.Get: %v", err)
			}
			if network.SelfLink != "" {
				log.Info("VPC created successfully")
				return nil
			}
		}
	}
}

// WaitForVPCDeleted waits until the VPC is deleted.
func (g *GCPVPC) WaitForVPCDeleted(
	ctx context.Context, timeout time.Duration,
) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	srv, err := compute.NewService(ctx, option.WithCredentialsFile(g.credentials))
	if err != nil {
		return fmt.Errorf("GKE: compute.NewService: %v", err)
	}

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("GKE: timeout waiting for VPC deletion")
		case <-ticker.C:
			_, err := srv.Networks.Get(g.projectID, g.vpcName).Context(ctx).Do()
			if err != nil {
				if apiErr, ok := err.(*googleapi.Error); ok && apiErr.Code == 404 {
					log.Info("GKE: VPC deleted successfully")
					return nil
				}
				return fmt.Errorf("GKE: Networks.Get: %v", err)
			}
			log.Info("GKE: Waiting for VPC to be deleted...")
		}
	}
}
