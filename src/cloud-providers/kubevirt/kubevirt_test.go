// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package kubevirt

import (
	"context"
	"fmt"
	k8sv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubevirtv1 "kubevirt.io/api/core/v1"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

type kubeconfigSetupResult struct {
	kubeconfigPath string
	cleanup        func()
}

type vmconfigSetupResult struct {
	vmconfigPath string
	cleanup      func()
}

type serviceconfigSetupResult struct {
	serviceconfigPath string
	tempDir           string
	cleanup           func()
}

const configDir = "/etc/config/caa/kubevirt"
const validkubeconfig = `apiVersion: v1
kind: Config
clusters:
- cluster:
    server: http://127.0.0.1:80
    insecure-skip-tls-verify: true
  name: test-cluster
contexts:
- context:
    cluster: test-cluster
    user: test-user
  name: test-context
current-context: test-context
preferences: {}
users:
- name: test-user
  user:
    token: dummy-token
`

const validvmconfig = `apiVersion: kubevirt.io/v1
kind: VirtualMachine
metadata:
  name: testvm
  namespace: default
spec:
  runStrategy: Always
  template:
    metadata:
      creationTimestamp: null
    spec:
      domain:
        devices:
          disks:
          - disk:
              bus: virtio
            name: containerdisk
          rng: {}
        resources:
          requests:
            memory: 4Gi
      volumes:
      - containerDisk:
          image: abcdefg
          path: hijklmnp
        name: containerdisk
status: {}
`

const validserviceconfig = `apiVersion: v1
Kind: Service
metadata:
  name: testservice
  namespace: default
`

func TestNewProviderClient(t *testing.T) {
	tests := []struct {
		name              string
		kubeconfigContent string
		shouldCreateFile  bool
		wantError         bool
		description       string
	}{
		{
			name:              "ValidKUBECONFIG",
			kubeconfigContent: validkubeconfig,
			shouldCreateFile:  true,
			wantError:         false,
			description:       "Valid kubeconfig file should create client successfully",
		},
		{
			name:              "MissingKUBECONFIG",
			kubeconfigContent: "",
			shouldCreateFile:  false,
			wantError:         true,
			description:       "Not found kubeconfig should return error",
		},
		{
			name: "InvalidKUBECONFIG",
			kubeconfigContent: `apiVersion: v1
clusters
  - cluster
  syntax error
`,
			shouldCreateFile: true,
			wantError:        true,
			description:      "Invalid kubeconfig format should return error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			WithKubeConfig(t, tt.name, tt.kubeconfigContent, tt.shouldCreateFile, func() {
				client, err := NewProviderClient()

				if tt.wantError {
					if err == nil {
						t.Errorf("Expected error for test case %s, but got none", tt.name)
					} else {
						t.Logf("Expected error occurred for test case %s: %v", tt.name, err)
					}
					if client != nil {
						t.Errorf("Expected nil client for test case %s, but got: %+v", tt.name, client)
					}
				} else {
					if err != nil {
						t.Logf("Expected connection error for test environment %s: %v", tt.name, err)
					}
					if client == nil {
						t.Errorf("Expected client to be created for test case %s, but got nil", tt.name)
					} else {
						t.Logf("Client successfully created for test case %s", tt.name)
					}
				}

				t.Logf("Test case %s completed: %s", tt.name, tt.description)
			})
		})
	}
}

func TestNewKubernetesClient(t *testing.T) {
	tests := []struct {
		name              string
		kubeconfigContent string
		shouldCreateFile  bool
		wantError         bool
		description       string
	}{
		{
			name:              "ValidKUBECONFIG",
			kubeconfigContent: validkubeconfig,
			shouldCreateFile:  true,
			wantError:         false,
			description:       "Valid kubeconfig file should create Kubernetes client successfully",
		},
		{
			name:              "MissingKUBECONFIG",
			kubeconfigContent: "",
			shouldCreateFile:  false,
			wantError:         true,
			description:       "Not found kubeconfig file should return error",
		},
		{
			name: "InvalidKUBECONFIG",
			kubeconfigContent: `apiVersion: v1
clusters
  - cluster
  syntax error
`,
			shouldCreateFile: true,
			wantError:        true,
			description:      "Invalid kubeconfig format should return error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			WithKubeConfig(t, tt.name, tt.kubeconfigContent, tt.shouldCreateFile, func() {
				client, err := NewKubernetesClient()

				if tt.wantError {
					if err == nil {
						t.Errorf("Expected error for test case %s, but got none", tt.name)
					} else {
						t.Logf("Expected error occurred for test case %s: %v", tt.name, err)
					}
					if client != nil {
						t.Errorf("Expected nil client for test case %s, but got: %+v", tt.name, client)
					}
				} else {
					if err != nil {
						t.Logf("Expected connection error for test environment %s: %v", tt.name, err)
					}
					if client == nil {
						t.Errorf("Expected client to be created for test case %s, but got nil", tt.name)
					} else {
						t.Logf("Kubernetes client successfully created for test case %s", tt.name)
					}
				}

				t.Logf("Test case %s completed: %s", tt.name, tt.description)
			})
		})
	}
}

func TestCreateVM(t *testing.T) {

	tests := []struct {
		name              string
		kubeconfigContent string
		shouldCreateFile  bool
		handler           http.HandlerFunc
		wantError         bool
	}{
		{
			name:              "CreateVMSuccess",
			kubeconfigContent: validkubeconfig,
			shouldCreateFile:  true,
			handler:           HandleCreateVMSuccess,
			wantError:         false,
		},
		{
			name:              "CreateVMFailed",
			kubeconfigContent: validkubeconfig,
			shouldCreateFile:  true,
			handler:           HandleCreateVMFailed,
			wantError:         true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			WithKubeConfig(t, tt.name, tt.kubeconfigContent, tt.shouldCreateFile, func() {

				mux := http.NewServeMux()

				mux.HandleFunc("/apis/kubevirt.io/v1/namespaces/default/virtualmachines", tt.handler)

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

				providerClient, err := NewProviderClient()
				if err != nil {
					t.Fatalf("Failed to create provider client: %v", err)
				}

				vm := &kubevirtv1.VirtualMachine{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "kubevirt.io/v1",
						Kind:       "VirtualMachine",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "testvm",
						Namespace: "default",
					},
					Spec: kubevirtv1.VirtualMachineSpec{
						Template: &kubevirtv1.VirtualMachineInstanceTemplateSpec{
							Spec: kubevirtv1.VirtualMachineInstanceSpec{
								Volumes: []kubevirtv1.Volume{},
								Domain: kubevirtv1.DomainSpec{
									Devices: kubevirtv1.Devices{},
								},
							},
						},
					},
				}

				createvm, err := providerClient.CreateVM(context.Background(), vm, "testvm")

				if tt.wantError {
					if err == nil {
						t.Errorf("Expected error for test case %v, but got none", tt.name)
					} else {
						t.Logf("Expected error occurred for test case %v: %v", tt.name, err)
					}
					if createvm != nil {
						t.Errorf("Expected nil createvm for test case %s, but got: %+v", tt.name, createvm)
					}
				} else {
					if err != nil {
						t.Errorf("Expected no error for test case %v, but got: %v", tt.name, err)
					}
					if createvm == nil {
						t.Errorf("Expected vm to be created for test case %s, but got nil", tt.name)
					} else {
						t.Logf("Create VM successfully %v, but got none", tt.name)
					}
				}

				t.Logf("Test case %s completed", tt.name)
			})
		})
	}
}

func TestDeleteVM(t *testing.T) {
	tests := []struct {
		name              string
		kubeconfigContent string
		shouldCreateFile  bool
		handler           http.HandlerFunc
		vmname            string
		wantError         bool
	}{
		{
			name:              "DeleteVMSuccess",
			kubeconfigContent: validkubeconfig,
			shouldCreateFile:  true,
			handler:           HandleDeleteVMSuccess,
			vmname:            "testvm",
			wantError:         false,
		},
		{
			name:              "DeleteVMNotFound",
			kubeconfigContent: validkubeconfig,
			shouldCreateFile:  true,
			handler:           HandleDeleteVMSuccess,
			vmname:            "notvm",
			wantError:         true,
		},
		{
			name:              "DeleteVMFailed",
			kubeconfigContent: validkubeconfig,
			shouldCreateFile:  true,
			handler:           HandleDeleteVMFailed,
			vmname:            "testvm",
			wantError:         true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			WithKubeConfig(t, tt.name, tt.kubeconfigContent, tt.shouldCreateFile, func() {
				mux := http.NewServeMux()

				mux.HandleFunc("/apis/kubevirt.io/v1/namespaces/default/virtualmachines/testvm", tt.handler)

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

				providerClient, err := NewProviderClient()
				if err != nil {
					t.Fatalf("Failed to create provider client: %v", err)
				}

				namespace := "default"

				err = providerClient.DeleteVM(context.Background(), namespace, tt.vmname)

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
						t.Logf("Delete VM successfully %v, but got none", tt.name)
					}
				}
			})
		})
	}
}

func TestGetPodVM(t *testing.T) {
	tests := []struct {
		name              string
		kubeconfigContent string
		shouldCreateFile  bool
		handler           http.HandlerFunc
		wantError         bool
	}{
		{
			name:              "GetPodVMSuccess",
			kubeconfigContent: validkubeconfig,
			shouldCreateFile:  true,
			handler:           HandleGetInstanceSuccess,
			wantError:         false,
		},
		{
			name:              "InterfacesNotFound",
			kubeconfigContent: validkubeconfig,
			shouldCreateFile:  true,
			handler:           HandleGetInstanceNotInterfaces,
			wantError:         true,
		},
		{
			name:              "IPAddressNotFound",
			kubeconfigContent: validkubeconfig,
			shouldCreateFile:  true,
			handler:           HandleGetInstanceNotIPAddress,
			wantError:         true,
		},

		{
			name:              "GetPodVMFailed",
			kubeconfigContent: validkubeconfig,
			shouldCreateFile:  true,
			handler:           HandleGetInstanceFailed,
			wantError:         true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			WithKubeConfig(t, tt.name, tt.kubeconfigContent, tt.shouldCreateFile, func() {
				mux := http.NewServeMux()

				mux.HandleFunc("/apis/kubevirt.io/v1/namespaces/default/virtualmachineinstances/testvm", tt.handler)

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

				providerClient, err := NewProviderClient()
				if err != nil {
					t.Fatalf("Failed to create provider client: %v", err)
				}

				namespace := "default"
				vmname := "testvm"

				createvmi, err := providerClient.GetPodVM(context.Background(), namespace, vmname)

				if tt.wantError {
					if err == nil {
						t.Errorf("Expected error for test case %v, but got none", tt.name)
					} else {
						t.Logf("Expected error occurred for test case %v: %v", tt.name, err)
					}
					if createvmi != nil {
						t.Errorf("Expected nil createvmi for test case %s, but got: %+v", tt.name, createvmi)
					}
				} else {
					if err != nil {
						t.Errorf("Expected no error for test case %v, but got: %v", tt.name, err)
					} else {
						t.Logf("Delete VM successfully %v, but got none", tt.name)
					}
				}
			})
		})
	}
}

func TestGetVM(t *testing.T) {
	tests := []struct {
		name              string
		kubeconfigContent string
		shouldCreateFile  bool
		handler           http.HandlerFunc
		targetuid         string
		wantError         bool
	}{
		{
			name:              "GetVMSuccess",
			kubeconfigContent: validkubeconfig,
			shouldCreateFile:  true,
			handler:           HandleGetVMSuccess,
			targetuid:         "a1b2c3d4-e5f6-7890-1234-567890abcdef",
			wantError:         false,
		},
		{
			name:              "NotFoundVM",
			kubeconfigContent: validkubeconfig,
			shouldCreateFile:  true,
			handler:           HandleGetVMSuccess,
			targetuid:         "a1b2c3d4bbb",
			wantError:         true,
		},
		{
			name:              "GetVMFailed",
			kubeconfigContent: validkubeconfig,
			shouldCreateFile:  true,
			handler:           HandleGetVMFailed,
			targetuid:         "a1b2c3d4-e5f6-7890-1234-567890abcdef",
			wantError:         true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			WithKubeConfig(t, tt.name, tt.kubeconfigContent, tt.shouldCreateFile, func() {
				mux := http.NewServeMux()

				mux.HandleFunc("/apis/kubevirt.io/v1/namespaces/default/virtualmachines", tt.handler)

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

				providerClient, err := NewProviderClient()
				if err != nil {
					t.Fatalf("Failed to create provider client: %v", err)
				}

				namespace := "default"

				getvm, err := providerClient.GetVM(context.Background(), namespace, tt.targetuid)

				if tt.wantError {
					if err == nil {
						t.Errorf("Expected error for test case %v, but got none", tt.name)
					} else {
						t.Logf("Expected error occurred for test case %v: %v", tt.name, err)
					}
					if getvm != nil {
						t.Errorf("Expected nil getvm for test case %s, but got: %+v", tt.name, getvm)
					}
				} else {
					if err != nil {
						t.Errorf("Expected no error for test case %v, but got: %v", tt.name, err)
					} else {
						t.Logf("Get VM successfully %v, but got none", tt.name)
					}
				}
			})
		})
	}
}

func TestCreateService(t *testing.T) {
	tests := []struct {
		name              string
		kubeconfigContent string
		shouldCreateFile  bool
		handler           http.HandlerFunc
		wantError         bool
	}{
		{
			name:              "CreateServiceSuccess",
			kubeconfigContent: validkubeconfig,
			shouldCreateFile:  true,
			handler:           HandleCreateServiceSuccess,
			wantError:         false,
		},
		{
			name:              "CreateServiceFailed",
			kubeconfigContent: validkubeconfig,
			shouldCreateFile:  true,
			handler:           HandleCreateServiceFailed,
			wantError:         true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			WithKubeConfig(t, tt.name, tt.kubeconfigContent, tt.shouldCreateFile, func() {
				mux := http.NewServeMux()

				mux.HandleFunc("/api/v1/namespaces/default/services", tt.handler)

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

				k8sClient, err := NewKubernetesClient()
				if err != nil {
					t.Fatalf("Failed to create provider client: %v", err)
				}
				service := &k8sv1.Service{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Service",
						APIVersion: "v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-loadbalancer-service",
						Namespace: "default",
					},
				}

				namespace := "default"

				createservice, err := k8sClient.CreateService(context.Background(), namespace, service)

				if tt.wantError {
					if err == nil {
						t.Errorf("Expected error for test case %v, but got none", tt.name)
					} else {
						t.Logf("Expected error occurred for test case %v: %v", tt.name, err)
					}
					if createservice != nil {
						t.Errorf("Expected nil service for test case %s, but got: %+v", tt.name, createservice)
					}
				} else {
					if err != nil {
						t.Errorf("Expected no error for test case %v, but got: %v", tt.name, err)
					} else {
						t.Logf("Get service successfully %v, but got none", tt.name)
					}
				}
			})
		})
	}
}

func TestDeleteService(t *testing.T) {
	tests := []struct {
		name              string
		kubeconfigContent string
		shouldCreateFile  bool
		handler           http.HandlerFunc
		servicename       string
		wantError         bool
	}{
		{
			name:              "DeleteServiceSuccess",
			kubeconfigContent: validkubeconfig,
			shouldCreateFile:  true,
			handler:           HandleDeleteServiceSuccess,
			servicename:       "testservice",
			wantError:         false,
		},
		{
			name:              "DeleteServiceNotFound",
			kubeconfigContent: validkubeconfig,
			shouldCreateFile:  true,
			handler:           HandleDeleteServiceSuccess,
			servicename:       "notservice",
			wantError:         true,
		},
		{
			name:              "DeleteServiceFailed",
			kubeconfigContent: validkubeconfig,
			shouldCreateFile:  true,
			handler:           HandleDeleteServiceFailed,
			servicename:       "testservice",
			wantError:         true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			WithKubeConfig(t, tt.name, tt.kubeconfigContent, tt.shouldCreateFile, func() {
				mux := http.NewServeMux()

				mux.HandleFunc("/api/v1/namespaces/default/services/testservice", tt.handler)

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

				k8sClient, err := NewKubernetesClient()
				if err != nil {
					t.Fatalf("Failed to create provider client: %v", err)
				}

				namespace := "default"

				err = k8sClient.DeleteService(context.Background(), namespace, tt.servicename)

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
						t.Logf("Get service successfully %v, but got none", tt.name)
					}
				}
			})
		})
	}
}

func TestCreateSecret(t *testing.T) {

	tests := []struct {
		name              string
		kubeconfigContent string
		shouldCreateFile  bool
		handler           http.HandlerFunc
		wantError         bool
	}{
		{
			name:              "CreateSecretSuccess",
			kubeconfigContent: validkubeconfig,
			shouldCreateFile:  true,
			handler:           HandleCreateSecretSuccess,
			wantError:         false,
		},
		{
			name:              "CreateSecretFailed",
			kubeconfigContent: validkubeconfig,
			shouldCreateFile:  true,
			handler:           HandleCreateSecretFailed,
			wantError:         true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			WithKubeConfig(t, tt.name, tt.kubeconfigContent, tt.shouldCreateFile, func() {

				mux := http.NewServeMux()

				mux.HandleFunc("/api/v1/namespaces/default/secrets", tt.handler)

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

				k8sClient, err := NewKubernetesClient()
				if err != nil {
					t.Fatalf("Failed to create provider client: %v", err)
				}

				namespace := "default"
				instancename := "testvm"
				cloudConfigData := "username: testname"
				_, err = k8sClient.CreateSecret(context.Background(), namespace, instancename, cloudConfigData)

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
						t.Logf("Create secret successfully %v, but got none", tt.name)
					}
				}

				t.Logf("Test case %s completed", tt.name)
			})
		})
	}
}

func TestDeleteSecret(t *testing.T) {
	tests := []struct {
		name              string
		kubeconfigContent string
		shouldCreateFile  bool
		handler           http.HandlerFunc
		vmname            string
		wantError         bool
	}{
		{
			name:              "DeleteSecretSuccess",
			kubeconfigContent: validkubeconfig,
			shouldCreateFile:  true,
			handler:           HandleDeleteSecretSuccess,
			vmname:            "testvm",
			wantError:         false,
		},
		{
			name:              "DeleteSecretNotFound",
			kubeconfigContent: validkubeconfig,
			shouldCreateFile:  true,
			handler:           HandleDeleteSecretSuccess,
			vmname:            "notvm",
			wantError:         true,
		},
		{
			name:              "DeleteSecretFailed",
			kubeconfigContent: validkubeconfig,
			shouldCreateFile:  true,
			handler:           HandleDeleteSecretFailed,
			vmname:            "testvm",
			wantError:         true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			WithKubeConfig(t, tt.name, tt.kubeconfigContent, tt.shouldCreateFile, func() {
				mux := http.NewServeMux()

				mux.HandleFunc("/api/v1/namespaces/default/secrets/testvm-secret", tt.handler)

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

				k8sClient, err := NewKubernetesClient()
				if err != nil {
					t.Fatalf("Failed to create provider client: %v", err)
				}

				namespace := "default"

				err = k8sClient.DeleteSecret(context.Background(), namespace, tt.vmname)

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
						t.Logf("Delete secret successfully %v, but got none", tt.name)
					}
				}
			})
		})
	}
}

func TestVMconfigUnmarshal(t *testing.T) {
	tests := []struct {
		name            string
		vmconfigContent string
		vmconfigFile    bool
		wantError       bool
	}{
		{
			name:            "validvmconfig",
			vmconfigContent: validvmconfig,
			vmconfigFile:    true,
			wantError:       false,
		},
		{
			name:            "vmconfig not found",
			vmconfigContent: validvmconfig,
			vmconfigFile:    false,
			wantError:       true,
		},
		{
			name: "vmconfig yaml format error",
			vmconfigContent: `apiVersion: v1,
kind: VirtualMachine
metadata:
broken: testvm
aaaaa
`,
			vmconfigFile: true,
			wantError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			WithVMConfig(t, tt.name, tt.vmconfigContent, tt.vmconfigFile, func() {
				_, err := VMconfigUnmarshal()
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
						t.Logf("Unmarshal vmconfig successfully %v, but got none", tt.name)
					}
				}
			})
		})
	}
}

func TestServiceconfigUnmarshal(t *testing.T) {
	tests := []struct {
		name                 string
		serviceconfigContent string
		serviceconfigFile    bool
		wantError            bool
	}{
		{
			name:                 "validserviceconfig",
			serviceconfigContent: validserviceconfig,
			serviceconfigFile:    true,
			wantError:            false,
		},
		{
			name:                 "serviceconfig not found",
			serviceconfigContent: validserviceconfig,
			serviceconfigFile:    false,
			wantError:            true,
		},
		{
			name: "serviceconfig yaml format error",
			serviceconfigContent: `apiVersion: v1,
kind: Service
metadata:
  broken: testservice
aaaaaaa
`,
			serviceconfigFile: true,
			wantError:         true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			WithServiceConfig(t, tt.name, tt.serviceconfigContent, tt.serviceconfigFile, func() {
				serviceconfigpath := os.Getenv("SERVICECONFIG")
				_, err := ServiceconfigUnmarshal(serviceconfigpath)
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
						t.Logf("Unmarshal serviceconfig successfully %v, but got none", tt.name)
					}
				}
			})
		})
	}
}

func HandleCreateVMSuccess(w http.ResponseWriter, r *http.Request) {

	w.Header().Set("Content-Type", "application/json")
	if r.Method == "POST" {
		w.WriteHeader(http.StatusOK)
		_, err := w.Write([]byte(`{
			"apiVersion": "kubevirt.io/v1",
			"kind": "VirtualMachine",
			"metadata": {
				"name": "testvm",
				"namespace": "default",
				"uid": "a1b2c3d4-e5f6-7890-1234-567890abcdef",
				"resourceVersion": "1",
				"creationTimestamp": "2026-04-03T10:00:00Z"
			},
    			"spec": {
        			"running": false
			},
			"status": {
				"ready": false,
				"created": true
			}
		}`))
		if err != nil {
			log.Printf("Warning: Failed to write response: %v", err)
		}
	} else {
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func HandleCreateVMFailed(w http.ResponseWriter, r *http.Request) {

	w.Header().Set("Content-Type", "application/json")
	if r.Method == "POST" {
		w.WriteHeader(http.StatusUnauthorized)
		_, err := w.Write([]byte(`{
			"apiVersion": "kubevirt.io/v1",
			"kind": "VirtualMachine",
			"metadata": {},
			"status": "Failure"
			"message": "Unauthorized",
			"reason": "Unauthorized",
			"code": 401
		}`))
		if err != nil {
			log.Printf("Warning: Failed to write response: %v", err)
		}
	} else {
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func HandleDeleteVMSuccess(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method == "DELETE" {
		w.WriteHeader(http.StatusOK)
		_, err := w.Write([]byte(`{
			"apiversion": "kubevirt.io/v1",
			"kind": "VirtualMachine",
			"status": "Success"
		}`))
		if err != nil {
			log.Printf("Warning: Failed to write response: %v", err)
		}
	} else {
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func HandleDeleteVMFailed(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method == "DELETE" {
		w.WriteHeader(http.StatusUnauthorized)
		_, err := w.Write([]byte(`{
			"message": "Unauthorized",
			"code": 401
		}`))
		if err != nil {
			log.Printf("Warning: Failed to write response: %v", err)
		}
	} else {
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func HandleGetVMSuccess(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method == "GET" {
		w.WriteHeader(http.StatusOK)
		_, err := w.Write([]byte(`{
			"kind": "VirtualMachineList",
			"apiVersion": "kubevirt.io/v1",
			"metadata": {	
				"resourceVersion": "12345",
				"continue": "",
				"remainingItemCount": null
			},
			"items": [
			{
				"apiVersion": "kubevirt.io/v1",
				"kind": "VirtualMachine",
				"metadata": {
					"name": "testvm",
					"namespace": "default",
					"uid": "a1b2c3d4-e5f6-7890-1234-567890abcdef",
					"resourceVersion": "100"
            			},
				"spec": {
					"running": false
				}
			},
        		{
				"apiVersion": "kubevirt.io/v1",
            			"kind": "VirtualMachine",
				"metadata": {
					"name": "testvm2",
					"namespace": "default",
					"uid": "b2c3d4e5-f6a7-8901-2345-67890abcdef0",
					"resourceVersion": "101"
            			},
				"spec": {
					"running": true
				}
			}
    			]
		}`))
		if err != nil {
			log.Printf("Warning: Failed to write response: %v", err)
		}
	} else {
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func HandleGetVMFailed(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method == "GET" {
		w.WriteHeader(http.StatusUnauthorized)
		_, err := w.Write([]byte(`{
			"kind": "VirtualMachineList",
			"apiVersion": "kubevirt.io/v1",
			"metadata": {},
			"status": "Failure",
			"message": "Unauthorized",
			"reason": "Unauthorized",
			"code": 401

		}`))
		if err != nil {
			log.Printf("Warning: Failed to write response: %v", err)
		}
	} else {
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func HandleGetInstanceSuccess(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method == "GET" {
		w.WriteHeader(http.StatusOK)
		_, err := w.Write([]byte(`{
			"kind": "VirtualMachineInstance",
			"apiVersion": "kubevirt.io/v1",
			"metadata": {
				"name": "testvm",
				"namespace": "default"
			},
			"spec": {
				"domain": {
					"resources": {
						"requests": {
							"memory": "1Gi"
						}
					}
				}
			},
			"status": {
				"phase": "Running",
				"interfaces": [
					{
						"name": "default",
						"interfaceName": "eth0",
						"ipAddress": "192.168.1.100"
					}
				]
			}
		}`))
		if err != nil {
			log.Printf("Warning: Failed to write response: %v", err)
		}
	} else {
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func HandleGetInstanceFailed(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method == "GET" {
		w.WriteHeader(http.StatusUnauthorized)
		_, err := w.Write([]byte(`{
			"kind": "VirtualMachineInstance",
			"apiVersion": "kubevirt.io/v1",
			"metadata": {},
			"status": "Failure",
			"message": "Unauthorized",
			"reason": "Unauthorized",
			"code": 401
		}`))
		if err != nil {
			log.Printf("Warning: Failed to write response: %v", err)
		}
	} else {
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func HandleGetInstanceNotInterfaces(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method == "GET" {
		w.WriteHeader(http.StatusOK)
		_, err := w.Write([]byte(`{
			"kind": "VirtualMachineInstance",
			"apiVersion": "kubevirt.io/v1",
			"metadata": {
				"name": "testvm",
				"namespace": "default"
			},
			"spec": {
				"domain": {
					"resources": {
						"requests": {
							"memory": "1Gi"
						}
					}
				}
			}
		}`))
		if err != nil {
			log.Printf("Warning: Failed to write response: %v", err)
		}
	} else {
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func HandleGetInstanceNotIPAddress(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method == "GET" {
		w.WriteHeader(http.StatusOK)
		_, err := w.Write([]byte(`{
			"kind": "VirtualMachineInstance",
			"apiVersion": "kubevirt.io/v1",
			"metadata": {
				"name": "testvm",
				"namespace": "default"
			},
			"spec": {
				"domain": {
					"resources": {
						"requests": {
							"memory": "1Gi"
						}
					}
				}
			},
			"status": {
				"phase": "Running",
				"interfaces": [
					{
						"name": "default",
						"interfaceName": "eth0",
						"ipAddress": ""
					}
				]
			}
		}`))
		if err != nil {
			log.Printf("Warning: Failed to write response: %v", err)
		}
	} else {
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func HandleCreateServiceSuccess(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method == "POST" {
		w.WriteHeader(http.StatusOK)
		_, err := w.Write([]byte(`{
			"kind": "Service",
			"apiVersion": "v1",
			"metadata": {
				"name": "test-service",
				"namespace": "default",
				"uid": "a1b2c3d4-e5f6-7890-1234-567890abcdef",
				"resourceVersion": "123456"
			},
			"spec": {
				"ports": [
					{
	    					"protocol": "TCP",
	    					"port": 80,
	    					"targetPort": 8080
					}
    				],
    			"selector": {
				"app": "my-loadbalancer-app"
    			},
			"clusterIP": "10.96.0.200",
			"clusterIPs": [
				"10.96.0.200"
			],
			"type": "LoadBalancer",
			"sessionAffinity": "None",
			"externalTrafficPolicy": "Cluster",
			"ipFamilies": [
				"IPv4"
			],
			"ipFamilyPolicy": "SingleStack"
			},
			"status": {
				"loadBalancer": {
					"ingress": [
	    					{
							"ip": "203.0.113.42"
	    					},
	    					{
							"hostname": "a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6.elb.ap-northeast-1.amazonaws.com"
	    					}
					]
    				}
			}
		}`))
		if err != nil {
			log.Printf("Warning: Failed to write response: %v", err)
		}
	} else {
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func HandleCreateServiceFailed(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method == "POST" {
		w.WriteHeader(http.StatusUnauthorized)
		_, err := w.Write([]byte(`{
			"kind": "Service",
			"apiVersion": "v1",
			"metadata": {},
			"status": "Failure",
			"message": "Unauthorized",
			"reason": "Unauthorized",
			"code": 401,

		}`))
		if err != nil {
			log.Printf("Warning: Failed to write response: %v", err)
		}
	} else {
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func HandleDeleteServiceSuccess(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method == "DELETE" {
		w.WriteHeader(http.StatusOK)
		_, err := w.Write([]byte(`{
			"kind": "Service"
			"apiVersion": "v1"
			"metadata": {},
			"status": "Success"

		}`))
		if err != nil {
			log.Printf("Warning: Failed to write response: %v", err)
		}
	} else {
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func HandleDeleteServiceFailed(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method == "DELETE" {
		w.WriteHeader(http.StatusUnauthorized)
		_, err := w.Write([]byte(`{
			"message": "Unauthorized",
			"code": 401
		}`))
		if err != nil {
			log.Printf("Warning: Failed to write response: %v", err)
		}
	} else {
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func HandleCreateSecretSuccess(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method == "POST" {
		w.WriteHeader(http.StatusCreated)
		_, err := w.Write([]byte(`{
			"apiVersion": "v1",
			"kind": "Secret",
			"metadata": {
				"name": "testvm-secret",
				"namespace": "default",
				"uid": "a1b2c3d4-e5f6-7890-1234-567890abcdef",
				"resourceVersion": "1",
				"creationTimestamp": "2024-04-23T10:00:00Z"
			},
			"data": {
				"username": "testname",
				"password": "testpass"
			},
			"type": "Opaque"
		}`))
		if err != nil {
			log.Printf("Warning: Failed to write response: %v", err)
		}
	} else {
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func HandleCreateSecretFailed(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method == "POST" {
		w.WriteHeader(http.StatusUnauthorized)
		_, err := w.Write([]byte(`{
                        "message": "Unauthorized",
                        "code": 401
                }`))
		if err != nil {
			log.Printf("Warning: Failed to write response: %v", err)
		}
	} else {
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func HandleDeleteSecretSuccess(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method == "DELETE" {
		w.WriteHeader(http.StatusOK)
		_, err := w.Write([]byte(`{
			"apiVersion": "v1",
			"kind": "Status",
			"metadata": {},
			"status": "Success",
			"details": {
				"name": "testvm-secret",
				"kind": "Secret",
				"uid": "a1b2c3d4-e5f6-7890-1234-567890abcdef"
			}
		}`))
		if err != nil {
			log.Printf("Warning: Failed to write response: %v", err)
		}
	} else {
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func HandleDeleteSecretFailed(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method == "DELETE" {
		w.WriteHeader(http.StatusUnauthorized)
		_, err := w.Write([]byte(`{
                        "message": "Unauthorized",
                        "code": 401
                }`))
		if err != nil {
			log.Printf("Warning: Failed to write response: %v", err)
		}
	} else {
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func SetupKubeConfig(t *testing.T, testName string, kubeconfigContent string, shouldCreateFile bool) *kubeconfigSetupResult {
	result := &kubeconfigSetupResult{}

	if shouldCreateFile {
		err := os.MkdirAll(configDir, 0755)
		if err != nil {
			t.Fatalf("Failed to create directory %s: %v", configDir, err)
		}
		result.kubeconfigPath = filepath.Join(configDir, "kubeconfig")
		err = os.WriteFile(result.kubeconfigPath, []byte(kubeconfigContent), 0600)
		if err != nil {
			t.Fatalf("Failed to create kubeconfig file for %s: %v", testName, err)
		}
		t.Logf("Created kubeconfig file for %s: %s", testName, result.kubeconfigPath)
	}
	result.cleanup = func() {
		if shouldCreateFile && result.kubeconfigPath != "" {
			if _, err := os.Stat(result.kubeconfigPath); err == nil {
				if err := os.Remove(result.kubeconfigPath); err != nil {
					t.Logf("Could not remove temp kubeconfig file for %s: %v", testName, err)
				}
			}
		}
	}

	return result
}

func WithKubeConfig(t *testing.T, testName string, kubeconfigContent string, shouldCreateFile bool, testFunc func()) {
	setup := SetupKubeConfig(t, testName, kubeconfigContent, shouldCreateFile)
	defer setup.cleanup()
	testFunc()
}

func SetupVMConfig(t *testing.T, testName string, vmconfigContent string, shouldCreateFile bool) *vmconfigSetupResult {
	result := &vmconfigSetupResult{}

	if shouldCreateFile {
		err := os.MkdirAll(configDir, 0755)

		if err != nil {
			t.Fatalf("Failed to create fixed config directory %s for %s: %v", configDir, testName, err)
		}

		result.vmconfigPath = filepath.Join(configDir, "podvm.yaml")
		err = os.WriteFile(result.vmconfigPath, []byte(vmconfigContent), 0600)
		if err != nil {
			t.Fatalf("Failed to create vmconfig file for %s: %v", testName, err)
		}
		t.Logf("Created vmconfig file for %s: %s", testName, result.vmconfigPath)
	}
	result.cleanup = func() {
		if shouldCreateFile && result.vmconfigPath != "" {
			if _, err := os.Stat(result.vmconfigPath); err == nil {
				if err := os.Remove(result.vmconfigPath); err != nil {
					t.Logf("Could not remove temp vmconfig file for %s: %v", testName, err)
				}
			}
		}
	}

	return result
}

func WithVMConfig(t *testing.T, testName string, vmconfigContent string, shouldCreateFile bool, testFunc func()) {
	setup := SetupVMConfig(t, testName, vmconfigContent, shouldCreateFile)
	defer setup.cleanup()
	testFunc()
}

func SetupServiceConfig(t *testing.T, testName string, serviceconfigContent string, shouldCreateFile bool) *serviceconfigSetupResult {
	result := &serviceconfigSetupResult{}

	if shouldCreateFile {
		result.tempDir = t.TempDir()
		result.serviceconfigPath = filepath.Join(result.tempDir, "serviceconfig")

		err := os.WriteFile(result.serviceconfigPath, []byte(serviceconfigContent), 0600)
		if err != nil {
			t.Fatalf("Failed to create serviceconfig file for %s: %v", testName, err)
		}
		t.Logf("Created serviceconfig file for %s: %s", testName, result.serviceconfigPath)

		os.Setenv("SERVICECONFIG", result.serviceconfigPath)
		t.Logf("Set SERVICECONFIG environment variable for %s: %s", testName, result.serviceconfigPath)
	}

	result.cleanup = func() {
		os.Unsetenv("SERVICECONFIG")
		if shouldCreateFile && result.serviceconfigPath != "" {
			if _, err := os.Stat(result.serviceconfigPath); err == nil {
				if err := os.Remove(result.serviceconfigPath); err != nil {
					t.Logf("Could not remove temp vmconfig file for %s: %v", testName, err)
				}
			}
		}
	}

	return result
}

func WithServiceConfig(t *testing.T, testName string, serviceconfigContent string, shouldCreateFile bool, testFunc func()) {
	setup := SetupServiceConfig(t, testName, serviceconfigContent, shouldCreateFile)
	defer setup.cleanup()
	testFunc()
}
