// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package kubevirt

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"testing"

	provider "github.com/confidential-containers/cloud-api-adaptor/src/cloud-providers"
	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-providers/util/cloudinit"
)

const ConfigDir = "/etc/config/cca/kubevirt"

type mockCloudConfig struct{}

func (c *mockCloudConfig) Generate() (string, error) {
	return "cloud config", nil
}

type errorCloudConfig struct{}

func (c *errorCloudConfig) Generate() (string, error) {
	return "", fmt.Errorf("invalid cloud config")
}

func TestNewProvider(t *testing.T) {
	tests := []struct {
		name                 string
		kubeconfigContent    string
		kubeconfigFile       bool
		vmconfigContent      string
		vmconfigFile         bool
		serviceconfigContent string
		serviceconfigFile    bool
		serviceconfigFlag    bool
		wantError            bool
	}{
		{
			name:                 "ValidConfig",
			kubeconfigContent:    validkubeconfig,
			kubeconfigFile:       true,
			vmconfigContent:      validvmconfig,
			vmconfigFile:         true,
			serviceconfigContent: validserviceconfig,
			serviceconfigFile:    true,
			serviceconfigFlag:    true,
			wantError:            false,
		},
		{
			name:                 "MissingKUBECONFIG",
			kubeconfigContent:    "",
			kubeconfigFile:       false,
			vmconfigContent:      validvmconfig,
			vmconfigFile:         true,
			serviceconfigContent: validserviceconfig,
			serviceconfigFile:    true,
			serviceconfigFlag:    true,
			wantError:            true,
		},
		{
			name: "InvalidKUBECONFIG",
			kubeconfigContent: `apiVersion: v1
clusters
  - cluster
    syntax error
`,
			kubeconfigFile:       true,
			vmconfigContent:      validvmconfig,
			vmconfigFile:         true,
			serviceconfigContent: validserviceconfig,
			serviceconfigFile:    true,
			serviceconfigFlag:    true,
			wantError:            true,
		},
		{
			name:                 "MissingVMCONFIG",
			kubeconfigContent:    validkubeconfig,
			kubeconfigFile:       true,
			vmconfigContent:      "",
			vmconfigFile:         false,
			serviceconfigContent: validserviceconfig,
			serviceconfigFile:    true,
			serviceconfigFlag:    true,
			wantError:            true,
		},
		{
			name:              "InvalidVMCONFIG",
			kubeconfigContent: validkubeconfig,
			kubeconfigFile:    true,
			vmconfigContent: `apiVersion: v1
kind: VirtualMachine
metadata:
broken: testvm
aaaaa
`,
			vmconfigFile:         true,
			serviceconfigContent: validserviceconfig,
			serviceconfigFile:    true,
			serviceconfigFlag:    true,
			wantError:            true,
		},
		{
			name:                 "SERVICECONFIG not set",
			kubeconfigContent:    validkubeconfig,
			kubeconfigFile:       true,
			vmconfigContent:      validvmconfig,
			vmconfigFile:         true,
			serviceconfigContent: "",
			serviceconfigFile:    false,
			serviceconfigFlag:    false,
			wantError:            false,
		},
		{
			name:              "InvalidSERVICECONFIG",
			kubeconfigContent: validkubeconfig,
			kubeconfigFile:    true,
			vmconfigContent:   validvmconfig,
			vmconfigFile:      true,
			serviceconfigContent: `apiVersion: v1
kind: Service
metadata:
  broken: testservice
aaaaaaa
`,
			serviceconfigFile: true,
			serviceconfigFlag: true,
			wantError:         true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			WithKubeConfig(t, tt.name, tt.kubeconfigContent, tt.kubeconfigFile, func() {
				WithVMConfig(t, tt.name, tt.vmconfigContent, tt.vmconfigFile, func() {
					WithServiceConfig(t, tt.name, tt.serviceconfigContent, tt.serviceconfigFile, func() {
						var config Config

						if tt.serviceconfigFlag {
							serviceconfigpath := os.Getenv("SERVICECONFIG")
							config = Config{
								serviceconfigfile: serviceconfigpath,
							}
						}

						provider, err := NewProvider(&config)

						if tt.wantError {
							if err == nil {
								t.Errorf("Expected error for test case %v, but got none", tt.name)
							} else {
								t.Logf("Expected error occurred for test case %v: %v", tt.name, err)
							}

							if provider != nil {
								t.Errorf("Expected provider to be nil for test case %v, but got: %+v", tt.name, provider)
							}
						} else {
							if err != nil {
								t.Errorf("Expected no error for test case %v, but got: %v", tt.name, err)
							}
							if provider == nil {
								t.Errorf("Expected provider to be non-nil for test case %v, but got nil", tt.name)
								return
							}

							kubevirtProvider, ok := provider.(*kubevirtProvider)
							if !ok {
								t.Errorf("Expected provider to be of type *kubevirtProvider for test case %v, but got: %T", tt.name, provider)
								return
							}

							if kubevirtProvider.kubevirtClient == nil {
								t.Errorf("Expected kubevirtClient to be non-nil for test case %v", tt.name)
							}
							if kubevirtProvider.kubernetesClient == nil {
								t.Errorf("Expected kubernetesClient to be non-nil for test case %v", tt.name)
							}
							if kubevirtProvider.serviceConfig == nil {
								t.Errorf("Expected serviceConfig to be non-nil for test case %v", tt.name)
							}
						}
					})
				})
			})

		})
	}
}

func TestCreateInstance(t *testing.T) {

	tests := []struct {
		name                 string
		kubeconfigContent    string
		kubeconfigFile       bool
		vmconfigContent      string
		vmconfigFile         bool
		serviceconfigContent string
		serviceconfigFile    bool
		serviceconfigFlag    bool
		cloudConfig          cloudinit.CloudConfigGenerator
		secrethandler        http.HandlerFunc
		createhandler        http.HandlerFunc
		podhandler           http.HandlerFunc
		servicehandler       http.HandlerFunc
		wantError            bool
	}{
		{
			name:                 "CreateInstanceSuccess",
			kubeconfigContent:    validkubeconfig,
			kubeconfigFile:       true,
			vmconfigContent:      validvmconfig,
			vmconfigFile:         true,
			serviceconfigContent: "",
			serviceconfigFile:    false,
			serviceconfigFlag:    false,
			cloudConfig:          &mockCloudConfig{},
			secrethandler:        HandleCreateSecretSuccess,
			createhandler:        HandleCreateVMSuccess,
			podhandler:           HandleGetInstanceSuccess,
			servicehandler:       nil,
			wantError:            false,
		},
		{
			name:                 "InvalidCloudConfig",
			kubeconfigContent:    validkubeconfig,
			kubeconfigFile:       true,
			vmconfigContent:      validvmconfig,
			vmconfigFile:         true,
			serviceconfigContent: "",
			serviceconfigFile:    false,
			serviceconfigFlag:    false,
			cloudConfig:          &errorCloudConfig{},
			secrethandler:        HandleCreateSecretSuccess,
			createhandler:        HandleCreateVMSuccess,
			podhandler:           HandleGetInstanceSuccess,
			servicehandler:       nil,
			wantError:            true,
		},
		{
			name:                 "SecretMethodFailed",
			kubeconfigContent:    validkubeconfig,
			kubeconfigFile:       true,
			vmconfigContent:      validvmconfig,
			vmconfigFile:         true,
			serviceconfigContent: "",
			serviceconfigFile:    false,
			serviceconfigFlag:    false,
			cloudConfig:          &mockCloudConfig{},
			secrethandler:        HandleCreateSecretFailed,
			createhandler:        HandleCreateVMSuccess,
			podhandler:           HandleGetInstanceSuccess,
			servicehandler:       nil,
			wantError:            true,
		},
		{
			name:                 "Createmethodfailed",
			kubeconfigContent:    validkubeconfig,
			kubeconfigFile:       true,
			vmconfigContent:      validvmconfig,
			vmconfigFile:         true,
			serviceconfigContent: "",
			serviceconfigFile:    false,
			serviceconfigFlag:    false,
			cloudConfig:          &mockCloudConfig{},
			secrethandler:        HandleCreateSecretSuccess,
			createhandler:        HandleCreateVMFailed,
			podhandler:           nil,
			servicehandler:       nil,
			wantError:            true,
		},
		{
			name:                 "Getmethodfailed",
			kubeconfigContent:    validkubeconfig,
			kubeconfigFile:       true,
			vmconfigContent:      validvmconfig,
			vmconfigFile:         true,
			serviceconfigContent: "",
			serviceconfigFile:    false,
			serviceconfigFlag:    false,
			cloudConfig:          &mockCloudConfig{},
			secrethandler:        HandleCreateSecretSuccess,
			createhandler:        HandleCreateVMSuccess,
			podhandler:           HandleGetVMFailed,
			servicehandler:       nil,
			wantError:            true,
		},
		{
			name:                 "Servicecreatesuccess",
			kubeconfigContent:    validkubeconfig,
			kubeconfigFile:       true,
			vmconfigContent:      validvmconfig,
			vmconfigFile:         true,
			serviceconfigContent: validserviceconfig,
			serviceconfigFile:    true,
			serviceconfigFlag:    true,
			cloudConfig:          &mockCloudConfig{},
			secrethandler:        HandleCreateSecretSuccess,
			createhandler:        HandleCreateVMSuccess,
			podhandler:           HandleGetInstanceSuccess,
			servicehandler:       HandleGetServiceSuccess,
			wantError:            false,
		},
		{
			name:                 "servicecreatemethod failed",
			kubeconfigContent:    validkubeconfig,
			kubeconfigFile:       true,
			vmconfigContent:      validvmconfig,
			vmconfigFile:         true,
			serviceconfigContent: validserviceconfig,
			serviceconfigFile:    true,
			serviceconfigFlag:    true,
			cloudConfig:          &mockCloudConfig{},
			secrethandler:        HandleCreateSecretSuccess,
			createhandler:        HandleCreateVMSuccess,
			podhandler:           HandleGetInstanceSuccess,
			servicehandler:       HandleGetServiceFailed,
			wantError:            true,
		},
	}

	for _, tt := range tests {
		WithKubeConfig(t, tt.name, tt.kubeconfigContent, tt.kubeconfigFile, func() {
			WithVMConfig(t, tt.name, tt.vmconfigContent, tt.vmconfigFile, func() {
				WithServiceConfig(t, tt.name, tt.serviceconfigContent, tt.serviceconfigFile, func() {
					mux := http.NewServeMux()

					if tt.secrethandler != nil {
						mux.HandleFunc("/api/v1/namespaces/default/secrets", tt.secrethandler)
					}

					if tt.createhandler != nil {
						mux.HandleFunc("/apis/kubevirt.io/v1/namespaces/default/virtualmachines", tt.createhandler)
					}

					if tt.podhandler != nil {
						mux.HandleFunc("/apis/kubevirt.io/v1/namespaces/default/virtualmachineinstances/testvm", tt.podhandler)
					}

					if tt.servicehandler != nil {
						mux.HandleFunc("/api/v1/namespaces/default/services", tt.servicehandler)
					}

					server := httptest.NewServer(mux)
					defer server.Close()

					content, err := os.ReadFile(kubeconfigpath)
					if err != nil {
						t.Fatalf("Failed to read kubeconfig: %v", err)
					}
					re := regexp.MustCompile(`server:\s*http://[^/\s]+`)
					originalContent := string(content)
					updatedContent := re.ReplaceAllString(originalContent, fmt.Sprintf("server: %s", server.URL))
					err = os.WriteFile(kubeconfigpath, []byte(updatedContent), 0600)
					if err != nil {
						t.Fatalf("Failed to write updated kubeconfig: %v", err)
					}

					var config Config

					if tt.serviceconfigFlag {
						serviceconfigpath := os.Getenv("SERVICECONFIG")
						config = Config{
							serviceconfigfile: serviceconfigpath,
						}
					}

					providerClient, err := NewProvider(&config)

					if err != nil {
						t.Fatalf("Expected provider to be created, but got error: %v", err)
					}

					podname := "testvm"
					sandboxID := "12345"
					spec := provider.InstanceTypeSpec{}
					instance, err := providerClient.CreateInstance(context.Background(), podname, sandboxID, tt.cloudConfig, spec)

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
			})
		})
	}
}

func TestDeleteInstance(t *testing.T) {
	tests := []struct {
		name                 string
		kubeconfigContent    string
		kubeconfigFile       bool
		vmconfigContent      string
		vmconfigFile         bool
		serviceconfigContent string
		serviceconfigFile    bool
		serviceconfigFlag    bool
		getvmhandler         http.HandlerFunc
		deletevmhandler      http.HandlerFunc
		secrethandler        http.HandlerFunc
		servicehandler       http.HandlerFunc
		wantError            bool
	}{
		{
			name:                 "DeleteInstanceSuccess",
			kubeconfigContent:    validkubeconfig,
			kubeconfigFile:       true,
			vmconfigContent:      validvmconfig,
			vmconfigFile:         true,
			serviceconfigContent: "",
			serviceconfigFile:    false,
			serviceconfigFlag:    false,
			getvmhandler:         HandleGetVMSuccess,
			deletevmhandler:      HandleDeleteVMSuccess,
			secrethandler:        HandleDeleteSecretSuccess,
			servicehandler:       nil,
			wantError:            false,
		},
		{
			name:                 "GetVM failed",
			kubeconfigContent:    validkubeconfig,
			kubeconfigFile:       true,
			vmconfigContent:      validvmconfig,
			vmconfigFile:         true,
			serviceconfigContent: "",
			serviceconfigFile:    false,
			serviceconfigFlag:    false,
			getvmhandler:         HandleGetVMFailed,
			deletevmhandler:      nil,
			secrethandler:        nil,
			servicehandler:       nil,
			wantError:            true,
		},
		{
			name:                 "DeleteVM failed",
			kubeconfigContent:    validkubeconfig,
			kubeconfigFile:       true,
			vmconfigContent:      validvmconfig,
			vmconfigFile:         true,
			serviceconfigContent: "",
			serviceconfigFile:    false,
			serviceconfigFlag:    false,
			getvmhandler:         HandleGetVMSuccess,
			deletevmhandler:      HandleDeleteVMFailed,
			secrethandler:        nil,
			servicehandler:       nil,
			wantError:            true,
		},
		{
			name:                 "DeleteSecret Failed",
			kubeconfigContent:    validkubeconfig,
			kubeconfigFile:       true,
			vmconfigContent:      validvmconfig,
			vmconfigFile:         true,
			serviceconfigContent: "",
			serviceconfigFile:    false,
			serviceconfigFlag:    false,
			getvmhandler:         HandleGetVMSuccess,
			deletevmhandler:      HandleDeleteVMSuccess,
			secrethandler:        HandleDeleteSecretFailed,
			servicehandler:       nil,
			wantError:            true,
		},

		{
			name:                 "DeleteServiceSuccess",
			kubeconfigContent:    validkubeconfig,
			kubeconfigFile:       true,
			vmconfigContent:      validvmconfig,
			vmconfigFile:         true,
			serviceconfigContent: validserviceconfig,
			serviceconfigFile:    true,
			serviceconfigFlag:    true,
			getvmhandler:         HandleGetVMSuccess,
			deletevmhandler:      HandleDeleteVMSuccess,
			secrethandler:        HandleDeleteSecretSuccess,
			servicehandler:       HandleDeleteServiceSuccess,
			wantError:            false,
		},
		{
			name:                 "servicedeletemethod failed",
			kubeconfigContent:    validkubeconfig,
			kubeconfigFile:       true,
			vmconfigContent:      validvmconfig,
			vmconfigFile:         true,
			serviceconfigContent: validserviceconfig,
			serviceconfigFile:    true,
			serviceconfigFlag:    true,
			getvmhandler:         HandleGetVMSuccess,
			deletevmhandler:      HandleDeleteVMSuccess,
			secrethandler:        HandleDeleteSecretSuccess,
			servicehandler:       HandleDeleteServiceFailed,
			wantError:            true,
		},
	}

	for _, tt := range tests {
		WithKubeConfig(t, tt.name, tt.kubeconfigContent, tt.kubeconfigFile, func() {
			WithVMConfig(t, tt.name, tt.vmconfigContent, tt.vmconfigFile, func() {
				WithServiceConfig(t, tt.name, tt.serviceconfigContent, tt.serviceconfigFile, func() {
					mux := http.NewServeMux()

					if tt.getvmhandler != nil {
						mux.HandleFunc("/apis/kubevirt.io/v1/namespaces/default/virtualmachines", tt.getvmhandler)
					}

					if tt.deletevmhandler != nil {
						mux.HandleFunc("/apis/kubevirt.io/v1/namespaces/default/virtualmachines/testvm", tt.deletevmhandler)
					}

					if tt.secrethandler != nil {
						mux.HandleFunc("/api/v1/namespaces/default/secrets/testvm-secret", tt.secrethandler)
					}

					if tt.servicehandler != nil {
						mux.HandleFunc("/api/v1/namespaces/default/services/testservice", tt.servicehandler)
					}

					server := httptest.NewServer(mux)
					defer server.Close()

					content, err := os.ReadFile(kubeconfigpath)
					if err != nil {
						t.Fatalf("Failed to read kubeconfig: %v", err)
					}
					re := regexp.MustCompile(`server:\s*http://[^/\s]+`)
					originalContent := string(content)
					updatedContent := re.ReplaceAllString(originalContent, fmt.Sprintf("server: %s", server.URL))
					err = os.WriteFile(kubeconfigpath, []byte(updatedContent), 0600)
					if err != nil {
						t.Fatalf("Failed to write updated kubeconfig: %v", err)
					}

					var config Config

					if tt.serviceconfigFlag {
						serviceconfigpath := os.Getenv("SERVICECONFIG")
						config = Config{
							serviceconfigfile: serviceconfigpath,
						}
					}

					providerClient, err := NewProvider(&config)

					if err != nil {
						t.Fatalf("Expected provider to be created, but got error: %v", err)
					}

					instanceID := "a1b2c3d4-e5f6-7890-1234-567890abcdef"

					err = providerClient.DeleteInstance(context.Background(), instanceID)

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
							t.Logf("DeleteInstance successfully for test case %v", tt.name)
						}
					}
				})
			})
		})
	}
}

func TestTeardown(t *testing.T) {
	tests := []struct {
		name              string
		kubeconfigContent string
		vmconfigContent   string
		shouldCreateFile  bool
		wantError         bool
	}{
		{
			name:              "Teardown success",
			kubeconfigContent: validkubeconfig,
			vmconfigContent:   validvmconfig,
			shouldCreateFile:  true,
			wantError:         false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			WithKubeConfig(t, tt.name, tt.kubeconfigContent, tt.shouldCreateFile, func() {
				WithVMConfig(t, tt.name, tt.vmconfigContent, tt.shouldCreateFile, func() {
					config := Config{}
					provider, err := NewProvider(&config)

					if tt.wantError {
						if err == nil {
							t.Errorf("Expected error for test case %v, but got none", tt.name)
						} else {
							t.Logf("Expected error occurred for test case %v: %v", tt.name, err)
						}

						if provider != nil {
							t.Errorf("Expected provider to be nil for test case %v, but got: %+v", tt.name, provider)
						}
					} else {
						if err != nil {
							t.Errorf("Expected no error for test case %v, but got: %v", tt.name, err)
						}
						if provider == nil {
							t.Errorf("Expected provider to be non-nil for test case %v, but got nil", tt.name)
							return
						}
					}

					err = provider.Teardown()

					if err != nil {
						t.Errorf("Expected no error in Teardown, but got: %v", err)
					} else {
						t.Logf("Teardown succeeded for test case: %v", t.Name())
					}
				})
			})
		})
	}
}

func TestConfigVerifier(t *testing.T) {
	tests := []struct {
		name              string
		kubeconfigContent string
		vmconfigContent   string
		shouldCreateFile  bool
		wantError         bool
	}{
		{
			name:              "ConfigVerifier success",
			kubeconfigContent: validkubeconfig,
			vmconfigContent:   validvmconfig,
			shouldCreateFile:  true,
			wantError:         false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			WithKubeConfig(t, tt.name, tt.kubeconfigContent, tt.shouldCreateFile, func() {
				WithVMConfig(t, tt.name, tt.vmconfigContent, tt.shouldCreateFile, func() {
					config := Config{}
					provider, err := NewProvider(&config)

					if tt.wantError {
						if err == nil {
							t.Errorf("Expected error for test case %v, but got none", tt.name)
						} else {
							t.Logf("Expected error occurred for test case %v: %v", tt.name, err)
						}

						if provider != nil {
							t.Errorf("Expected provider to be nil for test case %v, but got: %+v", tt.name, provider)
						}
					} else {
						if err != nil {
							t.Errorf("Expected no error for test case %v, but got: %v", tt.name, err)
						}
						if provider == nil {
							t.Errorf("Expected provider to be non-nil for test case %v, but got nil", tt.name)
							return
						}
					}

					err = provider.ConfigVerifier()

					if err != nil {
						t.Errorf("Expected no error in Teardown, but got: %v", err)
					} else {
						t.Logf("Teardown succeeded for test case: %v", t.Name())
					}
				})
			})
		})
	}
}
