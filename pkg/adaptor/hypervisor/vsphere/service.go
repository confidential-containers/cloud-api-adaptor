package vsphere

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/vmware/govmomi"

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
	Version                 = "0.0.0"
	VIM25LaunchTemplateName = "kata"
)

type hypervisorService struct {
	govmomiClient    *govmomi.Client
	serviceConfig    *Config
	hypervisorConfig *hypervisor.Config
	sandboxes        map[sandboxID]*sandbox
	podsDir          string
	daemonPort       string
	nodeName         string
	workerNode       podnetwork.WorkerNode
	sync.Mutex
}

func newService(govmomiClient *govmomi.Client, config *Config, hypervisorConfig *hypervisor.Config, workerNode podnetwork.WorkerNode, podsDir, daemonPort string) pb.HypervisorService {
	logger.Printf("service config %v", config)
	hostname, err := os.Hostname()
	if err != nil {
		panic(fmt.Errorf("failed to get hostname: %w", err))
	}

	i := strings.Index(hostname, ".")
	if i >= 0 {
		hostname = hostname[0:i]
	}

	return &hypervisorService{
		govmomiClient:    govmomiClient,
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
	agentProxy       proxy.AgentProxy
	podNetworkConfig *tunneler.Config
}

func (s *hypervisorService) Version(ctx context.Context, req *pb.VersionRequest) (*pb.VersionResponse, error) {
	return &pb.VersionResponse{Version: Version}, nil
}

func (s *hypervisorService) CreateVM(ctx context.Context, req *pb.CreateVMRequest) (*pb.CreateVMResponse, error) {

	logger.Printf("CreateVM")

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

	// TODO: Specify the maximum instance name length in vSphere
	vmname := hvutil.CreateInstanceName(sandbox.pod, string(sandbox.id), 0)

	logger.Printf("create a sandbox %s for pod %s in namespace %s (netns: %s)", req.Id, pod, namespace, sandbox.netNSPath)

	logger.Printf("CreateVM %s done", vmname)

	return &pb.CreateVMResponse{AgentSocketPath: socketPath}, nil
}

func (s *hypervisorService) StartVM(ctx context.Context, req *pb.StartVMRequest) (*pb.StartVMResponse, error) {

	err := CheckSessionWithRestore(ctx, s.serviceConfig, s.govmomiClient)
	if err != nil {
		logger.Printf("StartVM cannot find or create a new vcenter session")
		return nil, err
	}

	sandbox, err := s.getSandbox(req.Id)
	if err != nil {
		return nil, err
	}

	// TODO: Specify the maximum instance name length in vSphere
	vmname := hvutil.CreateInstanceName(sandbox.pod, string(sandbox.id), 0)
	logger.Printf("StartVM %s", vmname)

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

	result, err := CreateInstance(ctx, s.govmomiClient.Client, s.serviceConfig, vmname, userData)
	if err != nil {
		return nil, fmt.Errorf("creating instance for vm %s returned error: %s", vmname, err)
	}

	sandbox.vsi = result.uuid

	if err := s.workerNode.Setup(sandbox.netNSPath, result.ips, sandbox.podNetworkConfig); err != nil {
		return nil, fmt.Errorf("failed to set up pod network tunnel on netns %s: %w", sandbox.netNSPath, err)
	}

	serverURL := &url.URL{
		Scheme: "http",
		Host:   net.JoinHostPort(result.ips[0].String(), s.daemonPort),
		Path:   daemon.AgentURLPath,
	}

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

	logger.Printf("StartVM %s done", vmname)

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

func (s *hypervisorService) deleteSandbox(id string) error {
	sid := sandboxID(id)
	if id == "" {
		return errors.New("empty sandbox id")
	}
	s.Lock()
	defer s.Unlock()
	delete(s.sandboxes, sid)
	return nil
}

func (s *hypervisorService) deleteInstance(ctx context.Context, id string, vmname string) error {

	err := DeleteInstance(ctx, s.govmomiClient.Client, s.serviceConfig, vmname)
	if err != nil {
		logger.Printf("failed to delete the instance (%s): %v", id, err)
		return err
	}

	return nil
}

func (s *hypervisorService) StopVM(ctx context.Context, req *pb.StopVMRequest) (*pb.StopVMResponse, error) {

	err := CheckSessionWithRestore(ctx, s.serviceConfig, s.govmomiClient)
	if err != nil {
		logger.Printf("StopVM cannot find or create a new vcenter session")
		return nil, err
	}

	sandbox, err := s.getSandbox(req.Id)
	if err != nil {
		return nil, err
	}

	// TODO: Specify the maximum instance name length in vSphere
	vmname := hvutil.CreateInstanceName(sandbox.pod, string(sandbox.id), 0)

	logger.Printf("StopVM %s", vmname)

	if err := sandbox.agentProxy.Shutdown(); err != nil {
		logger.Printf("failed to stop agent proxy: %v", err)
	}

	if err := s.deleteInstance(ctx, sandbox.vsi, vmname); err != nil {
		return nil, err
	}

	if err := s.workerNode.Teardown(sandbox.netNSPath, sandbox.podNetworkConfig); err != nil {
		return nil, fmt.Errorf("failed to tear down netns %s: %w", sandbox.netNSPath, err)
	}

	err = s.deleteSandbox(req.Id)
	if err != nil {
		return nil, err
	}

	logger.Printf("StopVM %s done", vmname)
	return &pb.StopVMResponse{}, nil
}
