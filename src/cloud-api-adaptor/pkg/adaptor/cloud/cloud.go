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
	"sync"
	"time"

	"github.com/containerd/containerd/pkg/cri/annotations"
	pb "github.com/kata-containers/kata-containers/src/runtime/protocols/hypervisor"

	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/pkg/adaptor/k8sops"
	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/pkg/adaptor/proxy"
	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/pkg/adaptor/state"
	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/pkg/forwarder"
	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/pkg/paths"
	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/pkg/podnetwork"
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
	PeerPodsLimitPerNode    int
	RootVolumeSize          int
	EnableScratchSpace      bool
	// TLSMaterialPath is the path where TLS material is persisted across
	// CAA process restarts. Must survive process restarts (tmpfs is fine).
	// Default: /run/peerpod/tls-material.json
	TLSMaterialPath string
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

func (s *cloudService) reconnectToInstance(ctx context.Context, st *state.SandboxState) error {
	sid := sandboxID(st.SandboxID)

	// 1. Validate network namespace still exists
	if _, err := os.Stat(st.NetNSPath); err != nil {
		return fmt.Errorf("netns gone: %w", err)
	}

	// 2. Get instance IP from state
	if len(st.InstanceIPs) == 0 {
		return fmt.Errorf("no cached IPs")
	}
	instanceIP := st.InstanceIPs[0]

	// 3. Recreate socket path
	podDir := filepath.Join(s.serverConfig.PodsDir, st.SandboxID)
	socketPath := filepath.Join(podDir, proxy.SocketName)
	os.Remove(socketPath) // Remove stale socket

	// 4. Create new agent proxy
	agentProxy := s.proxyFactory.New(st.ServerName, socketPath)

	// 5. Build server URL
	forwarderPort := st.ForwarderPort
	if forwarderPort == "" {
		forwarderPort = s.serverConfig.ForwarderPort
	}
	serverURL := &url.URL{
		Scheme: "http",
		Host:   net.JoinHostPort(instanceIP, forwarderPort),
		Path:   forwarder.AgentURLPath,
	}

	// 6. Start proxy in background
	errCh := make(chan error)
	go func() {
		defer close(errCh)
		if err := agentProxy.Start(ctx, serverURL); err != nil {
			errCh <- err
		}
	}()

	// 7. Wait for ready or timeout
	// TODO: 30s may be too short for slow podvms under load, causing false
	// negatives that orphan running instances. Consider using ProxyTimeout
	// or a dedicated config, but note that longer timeouts block CAA startup
	// (N sandboxes × timeout each, non-cancellable).
	select {
	case <-time.After(30 * time.Second):
		if err := agentProxy.Shutdown(); err != nil {
			logger.Printf("agentProxy shutdown failed: %v", err)
		}
		return fmt.Errorf("timeout")
	case err := <-errCh:
		return fmt.Errorf("proxy failed: %w", err)
	case <-agentProxy.Ready():
	}

	// 8. Add to sandboxes map
	s.mutex.Lock()
	s.sandboxes[sid] = &sandbox{
		id:           sid,
		podName:      st.PodName,
		podNamespace: st.PodNamespace,
		netNSPath:    st.NetNSPath,
		instanceID:   st.InstanceID,
		instanceName: st.InstanceName,
		agentProxy:   agentProxy,
		restored:     true,
		podNetwork:   st.PodNetwork,
	}
	s.mutex.Unlock()

	return nil
}

func (s *cloudService) recoverSandboxes(ctx context.Context) {
	sandboxIDs, err := s.stateManager.List()
	if err != nil {
		logger.Printf("failed to list sandboxes: %v", err)
		return
	}

	if len(sandboxIDs) == 0 {
		return
	}

	logger.Printf("recovering %d sandboxes", len(sandboxIDs))

	var wg sync.WaitGroup
	for _, sid := range sandboxIDs {
		wg.Add(1)
		go func(sid string) {
			defer wg.Done()
			s.restoreSandbox(ctx, sid)
		}(sid)
	}
	wg.Wait()

	logger.Printf("sandbox recovery complete")
}

func (s *cloudService) restoreSandbox(ctx context.Context, sid string) {
	st, err := s.stateManager.Load(sid)
	if err != nil {
		logger.Printf("failed to load state for %s: %v (skipping)", sid, err)
		return
	}

	// Advance podIndex past restored indices to avoid VXLAN ID collisions
	// with tunnels that may still exist on the host.
	if st.PodNetwork != nil {
		podnetwork.SetMinPodIndex(st.PodNetwork.Index + 1)
	}

	if st.Running {
		if err := s.reconnectToInstance(ctx, st); err != nil {
			logger.Printf("failed to reconnect %s: %v (cleaning up stale state)", sid, err)
			s.cleanupSandboxState(sid, st)
		} else {
			logger.Printf("reconnected sandbox %s", sid)
		}
		return
	}

	logger.Printf("cleaning up incomplete sandbox %s", sid)
	s.cleanupSandboxState(sid, st)
}

func (s *cloudService) cleanupSandboxState(sid string, st *state.SandboxState) {
	if st.PodNetwork != nil {
		if err := s.workerNode.Teardown(st.NetNSPath, st.PodNetwork); err != nil {
			logger.Printf("network teardown for %s failed (non-fatal): %v", sid, err)
		}
	}
	if err := s.stateManager.Delete(sid); err != nil {
		logger.Printf("state delete for %s failed (non-fatal): %v", sid, err)
	}
}

func NewService(provider provider.Provider,
	proxyFactory proxy.Factory,
	workerNode podnetwork.WorkerNode,
	serverConfig *ServerConfig,
) Service {
	var err error

	s := &cloudService{
		provider:     provider,
		proxyFactory: proxyFactory,
		sandboxes:    map[sandboxID]*sandbox{},
		serverConfig: serverConfig,
		workerNode:   workerNode,
		stateManager: state.NewManager(serverConfig.PodsDir),
	}
	s.cond = sync.NewCond(&s.mutex)
	s.ppService, err = k8sops.NewPeerPodService()
	if err != nil {
		logger.Printf("failed to create PeerPodService, runtime failure may result in dangling resources %s", err)
	}

	// TODO: context.Background() makes recovery non-cancellable during shutdown.
	// Requires passing parent context from server.Start() through NewService.
	s.recoverSandboxes(context.Background())

	return s
}

func (s *cloudService) Teardown() error {
	// Gracefully shutdown all agent proxies to drain pending requests
	s.mutex.Lock()
	sandboxes := make([]*sandbox, 0, len(s.sandboxes))
	for _, sb := range s.sandboxes {
		sandboxes = append(sandboxes, sb)
	}
	s.mutex.Unlock()

	for _, sb := range sandboxes {
		if sb.agentProxy != nil {
			logger.Printf("draining sandbox %s", sb.id)
			if err := sb.agentProxy.Shutdown(); err != nil {
				logger.Printf("error shutting down agent proxy for sandbox %s: %v", sb.id, err)
			}
		}
	}

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

	// Idempotent: if sandbox was restored, return existing socket path
	s.mutex.Lock()
	if existing, ok := s.sandboxes[sid]; ok && existing.restored {
		socketPath := filepath.Join(s.serverConfig.PodsDir, string(sid), proxy.SocketName)
		s.mutex.Unlock()
		logger.Printf("sandbox %s already restored, returning existing socket", sid)
		return &pb.CreateVMResponse{AgentSocketPath: socketPath}, nil
	}
	s.mutex.Unlock()

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

	// Get Pod VM cpu, memory and gpu from annotations
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

	apfJSON, err := json.MarshalIndent(daemonConfig, "", "    ")
	if err != nil {
		return nil, fmt.Errorf("generating JSON data: %w", err)
	}

	// Store apf.json in worker node for debugging
	apfJSONPath := filepath.Join(podDir, "apf.json")
	if err := os.WriteFile(apfJSONPath, apfJSON, 0o666); err != nil {
		return nil, fmt.Errorf("storing %s: %w", apfJSONPath, err)
	}
	logger.Printf("stored %s", apfJSONPath)

	cloudConfig := &cloudinit.CloudConfig{
		WriteFiles: []cloudinit.WriteFile{
			{
				Path:    forwarder.DefaultConfigPath,
				Content: string(apfJSON),
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
				Path:    paths.AuthFilePath,
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
			Path:    paths.InitDataPath,
			Content: initdataEnc,
		})
	}

	// Set encrypted scratch space config
	if s.serverConfig.EnableScratchSpace {
		cloudConfig.WriteFiles = append(cloudConfig.WriteFiles, cloudinit.WriteFile{
			Path:    paths.ScratchSpacePath,
			Content: "",
		})
	}

	sandbox := &sandbox{
		id:           sid,
		podName:      pod,
		podNamespace: namespace,
		netNSPath:    netNSPath,
		agentProxy:   agentProxy,
		podNetwork:   podNetworkConfig,
		cloudConfig:  cloudConfig,
		spec:         vmSpec,
	}

	if err := s.addSandbox(sid, sandbox); err != nil {
		return nil, fmt.Errorf("adding sandbox: %w", err)
	}

	if err := s.stateManager.Save(&state.SandboxState{
		Version:      1,
		SandboxID:    string(sid),
		PodName:      pod,
		PodNamespace: namespace,
		NetNSPath:    netNSPath,
		Running:      false,
		CreatedAt:    time.Now(),

		// agentProxy info
		ServerName:    putil.GenerateInstanceName(pod, string(sid), 63),
		ForwarderPort: s.serverConfig.ForwarderPort,

		// Network config for teardown
		PodNetwork: podNetworkConfig,
	}); err != nil {
		logger.Printf("state save for %s failed (non-fatal): %v", sid, err)
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

	// Idempotent: skip if restored (instance already exists)
	if sandbox.restored {
		logger.Printf("sandbox %s already running (restored), skipping StartVM", sid)
		return &pb.StartVMResponse{}, nil
	}

	instance, err := s.provider.CreateInstance(ctx, sandbox.podName, string(sid), sandbox.cloudConfig, sandbox.spec)

	// Cleanup instance if it was created but an error occurred (either during creation or later)
	defer func() {
		if err != nil && instance != nil && instance.ID != "" {
			logger.Printf("cleaning up instance %s (ID: %s) due to error: %v", instance.Name, instance.ID, err)
			cleanupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
			defer cancel()
			if delErr := s.provider.DeleteInstance(cleanupCtx, instance.ID); delErr != nil {
				logger.Printf("failed to cleanup instance %s: %v", instance.ID, delErr)
			} else if s.ppService != nil {
				if relErr := s.ppService.ReleasePeerPod(sandbox.podName, sandbox.podNamespace, instance.ID); relErr != nil {
					logger.Printf("failed to release PeerPod during cleanup: %v", relErr)
				}
			}
		}
	}()

	if err != nil {
		return nil, fmt.Errorf("creating an instance : %w", err)
	}

	if s.ppService != nil {
		if ownErr := s.ppService.OwnPeerPod(sandbox.podName, sandbox.podNamespace, instance.ID); ownErr != nil {
			logger.Printf("failed to create PeerPod: %v", ownErr)
		}
	}

	if err = s.setInstance(sid, instance.ID, instance.Name); err != nil {
		return nil, fmt.Errorf("setting instance: %w", err)
	}

	if err = s.stateManager.UpdateInstance(string(sid), instance.ID, instance.Name, instance.IPs); err != nil {
		logger.Printf("failed to persist instance info: %v", err)
	}

	logger.Printf("created an instance %s for sandbox %s", instance.Name, sid)

	if len(instance.IPs) == 0 {
		return nil, fmt.Errorf("instance IP is not available")
	}

	instanceIP := instance.IPs[0].String()
	forwarderPort := s.serverConfig.ForwarderPort

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
		if shutdownErr := sandbox.agentProxy.Shutdown(); shutdownErr != nil {
			logger.Printf("stopping agent proxy: %v", shutdownErr)
		}
		return nil, ctx.Err()
	case err = <-errCh:
		return nil, err
	case <-sandbox.agentProxy.Ready():
	}

	// After proxy ready — persist final state including DestinationIP from Setup()
	if err := s.stateManager.SetReady(string(sid), sandbox.podNetwork); err != nil {
		logger.Printf("failed to mark sandbox as ready: %v", err)
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

	if sandbox.agentProxy != nil {
		if err := sandbox.agentProxy.Shutdown(); err != nil {
			logger.Printf("stopping agent proxy: %v", err)
		}
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

	// After successful cleanup
	if err = s.stateManager.Delete(string(sid)); err != nil {
		logger.Printf("state delete for %s failed (non-fatal): %v", sid, err)
	}

	return &pb.StopVMResponse{}, nil
}
