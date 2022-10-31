package vsphere

import (
	"context"
	"encoding/base64"
	"fmt"
	"net"
	"net/url"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/session"
	"github.com/vmware/govmomi/session/keepalive"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/methods"
	"github.com/vmware/govmomi/vim25/soap"
	"github.com/vmware/govmomi/vim25/types"
)

var glock sync.Mutex

func NewGovmomiClient(vmcfg Config) (*govmomi.Client, error) {

	ctx := context.TODO()

	urlinfo, err := soap.ParseURL(vmcfg.VcenterURL)
	if err != nil {
		return nil, err
	}

	// TODO make credentials secure
	urlinfo.User = url.UserPassword(vmcfg.UserName, vmcfg.Password)

	soapClient := soap.NewClient(urlinfo, vmcfg.Insecure)

	vim25Client, err := vim25.NewClient(ctx, soapClient)
	if err != nil {
		return nil, err
	}

	vim25Client.RoundTripper = keepalive.NewHandlerSOAP(vim25Client.RoundTripper, 1*time.Minute, soapKeepAliveHandler(ctx, vim25Client))

	manager := session.NewManager(vim25Client)
	err = manager.Login(ctx, urlinfo.User)
	if err != nil {
		return nil, err
	}

	logger.Printf("Created session for user %s", vmcfg.UserName)

	gclient := govmomi.Client{
		Client:         vim25Client,
		SessionManager: manager,
	}

	return &gclient, nil
}

func soapKeepAliveHandler(ctx context.Context, c *vim25.Client) func() error {

	return func() error {

		_, err := methods.GetCurrentTime(ctx, c)
		if err != nil {
			logger.Printf("SOAP keep-alive handler error %s", err)
			return err
		}

		return nil
	}
}

func DeleteGovmomiClient(gclient *govmomi.Client) {

	glock.Lock()

	defer glock.Unlock()

	err := gclient.SessionManager.Logout(context.Background())
	if err != nil {
		logger.Printf("Vcenter logout failed error: %s", err)
	}
}

func CheckSessionWithRestore(ctx context.Context, vmcfg *Config, gclient *govmomi.Client) error {

	glock.Lock()

	defer glock.Unlock()

	active, err := gclient.SessionManager.SessionIsActive(ctx)

	if active {
		if err == nil {
			return nil
		}
	}

	if err != nil {
		logger.Printf("Creating new sesssion for user %s due to current session error: %s", vmcfg.UserName, err)
	}

	_ = gclient.SessionManager.Logout(ctx) // Cleanup purposes

	urlinfo, _ := soap.ParseURL(vmcfg.VcenterURL)

	urlinfo.User = url.UserPassword(vmcfg.UserName, vmcfg.Password)

	err = gclient.SessionManager.Login(ctx, urlinfo.User)
	if err != nil {
		logger.Printf("Vcenter login failed error: %s", err)
		return err
	}

	logger.Printf("Created new session for user %s", vmcfg.UserName)

	return nil
}

type VmConfig []types.BaseOptionValue

func CreateInstance(ctx context.Context, vim25Client *vim25.Client, vmcfg *Config, vmname string, userData string) (*createInstanceOutput, error) {

	// this creates and starts the VM

	finder := find.NewFinder(vim25Client)

	// If vmcfg.Datacenter is null DatacenterOrDefault will return the default datacenter
	dc, err := finder.DatacenterOrDefault(ctx, vmcfg.Datacenter)
	if err != nil {
		logger.Printf("Cannot find vcenter datacenter %s error: %s", vmcfg.Datacenter, err)
		return nil, err
	}

	vmcfg.Datacenter = dc.Name()

	logger.Printf("Datacenter now is %s", vmcfg.Datacenter)

	finder.SetDatacenter(dc)

	// TODO change to tpl, err := template.FindTemplate(ctx, ctx.VSphereVM.Spec.Template)
	// "github.com/vmware/govmomi/template"

	vm, err := finder.VirtualMachine(ctx, vmcfg.Template)
	if err != nil {
		logger.Printf("Cannot find VM template %s error: %s", vmcfg.Template, err)
		return nil, err
	}

	// TODO for DRS ResourcePoolOrDefault()
	pool, err := finder.DefaultResourcePool(ctx)
	if err != nil {
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

	deploy_path_dirs := strings.Split(vmcfg.Deployfolder, "/")
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

	data := []byte(userData)

	for {
		decoded, err := base64.StdEncoding.DecodeString(string(data))
		if err != nil {
			break
		}
		data = decoded
	}

	userDataEnc := base64.StdEncoding.EncodeToString(data)

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

	if vmcfg.Datastore != "" {
		datastorepath := fmt.Sprintf("/%s/datastore/%s", vmcfg.Datacenter, vmcfg.Datastore)
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

	clone := object.NewVirtualMachine(vim25Client, info.Result.(types.ManagedObjectReference))
	name, err := clone.ObjectName(ctx)
	if err != nil {
		return nil, err
	}

	logger.Printf("clone %s, Clone UUID %s created", name, clone.UUID(ctx))

	//uuid := strings.Replace(mvm.Config.Uuid, "-", "", -1)

	podNodeIPs, err := getIPs(clone) // TODO Fix to get all ips
	if err != nil {
		logger.Printf("failed to get IPs for the instance : %v ", err)
		return nil, err
	}

	return &createInstanceOutput{
		uuid: clone.UUID(ctx),
		ips:  podNodeIPs,
	}, nil
}

func getIPs(vm *object.VirtualMachine) ([]net.IP, error) { // TODO Fix to get all ips
	var podNodeIPs []net.IP

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(300*time.Second))
	defer cancel()

	logger.Printf("start waiting for cloned vm ip")
	ip, err := vm.WaitForIP(ctx, true)
	if err != nil {
		return nil, err
	}

	logger.Printf("VM IP=%s", ip)
	ip_item := net.ParseIP(ip)
	if ip_item == nil {
		return nil, fmt.Errorf("failed to parse pod node IP %q", ip)
	}
	podNodeIPs = append(podNodeIPs, ip_item)

	return podNodeIPs, nil
}

func DeleteInstance(ctx context.Context, vim25Client *vim25.Client, vmcfg *Config, vmname string) (err error) {

	var (
		task  *object.Task
		state types.VirtualMachinePowerState
	)

	finder := find.NewFinder(vim25Client)

	// If vmcfg.Datacenter is null DatacenterOrDefault will return the default datacenter
	dc, err := finder.DatacenterOrDefault(ctx, vmcfg.Datacenter)

	if err != nil {
		logger.Printf("can't get vcenter datacenter %s", vmcfg.Datacenter)
		return err
	}

	finder.SetDatacenter(dc)

	vm_path := path.Join(dc.InventoryPath, "vm", vmcfg.Deployfolder, vmname)

	vm, err := finder.VirtualMachine(ctx, vm_path) //TODO may need find by UUID
	if err != nil {
		logger.Printf("can't find VM %s to delete it", vm_path)
		return err
	}

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

	return nil
}
