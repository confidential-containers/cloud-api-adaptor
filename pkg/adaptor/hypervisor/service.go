// (C) Copyright IBM Corp. 2022.
// SPDX-License-Identifier: Apache-2.0

package hypervisor

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
	"time"

	"github.com/IBM/vpc-go-sdk/vpcv1"

	"github.com/confidential-containers/peer-pod-opensource/pkg/adaptor/forwarder"
	daemon "github.com/confidential-containers/peer-pod-opensource/pkg/forwarder"
	"github.com/confidential-containers/peer-pod-opensource/pkg/podnetwork"
	"github.com/confidential-containers/peer-pod-opensource/pkg/podnetwork/tunneler"
	"github.com/confidential-containers/peer-pod-opensource/pkg/util/cloudinit"
	"github.com/containerd/containerd/pkg/cri/annotations"

	pb "github.com/kata-containers/kata-containers/src/runtime/protocols/hypervisor"
)

// TODO: implement a ttrpc server to serve hypervisor RPC calls from kata shim
// https://github.com/kata-containers/kata-containers/blob/2.2.0-alpha1/src/runtime/virtcontainers/hypervisor.go#L843-L883

const (
	Version       = "0.0.0"
	maxRetries    = 10
	queryInterval = 2
	subnetBits    = "/24"
)

type ServiceConfig struct {
	ProfileName              string
	ZoneName                 string
	ImageID                  string
	PrimarySubnetID          string
	PrimarySecurityGroupID   string
	SecondarySubnetID        string
	SecondarySecurityGroupID string
	KeyID                    string
	VpcID                    string
}

type hypervisorService struct {
	*ServiceConfig
	vpcV1         VpcV1
	cloudProvider *CloudProvider
	sandboxes     map[sandboxID]*sandbox
	podsDir       string
	daemonPort    string
	nodeName      string
	workerNode    podnetwork.WorkerNode
	sync.Mutex
}

func newService(vpcV1 VpcV1, config *ServiceConfig, workerNode podnetwork.WorkerNode, podsDir, daemonPort string) pb.HypervisorService {

	hostname, err := os.Hostname()
	if err != nil {
		panic(fmt.Errorf("failed to get hostname: %w", err))
	}

	i := strings.Index(hostname, ".")
	if i >= 0 {
		hostname = hostname[0:i]
	}

	return &hypervisorService{
		vpcV1:         vpcV1,
		ServiceConfig: config,
		sandboxes:     map[sandboxID]*sandbox{},
		podsDir:       podsDir,
		daemonPort:    daemonPort,
		nodeName:      hostname,
		workerNode:    workerNode,
	}
}

type sandboxID string

type sandbox struct {
	id               sandboxID
	pod              string
	namespace        string
	netNSPath        string
	podDirPath       string
	agentSocketPath  string
	vsi              string
	socketForwarder  forwarder.SocketForwarder
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
	pod := req.Annotations[annotations.SandboxName]
	if pod == "" {
		return nil, fmt.Errorf("pod name %s is missing in annotations", annotations.SandboxName)
	}
	namespace := req.Annotations[annotations.SandboxNamespace]
	if pod == "" {
		return nil, fmt.Errorf("namespace name %s is missing in annotations", annotations.SandboxNamespace)
	}

	podDirPath := filepath.Join(s.podsDir, string(sid))
	if err := os.MkdirAll(podDirPath, os.ModePerm); err != nil {
		return nil, fmt.Errorf("failed to create a pod directory: %s: %w", podDirPath, err)
	}

	socketPath := filepath.Join(podDirPath, forwarder.SocketName)

	netNSPath := req.NetworkNamespacePath

	podNetworkConfig, err := s.workerNode.Inspect(netNSPath)
	if err != nil {
		return nil, fmt.Errorf("failed to inspect netns %s: %w", netNSPath, err)
	}

	sandbox := &sandbox{
		id:               sid,
		pod:              pod,
		namespace:        namespace,
		netNSPath:        netNSPath,
		podDirPath:       podDirPath,
		agentSocketPath:  socketPath,
		podNetworkConfig: podNetworkConfig,
	}
	s.sandboxes[sid] = sandbox
	logger.Printf("create a sandbox %s for pod %s in namespace %s (netns: %s)", req.Id, pod, namespace, sandbox.netNSPath)
	return &pb.CreateVMResponse{AgentSocketPath: sandbox.agentSocketPath}, nil
}

func (s *hypervisorService) StartVM(ctx context.Context, req *pb.StartVMRequest) (*pb.StartVMResponse, error) {

	sandbox, err := s.getSandbox(req.Id)
	if err != nil {
		return nil, err
	}

	vmName := fmt.Sprintf("%s-%s-%s-%.8s", s.nodeName, sandbox.namespace, sandbox.pod, sandbox.id)

	daemonConfig := daemon.Config{
		PodNamespace: sandbox.namespace,
		PodName:      sandbox.pod,
		PodNetwork:   sandbox.podNetworkConfig,
	}
	daemonJSON, err := json.MarshalIndent(daemonConfig, "", "    ")
	if err != nil {
		return nil, err
	}

	// Store daemon.json in worker node for debuggig
	if err := os.WriteFile(filepath.Join(sandbox.podDirPath, "daemon.json"), daemonJSON, 0666); err != nil {
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

	userData, err := cloudConfig.Generate()
	if err != nil {
		return nil, err
	}

	prototype := &vpcv1.InstancePrototype{
		Name:     &vmName,
		Image:    &vpcv1.ImageIdentity{ID: &s.ImageID},
		UserData: &userData,
		Profile:  &vpcv1.InstanceProfileIdentity{Name: &s.ProfileName},
		Zone:     &vpcv1.ZoneIdentity{Name: &s.ZoneName},
		Keys: []vpcv1.KeyIdentityIntf{
			&vpcv1.KeyIdentity{ID: &s.KeyID},
		},
		VPC: &vpcv1.VPCIdentity{ID: &s.VpcID},
		PrimaryNetworkInterface: &vpcv1.NetworkInterfacePrototype{
			AllowIPSpoofing: func(b bool) *bool { return &b }(true),
			Subnet:          &vpcv1.SubnetIdentity{ID: &s.PrimarySubnetID},
			SecurityGroups: []vpcv1.SecurityGroupIdentityIntf{
				&vpcv1.SecurityGroupIdentityByID{ID: &s.PrimarySecurityGroupID},
			},
		},
		NetworkInterfaces: []vpcv1.NetworkInterfacePrototype{
			{
				AllowIPSpoofing: func(b bool) *bool { return &b }(true),
				Subnet:          &vpcv1.SubnetIdentity{ID: &s.SecondarySubnetID},
				SecurityGroups: []vpcv1.SecurityGroupIdentityIntf{
					&vpcv1.SecurityGroupIdentityByID{ID: &s.SecondarySecurityGroupID},
				},
			},
		},
	}

	result, resp, err := s.vpcV1.CreateInstance(&vpcv1.CreateInstanceOptions{InstancePrototype: prototype})
	if err != nil {
		logger.Printf("failed to create an instance : %v and the response is %s", err, resp)
		return nil, err
	}

	sandbox.vsi = *result.ID

	logger.Printf("created an instance %s for sandbox %s", *result.Name, req.Id)

	var podNodeIPs []net.IP

	for retries := 0; retries < maxRetries; retries++ {

		ips, err := getIPs(prototype, result)

		if err == nil {
			podNodeIPs = ips
			break
		} else if err != errNotReady {
			return nil, err
		}

		time.Sleep(time.Duration(queryInterval) * time.Second)

		var id string = *result.ID
		getResult, resp, err := s.vpcV1.GetInstance(&vpcv1.GetInstanceOptions{ID: &id})
		if err != nil {
			logger.Printf("failed to get an instance : %v and the response is %s", err, resp)
			return nil, err
		}
		result = getResult
	}

	if err := s.workerNode.Setup(sandbox.netNSPath, podNodeIPs, sandbox.podNetworkConfig); err != nil {
		return nil, fmt.Errorf("failed to set up pod network tunnel on netns %s: %w", sandbox.netNSPath, err)
	}

	serverURL := &url.URL{
		Scheme: "http",
		Host:   net.JoinHostPort(podNodeIPs[0].String(), s.daemonPort),
		Path:   daemon.AgentURLPath,
	}

	socketForwarder := forwarder.NewSocketForwarder(sandbox.agentSocketPath, serverURL)

	errCh := make(chan error)
	go func() {
		defer close(errCh)

		if err := socketForwarder.Start(context.Background()); err != nil {
			logger.Printf("error running socket forwarder: %v", err)
			errCh <- err
		}
	}()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case err := <-errCh:
		return nil, err
	case <-socketForwarder.Ready():
	}

	sandbox.socketForwarder = socketForwarder
	logger.Printf("socket forwarder is ready")
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

var errNotReady = errors.New("address not ready")

func getIPs(prototype *vpcv1.InstancePrototype, result *vpcv1.Instance) ([]net.IP, error) {

	if len(result.NetworkInterfaces) < 1+len(prototype.NetworkInterfaces) {
		return nil, errNotReady
	}

	interfaces := []*vpcv1.NetworkInterfaceInstanceContextReference{result.PrimaryNetworkInterface}
	for i, nic := range result.NetworkInterfaces {
		if *nic.ID != *result.PrimaryNetworkInterface.ID {
			interfaces = append(interfaces, &result.NetworkInterfaces[i])
		}
	}

	var podNodeIPs []net.IP

	for i, nic := range interfaces {

		addr := nic.PrimaryIpv4Address
		if addr == nil || *addr == "" || *addr == "0.0.0.0" {
			return nil, errNotReady
		}

		ip := net.ParseIP(*addr)
		if addr == nil {
			return nil, fmt.Errorf("failed to parse pod node IP %q", *addr)
		}
		podNodeIPs = append(podNodeIPs, ip)

		logger.Printf("podNodeIP[%d]=%s", i, ip.String())
	}

	return podNodeIPs, nil
}

func (s *hypervisorService) deleteInstance(id string) error {
	resp, err := s.vpcV1.DeleteInstance(&vpcv1.DeleteInstanceOptions{
		ID: func(s string) *string { return &s }(id),
	})
	if err != nil {
		logger.Printf("failed to delete an instance: %v and the response is %v", err, resp)
		return err
	}
	logger.Printf("deleted an instance %s", id)
	return nil
}

func (s *hypervisorService) StopVM(ctx context.Context, req *pb.StopVMRequest) (*pb.StopVMResponse, error) {
	sandbox, err := s.getSandbox(req.Id)
	if err != nil {
		return nil, err
	}
	if err := s.deleteInstance(sandbox.vsi); err != nil {
		return nil, err
	}

	if err := s.workerNode.Teardown(sandbox.netNSPath, sandbox.podNetworkConfig); err != nil {
		return nil, fmt.Errorf("failed to tear down netns %s: %w", sandbox.netNSPath, err)
	}

	return &pb.StopVMResponse{}, nil
}
