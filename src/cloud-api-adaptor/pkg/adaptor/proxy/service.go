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
		logger.Printf("Pulling image separately not support on main. It is required to use the nydus-snapshotter, which isn't configured properly here.")
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
