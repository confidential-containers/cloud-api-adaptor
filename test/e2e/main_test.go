package e2e

import (
	"context"
	"fmt"
	"os"
	kconf "sigs.k8s.io/e2e-framework/klient/conf"
	"sigs.k8s.io/e2e-framework/pkg/env"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"testing"
)

var (
	testEnv       env.Environment
	cloudProvider string
	provisioner   CloudProvision
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

	// In case TEST_E2E_PROVISION is exported then it will try to provision the test environment
	// in the cloud provider. Otherwise, assume the developer did setup the environment and it will
	// look for a suitable kubeconfig file.
	shouldProvisionCluster := false
	if os.Getenv("TEST_E2E_PROVISION") == "yes" {
		shouldProvisionCluster = true
	}

	if shouldProvisionCluster {
		// Get an provisioner instance for the cloud provider.
		provisioner, err = GetCloudProvisioner()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	} else {
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
			if err = provisioner.CreateVPC(ctx, cfg); err != nil {
				return ctx, err
			}

			if err = provisioner.CreateCluster(ctx, cfg); err != nil {
				return ctx, err
			}
		}

		peerPods := NewPeerPods(cloudProvider)
		fmt.Println("Deploy Peer Pods")
		if err = peerPods.Deploy(ctx, cfg); err != nil {
			return ctx, err
		}
		return ctx, nil
	})

	// Run *once* after the tests.
	testEnv.Finish(func(ctx context.Context, cfg *envconf.Config) (context.Context, error) {
		// TODO: implement me.
		return ctx, nil
	})

	// Start the tests execution workflow.
	os.Exit(testEnv.Run(m))
}
