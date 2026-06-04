// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package kubevirt

import (
	"flag"
	"fmt"
	"os"
	"testing"

	provider "github.com/confidential-containers/cloud-api-adaptor/src/cloud-providers"
)

type TestManager struct {
	*Manager
}

func TestInit(t *testing.T) {
	manager := provider.Get("kubevirt")
	if manager == nil {
		t.Fatal("kubevirt provider is not registered in the manager table")
	}

	t.Log("kubevirt provider is successfully registered in the manager table")
}

func TestParseCmd(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		envflag  string
		expected Config
	}{
		{
			name: "AllFlagSet",
			args: []string{
				"-serviceconfig=cmdserviceconfigfile",
			},
			envflag: "Noset",
			expected: Config{
				serviceconfigfile: "cmdserviceconfigfile",
			},
		},
		{
			name:    "DefaultValues",
			args:    []string{},
			envflag: "Noset",
			expected: Config{
				serviceconfigfile: "",
			},
		},
		{
			name:    "EnvsetValues",
			args:    []string{},
			envflag: "Set",
			expected: Config{
				serviceconfigfile: "envserviceconfigfile",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			flags := flag.NewFlagSet("test", flag.ContinueOnError)
			if tt.envflag == "Set" {
				os.Setenv("SERVICECONFIG", "envserviceconfigfile")
			}

			testManager := &Manager{}
			testManager.ParseCmd(flags)
			err := flags.Parse(tt.args)
			if err != nil {
				t.Errorf("Failed to parse flags: %v", err)
			}

			if !comparestructs(kubevirtcfg, tt.expected) {
				t.Errorf("Expected config: %+v, but got: %+v\n", tt.expected, kubevirtcfg)
			} else {
				t.Logf("Expected config: %+v, got config: %+v\n", tt.expected, kubevirtcfg)
			}

			flags = nil
			kubevirtcfg = Config{}
			if tt.envflag == "Set" {
				os.Unsetenv("SERVICECONFIG")
			}
		})
	}
}

func comparestructs(result, expected Config) bool {
	isEqual := true

	if result.serviceconfigfile != expected.serviceconfigfile {
		fmt.Printf("Expected serviceconfigfile: %v, but got: %v\n", expected.serviceconfigfile, result.serviceconfigfile)
		isEqual = false
	}

	return isEqual
}

func TestLoadEnv(t *testing.T) {
	testManager := &Manager{}
	testManager.LoadEnv()
}

func TestManagerNewProvider(t *testing.T) {
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
		description          string
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
			description:          "Valid config file should create client successfully",
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
			description:          "Missing kubeconfig file should failed",
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
			description:          "Invalid kubeconfig file should failed",
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
			description:          "Missing vmconfig file should failed",
		},
		{
			name:              "InvalidVMCONFIG",
			kubeconfigContent: validkubeconfig,
			kubeconfigFile:    true,
			vmconfigContent: `apiVersion: v1,
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
			description:          "Invalid vmconfig file should failed",
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
			description:          "It succeeds if there is no `serviceconfig` file but other configuration files are present",
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
aaaaaaaa
`,
			serviceconfigFile: true,
			serviceconfigFlag: true,
			wantError:         true,
			description:       "Invalid vmconfig file should failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			WithKubeConfig(t, tt.name, tt.kubeconfigContent, tt.kubeconfigFile, func() {
				WithVMConfig(t, tt.name, tt.vmconfigContent, tt.vmconfigFile, func() {
					WithServiceConfig(t, tt.name, tt.serviceconfigContent, tt.serviceconfigFile, func() {
						kubevirtcfg = Config{}
						if tt.serviceconfigFlag {
							serviceconfigpath := os.Getenv("SERVICECONFIG")
							kubevirtcfg = Config{
								serviceconfigfile: serviceconfigpath,
							}
						}
						testManager := &Manager{}
						provider, err := testManager.NewProvider()

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
							if provider != nil {
								t.Logf("Provider successfully created: %v", tt.name)
							} else {
								t.Errorf("Expected provider to be created for test case %v, but got nil", tt.name)
							}
						}
					})
				})
			})
		})
	}
}

func TestGetConfig(t *testing.T) {

	flags := flag.NewFlagSet("test", flag.ContinueOnError)
	testManager := &Manager{}

	args := []string{}

	os.Setenv("SERVICECONFIG", "envserviceconfigfile")

	testManager.ParseCmd(flags)
	err := flags.Parse(args)
	if err != nil {
		t.Fatalf("Failed to parse flags: %v", err)
	}

	expectedConfig := Config{
		serviceconfigfile: "envserviceconfigfile",
	}

	testManager.LoadEnv()
	testManager.GetConfig()

	if !comparestructs(kubevirtcfg, expectedConfig) {
		t.Errorf("After LoadEnv: Expected config: %+v, but got: %+v", expectedConfig, kubevirtcfg)
	} else {
		t.Logf("Expected config: %+v, got config: %+v\n", expectedConfig, kubevirtcfg)
	}

	os.Unsetenv("SERVICECONFIG")
	kubevirtcfg = Config{}
}
