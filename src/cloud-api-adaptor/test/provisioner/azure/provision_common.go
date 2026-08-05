// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package azure

import (
	"context"
	"encoding/binary"
	"fmt"
	"net/http"
	"net/netip"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/e2e-framework/pkg/envconf"

	pv "github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/test/provisioner"
	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-providers/util"
	"github.com/containerd/containerd/reference"
	log "github.com/sirupsen/logrus"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	armcompute "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v4"
	armcontainerservice "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice/v4"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/msi/armmsi"
	armnetwork "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v2"
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

const peerPodNetworkName = "peerpod"

// clusterVnet returns the VNET that AKS created for the cluster in its node
// resource group.
func clusterVnet(ctx context.Context, aksRg string) (*armnetwork.VirtualNetwork, error) {
	var vnet *armnetwork.VirtualNetwork

	pager := AzureProps.ManagedVnetClient.NewListPager(aksRg, nil)
	for pager.More() {
		nextResult, err := pager.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("getting VNETs of AKS %q: %w", AzureProps.ClusterName, err)
		}
		for _, v := range nextResult.Value {
			vnet = v
		}
	}

	if vnet == nil || vnet.ID == nil {
		return nil, fmt.Errorf("no VNET found in resource group %q", aksRg)
	}

	return vnet, nil
}

// nextPrefix returns the network that directly follows prefix, keeping the same
// mask length, e.g. 10.224.0.0/16 -> 10.225.0.0/16.
func nextPrefix(prefix netip.Prefix) (netip.Prefix, error) {
	addr := prefix.Masked().Addr()
	if !addr.Is4() {
		return netip.Prefix{}, fmt.Errorf("only IPv4 prefixes are supported, got %q", prefix)
	}

	next := binary.BigEndian.Uint32(addr.AsSlice()[:]) + 1<<(32-uint(prefix.Bits()))
	if next == 0 {
		return netip.Prefix{}, fmt.Errorf("no address space left after %q", prefix)
	}

	var buf [4]byte
	binary.BigEndian.PutUint32(buf[:], next)

	return netip.PrefixFrom(netip.AddrFrom4(buf), prefix.Bits()), nil
}

// peerPodPrefix picks a free network for the peer pod subnet, adjacent to the
// AKS node subnet and with the same size.
func peerPodPrefix(subnets []*armnetwork.Subnet) (netip.Prefix, error) {
	used := make([]netip.Prefix, 0, len(subnets))
	for _, subnet := range subnets {
		if subnet.Properties == nil || subnet.Properties.AddressPrefix == nil {
			continue
		}
		prefix, err := netip.ParsePrefix(*subnet.Properties.AddressPrefix)
		if err != nil {
			return netip.Prefix{}, fmt.Errorf("parsing address prefix of subnet %q: %w", *subnet.Name, err)
		}
		// Keep the prefix stable if the subnet is already there, so that
		// re-provisioning an existing cluster does not try to renumber it.
		if subnet.Name != nil && *subnet.Name == peerPodNetworkName {
			return prefix, nil
		}
		used = append(used, prefix)
	}

	if len(used) == 0 {
		return netip.Prefix{}, fmt.Errorf("no subnet with an address prefix found")
	}

	// Walk forward from the node subnet until we find a block that is free. The
	// VNET usually has spare space right after it, but a cluster may add subnets.
	candidate := used[0]
	for range len(used) + 1 {
		var err error
		if candidate, err = nextPrefix(candidate); err != nil {
			return netip.Prefix{}, err
		}

		if !slices.ContainsFunc(used, candidate.Overlaps) {
			return candidate, nil
		}
	}

	return netip.Prefix{}, fmt.Errorf("no free address space adjacent to %q", used[0])
}

// createPeerPodSubnet creates a subnet dedicated to peer pod VMs, with a NAT
// gateway attached so that the VMs can reach public OCI registries. It returns
// the ID of the subnet.
func createPeerPodSubnet(ctx context.Context, aksRg, vnetName string, subnets []*armnetwork.Subnet) (string, error) {
	prefix, err := peerPodPrefix(subnets)
	if err != nil {
		return "", fmt.Errorf("determining peer pod address prefix: %w", err)
	}

	log.Infof("Creating public IP %q in resource group %q", peerPodNetworkName, aksRg)
	ipPoller, err := AzureProps.ManagedPublicIPClient.BeginCreateOrUpdate(ctx, aksRg, peerPodNetworkName, armnetwork.PublicIPAddress{
		Location: to.Ptr(AzureProps.Location),
		SKU: &armnetwork.PublicIPAddressSKU{
			Name: to.Ptr(armnetwork.PublicIPAddressSKUNameStandard),
		},
		Properties: &armnetwork.PublicIPAddressPropertiesFormat{
			PublicIPAddressVersion:   to.Ptr(armnetwork.IPVersionIPv4),
			PublicIPAllocationMethod: to.Ptr(armnetwork.IPAllocationMethodStatic),
		},
	}, nil)
	if err != nil {
		return "", fmt.Errorf("creating public IP %q: %w", peerPodNetworkName, err)
	}

	publicIP, err := ipPoller.PollUntilDone(ctx, nil)
	if err != nil {
		return "", fmt.Errorf("waiting for public IP %q: %w", peerPodNetworkName, err)
	}

	log.Infof("Creating NAT gateway %q in resource group %q", peerPodNetworkName, aksRg)
	natPoller, err := AzureProps.ManagedNatGatewayClient.BeginCreateOrUpdate(ctx, aksRg, peerPodNetworkName, armnetwork.NatGateway{
		Location: to.Ptr(AzureProps.Location),
		SKU: &armnetwork.NatGatewaySKU{
			Name: to.Ptr(armnetwork.NatGatewaySKUNameStandard),
		},
		Properties: &armnetwork.NatGatewayPropertiesFormat{
			PublicIPAddresses: []*armnetwork.SubResource{{ID: publicIP.ID}},
		},
	}, nil)
	if err != nil {
		return "", fmt.Errorf("creating NAT gateway %q: %w", peerPodNetworkName, err)
	}

	natGateway, err := natPoller.PollUntilDone(ctx, nil)
	if err != nil {
		return "", fmt.Errorf("waiting for NAT gateway %q: %w", peerPodNetworkName, err)
	}

	log.Infof("Creating subnet %q (%s) in VNET %q", peerPodNetworkName, prefix, vnetName)
	subnetPoller, err := AzureProps.ManagedSubnetClient.BeginCreateOrUpdate(ctx, aksRg, vnetName, peerPodNetworkName, armnetwork.Subnet{
		Properties: &armnetwork.SubnetPropertiesFormat{
			AddressPrefix: to.Ptr(prefix.String()),
			NatGateway:    &armnetwork.SubResource{ID: natGateway.ID},
		},
	}, nil)
	if err != nil {
		return "", fmt.Errorf("creating subnet %q: %w", peerPodNetworkName, err)
	}

	subnet, err := subnetPoller.PollUntilDone(ctx, nil)
	if err != nil {
		return "", fmt.Errorf("waiting for subnet %q: %w", peerPodNetworkName, err)
	}

	log.Infof("Successfully created peer pod subnet %q (%s) with NAT gateway", peerPodNetworkName, prefix)

	return *subnet.ID, nil
}

// podVMNamePrefix is the prefix of the name of every pod VM instance, including
// the separator that GenerateInstanceName puts after it.
var podVMNamePrefix = util.PodVMNamePrefix + "-"

// vmAttachedToVnet reports whether any of the NICs of vm lives in the given VNET.
func vmAttachedToVnet(ctx context.Context, vm *armcompute.VirtualMachine, vnetID string) (bool, error) {
	if vm.Properties == nil || vm.Properties.NetworkProfile == nil {
		return false, nil
	}

	subnetPrefix := vnetID + "/subnets/"
	for _, nicRef := range vm.Properties.NetworkProfile.NetworkInterfaces {
		if nicRef.ID == nil {
			continue
		}
		// The last segment of a NIC id is its name. The NIC is created along
		// with the VM, so it lives in the same resource group.
		nicName := (*nicRef.ID)[strings.LastIndex(*nicRef.ID, "/")+1:]
		nic, err := AzureProps.ManagedNicClient.Get(ctx, AzureProps.ResourceGroupName, nicName, nil)
		if err != nil {
			return false, fmt.Errorf("getting network interface %q: %w", nicName, err)
		}

		if nic.Properties == nil {
			continue
		}
		for _, ipConfig := range nic.Properties.IPConfigurations {
			if ipConfig.Properties == nil || ipConfig.Properties.Subnet == nil || ipConfig.Properties.Subnet.ID == nil {
				continue
			}
			if strings.HasPrefix(*ipConfig.Properties.Subnet.ID, subnetPrefix) {
				return true, nil
			}
		}
	}

	return false, nil
}

// clusterPodVMs lists the pod VMs that belong to this cluster. Pod VMs are
// created in the shared resource group, which may be used by more than one
// cluster at a time, so they are matched on the VNET their NIC is attached to
// rather than on their name alone.
func clusterPodVMs(ctx context.Context, vnetID string) ([]string, error) {
	var names []string

	pager := AzureProps.ManagedVMClient.NewListPager(AzureProps.ResourceGroupName, nil)
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("listing VMs in resource group %q: %w", AzureProps.ResourceGroupName, err)
		}

		for _, vm := range page.Value {
			if vm.Name == nil || !strings.HasPrefix(*vm.Name, podVMNamePrefix) {
				continue
			}

			attached, err := vmAttachedToVnet(ctx, vm, vnetID)
			if err != nil {
				return nil, err
			}
			if attached {
				names = append(names, *vm.Name)
			}
		}
	}

	return names, nil
}

// CreatePodVMInstance is a no-op. Pod VMs are created on demand by the
// cloud-api-adaptor when a peer pod is scheduled.
func (p *AzureCloudProvisioner) CreatePodVMInstance(ctx context.Context, cfg *envconf.Config) error {
	return nil
}

// DeletePodVMInstance deletes pod VMs that the cloud-api-adaptor left behind.
// They are created in the shared resource group, which outlives the cluster, so
// they would leak if they were not cleaned up explicitly. This has to run before
// the cluster is deleted: the pod VM NICs are attached to the cluster VNET, which
// blocks the deletion of the node resource group while they still exist.
func (p *AzureCloudProvisioner) DeletePodVMInstance(ctx context.Context, cfg *envconf.Config) error {
	log.Trace("DeletePodVMInstance()")

	cluster, err := AzureProps.ManagedAksClient.Get(ctx, AzureProps.ResourceGroupName, AzureProps.ClusterName, nil)
	if err != nil {
		return fmt.Errorf("getting cluster %q: %w", AzureProps.ClusterName, err)
	}

	vnet, err := clusterVnet(ctx, *cluster.Properties.NodeResourceGroup)
	if err != nil {
		return fmt.Errorf("fetching cluster vnet: %w", err)
	}

	names, err := clusterPodVMs(ctx, *vnet.ID)
	if err != nil {
		return fmt.Errorf("listing pod VMs: %w", err)
	}

	if len(names) == 0 {
		log.Info("No pod VM instances left behind")
		return nil
	}

	// Delete the VMs concurrently, they are independent of each other and each
	// deletion takes a while. Their NICs and disks are configured to be deleted
	// along with the VM.
	log.Infof("Deleting %d pod VM instance(s): %v", len(names), names)
	pollers := make([]*runtime.Poller[armcompute.VirtualMachinesClientDeleteResponse], 0, len(names))
	for _, name := range names {
		poller, err := AzureProps.ManagedVMClient.BeginDelete(ctx, AzureProps.ResourceGroupName, name, nil)
		if err != nil {
			return fmt.Errorf("deleting pod VM %q: %w", name, err)
		}
		pollers = append(pollers, poller)
	}

	for i, poller := range pollers {
		if _, err := poller.PollUntilDone(ctx, nil); err != nil {
			return fmt.Errorf("waiting for pod VM %q to be deleted: %w", names[i], err)
		}
		log.Infof("Successfully deleted pod VM %q", names[i])
	}

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
	aksRg := *cluster.Properties.NodeResourceGroup

	virtualNetwork, err := clusterVnet(ctx, aksRg)
	if err != nil {
		return fmt.Errorf("fetching cluster vnet: %w", err)
	}
	vnetName := *virtualNetwork.Name

	subnets := virtualNetwork.Properties.Subnets
	if len(subnets) == 0 {
		return fmt.Errorf("no subnet found in the specified VNET: %q", vnetName)
	}

	// podvms need their own subnet which has outbound access to the internet
	// to pull images from public OCI registries
	peerPodSubnetID, err := createPeerPodSubnet(ctx, aksRg, vnetName, subnets)
	if err != nil {
		return fmt.Errorf("creating peer pod subnet: %w", err)
	}
	AzureProps.SubnetID = peerPodSubnetID

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
		"AZURE_INSTANCE_SIZES":  AzureProps.InstanceSizes,
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
	log.Trace("Image is uploaded via mkosi in case of azure")
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

func (a *AzureInstallChart) GetHelm() *pv.Helm { return a.Helm }

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
			a.Helm.OverrideValueMap["daemonset.podLabels"] = `{"azure.workload.identity/use":"true"}`
			log.Infof("Configuring helm: set pod label for workload identity")
		}
	}

	for k, v := range properties {
		switch k {
		case "AZURE_SUBSCRIPTION_ID", "AZURE_REGION", "AZURE_INSTANCE_SIZE", "AZURE_INSTANCE_SIZES", "AZURE_RESOURCE_GROUP", "AZURE_SUBNET_ID", "AZURE_IMAGE_ID", "INITDATA", "TAGS", "TUNNEL_TYPE", "VXLAN_PORT":
			log.Infof("Configuring helm: override value (%s)", k)
			a.Helm.OverrideProviderValues[k] = v
		case "AZURE_CLIENT_SECRET", "AZURE_TENANT_ID":
			log.Infof("Configuring helm: set secret (%s)", k)
			a.Helm.OverrideProviderSecrets[k] = v
		}
	}

	return nil
}
