//go:build libvirt
// +build libvirt

package libvirt

import (
	"context"
	"encoding/xml"
	"fmt"
	libvirt "libvirt.org/go/libvirt"
	libvirtxml "libvirt.org/go/libvirtxml"
	"log"
	"net"
	"os"
	"os/exec"
	"strconv"
	"time"
)

type libvirtClient struct {
	connection *libvirt.Connect

	// storage pool that holds all volumes
	pool *libvirt.StoragePool
	// cache pool's name so we don't have to call failable GetName() method on pool all the time.
	poolName string

	// libvirt network name
	networkName string

	dataDir string
}

// Create a base volume
// Create qcow2 image with prerequisites
// virsh vol-create-as --pool default --name podvm-base.qcow2 --capacity 107374182400 --allocation 2361393152 --prealloc-metadata --format qcow2
// virsh vol-upload --vol podvm-base.qcow2 ./podvm.qcow2 --pool default --sparse
const podBaseVolName = "podvm-base.qcow2"

func createCloudInitISO(v *vmConfig, libvirtClient *libvirtClient) string {
	logger.Printf("Create cloudInit iso\n")
	cloudInitIso := libvirtClient.dataDir + "/" + v.name + "-cloudinit.iso"

	if _, err := os.Stat("/usr/bin/genisoimage"); os.IsNotExist(err) {
		log.Fatal("'genisoimage' command doesn't exist.Please install the command before.")
	}

	// Set VM Hostname
	if err := os.MkdirAll(libvirtClient.dataDir, os.ModePerm); err != nil {
		log.Fatalf("Failed to create data-dir path %s: %s", v.metaData, err)
	}
	v.metaData = libvirtClient.dataDir + "/" + "meta-data"
	metaFile, _ := os.Create(v.metaData)
	if _, err := metaFile.WriteString("local-hostname: " + v.name); err != nil {
		metaFile.Close()
		log.Fatalf("Failed to write to %s: %s", v.metaData, err)
	}
	metaFile.Close()

	// Write the userData to a file
	userDataFile := libvirtClient.dataDir + "/" + "user-data"
	udf, _ := os.Create(userDataFile)
	if _, err := udf.WriteString(v.userData); err != nil {
		udf.Close()
		log.Fatalf("Failed to write to %s: %s", userDataFile, err)
	}
	udf.Close()

	fmt.Printf("Executing genisoimage\n")
	// genisoimage -output cloudInitIso.iso -volid cidata -joliet -rock user-data meta-data
	cmd := exec.Command("genisoimage", "-output", cloudInitIso, "-volid", "cidata", "-joliet", "-rock", userDataFile, v.metaData)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		log.Fatal(err)
	}
	logger.Printf("Created cloudInit iso\n")
	return cloudInitIso
}

func checkInstanceExistsByName(name string, libvirtClient *libvirtClient) (exist bool, err error) {

	logger.Printf("Checking if instance (%s) exists", name)
	domain, err := libvirtClient.connection.LookupDomainByName(name)
	if err != nil {
		if err.(libvirt.Error).Code == libvirt.ERR_NO_DOMAIN {
			return false, nil
		}
		return false, err
	}
	defer freeDomain(domain, &err)

	return true, nil

}

func checkInstanceExistsById(id uint32, libvirtClient *libvirtClient) (exist bool, err error) {

	logger.Printf("Checking if instance (%d) exists", id)
	domain, err := libvirtClient.connection.LookupDomainById(id)
	if err != nil {
		if err.(libvirt.Error).Code == libvirt.ERR_NO_DOMAIN {
			return false, nil
		}
		return false, err
	}
	defer freeDomain(domain, &err)

	return true, nil

}

func uploadIso(isoFile string, isoVolName string, libvirtClient *libvirtClient) (string, error) {

	fmt.Printf("Uploading iso file: %s\n", isoFile)
	volumeDef := newDefVolume(isoVolName)

	img, err := newImage(isoFile)
	if err != nil {
		return "", err
	}

	size, err := img.size()
	if err != nil {
		return "", err
	}

	volumeDef.Capacity.Unit = "B"
	volumeDef.Capacity.Value = size
	volumeDef.Target.Format.Type = "raw"

	return uploadVolume(libvirtClient, volumeDef, img)

}

func CreateInstance(ctx context.Context, libvirtClient *libvirtClient, v *vmConfig) (result *createInstanceOutput, err error) {

	v.cpu = uint(2)
	v.mem = uint(8)
	v.rootDiskSize = uint64(10)

	exists, err := checkInstanceExistsByName(v.name, libvirtClient)
	if err != nil {
		return nil, fmt.Errorf("Error in checking instance: %s", err)
	}
	if exists {
		logger.Printf("Instance already exists ")
		return &createInstanceOutput{
			instance: v,
		}, nil
	}

	rootVolName := v.name + "-root.qcow2"
	err = createVolume(rootVolName, v.rootDiskSize, podBaseVolName, libvirtClient)
	if err != nil {
		return nil, fmt.Errorf("Error in creating volume: %s", err)
	}

	cloudInitIso := createCloudInitISO(v, libvirtClient)

	isoVolName := v.name + "-cloudinit.iso"
	isoVolFile, err := uploadIso(cloudInitIso, isoVolName, libvirtClient)
	if err != nil {
		return nil, fmt.Errorf("Error in uploading iso volume: %s", err)
	}

	rootVol, err := getVolume(libvirtClient, rootVolName)
	if err != nil {
		return nil, fmt.Errorf("Error retrieving volume: %s", err)
	}

	rootVolFile, err := rootVol.GetPath()
	if err != nil {
		return nil, fmt.Errorf("Error retrieving volume path: %s", err)
	}

	// Gen Domain XML.
	var diskControllerAddr uint = 0
	domCfg := &libvirtxml.Domain{
		Type:        "kvm",
		Name:        v.name,
		Description: "This Virtual Machine is the peer-pod VM",
		Memory:      &libvirtxml.DomainMemory{Value: uint(v.mem), Unit: "GiB", DumpCore: "on"},
		VCPU:        &libvirtxml.DomainVCPU{Value: uint(v.cpu)},
		OS: &libvirtxml.DomainOS{
			Type: &libvirtxml.DomainOSType{Arch: "x86_64", Type: "hvm"},
		},
		// For Hot-Plug Feature.
		Features: &libvirtxml.DomainFeatureList{
			ACPI:   &libvirtxml.DomainFeature{},
			APIC:   &libvirtxml.DomainFeatureAPIC{},
			VMPort: &libvirtxml.DomainFeatureState{State: "off"},
		},
		CPU:      &libvirtxml.DomainCPU{Mode: "host-model"},
		OnReboot: "restart",
		Devices: &libvirtxml.DomainDeviceList{
			// Disks.
			Disks: []libvirtxml.DomainDisk{
				{
					Device: "disk",
					Driver: &libvirtxml.DomainDiskDriver{Type: "qcow2"},
					Source: &libvirtxml.DomainDiskSource{
						File: &libvirtxml.DomainDiskSourceFile{
							File: rootVolFile}},
					Target: &libvirtxml.DomainDiskTarget{
						Dev: "sda", Bus: "sata"},
					Boot: &libvirtxml.DomainDeviceBoot{Order: 1},
					Address: &libvirtxml.DomainAddress{
						Drive: &libvirtxml.DomainAddressDrive{
							Controller: &diskControllerAddr, Bus: &diskControllerAddr, Target: &diskControllerAddr, Unit: &diskControllerAddr}},
				},
				{
					Device: "cdrom",
					Driver: &libvirtxml.DomainDiskDriver{Name: "qemu", Type: "raw"},
					Source: &libvirtxml.DomainDiskSource{
						File: &libvirtxml.DomainDiskSourceFile{File: isoVolFile},
					},
					Target:   &libvirtxml.DomainDiskTarget{Dev: "hda", Bus: "ide"},
					ReadOnly: &libvirtxml.DomainDiskReadOnly{},
					Address: &libvirtxml.DomainAddress{
						Drive: &libvirtxml.DomainAddressDrive{
							Controller: &diskControllerAddr, Bus: &diskControllerAddr, Target: &diskControllerAddr, Unit: &diskControllerAddr}},
				},
			},
			// Network Interfaces.
			Interfaces: []libvirtxml.DomainInterface{
				{
					Source: &libvirtxml.DomainInterfaceSource{Network: &libvirtxml.DomainInterfaceSourceNetwork{Network: libvirtClient.networkName}},
					Model:  &libvirtxml.DomainInterfaceModel{Type: "virtio"},
				},
			},
			// Serial Console Devices.
			Consoles: []libvirtxml.DomainConsole{
				{
					Target: &libvirtxml.DomainConsoleTarget{Type: "serial"},
				},
			},
		},
	}

	logger.Printf("Create XML for '%s'", v.name)
	domXML, err := domCfg.Marshal()
	if err != nil {
		return nil, fmt.Errorf("Failed to create domain xml: %s", err)
	}

	logger.Printf("Creating VM '%s'", v.name)
	dom, err := libvirtClient.connection.DomainDefineXML(domXML)
	if err != nil {
		return nil, fmt.Errorf("Failed to define domain: %s", err)
	}

	// Start Domain.
	logger.Printf("Starting VM '%s'", v.name)
	err = dom.Create()
	if err != nil {
		return nil, fmt.Errorf("Failed to start VM: %s", err)
	}

	id, err := dom.GetID()
	if err != nil {
		return nil, fmt.Errorf("Failed to get domain ID: %s", err)
	}

	v.instanceId = strconv.FormatUint(uint64(id), 10)
	logger.Printf("VM id %s", v.instanceId)

	// Wait for sometime for the IP to be visible
	// TBD: Figure out a better mechanism
	time.Sleep(30 * time.Second)

	domInterface, err := dom.ListAllInterfaceAddresses(libvirt.DOMAIN_INTERFACE_ADDRESSES_SRC_LEASE)
	if err != nil {
		return nil, fmt.Errorf("Failed to get domain interfaces: %s", err)
	}

	logger.Printf("domain IP details %v", domInterface)

	if len(domInterface) > 0 {
		// TBD: ability to handle multiple interfaces and ips
		logger.Printf("VM IP %s", domInterface[0].Addrs[0].Addr)
		v.ips = append(v.ips, net.ParseIP(domInterface[0].Addrs[0].Addr))
		logger.Printf("VM IP list %v", v.ips)
	}

	logger.Printf("Instance created successfully")
	return &createInstanceOutput{
		instance: v,
	}, nil
}

func DeleteInstance(ctx context.Context, libvirtClient *libvirtClient, id string) (err error) {

	logger.Printf("Deleting instance (%s)", id)
	idUint, _ := strconv.ParseUint(id, 10, 64)
	// libvirt API takes uint32
	exists, err := checkInstanceExistsById(uint32(idUint), libvirtClient)
	if err != nil {
		logger.Printf("Unable to check instance (%s)", id)
		return err
	}
	if !exists {
		logger.Printf("Instance (%s) not found", id)
		return err
	}
	// Stop and undefine domain

	// Sadly couldn't find an API to do the following
	// virsh undefine <domid> --remove-all-storage

	domain, err := libvirtClient.connection.LookupDomainById(uint32(idUint))
	if err != nil {
		logger.Printf("Error retrieving libvirt domain: %s", err)
		return err
	}
	defer freeDomain(domain, &err)

	state, _, err := domain.GetState()
	if err != nil {
		logger.Printf("Couldn't get info about domain: %s", err)
		return err
	}

	if state == libvirt.DOMAIN_RUNNING || state == libvirt.DOMAIN_PAUSED {
		if err = domain.Destroy(); err != nil {
			logger.Printf("Couldn't destroy libvirt domain: %s", err)
			return err
		}
	}

	// Delete volumes
	domainXMLDesc, err := domain.GetXMLDesc(0)
	if err != nil {
		logger.Printf("Error retrieving libvirt domain XML description: %s", err)
		return err
	}
	domainDef := libvirtxml.Domain{}
	err = xml.Unmarshal([]byte(domainXMLDesc), &domainDef)
	if err != nil {
		logger.Printf("Unable to get the domain XML: %s", err)
	}

	// Get the volume path from the XML
	logger.Printf("domainDef %v", domainDef.Devices.Disks)
	vol1File := domainDef.Devices.Disks[0].Source.File.File
	vol2File := domainDef.Devices.Disks[1].Source.File.File

	err = deleteVolumeByPath(libvirtClient, vol1File)
	if err != nil {
		logger.Printf("Deleting volume (%s) returned error: %s", vol1File, err)
	}
	err = deleteVolumeByPath(libvirtClient, vol2File)
	if err != nil {
		logger.Printf("Deleting volume (%s) returned error: %s", vol2File, err)
	}
	// Undefine the domain
	if err := domain.UndefineFlags(libvirt.DOMAIN_UNDEFINE_NVRAM); err != nil {
		if e := err.(libvirt.Error); e.Code == libvirt.ERR_NO_SUPPORT || e.Code == libvirt.ERR_INVALID_ARG {
			logger.Printf("libvirt does not support undefine flags: will try again without flags")
			if err = domain.Undefine(); err != nil {
				logger.Printf("couldn't undefine libvirt domain: %v", err)
				return err
			}
		} else {
			logger.Printf("couldn't undefine libvirt domain with flags: %v", err)
			return err
		}
	}

	return nil
}

func NewLibvirtClient(libvirtCfg Config) (*libvirtClient, error) {

	// Define Domain via XML created before.
	conn, err := libvirt.NewConnect(libvirtCfg.URI)
	if err != nil {
		return nil, err
	}

	pool, err := conn.LookupStoragePoolByName(libvirtCfg.PoolName)
	if err != nil {
		return nil, fmt.Errorf("can't find storage pool %q: %v", libvirtCfg.PoolName, err)
	}

	fmt.Printf("Created libvirt connection")

	return &libvirtClient{
		connection:  conn,
		pool:        pool,
		poolName:    libvirtCfg.PoolName,
		networkName: libvirtCfg.NetworkName,
		dataDir:     libvirtCfg.DataDir,
	}, nil
}

// freeDomain releases the domain pointer. If the operation fail and the error
// context is nil then it gets updated, otherwise it preserve the pointer to
// keep any previous error reported.
func freeDomain(domain *libvirt.Domain, errCtx *error) {
	newErr := domain.Free()
	if newErr != nil && *errCtx == nil {
		*errCtx = newErr
	}
}
