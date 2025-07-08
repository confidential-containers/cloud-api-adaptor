//go:build cgo

// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package libvirt

import (
	"context"
	"encoding/xml"
	"fmt"
	"net/netip"
	"strconv"
	"time"

	retry "github.com/avast/retry-go/v4"
	libvirt "libvirt.org/go/libvirt"
	libvirtxml "libvirt.org/go/libvirtxml"
)

const (
	// architecture value for the s390x architecture
	archS390x = "s390x"
	// architecutre value for aarch64/arm64
	archAArch64 = "aarch64"
	// hvm indicates that the OS is one designed to run on bare metal, so requires full virtualization.
	typeHardwareVirtualMachine = "hvm"
	// The amount of retries to get the domain IP addresses
	GetDomainIPsRetries = 20
	// The sleep time between retries to get the domain IP addresses
	GetDomainIPsSleep = time.Second * 3
)

type domainConfig struct {
	name        string
	cpu         uint
	mem         uint
	networkName string
	bootDisk    string
	cidataDisk  string
}

// createCloudInitISO creates an ISO file with a userdata and a metadata file. The ISO image will be created in-memory since it is small
func createCloudInitISO(v *vmConfig) ([]byte, error) {
	logger.Println("Create cloudInit iso")

	userData := v.userData
	metaData := fmt.Sprintf("local-hostname: %s", v.name)

	return createCloudInit([]byte(userData), []byte(metaData))
}

func checkDomainExistsByName(name string, libvirtClient *libvirtClient) (exist bool, err error) {

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

func checkDomainExistsById(id uint32, libvirtClient *libvirtClient) (exist bool, err error) {

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

func uploadIso(isoData []byte, isoVolName string, libvirtClient *libvirtClient) (string, error) {

	logger.Printf("Uploading iso file: %s\n", isoVolName)
	volumeDef := newDefVolume(isoVolName)

	img, err := newImageFromBytes(isoData)
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

func getGuestForArchType(caps *libvirtxml.Caps, arch string, ostype string) (*libvirtxml.CapsGuest, error) {
	for _, guest := range caps.Guests {
		if guest.Arch.Name == arch && guest.OSType == ostype {
			return &guest, nil
		}
	}
	return nil, fmt.Errorf("could not find any guests for architecture type %s/%s", ostype, arch)
}

// getHostCapabilities returns the host capabilities as a struct
func getHostCapabilities(conn *libvirt.Connect) (*libvirtxml.Caps, error) {
	capsXML, err := conn.GetCapabilities()
	if err != nil {
		return nil, fmt.Errorf("unable to get capabilities, cause: %w", err)
	}

	caps := &libvirtxml.Caps{}
	err = xml.Unmarshal([]byte(capsXML), caps)
	if err != nil {
		return nil, fmt.Errorf("unable to unmarshal capabilities, cause: %w", err)
	}

	return caps, nil
}

func GetDomainCapabilities(conn *libvirt.Connect, emulatorbin string, arch string, machine string, virttype string, flags uint32) (*libvirtxml.DomainCaps, error) {
	capsXML, err := conn.GetDomainCapabilities(emulatorbin, arch, machine, virttype, flags)
	if err != nil {
		return nil, fmt.Errorf("unable to get domain capabilities, cause: %w", err)
	}
	caps := &libvirtxml.DomainCaps{}
	err = xml.Unmarshal([]byte(capsXML), caps)
	if err != nil {
		return nil, fmt.Errorf("unable to unmarshal domain capabilities, cause: %w", err)

	}
	return caps, nil
}

// lookupMachine finds the machine name from the set of available machines
func lookupMachine(machines []libvirtxml.CapsGuestMachine, targetmachine string) string {
	for _, machine := range machines {
		if machine.Name == targetmachine {
			if machine.Canonical != "" {
				return machine.Canonical
			}
			return machine.Name
		}
	}
	return ""
}

// getCanonicalMachineName returns the default (canonical) name of the guest machine based on capabilities
// this is equivalent to doing a `virsh capabilities` and then looking at the `machine` configuration, e.g. `<machine canonical='s390-ccw-virtio-rhel9.0.0' maxCpus='248'>s390-ccw-virtio</machine>`
func getCanonicalMachineName(caps *libvirtxml.Caps, arch string, virttype string, targetmachine string) (string, error) {
	guest, err := getGuestForArchType(caps, arch, virttype)
	if err != nil {
		return "", err
	}

	name := lookupMachine(guest.Arch.Machines, targetmachine)
	if name != "" {
		return name, nil
	}

	for _, domain := range guest.Arch.Domains {
		name := lookupMachine(domain.Machines, targetmachine)
		if name != "" {
			return name, nil
		}
	}

	return "", fmt.Errorf("cannot find machine type %s for %s/%s in %v", targetmachine, virttype, arch, caps)
}

func createDomainXMLs390x(client *libvirtClient, cfg *domainConfig, vm *vmConfig) (*libvirtxml.Domain, error) {

	guest, err := getGuestForArchType(client.caps, archS390x, typeHardwareVirtualMachine)
	if err != nil {
		return nil, err
	}

	canonicalmachine, err := getCanonicalMachineName(client.caps, archS390x, typeHardwareVirtualMachine, "s390-ccw-virtio")
	if err != nil {
		return nil, err
	}

	bootDisk := libvirtxml.DomainDisk{
		Device: "disk",
		Target: &libvirtxml.DomainDiskTarget{
			Dev: "vda",
			Bus: "virtio",
		},
		Driver: &libvirtxml.DomainDiskDriver{
			Name:  "qemu",
			Type:  "qcow2",
			IOMMU: "on",
		},
		Source: &libvirtxml.DomainDiskSource{
			File: &libvirtxml.DomainDiskSourceFile{
				File: cfg.bootDisk,
			},
		},
		Boot: &libvirtxml.DomainDeviceBoot{
			Order: 1,
		},
	}

	cloudInitDisk := libvirtxml.DomainDisk{
		Device: "disk",
		Target: &libvirtxml.DomainDiskTarget{
			Dev: "vdb",
			Bus: "virtio",
		},
		Driver: &libvirtxml.DomainDiskDriver{
			Name:  "qemu",
			Type:  "raw",
			IOMMU: "on",
		},
		Source: &libvirtxml.DomainDiskSource{
			File: &libvirtxml.DomainDiskSourceFile{
				File: cfg.cidataDisk,
			},
		},
	}

	return &libvirtxml.Domain{
		Type:        "kvm",
		Name:        cfg.name,
		Description: "This Virtual Machine is the peer-pod VM",
		OS: &libvirtxml.DomainOS{
			Type: &libvirtxml.DomainOSType{
				Type:    typeHardwareVirtualMachine,
				Arch:    archS390x,
				Machine: canonicalmachine,
			},
		},
		Metadata: &libvirtxml.DomainMetadata{},
		Memory: &libvirtxml.DomainMemory{
			Value: cfg.mem, Unit: "MiB",
		},
		CurrentMemory: &libvirtxml.DomainCurrentMemory{
			Value: cfg.mem, Unit: "MiB",
		},
		VCPU: &libvirtxml.DomainVCPU{
			Value: cfg.cpu,
		},
		Clock: &libvirtxml.DomainClock{
			Offset: "utc",
		},
		Devices: &libvirtxml.DomainDeviceList{
			Disks: []libvirtxml.DomainDisk{
				bootDisk,
				cloudInitDisk,
			},
			Emulator: guest.Arch.Emulator,
			MemBalloon: &libvirtxml.DomainMemBalloon{
				Model: "none",
			},
			RNGs: []libvirtxml.DomainRNG{
				{
					Model: "virtio",
					Backend: &libvirtxml.DomainRNGBackend{
						Random: &libvirtxml.DomainRNGBackendRandom{Device: "/dev/urandom"},
					},
				},
			},
			Consoles: []libvirtxml.DomainConsole{
				{
					Source: &libvirtxml.DomainChardevSource{
						Pty: &libvirtxml.DomainChardevSourcePty{},
					},
					Target: &libvirtxml.DomainConsoleTarget{
						Type: "sclp",
					},
				},
			},
			Interfaces: []libvirtxml.DomainInterface{
				{
					Model: &libvirtxml.DomainInterfaceModel{
						Type: "virtio",
					},
					Source: &libvirtxml.DomainInterfaceSource{
						Network: &libvirtxml.DomainInterfaceSourceNetwork{
							Network: cfg.networkName,
						},
					},
					Driver: &libvirtxml.DomainInterfaceDriver{
						IOMMU: "on",
					},
				},
			},
		},
	}, nil
}

func createDomainXMLx86_64(client *libvirtClient, cfg *domainConfig, vm *vmConfig) (*libvirtxml.Domain, error) {

	var diskControllerAddr uint = 0
	domain := &libvirtxml.Domain{
		Type:        "kvm",
		Name:        cfg.name,
		Description: "This Virtual Machine is the peer-pod VM",
		Memory:      &libvirtxml.DomainMemory{Value: cfg.mem, Unit: "MiB", DumpCore: "on"},
		VCPU:        &libvirtxml.DomainVCPU{Value: cfg.cpu},
		OS: &libvirtxml.DomainOS{
			Type: &libvirtxml.DomainOSType{Arch: "x86_64", Type: typeHardwareVirtualMachine},
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
							File: cfg.bootDisk}},
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
						File: &libvirtxml.DomainDiskSourceFile{File: cfg.cidataDisk},
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
					Source: &libvirtxml.DomainInterfaceSource{Network: &libvirtxml.DomainInterfaceSourceNetwork{Network: cfg.networkName}},
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

	if vm.firmware != "" {
		domain.OS.Loader = &libvirtxml.DomainLoader{
			Path:     vm.firmware,
			Readonly: "yes",
			Type:     "pflash",
		}

		domain.OS.Firmware = "efi"

		// TODO - IDE seems to only work with packer builds and sata only with mkosi,
		// so we temporarily use the firmware being non-blank to assume this is mkosi
		cidataDiskIndex := 1
		var cidataDiskAddr uint = 1
		domain.Devices.Disks[cidataDiskIndex].Target.Bus = "sata"
		domain.Devices.Disks[cidataDiskIndex].Target.Dev = "sdb"
		domain.Devices.Disks[cidataDiskIndex].Address.Drive.Unit = &cidataDiskAddr
	}

	switch l := vm.launchSecurityType; l {
	case NoLaunchSecurity:
		return domain, nil
	default:
		return nil, fmt.Errorf("launch Security type is not supported for this domain: %s", l)
	}

}

func createDomainXMLaarch64(client *libvirtClient, cfg *domainConfig, vm *vmConfig) (*libvirtxml.Domain, error) {

	guest, err := getGuestForArchType(client.caps, archAArch64, typeHardwareVirtualMachine)
	if err != nil {
		return nil, err
	}
	canonicalmachine, err := getCanonicalMachineName(client.caps, archAArch64, typeHardwareVirtualMachine, "virt")
	if err != nil {
		return nil, err
	}

	bootDisk := libvirtxml.DomainDisk{
		Device: "disk",
		Target: &libvirtxml.DomainDiskTarget{
			Dev: "vda",
			Bus: "virtio",
		},
		Driver: &libvirtxml.DomainDiskDriver{
			Name: "qemu",
			Type: "qcow2",
			// only for virtio device
			IOMMU: "on",
		},
		Source: &libvirtxml.DomainDiskSource{
			File: &libvirtxml.DomainDiskSourceFile{
				File: cfg.bootDisk,
			},
		},
		Boot: &libvirtxml.DomainDeviceBoot{
			Order: 1,
		},
	}
	cloudInitDisk := libvirtxml.DomainDisk{
		Device: "cdrom",
		Target: &libvirtxml.DomainDiskTarget{
			// logical dev name, just a hint
			Dev: "sda",
			Bus: "scsi",
		},
		Driver: &libvirtxml.DomainDiskDriver{
			Name: "qemu",
			Type: "raw",
		},
		Source: &libvirtxml.DomainDiskSource{
			File: &libvirtxml.DomainDiskSourceFile{
				File: cfg.cidataDisk,
			},
		},
		ReadOnly: &libvirtxml.DomainDiskReadOnly{},
	}

	domain := &libvirtxml.Domain{
		Type:        "kvm",
		Name:        cfg.name,
		Description: "This Virtual Machine is the peer-pod VM",
		OS: &libvirtxml.DomainOS{
			Type: &libvirtxml.DomainOSType{
				Type:    typeHardwareVirtualMachine,
				Arch:    archAArch64,
				Machine: canonicalmachine,
			},
			// firmware autoselection since libvirt v5.2.0
			// https://libvirt.org/formatdomain.html#bios-bootloader
			Firmware: "efi",
		},
		Memory: &libvirtxml.DomainMemory{Value: cfg.mem, Unit: "MiB"},
		VCPU:   &libvirtxml.DomainVCPU{Value: cfg.cpu},
		CPU:    &libvirtxml.DomainCPU{Mode: "host-passthrough"},
		Devices: &libvirtxml.DomainDeviceList{
			Disks: []libvirtxml.DomainDisk{
				bootDisk,
				cloudInitDisk,
			},
			// scsi target device for readonly ROM device
			// virtio-scsi controller for better compatibility
			Controllers: []libvirtxml.DomainController{
				{
					Type:  "scsi",
					Model: "virtio-scsi",
				},
			},
			Emulator:   guest.Arch.Emulator,
			MemBalloon: &libvirtxml.DomainMemBalloon{Model: "virtio", Driver: &libvirtxml.DomainMemBalloonDriver{IOMMU: "on"}},
			Interfaces: []libvirtxml.DomainInterface{
				{
					Model: &libvirtxml.DomainInterfaceModel{
						Type: "virtio",
					},
					Source: &libvirtxml.DomainInterfaceSource{
						Network: &libvirtxml.DomainInterfaceSourceNetwork{
							Network: cfg.networkName,
						},
					},
					Driver: &libvirtxml.DomainInterfaceDriver{
						IOMMU: "on",
					},
				},
			},
			Consoles: []libvirtxml.DomainConsole{
				{
					Target: &libvirtxml.DomainConsoleTarget{Type: "serial"},
				},
			},
		},
	}

	return domain, nil
}

// createDomainXML detects the machine type of the libvirt host and will return a libvirt XML for that machine type
func createDomainXML(client *libvirtClient, cfg *domainConfig, vm *vmConfig) (*libvirtxml.Domain, error) {
	switch client.nodeInfo.Model {
	case archS390x:
		return createDomainXMLs390x(client, cfg, vm)
	case archAArch64:
		return createDomainXMLaarch64(client, cfg, vm)
	default:
		return createDomainXMLx86_64(client, cfg, vm)
	}
}

// getDomainIPs get all IP addresses of all domain network interfaces
//
// Note that at the time this function is called the domain might
// not get IP addresses yet, so the list will be empty and none
// error is returned.
func getDomainIPs(dom *libvirt.Domain) ([]netip.Addr, error) {
	ips := []netip.Addr{}

	domIfList, err := dom.ListAllInterfaceAddresses(libvirt.DOMAIN_INTERFACE_ADDRESSES_SRC_LEASE)
	if err != nil {
		domName, _ := dom.GetName()
		return nil, fmt.Errorf("Failed to get domain %s interfaces: %s", domName, err)
	}

	for _, domIf := range domIfList {
		for _, addr := range domIf.Addrs {
			parsedAddr, err := netip.ParseAddr(addr.Addr)
			if err != nil {
				return nil, fmt.Errorf("Failed to parse address: %s", err)
			}
			ips = append(ips, parsedAddr)
		}
	}

	return ips, nil
}

func CreateDomain(ctx context.Context, libvirtClient *libvirtClient, v *vmConfig) (result *createDomainOutput, err error) {

	v.rootDiskSize = uint64(10)

	exists, err := checkDomainExistsByName(v.name, libvirtClient)
	if err != nil {
		return nil, fmt.Errorf("Error in checking instance: %s", err)
	}
	if exists {
		logger.Printf("Instance already exists ")
		return &createDomainOutput{
			instance: v,
		}, nil
	}

	rootVolName := v.name + "-root.qcow2"
	err = createVolume(rootVolName, v.rootDiskSize, libvirtClient.volName, libvirtClient)
	if err != nil {
		return nil, fmt.Errorf("Error in creating volume: %s", err)
	}

	cloudInitIso, err := createCloudInitISO(v)
	if err != nil {
		return nil, fmt.Errorf("error in creating cloud init ISO file, cause: %w", err)
	}

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

	domainCfg := domainConfig{
		name:        v.name,
		cpu:         v.cpu,
		mem:         v.mem,
		networkName: libvirtClient.networkName,
		bootDisk:    rootVolFile,
		cidataDisk:  isoVolFile,
	}

	domCfg, err := createDomainXML(libvirtClient, &domainCfg, v)
	if err != nil {
		return nil, fmt.Errorf("error building the libvirt XML, cause: %w", err)
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
	if err := retry.Do(
		func() error {
			ips, err := getDomainIPs(dom)
			if err != nil {
				// Something went completely wrong so it should return immediately
				return retry.Unrecoverable(fmt.Errorf("Internal error on getting domain IPs: %s", err))
			}

			if len(ips) > 0 {
				return nil
			}
			return fmt.Errorf("Domain has not IPs assigned yet")
		},
		retry.Attempts(GetDomainIPsRetries),
		retry.Delay(GetDomainIPsSleep),
	); err != nil {
		logger.Printf("Unable to get IP addresses after %d retries (sleep time=%ds): %s",
			GetDomainIPsRetries, GetDomainIPsSleep, err)
		return nil, fmt.Errorf("Domain (id=%d) IP addresses not found", id)
	}

	if v.ips, err = getDomainIPs(dom); err != nil {
		return nil, fmt.Errorf("Internal error on getting domain IPs: %s", err)
	}

	logger.Printf("Instance created successfully")
	return &createDomainOutput{
		instance: v,
	}, nil
}

func DeleteDomain(ctx context.Context, libvirtClient *libvirtClient, id string) (err error) {

	logger.Printf("Deleting instance (%s)", id)
	idUint, _ := strconv.ParseUint(id, 10, 32)
	// libvirt API takes uint32
	exists, err := checkDomainExistsById(uint32(idUint), libvirtClient)
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

	node, err := conn.GetNodeInfo()
	if err != nil {
		return nil, fmt.Errorf("error retrieving node info: %w", err)
	}

	caps, err := getHostCapabilities(conn)
	if err != nil {
		return nil, err
	}

	logger.Println("Created libvirt connection")

	return &libvirtClient{
		connection:  conn,
		pool:        pool,
		poolName:    libvirtCfg.PoolName,
		networkName: libvirtCfg.NetworkName,
		dataDir:     libvirtCfg.DataDir,
		volName:     libvirtCfg.VolName,
		nodeInfo:    node,
		caps:        caps,
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

// Attempts to determine launchSecurity Type from domain capabilities and hardware
// Currently only supports S390PV
func GetLaunchSecurityType(uri string) (LaunchSecurityType, error) {
	conn, err := libvirt.NewConnect(uri)
	if err != nil {
		return NoLaunchSecurity, fmt.Errorf("unable to get libvirt connection [%v]", err)
	}

	nodeInfo, err := conn.GetNodeInfo()
	if err != nil {
		return NoLaunchSecurity, fmt.Errorf("error retrieving node info: %v", err)
	}

	switch nodeInfo.Model {
	case archS390x:
		return S390PV, nil
	case "x86_64":
		return NoLaunchSecurity, nil
	default:
		return NoLaunchSecurity, nil
	}
}
