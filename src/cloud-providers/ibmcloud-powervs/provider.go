// Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package ibmcloud_powervs

import (
	"context"
	"encoding/base64"
	"errors"
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
)

const maxInstanceNameLen = 47

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

	pvsProvider := &ibmcloudPowerVSProvider{
		powervsService: *powervs,
		serviceConfig:  config,
	}
	pvsProvider.serviceConfig.PreCreatedInstances = new([]provider.Instance)

	if config.PoolSize > 0 {
		logger.Printf("Creating a new pod VM pool of size %d", config.PoolSize)
		if err := pvsProvider.createOrUpdatePodVMPool(context.Background(), config.PoolSize); err != nil {
			return nil, err
		}

		// Start a goroutine to periodically check the pool size and restock it if needed
		go pvsProvider.checkPodVMPoolSize(context.Background(), config.PoolSize)
	}

	return pvsProvider, nil
}

func (p *ibmcloudPowerVSProvider) CreateInstance(ctx context.Context, podName, sandboxID string, cloudConfig cloudinit.CloudConfigGenerator, spec provider.InstanceTypeSpec) (*provider.Instance, error) {

	instanceName := util.GenerateInstanceName(podName, sandboxID, maxInstanceNameLen)

	userData, err := cloudConfig.Generate()
	if err != nil {
		return nil, err
	}

	if len(*p.serviceConfig.PreCreatedInstances) > 0 {
		var conn net.Conn
		instance := (*p.serviceConfig.PreCreatedInstances)[0]
		*p.serviceConfig.PreCreatedInstances = (*p.serviceConfig.PreCreatedInstances)[1:]
		logger.Printf("Using instance from pre-created pod VM pool, name: %s ,id:%s", instance.Name, instance.ID)

		address := fmt.Sprintf("%s:%s", instance.IPs[0].String(), p.serviceConfig.PudPort)
		logger.Printf("Connecting to pre-created VM: %s", address)
		ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
		defer cancel()

		err := retry.Do(
			func() error {
				var err error
				conn, err = (&net.Dialer{}).DialContext(ctx, "tcp", address)
				return err
			},
			retry.Attempts(0),
			retry.Context(ctx),
			retry.MaxDelay(5*time.Second),
		)
		if err != nil {
			err = fmt.Errorf("failed to establish connection to process-user-data: %s: %w", address, err)
			logger.Print(err)
			return nil, err
		}
		defer conn.Close()

		logger.Println("Connection established, prepare to send data..")
		_, err = conn.Write([]byte(userData))
		if err != nil {
			return nil, err
		}

		logger.Printf("Successfully sent user-data to the pre-created VM %s, IP:%s", instance.Name, instance.IPs[0])
		return &instance, nil
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
		logger.Printf("Instance type selected by the cloud provider based on vCPU and memory annotations: %s-%fx%f", systemType, processors, memory)
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
		logger.Printf("Instance type selected by the cloud provider based on config: %s-%fx%f", systemType, processors, memory)
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
		UserData:   base64.StdEncoding.EncodeToString([]byte(userData)),
	}

	// Wait for VM to be active and fetch the IPs
	instance, err := p.createVM(ctx, instanceName, body)
	if err != nil {
		logger.Printf("failed to create the VM : %v", err)
		return nil, err
	}
	return instance, nil
}

func (p *ibmcloudPowerVSProvider) createVM(ctx context.Context, instanceName string, body *models.PVMInstanceCreate) (*provider.Instance, error) {
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
			in, err := p.powervsService.instanceClient(getctx).Get(instanceID)
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

	logger.Printf("deleted instance %s", instanceID)
	return nil
}

func (p *ibmcloudPowerVSProvider) Teardown() error {
	if p.serviceConfig.PoolSize > 0 {
		err := p.destroyPodVMPool(context.Background())
		if err != nil {
			return fmt.Errorf("failed to destroy podVM pool: %w", err)
		}
	}
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

// createOrUpdatePodVMPool created a new pool or updates the existing pool of pre-created VMs.
func (p *ibmcloudPowerVSProvider) createOrUpdatePodVMPool(ctx context.Context, poolSize int) error {
	for i := 0; i < poolSize; i++ {
		instanceName := util.GenerateInstanceName("pool", util.RandomString(5), maxInstanceNameLen)
		logger.Printf("Creating a VM with name %s", instanceName)
		body := &models.PVMInstanceCreate{
			ServerName:  &instanceName,
			ImageID:     &p.serviceConfig.ImageId,
			KeyPairName: p.serviceConfig.SSHKey,
			Networks: []*models.PVMInstanceAddNetwork{
				{
					NetworkID: &p.serviceConfig.NetworkID,
				}},
			Memory:     core.Float64Ptr(p.serviceConfig.Memory),
			Processors: core.Float64Ptr(p.serviceConfig.Processors),
			ProcType:   core.StringPtr(p.serviceConfig.ProcessorType),
			SysType:    p.serviceConfig.SystemType,
		}

		instance, err := p.createVM(ctx, instanceName, body)
		if err != nil {
			logger.Printf("failed to create the VM : %v", err)
			return err
		}
		*p.serviceConfig.PreCreatedInstances = append(*p.serviceConfig.PreCreatedInstances, *instance)
		logger.Printf("VM created successfully and added to the pool, Name: %s, ID: %s", instanceName, instance.ID)
	}

	logger.Printf("PreCreatedInstances: (%v)\nPodvm pool size: %d", p.serviceConfig.PreCreatedInstances, len(*p.serviceConfig.PreCreatedInstances))
	return nil
}

// checkPodVMPoolSize monitors and updates the podVM pool with the desired number of instances.
func (p *ibmcloudPowerVSProvider) checkPodVMPoolSize(ctx context.Context, poolSize int) {
	checkInterval := 15 * time.Minute

	for {
		logger.Println("Sleep for 15 mins before checking for pool restock...")
		time.Sleep(checkInterval)

		// Check if the VMs in the pool exist and are in active state in the cloud
		logger.Print("Checking the pod VMs state in the pool")
		for i, instance := range *p.serviceConfig.PreCreatedInstances {
			instancesList := *p.serviceConfig.PreCreatedInstances
			ins, err := p.powervsService.instanceClient(ctx).Get(instance.ID)
			if err != nil {
				if strings.Contains(err.Error(), "pvm-instance does not exist") {
					logger.Printf("pod VM %s does not exist in the pool", instance.Name)
					*p.serviceConfig.PreCreatedInstances = append(instancesList[:i], instancesList[i+1:]...)
					continue
				}
				logger.Printf("failed to get the instance %s present in the pool: %v", instance.Name, err)
				continue
			}

			if *ins.Status != "ACTIVE" || *ins.Status == "ERROR" {
				if err := p.DeleteInstance(ctx, instance.ID); err != nil {
					logger.Printf("failed to delete pre-created instance: %v", err)
					continue
				}
				logger.Printf("deleted pod VM in error state from the pool %s", instance.Name)
				*p.serviceConfig.PreCreatedInstances = append(instancesList[:i], instancesList[i+1:]...)
			}
			logger.Printf("Pod VM %s is in active state", instance.Name)
		}

		count := len(*p.serviceConfig.PreCreatedInstances)
		logger.Printf("PreCreatedInstances: (%v)\n Current pod VM pool size: %d", p.serviceConfig.PreCreatedInstances, count)

		if count < poolSize {
			logger.Printf("Pool size(%d) is less than desired size(%d), restocking the pool.", count, poolSize)
			podVMPoolSize := poolSize - count
			logger.Printf("Updating pool with adding %d pod VMs", podVMPoolSize)
			if err := p.createOrUpdatePodVMPool(ctx, podVMPoolSize); err != nil {
				logger.Printf("failed to update the podVM pool: %v", err)
				continue
			}
		}
	}
}

// destroyPodVMPool deletes the pre-created VMs.
func (p *ibmcloudPowerVSProvider) destroyPodVMPool(ctx context.Context) error {
	var errs []error
	logger.Println("Destroying the pod VM pool")
	for _, instance := range *p.serviceConfig.PreCreatedInstances {
		logger.Printf("Deleting instance, name: %s, id: %s", instance.Name, instance.ID)
		if err := p.DeleteInstance(ctx, instance.ID); err != nil {
			logger.Printf("failed to delete pre-created instance: %v", err)
			errs = append(errs, err)
			continue
		}
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}

	logger.Println(" Successfully destroyed pod VM pool")
	return nil
}
