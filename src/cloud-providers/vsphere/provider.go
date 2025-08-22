// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package vsphere

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"net/netip"
	"path"
	"strings"
	"time"

	provider "github.com/confidential-containers/cloud-api-adaptor/src/cloud-providers"
	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-providers/util"
	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-providers/util/cloudinit"
	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/types"
)

var logger = log.New(log.Writer(), "[adaptor/cloud/vsphere] ", log.LstdFlags|log.Lmsgprefix)

const maxInstanceNameLen = 63

type vsphereProvider struct {
	gclient       *govmomi.Client
	serviceConfig *Config
}

func NewProvider(config *Config) (provider.Provider, error) {

	logger.Printf("vsphere config: %#v", config.Redact())

	err := checkConfig(config)
	if err != nil {
		return nil, err
	}

	govmomiClient, err := NewGovmomiClient(*config)
	if err != nil {
		return nil, fmt.Errorf("error creating vcenter session for cloud provider: %s", err)
	}

	provider := &vsphereProvider{
		gclient:       govmomiClient,
		serviceConfig: config,
	}

	return provider, nil
}

func checkConfig(config *Config) error {

	// Do some initial checks of the optional input values

	if config.DRS == "true" {
		if config.Cluster == "" {
			return fmt.Errorf("error: A cluster name is required with DRS")
		}
		return nil
	}

	// Hosts and Datastores are required when not using DRS
	// We cannot check for a cluster as hosts do not have to be part of a cluster.

	if config.Host == "" || config.Datastore == "" {
		return fmt.Errorf("error: A host and datastore name are required")
	}

	return nil
}

type VMConfig []types.BaseOptionValue

func (p *vsphereProvider) CreateInstance(ctx context.Context, podName, sandboxID string, cloudConfig cloudinit.CloudConfigGenerator, requirement provider.InstanceTypeSpec) (*provider.Instance, error) {

	vmname := util.GenerateInstanceName(podName, sandboxID, maxInstanceNameLen)

	logger.Printf("Start CreateInstance VM name %s", vmname)

	err := CheckSessionWithRestore(ctx, p.serviceConfig, p.gclient)
	if err != nil {
		logger.Printf("CreateInstance cannot find or create a new vcenter session")
		return nil, err
	}

	finder := find.NewFinder(p.gclient.Client)

	dc, err := finder.Datacenter(ctx, p.serviceConfig.Datacenter)
	if err != nil {
		logger.Printf("Cannot find vcenter datacenter %s error: %s", p.serviceConfig.Datacenter, err)
		return nil, err
	}

	finder.SetDatacenter(dc)

	vm, err := finder.VirtualMachine(ctx, p.serviceConfig.Template)
	if err != nil {
		logger.Printf("Cannot find VM template %s error: %s", p.serviceConfig.Template, err)
		return nil, err
	}

	template, err := vm.IsTemplate(ctx)
	if err != nil {
		logger.Printf("VM template %s error: %s", p.serviceConfig.Template, err)
		return nil, err
	}
	if !template {
		err = fmt.Errorf("template not valid")
		logger.Printf("VM template %s error: %s", p.serviceConfig.Template, err)
		return nil, err
	}

	// vm path for indicated destination datacenter
	// Logical ( not physical ) vm destination folder placement.
	// /p.serviceConfig.Datacenter/vm/p.serviceConfig.Deployfolder. If no folder exists it
	// will be created in the p.serviceConfig.Datacenter inventory path ie /p.serviceConfig.Datacenter/vm/

	inventoryPath := path.Join(dc.InventoryPath, "vm")

	vmfolder, err := finder.Folder(ctx, inventoryPath)
	if err != nil {
		logger.Printf("Cannot find inventory folder %s error: %s", inventoryPath, err)
		return nil, err
	}

	deployPathDirs := strings.Split(p.serviceConfig.Deployfolder, "/")
	deployPath := inventoryPath

	for _, dir := range deployPathDirs {
		deployPath = path.Join(deployPath, dir)
		currentFolder, err := finder.Folder(ctx, deployPath)
		if err != nil {
			if _, ok := err.(*find.NotFoundError); ok {
				currentFolder, err = vmfolder.CreateFolder(ctx, dir)
				if err != nil {
					return nil, err
				}
			} else {
				return nil, err
			}
		}
		vmfolder = currentFolder
	}

	vmfolderref := vmfolder.Reference()

	var relocateSpec types.VirtualMachineRelocateSpec

	relocateSpec.Folder = &vmfolderref

	cloneSpec := &types.VirtualMachineCloneSpec{
		PowerOn:  true,
		Template: false,
	}

	var poolref *types.ManagedObjectReference

	if strings.EqualFold(p.serviceConfig.DRS, "true") {

		// For this implementation we are supporting DRS for automation=manual configured
		// Vcenter clusters only. This means that the user must provide the DRS cluster name.
		// DRS will suggest the best host and datastore on the cluster which we will use in our
		// clone placement. The user does not need to indicate a host or datastore and those
		// inputs will be ignored if present.

		logger.Printf("Looking for cluster %s DRS recommendations", p.serviceConfig.Cluster)

		cluster, err := finder.ClusterComputeResourceOrDefault(ctx, p.serviceConfig.Cluster)
		if err != nil {
			logger.Printf("Cluster %s compute resource error: %s", p.serviceConfig.Cluster, err)
			return nil, err
		}

		vmref := vm.Reference()
		spec := types.PlacementSpec{
			PlacementType: string(types.PlacementSpecPlacementTypeClone),
			CloneName:     vmname,
			CloneSpec:     cloneSpec,
			RelocateSpec:  &relocateSpec,
			Vm:            &vmref,
		}

		result, err := cluster.PlaceVm(ctx, spec)
		if err != nil {
			logger.Printf("Cluster %s placement error: %s", p.serviceConfig.Cluster, err)
			return nil, err
		}

		recs := result.Recommendations
		if len(recs) == 0 {
			return nil, fmt.Errorf("vcenter has no cluster recommendations for cluster %s", p.serviceConfig.Cluster)
		}

		rspec := *recs[0].Action[0].(*types.PlacementAction).RelocateSpec
		relocateSpec.Datastore = rspec.Datastore
		relocateSpec.Host = rspec.Host
		relocateSpec.Pool = rspec.Pool

	} else {

		// Since we are not asking for DRS placement suggestions here the user must supply a host configured
		// with a datastore. checkConfig() would have failed if no host and datastore. If the host is part
		// of a cluster then the cluster name must also be supplied.
		// DRS configured clusters are treated the same as non DRS clusters when DRS services are not requested.

		var hostpath string

		// The vCenter inventory paths for hosts are as such:
		// A host not part of a cluster /myDatacenter/host/myhost@lab.eng.mycompany.com
		// A host that is part of a cluster /myDatacenter/host/my_vcenter-cluster/myhost@lab.eng.mycompany.com

		if p.serviceConfig.Cluster != "" {
			hostpath = fmt.Sprintf("/%s/host/%s/%s", p.serviceConfig.Datacenter, p.serviceConfig.Cluster, p.serviceConfig.Host)
		} else {
			hostpath = fmt.Sprintf("/%s/host/%s", p.serviceConfig.Datacenter, p.serviceConfig.Host)
		}

		host, err := finder.HostSystem(ctx, hostpath)
		if err != nil {
			return nil, err
		}
		hostref := types.NewReference(host.Reference())
		relocateSpec.Host = hostref

		pool, err := host.ResourcePool(ctx)
		if err != nil {
			logger.Printf("Host %s Resource Pool error: %s", p.serviceConfig.Host, err)
			return nil, err
		}
		poolref = types.NewReference(pool.Reference())
		relocateSpec.Pool = poolref

		// The host's datastore

		datastorepath := fmt.Sprintf("/%s/datastore/%s", p.serviceConfig.Datacenter, p.serviceConfig.Datastore)
		datastore, err := finder.Datastore(ctx, datastorepath)
		if err != nil {
			logger.Printf("Datastore %s error: %s", p.serviceConfig.Datastore, err)
			return nil, err
		}
		datastoreref := types.NewReference(datastore.Reference())
		relocateSpec.Datastore = datastoreref
	}

	userData, err := cloudConfig.Generate()
	if err != nil {
		logger.Printf("cloud config error: %s", err)
		return nil, err
	}

	//Convert userData to base64
	userDataEnc := base64.StdEncoding.EncodeToString([]byte(userData))

	var extraconfig VMConfig

	extraconfig = append(extraconfig,
		&types.OptionValue{
			Key:   "guestinfo.userdata",
			Value: userDataEnc,
		},
		&types.OptionValue{
			Key:   "guestinfo.userdata.encoding",
			Value: "base64",
		},
	)

	configSpec := types.VirtualMachineConfigSpec{
		ExtraConfig: extraconfig,
	}

	cloneSpec.Location = relocateSpec
	cloneSpec.Config = &configSpec

	task, err := vm.Clone(ctx, vmfolder, vmname, *cloneSpec)
	if err != nil {
		logger.Printf("Can't clone to vm %s: %s", vmname, err)
		return nil, err
	}

	info, err := task.WaitForResult(ctx, nil) // TODO Fix to have a timeout
	if err != nil {
		logger.Printf("wait for clone task failed")
		return nil, err
	}

	clone := object.NewVirtualMachine(p.gclient.Client, info.Result.(types.ManagedObjectReference))
	name, err := clone.ObjectName(ctx)
	if err != nil {
		return nil, err
	}

	logger.Printf("VM %s, UUID %s created", name, clone.UUID(ctx))

	ips, err := getIPs(clone) // TODO Fix to get all ips
	if err != nil {
		logger.Printf("Failed to get IPs for the instance : %v ", err)
		return nil, err
	}

	instance := &provider.Instance{
		ID:   clone.UUID(ctx),
		Name: vmname,
		IPs:  ips,
	}

	logger.Printf("CreateInstance VM name %s UUID %s done", vmname, clone.UUID(ctx))
	return instance, nil
}

func getIPs(vm *object.VirtualMachine) ([]netip.Addr, error) { // TODO Fix to get all ips
	var podNodeIPs []netip.Addr

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(600*time.Second))
	defer cancel()

	logger.Printf("Start waiting for cloned vm ip")
	ip, err := vm.WaitForIP(ctx, true)
	if err != nil {
		return nil, err
	}

	logger.Printf("VM IP = %s", ip)
	ipItem, err := netip.ParseAddr(ip)
	if err != nil {
		return nil, fmt.Errorf("failed to parse pod node IP %q: %w", ip, err)
	}
	podNodeIPs = append(podNodeIPs, ipItem)

	return podNodeIPs, nil
}

func (p *vsphereProvider) DeleteInstance(ctx context.Context, instanceID string) error {

	if instanceID == "" {
		return fmt.Errorf("DeleteInstance no VM UUID available")
	}

	instanceID = strings.ToLower(strings.TrimSpace(instanceID))

	logger.Printf("Deleting VM UUID %s", instanceID)

	var (
		task  *object.Task
		state types.VirtualMachinePowerState
	)

	err := CheckSessionWithRestore(ctx, p.serviceConfig, p.gclient)
	if err != nil {
		logger.Printf("Cannot find or create a new vcenter session")
		return err
	}

	finder := find.NewFinder(p.gclient.Client)

	dc, err := finder.Datacenter(ctx, p.serviceConfig.Datacenter)

	if err != nil {
		logger.Printf("Cannot get vcenter datacenter %s", p.serviceConfig.Datacenter)
		return err
	}

	s := object.NewSearchIndex(dc.Client())

	vmref, err := s.FindByUuid(ctx, dc, instanceID, true, nil)
	if err != nil {
		logger.Printf("Delete VM can't find VM UUID %s to delete it", instanceID)
		return err
	}

	vm := object.NewVirtualMachine(dc.Client(), vmref.Reference())

	state, err = vm.PowerState(ctx)
	if err != nil {
		return err
	}

	if state == types.VirtualMachinePowerStatePoweredOn {
		task, err = vm.PowerOff(ctx)
		if err != nil {
			return err
		}

		// Ignore error since the VM may already been in powered off state.
		// vm.Destroy will fail if the VM is still powered on.
		_ = task.Wait(ctx)
	}

	task, err = vm.Destroy(ctx)
	if err != nil {
		return err
	}

	_ = task.Wait(ctx)

	logger.Printf("DeleteInstance VM UUID %s done", instanceID)

	return nil
}

func (p *vsphereProvider) Teardown() error {
	logger.Printf("Logout user %s", p.serviceConfig.UserName)
	return DeleteGovmomiClient(p.gclient)
}

func (p *vsphereProvider) ConfigVerifier() error {
	template := p.serviceConfig.Template
	if len(template) == 0 {
		return fmt.Errorf("template is empty")
	}
	return nil
}
