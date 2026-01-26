// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package openstack

import (
	"context"
	"fmt"
	"log"
	"net/netip"
	"time"

	"github.com/gophercloud/gophercloud/v2"
	"github.com/gophercloud/gophercloud/v2/openstack"
	"github.com/gophercloud/gophercloud/v2/openstack/compute/v2/attachinterfaces"
	"github.com/gophercloud/gophercloud/v2/openstack/compute/v2/servers"
	"github.com/gophercloud/gophercloud/v2/openstack/networking/v2/extensions/layer3/floatingips"
)

const (
	maxRetriesGetStatus = 60 // Maximum retries to get server status
	intervalGetStatus   = 3  // Interval (in seconds) between each retry to get server status
)

// NewProviderClient creates a new OpenStack provider client with authentication
func NewProviderClient(openstackcfg Config) (*gophercloud.ProviderClient, error) {
	authOpts := gophercloud.AuthOptions{
		IdentityEndpoint: openstackcfg.IdentityEndpoint,
		Username:         openstackcfg.Username,
		Password:         openstackcfg.Password,
		TenantName:       openstackcfg.TenantName,
		DomainName:       openstackcfg.DomainName,
		AllowReauth:      true, // Allow re-authentication
	}

	client, err := openstack.AuthenticatedClient(context.Background(), authOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to authenticate with OpenStack: %w", err)
	}

	return client, nil
}

// NewComputeClient creates a new OpenStack Compute service client
func NewComputeClient(providerClient *gophercloud.ProviderClient, endpointOpts gophercloud.EndpointOpts) (*gophercloud.ServiceClient, error) {
	client, err := openstack.NewComputeV2(providerClient, endpointOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to create OpenStack Compute client: %w", err)
	}
	return client, nil
}

// NewNetworkClient creates a new OpenStack Network service client
func NewNetworkClient(providerClient *gophercloud.ProviderClient, endpointOpts gophercloud.EndpointOpts) (*gophercloud.ServiceClient, error) {
	client, err := openstack.NewNetworkV2(providerClient, endpointOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to create OpenStack network client: %w", err)
	}
	return client, nil
}

// MakeNetworkList converts a list of network IDs into a list of OpenStack Network structs
func MakeNetworkList(networkIDs []string) []servers.Network {
	networks := make([]servers.Network, 0, len(networkIDs))
	for _, id := range networkIDs {
		networks = append(networks, servers.Network{UUID: id})
	}
	return networks
}

// extractIPsFromAddresses parses a map of OpenStack server addresses and returns a slice of netip.Addr IPs.
func extractFixedIPsFromAddresses(addresses map[string]any) ([]netip.Addr, error) {
	var ips []netip.Addr
	var ip6s []netip.Addr

	for _, addrList := range addresses {
		list, ok := addrList.([]any)
		if !ok {
			return nil, fmt.Errorf("unexpected type for addrList: got %T, want []any", addrList)
		}

		for _, addrItem := range list {
			addrMap, ok := addrItem.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("unexpected type for addrItem: got %T, want map[string]any", addrItem)
			}
			addrVal, ok := addrMap["addr"]
			if !ok {
				return nil, fmt.Errorf("missing 'addr' key in address item: %v", addrMap)
			}
			if addrVal == nil {
				return nil, fmt.Errorf("'addr' value is nil in address item: %v", addrMap)
			}
			addrStr, ok := addrVal.(string)
			if !ok {
				return nil, fmt.Errorf("unexpected type for 'addr' value: got %T, want string in %v", addrVal, addrMap)
			}
			ip, err := netip.ParseAddr(addrStr)
			if err != nil {
				return nil, fmt.Errorf("failed to parse IP address %s: %w", addrStr, err)
			}
			if ip.Is4() {
				ips = append(ips, ip)
			} else {
				ip6s = append(ip6s, ip)
			}
		}
	}
	return append(ips, ip6s...), nil
}

// Request the server status and get fixed IPs in the response.
func GetFixedIPs(ctx context.Context, computeClient *gophercloud.ServiceClient, serverID string) ([]netip.Addr, error) {
	var ips []netip.Addr
	for retries := 0; retries < maxRetriesGetStatus; retries++ {
		time.Sleep(intervalGetStatus * time.Second)
		server, err := servers.Get(ctx, computeClient, serverID).Extract()
		if err != nil {
			return nil, fmt.Errorf("failed to get server status: %w", err)
		}

		if server.Status != "ACTIVE" {
			continue
		}
		ips, err = extractFixedIPsFromAddresses(server.Addresses)
		if err != nil {
			return nil, err
		}
		if len(ips) != 0 {
			return ips, nil
		}
	}
	return nil, fmt.Errorf("failed to get fixed IPs after %d retries", maxRetriesGetStatus)
}

// AssignFloatingIP assigns a floating IP from the specified floating network to the given fixed IP.
func AssignFloatingIP(ctx context.Context, networkClient *gophercloud.ServiceClient, portID string, floatingNetworkID string) (netip.Addr, string, error) {
	res, err := floatingips.Create(ctx, networkClient, floatingips.CreateOpts{
		FloatingNetworkID: floatingNetworkID,
		PortID:            portID,
	}).Extract()
	if err != nil {
		return netip.Addr{}, "", fmt.Errorf("failed to create floating IP: %w", err)
	}
	fip, err := netip.ParseAddr(res.FloatingIP)
	if err != nil {
		return netip.Addr{}, "", fmt.Errorf("invalid floating IP address received: %+v", res)
	}

	return fip, res.ID, nil
}

// GetPortID retrieves the port ID associated with the given server ID and fixed IP.
func GetPortID(computeClient *gophercloud.ServiceClient, serverID string, fixedIP string) string {
	allPages, err := attachinterfaces.List(computeClient, serverID).AllPages(context.TODO())
	if err != nil {
		log.Printf("failed to list attached interfaces for server %s: %v", serverID, err)
		return ""
	}
	allInterfaces, err := attachinterfaces.ExtractInterfaces(allPages)
	if err != nil {
		log.Printf("failed to extract interfaces for server %s: %v", serverID, err)
		return ""
	}

	for _, eachInterface := range allInterfaces {
		for _, fixedIPs := range eachInterface.FixedIPs {
			if fixedIPs.IPAddress == fixedIP {
				return eachInterface.PortID
			}
		}
	}
	log.Printf("failed to locate a network interface associated with the fixedIP: %v", fixedIP)
	return ""
}

// DeleteFloatingIP deletes the floating IP with the specified ID.
func DeleteFloatingIP(ctx context.Context, networkClient *gophercloud.ServiceClient, floatingIPID string) error {
	err := floatingips.Delete(ctx, networkClient, floatingIPID).ExtractErr()
	if err != nil {
		log.Printf("failed to delete floating IP %s: %v", floatingIPID, err)
	}
	return nil
}
