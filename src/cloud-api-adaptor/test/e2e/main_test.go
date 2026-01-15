// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"context"
	"os"
	"testing"

	pv "github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/test/provisioner"
	log "github.com/sirupsen/logrus"

	kconf "sigs.k8s.io/e2e-framework/klient/conf"
	"sigs.k8s.io/e2e-framework/pkg/env"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
)

var (
	testEnv          env.Environment
	cloudProvider    string
	provisioner      pv.CloudProvisioner
	keyBrokerService *pv.KeyBrokerService
)

const (
	defaultContainerRuntime = "containerd"
)

func init() {
	initLogger()
}

func initLogger() {
	levelStr := os.Getenv("LOG_LEVEL")
	if levelStr == "" {
		levelStr = "info"
	}

	level, err := log.ParseLevel(levelStr)
	if err != nil {
		level = log.InfoLevel
	}

	log.SetLevel(level)
}

func TestMain(m *testing.M) {
	var err error

	// CLOUD_PROVIDER is required.
	cloudProvider = os.Getenv("CLOUD_PROVIDER")
	if cloudProvider == "" {
		log.Fatal("CLOUD_PROVIDER should be exported in the environment")
	}

	if os.Getenv("INSTALL_METHOD") == "helm" {
		log.Fatal("INSTALL_METHOD=helm is not implemented yet.")
	}

	// Create an empty test environment. At this point the client cannot connect to the cluster
	// unless it is running with an in-cluster configuration.
	testEnv = env.New()

	// TEST_TEARDOWN is an option variable which specifies whether the teardown code path
	// should run or not.
	shouldTeardown := true
	if os.Getenv("TEST_TEARDOWN") == "no" {
		shouldTeardown = false
	}
	// In case TEST_PROVISION is exported then it will try to provision the test environment
	// in the cloud provider. Otherwise, assume the developer did setup the environment and it will
	// look for a suitable kubeconfig file.
	shouldProvisionCluster := false
	if os.Getenv("TEST_PROVISION") == "yes" || os.Getenv("TEST_PROVISION") == "true" {
		shouldProvisionCluster = true
	}
	// Cloud API Adaptor is installed by default during e2e test.
	// In case TEST_INSTALL_CAA is exported as "no", it will skip the installation of
	// Cloud API Adaptor.
	// In scenario of teardown, CAA will be cleanup when shouldTeardown is true and shouldInstallCAA is true.
	shouldInstallCAA := true
	if os.Getenv("TEST_INSTALL_CAA") == "no" {
		shouldInstallCAA = false
	}

	// The TEST_PODVM_IMAGE is an option variable which specifies the path
	// to the podvm qcow2 image. If it set then the image should be uploaded to
	// the VPC images storage.
	podvmImage := os.Getenv("TEST_PODVM_IMAGE")

	// The TEST_PROVISION_FILE is an optional variable which specifies the path
	// to the provision properties file. The file must have the format:
	//
	//  key1 = "value1"
	//  key2 = "value2"
	provisionPropsFile := os.Getenv("TEST_PROVISION_FILE")

	// Get an provisioner instance for the cloud provider.
	provisioner, err = pv.GetCloudProvisioner(cloudProvider, provisionPropsFile)
	if err != nil {
		log.Fatal(err)
	}

	// The DEPLOY_KBS is exported then provisioner will install kbs before installing CAA
	shouldDeployKbs := false
	if os.Getenv("DEPLOY_KBS") == "true" || os.Getenv("DEPLOY_KBS") == "yes" {
		shouldDeployKbs = true
		os.Setenv("TEST_KBS", "true")
	}

	if !shouldProvisionCluster {
		// Look for a suitable kubeconfig file in the sequence: --kubeconfig flag,
		// or KUBECONFIG variable, or $HOME/.kube/config.
		kubeconfigPath := kconf.ResolveKubeConfigFile()
		if kubeconfigPath == "" {
			log.Fatal("Unabled to find a kubeconfig file")
		}
		cfg := envconf.NewWithKubeConfig(kubeconfigPath)
		testEnv = env.NewWithConfig(cfg)
	}
	var cloudAPIAdaptor *pv.CloudAPIAdaptor
	// Run *once* before the tests.
	testEnv.Setup(func(ctx context.Context, cfg *envconf.Config) (context.Context, error) {
		log.Info("Do setup")
		var err error

		// Get properties
		props := provisioner.GetProperties(ctx, cfg)

		// Set CONTAINER_RUNTIME env variable if present in the properties
		// Default value is containerd.
		containerRuntime := defaultContainerRuntime
		if props["CONTAINER_RUNTIME"] != "" {
			containerRuntime = props["CONTAINER_RUNTIME"]
		}

		log.Infof("Container runtime: %s", containerRuntime)
		os.Setenv("CONTAINER_RUNTIME", containerRuntime)

		if shouldProvisionCluster {
			log.Info("Cluster provisioning")
			if err = provisioner.CreateVPC(ctx, cfg); err != nil {
				return ctx, err
			}

			if err = provisioner.CreateCluster(ctx, cfg); err != nil {
				return ctx, err
			}
		}

		if shouldDeployKbs {
			log.Info("Deploying kbs")
			if keyBrokerService, err = pv.NewKeyBrokerService(props["CLUSTER_NAME"], cfg); err != nil {
				return ctx, err
			}

			if err = keyBrokerService.Deploy(ctx, cfg, props); err != nil {
				return ctx, err
			}
			var kbsEndpoint string
			if kbsEndpoint, err = keyBrokerService.GetKbsEndpoint(ctx, cfg); err != nil {
				return ctx, err
			}
			log.Infof("Https kbsEndpoint is: %s", kbsEndpoint)
		}

		if podvmImage != "" {
			log.Info("Podvm uploading")
			if err = provisioner.UploadPodvm(podvmImage, ctx, cfg); err != nil {
				return ctx, err
			}
		}

		// Check if provisioner supports PodVM instance creation and VM_POOL_IPS is empty
		if vmHandler, ok := provisioner.(pv.PodVMInstanceHandler); ok {
			currentProps := provisioner.GetProperties(ctx, cfg)
			if currentProps["VM_POOL_IPS"] == "" {
				log.Info("Creating PodVM instances...")
				if err = vmHandler.CreatePodVMInstance(ctx, cfg); err != nil {
					return ctx, err
				}
			}
		}

		if shouldInstallCAA {
			log.Info("Install Cloud API Adaptor")
			relativeInstallDirectory := "../../install"
			if cloudAPIAdaptor, err = pv.NewCloudAPIAdaptor(cloudProvider, relativeInstallDirectory); err != nil {
				return ctx, err
			}

			props = provisioner.GetProperties(ctx, cfg)
			log.Info("Deploy the Cloud API Adaptor")
			if err = cloudAPIAdaptor.Deploy(ctx, cfg, props); err != nil {
				return ctx, err
			}
		}

		if err = CreateAndWaitForNamespace(ctx, cfg.Client(), E2eNamespace); err != nil {
			return ctx, err
		}

		return ctx, nil
	})

	// Run *once* after the tests.
	testEnv.Finish(func(ctx context.Context, cfg *envconf.Config) (context.Context, error) {
		if !shouldTeardown {
			return ctx, nil
		}

		if err = DeleteAndWaitForNamespace(ctx, cfg.Client(), E2eNamespace); err != nil {
			return ctx, err
		}

		if shouldInstallCAA {
			log.Info("Delete the Cloud API Adaptor installation")
			if err = cloudAPIAdaptor.Delete(ctx, cfg); err != nil {
				return ctx, err
			}
		}

		if shouldDeployKbs {
			if err = keyBrokerService.Delete(ctx, cfg); err != nil {
				return ctx, err
			}
		}

		if shouldProvisionCluster {
			if err = provisioner.DeleteCluster(ctx, cfg); err != nil {
				return ctx, err
			}

			if err = provisioner.DeleteVPC(ctx, cfg); err != nil {
				log.Warnf("Failed to delete vpc resources, err: %s.", err)
				return ctx, nil
			}
		}

		// Clean up auto-created VM instances before deleting cluster
		if vmHandler, ok := provisioner.(pv.PodVMInstanceHandler); ok {
			log.Info("Deleting PodVM instances...")
			if err = vmHandler.DeletePodVMInstance(ctx, cfg); err != nil {
				log.Warnf("Failed to delete PodVM instances: %v", err)
			}
		}

		return ctx, nil
	})

	// Start the tests execution workflow.
	os.Exit(testEnv.Run(m))
}
