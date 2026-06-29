// (C) Copyright IBM Corp. 2022.
// SPDX-License-Identifier: Apache-2.0

package interceptor

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	retry "github.com/avast/retry-go/v4"
	pb "github.com/kata-containers/kata-containers/src/runtime/virtcontainers/pkg/agent/protocols/grpc"
	"github.com/moby/sys/mountinfo"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/pkg/util"
	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/pkg/util/agentproto"
)

const (
	volumeTargetPathKey  = "io.confidentialcontainers.org.peerpodvolumes.target_path"
	cloudVolumeMountBase = "/run/cloud-volumes"
	volumeCheckInterval  = 5 * time.Second
	volumeCheckTimeout   = 3 * time.Minute
	cdhSocketPath        = "/run/confidential-containers/cdh.sock"
)

var allowedFSTypes = map[string]bool{
	"ext4": true,
	"ext3": true,
	"xfs":  true,
}

var logger = log.New(log.Writer(), "[forwarder/interceptor] ", log.LstdFlags|log.Lmsgprefix)

type Interceptor interface {
	agentproto.Redirector
}

var allowedEncryptTypes = map[string]bool{
	"luks":  true,
	"luks2": true,
}

type cloudMount struct {
	path       string
	encrypted  bool
	mapperName string
}

type interceptor struct {
	agentproto.Redirector

	nsPath      string
	cloudMounts []cloudMount
}

// unmountCloudVolumes unmounts all cloud volume mount points in reverse order.
// For encrypted volumes, also closes the LUKS mapping after unmount.
func (i *interceptor) unmountCloudVolumes() {
	for idx := len(i.cloudMounts) - 1; idx >= 0; idx-- {
		cm := i.cloudMounts[idx]
		if err := syscall.Unmount(cm.path, 0); err != nil {
			logger.Printf("WARNING: failed to unmount cloud volume %s: %v", cm.path, err)
		} else {
			logger.Printf("Unmounted cloud volume %s", cm.path)
		}
		if cm.encrypted {
			name := cm.mapperName
			if name == "" {
				name = findMapperForMountPoint(cm.path)
			}
			if name == "" {
				logger.Printf("WARNING: cannot determine LUKS mapper name for %s, skipping cryptsetup close", cm.path)
				continue
			}
			if out, err := exec.Command("cryptsetup", "close", name).CombinedOutput(); err != nil {
				logger.Printf("WARNING: cryptsetup close %s failed: %v (%s)", name, err, string(out))
			} else {
				logger.Printf("Closed LUKS mapping %s", name)
			}
		}
	}
	i.cloudMounts = nil
}

// findMapperForMountPoint looks up /proc/mounts to find which
// /dev/mapper/* device is mounted at the given path.
func findMapperForMountPoint(mountPoint string) string {
	data, err := os.ReadFile("/proc/mounts")
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[1] == mountPoint && strings.HasPrefix(fields[0], "/dev/mapper/") {
			return strings.TrimPrefix(fields[0], "/dev/mapper/")
		}
	}
	return ""
}

func dial(ctx context.Context, agentSocket string) (net.Conn, error) {

	var conn net.Conn

	ctx, cancel := context.WithTimeout(ctx, 150*time.Second)
	defer cancel()

	logger.Printf("Trying to establish agent connection to %s", agentSocket)
	err := retry.Do(
		func() error {
			var err error
			conn, err = (&net.Dialer{}).DialContext(ctx, "unix", agentSocket)
			return err
		},
		retry.Context(ctx),
	)

	if err != nil {
		err = fmt.Errorf("failed to establish agent connection to %s: %w", agentSocket, err)
		logger.Print(err)
		return nil, err
	}

	logger.Printf("established agent connection to %s", agentSocket)
	return conn, nil
}

func NewInterceptor(agentSocket, nsPath string) Interceptor {

	agentDialer := func(ctx context.Context) (net.Conn, error) {
		return dial(ctx, agentSocket)
	}

	redirector := agentproto.NewRedirector(agentDialer)

	return &interceptor{
		Redirector: redirector,
		nsPath:     nsPath,
	}
}

func (i *interceptor) CreateContainer(ctx context.Context, req *pb.CreateContainerRequest) (*emptypb.Empty, error) {

	logger.Printf("CreateContainer: containerID:%s", req.ContainerId)

	// Specify the network namespace path in the container spec
	req.OCI.Linux.Namespaces = append(req.OCI.Linux.Namespaces, &pb.LinuxNamespace{
		Type: string(specs.NetworkNamespace),
		Path: i.nsPath,
	})

	logger.Printf("    namespaces:")
	for _, ns := range req.OCI.Linux.Namespaces {
		logger.Printf("    %s: %q", ns.Type, ns.Path)
	}

	// Handle cloud volumes: detect device, format if needed, mount, and
	// bind-mount into the container's mount namespace.
	if cvJSON, ok := req.OCI.Annotations[util.CloudVolumesAnnotationKey]; ok && cvJSON != "" {
		var cloudVolumes map[string]util.CloudVolumeAnnotation
		if err := json.Unmarshal([]byte(cvJSON), &cloudVolumes); err != nil {
			return nil, fmt.Errorf("corrupt cloud_volumes annotation (pod would start without volumes): %w", err)
		}

		volNames := make([]string, 0, len(cloudVolumes))
		for k := range cloudVolumes {
			volNames = append(volNames, k)
		}
		sort.Strings(volNames)

		for _, volName := range volNames {
			volInfo := cloudVolumes[volName]
			mountPoint := volInfo.MountPoint
			fsType := volInfo.FSType
			lunStr := volInfo.LUN
			if mountPoint == "" || lunStr == "" {
				return nil, fmt.Errorf("cloud volume %s missing required mount_point or lun field", volName)
			}

			safeName := filepath.Base(volName)
			if safeName != volName || safeName == "." || safeName == ".." {
				return nil, fmt.Errorf("cloud volume %q has unsafe name", volName)
			}

			if fsType == "" {
				fsType = "ext4"
			}
			if !allowedFSTypes[fsType] {
				return nil, fmt.Errorf("cloud volume %s requests unsupported filesystem type %q (allowed: ext4, ext3, xfs)", volName, fsType)
			}

			lunIdx, err := strconv.Atoi(lunStr)
			if err != nil {
				return nil, fmt.Errorf("cloud volume %s has invalid lun %q: %w", volName, lunStr, err)
			}

			diskID := volInfo.DiskID
			device, err := findDataDiskDevice(lunIdx, diskID)
			if err != nil {
				return nil, fmt.Errorf("cloud volume %s: %w", volName, err)
			}
			logger.Printf("cloud volume %s: LUN %d -> device %s", volName, lunIdx, device)

			hostMountPoint := filepath.Join(cloudVolumeMountBase, safeName)
			if err := os.MkdirAll(hostMountPoint, 0o755); err != nil {
				return nil, fmt.Errorf("creating mount point for %s: %w", volName, err)
			}

			if err := waitForDevice(device); err != nil {
				return nil, fmt.Errorf("cloud volume %s device %s not available: %w", volName, device, err)
			}

			if volInfo.EncryptType != "" {
				mapperName := "caa-" + safeName
				if err := secureMount(ctx, device, hostMountPoint, fsType, volInfo.EncryptType, volInfo.KeyID, mapperName); err != nil {
					return nil, fmt.Errorf("failed to secure-mount cloud volume %s at %s: %w", volName, hostMountPoint, err)
				}
				i.cloudMounts = append(i.cloudMounts, cloudMount{path: hostMountPoint, encrypted: true, mapperName: mapperName})
			} else {
				if err := formatAndMount(device, hostMountPoint, fsType); err != nil {
					return nil, fmt.Errorf("failed to mount cloud volume %s at %s: %w", volName, hostMountPoint, err)
				}
				i.cloudMounts = append(i.cloudMounts, cloudMount{path: hostMountPoint, encrypted: false})
			}

			if fsGroupStr := volInfo.FSGroup; fsGroupStr != "" {
				if gid, err := strconv.Atoi(fsGroupStr); err == nil {
					logger.Printf("cloud volume %s: applying fsGroup %d to %s", volName, gid, hostMountPoint)
					if err := os.Chown(hostMountPoint, -1, gid); err != nil {
						logger.Printf("WARNING: failed to chown %s to gid %d: %v", hostMountPoint, gid, err)
					}
					if err := os.Chmod(hostMountPoint, 0o2775); err != nil {
						logger.Printf("WARNING: failed to chmod %s: %v", hostMountPoint, err)
					}
				}
			}

			rewrote := false
			for idx, m := range req.OCI.Mounts {
				if m.Destination == mountPoint {
					req.OCI.Mounts[idx].Source = hostMountPoint
					req.OCI.Mounts[idx].Type = "bind"
					logger.Printf("cloud volume %s: rewrote mount source to %s", volName, hostMountPoint)
					rewrote = true
					break
				}
			}
			if !rewrote {
				logger.Printf("WARNING: cloud volume %s mount_point %q not found in container mounts", volName, mountPoint)
			}
		}
	}

	volumeTargetPath := req.OCI.Annotations[volumeTargetPathKey]
	volumeTargetPathSlice := strings.Split(volumeTargetPath, ",")
	if len(req.OCI.Mounts) > 0 {
		for _, m := range req.OCI.Mounts {
			if _, err := os.Stat(m.Source); os.IsNotExist(err) && m.Type == "bind" {
				logger.Printf("mount source %s doesn't exist, try to create", m.Source)
				if err = os.MkdirAll(m.Source, os.ModePerm); err != nil {
					logger.Printf("Failed to create dir: %v", err)
				}
			}
			for _, s := range volumeTargetPathSlice {
				if isTargetPath(m.Source, strings.TrimSpace(s)) {
					logger.Printf("Waiting for device mounted to: %s", m.Source)
					err := waitForDeviceMounted(ctx, m.Source)
					if err != nil {
						return nil, err
					}
				}
			}
		}
	}

	res, err := i.Redirector.CreateContainer(ctx, req)

	if err != nil {
		logger.Printf("CreateContainer failed with error: %v", err)
	}

	return res, err
}

func isTargetPath(path, targetPath string) bool {
	return targetPath != "" && targetPath == path
}

func waitForDeviceMounted(ctx context.Context, path string) error {

	ctx, cancel := context.WithTimeout(ctx, volumeCheckTimeout)
	defer cancel()

	err := retry.Do(
		func() error {
			isMounted, err := mountinfo.Mounted(path)
			if err != nil {
				logger.Printf("Mounted check error: %v", err)
				return err
			}

			if isMounted {
				logger.Printf("Device has been mounted to %s", path)
				return nil
			} else {
				err = fmt.Errorf("Device has not been mounted to %s", path)
				logger.Print(err)
				return err
			}
		},
		retry.Attempts(0),
		retry.Context(ctx),
		retry.MaxDelay(volumeCheckInterval),
	)

	if err != nil {
		err = fmt.Errorf("Timeout waiting for device to mount to %s: %w", path, err)
		logger.Print(err)
		return err
	}

	return nil

}

func (i *interceptor) StartContainer(ctx context.Context, req *pb.StartContainerRequest) (*emptypb.Empty, error) {

	logger.Printf("StartContainer: containerID:%s", req.ContainerId)

	res, err := i.Redirector.StartContainer(ctx, req)

	if err != nil {
		logger.Printf("StartContainer failed with error: %v", err)
	}

	return res, err
}

func (i *interceptor) RemoveContainer(ctx context.Context, req *pb.RemoveContainerRequest) (*emptypb.Empty, error) {

	logger.Printf("RemoveContainer: containerID:%s", req.ContainerId)

	res, err := i.Redirector.RemoveContainer(ctx, req)

	if err != nil {
		logger.Printf("RemoveContainer failed with error: %v", err)
	}
	return res, err
}

func (i *interceptor) CreateSandbox(ctx context.Context, req *pb.CreateSandboxRequest) (*emptypb.Empty, error) {

	logger.Printf("CreateSandbox: hostname:%s sandboxId:%s", req.Hostname, req.SandboxId)

	if len(req.Dns) > 0 {
		logger.Print("    dns:")
		for _, d := range req.Dns {
			logger.Printf("        %s", d)
		}

		logger.Print("      Eliminated the DNS setting above from CreateSandboxRequest to stop updating /etc/resolv.conf on the peer pod VM")
		logger.Print("      See https://github.com/confidential-containers/cloud-api-adaptor/issues/98 for the details.")
		logger.Println()
		req.Dns = nil
	}

	res, err := i.Redirector.CreateSandbox(ctx, req)

	if err != nil {
		logger.Printf("CreateSandbox failed with error: %v", err)
	}

	return res, err
}

func (i *interceptor) DestroySandbox(ctx context.Context, req *pb.DestroySandboxRequest) (*emptypb.Empty, error) {

	logger.Printf("DestroySandbox")

	i.unmountCloudVolumes()

	res, err := i.Redirector.DestroySandbox(ctx, req)

	if err != nil {
		logger.Printf("DestroySandbox failed with error: %v", err)
	}

	return res, err
}

// detectCloudProvider determines the cloud platform by probing
// provider-specific device paths inside the PodVM.
func detectCloudProvider() string {
	if _, err := os.Stat("/dev/disk/azure"); err == nil {
		return "azure"
	}
	// Azure VMs have a well-known DMI chassis asset tag. This is more
	// specific than checking /sys/bus/vmbus which exists on all Hyper-V
	// VMs (including on-prem and Azure Stack).
	if data, err := os.ReadFile("/sys/class/dmi/id/chassis_asset_tag"); err == nil {
		if strings.TrimSpace(string(data)) == "7783-7084-3265-9085-8269-3286-77" {
			return "azure"
		}
	}
	if matches, _ := filepath.Glob("/dev/disk/by-id/nvme-Amazon_Elastic_Block_Store_*"); len(matches) > 0 {
		return "aws"
	}
	if matches, _ := filepath.Glob("/dev/vd[a-z]"); len(matches) > 0 {
		return "libvirt"
	}
	return "generic"
}

// findDataDiskDevice locates the block device for the given LUN index.
// It auto-detects the cloud provider and uses provider-specific paths,
// then falls back to sysfs HCTL-based LUN matching.
// diskID is the cloud-provider volume identifier (e.g. EBS vol-xxx); it
// may be empty for providers that rely purely on LUN-index matching.
//
// Data disks may take several seconds to appear in the PodVM after boot
// (SCSI rescan, udev rules), so the entire detection is retried.
func findDataDiskDevice(lunIdx int, diskID string) (string, error) {
	provider := detectCloudProvider()
	logger.Printf("Cloud provider detected: %s (LUN %d, diskID %s)", provider, lunIdx, diskID)

	maxAttempts := 15
	retryDelay := 2 * time.Second
	if v := os.Getenv("CAA_DISK_DETECT_MAX_ATTEMPTS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			maxAttempts = n
		}
	}
	if v := os.Getenv("CAA_DISK_DETECT_RETRY_DELAY"); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			retryDelay = d
		}
	}

	if provider == "azure" {
		triggerUdevRescan()
	}

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if attempt > 1 {
			logger.Printf("Retry %d/%d for LUN %d...", attempt, maxAttempts, lunIdx)
			time.Sleep(retryDelay)
		}

		switch provider {
		case "azure":
			if dev, err := findAzureDataDisk(lunIdx); err == nil {
				return dev, nil
			}
		case "aws":
			if dev, err := findAWSDataDisk(lunIdx, diskID); err == nil {
				return dev, nil
			}
		case "libvirt":
			if dev, err := findLibvirtDataDisk(lunIdx); err == nil {
				return dev, nil
			}
		}

		if dev, err := findDataDiskBySysfsHCTL(lunIdx); err == nil {
			return dev, nil
		}
	}

	dumpBlockDeviceDiagnostics()
	return "", fmt.Errorf("no data disk found for LUN %d (provider=%s) after %d attempts", lunIdx, provider, maxAttempts)
}

// triggerUdevRescan triggers udev to process pending block device events and
// create symlinks (e.g. /dev/disk/azure/*, /dev/disk/by-path/*).
// Called once before the retry loop rather than on every attempt.
func triggerUdevRescan() {
	if _, err := exec.Command("udevadm", "trigger", "--subsystem-match=block").CombinedOutput(); err == nil {
		exec.Command("udevadm", "settle", "--timeout=5").CombinedOutput() //nolint:errcheck
	}
}

func findAzureDataDisk(lunIdx int) (string, error) {
	azurePaths := []string{
		fmt.Sprintf("/dev/disk/azure/data/by-lun/%d", lunIdx),
		fmt.Sprintf("/dev/disk/azure/scsi1/lun%d", lunIdx),
	}
	for _, p := range azurePaths {
		if target, err := filepath.EvalSymlinks(p); err == nil {
			logger.Printf("Found Azure data disk: %s -> %s", p, target)
			return target, nil
		}
	}

	byPathDir := "/dev/disk/by-path"
	if entries, err := os.ReadDir(byPathDir); err == nil {
		lunSuffix := fmt.Sprintf("-lun-%d", lunIdx)
		for _, e := range entries {
			name := e.Name()
			if strings.Contains(name, "vmbus") && strings.HasSuffix(name, lunSuffix) && !strings.Contains(name, "part") {
				fullPath := filepath.Join(byPathDir, name)
				if target, err := filepath.EvalSymlinks(fullPath); err == nil {
					if isOnSCSIHost0(filepath.Base(target)) {
						logger.Printf("Skipping by-path %s -> %s (SCSI host 0, OS disk area)", fullPath, target)
						continue
					}
					logger.Printf("Found Azure data disk via by-path: %s -> %s", fullPath, target)
					return target, nil
				}
			}
		}
	}
	return "", fmt.Errorf("Azure data disk LUN %d not found", lunIdx)
}

func findAWSDataDisk(lunIdx int, diskID string) (string, error) {
	entries, err := filepath.Glob("/dev/disk/by-id/nvme-Amazon_Elastic_Block_Store_*")
	if err != nil || len(entries) == 0 {
		return "", fmt.Errorf("no AWS EBS NVMe devices found")
	}

	var candidates []string
	for _, e := range entries {
		if !strings.Contains(e, "-part") {
			candidates = append(candidates, e)
		}
	}

	// Prefer matching by EBS volume ID for multi-disk correctness.
	// The NVMe by-id symlink encodes the volume ID with hyphens removed,
	// e.g. vol-0abc123 becomes nvme-Amazon_Elastic_Block_Store_vol0abc123.
	if diskID != "" {
		volIDNormalized := strings.ReplaceAll(diskID, "-", "")
		for _, c := range candidates {
			if strings.Contains(c, volIDNormalized) {
				target, err := filepath.EvalSymlinks(c)
				if err != nil {
					return "", err
				}
				logger.Printf("Found AWS EBS disk by volume ID: %s -> %s (diskID=%s)", c, target, diskID)
				return target, nil
			}
		}
		return "", fmt.Errorf("AWS EBS disk with volume ID %s not found in %d by-id symlinks", diskID, len(candidates))
	}

	// Index-based selection only when diskID is empty (legacy/testing).
	if lunIdx >= len(candidates) {
		return "", fmt.Errorf("AWS EBS disk index %d out of range (have %d)", lunIdx, len(candidates))
	}
	target, err := filepath.EvalSymlinks(candidates[lunIdx])
	if err != nil {
		return "", err
	}
	logger.Printf("Found AWS EBS disk by index: %s -> %s", candidates[lunIdx], target)
	return target, nil
}

func findLibvirtDataDisk(lunIdx int) (string, error) {
	if lunIdx < 0 || lunIdx > 24 {
		return "", fmt.Errorf("LUN index %d out of range for virtio devices (0-24)", lunIdx)
	}
	devLetter := 'b' + rune(lunIdx)
	device := fmt.Sprintf("/dev/vd%c", devLetter)
	if _, err := os.Stat(device); err == nil {
		logger.Printf("Found libvirt virtio disk: %s", device)
		return device, nil
	}
	return "", fmt.Errorf("libvirt device %s not found", device)
}

// isOnSCSIHost0 checks if a block device is on SCSI host controller 0,
// where Azure/Hyper-V places the OS and temp disks.
func isOnSCSIHost0(devName string) bool {
	devicePath := filepath.Join("/sys/block", devName, "device")
	realPath, err := filepath.EvalSymlinks(devicePath)
	if err != nil {
		return false
	}
	for _, part := range strings.Split(realPath, "/") {
		hctl := strings.Split(part, ":")
		if len(hctl) == 4 {
			host, err := strconv.Atoi(hctl[0])
			if err == nil {
				return host == 0
			}
		}
	}
	return false
}

// findDataDiskBySysfsHCTL matches block devices by their SCSI HCTL
// (Host:Channel:Target:Lun) address in sysfs rather than relying on
// directory listing order. On Azure/Hyper-V, SCSI controller 0 holds
// the OS and temp disks while data disks live on controller 1+, so
// controller 0 is skipped. If no match is found on non-zero hosts,
// a second pass looks at all hosts but filters out any device that
// is mounted or has partitions (indicating it is the OS/temp disk).
func findDataDiskBySysfsHCTL(lunIdx int) (string, error) {
	entries, err := os.ReadDir("/sys/block")
	if err != nil {
		return "", fmt.Errorf("cannot read /sys/block: %w", err)
	}

	type devInfo struct {
		name string
		host int
		lun  int
		hctl string
	}

	var allMatches []devInfo

	for _, e := range entries {
		name := e.Name()
		if !strings.HasPrefix(name, "sd") && !strings.HasPrefix(name, "nvme") && !strings.HasPrefix(name, "vd") {
			continue
		}

		devicePath := filepath.Join("/sys/block", name, "device")
		realPath, err := filepath.EvalSymlinks(devicePath)
		if err != nil {
			continue
		}

		parts := strings.Split(realPath, "/")
		for _, part := range parts {
			hctl := strings.Split(part, ":")
			if len(hctl) == 4 {
				host, hostErr := strconv.Atoi(hctl[0])
				lun, lunErr := strconv.Atoi(hctl[3])
				if hostErr != nil || lunErr != nil {
					continue
				}
				logger.Printf("sysfs scan: /dev/%s HCTL=%s (host=%d lun=%d)", name, part, host, lun)
				if lun == lunIdx {
					allMatches = append(allMatches, devInfo{name: name, host: host, lun: lun, hctl: part})
				}
			}
		}
	}

	// Pass 1: prefer devices on non-zero hosts (standard Azure data disk topology)
	for _, m := range allMatches {
		if m.host == 0 {
			continue
		}
		dev := "/dev/" + m.name
		if isRootOrMountedDevice(m.name) {
			logger.Printf("Skipping %s (LUN %d host %d): device is in use as root/mounted", dev, m.lun, m.host)
			continue
		}
		logger.Printf("Found data disk via sysfs HCTL (host>0): %s (LUN %d from %s)", dev, lunIdx, m.hctl)
		return dev, nil
	}

	// Pass 2: if no non-zero host matched, check host 0 devices that are
	// NOT mounted and have NO partitions (i.e. raw data disks on host 0).
	for _, m := range allMatches {
		if m.host != 0 {
			continue
		}
		dev := "/dev/" + m.name
		if isRootOrMountedDevice(m.name) {
			logger.Printf("Skipping %s (LUN %d host 0): device is mounted", dev, m.lun)
			continue
		}
		if hasPartitions(m.name) {
			logger.Printf("Skipping %s (LUN %d host 0): device has partitions (likely OS/temp disk)", dev, m.lun)
			continue
		}
		logger.Printf("Found data disk via sysfs HCTL (host=0, raw): %s (LUN %d from %s)", dev, lunIdx, m.hctl)
		return dev, nil
	}

	return "", fmt.Errorf("no data disk found for LUN %d via sysfs HCTL (%d candidates scanned)", lunIdx, len(allMatches))
}

// hasPartitions checks whether a block device has any partition sub-devices
// in /sys/block/<dev>/<dev>N (e.g. sda1, sda2).
func hasPartitions(devName string) bool {
	entries, err := os.ReadDir(filepath.Join("/sys/block", devName))
	if err != nil {
		return false
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), devName) {
			return true
		}
	}
	return false
}

// isRootOrMountedDevice returns true if the device or any of its partitions
// is mounted as a filesystem (especially / or /boot). This prevents
// accidentally selecting the OS disk as a data disk when HCTL host
// numbering doesn't match expectations.
func isRootOrMountedDevice(devName string) bool {
	data, err := os.ReadFile("/proc/mounts")
	if err != nil {
		return false
	}
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		mountDev := fields[0]
		if strings.HasPrefix(mountDev, "/dev/"+devName) {
			return true
		}
	}
	return false
}

func dumpBlockDeviceDiagnostics() {
	logger.Printf("=== Block Device Diagnostics ===")
	if out, err := exec.Command("lsblk", "-o", "NAME,SIZE,TYPE,MOUNTPOINT,FSTYPE").CombinedOutput(); err == nil {
		logger.Printf("lsblk:\n%s", string(out))
	}
	for _, dir := range []string{"/dev/disk/azure", "/dev/disk/by-path", "/dev/disk/by-id"} {
		if entries, err := os.ReadDir(dir); err == nil {
			for _, e := range entries {
				full := filepath.Join(dir, e.Name())
				if target, err := os.Readlink(full); err == nil {
					logger.Printf("  %s -> %s", full, target)
				}
			}
		}
	}
}

func waitForDevice(device string) error {
	for attempt := 0; attempt < 30; attempt++ {
		if strings.HasPrefix(device, "/dev/disk/") {
			if _, err := os.Lstat(device); err == nil {
				return nil
			}
		} else {
			devName := filepath.Base(device)
			if _, err := os.Stat(filepath.Join("/sys/block", devName)); err == nil {
				return nil
			}
		}
		time.Sleep(2 * time.Second)
	}
	return fmt.Errorf("device %s not available after 60s", device)
}

func formatAndMount(device, mountPoint, fsType string) error {
	if fsType == "" {
		fsType = "ext4"
	}
	if !allowedFSTypes[fsType] {
		return fmt.Errorf("unsupported filesystem type %q", fsType)
	}

	if alreadyMounted, err := mountinfo.Mounted(mountPoint); err == nil && alreadyMounted {
		logger.Printf("Mount point %s is already mounted, treating as success", mountPoint)
		return nil
	}

	// Try mounting first — if the disk already has a valid filesystem, this
	// avoids any risk of accidentally reformatting it.
	mountCmd := exec.Command("mount", "-t", fsType, device, mountPoint)
	if mountOut, mountErr := mountCmd.CombinedOutput(); mountErr == nil {
		logger.Printf("Mounted existing filesystem on %s at %s (type=%s)", device, mountPoint, fsType)
		return nil
	} else {
		logger.Printf("Initial mount of %s failed (expected for new disks): %s", device, strings.TrimSpace(string(mountOut)))
	}

	// Mount failed — check if there's any filesystem using blkid with retries.
	needsFormat := false
	for attempt := 0; attempt < 3; attempt++ {
		out, err := exec.Command("blkid", "-p", device).CombinedOutput()
		outStr := strings.TrimSpace(string(out))
		if err == nil && len(outStr) > 0 {
			autoMount := exec.Command("mount", device, mountPoint)
			if autoOut, autoErr := autoMount.CombinedOutput(); autoErr == nil {
				logger.Printf("Mounted %s at %s (auto-detected type from: %s)", device, mountPoint, outStr)
				return nil
			} else {
				return fmt.Errorf("device %s has filesystem signature (%s) but mount failed: %s", device, outStr, strings.TrimSpace(string(autoOut)))
			}
		}
		if err != nil {
			// Exit code 2 = no valid filesystem found (safe to format)
			if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 2 {
				needsFormat = true
				break
			}
			logger.Printf("blkid attempt %d for %s failed: %v (output: %s)", attempt+1, device, err, outStr)
			time.Sleep(2 * time.Second)
			continue
		}
		// err == nil but empty output — ambiguous state, retry
		logger.Printf("blkid returned empty output for %s (attempt %d), retrying...", device, attempt+1)
		time.Sleep(2 * time.Second)
		continue
	}

	if !needsFormat {
		return fmt.Errorf("cannot determine filesystem state of %s after retries; refusing to format to protect data", device)
	}

	logger.Printf("No filesystem on %s, formatting as %s", device, fsType)
	mkfsCmd := exec.Command("mkfs."+fsType, device)
	if mkfsOut, mkfsErr := mkfsCmd.CombinedOutput(); mkfsErr != nil {
		return fmt.Errorf("mkfs.%s on %s failed: %s: %w", fsType, device, strings.TrimSpace(string(mkfsOut)), mkfsErr)
	}
	logger.Printf("Formatted %s as %s", device, fsType)

	mountCmd2 := exec.Command("mount", "-t", fsType, device, mountPoint)
	if mountOut, mountErr := mountCmd2.CombinedOutput(); mountErr != nil {
		return fmt.Errorf("mount after format %s -> %s failed: %s: %w", device, mountPoint, strings.TrimSpace(string(mountOut)), mountErr)
	}
	logger.Printf("Mounted %s at %s (type=%s)", device, mountPoint, fsType)
	return nil
}

// isLuks checks if a device already has a LUKS header.
func isLuks(device string) bool {
	err := exec.Command("cryptsetup", "isLuks", device).Run()
	return err == nil
}

// validateEncryptParams checks that the encryption parameters are valid
// before attempting to connect to CDH.
func validateEncryptParams(encryptType, keyID string) (string, error) {
	if keyID == "" {
		return "", fmt.Errorf("encrypt_type %q requires a kbs-key-id but none was provided", encryptType)
	}
	normalized := strings.ToLower(encryptType)
	if !allowedEncryptTypes[normalized] {
		return "", fmt.Errorf("unsupported encrypt_type %q (allowed: luks, luks2)", encryptType)
	}
	return normalized, nil
}

// secureMount delegates block device encryption and mounting to CDH's
// secure_mount API. CDH handles LUKS formatting (for new disks) or opening
// (for existing encrypted disks), fetches the key from KBS via remote
// attestation, and mounts the plaintext filesystem at mountPoint.
// The mapperName is passed to CDH so it uses a predictable dm-crypt
// device name under /dev/mapper/, enabling reliable cleanup.
func secureMount(ctx context.Context, device, mountPoint, fsType, encryptType, keyID, mapperName string) error {
	normalized, err := validateEncryptParams(encryptType, keyID)
	if err != nil {
		return err
	}

	sourceType := "empty"
	if isLuks(device) {
		sourceType = "encrypted"
	}

	logger.Printf("secureMount: device=%s mountPoint=%s sourceType=%s encryptType=%s mapperName=%s",
		device, mountPoint, sourceType, normalized, mapperName)

	client, err := newCDHClient(cdhSocketPath)
	if err != nil {
		return fmt.Errorf("connecting to CDH at %s: %w", cdhSocketPath, err)
	}
	defer client.close()

	options := map[string]string{
		"devicePath":     device,
		"sourceType":     sourceType,
		"targetType":     "fileSystem",
		"encryptionType": normalized,
		"filesystemType": fsType,
		"key":            "kbs:///" + keyID,
	}
	if mapperName != "" {
		options["mapperName"] = mapperName
	}

	mountPath, err := client.secureMount(ctx, "block-device", options, []string{}, mountPoint)
	if err != nil {
		return fmt.Errorf("CDH secure_mount failed for %s: %w", device, err)
	}

	logger.Printf("secureMount: CDH mounted %s at %s", device, mountPath)
	return nil
}
