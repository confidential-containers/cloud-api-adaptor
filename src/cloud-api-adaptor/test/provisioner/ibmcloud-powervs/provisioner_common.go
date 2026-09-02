// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package ibmcloudpowervs

import (
	"context"

	log "github.com/sirupsen/logrus"
	"sigs.k8s.io/e2e-framework/pkg/envconf"

	pv "github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/test/provisioner"
	kindutils "github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/test/provisioner/common/kind"
)

// IBMCloudPowerVSProvisioner implements CloudProvisioner for IBM Cloud PowerVS.
type IBMCloudPowerVSProvisioner struct {
	kind *kindutils.KindCluster

	IBMCloudPowerVSAPIKey    string
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
