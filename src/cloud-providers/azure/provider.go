// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package azure

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"log"
	"net"
	"net/netip"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	armcompute "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v4"
	armnetwork "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v2"
	retry "github.com/avast/retry-go/v4"
	provider "github.com/confidential-containers/cloud-api-adaptor/src/cloud-providers"
	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-providers/util"
	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-providers/util/cloudinit"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

var logger = log.New(log.Writer(), "[adaptor/cloud/azure] ", log.LstdFlags|log.Lmsgprefix)
var errNotReady = errors.New("address not ready")
var errNotFound = errors.New("VM name not found")

const (
	maxInstanceNameLen = 63
	sshPort            = "22"
	remoteFile         = "/media/cidata/user-data"
)

type azureProvider struct {
	azureClient   azcore.TokenCredential
	serviceConfig *Config
	vmPool        *VMPool
}

func NewProvider(config *Config) (provider.Provider, error) {

	logger.Printf("azure config %+v", config.Redact())

	azureClient, err := NewAzureClient(*config)
	if err != nil {
		logger.Printf("creating azure client: %v", err)
		return nil, err
	}

	// Initialize SSH keys for authentication
	if err := initializeSSHKeys(config); err != nil {
		return nil, fmt.Errorf("failed to initialize SSH keys: %w", err)
	}

	provider := &azureProvider{
		azureClient:   azureClient,
		serviceConfig: config,
	}

	if err = provider.updateInstanceSizeSpecList(); err != nil {
		return nil, err
	}

	// Initialize VM pool if enabled
	vmPoolConfig := VMPoolConfig{
		Type:          VMPoolType(config.VMPoolType),
		PodRegex:      config.VMPoolPodRegex,
		InstanceTypes: config.VMPoolInstanceTypes,
		IPs:           config.VMPoolIPs,
	}

	provider.vmPool, err = NewVMPool(vmPoolConfig, provider)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize VM pool: %w", err)
	}
	if provider.vmPool != nil {
		total, available, _ := provider.vmPool.GetPoolStatus()
		logger.Printf("Initialized VM pool with %d VMs (%d available)", total, available)
	}

	return provider, nil
}

func parseIP(addr string) (*netip.Addr, error) {
	if addr == "" || addr == "0.0.0.0" {
		return nil, errNotReady
	}

	ip, err := netip.ParseAddr(addr)
	if err != nil {
		return nil, fmt.Errorf("parse pod vm IP %q: %w", addr, err)
	}
	return &ip, nil
}

func (p *azureProvider) getIPs(ctx context.Context, vm *armcompute.VirtualMachine) ([]netip.Addr, error) {
	nicClient, err := armnetwork.NewInterfacesClient(p.serviceConfig.SubscriptionId, p.azureClient, nil)
	if err != nil {
		return nil, fmt.Errorf("create network interfaces client: %w", err)
	}
	rgName := p.serviceConfig.ResourceGroupName
	nicRefs := vm.Properties.NetworkProfile.NetworkInterfaces

	var ips []netip.Addr
	var ipcs []*armnetwork.InterfaceIPConfiguration

	for _, nicRef := range nicRefs {
		nicId := *nicRef.ID
		// the last segment of a nic id is the name
		nicName := nicId[strings.LastIndex(nicId, "/")+1:]
		nic, err := nicClient.Get(ctx, rgName, nicName, nil)
		if err != nil {
			return nil, fmt.Errorf("get network interface: %w", err)
		}
		ipcs = append(ipcs, nic.Properties.IPConfigurations...)
	}

	// we add the public ip addresses as first elements, if available
	if p.serviceConfig.UsePublicIP {
		publicIPClient, err := armnetwork.NewPublicIPAddressesClient(p.serviceConfig.SubscriptionId, p.azureClient, nil)
		if err != nil {
			return nil, fmt.Errorf("create public ip client: %w", err)
		}
		for i, ipc := range ipcs {
			if ipc.Properties.PublicIPAddress == nil {
				continue
			}
			ipID := *ipc.Properties.PublicIPAddress.ID
			// the last segment of a ip id is the name
			ipName := ipID[strings.LastIndex(ipID, "/")+1:]
			publicIP, err := publicIPClient.Get(ctx, rgName, ipName, nil)
			if err != nil {
				return nil, fmt.Errorf("get public ip: %w", err)
			}
			addr := publicIP.Properties.IPAddress
			if addr != nil {
				ip, err := parseIP(*addr)
				if err != nil {
					return nil, err
				}
				ips = append(ips, *ip)
				logger.Printf("pod vm IP[%d][public]=%s", i, ip.String())
			}
		}
	}

	for i, ipc := range ipcs {
		addr := ipc.Properties.PrivateIPAddress
		if addr == nil {
			return nil, fmt.Errorf("private IP address not found in IP configuration %d", i)
		}
		ip, err := parseIP(*addr)
		if err != nil {
			return nil, err
		}
		ips = append(ips, *ip)
		logger.Printf("pod vm IP[%d][private]=%s", i, ip.String())
	}

	return ips, nil
}

func (p *azureProvider) create(ctx context.Context, parameters *armcompute.VirtualMachine) (*armcompute.VirtualMachine, error) {
	vmClient, err := armcompute.NewVirtualMachinesClient(p.serviceConfig.SubscriptionId, p.azureClient, nil)
	if err != nil {
		return nil, fmt.Errorf("creating VM client: %w", err)
	}

	vmName := *parameters.Properties.OSProfile.ComputerName

	pollerResponse, err := vmClient.BeginCreateOrUpdate(ctx, p.serviceConfig.ResourceGroupName, vmName, *parameters, nil)
	if err != nil {
		return nil, fmt.Errorf("beginning VM creation or update: %w", err)
	}

	resp, err := pollerResponse.PollUntilDone(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("waiting for the VM creation: %w", err)
	}

	logger.Printf("created VM successfully: %s", *resp.ID)

	return &resp.VirtualMachine, nil
}

func (p *azureProvider) buildNetworkConfig(nicName string) *armcompute.VirtualMachineNetworkInterfaceConfiguration {
	ipConfig := armcompute.VirtualMachineNetworkInterfaceIPConfiguration{
		Name: to.Ptr("ip-config"),
		Properties: &armcompute.VirtualMachineNetworkInterfaceIPConfigurationProperties{
			Subnet: &armcompute.SubResource{
				ID: to.Ptr(p.serviceConfig.SubnetId),
			},
		},
	}

	if p.serviceConfig.UsePublicIP {
		publicIpConfig := armcompute.VirtualMachinePublicIPAddressConfiguration{
			Name: to.Ptr(nicName),
			Properties: &armcompute.VirtualMachinePublicIPAddressConfigurationProperties{
				DeleteOption: to.Ptr(armcompute.DeleteOptionsDelete),
			},
		}
		ipConfig.Properties.PublicIPAddressConfiguration = &publicIpConfig
	}

	config := armcompute.VirtualMachineNetworkInterfaceConfiguration{
		Name: to.Ptr(nicName),
		Properties: &armcompute.VirtualMachineNetworkInterfaceConfigurationProperties{
			DeleteOption:     to.Ptr(armcompute.DeleteOptionsDelete),
			IPConfigurations: []*armcompute.VirtualMachineNetworkInterfaceIPConfiguration{&ipConfig},
		},
	}

	if p.serviceConfig.SecurityGroupId != "" {
		config.Properties.NetworkSecurityGroup = &armcompute.SubResource{
			ID: to.Ptr(p.serviceConfig.SecurityGroupId),
		}
	}

	return &config
}

func (p *azureProvider) CreateInstance(ctx context.Context, podName, sandboxID string, cloudConfig cloudinit.CloudConfigGenerator, spec provider.InstanceTypeSpec) (*provider.Instance, error) {

	// Determine the instance type that will be used
	instanceSize, err := p.selectInstanceType(ctx, spec)
	if err != nil {
		return nil, err
	}

	// Check if VM pool should be used
	if p.vmPool != nil && p.vmPool.ShouldUsePool(podName, instanceSize) {
		return p.AllocateFromVMPool(ctx, podName, sandboxID, cloudConfig, instanceSize)
	}

	instanceName := util.GenerateInstanceName(podName, sandboxID, maxInstanceNameLen)

	cloudConfigData, err := cloudConfig.Generate()
	if err != nil {
		return nil, err
	}

	diskName := fmt.Sprintf("%s-disk", instanceName)
	nicName := fmt.Sprintf("%s-net", instanceName)

	// Use the configured public key for SSH authentication
	sshBytes := []byte(strings.TrimSpace(p.serviceConfig.SSHPubKey))

	imageId := p.serviceConfig.ImageId

	if spec.Image != "" {
		logger.Printf("Choosing %s from annotation as the Azure Image for the PodVM image", spec.Image)
		imageId = spec.Image
	}

	vmParameters, err := p.getVMParameters(instanceSize, diskName, cloudConfigData, sshBytes, instanceName, nicName, imageId)
	if err != nil {
		return nil, err
	}

	logger.Printf("CreateInstance: name: %q", instanceName)

	vm, err := p.create(ctx, vmParameters)
	if err != nil {
		return nil, fmt.Errorf("Creating instance (%v): %s", vm, err)
	}

	ips, err := p.getIPs(ctx, vm)
	if err != nil {
		logger.Printf("getting IPs for the instance : %v ", err)
		return nil, err
	}

	if p.serviceConfig.EnableSftp {
		err = p.sendConfigFile(ctx, cloudConfigData, ips[0])
		if err != nil {
			return nil, fmt.Errorf("failed to send user data using ssh connection: %w", err)
		}
	}

	instance := &provider.Instance{
		ID:   *vm.ID,
		Name: instanceName,
		IPs:  ips,
	}

	return instance, nil
}

func (p *azureProvider) DeleteInstance(ctx context.Context, instanceID string) error {
	// Check if VM pool is enabled and try to deallocate from pool first
	if p.vmPool != nil {
		err := p.DeallocateFromVMPool(instanceID)
		if err == nil {
			logger.Printf("VM successfully returned to pool (not deleted): %s", instanceID)
			return nil // Successfully deallocated from pool - VM preserved for reuse
		}
		// If deallocation failed, it's not a pooled VM, continue with actual deletion
		logger.Printf("VM not found in pool, proceeding with actual deletion: %v", err)
	}

	vmClient, err := armcompute.NewVirtualMachinesClient(p.serviceConfig.SubscriptionId, p.azureClient, nil)
	if err != nil {
		return fmt.Errorf("creating VM client: %w", err)
	}

	// instanceID in the form of /subscriptions/<subID>/resourceGroups/<resource_name>/providers/Microsoft.Compute/virtualMachines/<VM_Name>.
	re := regexp.MustCompile(`^/subscriptions/[^/]+/resourceGroups/[^/]+/providers/Microsoft\.Compute/virtualMachines/(.*)$`)
	match := re.FindStringSubmatch(instanceID)
	if len(match) < 1 {
		logger.Print("finding VM name using regexp:", match)
		return errNotFound
	}

	vmName := match[1]

	pollerResponse, err := vmClient.BeginDelete(ctx, p.serviceConfig.ResourceGroupName, vmName, nil)
	if err != nil {
		return fmt.Errorf("beginning VM deletion: %w", err)
	}

	if _, err = pollerResponse.PollUntilDone(ctx, nil); err != nil {
		return fmt.Errorf("waiting for the VM deletion: %w", err)
	}

	logger.Printf("VM permanently deleted: %s", vmName)
	return nil
}

func (p *azureProvider) Teardown() error {
	return nil
}

func (p *azureProvider) ConfigVerifier() error {
	imageId := p.serviceConfig.ImageId
	if len(imageId) == 0 {
		return fmt.Errorf("ImageId is empty")
	}

	return nil
}

// Add SelectInstanceType method to select an instance type based on the memory and vcpu requirements
func (p *azureProvider) selectInstanceType(ctx context.Context, spec provider.InstanceTypeSpec) (string, error) {

	return provider.SelectInstanceTypeToUse(spec, p.serviceConfig.InstanceSizeSpecList, p.serviceConfig.InstanceSizes, p.serviceConfig.Size)
}

// Add a method to populate InstanceSizeSpecList for all the instanceSizes
// available in Azure
func (p *azureProvider) updateInstanceSizeSpecList() error {

	// Create a new instance of the Virtual Machine Sizes client
	vmSizesClient, err := armcompute.NewVirtualMachineSizesClient(p.serviceConfig.SubscriptionId, p.azureClient, nil)
	if err != nil {
		return fmt.Errorf("creating VM sizes client: %w", err)
	}
	// Get the instance sizes from the service config
	instanceSizes := p.serviceConfig.InstanceSizes

	// If instanceTypes is empty then populate it with the default instance type
	if len(instanceSizes) == 0 {
		instanceSizes = append(instanceSizes, p.serviceConfig.Size)
	}

	// Create a list of instancesizespec
	var instanceSizeSpecList []provider.InstanceTypeSpec

	// TODO: Is there an optimal method for this?
	// Create NewListPager to iterate over the instance types
	pager := vmSizesClient.NewListPager(p.serviceConfig.Region, &armcompute.VirtualMachineSizesClientListOptions{})

	// Iterate over the page and populate the instanceSizeSpecList for all the instanceSizes
	for pager.More() {
		nextResult, err := pager.NextPage(context.Background())
		if err != nil {
			return fmt.Errorf("getting next page of VM sizes: %w", err)
		}
		for _, vmSize := range nextResult.VirtualMachineSizeListResult.Value {
			if util.Contains(instanceSizes, *vmSize.Name) {
				instanceSizeSpecList = append(instanceSizeSpecList, provider.InstanceTypeSpec{InstanceType: *vmSize.Name, VCPUs: int64(*vmSize.NumberOfCores), Memory: int64(*vmSize.MemoryInMB)})
			}
		}
	}

	// Sort the InstanceSizeSpecList and update the serviceConfig
	p.serviceConfig.InstanceSizeSpecList = provider.SortInstanceTypesOnResources(instanceSizeSpecList)
	logger.Printf("instanceSizeSpecList (%v)", p.serviceConfig.InstanceSizeSpecList)
	return nil
}

func (p *azureProvider) getResourceTags() map[string]*string {
	tags := map[string]*string{}

	// Add custom tags from serviceConfig.Tags
	for k, v := range p.serviceConfig.Tags {
		tags[k] = to.Ptr(v)
	}
	return tags
}

func (p *azureProvider) getVMParameters(instanceSize, diskName, cloudConfig string, sshBytes []byte, instanceName, nicName string, imageId string) (*armcompute.VirtualMachine, error) {
	var userDataB64 string

	if p.serviceConfig.EnableSftp {
		// When SFTP is enabled, send minimal cloud-config with just SSH key
		sshKeyUserData := fmt.Sprintf(`#cloud-config
users:
  - name: %s
    ssh-authorized-keys:
      - %s
`, p.serviceConfig.SSHUserName, strings.TrimSpace(p.serviceConfig.SSHPubKey))
		userDataB64 = base64.StdEncoding.EncodeToString([]byte(sshKeyUserData))
	} else {
		// Normal mode: send the full cloud config
		userDataB64 = base64.StdEncoding.EncodeToString([]byte(cloudConfig))

		// Azure limits the base64 encrypted userData to 64KB.
		// Ref: https://learn.microsoft.com/en-us/azure/virtual-machines/user-data
		// If the b64EncData is greater than 64KB then return an error
		if len(userDataB64) > 64*1024 {
			return nil, fmt.Errorf("base64 encoded userData is greater than 64KB")
		}
	}
	var managedDiskParams *armcompute.ManagedDiskParameters
	var securityProfile *armcompute.SecurityProfile
	if !p.serviceConfig.DisableCVM {
		managedDiskParams = &armcompute.ManagedDiskParameters{
			StorageAccountType: to.Ptr(armcompute.StorageAccountTypesStandardLRS),
			SecurityProfile: &armcompute.VMDiskSecurityProfile{
				SecurityEncryptionType: to.Ptr(armcompute.SecurityEncryptionTypesVMGuestStateOnly),
			},
		}

		securityProfile = &armcompute.SecurityProfile{
			SecurityType: to.Ptr(armcompute.SecurityTypesConfidentialVM),
			UefiSettings: &armcompute.UefiSettings{
				SecureBootEnabled: to.Ptr(p.serviceConfig.EnableSecureBoot),
				VTpmEnabled:       to.Ptr(true),
			},
		}
	} else {
		managedDiskParams = &armcompute.ManagedDiskParameters{
			StorageAccountType: to.Ptr(armcompute.StorageAccountTypesStandardLRS),
		}

		securityProfile = nil
	}

	imgRef := &armcompute.ImageReference{
		ID: to.Ptr(imageId),
	}
	if strings.HasPrefix(imageId, "/CommunityGalleries/") {
		imgRef = &armcompute.ImageReference{
			CommunityGalleryImageID: to.Ptr(imageId),
		}
	}

	networkConfig := p.buildNetworkConfig(nicName)

	// Configure OS disk with optional root volume size
	osDisk := &armcompute.OSDisk{
		Name:         to.Ptr(diskName),
		CreateOption: to.Ptr(armcompute.DiskCreateOptionTypesFromImage),
		Caching:      to.Ptr(armcompute.CachingTypesReadWrite),
		DeleteOption: to.Ptr(armcompute.DiskDeleteOptionTypesDelete),
		ManagedDisk:  managedDiskParams,
	}

	// Set disk size if RootVolumeSize is configured
	if p.serviceConfig.RootVolumeSize > 0 {
		osDisk.DiskSizeGB = to.Ptr(int32(p.serviceConfig.RootVolumeSize))
		logger.Printf("Setting root volume size to %d GB", p.serviceConfig.RootVolumeSize)
	}

	vmParameters := armcompute.VirtualMachine{
		Location: to.Ptr(p.serviceConfig.Region),
		Properties: &armcompute.VirtualMachineProperties{
			HardwareProfile: &armcompute.HardwareProfile{
				VMSize: to.Ptr(armcompute.VirtualMachineSizeTypes(instanceSize)),
			},
			StorageProfile: &armcompute.StorageProfile{
				ImageReference: imgRef,
				OSDisk:         osDisk,
			},
			OSProfile: &armcompute.OSProfile{
				AdminUsername: to.Ptr(p.serviceConfig.SSHUserName),
				ComputerName:  to.Ptr(instanceName),
				LinuxConfiguration: &armcompute.LinuxConfiguration{
					DisablePasswordAuthentication: to.Ptr(true),
					//TBD: replace with a suitable mechanism to use precreated SSH key
					SSH: &armcompute.SSHConfiguration{
						PublicKeys: []*armcompute.SSHPublicKey{{
							Path:    to.Ptr(fmt.Sprintf("/home/%s/.ssh/authorized_keys", p.serviceConfig.SSHUserName)),
							KeyData: to.Ptr(string(sshBytes)),
						}},
					},
				},
			},
			NetworkProfile: &armcompute.NetworkProfile{
				NetworkAPIVersion:              to.Ptr(armcompute.NetworkAPIVersionTwoThousandTwenty1101),
				NetworkInterfaceConfigurations: []*armcompute.VirtualMachineNetworkInterfaceConfiguration{networkConfig},
			},
			SecurityProfile: securityProfile,
			DiagnosticsProfile: &armcompute.DiagnosticsProfile{
				BootDiagnostics: &armcompute.BootDiagnostics{
					Enabled: to.Ptr(true),
				},
			},
			UserData: to.Ptr(userDataB64),
		},
		Tags: p.getResourceTags(),
	}

	return &vmParameters, nil
}

func (p *azureProvider) sendConfigFile(ctx context.Context, data string, ip netip.Addr) error {
	server := ip.String() + ":" + sshPort

	signer, err := ssh.ParsePrivateKey([]byte(p.serviceConfig.SSHPrivKey))
	if err != nil {
		return fmt.Errorf("failed to parse private key: %w", err)
	}

	// Dynamically fetch the server host key
	hostKey, err := p.getServerHostKey(ctx, server)
	if err != nil {
		return fmt.Errorf("failed to fetch the server host key : %w", err)
	}

	sshConfig := &ssh.ClientConfig{
		User: p.serviceConfig.SSHUserName,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: ssh.FixedHostKey(hostKey),
		Timeout:         5 * time.Second,
	}

	logger.Printf("Trying to establish SSH connection to %s", server)
	sshClient, err := ssh.Dial("tcp", server, sshConfig)
	if err != nil {
		return fmt.Errorf("failed to establish ssh connection: %w", err)
	}

	logger.Printf("SSH connection to %s established successfully", server)
	defer sshClient.Close()

	sftpClient, err := sftp.NewClient(sshClient)
	if err != nil {
		return fmt.Errorf("failed to create sftp client: %w", err)
	}
	defer sftpClient.Close()

	file, err := sftpClient.Create(remoteFile)
	if err != nil {
		return fmt.Errorf("failed to create remote file %q: %w", remoteFile, err)
	}
	defer file.Close()

	if _, err := file.Write([]byte(data)); err != nil {
		return fmt.Errorf("failed to write data to remote file: %w", err)
	}

	fmt.Printf("Successfully transferred user data to remote file %s\n", remoteFile)
	return nil
}

// getServerHostKey will establish an initial unsecure connection to the VM
// to fetch the server host key to be used further for authentication to
// create an SSH connection.
func (p *azureProvider) getServerHostKey(ctx context.Context, addr string) (ssh.PublicKey, error) {
	var (
		conn    net.Conn
		err     error
		hostKey ssh.PublicKey
	)
	ctx, cancel := context.WithTimeout(ctx, 240*time.Second)
	defer cancel()

	err = retry.Do(
		func() error {
			logger.Printf("Trying to establish TCP connection to %s", addr)
			conn, err = net.Dial("tcp", addr)
			if err != nil {
				return err
			}
			return nil
		},
		retry.Context(ctx),
		retry.Attempts(0),
		retry.MaxDelay(10*time.Second),
	)

	if err != nil {
		return nil, fmt.Errorf("failed to establish TCP connection: %w", err)
	}
	defer conn.Close()

	conf := &ssh.ClientConfig{
		User: p.serviceConfig.SSHUserName,
		HostKeyCallback: func(hostname string, remote net.Addr, key ssh.PublicKey) error {
			hostKey = key
			return nil
		},
		Timeout: 5 * time.Second,
	}

	_, _, _, _ = ssh.NewClientConn(conn, addr, conf)
	if hostKey == nil {
		return nil, fmt.Errorf("SSH handshake failed: %w", err)
	}
	return hostKey, nil
}

// initializeSSHKeys sets up SSH keys for authentication
// If SSH keys are provided, use them. Otherwise auto-generate for SFTP.
func initializeSSHKeys(config *Config) error {
	// Try to read SSH keys from file paths
	if config.SSHPubKeyPath != "" {
		// Read public key from file
		pubKeyData, err := os.ReadFile(config.SSHPubKeyPath)
		if err != nil {
			logger.Printf("Could not read SSH public key from %s: %v", config.SSHPubKeyPath, err)
		} else {
			pubKeyContent := strings.TrimSpace(string(pubKeyData))
			// Validate SSH public key format
			if err := validateSSHPublicKey(pubKeyContent); err != nil {
				return fmt.Errorf("invalid SSH public key from file %s: %w", config.SSHPubKeyPath, err)
			}

			config.SSHPubKey = pubKeyContent
			logger.Printf("Using SSH public key from file: %s", config.SSHPubKeyPath)

			// If SFTP is enabled, try to read private key
			if config.EnableSftp {
				if config.SSHPrivKeyPath != "" {
					privKeyData, err := os.ReadFile(config.SSHPrivKeyPath)
					if err != nil {
						logger.Printf("Could not read SSH private key from %s: %v", config.SSHPrivKeyPath, err)
						logger.Printf("Public key provided but private key missing for SFTP, auto-generating new key pair")
					} else {
						privKeyContent := strings.TrimSpace(string(privKeyData))
						// Validate SSH private key format
						if err := validateSSHPrivateKey(privKeyContent); err != nil {
							return fmt.Errorf("invalid SSH private key from file %s: %w", config.SSHPrivKeyPath, err)
						}
						config.SSHPrivKey = privKeyContent
						logger.Printf("Using SSH key pair from files for SFTP")
						return nil
					}
				}
			} else {
				logger.Printf("Using SSH public key from file")
				return nil
			}
		}
	}

	// Auto-generate keys if none were successfully loaded from files
	if config.SSHPubKey == "" {
		if config.EnableSftp {
			logger.Printf("Auto-generating SSH key pair for SFTP")
		} else {
			logger.Printf("Auto-generating SSH key pair for Azure VM creation")
		}

		pubKeyString, privKeyString, err := generateSSHKeyPair()
		if err != nil {
			return fmt.Errorf("failed to generate SSH key pair: %w", err)
		}

		if pubKeyString == "" || privKeyString == "" {
			return fmt.Errorf("generated SSH key pair is empty")
		}

		config.SSHPubKey = pubKeyString
		if config.EnableSftp {
			config.SSHPrivKey = privKeyString
		}
		// Note: Private key not stored for non-SFTP mode as it's not needed
	}

	return nil
}

// validateSSHPublicKey validates that the provided string is a valid SSH public key
func validateSSHPublicKey(pubKey string) error {
	pubKey = strings.TrimSpace(pubKey)
	if pubKey == "" {
		return fmt.Errorf("public key is empty")
	}

	// Try to parse the public key
	_, _, _, _, err := ssh.ParseAuthorizedKey([]byte(pubKey))
	if err != nil {
		return fmt.Errorf("failed to parse SSH public key: %w", err)
	}

	return nil
}

// validateSSHPrivateKey validates that the provided string is a valid SSH private key
func validateSSHPrivateKey(privKey string) error {
	privKey = strings.TrimSpace(privKey)
	if privKey == "" {
		return fmt.Errorf("private key is empty")
	}

	// Try to parse the private key
	_, err := ssh.ParsePrivateKey([]byte(privKey))
	if err != nil {
		return fmt.Errorf("failed to parse SSH private key: %w", err)
	}

	return nil
}

func generateSSHKeyPair() (string, string, error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return "", "", fmt.Errorf("failed to generate private key: %w", err)
	}

	privateKeyPEM := pem.EncodeToMemory(
		&pem.Block{
			Type:  "RSA PRIVATE KEY",
			Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
		},
	)

	publicKey, err := ssh.NewPublicKey(&privateKey.PublicKey)
	if err != nil {
		return "", "", fmt.Errorf("failed to generate public key: %w", err)
	}

	publicKeyBytes := ssh.MarshalAuthorizedKey(publicKey)

	return string(publicKeyBytes), string(privateKeyPEM), nil
}

// getVMIPsFromNetworkProfile extracts IP addresses from VM's network profile
func (p *azureProvider) getVMIPsFromNetworkProfile(nicClient *armnetwork.InterfacesClient, publicIPClient *armnetwork.PublicIPAddressesClient, nicRefs []*armcompute.NetworkInterfaceReference, rgName string) ([]netip.Addr, error) {
	var ips []netip.Addr
	var ipcs []*armnetwork.InterfaceIPConfiguration

	// Get all IP configurations from network interfaces
	for _, nicRef := range nicRefs {
		nicId := *nicRef.ID
		nicName := nicId[strings.LastIndex(nicId, "/")+1:]
		nic, err := nicClient.Get(context.Background(), rgName, nicName, nil)
		if err != nil {
			return nil, fmt.Errorf("get network interface %s: %w", nicName, err)
		}
		ipcs = append(ipcs, nic.Properties.IPConfigurations...)
	}

	// Get public IPs first if UsePublicIP is enabled
	if p.serviceConfig.UsePublicIP {
		for _, ipc := range ipcs {
			if ipc.Properties.PublicIPAddress == nil {
				continue
			}
			ipID := *ipc.Properties.PublicIPAddress.ID
			ipName := ipID[strings.LastIndex(ipID, "/")+1:]
			publicIP, err := publicIPClient.Get(context.Background(), rgName, ipName, nil)
			if err != nil {
				return nil, fmt.Errorf("get public IP %s: %w", ipName, err)
			}
			addr := publicIP.Properties.IPAddress
			if addr != nil {
				ip, err := parseIP(*addr)
				if err != nil {
					return nil, err
				}
				ips = append(ips, *ip)
			}
		}
	}

	// Get private IPs
	for _, ipc := range ipcs {
		addr := ipc.Properties.PrivateIPAddress
		if addr == nil {
			continue
		}
		ip, err := parseIP(*addr)
		if err != nil {
			return nil, err
		}
		ips = append(ips, *ip)
	}

	return ips, nil
}
