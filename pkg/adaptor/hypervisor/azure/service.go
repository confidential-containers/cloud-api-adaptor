//go:build azure

package azure

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	armcompute "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v3"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork"

	"github.com/confidential-containers/cloud-api-adaptor/pkg/adaptor/hypervisor"
	"github.com/confidential-containers/cloud-api-adaptor/pkg/adaptor/proxy"
	daemon "github.com/confidential-containers/cloud-api-adaptor/pkg/forwarder"
	"github.com/confidential-containers/cloud-api-adaptor/pkg/podnetwork"
	"github.com/confidential-containers/cloud-api-adaptor/pkg/podnetwork/tunneler"
	"github.com/confidential-containers/cloud-api-adaptor/pkg/util/cloudinit"
	"github.com/confidential-containers/cloud-api-adaptor/pkg/util/hvutil"
	"github.com/containerd/containerd/pkg/cri/annotations"

	pb "github.com/kata-containers/kata-containers/src/runtime/protocols/hypervisor"
)

const (
	Version         = "0.0.0"
	DefaultUserName = "ubuntu"
)

type hypervisorService struct {
	azureClient      azcore.TokenCredential
	serviceConfig    *Config
	hypervisorConfig *hypervisor.Config
	sandboxes        map[sandboxID]*sandbox
	podsDir          string
	daemonPort       string
	nodeName         string
	workerNode       podnetwork.WorkerNode
	sync.Mutex
}

func newService(azureClient azcore.TokenCredential, config *Config, hypervisorConfig *hypervisor.Config, workerNode podnetwork.WorkerNode, podsDir, daemonPort string) pb.HypervisorService {
	logger.Printf("service config %+v", config)

	hostname, err := os.Hostname()
	if err != nil {
		panic(fmt.Errorf("failed to get hostname: %w", err))
	}

	i := strings.Index(hostname, ".")
	if i >= 0 {
		hostname = hostname[0:i]
	}

	return &hypervisorService{
		azureClient:      azureClient,
		serviceConfig:    config,
		hypervisorConfig: hypervisorConfig,
		sandboxes:        map[sandboxID]*sandbox{},
		podsDir:          podsDir,
		daemonPort:       daemonPort,
		nodeName:         hostname,
		workerNode:       workerNode,
	}
}

type sandboxID string

type sandbox struct {
	id               sandboxID
	pod              string
	namespace        string
	netNSPath        string
	podDirPath       string
	vsi              string
	vmName           string
	agentProxy       proxy.AgentProxy
	podNetworkConfig *tunneler.Config
}

func (s *hypervisorService) Version(ctx context.Context, req *pb.VersionRequest) (*pb.VersionResponse, error) {
	return &pb.VersionResponse{Version: Version}, nil
}

func (s *hypervisorService) CreateVM(ctx context.Context, req *pb.CreateVMRequest) (*pb.CreateVMResponse, error) {

	sid := sandboxID(req.Id)

	if sid == "" {
		return nil, errors.New("empty sandbox id")
	}
	s.Lock()
	defer s.Unlock()
	if _, exists := s.sandboxes[sid]; exists {
		return nil, fmt.Errorf("sandbox %s already exists", sid)
	}
	pod := hvutil.GetPodName(req.Annotations)
	if pod == "" {
		return nil, fmt.Errorf("pod name %s is missing in annotations", annotations.SandboxName)
	}

	namespace := hvutil.GetPodNamespace(req.Annotations)
	if namespace == "" {
		return nil, fmt.Errorf("namespace name %s is missing in annotations", annotations.SandboxNamespace)
	}

	podDirPath := filepath.Join(s.podsDir, string(sid))
	if err := os.MkdirAll(podDirPath, os.ModePerm); err != nil {
		return nil, fmt.Errorf("failed to create a pod directory: %s: %w", podDirPath, err)
	}

	socketPath := filepath.Join(podDirPath, proxy.SocketName)

	netNSPath := req.NetworkNamespacePath

	podNetworkConfig, err := s.workerNode.Inspect(netNSPath)
	if err != nil {
		return nil, fmt.Errorf("failed to inspect netns %s: %w", netNSPath, err)
	}

	agentProxy := proxy.NewAgentProxy(socketPath, s.hypervisorConfig.CriSocketPath, s.hypervisorConfig.PauseImage)

	sandbox := &sandbox{
		id:               sid,
		pod:              pod,
		namespace:        namespace,
		netNSPath:        netNSPath,
		podDirPath:       podDirPath,
		agentProxy:       agentProxy,
		podNetworkConfig: podNetworkConfig,
	}
	s.sandboxes[sid] = sandbox
	logger.Printf("create a sandbox %s for pod %s in namespace %s (netns: %s)", req.Id, pod, namespace, sandbox.netNSPath)
	return &pb.CreateVMResponse{AgentSocketPath: socketPath}, nil
}

func (s *hypervisorService) StartVM(ctx context.Context, req *pb.StartVMRequest) (*pb.StartVMResponse, error) {
	sandbox, err := s.getSandbox(req.Id)
	if err != nil {
		return nil, err
	}

	daemonConfig := daemon.Config{
		PodNamespace: sandbox.namespace,
		PodName:      sandbox.pod,
		PodNetwork:   sandbox.podNetworkConfig,
	}
	daemonJSON, err := json.MarshalIndent(daemonConfig, "", "    ")
	if err != nil {
		return nil, err
	}

	// Store daemon.json in worker node for debugging
	if err = os.WriteFile(filepath.Join(sandbox.podDirPath, "daemon.json"), daemonJSON, 0666); err != nil {
		return nil, fmt.Errorf("failed to store daemon.json at %s: %w", sandbox.podDirPath, err)
	}
	logger.Printf("store daemon.json at %s", sandbox.podDirPath)

	cloudConfig := &cloudinit.CloudConfig{
		WriteFiles: []cloudinit.WriteFile{
			{
				Path:    daemon.DefaultConfigPath,
				Content: string(daemonJSON),
			},
		},
	}

	if authJSON, err := os.ReadFile(cloudinit.DefaultAuthfileSrcPath); err == nil {
		if json.Valid(authJSON) && (len(authJSON) < cloudinit.DefaultAuthfileLimit) {
			cloudConfig.WriteFiles = append(cloudConfig.WriteFiles,
				cloudinit.WriteFile{
					Path:    cloudinit.DefaultAuthfileDstPath,
					Content: cloudinit.AuthJSONToResourcesJSON(string(authJSON)),
				})
		} else if len(authJSON) >= cloudinit.DefaultAuthfileLimit {
			logger.Printf("Credentials file size (%d) is too large to use as userdata, ignored", len(authJSON))
		} else {
			logger.Printf("Credentials file is not in a valid Json format, ignored")
		}
	}

	userData, err := cloudConfig.Generate()
	if err != nil {
		return nil, err
	}

	//Convert userData to base64
	userDataEnc := base64.StdEncoding.EncodeToString([]byte(userData))

	// TODO: Specify the maximum instance name length in Azure
	vmName := hvutil.CreateInstanceName(sandbox.pod, string(sandbox.id), 0)
	diskName := fmt.Sprintf("%s-disk", vmName)
	nicName := fmt.Sprintf("%s-net", vmName)

	// Set the vm name to the sandbox early for cleanup purposes
	sandbox.vmName = vmName

	// Get NIC using subnet and allow ports on the ssh group
	vmNIC, err := s.createNetworkInterface(ctx, nicName)
	if err != nil {
		err = fmt.Errorf("creating VM network interface: %w", err)
		logger.Printf("%v", err)
		return nil, err
	}

	// require ssh key for authentication on linux
	sshPublicKeyPath := os.ExpandEnv(s.serviceConfig.SSHKeyPath)
	var sshBytes []byte
	if _, err := os.Stat(sshPublicKeyPath); err == nil {
		sshBytes, err = ioutil.ReadFile(sshPublicKeyPath)
		if err != nil {
			err = fmt.Errorf("reading ssh public key file: %w", err)
			logger.Printf("%v", err)
			return nil, err
		}
	} else {
		err = fmt.Errorf("ssh public key: %w", err)
		logger.Printf("%v", err)
		return nil, err
	}

	vmParameters := armcompute.VirtualMachine{
		Location: to.Ptr(s.serviceConfig.Region),
		Properties: &armcompute.VirtualMachineProperties{
			HardwareProfile: &armcompute.HardwareProfile{
				VMSize: to.Ptr(armcompute.VirtualMachineSizeTypes(s.serviceConfig.Size)),
			},
			StorageProfile: &armcompute.StorageProfile{
				ImageReference: &armcompute.ImageReference{
					ID: to.Ptr(s.serviceConfig.ImageId),
				},
				OSDisk: &armcompute.OSDisk{
					Name:         to.Ptr(diskName),
					CreateOption: to.Ptr(armcompute.DiskCreateOptionTypesFromImage),
					Caching:      to.Ptr(armcompute.CachingTypesReadWrite),
					ManagedDisk: &armcompute.ManagedDiskParameters{
						StorageAccountType: to.Ptr(armcompute.StorageAccountTypesStandardLRS),
					},
				},
			},
			OSProfile: &armcompute.OSProfile{
				AdminUsername: to.Ptr(DefaultUserName),
				ComputerName:  to.Ptr(vmName),
				CustomData:    to.Ptr(userDataEnc),
				LinuxConfiguration: &armcompute.LinuxConfiguration{
					DisablePasswordAuthentication: to.Ptr(true),
					//TBD: replace with a suitable mechanism to use precreated SSH key
					SSH: &armcompute.SSHConfiguration{
						PublicKeys: []*armcompute.SSHPublicKey{{
							Path:    to.Ptr(fmt.Sprintf("/home/%s/.ssh/authorized_keys", DefaultUserName)),
							KeyData: to.Ptr(string(sshBytes)),
						}},
					},
				},
			},
			NetworkProfile: &armcompute.NetworkProfile{
				NetworkInterfaces: []*armcompute.NetworkInterfaceReference{
					{ID: vmNIC.ID},
				},
			},
		},
	}

	vm, err := CreateInstance(context.TODO(), s, &vmParameters)
	if err != nil {
		err = fmt.Errorf("Creating instance returned error: %s", err)
		logger.Printf("%v", err)
		return nil, err
	}

	// Set vsi to instance id
	sandbox.vsi = *vm.ID

	logger.Printf("created an instance %s for sandbox %s", *vm.Name, req.Id)

	podNodeIPs, err := getIPs(vmNIC)
	if err != nil {
		err = fmt.Errorf("failed to get IPs for the instance : %w", err)
		logger.Printf("%v", err)
		return nil, err
	}

	if err := s.workerNode.Setup(sandbox.netNSPath, podNodeIPs, sandbox.podNetworkConfig); err != nil {
		err = fmt.Errorf("failed to set up pod network tunnel on netns %s: %w", sandbox.netNSPath, err)
		logger.Printf("%v", err)
		return nil, err
	}

	serverURL := &url.URL{
		Scheme: "http",
		Host:   net.JoinHostPort(podNodeIPs[0].String(), s.daemonPort),
		Path:   daemon.AgentURLPath,
	}

	logger.Printf("server URL running the agent: %v", serverURL)

	errCh := make(chan error)
	go func() {
		defer close(errCh)

		if err := sandbox.agentProxy.Start(context.Background(), serverURL); err != nil {
			logger.Printf("error running agent proxy: %v", err)
			errCh <- err
		}
	}()

	select {
	case <-ctx.Done():
		_ = sandbox.agentProxy.Shutdown()
		return nil, ctx.Err()
	case err := <-errCh:
		return nil, err
	case <-sandbox.agentProxy.Ready():
	}

	logger.Printf("agent proxy is ready")
	return &pb.StartVMResponse{}, nil
}

func (s *hypervisorService) getSandbox(id string) (*sandbox, error) {

	sid := sandboxID(id)

	if id == "" {
		return nil, errors.New("empty sandbox id")
	}
	s.Lock()
	defer s.Unlock()
	if _, exists := s.sandboxes[sid]; !exists {
		return nil, fmt.Errorf("sandbox %s does not exist", sid)
	}
	return s.sandboxes[sid], nil
}

func getIPs(nic *armnetwork.Interface) ([]net.IP, error) {
	var podNodeIPs []net.IP

	for _, ipc := range nic.Properties.IPConfigurations {
		podNodeIPs = append(podNodeIPs, net.ParseIP(*ipc.Properties.PrivateIPAddress))
	}

	return podNodeIPs, nil
}

func (s *hypervisorService) deleteInstance(ctx context.Context, vmName string) error {

	if err := DeleteInstance(ctx, s, vmName); err != nil {
		err = fmt.Errorf("failed to delete an instance: %w", err)
		logger.Printf("%v", err)
		return err
	}

	logger.Printf("deleted an instance %s", vmName)

	diskName := fmt.Sprintf("%s-disk", vmName)
	if err := s.deleteDisk(ctx, diskName); err != nil {
		err = fmt.Errorf("failed to delete disk: %w", err)
		logger.Print(err)
		return err
	}

	nicName := fmt.Sprintf("%s-net", vmName)
	if err := s.deleteNetworkInterface(ctx, nicName); err != nil {
		err = fmt.Errorf("failed to delete network interface: %w", err)
		logger.Print(err)
		return err
	}

	return nil
}

func (s *hypervisorService) StopVM(ctx context.Context, req *pb.StopVMRequest) (*pb.StopVMResponse, error) {
	sandbox, err := s.getSandbox(req.Id)
	if err != nil {
		return nil, err
	}

	if err := sandbox.agentProxy.Shutdown(); err != nil {
		logger.Printf("failed to stop agent proxy: %v", err)
	}

	if err := s.deleteInstance(ctx, sandbox.vmName); err != nil {
		return nil, err
	}

	if err := s.workerNode.Teardown(sandbox.netNSPath, sandbox.podNetworkConfig); err != nil {
		return nil, fmt.Errorf("failed to tear down netns %s: %w", sandbox.netNSPath, err)
	}

	return &pb.StopVMResponse{}, nil
}

func (s *hypervisorService) createNetworkInterface(ctx context.Context, nicName string) (*armnetwork.Interface, error) {
	nicClient, err := armnetwork.NewInterfacesClient(s.serviceConfig.SubscriptionId, s.azureClient, nil)
	if err != nil {
		return nil, fmt.Errorf("creating network interfaces client: %w", err)
	}

	parameters := armnetwork.Interface{
		Location: to.Ptr(s.serviceConfig.Region),
		Properties: &armnetwork.InterfacePropertiesFormat{
			IPConfigurations: []*armnetwork.InterfaceIPConfiguration{
				{
					Name: to.Ptr(fmt.Sprintf("%s-ipConfig", nicName)),
					Properties: &armnetwork.InterfaceIPConfigurationPropertiesFormat{
						PrivateIPAllocationMethod: to.Ptr(armnetwork.IPAllocationMethodDynamic),
						Subnet: &armnetwork.Subnet{
							ID: to.Ptr(s.serviceConfig.SubnetId),
						},
					},
				},
			},
			NetworkSecurityGroup: &armnetwork.SecurityGroup{
				ID: to.Ptr(s.serviceConfig.SecurityGroupId),
			},
		},
	}

	pollerResponse, err := nicClient.BeginCreateOrUpdate(ctx, s.serviceConfig.ResourceGroupName, nicName, parameters, nil)
	if err != nil {
		return nil, fmt.Errorf("beginning creation or update of network interface: %w", err)
	}

	resp, err := pollerResponse.PollUntilDone(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("polling network interface creation: %w", err)
	}

	return &resp.Interface, nil
}

func (s *hypervisorService) deleteNetworkInterface(ctx context.Context, nicName string) error {
	nicClient, err := armnetwork.NewInterfacesClient(s.serviceConfig.SubscriptionId, s.azureClient, nil)
	if err != nil {
		return fmt.Errorf("creating network interfaces client: %w", err)
	}

	pollerResponse, err := nicClient.BeginDelete(ctx, s.serviceConfig.ResourceGroupName, nicName, nil)
	if err != nil {
		return fmt.Errorf("beginning deletion of network interface: %w", err)
	}

	_, err = pollerResponse.PollUntilDone(ctx, nil)
	if err != nil {
		return fmt.Errorf("polling network interface deletion: %w", err)
	}

	logger.Printf("deleted network interface successfully: %s", nicName)

	return nil
}

func (s *hypervisorService) deleteDisk(ctx context.Context, diskName string) error {
	diskClient, err := armcompute.NewDisksClient(s.serviceConfig.SubscriptionId, s.azureClient, nil)
	if err != nil {
		return fmt.Errorf("creating disk client: %w", err)
	}

	pollerResponse, err := diskClient.BeginDelete(ctx, s.serviceConfig.ResourceGroupName, diskName, nil)
	if err != nil {
		return fmt.Errorf("beginning disk deletion: %w", err)
	}

	_, err = pollerResponse.PollUntilDone(ctx, nil)
	if err != nil {
		return fmt.Errorf("waiting for the disk deletion: %w", err)
	}

	logger.Printf("deleted disk successfully: %s", diskName)

	return nil
}
