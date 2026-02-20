// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package openstack

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	provider "github.com/confidential-containers/cloud-api-adaptor/src/cloud-providers"
	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-providers/util/cloudinit"
	"github.com/gophercloud/gophercloud/v2"
)

func TestNewProvider(t *testing.T) {
	server := CreateServer()
	serverNoNetwork := CreateServerNoNetwork()

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
				Region:           "test-region",
			},
			wantError: false,
		},
		{
			name: "InvalidEndpoint",
			cfg: Config{
				IdentityEndpoint: "http://bad-address.example.com/v3",
				Username:         "test-user",
				Password:         "test-password",
				TenantName:       "test-tenant",
				DomainName:       "test-domain",
				Region:           "test-region",
			},
			wantError: true,
		},
		{
			name: "InvalidRegion",
			cfg: Config{
				IdentityEndpoint: server.URL + "/v3",
				Username:         "test-user",
				Password:         "test-password",
				TenantName:       "test-tenant",
				DomainName:       "test-domain",
				Region:           "bad-region",
			},
			wantError: true,
		},
		{
			name: "InvalidNetworkClient",
			cfg: Config{
				IdentityEndpoint: serverNoNetwork.URL + "/v3",
				Username:         "test-user",
				Password:         "test-password",
				TenantName:       "test-tenant",
				DomainName:       "test-domain",
				Region:           "test-region",
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider, err := NewProvider(&tt.cfg)
			if tt.wantError {
				if err == nil {
					t.Errorf("Expected error for test case %v, but got none", tt.name)
				} else {
					t.Logf("Expected error occurred for test case %v: %v", tt.name, err)
				}

				if provider != nil {
					t.Errorf("Expected provider to be nil for test case %v, but got: %+v", tt.name, provider)
				}

				switch tt.name {
				case "InvalidEndpoint":
					providerClient, err := NewProviderClient(tt.cfg)
					if err == nil {
						t.Errorf("Expected ProviderClient creation to fail for %v, but got no error", tt.name)
					} else {
						t.Logf("ProviderClient creation failed as expected for %v: %v", tt.name, err)
					}
					if providerClient != nil {
						t.Errorf("Expected ProviderClient to be nil for %v, but got: %+v", tt.name, providerClient)
					}

				case "InvalidRegion":
					providerClient, err := NewProviderClient(tt.cfg)
					if err != nil {
						t.Errorf("Expected ProviderClient creation to succeed for %v, but got error: %v", tt.name, err)
					} else {
						t.Logf("ProviderClient creation succeeded for %v", tt.name)
					}
					if providerClient == nil {
						t.Errorf("Expected ProviderClient to be non-nil for %v, but got nil", tt.name)
						return
					}

					computeClient, err := NewComputeClient(providerClient, gophercloud.EndpointOpts{Region: tt.cfg.Region})
					if err == nil {
						t.Errorf("Expected ComputeClient creation to fail for %v, but got no error", tt.name)
					} else {
						t.Logf("ComputeClient creation failed as expected for %v: %v", tt.name, err)
					}
					if computeClient != nil {
						t.Errorf("Expected ComputeClient to be nil for %v, but got: %+v", tt.name, computeClient)
					}

				case "InvalidNetworkClient":
					providerClient, err := NewProviderClient(tt.cfg)
					if err != nil {
						t.Errorf("Expected ProviderClient creation to succeed for %v, but got error: %v", tt.name, err)
					} else {
						t.Logf("ProviderClient creation succeeded for %v", tt.name)
					}
					if providerClient == nil {
						t.Errorf("Expected ProviderClient to be non-nil for %v, but got nil", tt.name)
						return
					}

					computeClient, err := NewComputeClient(providerClient, gophercloud.EndpointOpts{Region: tt.cfg.Region})
					if err != nil {
						t.Errorf("Expected ComputeClient creation to succeed for %v, but got error: %v", tt.name, err)
					} else {
						t.Logf("ComputeClient creation succeeded for %v", tt.name)
					}
					if computeClient == nil {
						t.Errorf("Expected ComputeClient to be non-nil for %v, but got nil", tt.name)
						return
					}

					networkClient, err := NewNetworkClient(providerClient, gophercloud.EndpointOpts{Region: tt.cfg.Region})
					if err == nil {
						t.Errorf("Expected NetworkClient creation to fail for %v, but got no error", tt.name)
					} else {
						t.Logf("NetworkClient creation failed as expected for %v: %v", tt.name, err)
					}
					if networkClient != nil {
						t.Errorf("Expected NetworkClient to be nil for %v, but got: %+v", tt.name, networkClient)
					}
				}

			} else {
				if err != nil {
					t.Errorf("Expected no error for test case %v, but got: %v", tt.name, err)
				}

				if provider == nil {
					t.Errorf("Expected provider to be non-nil for test case %v, but got nil", tt.name)
					return
				}

				openstackProvider, ok := provider.(*openstackProvider)
				if !ok {
					t.Errorf("Expected provider to be of type *openstackProvider for test case %v, but got: %T", tt.name, provider)
					return
				}

				if openstackProvider.providerClient == nil {
					t.Errorf("Expected providerClient to be non-nil for test case %v", tt.name)
				}
				if openstackProvider.computeClient == nil {
					t.Errorf("Expected computeClient to be non-nil for test case %v", tt.name)
				}
				if openstackProvider.networkClient == nil {
					t.Errorf("Expected networkClient to be non-nil for test case %v", tt.name)
				}
				if openstackProvider.serviceConfig == nil {
					t.Errorf("Expected serviceConfig to be non-nil for test case %v", tt.name)
				}

				if openstackProvider.serviceConfig != nil {
					if openstackProvider.serviceConfig.IdentityEndpoint != tt.cfg.IdentityEndpoint {
						t.Errorf("Expected IdentityEndpoint %v, but got %v for test case %v", tt.cfg.IdentityEndpoint, openstackProvider.serviceConfig.IdentityEndpoint, tt.name)
					}
					if openstackProvider.serviceConfig.Username != tt.cfg.Username {
						t.Errorf("Expected Username %v, but got %v for test case %v", tt.cfg.Username, openstackProvider.serviceConfig.Username, tt.name)
					}
					if openstackProvider.serviceConfig.Password != tt.cfg.Password {
						t.Errorf("Expected Password %v, but got %v for test case %v", tt.cfg.Password, openstackProvider.serviceConfig.Password, tt.name)
					}
					if openstackProvider.serviceConfig.TenantName != tt.cfg.TenantName {
						t.Errorf("Expected TenantName %v, but got %v for test case %v", tt.cfg.TenantName, openstackProvider.serviceConfig.TenantName, tt.name)
					}
					if openstackProvider.serviceConfig.DomainName != tt.cfg.DomainName {
						t.Errorf("Expected DomainName %v, but got %v for test case %v", tt.cfg.DomainName, openstackProvider.serviceConfig.DomainName, tt.name)
					}
					if openstackProvider.serviceConfig.Region != tt.cfg.Region {
						t.Errorf("Expected Region %v, but got %v for test case %v", tt.cfg.Region, openstackProvider.serviceConfig.Region, tt.name)
					}
				}
			}
		})
	}
}

type mockCloudConfig struct{}

func (c *mockCloudConfig) Generate() (string, error) {
	return "cloud config", nil
}

type errorCloudConfig struct{}

func (c *errorCloudConfig) Generate() (string, error) {
	return "", fmt.Errorf("invalid cloud config")
}

func TestCreateInstance(t *testing.T) {
	tests := []struct {
		name        string
		config      Config
		networks    []string
		cloudConfig cloudinit.CloudConfigGenerator
		handler     http.HandlerFunc
		getHandler  http.HandlerFunc
		wantError   bool
	}{
		{
			name: "ValidConfig",
			config: Config{
				IdentityEndpoint: "",
				Username:         "test-user",
				Password:         "test-password",
				TenantName:       "test-tenant",
				DomainName:       "test-domain",
			},
			networks:    nil,
			cloudConfig: &mockCloudConfig{},
			handler:     HandlerFuncServers,
			getHandler:  HandlerFuncServersGetSuccess,
			wantError:   false,
		},
		{
			name: "NetworksEmpty",
			config: Config{
				IdentityEndpoint: "",
				Username:         "test-user",
				Password:         "test-password",
				TenantName:       "test-tenant",
				DomainName:       "test-domain",
			},
			networks:    []string{},
			cloudConfig: &mockCloudConfig{},
			handler:     HandlerFuncServersNetwork,
			getHandler:  HandlerFuncServersGetNetwork,
			wantError:   false,
		},
		{
			name: "NetworksSet",
			config: Config{
				IdentityEndpoint: "",
				Username:         "test-user",
				Password:         "test-password",
				TenantName:       "test-tenant",
				DomainName:       "test-domain",
			},
			networks:    []string{"test_net"},
			cloudConfig: &mockCloudConfig{},
			handler:     HandlerFuncServersNetwork,
			getHandler:  HandlerFuncServersGetNetworkSet,
			wantError:   false,
		},
		{
			name: "InvalidCloudConfig",
			config: Config{
				IdentityEndpoint: "",
				Username:         "test-user",
				Password:         "test-password",
				TenantName:       "test-tenant",
				DomainName:       "test-domain",
			},
			networks:    nil,
			cloudConfig: &errorCloudConfig{},
			handler:     HandlerFuncServers,
			getHandler:  nil,
			wantError:   true,
		},
		{
			name: "ServersCreateError",
			config: Config{
				IdentityEndpoint: "",
				Username:         "test-user",
				Password:         "test-password",
				TenantName:       "test-tenant",
				DomainName:       "test-domain",
			},
			networks:    nil,
			cloudConfig: &mockCloudConfig{},
			handler:     HandlerFuncServersError,
			getHandler:  nil,
			wantError:   true,
		},
		{
			name: "InvalidServerAddress",
			config: Config{
				IdentityEndpoint: "",
				Username:         "test-user",
				Password:         "test-password",
				TenantName:       "test-tenant",
				DomainName:       "test-domain",
			},
			networks:    nil,
			cloudConfig: &mockCloudConfig{},
			handler:     HandlerFuncServersInvalidAddr,
			getHandler:  HandlerFuncServersGetInvalidAddr,
			wantError:   true,
		},
		{
			name: "GetFixedIPError",
			config: Config{
				IdentityEndpoint: "",
				Username:         "test-user",
				Password:         "test-password",
				TenantName:       "test-tenant",
				DomainName:       "test-domain",
			},
			networks:    nil,
			cloudConfig: &mockCloudConfig{},
			handler:     HandlerFuncServersCreateFixedIPError,
			getHandler:  HandlerFuncServersGetFixedIPError,
			wantError:   true,
		},
		{
			name: "AssignFloatingIPError",
			config: Config{
				IdentityEndpoint: "",
				Username:         "test-user",
				Password:         "test-password",
				TenantName:       "test-tenant",
				DomainName:       "test-domain",
			},
			networks:    nil,
			cloudConfig: &mockCloudConfig{},
			handler:     HandlerFuncServersAssignFloatingIPError,
			getHandler:  nil,
			wantError:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mux := http.NewServeMux()
			mux.HandleFunc("/v2.0/test-tenant/servers", tt.handler)
			mux.HandleFunc("/v2.0/tokens", HandlerFuncV2)
			mux.HandleFunc("/v3/auth/tokens", HandlerFuncV3)

			if tt.getHandler != nil {
				mux.HandleFunc("/v2.0/test-tenant/servers/test-server-id-12345", tt.getHandler)
			}

			if tt.name == "NetworksEmpty" || tt.name == "NetworksSet" {
				if tt.name == "NetworksSet" {
					mux.HandleFunc("/v2.0/test-tenant/servers/test-server-id-12345/os-interface", HandlerFuncOSInterfaceNetworkSet)
				} else {
					mux.HandleFunc("/v2.0/test-tenant/servers/test-server-id-12345/os-interface", HandlerFuncOSInterfaceNetwork)
				}
			} else {
				mux.HandleFunc("/v2.0/test-tenant/servers/test-server-id-12345/os-interface", HandlerFuncOSInterface)
			}

			server := httptest.NewServer(mux)
			defer server.Close()

			cfg := tt.config
			cfg.IdentityEndpoint = server.URL + "/v3"
			cfg.NetworkIDs = tt.networks

			testProvider, err := NewProvider(&cfg)
			if err != nil {
				t.Fatalf("Expected provider to be created, but got error: %v", err)
			}

			podname := "test-vm"
			sandboxID := "12345"
			spec := provider.InstanceTypeSpec{}
			instance, err := testProvider.CreateInstance(context.Background(), podname, sandboxID, tt.cloudConfig, spec)

			if tt.wantError {
				if err == nil {
					t.Errorf("Expected error for test case %v, but got none", tt.name)
				} else if instance != nil {
					t.Errorf("Expected instance to be nil for test case %v, but got: %+v", tt.name, instance)
				} else {
					t.Logf("Expected error occurred for test case %v: %v", tt.name, err)
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error for test case %v, but got: %v", tt.name, err)
				} else if instance == nil {
					t.Errorf("Expected instance to be created for test case %v, but got nil", tt.name)
				} else {
					t.Logf("Instance successfully created for test case %v: %+v", tt.name, instance)
				}
			}
		})
	}
}

func TestDeleteInstance(t *testing.T) {
	tests := []struct {
		name      string
		handler   http.HandlerFunc
		wantError bool
	}{
		{
			name:      "DeleteOK",
			handler:   HandlerFuncDeleteInstanceOK,
			wantError: false,
		},
		{
			name:      "AlreadyDeleted",
			handler:   HandlerFuncDeleteInstanceAlreadyDeleted,
			wantError: false,
		},
		{
			name:      "FloatingIPNotAssigned",
			handler:   HandlerFuncDeleteInstanceFloatingIPNotAssigned,
			wantError: false,
		},
		{
			name:      "DeleteError",
			handler:   HandlerFuncDeleteInstanceError,
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mux := http.NewServeMux()
			mux.HandleFunc("/v2.0/test-tenant/servers/12345", tt.handler)
			mux.HandleFunc("/v2.0/tokens", HandlerFuncV2)
			mux.HandleFunc("/v3/auth/tokens", HandlerFuncV3)
			server := httptest.NewServer(mux)
			defer server.Close()

			openstackcfg := Config{
				IdentityEndpoint: server.URL + "/v3",
				Username:         "test-user",
				Password:         "test-password",
				TenantName:       "test-tenant",
				DomainName:       "test-domain",
			}

			deleteProvider, err := NewProvider(&openstackcfg)
			if err != nil {
				t.Fatalf("Expected provider to be created, but got error: %v", err)
			} else if deleteProvider == nil {
				t.Fatalf("Expected provider to be created, but got nil")
			} else {
				t.Logf("Provider successfully created: %+v", deleteProvider)
			}

			instanceID := "12345"

			if tt.name == "DeleteOK" || tt.name == "DeleteError" {
				openstackProvider, ok := deleteProvider.(*openstackProvider)
				if !ok {
					t.Fatalf("Expected provider to be of type *openstackProvider, but got: %T", deleteProvider)
				}
				openstackProvider.floatingIPPool[instanceID] = "test-floating-ip-id-12345"

				mux.HandleFunc("/v2.0/v2.0/floatingips/test-floating-ip-id-12345", HandlerFuncDeleteFloatingIPSuccess)
			}

			err = deleteProvider.DeleteInstance(context.Background(), instanceID)

			if tt.wantError {
				if err == nil {
					t.Errorf("Expected error for test case %v, but got none", tt.name)
				} else {
					t.Logf("Expected error occurred for test case %v: %v", tt.name, err)
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error for test case %v, but got: %v", tt.name, err)
				} else {
					t.Logf("DeleteInstance succeeded for test case: %v", tt.name)
				}
			}
		})
	}
}

func TestTeardown(t *testing.T) {
	server := CreateServer()

	openstackcfg := Config{
		IdentityEndpoint: server.URL + "/v3",
		Username:         "test-user",
		Password:         "test-password",
		TenantName:       "test-tenant",
		DomainName:       "test-domain",
	}

	provider, err := NewProvider(&openstackcfg)

	if err != nil {
		t.Fatalf("Expected provider to be created, but got error: %v", err)
	} else if provider == nil {
		t.Fatalf("Expected provider to be created, but got nil")
	} else {
		t.Logf("Provider successfully created: %+v", provider)
	}

	err = provider.Teardown()

	if err != nil {
		t.Errorf("Expected no error in Teardown, but got: %v", err)
	} else {
		t.Logf("Teardown succeeded for test case: %v", t.Name())
	}
}

func TestConfigVerifier(t *testing.T) {
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
				ImageID:          "test-image-id",
			},
			wantError: false,
		},
		{
			name: "EmptyImageID",
			cfg: Config{
				IdentityEndpoint: server.URL + "/v3",
				Username:         "test-user",
				Password:         "test-password",
				TenantName:       "test-tenant",
				DomainName:       "test-domain",
				ImageID:          "",
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider, err := NewProvider(&tt.cfg)
			if err != nil {
				t.Fatalf("Expected provider to be created, but got error: %v", err)
			} else if provider == nil {
				t.Fatalf("Expected provider to be created, but got nil")
			} else {
				t.Logf("Provider successfully created for test case %v", tt.name)
			}

			err = provider.ConfigVerifier()

			if tt.wantError {
				if err == nil {
					t.Errorf("Expected error for test case %v, but got none", tt.name)
				} else {
					t.Logf("Expected error occurred for test case %v: %v", tt.name, err)
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error for test case %v, but got: %v", tt.name, err)
				} else {
					t.Logf("ConfigVerifier succeeded for test case: %v", tt.name)
				}
			}
		})
	}
}

func HandlerFuncServers(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method == "POST" {
		w.WriteHeader(http.StatusAccepted)
		w.Write([]byte(`{
            "server": {
                "id": "test-server-id-12345",
                "name": "test-vm",
                "status": "BUILD",
                "created": "2024-11-20T12:00:00Z",
                "hostId": "",
                "progress": 0,
                "accessIPv4": "",
                "accessIPv6": "",
                "image": {
                    "id": "test-image"
                },
                "flavor": {
                    "id": "test-flavor"
                },
                "addresses": {},
                "metadata": {},
                "links": []
            }
        }`))
	} else {
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func HandlerFuncServersError(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method == "POST" {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error": "internal server error"}`))
	} else {
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func HandlerFuncServersInvalidAddr(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method == "POST" {
		w.WriteHeader(http.StatusAccepted)
		w.Write([]byte(`{
            "server": {
                "id": "test-server-id-12345",
                "name": "test-vm",
                "status": "BUILD",
                "created": "2024-11-20T12:00:00Z",
                "hostId": "",
                "progress": 0,
                "accessIPv4": "",
                "accessIPv6": "",
                "image": {
                    "id": "test-image"
                },
                "flavor": {
                    "id": "test-flavor"
                },
                "addresses": {"private": [{"addr": "invalid_ip"}]},
                "metadata": {},
                "links": []
            }
        }`))
	} else {
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func HandlerFuncServersNetwork(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method == "POST" {
		var reqBody map[string]interface{}
		err := json.NewDecoder(r.Body).Decode(&reqBody)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		server := reqBody["server"].(map[string]interface{})
		networks, hasNetworks := server["networks"]

		var responseJSON string
		log.Printf("Request body for server creation: %+v", reqBody)

		if !hasNetworks || networks == nil || networks == "auto" || isEmptyNetworkSlice(networks) {
			responseJSON = `{
				"server": {
					"id": "test-server-id-12345",
					"name": "test-vm",
					"status": "BUILD",
					"created": "2024-11-20T12:00:00Z",
					"addresses": {"auto": [{"addr": "192.168.1.30"}]},
					"metadata": {},
					"links": []
				}
			}`
		} else {
			responseJSON = `{
				"server": {
					"id": "test-server-id-12345",
					"name": "test-vm",
					"status": "BUILD",
					"created": "2024-11-20T12:00:00Z",
					"addresses": {"test_net": [{"addr": "10.0.0.5"}]},
					"metadata": {},
					"links": []
				}
			}`
		}

		w.WriteHeader(http.StatusAccepted)
		w.Write([]byte(responseJSON))
	} else {
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func isEmptyNetworkSlice(networks interface{}) bool {
	if networks == nil {
		return true
	}
	if reflect.TypeOf(networks).Kind() == reflect.Slice {
		return len(networks.([]interface{})) == 0
	}
	return false
}

func HandlerFuncServersCreateFixedIPError(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method == "POST" {
		w.WriteHeader(http.StatusAccepted)
		w.Write([]byte(`{
            "server": {
                "id": "test-server-id-12345",
                "name": "test-vm",
                "status": "BUILD",
                "created": "2024-11-20T12:00:00Z",
                "hostId": "",
                "progress": 0,
                "accessIPv4": "",
                "accessIPv6": "",
                "image": {
                    "id": "test-image"
                },
                "flavor": {
                    "id": "test-flavor"
                },
                "addresses": {"private": [{"addr": ""}]},
                "metadata": {},
                "links": []
            }
        }`))
	} else {
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func HandlerFuncServersAssignFloatingIPError(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method == "POST" {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error": {"message": "Failed to assign floating IP"}}`))
	} else {
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func HandlerFuncServersGetSuccess(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method == "GET" {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
            "server": {
                "id": "test-server-id-12345",
                "name": "test-vm",
                "status": "ACTIVE",
                "created": "2024-11-20T12:00:00Z",
                "addresses": {"private": [{"addr": "192.168.1.10"}]},
                "metadata": {},
                "links": []
            }
        }`))
	} else {
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func HandlerFuncServersGetNetwork(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method == "GET" {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
            "server": {
                "id": "test-server-id-12345",
                "name": "test-vm",
                "status": "ACTIVE",
                "created": "2024-11-20T12:00:00Z",
                "addresses": {"auto": [{"addr": "192.168.1.30"}]},
                "metadata": {},
                "links": []
            }
        }`))
	} else {
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func HandlerFuncServersGetNetworkSet(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method == "GET" {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
            "server": {
                "id": "test-server-id-12345",
                "name": "test-vm",
                "status": "ACTIVE",
                "created": "2024-11-20T12:00:00Z",
                "addresses": {"test_net": [{"addr": "10.0.0.5"}]},
                "metadata": {},
                "links": []
            }
        }`))
	} else {
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func HandlerFuncServersGetInvalidAddr(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method == "GET" {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
            "server": {
                "id": "test-server-id-12345",
                "name": "test-vm",
                "status": "ACTIVE",
                "created": "2024-11-20T12:00:00Z",
                "addresses": {"private": [{"addr": "invalid_ip"}]},
                "metadata": {},
                "links": []
            }
        }`))
	} else {
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func HandlerFuncServersGetFixedIPError(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method == "GET" {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
            "server": {
                "id": "test-server-id-12345",
                "name": "test-vm",
                "status": "ACTIVE",
                "created": "2024-11-20T12:00:00Z",
                "addresses": {"private": [{"addr": ""}]},
                "metadata": {},
                "links": []
            }
        }`))
	} else {
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func HandlerFuncDeleteInstanceOK(w http.ResponseWriter, r *http.Request) {
	if r.Method == "DELETE" {
		w.WriteHeader(http.StatusNoContent)
	} else {
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func HandlerFuncDeleteInstanceAlreadyDeleted(w http.ResponseWriter, r *http.Request) {
	if r.Method == "DELETE" {
		w.WriteHeader(http.StatusNotFound)
	} else {
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func HandlerFuncDeleteInstanceFloatingIPNotAssigned(w http.ResponseWriter, r *http.Request) {
	if r.Method == "DELETE" {
		w.WriteHeader(http.StatusNoContent)
	} else {
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func HandlerFuncDeleteInstanceError(w http.ResponseWriter, r *http.Request) {
	if r.Method == "DELETE" {
		w.WriteHeader(http.StatusNotImplemented)
	} else {
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func HandlerFuncOSInterface(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method == "GET" {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"interfaceAttachments": [
				{
					"port_id": "test-port-id-12345",
					"port_state": "ACTIVE",
					"net_id": "test-network-id",
					"mac_addr": "fa:16:3e:12:34:56",
					"fixed_ips": [
						{
							"subnet_id": "test-subnet-id",
							"ip_address": "192.168.1.10"
						}
					]
				}
			]
		}`))
	} else {
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func HandlerFuncOSInterfaceNetwork(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method == "GET" {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"interfaceAttachments": [
				{
					"port_id": "test-port-id-network",
					"port_state": "ACTIVE",
					"net_id": "test-network-id",
					"mac_addr": "fa:16:3e:aa:bb:cc",
					"fixed_ips": [
						{
							"subnet_id": "test-subnet-id",
							"ip_address": "192.168.1.30"
						}
					]
				}
			]
		}`))
	} else {
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func HandlerFuncOSInterfaceNetworkSet(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method == "GET" {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"interfaceAttachments": [
				{
					"port_id": "test-port-id-network-set",
					"port_state": "ACTIVE",
					"net_id": "test-net-id",
					"mac_addr": "fa:16:3e:dd:ee:ff",
					"fixed_ips": [
						{
							"subnet_id": "test-subnet-id",
							"ip_address": "10.0.0.5"
						}
					]
				}
			]
		}`))
	} else {
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}
