// Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package proxy

import (
	"context"
	b64 "encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"

	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/pkg/util"
	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/pkg/util/agentproto"
	pb "github.com/kata-containers/kata-containers/src/runtime/virtcontainers/pkg/agent/protocols/grpc"
	"github.com/kata-containers/kata-containers/src/runtime/virtcontainers/types"
	"google.golang.org/protobuf/types/known/emptypb"
)

type proxyService struct {
	agentproto.Redirector
	pauseImage string
}

const (
	defaultPauseImage     = "registry.k8s.io/pause:3.7"
	volumeTargetPathKey   = "io.confidentialcontainers.org.peerpodvolumes.target_path"
	imageGuestPull        = "image_guest_pull"
	cdiAnnotationKey      = "cdi.k8s.io/peer-pods"
	defaultCDIType        = "nvidia.com/gpu=all"
	defaultGPUsAnnotation = "io.katacontainers.config.hypervisor.default_gpus"
)

func newProxyService(dialer func(context.Context) (net.Conn, error), pauseImage string) *proxyService {

	redirector := agentproto.NewRedirector(dialer)

	return &proxyService{
		Redirector: redirector,
		pauseImage: pauseImage,
	}
}

// AgentServiceService methods

func (s *proxyService) CreateContainer(ctx context.Context, req *pb.CreateContainerRequest) (*emptypb.Empty, error) {
	var pullImageInGuest bool
	logger.Printf("CreateContainer: containerID:%s", req.ContainerId)
	if req.OCI.Annotations == nil {
		req.OCI.Annotations = make(map[string]string)
	}

	if len(req.OCI.Mounts) > 0 {
		logger.Print("    mounts:")
		for i, m := range req.OCI.Mounts {
			logger.Printf("        destination:%s source:%s type:%s", m.Destination, m.Source, m.Type)

			if isNodePublishVolumeTargetPath(m.Source, util.KataDirectVolumesDir) {
				if i > 0 {
					req.OCI.Annotations[volumeTargetPathKey] += ","
				}
				req.OCI.Annotations[volumeTargetPathKey] += m.Source
			}
		}
	}
	if len(req.OCI.Annotations) > 0 {
		logger.Print("    annotations:")
		for k, v := range req.OCI.Annotations {
			logger.Printf("        %s: %s", k, v)
		}
	}

	if len(req.Storages) > 0 {
		logger.Print("    storages:")
		for _, s := range req.Storages {
			logger.Printf("        mount_point:%s source:%s fstype:%s driver:%s", s.MountPoint, s.Source, s.Fstype, s.Driver)
			// remote-snapshotter in contanerd appends image_guest_pull drivers for image layer will be pulled in guest.
			// Image will be pull in guest via image-rs according to the driver info.
			if s.Driver == imageGuestPull {
				pullImageInGuest = true
			}
		}
	}
	if len(req.Devices) > 0 {
		logger.Print("    devices:")
		for _, d := range req.Devices {
			logger.Printf("        container_path:%s vm_path:%s type:%s", d.ContainerPath, d.VmPath, d.Type)
		}
	}

	if req.OCI.Annotations != nil && req.OCI.Annotations[defaultGPUsAnnotation] != "" {
		req.OCI.Annotations[cdiAnnotationKey] = defaultCDIType
		logger.Printf("adding CDI annotation %s: %s", cdiAnnotationKey, defaultCDIType)
	}

	if !pullImageInGuest {
		// There is some issue with nydus(error unpacking image) when the image layers are missing due to
		//  - discard_unpacked_layers set to true
		//  - other reasons we don't know yet
		// Run: ctr -n k8s.io image check
		// to see whether the image is complete(all layers are present)
		//
		// nydus adds one mount that carries the image information which is then picked up
		// by kata shim, then kata shim passes it to kata agent in the PodVM. Without nydus, we
		// have to add the mount point manually.
		vol, err := handleVirtualVolumeStorageObject(req)
		if err != nil {
			return nil, err
		}

		req.Storages = append(req.Storages, vol)
		storage := req.Storages[len(req.Storages)-1]
		logger.Print("    storages added for guest_image_pull:")
		logger.Printf("        mount_point:%s source:%s fstype:%s driver:%s", storage.MountPoint, storage.Source, storage.Fstype, storage.Driver)
	}

	// Detect cloud volumes by scanning the direct-volumes directory in canonical
	// order. The scan order matches GetCSIVolumesForPod (os.ReadDir sorts by
	// name), so the LUN index here is consistent with the cloud provider's
	// disk attachment order.
	cloudVolumes := make(map[string]util.CloudVolumeAnnotation)
	podUID := req.OCI.Annotations["io.kubernetes.cri.sandbox-uid"]

	dirEntries, dirErr := os.ReadDir(util.KataDirectVolumesDir)
	if dirErr == nil {
		canonicalIdx := 0
		for _, entry := range dirEntries {
			if !entry.IsDir() {
				continue
			}
			decodedBytes, err := b64.URLEncoding.DecodeString(entry.Name())
			if err != nil {
				continue
			}
			decodedPath := string(decodedBytes)

			if !strings.Contains(decodedPath, "/volumes/"+util.CSIPluginEscapeQualifiedName+"/") {
				continue
			}
			if podUID != "" && !strings.Contains(decodedPath, "/pods/"+podUID+"/") {
				continue
			}

			mountInfoPath := filepath.Join(util.KataDirectVolumesDir, entry.Name(), "mountInfo.json")
			data, err := os.ReadFile(mountInfoPath)
			if err != nil {
				logger.Printf("could not read mountInfo.json for %s: %v", decodedPath, err)
				continue
			}
			var mountInfo map[string]interface{}
			if err := json.Unmarshal(data, &mountInfo); err != nil {
				logger.Printf("could not parse mountInfo.json for %s: %v", decodedPath, err)
				continue
			}

			diskID := ""
			if md, ok := mountInfo["metadata"].(map[string]interface{}); ok {
				if cp, ok := md["cloud-volume-path"].(string); ok && cp != "" {
					diskID = cp
				}
			}
			if diskID == "" {
				if d, ok := mountInfo["device"].(string); ok {
					diskID = d
				}
			}
			if diskID == "" {
				logger.Printf("cloud volume at %s has no disk ID, skipping", decodedPath)
				continue
			}

			fsType := "ext4"
			if ft, ok := mountInfo["fstype"].(string); ok && ft != "" {
				fsType = ft
			}

			mountDest := ""
			for _, m := range req.OCI.Mounts {
				if m.Source == decodedPath {
					mountDest = m.Destination
					break
				}
			}
			if mountDest == "" {
				logger.Printf("cloud volume disk %s has no matching mount in container spec, skipping", diskID)
				canonicalIdx++
				continue
			}

			volKey := fmt.Sprintf("vol-%d", canonicalIdx)
			cloudVolumes[volKey] = util.CloudVolumeAnnotation{
				MountPoint: mountDest,
				FSType:     fsType,
				LUN:        fmt.Sprintf("%d", canonicalIdx),
				DiskID:     diskID,
			}
			logger.Printf("Detected cloud volume %s -> %s (lun=%d, disk=%s, fs=%s)", volKey, mountDest, canonicalIdx, diskID, fsType)
			canonicalIdx++
		}
	}

	if len(cloudVolumes) > 0 {
		cvJSON, err := json.Marshal(cloudVolumes)
		if err != nil {
			logger.Printf("failed to marshal cloud_volumes annotation: %v", err)
		} else {
			req.OCI.Annotations[util.CloudVolumesAnnotationKey] = string(cvJSON)
			logger.Printf("Set cloud_volumes annotation: %s", string(cvJSON))
		}
	}

	res, err := s.Redirector.CreateContainer(ctx, req)

	if err != nil {
		logger.Printf("CreateContainer fails: %v", err)
	}

	return res, err
}

// The following fucntions are originally from https://github.com/kata-containers/kata-containers/blob/main/src/runtime/virtcontainers/kata_agent.go
//   - handleVirtualVolumeStorageObject
//   - handleImageGuestPullBlockVolume
//   - getContainerTypeforCRI
//
// Modified handleVirtualVolumeStorageObject
func handleVirtualVolumeStorageObject(req *pb.CreateContainerRequest) (*pb.Storage, error) {
	var vol *pb.Storage
	virtVolume := &types.KataVirtualVolume{
		VolumeType: types.KataVirtualVolumeImageGuestPullType,
		ImagePull: &types.ImagePullVolume{
			Metadata: map[string]string{},
		},
	}

	var err error
	vol = &pb.Storage{}
	vol, err = handleImageGuestPullBlockVolume(req.OCI.Annotations, virtVolume, vol)
	if err != nil {
		return nil, err
	}
	vol.MountPoint = filepath.Join("/run/kata-containers/", req.ContainerId, "rootfs")
	return vol, nil
}

// Modified handleImageGuestPullBlockVolume
func handleImageGuestPullBlockVolume(containerAnnotations map[string]string, virtualVolumeInfo *types.KataVirtualVolume, vol *pb.Storage) (*pb.Storage, error) {
	containerType, criContainerType := getContainerTypeforCRI(containerAnnotations)

	var imageRef string
	if containerType == "pod_sandbox" {
		imageRef = "pause"
	} else {
		const ctrContainerType = "io.kubernetes.cri.container-type"
		const crioContainerType = "io.kubernetes.cri-o.ContainerType"
		const kubernetesCRIImageName = "io.kubernetes.cri.image-name"
		const kubernetesCRIOImageName = "io.kubernetes.cri-o.ImageName"

		switch criContainerType {
		case ctrContainerType:
			imageRef = containerAnnotations[kubernetesCRIImageName]
		case crioContainerType:
			imageRef = containerAnnotations[kubernetesCRIOImageName]
		default:
			imageRef = containerAnnotations[kubernetesCRIImageName]
		}

		if imageRef == "" {
			return nil, fmt.Errorf("Failed to get image name from annotations")
		}
	}
	virtualVolumeInfo.Source = imageRef

	//merge virtualVolumeInfo.ImagePull.Metadata and container_annotations
	for k, v := range containerAnnotations {
		virtualVolumeInfo.ImagePull.Metadata[k] = v
	}

	imagePullBytes, err := json.Marshal(virtualVolumeInfo.ImagePull)
	if err != nil {
		return nil, err
	}
	vol.Driver = types.KataVirtualVolumeImageGuestPullType
	vol.DriverOptions = append(vol.DriverOptions, types.KataVirtualVolumeImageGuestPullType+"="+string(imagePullBytes))
	vol.Source = virtualVolumeInfo.Source
	vol.Fstype = "overlay"
	return vol, nil
}

// Modified getContainerTypeforCRI
func getContainerTypeforCRI(containerAnnotations map[string]string) (string, string) {
	CRIContainerTypeKeyList := []string{
		"io.kubernetes.cri.container-type",
		"io.kubernetes.cri-o.ContainerType",
	}

	containerType := containerAnnotations["io.katacontainers.pkg.oci.container_type"]
	for _, key := range CRIContainerTypeKeyList {
		_, ok := containerAnnotations[key]
		if ok {
			return containerType, key
		}
	}
	return "", ""
}

func isNodePublishVolumeTargetPath(volumePath, directVolumesDir string) bool {
	if !strings.Contains(filepath.Clean(volumePath), "/volumes/"+util.CSIPluginEscapeQualifiedName+"/") {
		return false
	}

	volumeDir := filepath.Join(directVolumesDir, b64.URLEncoding.EncodeToString([]byte(volumePath)))
	_, err := os.Stat(volumeDir)

	return err == nil
}

func (s *proxyService) SetPolicy(ctx context.Context, req *pb.SetPolicyRequest) (*emptypb.Empty, error) {

	logger.Printf("SetPolicy: policy:%s", req.Policy)

	res, err := s.Redirector.SetPolicy(ctx, req)

	if err != nil {
		logger.Printf("SetPolicy fails: %v", err)
	}

	return res, err
}

func (s *proxyService) StartContainer(ctx context.Context, req *pb.StartContainerRequest) (*emptypb.Empty, error) {

	logger.Printf("StartContainer: containerID:%s", req.ContainerId)

	res, err := s.Redirector.StartContainer(ctx, req)

	if err != nil {
		logger.Printf("StartContainer fails: %v", err)
	}

	return res, err
}

func (s *proxyService) RemoveContainer(ctx context.Context, req *pb.RemoveContainerRequest) (*emptypb.Empty, error) {

	logger.Printf("RemoveContainer: containerID:%s", req.ContainerId)

	res, err := s.Redirector.RemoveContainer(ctx, req)

	if err != nil {
		logger.Printf("RemoveContainer fails: %v", err)
	}

	return res, err
}

func (s *proxyService) CreateSandbox(ctx context.Context, req *pb.CreateSandboxRequest) (*emptypb.Empty, error) {

	logger.Printf("CreateSandbox: hostname:%s sandboxId:%s", req.Hostname, req.SandboxId)

	if len(req.Storages) > 0 {
		logger.Print("    storages:")
		for _, s := range req.Storages {
			logger.Printf("        mountpoint:%s source:%s fstype:%s driver:%s", s.MountPoint, s.Source, s.Fstype, s.Driver)
		}
	}

	res, err := s.Redirector.CreateSandbox(ctx, req)

	if err != nil {
		logger.Printf("CreateSandbox fails: %v", err)
	}

	return res, err
}

func (s *proxyService) DestroySandbox(ctx context.Context, req *pb.DestroySandboxRequest) (*emptypb.Empty, error) {

	logger.Printf("DestroySandbox")

	res, err := s.Redirector.DestroySandbox(ctx, req)

	if err != nil {
		logger.Printf("DestroySandbox fails: %v", err)
	}

	return res, err
}
