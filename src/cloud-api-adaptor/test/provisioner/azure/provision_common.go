// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package azure

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/e2e-framework/pkg/envconf"

	pv "github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/test/provisioner"
	"github.com/containerd/containerd/reference"
	log "github.com/sirupsen/logrus"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	armcontainerservice "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice/v4"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/msi/armmsi"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
	"github.com/Azure/go-autorest/autorest"
)

func createResourceGroup() error {
	if AzureProps.IsCIManaged {
		log.Infof("Resource group %q is CI managed. No need to create new one manually.", AzureProps.ResourceGroupName)

		_, err := AzureProps.ResourceGroupClient.Get(context.Background(), AzureProps.ResourceGroupName, nil)
		if err != nil {
			err = fmt.Errorf("getting resource group %s: %w", AzureProps.ResourceGroupName, err)
			log.Errorf("%v", err)
			return err
		}

		return nil
	}

	newRG := armresources.ResourceGroup{
		Location: &AzureProps.Location,
	}

	log.Infof("Creating Resource group %s.\n", AzureProps.ResourceGroupName)
	_, err := AzureProps.ResourceGroupClient.CreateOrUpdate(context.Background(), AzureProps.ResourceGroupName, newRG, nil)
	if err != nil {
		err = fmt.Errorf("creating resource group %s: %w", AzureProps.ResourceGroupName, err)
		log.Errorf("%v", err)
		return err
	}

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
	err := wait.PollUntilContextTimeout(context.Background(), interval, timeout, true, func(_ context.Context) (bool, error) {
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
	Overlay *pv.KustomizeOverlay
}

// AzureInstallChart implements the InstallChart interface
type AzureInstallChart struct {
	Helm *pv.Helm
}

func NewAzureCloudProvisioner(properties map[string]string) (pv.CloudProvisioner, error) {
	if err := initAzureProperties(properties); err != nil {
		return nil, err
	}

	if AzureProps.IsSelfManaged {
		return &AzureSelfManagedClusterProvisioner{}, nil
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
	namespace := pv.GetCAANamespace()
	serviceAccountName := "cloud-api-adaptor"

	if _, err := AzureProps.FederatedIdentityCredentialsClient.CreateOrUpdate(
		context.Background(),
		AzureProps.ResourceGroupName,
		AzureProps.ManagedIdentityName,
		AzureProps.FederatedCredentialName,
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

	log.Infof("Successfully created federated identity credential %q in resource group %q", AzureProps.FederatedCredentialName, AzureProps.ResourceGroupName)

	return nil
}

func deleteFederatedIdentityCredential() error {
	if _, err := AzureProps.FederatedIdentityCredentialsClient.Delete(
		context.Background(),
		AzureProps.ResourceGroupName,
		AzureProps.ManagedIdentityName,
		AzureProps.FederatedCredentialName,
		nil,
	); err != nil {
		return fmt.Errorf("deleting federated identity credential: %w", err)
	}

	log.Infof("Successfully deleted federated identity credential %q in resource group %q", AzureProps.FederatedCredentialName, AzureProps.ResourceGroupName)

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
					VMSize:             to.Ptr("Standard_F4s_v2"),
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

func getPropertiesImpl() map[string]string {
	props := map[string]string{
		"CLOUD_PROVIDER":        "azure",
		"AZURE_SUBSCRIPTION_ID": AzureProps.SubscriptionID,
		"AZURE_CLIENT_ID":       AzureProps.ClientID,
		"AZURE_RESOURCE_GROUP":  AzureProps.ResourceGroupName,
		"CLUSTER_NAME":          AzureProps.ClusterName,
		"AZURE_REGION":          AzureProps.Location,
		"AZURE_IMAGE_ID":        AzureProps.ImageID,
		"AZURE_SUBNET_ID":       AzureProps.SubnetID,
		"AZURE_INSTANCE_SIZE":   AzureProps.InstanceSize,
		"TAGS":                  AzureProps.Tags,
		"CONTAINER_RUNTIME":     AzureProps.ContainerRuntime,
		"TUNNEL_TYPE":           AzureProps.TunnelType,
		"VXLAN_PORT":            AzureProps.VxlanPort,
	}

	return props
}

func (p *AzureCloudProvisioner) GetProperties(ctx context.Context, cfg *envconf.Config) map[string]string {
	log.Trace("GetProperties()")
	return getPropertiesImpl()
}

func (p *AzureCloudProvisioner) UploadPodvm(imagePath string, ctx context.Context, cfg *envconf.Config) error {
	log.Trace("UploadPodvm()")
	log.Trace("Image is uploaded via packer in case of azure")
	return nil
}

func isAzureKustomizeConfigMapKey(key string) bool {
	switch key {
	case "CLOUD_PROVIDER", "AZURE_SUBSCRIPTION_ID", "AZURE_REGION", "AZURE_INSTANCE_SIZE", "AZURE_RESOURCE_GROUP", "AZURE_SUBNET_ID", "AZURE_IMAGE_ID", "INITDATA", "TAGS", "TUNNEL_TYPE", "VXLAN_PORT":
		return true
	default:
		return false
	}
}

func isAzureKustomizeSecretKey(key string) bool {
	return key == "AZURE_CLIENT_ID"
}

func NewAzureInstallOverlay(installDir, provider string) (pv.InstallOverlay, error) {
	overlay, err := pv.NewKustomizeOverlay(filepath.Join(installDir, "overlays", provider))
	if err != nil {
		return nil, err
	}

	return &AzureInstallOverlay{
		Overlay: overlay,
	}, nil
}

func (lio *AzureInstallOverlay) Apply(ctx context.Context, cfg *envconf.Config) error {
	return lio.Overlay.Apply(ctx, cfg)
}

func (lio *AzureInstallOverlay) Delete(ctx context.Context, cfg *envconf.Config) error {
	return lio.Overlay.Delete(ctx, cfg)
}

func (lio *AzureInstallOverlay) Edit(ctx context.Context, cfg *envconf.Config, properties map[string]string) error {
	var err error

	// If a custom image is defined then update it in the kustomization file.
	if AzureProps.CaaImage != "" {
		spec, err := reference.Parse(AzureProps.CaaImage)
		if err != nil {
			return fmt.Errorf("parsing image: %w", err)
		}

		log.Infof("Updating CAA image with %q", spec.Locator)
		if err = lio.Overlay.SetKustomizeImage("cloud-api-adaptor", "newName", spec.Locator); err != nil {
			return err
		}

		digest := spec.Digest()
		tag := spec.Object
		if i := strings.Index(tag, "@"); i >= 0 {
			tag = tag[:i]
		}

		log.Infof("Updating CAA image tag with %q", tag)
		if err = lio.Overlay.SetKustomizeImage("cloud-api-adaptor", "newTag", tag); err != nil {
			return err
		}

		log.Infof("Updating CAA image digest with %q", digest)
		if err = lio.Overlay.SetKustomizeImage("cloud-api-adaptor", "digest", digest.String()); err != nil {
			return err
		}
	}

	for k, v := range properties {
		// configMapGenerator
		if isAzureKustomizeConfigMapKey(k) {
			if err = lio.Overlay.SetKustomizeConfigMapGeneratorLiteral("peer-pods-cm", k, v); err != nil {
				return err
			}
		}
		// secretGenerator
		if isAzureKustomizeSecretKey(k) {
			if err = lio.Overlay.SetKustomizeSecretGeneratorLiteral("peer-pods-secret", k, v); err != nil {
				return err
			}
		}
	}

	// Replace the contents of the `workload-identity.yaml` with the client id
	workloadIdentity := filepath.Join(lio.Overlay.ConfigDir, "workload-identity.yaml")
	if err = replaceTextInFile(workloadIdentity, "00000000-0000-0000-0000-000000000000", AzureProps.ClientID); err != nil {
		return fmt.Errorf("replacing client id in workload-identity.yaml: %w", err)
	}

	if err = lio.Overlay.AddToPatchesStrategicMerge("workload-identity.yaml"); err != nil {
		return err
	}

	if err = lio.Overlay.YamlReload(); err != nil {
		return err
	}

	return nil
}

func NewAzureInstallChart(installDir, provider string) (pv.InstallChart, error) {
	chartPath := filepath.Join(installDir, "charts", "peerpods")
	namespace := pv.GetCAANamespace()
	releaseName := "peerpods"
	debug := false

	helm, err := pv.NewHelm(chartPath, namespace, releaseName, provider, debug)
	if err != nil {
		return nil, err
	}

	return &AzureInstallChart{
		Helm: helm,
	}, nil
}

func (a *AzureInstallChart) Install(ctx context.Context, cfg *envconf.Config) error {
	if err := a.Helm.Install(ctx, cfg); err != nil {
		return err
	}

	return nil
}

func (a *AzureInstallChart) Uninstall(ctx context.Context, cfg *envconf.Config) error {
	return a.Helm.Uninstall(ctx, cfg)
}

func (a *AzureInstallChart) Configure(ctx context.Context, cfg *envconf.Config, properties map[string]string) error {
	// Handle CAA image - parse it like kustomization does
	// CAA_IMAGE might be a full image reference (name:tag@digest) or just the name
	if AzureProps.CaaImage != "" {
		spec, err := reference.Parse(AzureProps.CaaImage)
		if err != nil {
			return fmt.Errorf("parsing CAA image: %w", err)
		}

		log.Infof("Configuring helm: CAA image %q", spec.Locator)
		a.Helm.OverrideValues["image.name"] = spec.Locator

		// For Helm, pass tag and digest together in image.tag
		// spec.Object contains the tag part (which may include @digest)
		tag := spec.Object
		if tag != "" {
			log.Infof("Configuring helm: CAA image tag %q", tag)
			a.Helm.OverrideValues["image.tag"] = tag
		}
	}

	if AzureProps.ClientID != "" {
		a.Helm.OverrideProviderSecrets["AZURE_CLIENT_ID"] = AzureProps.ClientID
		log.Infof("Configuring helm: set secret (AZURE_CLIENT_ID)")
		if properties["AZURE_CLIENT_SECRET"] == "" {
			// Set pod label for workload identity
			// The chart supports daemonset.podLabels which will add labels to the pod template
			// Note: For nested keys with dots/slashes, we need to use the escaped format
			// Helm will interpret this as a nested map: daemonset.podLabels["azure.workload.identity/use"] = "true"
			a.Helm.OverrideValues["daemonset.podLabels.azure\\.workload\\.identity/use"] = "true"
			log.Infof("Configuring helm: set pod label for workload identity")
		}
	}

	for k, v := range properties {
		if isAzureKustomizeConfigMapKey(k) {
			a.Helm.OverrideProviderValues[v] = properties[k]
			continue
		}
		if k == "AZURE_CLIENT_SECRET" || k == "AZURE_TENANT_ID" {
			log.Infof("Configuring helm: set secret (%s)", k)
			a.Helm.OverrideProviderSecrets[k] = properties[k]
		}
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
