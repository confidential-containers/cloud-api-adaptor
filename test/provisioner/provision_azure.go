//go:build azure

// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package provisioner

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"sigs.k8s.io/e2e-framework/pkg/envconf"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice/v4"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/msi/armmsi"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
	"github.com/Azure/go-autorest/autorest"
	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/util/wait"
)

func init() {
	newProvisionerFunctions["azure"] = NewAzureCloudProvisioner
	newInstallOverlayFunctions["azure"] = NewAzureInstallOverlay
}

func createResourceGroup() error {
	if AzureProps.IsCIManaged {
		log.Infof("Resource group %q is CI managed. No need to create new one manually.", AzureProps.ResourceGroupName)
		return nil
	}

	newRG := armresources.ResourceGroup{
		Location: &AzureProps.Location,
	}

	log.Infof("Creating Resource group %s.\n", AzureProps.ResourceGroupName)
	resourceGroupResp, err := AzureProps.ResourceGroupClient.CreateOrUpdate(context.Background(), AzureProps.ResourceGroupName, newRG, nil)
	if err != nil {
		err = fmt.Errorf("creating resource group %s: %w", AzureProps.ResourceGroupName, err)
		log.Errorf("%v", err)
		return err
	}

	AzureProps.ResourceGroup = &resourceGroupResp.ResourceGroup

	log.Infof("Successfully Created Resource group %s.\n", AzureProps.ResourceGroupName)
	return nil
}

func deleteResourceGroup() error {
	if AzureProps.IsCIManaged {
		log.Infof("Resource group %s is CI managed. No need to delete manually\n", AzureProps.ResourceGroupName)
		return nil
	}

	log.Infof("Deleting Resource group %s.\n", AzureProps.ResourceGroupName)
	pollerResponse, err := AzureProps.ResourceGroupClient.BeginDelete(context.Background(), AzureProps.ResourceGroupName, nil)
	if err != nil {
		if typedError, ok := err.(autorest.DetailedError); ok {
			if typedError.StatusCode == http.StatusNotFound {
				return nil
			}
		}
		err = fmt.Errorf("deleting resource group %s: %w", AzureProps.ResourceGroupName, err)
		log.Error(err)
		return err
	}

	_, err = pollerResponse.PollUntilDone(context.Background(), nil)
	if err != nil {
		return fmt.Errorf("timeout waiting for deletion of resource group %s: %w", AzureProps.ResourceGroupName, err)
	}

	log.Infof("Successfully deleted Resource group %s.\n", AzureProps.ResourceGroupName)
	return nil
}

func createResourceImpl() error {
	err := createResourceGroup()
	if err != nil {
		return fmt.Errorf("creating resource group: %w", err)
	}

	return nil
}

func deleteResourceImpl() error {
	return deleteResourceGroup()
}

func syncKubeconfig(kubeconfigdirpath string, kubeconfigpath string) error {
	credentialsresp, err := AzureProps.ManagedAksClient.ListClusterAdminCredentials(context.Background(), AzureProps.ResourceGroupName, AzureProps.ClusterName, nil)
	if err != nil {
		return fmt.Errorf("sync kubeconfig: %w", err)
	}

	kubeconfigStr := (credentialsresp.CredentialResults.Kubeconfigs)[0].Value

	err = os.MkdirAll(kubeconfigdirpath, 0755)
	if err != nil {
		return fmt.Errorf("creating kubeconfig directory: %w", err)
	}

	file, err := os.Create(kubeconfigpath)
	if err != nil {
		return fmt.Errorf("opening kubeconfig file: %w", err)
	}
	defer file.Close()

	_, err = file.Write([]byte(kubeconfigStr))
	if err != nil {
		return fmt.Errorf("writing kubeconfig to file: %w", err)
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
		return fmt.Errorf("waiting for condition: %w", err)
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

// CAA pods will use this identity to talk to the Azure API. This ensures we don't need to pass secrets.
func createFederatedIdentityCredential(aksOIDCIssuer string) error {
	namespace := "confidential-containers-system"
	serviceAccountName := "cloud-api-adaptor"

	if _, err := AzureProps.FederatedIdentityCredentialsClient.CreateOrUpdate(
		context.Background(),
		AzureProps.ResourceGroupName,
		AzureProps.ManagedIdentityName,
		AzureProps.federatedIdentityCredentialName,
		armmsi.FederatedIdentityCredential{
			Properties: &armmsi.FederatedIdentityCredentialProperties{
				Audiences: []*string{to.Ptr("api://AzureADTokenExchange")},
				Issuer:    to.Ptr(aksOIDCIssuer),
				Subject:   to.Ptr(fmt.Sprintf("system:serviceaccount:%s:%s", namespace, serviceAccountName)),
			},
		},
		nil,
	); err != nil {
		return fmt.Errorf("creating federated identity credential: %w", err)
	}

	log.Infof("Successfully created federated identity credential %q in resource group %q", AzureProps.federatedIdentityCredentialName, AzureProps.ResourceGroupName)

	return nil
}

func deleteFederatedIdentityCredential() error {
	if _, err := AzureProps.FederatedIdentityCredentialsClient.Delete(
		context.Background(),
		AzureProps.ResourceGroupName,
		AzureProps.ManagedIdentityName,
		AzureProps.federatedIdentityCredentialName,
		nil,
	); err != nil {
		return fmt.Errorf("deleting federated identity credential: %w", err)
	}

	log.Infof("Successfully deleted federated identity credential %q in resource group %q", AzureProps.federatedIdentityCredentialName, AzureProps.ResourceGroupName)

	return nil
}

func (p *AzureCloudProvisioner) CreateCluster(ctx context.Context, cfg *envconf.Config) error {
	log.Trace("CreateCluster()")

	managedcluster := &armcontainerservice.ManagedCluster{
		Location: to.Ptr(AzureProps.Location),
		Properties: &armcontainerservice.ManagedClusterProperties{
			DNSPrefix: to.Ptr("caa"),
			AgentPoolProfiles: []*armcontainerservice.ManagedClusterAgentPoolProfile{
				{

					Name:               to.Ptr(AzureProps.NodeName),
					Count:              to.Ptr[int32](1),
					VMSize:             to.Ptr(AzureProps.InstanceSize),
					Mode:               to.Ptr(armcontainerservice.AgentPoolModeSystem),
					OSType:             to.Ptr(armcontainerservice.OSType(AzureProps.OsType)),
					EnableNodePublicIP: to.Ptr(false),
					NodeLabels:         map[string]*string{"node.kubernetes.io/worker": to.Ptr("")},
				},
			},
			OidcIssuerProfile: &armcontainerservice.ManagedClusterOIDCIssuerProfile{
				Enabled: to.Ptr(true),
			},
			SecurityProfile: &armcontainerservice.ManagedClusterSecurityProfile{
				WorkloadIdentity: &armcontainerservice.ManagedClusterSecurityProfileWorkloadIdentity{
					Enabled: to.Ptr(true),
				},
			},
		},
		Identity: &armcontainerservice.ManagedClusterIdentity{
			Type: to.Ptr(armcontainerservice.ResourceIdentityTypeSystemAssigned),
		},
	}

	// Enable service principal when not using the az CLI authentication method.
	if !AzureProps.IsAzCliAuth {
		spProfile := &armcontainerservice.ManagedClusterServicePrincipalProfile{
			ClientID: to.Ptr(AzureProps.ClientID),
			Secret:   to.Ptr(AzureProps.ClientSecret),
		}

		managedcluster.Properties.ServicePrincipalProfile = spProfile
	}

	pollerResp, err := AzureProps.ManagedAksClient.BeginCreateOrUpdate(
		context.Background(),
		AzureProps.ResourceGroupName,
		AzureProps.ClusterName,
		*managedcluster,
		nil,
	)

	if err != nil {
		return err
	}

	_, err = pollerResp.PollUntilDone(ctx, nil)
	if err != nil {
		err = fmt.Errorf("waiting for cluster %q to be ready: %w.", AzureProps.ClusterName, err)
		log.Errorf("%v", err)
		return err
	}

	cluster, err := pollerResp.Result(ctx)
	if err != nil {
		return fmt.Errorf("getting cluster object: %w", err)
	}

	aksOIDCIssuer := *cluster.Properties.OidcIssuerProfile.IssuerURL
	if err := createFederatedIdentityCredential(aksOIDCIssuer); err != nil {
		return fmt.Errorf("creating federated identity credential: %w", err)
	}

	// Fetch aks-rg details
	aks_rg := *cluster.Properties.NodeResourceGroup

	// Fetch default vnet name
	vnetName := ""
	pager := AzureProps.ManagedVnetClient.NewListPager(aks_rg, nil)
	for pager.More() {
		nextResult, err := pager.NextPage(ctx)
		if err != nil {
			return fmt.Errorf("getting VNETs of AKS: %q: %w", AzureProps.ClusterName, err)
		}
		for _, v := range nextResult.Value {
			vnetName = *v.Name
		}
	}

	virtualNetwork, err := AzureProps.ManagedVnetClient.Get(ctx, aks_rg, vnetName, nil)
	if err != nil {
		return fmt.Errorf("failed to fetch vnet: %q: %v", vnetName, err)
	}

	SubnetsPtr := &virtualNetwork.Properties.Subnets
	if SubnetsPtr == nil || len(*SubnetsPtr) == 0 {
		return fmt.Errorf("no subnet found in the specified VNET: %q: %v", vnetName, err)
	}

	// Get the ID of the first subnet
	subnetID := (*SubnetsPtr)[0].ID
	AzureProps.SubnetID = *subnetID

	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("getting user home directory: %w", err)
	}

	kubeconfigdirpath := path.Join(home, ".kube")
	kubeconfigFilename := "config"
	kubeconfigPath := path.Join(home, ".kube", kubeconfigFilename)

	log.Infof("Sync cluster kubeconfig with current config context")
	if err = syncKubeconfig(kubeconfigdirpath, kubeconfigPath); err != nil {
		return fmt.Errorf("syncing kubeconfig to %s: %w", kubeconfigPath, err)
	}

	cfg.WithKubeconfigFile(kubeconfigPath)
	return nil
}

func (p *AzureCloudProvisioner) DeleteCluster(ctx context.Context, cfg *envconf.Config) error {
	log.Trace("DeleteCluster()")
	log.Infof("Deleting Cluster %s.\n", AzureProps.ClusterName)

	if err := deleteFederatedIdentityCredential(); err != nil {
		return fmt.Errorf("deleting federated identity credential: %w", err)
	}

	pollerResp, err := AzureProps.ManagedAksClient.BeginDelete(context.Background(), AzureProps.ResourceGroupName, AzureProps.ClusterName, nil)
	if err != nil {
		return fmt.Errorf("deleting cluster %s: %w", AzureProps.ResourceGroupName, err)
	}

	_, err = pollerResp.PollUntilDone(ctx, nil)
	if err != nil {
		err = fmt.Errorf("waiting for cluster %q to be deleted %w", AzureProps.ClusterName, err)
		log.Errorf("%v", err)
		return err
	}

	return nil
}

func (p *AzureCloudProvisioner) GetProperties(ctx context.Context, cfg *envconf.Config) map[string]string {
	props := map[string]string{
		"CLOUD_PROVIDER":        "azure",
		"AZURE_SUBSCRIPTION_ID": AzureProps.SubscriptionID,
		"AZURE_CLIENT_ID":       AzureProps.ClientID,
		"AZURE_RESOURCE_GROUP":  AzureProps.ResourceGroupName,
		"CLUSTER_NAME":          AzureProps.ClusterName,
		"AZURE_REGION":          AzureProps.Location,
		"SSH_KEY_ID":            AzureProps.SshPrivateKey,
		"SSH_USERNAME":          AzureProps.SshUserName,
		"AZURE_IMAGE_ID":        AzureProps.ImageID,
		"AZURE_SUBNET_ID":       AzureProps.SubnetID,
		"AZURE_INSTANCE_SIZE":   AzureProps.InstanceSize,
	}

	if AzureProps.ClientSecret != "" {
		props["AZURE_CLIENT_SECRET"] = AzureProps.ClientSecret
	}

	if AzureProps.TenantID != "" {
		props["AZURE_TENANT_ID"] = AzureProps.TenantID
	}

	return props
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

func NewAzureInstallOverlay(installDir string) (InstallOverlay, error) {
	overlay, err := NewKustomizeOverlay(filepath.Join(installDir, "overlays/azure"))
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

	// Replace the contents of the `workload-identity.yaml` with the client id
	workloadIdentity := filepath.Join(lio.overlay.configDir, "workload-identity.yaml")
	if err = replaceTextInFile(workloadIdentity, "00000000-0000-0000-0000-000000000000", AzureProps.ClientID); err != nil {
		return fmt.Errorf("replacing client id in workload-identity.yaml: %w", err)
	}

	if err = lio.overlay.AddToPatchesStrategicMerge("workload-identity.yaml"); err != nil {
		return err
	}

	if err = lio.overlay.YamlReload(); err != nil {
		return err
	}

	return nil
}

func replaceTextInFile(filePath, oldText, newText string) error {
	// Read the file content
	content, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	// Replace the old text with the new text
	newContent := strings.ReplaceAll(string(content), oldText, newText)

	// Write the modified content back to the file
	err = os.WriteFile(filePath, []byte(newContent), 0)
	if err != nil {
		return err
	}

	return nil
}
