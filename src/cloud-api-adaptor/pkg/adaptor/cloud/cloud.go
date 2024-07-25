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

	"github.com/containerd/containerd/pkg/cri/annotations"
	pb "github.com/kata-containers/kata-containers/src/runtime/protocols/hypervisor"

	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/pkg/aa"
	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/pkg/adaptor/k8sops"
	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/pkg/adaptor/proxy"
	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/pkg/agent"
	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/pkg/cdh"
	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/pkg/forwarder"
	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/pkg/podnetwork"
	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/pkg/util"
	provider "github.com/confidential-containers/cloud-api-adaptor/src/cloud-providers"
	putil "github.com/confidential-containers/cloud-api-adaptor/src/cloud-providers/util"
	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-providers/util/cloudinit"

	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/pkg/securecomms/wnssh"
)

const (
	SrcAuthfilePath = "/root/containers/auth.json"
	AgentConfigPath = "/run/peerpod/agent-config.toml"
	AuthFilePath    = "/run/peerpod/auth.json"
	Version         = "0.0.0"
)

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
	secureComms bool, secureCommsInbounds, secureCommsOutbounds, kbsAddress, podsDir, daemonPort, aaKBCParams, sshport string) Service {
	var err error
	var sshClient *wnssh.SshClient

	if secureComms {
		inbounds := append([]string{"KUBERNETES_PHASE:KATAAGENT:0"}, strings.Split(secureCommsInbounds, ",")...)
		outbounds := append([]string{"BOTH_PHASES:KBS:" + kbsAddress}, strings.Split(secureCommsOutbounds, ",")...)
		sshClient, err = wnssh.InitSshClient(inbounds, outbounds, kbsAddress, sshport)
		if err != nil {
			log.Fatalf("InitSshClient %v", err)
		}
	}

	s := &cloudService{
		provider:     provider,
		proxyFactory: proxyFactory,
		sandboxes:    map[sandboxID]*sandbox{},
		podsDir:      podsDir,
		daemonPort:   daemonPort,
		workerNode:   workerNode,
		aaKBCParams:  aaKBCParams,
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
	vcpus, memory := util.GetCPUAndMemoryFromAnnotation(req.Annotations)

	// Pod VM spec
	vmSpec := provider.InstanceTypeSpec{
		InstanceType: instanceType,
		VCPUs:        vcpus,
		Memory:       memory,
	}

	// TODO: server name is also generated in each cloud provider, and possibly inconsistent
	serverName := putil.GenerateInstanceName(pod, string(sid), 63)

	netNSPath := req.NetworkNamespacePath

	podNetworkConfig, err := s.workerNode.Inspect(netNSPath)
	if err != nil {
		return nil, fmt.Errorf("failed to inspect netns %s: %w", netNSPath, err)
	}

	podDir := filepath.Join(s.podsDir, string(sid))
	if err := os.MkdirAll(podDir, os.ModePerm); err != nil {
		return nil, fmt.Errorf("creating a pod directory: %s, %w", podDir, err)
	}
	socketPath := filepath.Join(podDir, proxy.SocketName)

	agentProxy := s.proxyFactory.New(serverName, socketPath)

	var authJSON []byte
	var authFilePath string
	_, err = os.Stat(SrcAuthfilePath)
	if err != nil {
		logger.Printf("credential file %s is not present, skipping image auth config", SrcAuthfilePath)
	} else {
		authJSON, err = os.ReadFile(SrcAuthfilePath)
		if err != nil {
			return nil, fmt.Errorf("error reading %s: %v", SrcAuthfilePath, err)
		}
		authFilePath = AuthFilePath
		logger.Printf("configure agent to use credentials file %s", SrcAuthfilePath)
	}

	agentConfig, err := agent.CreateConfigFile(authFilePath)
	if err != nil {
		return nil, fmt.Errorf("creating agent config: %w", err)
	}

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

	daemonJSON, err := json.MarshalIndent(daemonConfig, "", "    ")
	if err != nil {
		return nil, fmt.Errorf("generating JSON data: %w", err)
	}

	// Store daemon.json in worker node for debugging
	daemonJSONPath := filepath.Join(podDir, "daemon.json")
	if err := os.WriteFile(daemonJSONPath, daemonJSON, 0666); err != nil {
		return nil, fmt.Errorf("storing %s: %w", daemonJSONPath, err)
	}
	logger.Printf("stored %s", daemonJSONPath)

	cloudConfig := &cloudinit.CloudConfig{
		WriteFiles: []cloudinit.WriteFile{
			{
				Path:    AgentConfigPath,
				Content: agentConfig,
			},
			{
				Path:    forwarder.DefaultConfigPath,
				Content: string(daemonJSON),
			},
		},
	}

	if s.aaKBCParams != "" {
		logger.Printf("aaKBCParams: %s, support cc_kbc::*", s.aaKBCParams)
		toml, err := cdh.CreateConfigFile(s.aaKBCParams)
		if err != nil {
			return nil, fmt.Errorf("creating CDH config: %w", err)
		}
		cloudConfig.WriteFiles = append(cloudConfig.WriteFiles, cloudinit.WriteFile{
			Path:    cdh.ConfigFilePath,
			Content: toml,
		})

		toml, err = aa.CreateConfigFile(s.aaKBCParams)
		if err != nil {
			return nil, fmt.Errorf("creating attestation agent config: %w", err)
		}
		cloudConfig.WriteFiles = append(cloudConfig.WriteFiles, cloudinit.WriteFile{
			Path:    aa.DefaultAaConfigPath,
			Content: toml,
		})
	}

	if authJSON != nil {
		if len(authJSON) > cloudinit.DefaultAuthfileLimit {
			logger.Printf("Credentials file is too large to be included in cloud-config")
		} else {
			cloudConfig.WriteFiles = append(cloudConfig.WriteFiles, cloudinit.WriteFile{
				Path:    AuthFilePath,
				Content: string(authJSON),
			})
		}
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

	logger.Printf("create a sandbox %s for pod %s in namespace %s (netns: %s)", req.Id, pod, namespace, sandbox.netNSPath)

	return &pb.CreateVMResponse{AgentSocketPath: socketPath}, nil
}

func (s *cloudService) StartVM(ctx context.Context, req *pb.StartVMRequest) (res *pb.StartVMResponse, err error) {

	defer func() {
		if err != nil {
			logger.Print(err)
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
			logger.Printf("failed to create PeerPod: %s", err.Error())
		}
	}

	if err := s.setInstance(sid, instance.ID, instance.Name); err != nil {
		return nil, fmt.Errorf("setting instance: %w", err)
	}

	logger.Printf("created an instance %s for sandbox %s", instance.Name, sid)

	instanceIP := instance.IPs[0].String()
	forwarderPort := s.daemonPort

	if s.sshClient != nil {
		ci := s.sshClient.InitPP(context.Background(), string(sid), instance.IPs)
		if ci == nil {
			return nil, fmt.Errorf("failed sshClient.InitPP")
		}

		if err := ci.Start(); err != nil {
			return nil, fmt.Errorf("failed SshClientInstance.Start: %s", err)
		}

		// Set agentProxy
		instanceIP = "127.0.0.1"
		forwarderPort = ci.GetPort("KATAAGENT")

		// Set ci in sandbox
		sandbox.sshClientInst = ci
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
		_ = sandbox.agentProxy.Shutdown()
		return nil, ctx.Err()
	case err := <-errCh:
		return nil, err
	case <-sandbox.agentProxy.Ready():
	}

	logger.Printf("agent proxy is ready")
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
