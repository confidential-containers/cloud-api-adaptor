// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package openstack

import (
	"bytes"
	"context"
	"log"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/gophercloud/gophercloud/v2"
	"github.com/gophercloud/gophercloud/v2/openstack/compute/v2/servers"
)

func TestNewProviderClient(t *testing.T) {
	server := CreateServer()

	tests := []struct {
		name      string
		cfg       Config
		wantError bool
	}{
		{
			name: "ValidConfig",
			cfg: Config{
				IdentityEndpoint: server.URL + "/v3",
				Username:         "test-user",
				Password:         "test-password",
				TenantName:       "test-tenant",
				DomainName:       "test-domain",
			},
			wantError: false,
		},
		{
			name: "InvalidEndpointConfig",
			cfg: Config{
				IdentityEndpoint: "http://bad-address.example.com/v3",
				Username:         "test-user",
				Password:         "test-password",
				TenantName:       "test-tenant",
				DomainName:       "test-domain",
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			providerClient, err := NewProviderClient(tt.cfg)
			if tt.wantError {
				if err == nil {
					t.Errorf("Expected error for test case %v, but got none\n", tt.name)
				} else {
					t.Logf("Expected error occurred for test case %v: %v\n", tt.name, err)
				}
				if providerClient != nil {
					t.Errorf("Expected providerClient to be nil for test case %v, but got: %+v\n", tt.name, providerClient)
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error for test case %v, but got: %v\n", tt.name, err)
				}
				if providerClient != nil {
					t.Logf("ProviderClient successfully created for test case %v: %+v\n", tt.name, providerClient)
				} else {
					t.Errorf("Expected providerClient to be created for test case %v, but got nil\n", tt.name)
				}
			}
		})
	}
}

func TestNewComputeClient(t *testing.T) {
	server := CreateServer()
	serverNoCompute := CreateServerNoCompute()

	tests := []struct {
		name         string
		cfg          Config
		endpointOpts gophercloud.EndpointOpts
		server       *httptest.Server
		wantError    bool
	}{
		{
			name: "ValidEndpointOptsAndProviderClient",
			cfg: Config{
				IdentityEndpoint: server.URL + "/v3",
				Username:         "test-user",
				Password:         "test-password",
				TenantName:       "test-tenant",
				DomainName:       "test-domain",
			},
			endpointOpts: gophercloud.EndpointOpts{
				Region: "test-region",
			},
			server:    server,
			wantError: false,
		},
		{
			name: "InvalidProviderClient",
			cfg: Config{
				IdentityEndpoint: serverNoCompute.URL + "/v3",
				Username:         "test-user",
				Password:         "test-password",
				TenantName:       "test-tenant",
				DomainName:       "test-domain",
			},
			endpointOpts: gophercloud.EndpointOpts{
				Region: "test-region",
			},
			server:    serverNoCompute,
			wantError: true,
		},
		{
			name: "InvalidEndpointOpts",
			cfg: Config{
				IdentityEndpoint: server.URL + "/v3",
				Username:         "test-user",
				Password:         "test-password",
				TenantName:       "test-tenant",
				DomainName:       "test-domain",
			},
			endpointOpts: gophercloud.EndpointOpts{
				Region: "bad-region",
			},
			server:    server,
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			providerClient, err := NewProviderClient(tt.cfg)
			if err != nil {
				if tt.wantError {
					t.Logf("Expected error occurred in provider client creation for %v: %v", tt.name, err)
					return
				}
				t.Fatalf("Unexpected error in provider client creation for %v: %v", tt.name, err)
			} else {
				t.Logf("Using provider client's pointer: %p", providerClient)
			}

			computeClient, err := NewComputeClient(providerClient, tt.endpointOpts)
			if tt.wantError {
				if err == nil {
					t.Errorf("Expected error for test case %v, but got none", tt.name)
				} else {
					t.Logf("Expected error occurred for test case %v: %v", tt.name, err)
				}
				if computeClient != nil {
					t.Errorf("Expected computeClient to be nil for test case %v, but got: %+v", tt.name, computeClient)
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error for test case %v, but got: %v", tt.name, err)
				}
				if computeClient != nil {
					t.Logf("ComputeClient successfully created for test case %v: %+v", tt.name, computeClient)
				} else {
					t.Errorf("Expected computeClient to be created for test case %v, but got nil", tt.name)
				}
			}
		})
	}
}

func TestNewNetworkClient(t *testing.T) {
	server := CreateServer()
	serverNoNetwork := CreateServerNoNetwork()

	tests := []struct {
		name         string
		cfg          Config
		endpointOpts gophercloud.EndpointOpts
		server       *httptest.Server
		wantError    bool
	}{
		{
			name: "ValidEndpointOptsAndProviderClient",
			cfg: Config{
				IdentityEndpoint: server.URL + "/v3",
				Username:         "test-user",
				Password:         "test-password",
				TenantName:       "test-tenant",
				DomainName:       "test-domain",
			},
			endpointOpts: gophercloud.EndpointOpts{
				Region: "test-region",
			},
			server:    server,
			wantError: false,
		},
		{
			name: "InvalidProviderClient",
			cfg: Config{
				IdentityEndpoint: serverNoNetwork.URL + "/v3",
				Username:         "test-user",
				Password:         "test-password",
				TenantName:       "test-tenant",
				DomainName:       "test-domain",
			},
			endpointOpts: gophercloud.EndpointOpts{
				Region: "test-region",
			},
			server:    serverNoNetwork,
			wantError: true,
		},
		{
			name: "InvalidEndpointOpts",
			cfg: Config{
				IdentityEndpoint: server.URL + "/v3",
				Username:         "test-user",
				Password:         "test-password",
				TenantName:       "test-tenant",
				DomainName:       "test-domain",
			},
			endpointOpts: gophercloud.EndpointOpts{
				Region: "bad-region",
			},
			server:    server,
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			providerClient, err := NewProviderClient(tt.cfg)
			if err != nil {
				if tt.wantError {
					t.Logf("Expected error occurred in provider client creation for %v: %v", tt.name, err)
					return
				}
				t.Fatalf("Unexpected error in provider client creation for %v: %v", tt.name, err)
			} else {
				t.Logf("Using provider client's pointer: %p", providerClient)
			}

			networkClient, err := NewNetworkClient(providerClient, tt.endpointOpts)
			if tt.wantError {
				if err == nil {
					t.Errorf("Expected error for test case %v, but got none", tt.name)
				} else {
					t.Logf("Expected error occurred for test case %v: %v", tt.name, err)
				}
				if networkClient != nil {
					t.Errorf("Expected networkClient to be nil for test case %v, but got: %+v", tt.name, networkClient)
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error for test case %v, but got: %v", tt.name, err)
				}
				if networkClient != nil {
					t.Logf("NetworkClient successfully created for test case %v: %+v", tt.name, networkClient)
				} else {
					t.Errorf("Expected networkClient to be created for test case %v, but got nil", tt.name)
				}
			}
		})
	}
}

func TestMakeNetworkList(t *testing.T) {
	tests := []struct {
		name   string
		input  []string
		expect []servers.Network
	}{
		{
			name:   "SingleNetwork",
			input:  []string{"net-1"},
			expect: []servers.Network{{UUID: "net-1"}},
		},
		{
			name:   "NoNetwork",
			input:  []string{},
			expect: []servers.Network{},
		},
		{
			name:   "MultipleNetworks",
			input:  []string{"net-1", "net-2", "net-3"},
			expect: []servers.Network{{UUID: "net-1"}, {UUID: "net-2"}, {UUID: "net-3"}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			networks := MakeNetworkList(tt.input)
			if !reflect.DeepEqual(tt.expect, networks) {
				t.Errorf("Expected networks: %v, but got: %v", tt.expect, networks)
			} else {
				t.Logf("Expected networks: %+v, got networks: %+v", tt.expect, networks)
			}
		})
	}
}

func TestExtractFixedIPsFromAddresses(t *testing.T) {
	tests := []struct {
		name     string
		input    map[string]any
		expected []string
		wantErr  bool
	}{
		{
			name:     "EmptyAddresses",
			input:    map[string]any{},
			expected: []string{},
			wantErr:  false,
		},
		{
			name: "SingleAddress",
			input: map[string]any{
				"address1": []any{
					map[string]any{"addr": "11.22.33.44"},
				},
			},
			expected: []string{"11.22.33.44"},
			wantErr:  false,
		},
		{
			name: "MultipleAddresses",
			input: map[string]any{
				"address1": []any{
					map[string]any{"addr": "11.22.33.44"},
				},
				"address2": []any{
					map[string]any{"addr": "55.66.77.88"},
				},
				"address3": []any{
					map[string]any{"addr": "12.34.56.78"},
				},
			},
			expected: []string{"11.22.33.44", "55.66.77.88", "12.34.56.78"},
			wantErr:  false,
		},
		{
			name: "InvalidAddressFormat",
			input: map[string]any{
				"address1": []any{
					map[string]any{"addr": "xxxxxxxxxxx"},
				},
			},
			expected: nil,
			wantErr:  true,
		},
		{
			name: "MissingAddrKey",
			input: map[string]any{
				"address1": []any{
					map[string]any{"ng": "11.22.33.44"},
				},
			},
			expected: nil,
			wantErr:  true,
		},
		{
			name: "OnlyIPv6",
			input: map[string]any{
				"address1": []any{
					map[string]any{"addr": "2001:db8::1"},
					map[string]any{"addr": "2001:db8::2"},
					map[string]any{"addr": "2001:db8::3"},
				},
			},
			expected: []string{"2001:db8::1", "2001:db8::2", "2001:db8::3"},
			wantErr:  false,
		},
		{
			name: "MixedIPv4AndIPv6",
			input: map[string]any{
				"address1": []any{
					map[string]any{"addr": "12.34.56.78"},
					map[string]any{"addr": "2001:db8::1"},
				},
			},
			expected: []string{"12.34.56.78", "2001:db8::1"},
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ips, err := extractFixedIPsFromAddresses(tt.input)
			t.Log("Extracted IPs:", ips)
			if tt.wantErr {
				if err == nil {
					t.Errorf("Expected error for test case %v, but got none", tt.name)
				} else {
					t.Logf("Expected error occurred for test case %v: %v", tt.name, err)
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error for test case %v, but got: %v", tt.name, err)
				} else if tt.name == "MixedIPv4AndIPv6" {
					actualIPs := make([]string, len(ips))
					for i, ip := range ips {
						actualIPs[i] = ip.String()
					}

					if !reflect.DeepEqual(tt.expected, actualIPs) {
						t.Errorf("Expected IPs: %v, but got: %v for test case %v", tt.expected, actualIPs, tt.name)
					} else {
						t.Logf("Extracted IPs match expected for test case %v: %+v", tt.name, actualIPs)
					}
				}
			}
		})
	}
}

func TestGetFixedIPs(t *testing.T) {
	tests := []struct {
		name      string
		cfg       Config
		handler   http.HandlerFunc
		wantError bool
	}{
		{
			name: "InvalidComputeClient",
			cfg: Config{
				Username:   "test-user",
				Password:   "test-password",
				TenantName: "test-tenant",
				DomainName: "test-domain",
			},
			handler:   HandlerFuncNoCompute,
			wantError: true,
		},
		{
			name: "InvalidServerID",
			cfg: Config{
				Username:   "test-user",
				Password:   "test-password",
				TenantName: "test-tenant",
				DomainName: "test-domain",
			},
			handler:   HandlerFuncInvalidServerID,
			wantError: true,
		},
		{
			name: "ExceedsMaxRetries",
			cfg: Config{
				Username:   "test-user",
				Password:   "test-password",
				TenantName: "test-tenant",
				DomainName: "test-domain",
			},
			handler:   HandlerFuncExceedsMaxRetries,
			wantError: true,
		},
		{
			name: "InvalidServerAddressReceived",
			cfg: Config{
				Username:   "test-user",
				Password:   "test-password",
				TenantName: "test-tenant",
				DomainName: "test-domain",
			},
			handler:   HandlerFuncInvalidServerAddress,
			wantError: true,
		},
		{
			name: "RetriesAndSucceeds",
			cfg: Config{
				Username:   "test-user",
				Password:   "test-password",
				TenantName: "test-tenant",
				DomainName: "test-domain",
			},
			handler:   HandlerFuncRetriesAndSucceeds,
			wantError: false,
		},
		{
			name: "SucceedsWithoutRetries",
			cfg: Config{
				Username:   "test-user",
				Password:   "test-password",
				TenantName: "test-tenant",
				DomainName: "test-domain",
			},
			handler:   HandlerFuncSucceedsWithoutRetries,
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.name == "RetriesAndSucceeds" {
				atomic.StoreInt32(&requestCount, 0)
			}

			var computeClient *gophercloud.ServiceClient

			if tt.name == "InvalidComputeClient" {
				computeClient = &gophercloud.ServiceClient{
					ProviderClient: &gophercloud.ProviderClient{},
					Endpoint:       "http://bad-endpoint/",
				}
			} else {
				mux := http.NewServeMux()

				mux.HandleFunc("/v3/auth/tokens", HandlerFuncV3)
				mux.HandleFunc("/v2.0/tokens", HandlerFuncV2)

				mux.HandleFunc("/v2.0/test-tenant/servers/test-server-id", tt.handler)

				server := httptest.NewServer(mux)
				defer server.Close()

				cfg := tt.cfg
				cfg.IdentityEndpoint = server.URL + "/v3"

				providerClient, err := NewProviderClient(cfg)
				if err != nil {
					t.Fatalf("Failed to create provider client: %v", err)
				}

				computeClient, err = NewComputeClient(providerClient, gophercloud.EndpointOpts{
					Region: "test-region",
				})
				if err != nil {
					t.Fatalf("Failed to create compute client: %v", err)
				}
			}

			ips, err := GetFixedIPs(context.Background(), computeClient, "test-server-id")

			if tt.wantError {
				if err == nil {
					t.Errorf("Expected error for test case %v, but got none", tt.name)
				} else {
					t.Logf("Expected error occurred for test case %v: %v", tt.name, err)
				}
				if len(ips) > 0 {
					t.Errorf("Expected IPs to be nil or empty when error occurs for test case %v, but got: %v", tt.name, ips)
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error for test case %v, but got: %v", tt.name, err)
				} else {
					t.Logf("Successfully retrieved IPs for test case %v: %v", tt.name, ips)
				}
				if len(ips) == 0 {
					t.Errorf("Expected IPs to have values when no error occurs for test case %v, but got nil or empty: %v", tt.name, ips)
				}
			}
		})
	}
}

func TestAssignFloatingIP(t *testing.T) {
	tests := []struct {
		name      string
		cfg       Config
		handler   http.HandlerFunc
		wantError bool
	}{
		{
			name: "InvalidNetworkClient",
			cfg: Config{
				Username:   "test-user",
				Password:   "test-password",
				TenantName: "test-tenant",
				DomainName: "test-domain",
			},
			handler:   HandlerFuncInvalidNetworkClient,
			wantError: true,
		},
		{
			name: "InvalidFloatingNetworkID",
			cfg: Config{
				Username:   "test-user",
				Password:   "test-password",
				TenantName: "test-tenant",
				DomainName: "test-domain",
			},
			handler:   HandlerFuncInvalidFloatingNetworkID,
			wantError: true,
		},
		{
			name: "InvalidPortID",
			cfg: Config{
				Username:   "test-user",
				Password:   "test-password",
				TenantName: "test-tenant",
				DomainName: "test-domain",
			},
			handler:   HandlerFuncInvalidPortID,
			wantError: true,
		},
		{
			name: "InvalidResponse",
			cfg: Config{
				Username:   "test-user",
				Password:   "test-password",
				TenantName: "test-tenant",
				DomainName: "test-domain",
			},
			handler:   HandlerFuncInvalidResponse,
			wantError: true,
		},
		{
			name: "Success",
			cfg: Config{
				Username:   "test-user",
				Password:   "test-password",
				TenantName: "test-tenant",
				DomainName: "test-domain",
			},
			handler:   HandlerFuncAssignFloatingIPSuccess,
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var networkClient *gophercloud.ServiceClient

			if tt.name == "InvalidNetworkClient" {

				networkClient = &gophercloud.ServiceClient{
					ProviderClient: &gophercloud.ProviderClient{},
					Endpoint:       "http://bad-endpoint/",
				}
			} else {
				mux := http.NewServeMux()

				mux.HandleFunc("/v3/auth/tokens", HandlerFuncV3)
				mux.HandleFunc("/v2.0/tokens", HandlerFuncV2)

				mux.HandleFunc("/v2.0/v2.0/floatingips", tt.handler)

				server := httptest.NewServer(mux)
				defer server.Close()

				cfg := tt.cfg
				cfg.IdentityEndpoint = server.URL + "/v3"

				providerClient, err := NewProviderClient(cfg)
				if err != nil {
					t.Fatalf("Failed to create provider client: %v", err)
				}

				networkClient, err = NewNetworkClient(providerClient, gophercloud.EndpointOpts{
					Region: "test-region",
				})
				if err != nil {
					t.Fatalf("Failed to create network client: %v", err)
				}
			}

			portID := "test-port-id"
			floatingNetworkID := "test-floating-network-id"

			floatingIP, floatingIPID, err := AssignFloatingIP(context.Background(), networkClient, portID, floatingNetworkID)

			if tt.wantError {
				if err == nil {
					t.Errorf("Expected error for test case %v, but got none", tt.name)
				} else {
					t.Logf("Expected error occurred for test case %v: %v", tt.name, err)
				}
				if floatingIP.IsValid() {
					t.Errorf("Expected floatingIP to be empty when error occurs for test case %v, but got: %v", tt.name, floatingIP)
				}
				if floatingIPID != "" {
					t.Errorf("Expected floatingIPID to be empty when error occurs for test case %v, but got: %v", tt.name, floatingIPID)
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error for test case %v, but got: %v", tt.name, err)
				} else {
					t.Logf("Successfully assigned floating IP for test case %v: IP=%v, ID=%v", tt.name, floatingIP, floatingIPID)
				}
				if !floatingIP.IsValid() {
					t.Errorf("Expected floatingIP to have valid value when no error occurs for test case %v, but got invalid: %v", tt.name, floatingIP)
				}
				if floatingIPID == "" {
					t.Errorf("Expected floatingIPID to have value when no error occurs for test case %v, but got empty: %v", tt.name, floatingIPID)
				}
			}
		})
	}
}

func TestGetPortID(t *testing.T) {
	tests := []struct {
		name      string
		cfg       Config
		handler   http.HandlerFunc
		wantError bool
	}{
		{
			name: "InvalidComputeClient",
			cfg: Config{
				Username:   "test-user",
				Password:   "test-password",
				TenantName: "test-tenant",
				DomainName: "test-domain",
			},
			handler:   HandlerFuncGetPortIDInvalidComputeClient,
			wantError: true,
		},
		{
			name: "InvalidServerID",
			cfg: Config{
				Username:   "test-user",
				Password:   "test-password",
				TenantName: "test-tenant",
				DomainName: "test-domain",
			},
			handler:   HandlerFuncGetPortIDInvalidServerID,
			wantError: true,
		},
		{
			name: "InvalidFixedIP",
			cfg: Config{
				Username:   "test-user",
				Password:   "test-password",
				TenantName: "test-tenant",
				DomainName: "test-domain",
			},
			handler:   HandlerFuncGetPortIDInvalidFixedIP,
			wantError: true,
		},
		{
			name: "FixedIPPortIDNotLinked",
			cfg: Config{
				Username:   "test-user",
				Password:   "test-password",
				TenantName: "test-tenant",
				DomainName: "test-domain",
			},
			handler:   HandlerFuncGetPortIDNotLinked,
			wantError: true,
		},
		{
			name: "Success",
			cfg: Config{
				Username:   "test-user",
				Password:   "test-password",
				TenantName: "test-tenant",
				DomainName: "test-domain",
			},
			handler:   HandlerFuncGetPortIDSuccess,
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var computeClient *gophercloud.ServiceClient

			if tt.name == "InvalidComputeClient" {
				computeClient = &gophercloud.ServiceClient{
					ProviderClient: &gophercloud.ProviderClient{},
					Endpoint:       "http://bad-endpoint/",
				}
			} else {
				mux := http.NewServeMux()

				mux.HandleFunc("/v3/auth/tokens", HandlerFuncV3)
				mux.HandleFunc("/v2.0/tokens", HandlerFuncV2)

				mux.HandleFunc("/v2.0/test-tenant/servers/test-server-id/os-interface", tt.handler)

				server := httptest.NewServer(mux)
				defer server.Close()

				cfg := tt.cfg
				cfg.IdentityEndpoint = server.URL + "/v3"

				providerClient, err := NewProviderClient(cfg)
				if err != nil {
					t.Fatalf("Failed to create provider client: %v", err)
				}

				computeClient, err = NewComputeClient(providerClient, gophercloud.EndpointOpts{
					Region: "test-region",
				})
				if err != nil {
					t.Fatalf("Failed to create compute client: %v", err)
				}
			}

			serverID := "test-server-id"
			fixedIP := "192.168.1.100"

			portID := GetPortID(computeClient, serverID, fixedIP)

			if tt.wantError {
				if portID != "" {
					t.Errorf("Expected empty port ID for test case %v, but got: %v", tt.name, portID)
				} else {
					t.Logf("Expected empty port ID returned for test case %v", tt.name)
				}
			} else {
				if portID == "" {
					t.Errorf("Expected non-empty port ID for test case %v, but got empty string", tt.name)
				} else {
					t.Logf("Successfully retrieved port ID for test case %v: %v", tt.name, portID)
				}
			}
		})
	}
}

func TestDeleteFloatingIP(t *testing.T) {
	tests := []struct {
		name            string
		cfg             Config
		handler         http.HandlerFunc
		wantLogContains string
	}{
		{
			name: "InvalidNetworkClient",
			cfg: Config{
				Username:   "test-user",
				Password:   "test-password",
				TenantName: "test-tenant",
				DomainName: "test-domain",
			},
			handler:         HandlerFuncDeleteFloatingIPInvalidNetworkClient,
			wantLogContains: "failed to delete floating IP test-floating-ip-id",
		},
		{
			name: "InvalidFloatingIPID",
			cfg: Config{
				Username:   "test-user",
				Password:   "test-password",
				TenantName: "test-tenant",
				DomainName: "test-domain",
			},
			handler:         HandlerFuncDeleteFloatingIPInvalidID,
			wantLogContains: "failed to delete floating IP test-floating-ip-id",
		},
		{
			name: "Success",
			cfg: Config{
				Username:   "test-user",
				Password:   "test-password",
				TenantName: "test-tenant",
				DomainName: "test-domain",
			},
			handler:         HandlerFuncDeleteFloatingIPSuccess,
			wantLogContains: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var logBuffer bytes.Buffer
			oldOutput := log.Writer()
			log.SetOutput(&logBuffer)
			defer func() {
				log.SetOutput(oldOutput)
			}()

			var networkClient *gophercloud.ServiceClient

			if tt.name == "InvalidNetworkClient" {
				networkClient = &gophercloud.ServiceClient{
					ProviderClient: &gophercloud.ProviderClient{},
					Endpoint:       "http://bad-endpoint/",
				}
			} else {
				mux := http.NewServeMux()

				mux.HandleFunc("/v3/auth/tokens", HandlerFuncV3)
				mux.HandleFunc("/v2.0/tokens", HandlerFuncV2)

				mux.HandleFunc("/v2.0/v2.0/floatingips/test-floating-ip-id", tt.handler)

				server := httptest.NewServer(mux)
				defer server.Close()

				cfg := tt.cfg
				cfg.IdentityEndpoint = server.URL + "/v3"

				providerClient, err := NewProviderClient(cfg)
				if err != nil {
					t.Fatalf("Failed to create provider client: %v", err)
				}

				networkClient, err = NewNetworkClient(providerClient, gophercloud.EndpointOpts{
					Region: "test-region",
				})
				if err != nil {
					t.Fatalf("Failed to create network client: %v", err)
				}
			}

			floatingIPID := "test-floating-ip-id"

			err := DeleteFloatingIP(context.Background(), networkClient, floatingIPID)

			if err != nil {
				t.Errorf("Expected DeleteFloatingIP to always return nil, but got: %v", err)
			}

			logOutput := logBuffer.String()
			if tt.wantLogContains != "" {
				if !strings.Contains(logOutput, tt.wantLogContains) {
					t.Errorf("Expected log to contain '%s', but got: %s", tt.wantLogContains, logOutput)
				} else {
					t.Logf("Expected error log found for test case %v: %s", tt.name, strings.TrimSpace(logOutput))
				}
			} else {
				if strings.Contains(logOutput, "failed to delete floating IP") {
					t.Errorf("Expected no error log for test case %v, but got: %s", tt.name, logOutput)
				} else {
					t.Logf("Successfully deleted floating IP for test case %v (no error log as expected)", tt.name)
				}
			}
		})
	}
}

func CreateServer() *httptest.Server {
	mux := http.NewServeMux()

	mux.HandleFunc("/v3/auth/tokens", HandlerFuncV3)
	mux.HandleFunc("/v2.0/tokens", HandlerFuncV2)

	return httptest.NewServer(mux)
}

func CreateServerNoCompute() *httptest.Server {
	mux := http.NewServeMux()

	mux.HandleFunc("/v3/auth/tokens", HandlerFuncV3NoCompute)
	mux.HandleFunc("/v2.0/tokens", HandlerFuncV2NoCompute)

	return httptest.NewServer(mux)
}

func CreateServerNoNetwork() *httptest.Server {
	mux := http.NewServeMux()

	mux.HandleFunc("/v3/auth/tokens", HandlerFuncV3NoNetwork)
	mux.HandleFunc("/v2.0/tokens", HandlerFuncV2NoNetwork)

	return httptest.NewServer(mux)
}

func HandlerFuncV3(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusCreated)
	w.Write([]byte(`{
                                "token": {
                                        "id": "01234567890",
                                        "expires": "2014-10-01T10:00:00.000000Z",
                                        "catalog": [
                                                {
                                                        "id": "a1b2c3d4-e5f6-7890-1234-567890abcdef",
                                                        "name": "nova",
                                                        "type": "compute",
                                                        "endpoints": [
                                                                {
                                                                        "id": "1a2b3c4d-5e6f-7890-1234-567890abcdef",
                                                                        "interface": "public",
                                                                        "region": "test-region",
                                                                        "url": "http://` + r.Host + `/v2.0/test-tenant/"
                                                                }
                                                        ]
                                                },
                                                {
                                                        "id": "b2c3d4e5-f6a7-8901-2345-67890abcdef0",
                                                        "name": "neutron",
                                                        "type": "network",
                                                        "endpoints": [
                                                                {
                                                                        "id": "2b3c4d5e-6f7a-8901-2345-67890abcdef0",
                                                                        "interface": "public",
                                                                        "region": "test-region",
                                                                        "url": "http://` + r.Host + `/v2.0/"
                                                                }
                                                        ]
                                                }
                                        ]
                                }
                        }`))
}

func HandlerFuncV2(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{
                                "access": {
                                        "token": {
                                                "id": "01234567890",
                                                "expires": "2014-10-01T10:00:00.000000Z"
                                        },
                                        "serviceCatalog": [
                                                {
                                                        "name": "Cloud Servers",
                                                        "type": "compute",
                                                        "endpoints": [
                                                                {
                                                                        "tenantId": "test-tenant",
                                                                        "publicURL": "http://` + r.Host + `/v2.0/test-tenant/",
                                                                        "region": "test-region"
                                                                }
                                                        ],
                                                        "endpoints_links": []
                                                },
                                                {
                                                        "name": "Neutron",
                                                        "type": "network",
                                                        "endpoints": [
                                                                {
                                                                        "tenantId": "test-tenant",
                                                                        "publicURL": "http://` + r.Host + `/v2.0/",
                                                                        "region": "test-region"
                                                                }
                                                        ],
                                                        "endpoints_links": []
                                                }
                                        ]
                                }
                        }`))
}

func HandlerFuncV2NoCompute(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{
        "access": {
            "token": {
                "id": "01234567890",
                "expires": "2014-10-01T10:00:00.000000Z"
            },
            "serviceCatalog": [
                {
                    "name": "Neutron",
                    "type": "network",
                    "endpoints": [
                        {
                            "tenantId": "test-tenant",
                            "publicURL": "http://` + r.Host + `/v2.0/",
                            "region": "test-region"
                        }
                    ],
                    "endpoints_links": []
                }
            ]
        }
    }`))
}

func HandlerFuncV3NoCompute(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusCreated)
	w.Write([]byte(`{
                                "token": {
                                        "id": "01234567890",
                                        "expires": "2014-10-01T10:00:00.000000Z",
                                        "catalog": [
                                                {
                                                        "id": "b2c3d4e5-f6a7-8901-2345-67890abcdef0",
                                                        "name": "neutron",
                                                        "type": "network",
                                                        "endpoints": [
                                                                {
                                                                        "id": "2b3c4d5e-6f7a-8901-2345-67890abcdef0",
                                                                        "interface": "public",
                                                                        "region": "test-region",
                                                                        "url": "http://` + r.Host + `/v2.0/"
                                                                }
                                                        ]
                                                }
                                        ]
                                }
                        }`))
}

func HandlerFuncV2NoNetwork(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{
                "access": {
                        "token": {
                                "id": "01234567890",
                                "expires": "2014-10-01T10:00:00.000000Z"
                        },
                        "serviceCatalog": [
                                {
                                        "name": "Cloud Servers",
                                        "type": "compute",
                                        "endpoints": [
                                                {
                                                        "tenantId": "test-tenant",
                                                        "publicURL": "http://` + r.Host + `/v2.0/test-tenant/",
                                                        "region": "test-region"
                                                }
                                        ],
                                        "endpoints_links": []
                                }
                        ]
                }
        }`))
}

func HandlerFuncV3NoNetwork(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusCreated)
	w.Write([]byte(`{
                                "token": {
                                        "id": "01234567890",
                                        "expires": "2014-10-01T10:00:00.000000Z",
                                        "catalog": [
                                                {
                                                        "id": "a1b2c3d4-e5f6-7890-1234-567890abcdef",
                                                        "name": "nova",
                                                        "type": "compute",
                                                        "endpoints": [
                                                                {
                                                                        "id": "1a2b3c4d-5e6f-7890-1234-567890abcdef",
                                                                        "interface": "public",
                                                                        "region": "test-region",
                                                                        "url": "http://` + r.Host + `/v2.0/test-tenant/"
                                                                }
                                                        ]
                                                }
                                        ]
                                }
                        }`))
}

func HandlerFuncNoCompute(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusServiceUnavailable)
	w.Write([]byte(`{
                "error": {
                        "message": "Compute service is temporarily unavailable",
                        "code": 503,
                        "type": "ServiceUnavailable"
                }
        }`))
}

func HandlerFuncInvalidServerID(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotFound)
	w.Write([]byte(`{
                "itemNotFound": {
                        "message": "Instance test-server-id could not be found.",
                        "code": 404
                }
        }`))
}

func HandlerFuncExceedsMaxRetries(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{
                "server": {
                        "id": "test-server-id",
                        "name": "test-server",
                        "status": "BUILD",
                        "addresses": {}
                }
        }`))
}

func HandlerFuncInvalidServerAddress(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{
                "server": {
                        "id": "test-server-id",
                        "name": "test-server",
                        "status": "ACTIVE",
                        "addresses": {
                                "private": [
                                        {
                                                "addr": "xxxxxxxxxxx",
                                                "version": 4,
                                                "type": "fixed"
                                        }
                                ]
                        }
                }
        }`))
}

var requestCount int32

func HandlerFuncRetriesAndSucceeds(w http.ResponseWriter, r *http.Request) {
	retriesCount := atomic.AddInt32(&requestCount, 1)
	w.Header().Set("Content-Type", "application/json")

	if retriesCount <= 3 {
		log.Printf("Retry attempt %d: Server status BUILD (not ready yet)", retriesCount)
		w.Write([]byte(`{
                        "server": {
                                "id": "test-server-id",
                                "name": "test-server",
                                "status": "BUILD",
                                "addresses": {}
                        }
                }`))
	} else {
		log.Printf("Retry attempt %d: Server status ACTIVE (ready with IP addresses)", retriesCount)
		w.Write([]byte(`{
                "server": {
                        "id": "test-server-id",
                        "name": "test-server",
                        "status": "ACTIVE",
                        "addresses": {
                                "private": [
                                        {
                                                "addr": "172.24.4.9",
                                                "version": 4,
                                                "type": "fixed"
                                        }
                                ]
                        }
                }
        }`))
	}
}

func HandlerFuncSucceedsWithoutRetries(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{
                "server": {
                        "id": "test-server-id",
                        "name": "test-server",
                        "status": "ACTIVE",
                        "addresses": {
                                "private": [
                                        {
                                                "addr": "10.0.0.10",
                                                "version": 4,
                                                "type": "fixed"
                                        }
                                ],
                                "public": [
                                        {
                                                "addr": "172.24.4.10",
                                                "version": 4,
                                                "type": "fixed"
                                        }
                                ]
                        }
                }
        }`))
}

func HandlerFuncInvalidNetworkClient(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	w.Write([]byte(`{
                "error": {
                        "message": "Invalid network client credentials",
                        "code": 401
                }
        }`))
}

func HandlerFuncInvalidFloatingNetworkID(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotFound)
	w.Write([]byte(`{
                "NeutronError": {
                        "message": "Network invalid-floating-network-id could not be found",
                        "type": "NetworkNotFound",
                        "detail": ""
                }
        }`))
}

func HandlerFuncInvalidPortID(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotFound)
	w.Write([]byte(`{
                "NeutronError": {
                        "message": "Port invalid-port-id could not be found",
                        "type": "PortNotFound",
                        "detail": ""
                }
        }`))
}

func HandlerFuncInvalidResponse(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadRequest)
	w.Write([]byte(`{
                "badRequest": {
                        "message": "Invalid request format",
                        "code": 400
                }
        }`))
}

func HandlerFuncAssignFloatingIPSuccess(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method == "POST" {
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{
                        "floatingip": {
                                "id": "test-floating-ip-id",
                                "floating_ip_address": "203.0.113.100",
                                "port_id": "test-port-id",
                                "status": "ACTIVE"
                        }
                }`))
	} else {
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func HandlerFuncGetPortIDInvalidComputeClient(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	w.Write([]byte(`{
                "error": {
                        "message": "Invalid compute client credentials",
                        "code": 401
                }
        }`))
}

func HandlerFuncGetPortIDInvalidServerID(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotFound)
	w.Write([]byte(`{
                "itemNotFound": {
                        "message": "Instance invalid-server-id could not be found.",
                        "code": 404
                }
        }`))
}

func HandlerFuncGetPortIDInvalidFixedIP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadRequest)
	w.Write([]byte(`{
                "badRequest": {
                        "message": "Invalid IP address format",
                        "code": 400
                }
        }`))
}

func HandlerFuncGetPortIDNotLinked(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotFound)
	w.Write([]byte(`{
                "itemNotFound": {
                        "message": "No port found for the specified fixed IP address",
                        "code": 404
                }
        }`))
}

func HandlerFuncGetPortIDSuccess(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{
                "interfaceAttachments": [
                        {
                                "port_id": "test-port-id-12345",
                                "fixed_ips": [
                                        {
                                                "ip_address": "192.168.1.100",
                                                "subnet_id": "test-subnet-id"
                                        }
                                ],
                                "net_id": "test-network-id",
                                "port_state": "ACTIVE"
                        }
                ]
        }`))
}

func HandlerFuncDeleteFloatingIPInvalidNetworkClient(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	w.Write([]byte(`{
                "error": {
                        "message": "Invalid network client credentials",
                        "code": 401
                }
        }`))
}

func HandlerFuncDeleteFloatingIPInvalidID(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotFound)
	w.Write([]byte(`{
                "itemNotFound": {
                        "message": "Floating IP invalid-floating-ip-id could not be found",
                        "code": 404
                }
        }`))
}

func HandlerFuncDeleteFloatingIPSuccess(w http.ResponseWriter, r *http.Request) {
	if r.Method == "DELETE" {
		w.WriteHeader(http.StatusNoContent)
	} else {
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}
