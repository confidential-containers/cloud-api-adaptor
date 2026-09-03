// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package ibmcloudpowervs

import (
	"context"
	"fmt"

	"github.com/IBM-Cloud/power-go-client/clients/instance"
	"github.com/IBM-Cloud/power-go-client/ibmpisession"
	"github.com/IBM/go-sdk-core/v5/core"
	log "github.com/sirupsen/logrus"
	"sigs.k8s.io/e2e-framework/pkg/envconf"

	pv "github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/test/provisioner"
	kindutils "github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/test/provisioner/common/kind"
)

// IBMCloudPowerVSProvisioner implements CloudProvisioner for IBM Cloud PowerVS.
type IBMCloudPowerVSProvisioner struct {
	kind *kindutils.KindCluster

	IBMCloudPowerVSAPIKey    string
	IBMCloudAccountID        string
	PowerVSZone              string
	PowerVSServiceInstanceID string
	PowerVSImageID           string
	PowerVSNetworkID         string
	PowerVSSSHKeyName        string
	PowerVSSystemType        string
	PowerVSMemory            string
	PowerVSProcessorType     string
	PowerVSProcessors        string
	ForwarderPort            string
	ProxyTimeout             string
	UsePublicIP              string
}

func (p *IBMCloudPowerVSProvisioner) CreateCluster(ctx context.Context, cfg *envconf.Config) error {
	log.Infof("IBMCloudPowerVS: checking podvm image %s exists and is active", p.PowerVSImageID)
	if err := p.CheckImageExistsAndActive(ctx); err != nil {
		return fmt.Errorf("podvm image check failed: %w", err)
	}
	log.Infof("IBMCloudPowerVS: podvm image %s is active", p.PowerVSImageID)

	log.Info("IBMCloudPowerVS: provisioning local kind cluster for e2e tests")
	if err := p.kind.CreateCluster(ctx, cfg); err != nil {
		return err
	}
	log.Info("IBMCloudPowerVS: kind cluster ready")
	return nil
}

func (p *IBMCloudPowerVSProvisioner) DeleteCluster(ctx context.Context, cfg *envconf.Config) error {
	log.Info("IBMCloudPowerVS: deleting local kind cluster")
	if err := p.kind.DeleteCluster(ctx, cfg); err != nil {
		return err
	}
	log.Info("IBMCloudPowerVS: kind cluster deleted")
	return nil
}

func (p *IBMCloudPowerVSProvisioner) CreateVPC(ctx context.Context, cfg *envconf.Config) error {
	return nil
}

func (p *IBMCloudPowerVSProvisioner) DeleteVPC(ctx context.Context, cfg *envconf.Config) error {
	return nil
}

func (p *IBMCloudPowerVSProvisioner) GetProperties(ctx context.Context, cfg *envconf.Config) map[string]string {
	return map[string]string{
		"IBMCLOUD_API_KEY":            p.IBMCloudPowerVSAPIKey,
		"IBMCLOUD_ACCOUNT_ID":         p.IBMCloudAccountID,
		"POWERVS_ZONE":                p.PowerVSZone,
		"POWERVS_SERVICE_INSTANCE_ID": p.PowerVSServiceInstanceID,
		"POWERVS_IMAGE_ID":            p.PowerVSImageID,
		"POWERVS_NETWORK_ID":          p.PowerVSNetworkID,
		"POWERVS_SSH_KEY_NAME":        p.PowerVSSSHKeyName,
		"POWERVS_SYSTEM_TYPE":         p.PowerVSSystemType,
		"POWERVS_MEMORY":              p.PowerVSMemory,
		"POWERVS_PROCESSOR_TYPE":      p.PowerVSProcessorType,
		"POWERVS_PROCESSORS":          p.PowerVSProcessors,
		"FORWARDER_PORT":              p.ForwarderPort,
		"PROXY_TIMEOUT":               p.ProxyTimeout,
		"USE_PUBLIC_IP":               p.UsePublicIP,
	}
}

func (p *IBMCloudPowerVSProvisioner) UploadPodvm(imagePath string, ctx context.Context, cfg *envconf.Config) error {
	return nil
}

func NewIBMCloudPowerVSProvisioner(properties map[string]string) (pv.CloudProvisioner, error) {
	provisioner, err := newIBMCloudPowerVSProvisioner(properties)
	if err != nil {
		return nil, err
	}

	provisioner.kind, err = kindutils.NewKindCluster(properties)
	if err != nil {
		return nil, err
	}

	return provisioner, nil
}

func (p *IBMCloudPowerVSProvisioner) CheckImageExistsAndActive(ctx context.Context) error {
	if p.PowerVSImageID == "" {
		return fmt.Errorf("POWERVS_IMAGE_ID is not set")
	}

	client, err := newPowerVSImageClient(ctx, p.IBMCloudPowerVSAPIKey, p.IBMCloudAccountID, p.PowerVSServiceInstanceID, p.PowerVSZone)
	if err != nil {
		return fmt.Errorf("failed to create PowerVS client: %w", err)
	}

	image, err := client.Get(p.PowerVSImageID)
	if err != nil {
		return fmt.Errorf("failed to get image with ID %s: %w", p.PowerVSImageID, err)
	}
	if image == nil {
		return fmt.Errorf("image with ID %s was not found", p.PowerVSImageID)
	}

	if image.State != "active" {
		return fmt.Errorf("image with ID %s is not active (current state: %q)", p.PowerVSImageID, image.State)
	}

	return nil
}

func newPowerVSImageClient(ctx context.Context, apiKey, accountID, serviceInstanceID, zone string) (*instance.IBMPIImageClient, error) {
	authenticator := &core.IamAuthenticator{
		ApiKey: apiKey,
	}

	session, err := ibmpisession.NewIBMPISession(&ibmpisession.IBMPIOptions{
		Authenticator: authenticator,
		UserAccount:   accountID,
		Zone:          zone,
	})
	if err != nil {
		return nil, err
	}

	return instance.NewIBMPIImageClient(ctx, session, serviceInstanceID), nil
}
