// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package cloud

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/containerd/containerd/pkg/cri/annotations"
	pb "github.com/kata-containers/kata-containers/src/runtime/protocols/hypervisor"

	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/pkg/adaptor/k8sops"
	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/pkg/adaptor/proxy"
	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/pkg/forwarder"
	. "github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/pkg/paths"
	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/pkg/podnetwork"
	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/pkg/securecomms/wnssh"
	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/pkg/util"
	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/pkg/util/tlsutil"
	provider "github.com/confidential-containers/cloud-api-adaptor/src/cloud-providers"
	putil "github.com/confidential-containers/cloud-api-adaptor/src/cloud-providers/util"
	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-providers/util/cloudinit"
)

const (
	Version = "0.0.0"
)

type ServerConfig struct {
	TLSConfig               *tlsutil.TLSConfig
	SocketPath              string
	PauseImage              string
	PodsDir                 string
	ForwarderPort           string
	ProxyTimeout            time.Duration
	Initdata                string
	EnableCloudConfigVerify bool
	SecureComms             bool
	SecureCommsTrustee      bool
	SecureCommsInbounds     string
	SecureCommsOutbounds    string
	SecureCommsPpInbounds   string
	SecureCommsPpOutbounds  string
	SecureCommsKbsAddress   string
	PeerPodsLimitPerNode    int
	RootVolumeSize          int
	EnableScratchDisk       bool
	EnableScratchEncryption bool
}

var logger = log.New(log.Writer(), "[adaptor/cloud] ", log.LstdFlags|log.Lmsgprefix)

func (s *cloudService) addSandbox(sid sandboxID, sandbox *sandbox) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if _, exists := s.sandboxes[sid]; exists {
		return fmt.Errorf("sandbox %s already exists", sid)
	}

	s.sandboxes[sid] = sandbox

	return nil
}

func (s *cloudService) getSandbox(sid sandboxID) (*sandbox, error) {
	if sid == "" {
		return nil, errors.New("empty sandbox id")
	}
	s.mutex.Lock()
	defer s.mutex.Unlock()
	if _, exists := s.sandboxes[sid]; !exists {
		return nil, fmt.Errorf("sandbox %s does not exist", sid)
	}
	return s.sandboxes[sid], nil
}

func (s *cloudService) removeSandbox(id sandboxID) error {
	sid := sandboxID(id)
	if id == "" {
		return errors.New("empty sandbox id")
	}
	s.mutex.Lock()
	defer s.mutex.Unlock()
	delete(s.sandboxes, sid)
	return nil
}

func NewService(provider provider.Provider, proxyFactory proxy.Factory, workerNode podnetwork.WorkerNode,
	serverConfig *ServerConfig, sshport string) Service {
	var err error
	var sshClient *wnssh.SshClient

	if serverConfig.SecureComms {
		inbounds := append([]string{"KUBERNETES_PHASE:KATAAGENT:0"}, strings.Split(serverConfig.SecureCommsInbounds, ",")...)

		var outbounds []string
		outbounds = append(outbounds, strings.Split(serverConfig.SecureCommsOutbounds, ",")...)
		if serverConfig.SecureCommsTrustee {
			outbounds = append(outbounds, "BOTH_PHASES:KBS:"+serverConfig.SecureCommsKbsAddress)
		}

		sshClient, err = wnssh.InitSshClient(inbounds, outbounds, serverConfig.SecureCommsTrustee, serverConfig.SecureCommsKbsAddress, sshport)
		if err != nil {
			log.Fatalf("InitSshClient %v", err)
		}
	}

	s := &cloudService{
		provider:     provider,
		proxyFactory: proxyFactory,
		sandboxes:    map[sandboxID]*sandbox{},
		serverConfig: serverConfig,
		workerNode:   workerNode,
		sshClient:    sshClient,
	}
	s.cond = sync.NewCond(&s.mutex)
	s.ppService, err = k8sops.NewPeerPodService()
	if err != nil {
		logger.Printf("failed to create PeerPodService, runtime failure may result in dangling resources %s", err)
	}

	return s
}

func (s *cloudService) Teardown() error {
	return s.provider.Teardown()
}

func (s *cloudService) ConfigVerifier() error {
	return s.provider.ConfigVerifier()
}

func (s *cloudService) setInstance(sid sandboxID, instanceID, instanceName string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	sandbox, ok := s.sandboxes[sid]
	if !ok {
		return fmt.Errorf("sandbox %s does not exist", sid)
	}

	sandbox.instanceID = instanceID
	sandbox.instanceName = instanceName

	s.cond.Broadcast()

	return nil
}

func (s *cloudService) GetInstanceID(ctx context.Context, podNamespace, podName string, wait bool) (string, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	for {
		for _, sandbox := range s.sandboxes {
			if sandbox.podNamespace == podNamespace && sandbox.podName == podName {
				return sandbox.instanceID, nil
			}
		}

		if !wait {
			return "", nil
		}

		select {
		case <-ctx.Done():
			return "", fmt.Errorf("getting instance ID: %w", ctx.Err())
		default:
		}

		s.cond.Wait()
	}
}

func (s *cloudService) Version(ctx context.Context, req *pb.VersionRequest) (*pb.VersionResponse, error) {
	return &pb.VersionResponse{Version: Version}, nil
}

func (s *cloudService) CreateVM(ctx context.Context, req *pb.CreateVMRequest) (res *pb.CreateVMResponse, err error) {
	defer func() {
		if err != nil {
			logger.Print(err)
		}
	}()

	sid := sandboxID(req.Id)

	if sid == "" {
		return nil, fmt.Errorf("empty sandbox id")
	}

	pod := util.GetPodName(req.Annotations)
	if pod == "" {
		return nil, fmt.Errorf("pod name %s is missing in annotations", annotations.SandboxName)
	}

	namespace := util.GetPodNamespace(req.Annotations)
	if namespace == "" {
		return nil, fmt.Errorf("namespace name %s is missing in annotations", annotations.SandboxNamespace)
	}

	// Get Pod VM instance type from annotations
	instanceType := util.GetInstanceTypeFromAnnotation(req.Annotations)

	// Get Pod VM cpu and memory from annotations
	vcpus, memory, gpus := util.GetPodvmResourcesFromAnnotation(req.Annotations)

	// Get Pod VM image from annotations
	image := util.GetImageFromAnnotation(req.Annotations)

	netNSPath := req.NetworkNamespacePath

	podNetworkConfig, err := s.workerNode.Inspect(netNSPath)
	if err != nil {
		return nil, fmt.Errorf("failed to inspect netns %s: %w", netNSPath, err)
	}

	if podNetworkConfig == nil {
		return nil, fmt.Errorf("pod network config is nil")
	}

	// Pod VM spec
	vmSpec := provider.InstanceTypeSpec{
		InstanceType: instanceType,
		VCPUs:        vcpus,
		Memory:       memory,
		GPUs:         gpus,
		Image:        image,
		MultiNic:     podNetworkConfig.ExternalNetViaPodVM,
	}

	// TODO: server name is also generated in each cloud provider, and possibly inconsistent
	serverName := putil.GenerateInstanceName(pod, string(sid), 63)

	podDir := filepath.Join(s.serverConfig.PodsDir, string(sid))
	if err := os.MkdirAll(podDir, os.ModePerm); err != nil {
		return nil, fmt.Errorf("creating a pod directory: %s, %w", podDir, err)
	}
	socketPath := filepath.Join(podDir, proxy.SocketName)

	agentProxy := s.proxyFactory.New(serverName, socketPath)

	daemonConfig := forwarder.Config{
		PodNamespace: namespace,
		PodName:      pod,
		PodNetwork:   podNetworkConfig,
		TLSClientCA:  string(agentProxy.ClientCA()),
	}

	if caService := agentProxy.CAService(); caService != nil {
		certPEM, keyPEM, err := caService.Issue(serverName)
		if err != nil {
			return nil, fmt.Errorf("creating TLS certificate for communication between worker node and peer pod VM")
		}

		daemonConfig.TLSServerCert = string(certPEM)
		daemonConfig.TLSServerKey = string(keyPEM)
	}

	var sshCi *wnssh.SshClientInstance

	if s.sshClient != nil {
		var ppPrivateKey []byte
		sshCi, ppPrivateKey = s.sshClient.InitPP(context.Background(), string(sid))
		if sshCi == nil {
			return nil, fmt.Errorf("failed sshClient.InitPP")
		}
		if !s.serverConfig.SecureCommsTrustee {
			daemonConfig.WnPublicKey = s.sshClient.GetWnPublicKey()
			daemonConfig.PpPrivateKey = ppPrivateKey
			daemonConfig.SecureCommsOutbounds = s.serverConfig.SecureCommsPpOutbounds
			daemonConfig.SecureCommsInbounds = s.serverConfig.SecureCommsPpInbounds
			daemonConfig.SecureComms = true
		}
	}

	// Set scratch disk configuration
	// If EnableScratchEncryption is set, then automatically set EnableScratchDisk
	// as well and log it
	daemonConfig.EnableScratchDisk = s.serverConfig.EnableScratchDisk
	if s.serverConfig.EnableScratchEncryption {
		daemonConfig.EnableScratchDisk = true
		daemonConfig.EnableScratchEncryption = true
		logger.Printf("EnableScratchEncryption is set, enabling scratch disk as well")
	}

	daemonJSON, err := json.MarshalIndent(daemonConfig, "", "    ")
	if err != nil {
		return nil, fmt.Errorf("generating JSON data: %w", err)
	}

	// Store daemon.json in worker node for debugging
	daemonJSONPath := filepath.Join(podDir, "daemon.json")
	if err := os.WriteFile(daemonJSONPath, daemonJSON, 0o666); err != nil {
		return nil, fmt.Errorf("storing %s: %w", daemonJSONPath, err)
	}
	logger.Printf("stored %s", daemonJSONPath)

	cloudConfig := &cloudinit.CloudConfig{
		WriteFiles: []cloudinit.WriteFile{
			{
				Path:    forwarder.DefaultConfigPath,
				Content: string(daemonJSON),
			},
		},
	}

	// Look up image pull secrets for the pod
	authJSON, err := k8sops.GetImagePullSecrets(pod, namespace)
	if err != nil {
		// Ignore errors getting secrets to match K8S behavior
		logger.Printf("error reading image pull secrets: %v", err)
	}
	if authJSON != nil {
		logger.Printf("successfully retrieved pod image pull secrets for %s/%s", namespace, pod)
		if len(authJSON) > cloudinit.DefaultAuthfileLimit {
			logger.Printf("Credentials file is too large to be included in cloud-config")
		} else {
			cloudConfig.WriteFiles = append(cloudConfig.WriteFiles, cloudinit.WriteFile{
				Path:    AuthFilePath,
				Content: string(authJSON),
			})
		}
	}

	initdataEnc := ""
	initdataEnc, err = util.GetInitdataFromAnnotation(req.Annotations)
	if err != nil {
		return nil, fmt.Errorf("failed to set initdata from annotation: %w", err)
	}

	// initdata in pod annotation is empty. use global initdata, if set
	if initdataEnc == "" && s.serverConfig.Initdata != "" {
		initdataEnc = s.serverConfig.Initdata
	}

	if initdataEnc != "" {
		cloudConfig.WriteFiles = append(cloudConfig.WriteFiles, cloudinit.WriteFile{
			Path:    InitDataPath,
			Content: initdataEnc,
		})
	}

	sandbox := &sandbox{
		id:            sid,
		podName:       pod,
		podNamespace:  namespace,
		netNSPath:     netNSPath,
		agentProxy:    agentProxy,
		podNetwork:    podNetworkConfig,
		cloudConfig:   cloudConfig,
		spec:          vmSpec,
		sshClientInst: sshCi,
	}

	if err := s.addSandbox(sid, sandbox); err != nil {
		return nil, fmt.Errorf("adding sandbox: %w", err)
	}

	logger.Printf("create a sandbox %s for pod %s in namespace %s (netns: %s)", req.Id, pod, namespace, sandbox.netNSPath)

	return &pb.CreateVMResponse{AgentSocketPath: socketPath}, nil
}

func (s *cloudService) StartVM(ctx context.Context, req *pb.StartVMRequest) (res *pb.StartVMResponse, err error) {
	defer func() {
		if err != nil {
			logger.Printf("error starting instance: %v", err)
		}
	}()

	sid := sandboxID(req.Id)

	sandbox, err := s.getSandbox(sid)
	if err != nil {
		return nil, fmt.Errorf("getting sandbox: %w", err)
	}

	instance, err := s.provider.CreateInstance(ctx, sandbox.podName, string(sid), sandbox.cloudConfig, sandbox.spec)
	if err != nil {
		return nil, fmt.Errorf("creating an instance : %w", err)
	}

	if s.ppService != nil {
		if err := s.ppService.OwnPeerPod(sandbox.podName, sandbox.podNamespace, instance.ID); err != nil {
			logger.Printf("failed to create PeerPod: %v", err)
		}
	}

	if err := s.setInstance(sid, instance.ID, instance.Name); err != nil {
		return nil, fmt.Errorf("setting instance: %w", err)
	}

	logger.Printf("created an instance %s for sandbox %s", instance.Name, sid)

	if len(instance.IPs) == 0 {
		return nil, fmt.Errorf("instance IP is not available")
	}

	instanceIP := instance.IPs[0].String()
	forwarderPort := s.serverConfig.ForwarderPort

	if s.sshClient != nil {
		if err := sandbox.sshClientInst.Start(instance.IPs); err != nil {
			return nil, fmt.Errorf("failed SshClientInstance.Start: %w", err)
		}

		// Set agentProxy
		instanceIP = "127.0.0.1"
		forwarderPort = sandbox.sshClientInst.GetPort("KATAAGENT")
	}

	if err := s.workerNode.Setup(sandbox.netNSPath, instance.IPs, sandbox.podNetwork); err != nil {
		return nil, fmt.Errorf("setting up pod network tunnel on netns %s: %w", sandbox.netNSPath, err)
	}

	serverURL := &url.URL{
		Scheme: "http",
		Host:   net.JoinHostPort(instanceIP, forwarderPort),
		Path:   forwarder.AgentURLPath,
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
		// Start VM operation interrupted (calling context canceled)
		logger.Printf("Error: start instance interrupted (%v). Cleaning up...", ctx.Err())
		if err := sandbox.agentProxy.Shutdown(); err != nil {
			logger.Printf("stopping agent proxy: %v", err)
		}
		return nil, ctx.Err()
	case err := <-errCh:
		return nil, err
	case <-sandbox.agentProxy.Ready():
	}

	logger.Print("agent proxy is ready")

	return &pb.StartVMResponse{}, nil
}

func (s *cloudService) StopVM(ctx context.Context, req *pb.StopVMRequest) (*pb.StopVMResponse, error) {
	sid := sandboxID(req.Id)

	sandbox, err := s.getSandbox(sid)
	if err != nil {
		err = fmt.Errorf("stopping VM: %v", err)
		logger.Print(err)
		return nil, err
	}

	if err := sandbox.agentProxy.Shutdown(); err != nil {
		logger.Printf("stopping agent proxy: %v", err)
	}

	if sandbox.sshClientInst != nil {
		sandbox.sshClientInst.DisconnectPP(string(sid))
	}

	if err := s.provider.DeleteInstance(ctx, sandbox.instanceID); err != nil {
		logger.Printf("Error deleting an instance %s: %v", sandbox.instanceID, err)
	} else if s.ppService != nil {
		if err := s.ppService.ReleasePeerPod(sandbox.podName, sandbox.podNamespace, sandbox.instanceID); err != nil {
			logger.Printf("failed to release PeerPod %v", err)
		}
	}

	if err := s.workerNode.Teardown(sandbox.netNSPath, sandbox.podNetwork); err != nil {
		logger.Printf("tearing down netns %s: %v", sandbox.netNSPath, err)
	}

	if err = s.removeSandbox(sid); err != nil {
		logger.Printf("removing sandbox %s: %v", sid, err)
	}

	return &pb.StopVMResponse{}, nil
}
