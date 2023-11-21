// Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package proxy

import (
	"context"
	b64 "encoding/base64"
	"net"
	"os"
	"path/filepath"
	"strings"

	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/pkg/util/agentproto"
	"github.com/gogo/protobuf/types"
	pb "github.com/kata-containers/kata-containers/src/runtime/virtcontainers/pkg/agent/protocols/grpc"
)

type proxyService struct {
	agentproto.Redirector
	criClient  *criClient
	pauseImage string
}

const (
	defaultPauseImage            = "registry.k8s.io/pause:3.7"
	kataDirectVolumesDir         = "/run/kata-containers/shared/direct-volumes"
	volumeTargetPathKey          = "io.confidentialcontainers.org.peerpodvolumes.target_path"
	csiPluginEscapeQualifiedName = "kubernetes.io~csi"
	imageGuestPull               = "image_guest_pull"
)

func newProxyService(dialer func(context.Context) (net.Conn, error), criClient *criClient, pauseImage string) *proxyService {

	redirector := agentproto.NewRedirector(dialer)

	return &proxyService{
		Redirector: redirector,
		criClient:  criClient,
		pauseImage: pauseImage,
	}
}

// AgentServiceService methods

func (s *proxyService) CreateContainer(ctx context.Context, req *pb.CreateContainerRequest) (*types.Empty, error) {
	var pullImageInGuest bool
	logger.Printf("CreateContainer: containerID:%s", req.ContainerId)
	if len(req.OCI.Mounts) > 0 {
		logger.Print("    mounts:")
		for i, m := range req.OCI.Mounts {
			logger.Printf("        destination:%s source:%s type:%s", m.Destination, m.Source, m.Type)

			if isNodePublishVolumeTargetPath(m.Source, kataDirectVolumesDir) {
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

	if !pullImageInGuest {
		logger.Printf("Pulling image separately not support on main")
	}

	res, err := s.Redirector.CreateContainer(ctx, req)

	if err != nil {
		logger.Printf("CreateContainer fails: %v", err)
	}

	return res, err
}

func isNodePublishVolumeTargetPath(volumePath, directVolumesDir string) bool {
	if !strings.Contains(filepath.Clean(volumePath), "/volumes/"+csiPluginEscapeQualifiedName+"/") {
		return false
	}

	volumeDir := filepath.Join(directVolumesDir, b64.URLEncoding.EncodeToString([]byte(volumePath)))
	_, err := os.Stat(volumeDir)

	return err == nil
}

func (s *proxyService) SetPolicy(ctx context.Context, req *pb.SetPolicyRequest) (*types.Empty, error) {

	logger.Printf("SetPolicy: policy:%s", req.Policy)

	res, err := s.Redirector.SetPolicy(ctx, req)

	if err != nil {
		logger.Printf("SetPolicy fails: %v", err)
	}

	return res, err
}

func (s *proxyService) StartContainer(ctx context.Context, req *pb.StartContainerRequest) (*types.Empty, error) {

	logger.Printf("StartContainer: containerID:%s", req.ContainerId)

	res, err := s.Redirector.StartContainer(ctx, req)

	if err != nil {
		logger.Printf("StartContainer fails: %v", err)
	}

	return res, err
}

func (s *proxyService) RemoveContainer(ctx context.Context, req *pb.RemoveContainerRequest) (*types.Empty, error) {

	logger.Printf("RemoveContainer: containerID:%s", req.ContainerId)

	res, err := s.Redirector.RemoveContainer(ctx, req)

	if err != nil {
		logger.Printf("RemoveContainer fails: %v", err)
	}

	return res, err
}

func (s *proxyService) CreateSandbox(ctx context.Context, req *pb.CreateSandboxRequest) (*types.Empty, error) {

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

func (s *proxyService) DestroySandbox(ctx context.Context, req *pb.DestroySandboxRequest) (*types.Empty, error) {

	logger.Printf("DestroySandbox")

	res, err := s.Redirector.DestroySandbox(ctx, req)

	if err != nil {
		logger.Printf("DestroySandbox fails: %v", err)
	}

	return res, err
}
