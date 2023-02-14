// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package vsphere

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"net"
	"path"
	"strings"
	"time"

	"github.com/confidential-containers/cloud-api-adaptor/pkg/adaptor/cloud"
	"github.com/confidential-containers/cloud-api-adaptor/pkg/util"
	"github.com/confidential-containers/cloud-api-adaptor/pkg/util/cloudinit"
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

func NewProvider(config *Config) (cloud.Provider, error) {

	logger.Printf("vsphere config: %#v", config.Redact())

	govmomiClient, err := NewGovmomiClient(*config)
	if err != nil {
		logger.Printf("Error creating vcenter session for cloud provider:  %s", err)
		return nil, err
	}

	provider := &vsphereProvider{
		gclient:       govmomiClient,
		serviceConfig: config,
	}

	return provider, nil
}

type VmConfig []types.BaseOptionValue

func (p *vsphereProvider) CreateInstance(ctx context.Context, podName, sandboxID string, cloudConfig cloudinit.CloudConfigGenerator) (*cloud.Instance, error) {

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

	// TODO change to tpl, err := template.FindTemplate(ctx, ctx.VSphereVM.Spec.Template)
	// "github.com/vmware/govmomi/template"

	vm, err := finder.VirtualMachine(ctx, p.serviceConfig.Template)
	if err != nil {
		logger.Printf("Cannot find VM template %s error: %s", p.serviceConfig.Template, err)
		return nil, err
	}

	// TODO for DRS ResourcePoolOrDefault()
	pool, err := finder.DefaultResourcePool(ctx)
	if err != nil {
		logger.Printf("Cannot find default resource pool error: %s", err)
		return nil, err
	}
	poolref := types.NewReference(pool.Reference())

	// Logical ( not physical ) vm folder placement. If no folder exists it
	// will be created in the current inventory path ie /your-current-datacenter/vm/newfolder/

	inventory_path := path.Join(dc.InventoryPath, "vm")

	vmfolder, err := finder.Folder(ctx, inventory_path)
	if err != nil {
		logger.Printf("Cannot find inventory folder %s error: %s", inventory_path, err)
		return nil, err
	}

	deploy_path_dirs := strings.Split(p.serviceConfig.Deployfolder, "/")
	deploy_path := inventory_path

	for _, dir := range deploy_path_dirs {
		deploy_path = path.Join(deploy_path, dir)
		current_folder, err := finder.Folder(ctx, deploy_path)
		if err != nil {
			if _, ok := err.(*find.NotFoundError); ok {
				current_folder, err = vmfolder.CreateFolder(ctx, dir)
				if err != nil {
					return nil, err
				}
			} else {
				return nil, err
			}
		}
		vmfolder = current_folder
	}

	vmfolderref := vmfolder.Reference()

	relocateSpec := types.VirtualMachineRelocateSpec{
		Folder: &vmfolderref,
		Pool:   poolref,
	}

	userData, err := cloudConfig.Generate()
	if err != nil {
		logger.Printf("cloud config error: %s", err)
		return nil, err
	}

	//Convert userData to base64
	userDataEnc := base64.StdEncoding.EncodeToString([]byte(userData))

	var extraconfig VmConfig

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

	if p.serviceConfig.Datastore != "" {
		datastorepath := fmt.Sprintf("/%s/datastore/%s", p.serviceConfig.Datacenter, p.serviceConfig.Datastore)
		datastore, err := finder.Datastore(ctx, datastorepath)
		if err != nil {
			return nil, err
		}
		datastoreref := types.NewReference(datastore.Reference())
		relocateSpec.Datastore = datastoreref
	}

	cloneSpec := &types.VirtualMachineCloneSpec{
		Location: relocateSpec,
		PowerOn:  true,
		Template: false,
		Config:   &configSpec,
	}

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

	//uuid := strings.Replace(mvm.Config.Uuid, "-", "", -1)

	ips, err := getIPs(clone) // TODO Fix to get all ips
	if err != nil {
		logger.Printf("Failed to get IPs for the instance : %v ", err)
		return nil, err
	}

	instance := &cloud.Instance{
		ID:   clone.UUID(ctx),
		Name: vmname,
		IPs:  ips,
	}

	logger.Printf("CreateInstance VM name %s UUID %s done", vmname, clone.UUID(ctx))
	return instance, nil
}

func getIPs(vm *object.VirtualMachine) ([]net.IP, error) { // TODO Fix to get all ips
	var podNodeIPs []net.IP

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(600*time.Second))
	defer cancel()

	logger.Printf("Start waiting for cloned vm ip")
	ip, err := vm.WaitForIP(ctx, true)
	if err != nil {
		return nil, err
	}

	logger.Printf("VM IP = %s", ip)
	ip_item := net.ParseIP(ip)
	if ip_item == nil {
		return nil, fmt.Errorf("failed to parse pod node IP %q", ip)
	}
	podNodeIPs = append(podNodeIPs, ip_item)

	return podNodeIPs, nil
}

func (p *vsphereProvider) DeleteInstance(ctx context.Context, instanceID string) error {

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
