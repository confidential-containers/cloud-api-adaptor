// Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package ibmcloud_powervs

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"log"
	"net"
	"net/netip"
	"strconv"
	"strings"
	"time"

	"github.com/IBM-Cloud/power-go-client/power/models"
	"github.com/IBM/go-sdk-core/v5/core"
	retry "github.com/avast/retry-go/v4"
	provider "github.com/confidential-containers/cloud-api-adaptor/src/cloud-providers"
	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-providers/util"
	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-providers/util/cloudinit"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

const (
	maxInstanceNameLen = 47
	sshPort            = "22"
	remoteFile         = "/media/cidata/user-data"
)

var logger = log.New(log.Writer(), "[adaptor/cloud/ibmcloud-powervs] ", log.LstdFlags|log.Lmsgprefix)

type ibmcloudPowerVSProvider struct {
	powervsService
	serviceConfig *Config
}

func NewProvider(config *Config) (provider.Provider, error) {

	logger.Printf("ibmcloud-powervs config: %#v", config.Redact())

	powervs, err := newPowervsClient(config.ApiKey, config.ServiceInstanceID, config.Zone)
	if err != nil {
		return nil, err
	}

	if config.EnableSftp {
		pubKeyString, privKeyString, err := generateSSHKeyPair()
		if err != nil {
			return nil, fmt.Errorf("failed to create SSH key pair: %w", err)
		}

		if pubKeyString == "" || privKeyString == "" {
			return nil, fmt.Errorf("generated SSH key pair is empty")
		}

		config.pubKey, config.privKey = pubKeyString, privKeyString
	}

	return &ibmcloudPowerVSProvider{
		powervsService: *powervs,
		serviceConfig:  config,
	}, nil
}

func (p *ibmcloudPowerVSProvider) CreateInstance(ctx context.Context, podName, sandboxID string, cloudConfig cloudinit.CloudConfigGenerator, spec provider.InstanceTypeSpec) (*provider.Instance, error) {
	instanceName := util.GenerateInstanceName(podName, sandboxID, maxInstanceNameLen)

	userData, err := cloudConfig.Generate()
	if err != nil {
		return nil, err
	}

	imageId := p.serviceConfig.ImageId

	if spec.Image != "" {
		logger.Printf("Choosing %s from annotation as the Power VS image for the PodVM image", spec.Image)
		imageId = spec.Image
	}

	memory := p.serviceConfig.Memory
	processors := p.serviceConfig.Processors
	systemType := p.serviceConfig.SystemType

	// If vCPU and memory are set in annotations then use it
	// If machine type is set in annotations then use it (ie. shape <system_type>-<cpu>x<memoery>)
	// vCPU and Memory gets higher priority than instance type from annotation
	if spec.VCPUs != 0 && spec.Memory != 0 {
		memory = float64(spec.Memory / 1024)
		processors = float64(spec.VCPUs)
		logger.Printf("Instance type selected by the cloud provider based on vCPU and memory annotations: %s-%gx%g", systemType, processors, memory)
	} else if spec.InstanceType != "" {
		typeAndSize := strings.Split(spec.InstanceType, "-")
		systemType = typeAndSize[0]
		size := strings.Split(typeAndSize[1], "x")
		f, err := strconv.Atoi(size[0])
		if err != nil {
			return nil, err
		}
		processors = float64(f)
		m, err := strconv.Atoi(size[1])
		if err != nil {
			return nil, err
		}
		memory = float64(m)
		logger.Printf("Instance type selected by the cloud provider based on instance type annotation: %s", spec.InstanceType)
	} else {
		logger.Printf("Instance type selected by the cloud provider based on config: %s-%gx%g", systemType, processors, memory)
	}

	body := &models.PVMInstanceCreate{
		ServerName:  &instanceName,
		ImageID:     &imageId,
		KeyPairName: p.serviceConfig.SSHKey,
		Networks: []*models.PVMInstanceAddNetwork{
			{
				NetworkID: &p.serviceConfig.NetworkID,
			}},
		Memory:     core.Float64Ptr(memory),
		Processors: core.Float64Ptr(processors),
		ProcType:   core.StringPtr(p.serviceConfig.ProcessorType),
		SysType:    systemType,
	}

	if p.serviceConfig.EnableSftp {
		sshKeyUserData := fmt.Sprintf(`#cloud-config
users:
  - name: %s
    ssh-authorized-keys:
      - %s
`, p.serviceConfig.CloudUserName, strings.TrimSpace(p.serviceConfig.pubKey))
		body.UserData = base64.StdEncoding.EncodeToString([]byte(sshKeyUserData))
	} else {
		body.UserData = base64.StdEncoding.EncodeToString([]byte(userData))
	}

	logger.Printf("CreateInstance: name: %q", instanceName)

	pvsInstances, err := p.powervsService.instanceClient(ctx).Create(body)
	if err != nil {
		logger.Printf("failed to create an instance : %v", err)
		return nil, err
	}

	if len(*pvsInstances) <= 0 {
		return nil, fmt.Errorf("there are no instances created")
	}

	ins := (*pvsInstances)[0]
	instanceID := *ins.PvmInstanceID

	getctx, cancel := context.WithTimeout(ctx, 150*time.Second)
	defer cancel()

	logger.Printf("Waiting for instance to reach state: ACTIVE")
	err = retry.Do(
		func() error {
			in, err := p.powervsService.instanceClient(getctx).Get(*ins.PvmInstanceID)
			if err != nil {
				return fmt.Errorf("failed to get the instance: %v", err)
			}

			if *in.Status == "ERROR" {
				return fmt.Errorf("instance is in error state")
			}

			if *in.Status == "ACTIVE" {
				logger.Printf("instance is in desired state: %s", *in.Status)
				return nil
			}

			return fmt.Errorf("Instance failed to reach ACTIVE state")
		},
		retry.Context(getctx),
		retry.Attempts(0),
		retry.MaxDelay(5*time.Second),
	)

	if err != nil {
		logger.Print(err)
		return nil, err
	}

	ips, err := p.getVMIPs(ctx, instanceID)
	if err != nil {
		return nil, fmt.Errorf("failed to get IPs for the instance : %v", err)
	}

	if p.serviceConfig.EnableSftp {
		err = p.sendConfigFile(ctx, userData, ips[0])
		if err != nil {
			return nil, fmt.Errorf("failed to send user data using ssh connection: %w", err)
		}
	}

	return &provider.Instance{
		ID:   instanceID,
		Name: instanceName,
		IPs:  ips,
	}, nil
}

func (p *ibmcloudPowerVSProvider) DeleteInstance(ctx context.Context, instanceID string) error {

	err := p.powervsService.instanceClient(ctx).Delete(instanceID)
	if err != nil {
		logger.Printf("failed to delete an instance: %v", err)
		return err
	}

	logger.Printf("deleted an instance %s", instanceID)
	return nil
}

func (p *ibmcloudPowerVSProvider) Teardown() error {
	return nil
}

func (p *ibmcloudPowerVSProvider) ConfigVerifier() error {
	imageId := p.serviceConfig.ImageId
	if len(imageId) == 0 {
		return fmt.Errorf("ImageId is empty")
	}
	return nil
}

func (p *ibmcloudPowerVSProvider) getVMIPs(ctx context.Context, instanceID string) ([]netip.Addr, error) {
	var ips []netip.Addr
	ins, err := p.powervsService.instanceClient(ctx).Get(instanceID)
	if err != nil {
		return nil, fmt.Errorf("failed to get the instance: %v", err)
	}

	for i, network := range ins.Networks {
		if ins.Networks[i].Type == "fixed" {
			ip_address := network.IPAddress
			if p.serviceConfig.UsePublicIP {
				ip_address = network.ExternalIP
			}

			ip, err := netip.ParseAddr(ip_address)
			if err != nil {
				return nil, fmt.Errorf("failed to parse pod node IP %q: %w", network.IPAddress, err)
			}

			ips = append(ips, ip)
			logger.Printf("podNodeIP[%d]=%s", i, ip.String())
		}
	}

	if len(ips) > 0 {
		return ips, nil
	}

	ctx, cancel := context.WithTimeout(ctx, 750*time.Second)
	defer cancel()

	// If IP is not assigned to the instance, fetch it from DHCP server
	logger.Printf("Trying to fetch IP from DHCP server..")
	err = retry.Do(func() error {
		ip, err := p.getIPFromDHCPServer(ctx, ins)
		if err != nil {
			logger.Print(err)
			return err
		}
		if ip == nil {
			return fmt.Errorf("failed to get IP from DHCP server: %v", err)
		}

		addr, err := netip.ParseAddr(*ip)
		if err != nil {
			return fmt.Errorf("failed to parse pod node IP %q: %w", *ip, err)
		}

		ips = append(ips, addr)
		logger.Printf("podNodeIP=%s", addr.String())
		return nil
	},
		retry.Context(ctx),
		retry.Attempts(0),
		retry.MaxDelay(10*time.Second),
	)

	if err != nil {
		logger.Print(err)
		return nil, err
	}

	return ips, nil
}

func (p *ibmcloudPowerVSProvider) getIPFromDHCPServer(ctx context.Context, instance *models.PVMInstance) (*string, error) {
	networkID := p.serviceConfig.NetworkID

	var pvsNetwork *models.PVMInstanceNetwork
	for _, net := range instance.Networks {
		if net.NetworkID == networkID {
			pvsNetwork = net
		}
	}
	if pvsNetwork == nil {
		return nil, fmt.Errorf("failed to get network attached to instance")
	}

	dhcpServers, err := p.powervsService.dhcpClient(ctx).GetAll()
	if err != nil {
		return nil, fmt.Errorf("failed to get the DHCP servers: %v", err)
	}

	var dhcpServerDetails *models.DHCPServerDetail
	for _, server := range dhcpServers {
		if *server.Network.ID == networkID {
			dhcpServerDetails, err = p.powervsService.dhcpClient(ctx).Get(*server.ID)
			if err != nil {
				return nil, fmt.Errorf("failed to get DHCP server details: %v", err)
			}
			break
		}
	}

	if dhcpServerDetails == nil {
		return nil, fmt.Errorf("DHCP server associated with network is nil")
	}

	var ip *string
	for _, lease := range dhcpServerDetails.Leases {
		if *lease.InstanceMacAddress == pvsNetwork.MacAddress {
			ip = lease.InstanceIP
			break
		}
	}

	return ip, nil
}

func (p *ibmcloudPowerVSProvider) sendConfigFile(ctx context.Context, data string, ip netip.Addr) error {
	server := ip.String() + ":" + sshPort

	signer, err := ssh.ParsePrivateKey([]byte(p.serviceConfig.privKey))
	if err != nil {
		return fmt.Errorf("failed to parse private key: %w", err)
	}

	// Dynamically fetch the server host key
	hostKey, err := p.getServerHostKey(ctx, server)
	if err != nil {
		return fmt.Errorf("failed to fetch the server host key : %w", err)
	}

	sshConfig := &ssh.ClientConfig{
		User: p.serviceConfig.CloudUserName,
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
func (p *ibmcloudPowerVSProvider) getServerHostKey(ctx context.Context, addr string) (ssh.PublicKey, error) {
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
		User: p.serviceConfig.CloudUserName,
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
