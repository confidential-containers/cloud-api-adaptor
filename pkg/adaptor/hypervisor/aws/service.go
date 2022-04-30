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

	"github.com/confidential-containers/cloud-api-adaptor/pkg/adaptor/forwarder"
	daemon "github.com/confidential-containers/cloud-api-adaptor/pkg/forwarder"
	"github.com/confidential-containers/cloud-api-adaptor/pkg/podnetwork"
	"github.com/confidential-containers/cloud-api-adaptor/pkg/podnetwork/tunneler"
	"github.com/confidential-containers/cloud-api-adaptor/pkg/util/cloudinit"
	"github.com/containerd/containerd/pkg/cri/annotations"

	pb "github.com/kata-containers/kata-containers/src/runtime/protocols/hypervisor"
)


const (
	Version       = "0.0.0"
	EC2LaunchTemplateName = "kata"
)

type hypervisorService struct {
	ec2Client     *ec2.Client
	serviceConfig *Config
	sandboxes     map[sandboxID]*sandbox
	podsDir       string
	daemonPort    string
	nodeName      string
	workerNode    podnetwork.WorkerNode
	sync.Mutex
}

func newService(ec2Client *ec2.Client, config *Config, workerNode podnetwork.WorkerNode, podsDir, daemonPort string) pb.HypervisorService {
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
		ec2Client:     ec2Client,
		serviceConfig: config,
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

	//Convert userData to base64
	userDataEnc := base64.StdEncoding.EncodeToString([]byte(userData))

	input := &ec2.RunInstancesInput{
		MinCount: aws.Int32(1),
		MaxCount: aws.Int32(1),
		LaunchTemplate: &types.LaunchTemplateSpecification{
			LaunchTemplateName: aws.String(EC2LaunchTemplateName),
		},
		UserData: &userDataEnc,
	}

	result, err := CreateInstance(context.TODO(), s.ec2Client, input)
	if err != nil {
		logger.Printf("failed to create an instance : %v and the response is %s", err, result)
		return nil, err
	}

	sandbox.vsi = *result.Instances[0].InstanceId

	logger.Printf("created an instance %s for sandbox %s", *result.Instances[0].PublicDnsName, req.Id)

	vmName := fmt.Sprintf("%s-%s-%s-%.8s", s.nodeName, sandbox.namespace, sandbox.pod, sandbox.id)
        tagInput := &ec2.CreateTagsInput{
		Resources: []string{*result.Instances[0].InstanceId},
		Tags: []types.Tag{
			{
				Key:   aws.String("name"),
				Value: aws.String(vmName),
			},
		},
	}

        _, err = MakeTags(context.TODO(), s.ec2Client, tagInput)
        if err != nil {
                logger.Printf("failed to tag the instance", err)
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

func getIPs(instance types.Instance) ([]net.IP, error) {

	var podNodeIPs []net.IP
	for i, nic := range instance.NetworkInterfaces {
		addr := nic.PrivateIpAddress

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

	terminateInput := &ec2.TerminateInstancesInput{
		InstanceIds: []string{
			id,
		},
	}

	resp, err := DeleteInstance(context.TODO(), s.ec2Client, terminateInput)

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
