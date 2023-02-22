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

	"github.com/containerd/containerd/pkg/cri/annotations"
	pb "github.com/kata-containers/kata-containers/src/runtime/protocols/hypervisor"

	"github.com/confidential-containers/cloud-api-adaptor/pkg/adaptor/proxy"
	"github.com/confidential-containers/cloud-api-adaptor/pkg/forwarder"
	"github.com/confidential-containers/cloud-api-adaptor/pkg/podnetwork"
	"github.com/confidential-containers/cloud-api-adaptor/pkg/podnetwork/tunneler"
	"github.com/confidential-containers/cloud-api-adaptor/pkg/util"
	"github.com/confidential-containers/cloud-api-adaptor/pkg/util/cloudinit"
)

const (
	Version = "0.0.0"
)

var logger = log.New(log.Writer(), "[adaptor/cloud] ", log.LstdFlags|log.Lmsgprefix)

type Provider interface {
	CreateInstance(ctx context.Context, podName, sandboxID string, cloudConfig cloudinit.CloudConfigGenerator) (instance *Instance, err error)
	DeleteInstance(ctx context.Context, instanceID string) error
	Teardown() error
}

type Instance struct {
	ID   string
	Name string
	IPs  []net.IP
}

type Service interface {
	pb.HypervisorService
	GetInstanceID(ctx context.Context, podNamespace, podName string, wait bool) (string, error)
	Teardown() error
}

type cloudService struct {
	provider     Provider
	proxyFactory proxy.Factory
	workerNode   podnetwork.WorkerNode
	sandboxes    map[sandboxID]*sandbox
	cond         *sync.Cond
	podsDir      string
	daemonPort   string
	mutex        sync.Mutex
}

type sandboxID string

type sandbox struct {
	agentProxy   proxy.AgentProxy
	podNetwork   *tunneler.Config
	cloudConfig  *cloudinit.CloudConfig
	id           sandboxID
	podName      string
	podNamespace string
	instanceName string
	instanceID   string
	netNSPath    string
}

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

func NewService(provider Provider, proxyFactory proxy.Factory, workerNode podnetwork.WorkerNode, podsDir, daemonPort string) Service {

	s := &cloudService{
		provider:     provider,
		proxyFactory: proxyFactory,
		sandboxes:    map[sandboxID]*sandbox{},
		podsDir:      podsDir,
		daemonPort:   daemonPort,
		workerNode:   workerNode,
	}
	s.cond = sync.NewCond(&s.mutex)

	return s
}

func (s *cloudService) Teardown() error {
	return s.provider.Teardown()
}

func (s *cloudService) setInstance(sid sandboxID, instanceID, instanceName string) error {

	s.mutex.Lock()
	defer s.mutex.Unlock()

	sandbox, ok := s.sandboxes[sid]
	if !ok {
		return fmt.Errorf("sandbox %s does not exist", sid)
	}

	sandbox.instanceID = instanceID
	sandbox.instanceName = instanceID

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

	netNSPath := req.NetworkNamespacePath

	podNetworkConfig, err := s.workerNode.Inspect(netNSPath)
	if err != nil {
		return nil, fmt.Errorf("failed to inspect netns %s: %w", netNSPath, err)
	}

	daemonConfig := forwarder.Config{
		PodNamespace: namespace,
		PodName:      pod,
		PodNetwork:   podNetworkConfig,
	}
	daemonJSON, err := json.MarshalIndent(daemonConfig, "", "    ")
	if err != nil {
		return nil, fmt.Errorf("generating JSON data: %w", err)
	}

	// Store daemon.json in worker node for debugging
	podDir := filepath.Join(s.podsDir, string(sid))
	if err := os.MkdirAll(podDir, os.ModePerm); err != nil {
		return nil, fmt.Errorf("creating a directory for daemon.json: %s, %w", podDir, err)
	}
	daemonJSONPath := filepath.Join(podDir, "daemon.json")
	if err := os.WriteFile(daemonJSONPath, daemonJSON, 0666); err != nil {
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

	socketPath := filepath.Join(podDir, proxy.SocketName)

	agentProxy := s.proxyFactory.New(socketPath)

	sandbox := &sandbox{
		id:           sid,
		podName:      pod,
		podNamespace: namespace,
		netNSPath:    netNSPath,
		agentProxy:   agentProxy,
		podNetwork:   podNetworkConfig,
		cloudConfig:  cloudConfig,
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

	instance, err := s.provider.CreateInstance(ctx, sandbox.podName, string(sid), sandbox.cloudConfig)
	if err != nil {
		return nil, fmt.Errorf("creating an instance : %w", err)
	}

	if err := s.setInstance(sid, instance.ID, instance.Name); err != nil {
		return nil, fmt.Errorf("setting instance: %w", err)
	}

	logger.Printf("created an instance %s for sandbox %s", instance.Name, sid)

	if err := s.workerNode.Setup(sandbox.netNSPath, instance.IPs, sandbox.podNetwork); err != nil {
		return nil, fmt.Errorf("setting up pod network tunnel on netns %s: %w", sandbox.netNSPath, err)
	}

	serverURL := &url.URL{
		Scheme: "http",
		Host:   net.JoinHostPort(instance.IPs[0].String(), s.daemonPort),
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

	if err := s.provider.DeleteInstance(ctx, sandbox.instanceID); err != nil {
		logger.Printf("deleting an instance %s: %v", sandbox.instanceID, err)
	}

	if err := s.workerNode.Teardown(sandbox.netNSPath, sandbox.podNetwork); err != nil {
		logger.Printf("tearing down netns %s: %v", sandbox.netNSPath, err)
	}

	if err = s.removeSandbox(sid); err != nil {
		logger.Printf("removing sandbox %s: %v", sid, err)
	}

	return &pb.StopVMResponse{}, nil
}
