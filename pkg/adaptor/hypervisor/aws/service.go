//go:build aws

package aws

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"

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
	Version = "0.0.0"
)

type hypervisorService struct {
	ec2Client        *ec2.Client
	serviceConfig    *Config
	hypervisorConfig *hypervisor.Config
	sandboxes        map[sandboxID]*sandbox
	podsDir          string
	daemonPort       string
	nodeName         string
	workerNode       podnetwork.WorkerNode
	sync.Mutex
}

func newService(ec2Client *ec2.Client, config *Config, hypervisorConfig *hypervisor.Config, workerNode podnetwork.WorkerNode, podsDir, daemonPort string) pb.HypervisorService {
	logger.Printf("service config %v", config.Redact())

	hostname, err := os.Hostname()
	if err != nil {
		panic(fmt.Errorf("failed to get hostname: %w", err))
	}

	i := strings.Index(hostname, ".")
	if i >= 0 {
		hostname = hostname[0:i]
	}

	return &hypervisorService{
		ec2Client:        ec2Client,
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

	var input *ec2.RunInstancesInput

	if s.serviceConfig.UseLaunchTemplate {
		input = &ec2.RunInstancesInput{
			MinCount: aws.Int32(1),
			MaxCount: aws.Int32(1),
			LaunchTemplate: &types.LaunchTemplateSpecification{
				LaunchTemplateName: aws.String(s.serviceConfig.LaunchTemplateName),
			},
			UserData: &userDataEnc,
		}
	} else {
		input = &ec2.RunInstancesInput{
			MinCount:         aws.Int32(1),
			MaxCount:         aws.Int32(1),
			ImageId:          aws.String(s.serviceConfig.ImageId),
			InstanceType:     types.InstanceType(s.serviceConfig.InstanceType),
			SecurityGroupIds: s.serviceConfig.SecurityGroupIds,
			KeyName:          aws.String(s.serviceConfig.KeyName),
			SubnetId:         aws.String(s.serviceConfig.SubnetId),
			UserData:         &userDataEnc,
		}
	}

	result, err := CreateInstance(ctx, s.ec2Client, input)
	if err != nil {
		return nil, fmt.Errorf("Creating instance (%v) returned error: %s", result, err)
	}

	sandbox.vsi = *result.Instances[0].InstanceId

	logger.Printf("created an instance %s for sandbox %s", *result.Instances[0].PublicDnsName, req.Id)

	// TODO: Specify the maximum instance name length in AWS
	vmName := hvutil.CreateInstanceName(sandbox.pod, string(sandbox.id), 0)
	tagInput := &ec2.CreateTagsInput{
		Resources: []string{*result.Instances[0].InstanceId},
		Tags: []types.Tag{
			{
				Key:   aws.String("Name"),
				Value: aws.String(vmName),
			},
		},
	}

	_, err = MakeTags(ctx, s.ec2Client, tagInput)
	if err != nil {
		logger.Printf("Adding tags to the instance failed with error: %s", err)
	}

	podNodeIPs, err := getIPs(result.Instances[0])
	if err != nil {
		logger.Printf("failed to get IPs for the instance : %v ", err)
		return nil, err
	}

	if err := s.workerNode.Setup(sandbox.netNSPath, podNodeIPs, sandbox.podNetworkConfig); err != nil {
		return nil, fmt.Errorf("failed to set up pod network tunnel on netns %s: %w", sandbox.netNSPath, err)
	}

	serverURL := &url.URL{
		Scheme: "http",
		Host:   net.JoinHostPort(podNodeIPs[0].String(), s.daemonPort),
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

var errNotReady = errors.New("address not ready")

func getIPs(instance types.Instance) ([]net.IP, error) {

	var podNodeIPs []net.IP
	for i, nic := range instance.NetworkInterfaces {
		addr := nic.PrivateIpAddress

		if addr == nil || *addr == "" || *addr == "0.0.0.0" {
			return nil, errNotReady
		}

		ip := net.ParseIP(*addr)
		if ip == nil {
			return nil, fmt.Errorf("failed to parse pod node IP %q", *addr)
		}
		podNodeIPs = append(podNodeIPs, ip)

		logger.Printf("podNodeIP[%d]=%s", i, ip.String())
	}

	return podNodeIPs, nil
}

func (s *hypervisorService) deleteInstance(ctx context.Context, id string) error {

	terminateInput := &ec2.TerminateInstancesInput{
		InstanceIds: []string{
			id,
		},
	}

	resp, err := DeleteInstance(ctx, s.ec2Client, terminateInput)

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

	if err := sandbox.agentProxy.Shutdown(); err != nil {
		logger.Printf("failed to stop agent proxy: %v", err)
	}

	if err := s.deleteInstance(ctx, sandbox.vsi); err != nil {
		return nil, err
	}

	if err := s.workerNode.Teardown(sandbox.netNSPath, sandbox.podNetworkConfig); err != nil {
		return nil, fmt.Errorf("failed to tear down netns %s: %w", sandbox.netNSPath, err)
	}
	err = s.deleteSandbox(req.Id)
	if err != nil {
		return nil, err
	}
	return &pb.StopVMResponse{}, nil
}
