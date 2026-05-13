// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package openstack

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	pv "github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/test/provisioner"
	"github.com/gophercloud/gophercloud/v2"
	gophcos "github.com/gophercloud/gophercloud/v2/openstack"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
)

var OpenStackProvs = &OpenStackProvisioner{}

type OpenStackProvisioner struct {
	OpenStackClient *gophercloud.ProviderClient
	Properties      map[string]string
	CaaImage        string
}

func newOpenStackClient(credentials map[string]string) (*gophercloud.ProviderClient, error) {
	authOpts := gophercloud.AuthOptions{
		IdentityEndpoint: credentials["OPENSTACK_IDENTITY_ENDPOINT"],
		Username:         credentials["OPENSTACK_USERNAME"],
		Password:         credentials["OPENSTACK_PASSWORD"],
		TenantName:       credentials["OPENSTACK_TENANT_NAME"],
		DomainName:       credentials["OPENSTACK_DOMAIN_NAME"],
		AllowReauth:      true, // Allow re-authentication
	}

	client, err := gophcos.AuthenticatedClient(context.Background(), authOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to authenticate with OpenStack: %w", err)
	}

	return client, nil
}

func NewOpenStackProvisioner(properties map[string]string) (pv.CloudProvisioner, error) {
	// Validate required credentials
	requiredCredential := []string{
		"OPENSTACK_IDENTITY_ENDPOINT",
		"OPENSTACK_USERNAME",
		"OPENSTACK_PASSWORD",
		"OPENSTACK_TENANT_NAME",
		"OPENSTACK_DOMAIN_NAME",
	}

	credentials := make(map[string]string)
	for _, v := range requiredCredential {
		if val, ok := properties[v]; ok {
			credentials[v] = val
		} else {
			return nil, fmt.Errorf("missing required OpenStack credential: %s", v)
		}
	}

	// Create OpenStack Provider Client
	println("Create an OpenStack client with the following credentials:")
	for k, v := range credentials {
		println(k, "=", v)
	}
	client, err := newOpenStackClient(credentials)
	if err != nil {
		return nil, err
	}

	println("Properties received by NewOpenStackProvisioner:")
	props := make(map[string]string)
	for k, v := range properties {
		if strings.HasPrefix(k, "OPENSTACK_") {
			props[k] = v
			println(k, "=", v)
		}
	}

	OpenStackProvs = &OpenStackProvisioner{
		OpenStackClient: client,
		Properties:      props,
		CaaImage:        properties["CAA_IMAGE"],
	}
	return OpenStackProvs, nil
}

func (p *OpenStackProvisioner) CreateCluster(ctx context.Context, cfg *envconf.Config) error {
	return nil
}

func (p *OpenStackProvisioner) CreateVPC(ctx context.Context, cfg *envconf.Config) error {
	return nil
}

func (p *OpenStackProvisioner) DeleteCluster(ctx context.Context, cfg *envconf.Config) error {
	return nil
}

func (p *OpenStackProvisioner) DeleteVPC(ctx context.Context, cfg *envconf.Config) error {
	return nil
}

func (p *OpenStackProvisioner) GetProperties(ctx context.Context, cfg *envconf.Config) map[string]string {
	props := make(map[string]string)
	props["CAA_IMAGE"] = p.CaaImage

	for k, v := range p.Properties {
		props[k] = v
	}
	return props
}

func (p *OpenStackProvisioner) UploadPodvm(imagePath string, ctx context.Context, cfg *envconf.Config) error {
	return nil
}

type OpenStackInstallChart struct {
	Helm *pv.Helm
}

func NewOpenStackInstallChart(installDir, provider string) (pv.InstallChart, error) {
	chartPath := filepath.Join(installDir, "charts", "peerpods")
	namespace := pv.GetCAANamespace()
	releaseName := "peerpods"
	debug := false

	helm, err := pv.NewHelm(chartPath, namespace, releaseName, provider, debug)
	if err != nil {
		return nil, err
	}

	return &OpenStackInstallChart{
		Helm: helm,
	}, nil
}

func (o *OpenStackInstallChart) Install(ctx context.Context, cfg *envconf.Config) error {
	return o.Helm.Install(ctx, cfg)
}

func (o *OpenStackInstallChart) Uninstall(ctx context.Context, cfg *envconf.Config) error {
	return o.Helm.Uninstall(ctx, cfg)
}

func (o *OpenStackInstallChart) Configure(ctx context.Context, cfg *envconf.Config, properties map[string]string) error {
	if OpenStackProvs.CaaImage != "" {

		// Split the CAA image into name and tag using the last colon as the separator
		lastColonIndex := strings.LastIndex(OpenStackProvs.CaaImage, ":")

		if lastColonIndex == -1 {
			o.Helm.OverrideValues["image.name"] = OpenStackProvs.CaaImage
		} else if lastColonIndex == 0 || lastColonIndex == len(OpenStackProvs.CaaImage)-1 {
			return fmt.Errorf("Invalid CAA image format: %s", OpenStackProvs.CaaImage)
		} else {
			o.Helm.OverrideValues["image.name"] = OpenStackProvs.CaaImage[:lastColonIndex]
			o.Helm.OverrideValues["image.tag"] = OpenStackProvs.CaaImage[lastColonIndex+1:]
		}
	}

	// Override provider values
	providerValueKeys := []string{
		"OPENSTACK_SERVER_PREFIX",
		"OPENSTACK_IMAGE_ID",
		"OPENSTACK_FLAVOR_ID",
		"OPENSTACK_NETWORK_ID",
		"OPENSTACK_SECURITY_GROUP",
		"OPENSTACK_FLOATING_IP_NETWORK_ID",
		"OPENSTACK_IDENTITY_ENDPOINT",
		"OPENSTACK_DOMAIN_NAME",
		"OPENSTACK_REGION",
	}
	for _, k := range providerValueKeys {
		if OpenStackProvs.Properties[k] != "" {
			o.Helm.OverrideProviderValues[k] = OpenStackProvs.Properties[k]
		}
	}

	// Override provider secrets
	secretKeys := []string{
		"OPENSTACK_USERNAME",
		"OPENSTACK_PASSWORD",
		"OPENSTACK_TENANT_NAME",
	}

	for _, k := range secretKeys {
		if OpenStackProvs.Properties[k] != "" {
			o.Helm.OverrideProviderSecrets[k] = OpenStackProvs.Properties[k]
		}
	}
	return nil
}
