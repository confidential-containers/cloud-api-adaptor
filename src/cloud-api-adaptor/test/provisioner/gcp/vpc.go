// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package gcp

import (
	"context"
	"fmt"
	"time"

	log "github.com/sirupsen/logrus"
	compute "google.golang.org/api/compute/v1"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/option"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
)

// GCPVPC implements the Google Compute VPC interface.
type GCPVPC struct {
	vpcName     string
	credentials string
	ProjectID   string
}

// NewGCPVPC creates a new GCPVPC object.
func NewGCPVPC(properties map[string]string) (*GCPVPC, error) {
	defaults := map[string]string{}

	for key, value := range properties {
		defaults[key] = value
	}

	requiredFields := []string{"project_id", "credentials", "vpc_name"}
	for _, field := range requiredFields {
		if _, ok := defaults[field]; !ok {
			return nil, fmt.Errorf("%s is required", field)
		}
	}

	return &GCPVPC{
		vpcName:     defaults["vpc_name"],
		credentials: defaults["credentials"],
		ProjectID:   defaults["project_id"],
	}, nil
}

// CreateVPC creates a new VPC in Google Cloud.
func (g *GCPVPC) CreateVPC(
	ctx context.Context, cfg *envconf.Config,
) error {
	ctx, cancel := context.WithTimeout(ctx, time.Hour)
	defer cancel()

	credentials, err := expandUser(g.credentials)
	if err != nil {
		return nil
	}

	srv, err := compute.NewService(ctx, option.WithCredentialsFile(credentials))
	if err != nil {
		return fmt.Errorf("GKE: compute.NewService: %v", err)
	}

	_, err = srv.Networks.Get(g.ProjectID, g.vpcName).Context(ctx).Do()
	if err == nil {
		log.Infof("GKE: Using existing VPC %s.\n", g.vpcName)
		return nil
	}

	network := &compute.Network{
		Name:                  g.vpcName,
		AutoCreateSubnetworks: true,
	}

	op, err := srv.Networks.Insert(g.ProjectID, network).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("GKE: Networks.Insert: %v", err)
	}

	log.Infof("GKE: VPC creation operation started: %v\n", op.Name)

	err = waitForOperation(ctx, srv, g.ProjectID, op.Name, 5*time.Minute)
	if err != nil {
		return fmt.Errorf("GKE: Error waiting for VPC creation operation: %v", err)
	}

	// err = g.WaitForVPCCreation(ctx, 30*time.Minute)
	// if err != nil {
	// 	return fmt.Errorf("GKE: Error waiting for VPC to be created: %v", err)
	// }

	firewall := &compute.Firewall{
		Name:    fmt.Sprintf("allow-port-15150-%s", g.vpcName),
		Network: fmt.Sprintf("projects/%s/global/networks/%s", g.ProjectID, g.vpcName),
		Allowed: []*compute.FirewallAllowed{
			{
				IPProtocol: "tcp",
				Ports:      []string{"15150"},
			},
		},
		Direction: "INGRESS",
	}

	fwOp, err := srv.Firewalls.Insert(g.ProjectID, firewall).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("GKE: Firewalls.Insert: %v", err)
	}

	log.Infof("GKE: Firewall rule creation started: %v\n", fwOp.Name)

	return nil
}

// DeleteVPC deletes a VPC in Google Cloud.
func (g *GCPVPC) DeleteVPC(ctx context.Context, cfg *envconf.Config) error {
	srv, err := compute.NewService(ctx, option.WithCredentialsFile(g.credentials))
	if err != nil {
		return fmt.Errorf("GKE: compute.NewService: %v", err)
	}

	firewallList, err := srv.Firewalls.List(g.ProjectID).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("GKE: Firewalls.List: %v", err)
	}

	for _, rule := range firewallList.Items {
		if rule.Network == fmt.Sprintf("projects/%s/global/networks/%s", g.ProjectID, g.vpcName) {
			_, err := srv.Firewalls.Delete(g.ProjectID, rule.Name).Context(ctx).Do()
			if err != nil {
				return fmt.Errorf("GKE: Firewalls.Delete (%s): %v", rule.Name, err)
			}
			log.Infof("GKE: Deleted firewall rule: %s", rule.Name)
		}
	}

	op, err := srv.Networks.Delete(g.ProjectID, g.vpcName).Context(ctx).Do()
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

func waitForOperation(ctx context.Context, srv *compute.Service, projectID, operationName string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		if time.Now().After(deadline) {
			return fmt.Errorf("timeout waiting for operation %s", operationName)
		}

		op, err := srv.GlobalOperations.Get(projectID, operationName).Context(ctx).Do()
		if err != nil {
			return fmt.Errorf("GlobalOperations.Get: %v", err)
		}

		if op.Status == "DONE" {
			if op.Error != nil {
				return fmt.Errorf("operation %s failed: %v", operationName, op.Error)
			}
			log.Infof("Operation %s completed successfully.", operationName)
			return nil
		}

		log.Infof("Waiting for operation %s to complete...", operationName)
		time.Sleep(5 * time.Second)
	}
}

// WaitForVPCCreation waits until the VPC is created and available.
func (g *GCPVPC) WaitForVPCCreation(ctx context.Context, timeout time.Duration) error {
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
			network, err := srv.Networks.Get(g.ProjectID, g.vpcName).Context(ctx).Do()
			if err != nil {
				if apiErr, ok := err.(*googleapi.Error); ok && apiErr.Code == 404 {
					log.Info("Waiting for VPC to be created...")
					continue
				}
				return fmt.Errorf("Networks.Get: %v", err)
			}

			// New check: Ensure VPC is fully usable
			if network.SelfLink != "" && network.RoutingConfig != nil {
				log.Info("VPC is fully ready.")
				return nil
			}

			log.Info("VPC exists but is still initializing...")
		}
	}
}

// func (g *GCPVPC) WaitForVPCCreation(
// 	ctx context.Context, timeout time.Duration,
// ) error {
// 	ctx, cancel := context.WithTimeout(ctx, timeout)
// 	defer cancel()
//
// 	srv, err := compute.NewService(ctx, option.WithCredentialsFile(g.credentials))
// 	if err != nil {
// 		return fmt.Errorf("compute.NewService: %v", err)
// 	}
//
// 	ticker := time.NewTicker(5 * time.Second)
// 	defer ticker.Stop()
//
// 	for {
// 		select {
// 		case <-ctx.Done():
// 			return fmt.Errorf("timeout waiting for VPC creation")
// 		case <-ticker.C:
// 			network, err := srv.Networks.Get(g.ProjectID, g.vpcName).Context(ctx).Do()
// 			if err != nil {
// 				if apiErr, ok := err.(*googleapi.Error); ok && apiErr.Code == 404 {
// 					log.Info("Waiting for VPC to be created...")
// 					continue
// 				}
// 				return fmt.Errorf("Networks.Get: %v", err)
// 			}
// 			if network.SelfLink != "" {
// 				log.Info("VPC created successfully")
// 				return nil
// 			}
// 		}
// 	}
// }

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
			_, err := srv.Networks.Get(g.ProjectID, g.vpcName).Context(ctx).Do()
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
