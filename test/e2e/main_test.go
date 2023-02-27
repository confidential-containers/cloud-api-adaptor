package e2e

import (
	"context"
	"fmt"
	"os"
	"testing"

	kconf "sigs.k8s.io/e2e-framework/klient/conf"
	"sigs.k8s.io/e2e-framework/pkg/env"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
)

var (
	testEnv         env.Environment
	cloudProvider   string
	provisioner     CloudProvision
	cloudAPIAdaptor *CloudAPIAdaptor
)

func TestMain(m *testing.M) {
	var err error

	// CLOUD_PROVIDER is required.
	cloudProvider = os.Getenv("CLOUD_PROVIDER")
	if cloudProvider == "" {
		fmt.Println("CLOUD_PROVIDER should be exported in the environment")
		os.Exit(1)
	}

	// Create an empty test environment. At this point the client cannot connect to the cluster
	// unless it is running with an in-cluster configuration.
	testEnv = env.New()

	// TEST_E2E_TEARDOWN is an option variable which specifies whether the teardown code path
	// should run or not.
	shouldTeardown := true
	if os.Getenv("TEST_E2E_TEARDOWN") == "no" {
		shouldTeardown = false
	}
	// In case TEST_E2E_PROVISION is exported then it will try to provision the test environment
	// in the cloud provider. Otherwise, assume the developer did setup the environment and it will
	// look for a suitable kubeconfig file.
	shouldProvisionCluster := false
	if os.Getenv("TEST_E2E_PROVISION") == "yes" {
		shouldProvisionCluster = true
	}

	// The TEST_E2E_PODVM_IMAGE is an option variable which specifies the path
	// to the podvm qcow2 image. If it set then the image should be uploaded to
	// the VPC images storage.
	podvmImage := os.Getenv("TEST_E2E_PODVM_IMAGE")

	if shouldProvisionCluster || podvmImage != "" {
		// Get an provisioner instance for the cloud provider.
		provisioner, err = GetCloudProvisioner(cloudProvider)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	}

	if !shouldProvisionCluster {
		// Look for a suitable kubeconfig file in the sequence: --kubeconfig flag,
		// or KUBECONFIG variable, or $HOME/.kube/config.
		kubeconfigPath := kconf.ResolveKubeConfigFile()
		if kubeconfigPath == "" {
			fmt.Fprintln(os.Stderr, "Unabled to find a kubeconfig file")
			os.Exit(1)
		}
		cfg := envconf.NewWithKubeConfig(kubeconfigPath)
		testEnv = env.NewWithConfig(cfg)
	}

	// Run *once* before the tests.
	testEnv.Setup(func(ctx context.Context, cfg *envconf.Config) (context.Context, error) {
		fmt.Println("Do setup")
		var err error

		if shouldProvisionCluster {
			fmt.Println("Cluster provisioning")
			if err = provisioner.CreateVPC(ctx, cfg); err != nil {
				return ctx, err
			}

			if err = provisioner.CreateCluster(ctx, cfg); err != nil {
				return ctx, err
			}
		}

		if podvmImage != "" {
			if err = provisioner.UploadPodvm(podvmImage, ctx, cfg); err != nil {
				return ctx, err
			}
		}

		cloudAPIAdaptor = NewCloudAPIAdaptor(cloudProvider)
		fmt.Println("Deploy the Cloud API Adaptor")
		if err = cloudAPIAdaptor.Deploy(ctx, cfg); err != nil {
			return ctx, err
		}
		return ctx, nil
	})

	// Run *once* after the tests.
	testEnv.Finish(func(ctx context.Context, cfg *envconf.Config) (context.Context, error) {
		if !shouldTeardown {
			return ctx, nil
		}
		if shouldProvisionCluster {
			if err = provisioner.DeleteCluster(ctx, cfg); err != nil {
				return ctx, err
			}

			if err = provisioner.DeleteVPC(ctx, cfg); err != nil {
				return ctx, nil
			}
		} else {
			fmt.Println("Delete the Cloud API Adaptor installation")
			if err = cloudAPIAdaptor.Delete(ctx, cfg); err != nil {
				return ctx, err
			}
		}

		return ctx, nil
	})

	// Start the tests execution workflow.
	os.Exit(testEnv.Run(m))
}
