//go:build azure

// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package provisioner

import (
	"context"

	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"time"

	"github.com/Azure/azure-sdk-for-go/profiles/latest/containerservice/mgmt/containerservice"
	"github.com/Azure/azure-sdk-for-go/profiles/latest/network/mgmt/network"
	"github.com/Azure/azure-sdk-for-go/profiles/latest/resources/mgmt/resources"
	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/to"
	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/util/wait"
)

func init() {
	newProvisionerFunctions["azure"] = NewAzureCloudProvisioner
	newInstallOverlayFunctions["azure"] = NewAzureInstallOverlay
}

func createResourceGroup() error {
	newRG := resources.Group{
		Location: &AzureProps.Location,
	}

	log.Infof("Creating Resource group %s.\n", AzureProps.ResourceGroupName)
	resourceGroup, err := AzureProps.ResourceGroupClient.CreateOrUpdate(context.Background(), AzureProps.ResourceGroupName, newRG)
	if err != nil {
		log.Infof("Failed to create resource group: %s:%v.\n", AzureProps.ResourceGroupName, err)
		return fmt.Errorf("creating resource group: %w", err)
	}

	AzureProps.ResourceGroup = &resourceGroup

	log.Infof("Successfully Created Resource group %s.\n", AzureProps.ResourceGroupName)
	return nil
}

func deleteResourceGroup() error {
	log.Infof("Deleting Resource group %s.\n", AzureProps.ResourceGroupName)
	_, err := AzureProps.ResourceGroupClient.Delete(context.Background(), AzureProps.ResourceGroupName)
	if err != nil {
		if typedError, ok := err.(autorest.DetailedError); ok {
			if typedError.StatusCode == http.StatusNotFound {
				return nil
			}
		}

		log.Infof("Failed to delete resource group %s:%v.\n", AzureProps.ResourceGroupName, err)
		return fmt.Errorf("deleting resource group: %w", err)
	}

	log.Infof("Successfully Deleted Resource group %s.\n", AzureProps.ResourceGroupName)

	return nil
}

func createVnetSubnet() error {
	addressPrefix := "10.2.0.0/16"
	subnetAddressPrefix := "10.2.0.0/24"
	vnetParams := network.VirtualNetwork{
		Location: &AzureProps.Location,
		Name:     &AzureProps.VnetName,
		VirtualNetworkPropertiesFormat: &network.VirtualNetworkPropertiesFormat{
			AddressSpace: &network.AddressSpace{
				AddressPrefixes: &[]string{addressPrefix},
			},
			Subnets: &[]network.Subnet{
				{
					Name: &AzureProps.SubnetName,
					SubnetPropertiesFormat: &network.SubnetPropertiesFormat{
						AddressPrefix: &subnetAddressPrefix,
					},
				},
			},
		},
	}

	// Create the virtual network
	log.Infof("Creating  vnet %s in resource group %s with addressPrefix %s subnetAddressPrefix %s.\n", AzureProps.VnetName, AzureProps.ResourceGroupName, addressPrefix, subnetAddressPrefix)
	_, err := AzureProps.ManagedVnetClient.CreateOrUpdate(context.Background(), AzureProps.ResourceGroupName, AzureProps.VnetName, vnetParams)
	if err != nil {
		return fmt.Errorf("creating vnet: %w", err)
	}

	subnet, err := AzureProps.ManagedSubnetClient.Get(context.Background(), AzureProps.ResourceGroupName, AzureProps.VnetName, AzureProps.SubnetName, "")
	if err != nil {
		return fmt.Errorf("fetching subnet after creating vnet: %w", err)
	}

	if subnet.ID == nil || *subnet.ID == "" {
		return errors.New("SubnetID is empty, unknown error happened when creating subnet.")
	}

	AzureProps.SubnetID = *subnet.ID

	log.Infof("Successfully Created vnet %s with Subnet %s in resource group %s.\n", AzureProps.VnetName, AzureProps.SubnetID, AzureProps.ResourceGroupName)

	return nil
}

func createResourceImpl() error {
	err := createResourceGroup()
	if err != nil {
		return fmt.Errorf("creating resource group: %w", err)
	}

	// rg creation takes few seconds to complete keeping it as 60 second to be on safe side.
	const sleeptime = time.Duration(60) * time.Second
	log.Info("waiting for the Resource group to be available before creating vnet...")
	time.Sleep(sleeptime)
	return createVnetSubnet()
}

func deleteResourceImpl() error {
	return deleteResourceGroup()
}

func isAzureClusterReady(resourceGroupName string, clusterName string) (bool, error) {
	log.Debug("isAzureClusterReady()")
	cluster, err := AzureProps.ManagedAksClient.Get(context.Background(), resourceGroupName, clusterName)
	if err != nil {
		log.Errorf("failed to get cluster: %v", err)
		return false, fmt.Errorf("failed to get cluster: %w", err)
	}

	if *cluster.ProvisioningState != "Succeeded" {
		return false, nil
	}

	return true, nil
}

func isClusterDeleted(resourceGroupName string, clusterName string) (bool, error) {
	log.Debug("isClusterDeleted()")
	_, err := AzureProps.ManagedAksClient.Get(context.Background(), resourceGroupName, clusterName)
	if err != nil {
		if typedError, ok := err.(autorest.DetailedError); ok {
			if typedError.StatusCode == http.StatusNotFound {
				return true, nil
			}
		}
		log.Errorf("failed to get cluster: %v", err)
		return false, fmt.Errorf("failed to get cluster: %w", err)
	}

	return false, nil
}

func syncKubeconfig(kubeconfigdirpath string, kubeconfigpath string) error {
	kubeconfig, err := AzureProps.ManagedAksClient.ListClusterAdminCredentials(context.Background(), AzureProps.ResourceGroupName, AzureProps.ClusterName, "clusterkubeconfig")
	if err != nil {
		return fmt.Errorf("sync kubeconfig: %w", err)
	}

	kubeconfigStr := *(*kubeconfig.Kubeconfigs)[0].Value

	err = os.MkdirAll(kubeconfigdirpath, 0755)
	if err != nil {
		return fmt.Errorf("failed to create kubeconfig directory: %w", err)
	}

	file, err := os.Create(kubeconfigpath)
	if err != nil {
		return fmt.Errorf("failed to open kubeconfig file: %w", err)
	}
	defer file.Close()

	_, err = file.Write([]byte(kubeconfigStr))
	if err != nil {
		return fmt.Errorf("failed writing kubeconfig to file: %w", err)
	}

	return nil
}

func WaitForCondition(pollingFunc func() (bool, error), timeout time.Duration, interval time.Duration) error {
	err := wait.PollImmediate(interval, timeout, func() (bool, error) {
		condition, err := pollingFunc()
		if err != nil {
			return false, err
		}
		return condition, nil
	})
	if err != nil {
		return fmt.Errorf("failed to wait for condition: %w", err)
	}
	return nil
}

// AzureCloudProvisioner implements the CloudProvision interface for azure.
type AzureCloudProvisioner struct {
}

// AzureInstallOverlay implements the InstallOverlay interface
type AzureInstallOverlay struct {
	overlay *KustomizeOverlay
}

func NewAzureCloudProvisioner(properties map[string]string) (CloudProvisioner, error) {
	if err := initAzureProperties(properties); err != nil {
		return nil, err
	}

	return &AzureCloudProvisioner{}, nil
}

func (p *AzureCloudProvisioner) CreateVPC(ctx context.Context, cfg *envconf.Config) error {
	log.Trace("CreateVPC()")
	return createResourceImpl()
}

func (p *AzureCloudProvisioner) DeleteVPC(ctx context.Context, cfg *envconf.Config) error {
	log.Trace("DeleteVPC()")
	return deleteResourceImpl()
}

func (p *AzureCloudProvisioner) CreateCluster(ctx context.Context, cfg *envconf.Config) error {
	log.Trace("CreateCluster()")
	agentPoolProperties := []containerservice.ManagedClusterAgentPoolProfile{
		{
			Name:               &AzureProps.NodeName,
			Count:              to.Int32Ptr(1),
			VMSize:             &AzureProps.InstanceSize,
			Mode:               containerservice.AgentPoolModeSystem,
			OsType:             containerservice.OSType(AzureProps.OsType),
			EnableNodePublicIP: to.BoolPtr(false),
			VnetSubnetID:       &AzureProps.SubnetID,
		},
	}

	servicePrincipalProfile := &containerservice.ManagedClusterServicePrincipalProfile{
		ClientID: &AzureProps.ClientID,
		Secret:   &AzureProps.ClientSecret,
	}

	ManagedClusterProperties := &containerservice.ManagedClusterProperties{
		ServicePrincipalProfile: servicePrincipalProfile,
		DNSPrefix:               to.StringPtr("caa"),
		AgentPoolProfiles: &[]containerservice.ManagedClusterAgentPoolProfile{
			agentPoolProperties[0],
		},
	}

	ManagedCluster := &containerservice.ManagedCluster{
		Location:                 to.StringPtr("eastus"),
		ManagedClusterProperties: ManagedClusterProperties,
	}

	// Create the cluster
	log.Infof("Creating cluster %s.\n", AzureProps.ClusterName)
	_, err := AzureProps.ManagedAksClient.CreateOrUpdate(context.Background(), AzureProps.ResourceGroupName, AzureProps.ClusterName, *ManagedCluster)
	if err != nil {
		log.Errorf("Failed to created cluster %s: %v.\n", AzureProps.ClusterName, err)
		return fmt.Errorf("Failed to created cluster: %w", err)
	}

	// check if cluster is ready after creation
	waitDuration := time.Duration(10) * time.Minute
	retryInterval := time.Duration(60) * time.Second
	log.Infof("Waiting for cluster %s to be available.\n", AzureProps.ClusterName)
	err = WaitForCondition(func() (bool, error) {
		return isAzureClusterReady(AzureProps.ResourceGroupName, AzureProps.ClusterName)
	}, waitDuration, retryInterval)

	if err != nil {
		log.Errorf("Failed waiting  cluster %s to be ready: %v.\n", AzureProps.ClusterName, err)
		return fmt.Errorf("waiting for cluster to be ready %w", err)
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get user home directory: %w", err)
	}

	kubeconfigdirpath := path.Join(home, ".kube")
	kubeconfigFilename := "config"
	kubeconfigPath := path.Join(home, ".kube", kubeconfigFilename)

	log.Infof("Sync cluster kubeconfig with current config context")
	if err = syncKubeconfig(kubeconfigdirpath, kubeconfigPath); err != nil {
		return fmt.Errorf("Failed to sync kubeconfig to %s: %w", kubeconfigPath, err)
	}

	cfg.WithKubeconfigFile(kubeconfigPath)

	// Use cli to label nodes until label is changed from "node-role.kubernetes.io/worker" to "node.kubernetes.io/worker"
	cmd := exec.Command("kubectl", "label", "nodes", "--all", fmt.Sprintf("%s=%s", "node-role.kubernetes.io/worker", ""))
	cmd.Env = append(cmd.Env, fmt.Sprintf("KUBECONFIG="+kubeconfigPath))

	_, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("labeling nodes: %w", err)
	}
	log.Info("Nodes labeled successfully.")

	return nil
}

func (p *AzureCloudProvisioner) DeleteCluster(ctx context.Context, cfg *envconf.Config) error {
	log.Trace("DeleteCluster()")
	log.Infof("Deleting Cluster %s.\n", AzureProps.ClusterName)
	_, err := AzureProps.ManagedAksClient.Delete(context.Background(), AzureProps.ResourceGroupName, AzureProps.ClusterName)
	if err != nil {
		return fmt.Errorf("Failed deleting cluster %s: %w", AzureProps.ResourceGroupName, err)
	}

	// check if cluster is ready after creation
	waitDuration := time.Duration(10) * time.Minute
	retryInterval := time.Duration(60) * time.Second
	log.Infof("Waiting for cluster %s to be removed...\n", AzureProps.ClusterName)
	err = WaitForCondition(func() (bool, error) {
		return isClusterDeleted(AzureProps.ResourceGroupName, AzureProps.ClusterName)
	}, waitDuration, retryInterval)

	if err != nil {
		return fmt.Errorf("waiting for cluster to be deleted %w", err)
	}

	return nil
}

func (p *AzureCloudProvisioner) GetProperties(ctx context.Context, cfg *envconf.Config) map[string]string {
	return map[string]string{
		"CLOUD_PROVIDER":        "azure",
		"AZURE_SUBSCRIPTION_ID": AzureProps.SubscriptionID,
		"AZURE_CLIENT_ID":       AzureProps.ClientID,
		"AZURE_CLIENT_SECRET":   AzureProps.ClientSecret,
		"AZURE_TENANT_ID":       AzureProps.TenantID,
		"AZURE_RESOURCE_GROUP":  AzureProps.ResourceGroupName,
		"CLUSTER_NAME":          AzureProps.ClusterName,
		"AZURE_REGION":          AzureProps.Location,
		"SSH_KEY_ID":            AzureProps.SshPrivateKey,
		"SSH_USERNAME":          AzureProps.SshUserName,
		"AZURE_IMAGE_ID":        AzureProps.ImageID,
		"AZURE_SUBNET_ID":       AzureProps.SubnetID,
		"AZURE_INSTANCE_SIZE":   AzureProps.InstanceSize,
	}
}

func (p *AzureCloudProvisioner) UploadPodvm(imagePath string, ctx context.Context, cfg *envconf.Config) error {
	log.Trace("UploadPodvm()")
	log.Trace("Image is uploaded via packer in case of azure")
	return nil
}

func isAzureKustomizeConfigMapKey(key string) bool {
	switch key {
	case "CLOUD_PROVIDER", "AZURE_SUBSCRIPTION_ID", "AZURE_REGION", "AZURE_INSTANCE_SIZE", "AZURE_RESOURCE_GROUP", "AZURE_SUBNET_ID", "AZURE_IMAGE_ID", "SSH_USERNAME":
		return true
	default:
		return false
	}
}

func isAzureKustomizeSecretKey(key string) bool {
	switch key {
	case "AZURE_CLIENT_ID", "AZURE_CLIENT_SECRET", "AZURE_TENANT_ID":
		return true
	default:
		return false
	}
}

func NewAzureInstallOverlay() (InstallOverlay, error) {
	overlay, err := NewKustomizeOverlay("../../install/overlays/azure")
	if err != nil {
		return nil, err
	}

	return &AzureInstallOverlay{
		overlay: overlay,
	}, nil
}

func (lio *AzureInstallOverlay) Apply(ctx context.Context, cfg *envconf.Config) error {
	return lio.overlay.Apply(ctx, cfg)
}

func (lio *AzureInstallOverlay) Delete(ctx context.Context, cfg *envconf.Config) error {
	return lio.overlay.Delete(ctx, cfg)
}

func (lio *AzureInstallOverlay) Edit(ctx context.Context, cfg *envconf.Config, properties map[string]string) error {
	var err error
	for k, v := range properties {
		// configMapGenerator
		if isAzureKustomizeConfigMapKey(k) {
			if err = lio.overlay.SetKustomizeConfigMapGeneratorLiteral("peer-pods-cm", k, v); err != nil {
				return err
			}
		}
		// secretGenerator
		if isAzureKustomizeSecretKey(k) {
			if err = lio.overlay.SetKustomizeSecretGeneratorLiteral("peer-pods-secret", k, v); err != nil {
				return err
			}
		}
		// ssh key id
		if k == "SSH_KEY_ID" {
			if err = lio.overlay.SetKustomizeSecretGeneratorFile("ssh-key-secret", v); err != nil {
				return err
			}
		}
	}

	if err = lio.overlay.YamlReload(); err != nil {
		return err
	}

	return nil
}
