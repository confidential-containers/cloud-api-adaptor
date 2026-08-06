// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package azure

import (
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	armnetwork "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v2"
)

func TestBuildNetworkConfigsSingleNIC(t *testing.T) {
	p := &azureProvider{serviceConfig: &Config{
		SubnetID:    "/subscriptions/sub/.../subnets/primary",
		UsePublicIP: true,
	}}

	configs := p.buildNetworkConfigs("instance1", false)
	if len(configs) != 1 {
		t.Fatalf("expected 1 NIC config, got %d", len(configs))
	}

	nic := configs[0]
	if *nic.Name != "instance1-net" {
		t.Errorf("expected NIC name %q, got %q", "instance1-net", *nic.Name)
	}
	if !*nic.Properties.Primary {
		t.Errorf("expected the only NIC in single-NIC mode to be primary")
	}
	if *nic.Properties.IPConfigurations[0].Properties.Subnet.ID != p.serviceConfig.SubnetID {
		t.Errorf("expected subnet %q, got %q", p.serviceConfig.SubnetID, *nic.Properties.IPConfigurations[0].Properties.Subnet.ID)
	}
	if nic.Properties.IPConfigurations[0].Properties.PublicIPAddressConfiguration == nil {
		t.Errorf("expected a public IP configuration since UsePublicIP is true")
	}
}

func TestBuildNetworkConfigsMultiNIC(t *testing.T) {
	p := &azureProvider{serviceConfig: &Config{
		SubnetID:          "/subscriptions/sub/.../subnets/primary",
		SecondarySubnetID: "/subscriptions/sub/.../subnets/secondary",
		UsePublicIP:       true,
	}}

	configs := p.buildNetworkConfigs("instance1", true)
	if len(configs) != 2 {
		t.Fatalf("expected 2 NIC configs, got %d", len(configs))
	}

	primary, secondary := configs[0], configs[1]

	if *primary.Name != "instance1-net" {
		t.Errorf("expected primary NIC name %q, got %q", "instance1-net", *primary.Name)
	}
	if !*primary.Properties.Primary {
		t.Errorf("expected the first NIC to be marked primary")
	}
	if *primary.Properties.IPConfigurations[0].Properties.Subnet.ID != p.serviceConfig.SubnetID {
		t.Errorf("expected primary NIC on subnet %q, got %q", p.serviceConfig.SubnetID, *primary.Properties.IPConfigurations[0].Properties.Subnet.ID)
	}
	if primary.Properties.IPConfigurations[0].Properties.PublicIPAddressConfiguration != nil {
		t.Errorf("did not expect a public IP on the primary (control-plane) NIC in multi-NIC mode")
	}

	if *secondary.Name != "instance1-ext" {
		t.Errorf("expected secondary NIC name %q, got %q", "instance1-ext", *secondary.Name)
	}
	if *secondary.Properties.Primary {
		t.Errorf("expected the second NIC to not be marked primary")
	}
	if *secondary.Properties.IPConfigurations[0].Properties.Subnet.ID != p.serviceConfig.SecondarySubnetID {
		t.Errorf("expected secondary NIC on subnet %q, got %q", p.serviceConfig.SecondarySubnetID, *secondary.Properties.IPConfigurations[0].Properties.Subnet.ID)
	}
	if secondary.Properties.IPConfigurations[0].Properties.PublicIPAddressConfiguration == nil {
		t.Errorf("expected a public IP on the secondary (external) NIC since UsePublicIP is true")
	}
}

func TestBuildNetworkConfigsMultiNICNoPublicIP(t *testing.T) {
	p := &azureProvider{serviceConfig: &Config{
		SubnetID:          "/subscriptions/sub/.../subnets/primary",
		SecondarySubnetID: "/subscriptions/sub/.../subnets/secondary",
		UsePublicIP:       false,
	}}

	configs := p.buildNetworkConfigs("instance1", true)
	for _, nic := range configs {
		if nic.Properties.IPConfigurations[0].Properties.PublicIPAddressConfiguration != nil {
			t.Errorf("did not expect a public IP configuration on %q since UsePublicIP is false", *nic.Name)
		}
	}
}

func TestOrderByPrimary(t *testing.T) {
	primaryIPC := &armnetwork.InterfaceIPConfiguration{
		Properties: &armnetwork.InterfaceIPConfigurationPropertiesFormat{
			PrivateIPAddress: to.Ptr("10.0.0.4"),
		},
	}
	secondaryIPC := &armnetwork.InterfaceIPConfiguration{
		Properties: &armnetwork.InterfaceIPConfigurationPropertiesFormat{
			PrivateIPAddress: to.Ptr("192.168.0.4"),
		},
	}

	primaryNic := &armnetwork.Interface{
		Properties: &armnetwork.InterfacePropertiesFormat{
			Primary:          to.Ptr(true),
			IPConfigurations: []*armnetwork.InterfaceIPConfiguration{primaryIPC},
		},
	}
	secondaryNic := &armnetwork.Interface{
		Properties: &armnetwork.InterfacePropertiesFormat{
			Primary:          to.Ptr(false),
			IPConfigurations: []*armnetwork.InterfaceIPConfiguration{secondaryIPC},
		},
	}

	t.Run("secondary returned by the API before primary is still reordered", func(t *testing.T) {
		got := orderByPrimary([]*armnetwork.Interface{secondaryNic, primaryNic})
		if len(got) != 2 {
			t.Fatalf("expected 2 IP configurations, got %d", len(got))
		}
		if *got[0].Properties.PrivateIPAddress != "10.0.0.4" {
			t.Errorf("expected the primary NIC's IP first, got %q", *got[0].Properties.PrivateIPAddress)
		}
		if *got[1].Properties.PrivateIPAddress != "192.168.0.4" {
			t.Errorf("expected the secondary NIC's IP second, got %q", *got[1].Properties.PrivateIPAddress)
		}
	})

	t.Run("single NIC is unaffected", func(t *testing.T) {
		got := orderByPrimary([]*armnetwork.Interface{primaryNic})
		if len(got) != 1 || *got[0].Properties.PrivateIPAddress != "10.0.0.4" {
			t.Fatalf("unexpected result: %+v", got)
		}
	})
}

func TestConfigVerifierSecondarySubnet(t *testing.T) {
	cases := []struct {
		name      string
		subnet    string
		secondary string
		wantErr   bool
	}{
		{name: "no secondary subnet configured", subnet: "subnet-a", secondary: "", wantErr: false},
		{name: "distinct secondary subnet", subnet: "subnet-a", secondary: "subnet-b", wantErr: false},
		{name: "secondary same as primary", subnet: "subnet-a", secondary: "subnet-a", wantErr: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := &azureProvider{serviceConfig: &Config{
				ImageID:           "image-1",
				SubnetID:          tc.subnet,
				SecondarySubnetID: tc.secondary,
			}}
			err := p.ConfigVerifier()
			if tc.wantErr && err == nil {
				t.Errorf("expected an error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Errorf("expected no error, got %v", err)
			}
		})
	}
}
