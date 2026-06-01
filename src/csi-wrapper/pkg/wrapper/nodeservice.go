// Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package wrapper

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strings"

	podvminfo "github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/proto/podvminfo"
	peerpodvolumeV1alpha1 "github.com/confidential-containers/cloud-api-adaptor/src/csi-wrapper/pkg/apis/peerpodvolume/v1alpha1"
	peerpodvolume "github.com/confidential-containers/cloud-api-adaptor/src/csi-wrapper/pkg/generated/peerpodvolume/clientset/versioned"
	"github.com/confidential-containers/cloud-api-adaptor/src/csi-wrapper/pkg/utils"
	"github.com/containerd/ttrpc"
	volume "github.com/kata-containers/kata-containers/src/runtime/pkg/direct-volume"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/golang/glog"
	"github.com/golang/protobuf/jsonpb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const (
	DefaultKubeletLibDir  = "/var/lib/kubelet"
	DefaultKubeletDataDir = "/var/data/kubelet"
	DefaultMountInfo      = "{\"Device\": \"/dev/zero\", \"fstype\": \"ext4\"}"
	// Block-mode mountInfo. volume-type=block tells kata-agent to
	// passthrough the device without mounting a filesystem; the workload
	// receives a raw block device at the bind-mount target.
	BlockMountInfo = "{\"volume-type\": \"block\", \"device\": \"/dev/zero\", \"fstype\": \"\"}"
)

type NodeService struct {
	TargetEndpoint          string
	Namespace               string
	PeerpodvolumeClient     *peerpodvolume.Clientset
	VMIDInformationEndpoint string
}

// Leverage Kata’s mechanism of direct block device assignment to prevent CSI mount source from replacing by Kata shim code
func addKataDirectVolume(volumePath string) {
	addKataDirectVolumeWithMode(volumePath, false)
}

// addKataDirectVolumeWithMode lets callers register a passthrough block
// volume by passing block=true, which sets MountInfo.volume-type=block
// (vs the default filesystem-style mountInfo).
func addKataDirectVolumeWithMode(volumePath string, block bool) {
	mountInfo := DefaultMountInfo
	if block {
		mountInfo = BlockMountInfo
	}
	err := volume.Add(volumePath, mountInfo)
	if err != nil {
		glog.Warningf("Failed to add kata direct volume: %v", err.Error())
	}
}

func removeKataDirectVolume(volumePath string) {
	err := volume.Remove(volumePath)
	if err != nil {
		glog.Warningf("Failed to remove kata direct volume: %v", err.Error())
	}
}

func NewNodeService(targetEndpoint, namespace string, peerpodvolumeClientSet *peerpodvolume.Clientset, vmIDInformationEndpoint string) *NodeService {
	addKataDirectVolume(DefaultKubeletLibDir)
	addKataDirectVolume(DefaultKubeletDataDir)

	return &NodeService{
		Namespace:               namespace,
		TargetEndpoint:          fmt.Sprintf("unix://%s", targetEndpoint),
		PeerpodvolumeClient:     peerpodvolumeClientSet,
		VMIDInformationEndpoint: vmIDInformationEndpoint,
	}
}

func (s *NodeService) redirect(ctx context.Context, req interface{}, fn func(context.Context, csi.NodeClient)) error {
	// grpc.Dial is deprecated and supported only with grpc 1.x
	//nolint:staticcheck
	conn, err := grpc.Dial(s.TargetEndpoint, grpc.WithBlock(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return err
	}
	defer conn.Close()

	client := csi.NewNodeClient(conn)

	fn(ctx, client)

	return nil
}
// isBlockTargetPath returns true if the given CSI targetPath looks like
// the block-mode layout that kubelet uses for volumeMode: Block PVCs:
//   /var/lib/kubelet/plugins/kubernetes.io/csi/volumeDevices/publish/<volname>/<podUID>
// vs the filesystem-mode layout:
//   /var/lib/kubelet/pods/<podUID>/volumes/kubernetes.io~csi/<volname>/mount
func isBlockTargetPath(targetPath string) bool {
	return strings.Contains(targetPath, "/volumeDevices/publish/")
}

// isBlockStagingPath mirrors isBlockTargetPath for the stagingTargetPath
// that NodeStageVolume/NodeUnstageVolume see.
//   block:      /var/lib/kubelet/plugins/kubernetes.io/csi/volumeDevices/staging/<volname>
//   filesystem: /var/lib/kubelet/plugins/kubernetes.io/csi/<plugin>/<sha>/globalmount
func isBlockStagingPath(stagingPath string) bool {
	return strings.Contains(stagingPath, "/volumeDevices/staging/")
}

func (s *NodeService) getPodUIDandVolumeName(targetPath string) (podUID, volumeName string) {
	paths := strings.Split(targetPath, "/")
	glog.Infof("split paths is :%v", paths)
	if isBlockTargetPath(targetPath) {
		// Block mode:
		// /var/lib/kubelet/plugins/kubernetes.io/csi/volumeDevices/publish/<volname>/<podUID>
		// idx:       1   2   3       4         5            6    7              8        9          10
		if len(paths) >= 11 {
			podUID = paths[10]
			volumeName = paths[9]
		}
	} else {
		// Filesystem mode:
		// /var/lib/kubelet/pods/<podUID>/volumes/kubernetes.io~csi/<volname>/mount
		// idx:       1   2   3       4    5        6        7                  8        9
		if len(paths) >= 9 {
			podUID = paths[5]
			volumeName = paths[8]
		}
	}
	glog.Infof("podUid is :%v, volumeName is: %v", podUID, volumeName)
	return
}

func (s *NodeService) NodePublishVolume(ctx context.Context, req *csi.NodePublishVolumeRequest) (res *csi.NodePublishVolumeResponse, err error) {
	volumeID := utils.NormalizeVolumeID(req.GetVolumeId())
	savedPeerpodvolume, err := s.PeerpodvolumeClient.ConfidentialcontainersV1alpha1().PeerpodVolumes(s.Namespace).Get(context.Background(), volumeID, metav1.GetOptions{})
	if err != nil {
		glog.Infof("Not found PeerpodVolume with volumeID: %v, err: %v", volumeID, err.Error())
		if e := s.redirect(ctx, req, func(ctx context.Context, client csi.NodeClient) {
			res, err = client.NodePublishVolume(ctx, req)
		}); e != nil {
			return nil, e
		}
	} else {
		// Do fake action for NodePublishVolume on worker node
		publishContext := req.GetPublishContext()
		stagingTargetPath := req.GetStagingTargetPath()
		targetPath := req.GetTargetPath()
		// In block mode containerd's runtime spec generation calls
		// stat() on targetPath and rejects it unless it's a device
		// node. The actual EBS device lives in the peer-pod VM, so we
		// give containerd a placeholder block device on the worker:
		// create a 1 MiB sparse file, attach it to /dev/loopN via
		// losetup, then bind-mount that loop device over targetPath.
		// kata-agent recognises the kata-direct-volume entry we publish
		// alongside and replaces the placeholder with the real EBS
		// device when it builds the guest OCI spec.
		if isBlockTargetPath(targetPath) {
			parent := targetPath[:strings.LastIndex(targetPath, "/")]
			if err := os.MkdirAll(parent, os.FileMode(0755)); err != nil {
				glog.Warningf("Failed to create fake block targetPath parent, err is: %v", err)
			}
			// Idempotent: lazy-unmount any leftover bind-mount from a
			// prior wrapper version (or a prior pod with the same uid).
			_ = exec.Command("umount", "-l", targetPath).Run()
			// Create sparse backing file at the targetPath so kubelet's
			// MakeGlobalVolumePath (which propagates from this path to
			// globalMapPath via shared mounts) also sees a regular file.
			f, ferr := os.OpenFile(targetPath, os.O_CREATE|os.O_RDWR|os.O_TRUNC, os.FileMode(0644))
			if ferr != nil {
				glog.Warningf("Failed to create fake block targetPath file, err is: %v", ferr)
			} else {
				if err := f.Truncate(1 << 20); err != nil {
					glog.Warningf("Failed to truncate fake block targetPath file, err is: %v", err)
				}
				_ = f.Close()
			}
			// losetup -f --show <file> → /dev/loopN
			loopOut, lerr := exec.Command("losetup", "-f", "--show", targetPath).CombinedOutput()
			if lerr != nil {
				glog.Warningf("losetup -f failed for block targetPath, err: %v, out: %s", lerr, string(loopOut))
			} else {
				loopDev := strings.TrimSpace(string(loopOut))
				glog.Infof("attached %v to %v", targetPath, loopDev)
				// Bind-mount the loop device over the targetPath so
				// containerd's stat() on targetPath returns a block
				// device. The original sparse file is still backing the
				// loop device underneath.
				if out, berr := exec.Command("mount", "--bind", loopDev, targetPath).CombinedOutput(); berr != nil {
					if !strings.Contains(string(out), "already mounted") &&
						!strings.Contains(string(out), "busy") {
						glog.Warningf("bind-mount %v over %v failed, err: %v, out: %s", loopDev, targetPath, berr, string(out))
					}
				}
			}
		} else {
			osErr := os.MkdirAll(targetPath, os.FileMode(0755))
			if osErr != nil {
				glog.Warningf("Failed to create fake targetPath, err is: %v", osErr)
			}
		}
		glog.Infof("The stagingTargetPath is :%v", stagingTargetPath)
		glog.Infof("The targetPath is :%v", targetPath)
		glog.Infof("The publishContext is :%v", publishContext)

		// Block-mode volumes register as volume-type=block so kata-agent
		// passes the device through without mounting a filesystem.
		blockMode := isBlockTargetPath(targetPath)
		addKataDirectVolumeWithMode(targetPath, blockMode)

		var reqBuf bytes.Buffer
		if err := (&jsonpb.Marshaler{}).Marshal(&reqBuf, req); err != nil {
			glog.Error(err, "Error happens while Marshal NodePublishVolumeRequest")
		}
		nodePublishVolumeRequest := reqBuf.String()
		glog.Infof("NodePublishVolumeRequest JSON string: %s\n", nodePublishVolumeRequest)
		savedPeerpodvolume.Spec.TargetPath = targetPath
		podUID, volumeName := s.getPodUIDandVolumeName(targetPath)
		savedPeerpodvolume.Labels["podUid"] = podUID
		savedPeerpodvolume.Spec.PodUID = podUID
		savedVolumeName := savedPeerpodvolume.Spec.VolumeName
		if volumeName != savedVolumeName && savedVolumeName != peerpodVolumeNamePlaceholder {
			glog.Error("The volume name from target path doesn't match with the CR")
			return nil, errors.New("the volume name from target path doesn't match with the CR")
		}
		if savedVolumeName == peerpodVolumeNamePlaceholder {
			glog.Info("Detected a placeholder volume name in the CR. Updating the CR with the volume name from the target path")
			savedPeerpodvolume.Labels["volumeName"] = volumeName
			savedPeerpodvolume.Spec.VolumeName = volumeName
		}

		savedPeerpodvolume.Spec.WrapperNodePublishVolumeReq = nodePublishVolumeRequest
		_, err = s.PeerpodvolumeClient.ConfidentialcontainersV1alpha1().PeerpodVolumes(s.Namespace).Update(context.Background(), savedPeerpodvolume, metav1.UpdateOptions{})
		if err != nil {
			glog.Errorf("Error happens while Update PeerpodVolume, err: %v", err.Error())
			return
		}
		// TODO: error check
		savedPeerpodvolume, _ = s.PeerpodvolumeClient.ConfidentialcontainersV1alpha1().PeerpodVolumes(s.Namespace).Get(context.Background(), volumeID, metav1.GetOptions{})
		savedPeerpodvolume.Status = peerpodvolumeV1alpha1.PeerpodVolumeStatus{
			State: peerpodvolumeV1alpha1.NodePublishVolumeCached,
		}
		_, err = s.PeerpodvolumeClient.ConfidentialcontainersV1alpha1().PeerpodVolumes(s.Namespace).UpdateStatus(context.Background(), savedPeerpodvolume, metav1.UpdateOptions{})
		if err != nil {
			glog.Errorf("Error happens while Update PeerpodVolume status to NodePublishVolumeCached, err: %v", err.Error())
			return
		}

		res = &csi.NodePublishVolumeResponse{}
	}

	return
}

func (s *NodeService) NodeUnpublishVolume(ctx context.Context, req *csi.NodeUnpublishVolumeRequest) (res *csi.NodeUnpublishVolumeResponse, err error) {
	volumeID := utils.NormalizeVolumeID(req.GetVolumeId())
	savedPeerpodvolume, err := s.PeerpodvolumeClient.ConfidentialcontainersV1alpha1().PeerpodVolumes(s.Namespace).Get(context.Background(), volumeID, metav1.GetOptions{})
	if err != nil {
		glog.Infof("Not found PeerpodVolume with volumeID: %v, err: %v", volumeID, err.Error())
		if e := s.redirect(ctx, req, func(ctx context.Context, client csi.NodeClient) {
			res, err = client.NodeUnpublishVolume(ctx, req)
		}); e != nil {
			return nil, e
		}
	} else {
		// Do fake action for NodeUnpublishVolume on worker node
		targetPath := req.GetTargetPath()
		glog.Infof("The targetPath is :%v", targetPath)
		// In block mode we left a /dev/loopN bind-mounted over the
		// targetPath (a sparse backing file). Tear down in the right
		// order: unmount the bind, detach the loop device, then let
		// RemoveAll clean up the file. Lazy-unmount with -l so we
		// don't EBUSY if a CRI client still has a handle.
		if isBlockTargetPath(targetPath) {
			// Best-effort: query the loop device backing this file
			// before we unmount, so we can `losetup -d` it cleanly.
			loopOut, _ := exec.Command("losetup", "-j", targetPath).Output()
			_ = exec.Command("umount", "-l", targetPath).Run()
			for _, line := range strings.Split(strings.TrimSpace(string(loopOut)), "\n") {
				if i := strings.Index(line, ":"); i > 0 {
					if dev := strings.TrimSpace(line[:i]); strings.HasPrefix(dev, "/dev/loop") {
						_ = exec.Command("losetup", "-d", dev).Run()
					}
				}
			}
		}
		osErr := os.RemoveAll(targetPath)
		if osErr != nil {
			glog.Warningf("Failed to remove fake targetPath, err is: %v", osErr)
		}

		removeKataDirectVolume(targetPath)

		var reqBuf bytes.Buffer
		if err := (&jsonpb.Marshaler{}).Marshal(&reqBuf, req); err != nil {
			glog.Error(err, "Error happens while Marshal NodeUnpublishVolumeRequest")
		}
		nodeUnpublishVolumeRequest := reqBuf.String()
		glog.Infof("NodeUnpublishVolumeRequest JSON string: %s\n", nodeUnpublishVolumeRequest)

		savedPeerpodvolume.Spec.WrapperNodeUnpublishVolumeReq = nodeUnpublishVolumeRequest
		updatedPeerpodvolume, upErr := s.PeerpodvolumeClient.ConfidentialcontainersV1alpha1().PeerpodVolumes(s.Namespace).Update(context.Background(), savedPeerpodvolume, metav1.UpdateOptions{})
		if upErr != nil {
			glog.Errorf("Error happens while Update PeerpodVolume, err: %v", upErr.Error())
			return nil, upErr
		}
		updatedPeerpodvolume.Status = peerpodvolumeV1alpha1.PeerpodVolumeStatus{
			State: peerpodvolumeV1alpha1.NodeUnpublishVolumeCached,
		}
		_, err = s.PeerpodvolumeClient.ConfidentialcontainersV1alpha1().PeerpodVolumes(s.Namespace).UpdateStatus(context.Background(), savedPeerpodvolume, metav1.UpdateOptions{})
		if err != nil {
			glog.Errorf("Error happens while Update PeerpodVolume status to NodeUnpublishVolumeCached, err: %v", err.Error())
			return
		}

		res = &csi.NodeUnpublishVolumeResponse{}
	}

	return
}

func (s *NodeService) NodeStageVolume(ctx context.Context, req *csi.NodeStageVolumeRequest) (res *csi.NodeStageVolumeResponse, err error) {
	volumeID := utils.NormalizeVolumeID(req.GetVolumeId())
	savedPeerpodvolume, err := s.PeerpodvolumeClient.ConfidentialcontainersV1alpha1().PeerpodVolumes(s.Namespace).Get(context.Background(), volumeID, metav1.GetOptions{})
	if err != nil {
		glog.Infof("Not found PeerpodVolume with volumeID: %v, err: %v", volumeID, err.Error())
		if e := s.redirect(ctx, req, func(ctx context.Context, client csi.NodeClient) {
			res, err = client.NodeStageVolume(ctx, req)
		}); e != nil {
			return nil, e
		}
	} else {
		// Create empty dummy folders on worker node
		publishContext := req.GetPublishContext()
		stagingTargetPath := req.GetStagingTargetPath()
		_ = os.MkdirAll(stagingTargetPath, os.FileMode(0755)) // TODO: error check
		glog.Infof("The stagingTargetPath for volume %v is :%v", volumeID, stagingTargetPath)
		glog.Infof("The publishContext is :%v", publishContext)

		var reqBuf bytes.Buffer
		if err := (&jsonpb.Marshaler{}).Marshal(&reqBuf, req); err != nil {
			glog.Error(err, "Error happens while Marshal NodeStageVolumeRequest")
		}
		nodeStageVolumeRequest := reqBuf.String()
		glog.Infof("NodeStageVolumeRequest JSON string: %s\n", nodeStageVolumeRequest)
		savedPeerpodvolume.Spec.StagingTargetPath = stagingTargetPath
		savedPeerpodvolume.Spec.WrapperNodeStageVolumeReq = nodeStageVolumeRequest
		_, err = s.PeerpodvolumeClient.ConfidentialcontainersV1alpha1().PeerpodVolumes(s.Namespace).Update(context.Background(), savedPeerpodvolume, metav1.UpdateOptions{})
		if err != nil {
			glog.Errorf("Error happens while Update PeerpodVolume, err: %v", err.Error())
			return
		}
		// TODO: error check
		savedPeerpodvolume, _ = s.PeerpodvolumeClient.ConfidentialcontainersV1alpha1().PeerpodVolumes(s.Namespace).Get(context.Background(), volumeID, metav1.GetOptions{})
		savedPeerpodvolume.Status = peerpodvolumeV1alpha1.PeerpodVolumeStatus{
			State: peerpodvolumeV1alpha1.NodeStageVolumeCached,
		}
		_, err = s.PeerpodvolumeClient.ConfidentialcontainersV1alpha1().PeerpodVolumes(s.Namespace).UpdateStatus(context.Background(), savedPeerpodvolume, metav1.UpdateOptions{})
		if err != nil {
			glog.Errorf("Error happens while Update PeerpodVolume status to NodeStageVolumeCached, err: %v", err.Error())
			return
		}

		res = &csi.NodeStageVolumeResponse{}
	}

	return
}

func (s *NodeService) NodeUnstageVolume(ctx context.Context, req *csi.NodeUnstageVolumeRequest) (res *csi.NodeUnstageVolumeResponse, err error) {
	volumeID := utils.NormalizeVolumeID(req.GetVolumeId())
	savedPeerpodvolume, err := s.PeerpodvolumeClient.ConfidentialcontainersV1alpha1().PeerpodVolumes(s.Namespace).Get(context.Background(), volumeID, metav1.GetOptions{})
	if err != nil {
		glog.Infof("Not found PeerpodVolume with volumeID: %v, err: %v", volumeID, err.Error())
		if e := s.redirect(ctx, req, func(ctx context.Context, client csi.NodeClient) {
			res, err = client.NodeUnstageVolume(ctx, req)
		}); e != nil {
			return nil, e
		}
	} else {
		// Do fake action for NodeUnstageVolume  on worker node
		stagingTargetPath := req.GetStagingTargetPath()
		glog.Infof("The stagingTargetPath for volume %v is :%v", volumeID, stagingTargetPath)
		osErr := os.RemoveAll(stagingTargetPath)
		if osErr != nil {
			glog.Warningf("Failed to remove fake stagingTargetPath for volume %v, err is: %v", volumeID, osErr)
		}

		var reqBuf bytes.Buffer
		if err := (&jsonpb.Marshaler{}).Marshal(&reqBuf, req); err != nil {
			glog.Error(err, "Error happens while Marshal NodeUnstageVolumeRequest")
		}
		nodeUnstageVolumeRequest := reqBuf.String()
		glog.Infof("NodeUnstageVolumeRequest JSON string: %s\n", nodeUnstageVolumeRequest)

		savedPeerpodvolume.Spec.WrapperNodeUnstageVolumeReq = nodeUnstageVolumeRequest
		updatedPeerpodvolume, upErr := s.PeerpodvolumeClient.ConfidentialcontainersV1alpha1().PeerpodVolumes(s.Namespace).Update(context.Background(), savedPeerpodvolume, metav1.UpdateOptions{})
		if upErr != nil {
			glog.Errorf("Error happens while Update PeerpodVolume, err: %v", upErr.Error())
			return nil, upErr
		}
		updatedPeerpodvolume.Status = peerpodvolumeV1alpha1.PeerpodVolumeStatus{
			State: peerpodvolumeV1alpha1.NodeUnstageVolumeCached,
		}
		_, err = s.PeerpodvolumeClient.ConfidentialcontainersV1alpha1().PeerpodVolumes(s.Namespace).UpdateStatus(context.Background(), savedPeerpodvolume, metav1.UpdateOptions{})
		if err != nil {
			glog.Errorf("Error happens while Update PeerpodVolume status to NodeUnstageVolumeCached, err: %v", err.Error())
			return
		}

		res = &csi.NodeUnstageVolumeResponse{}
	}

	return
}

func (s *NodeService) NodeGetInfo(ctx context.Context, req *csi.NodeGetInfoRequest) (res *csi.NodeGetInfoResponse, err error) {
	if e := s.redirect(ctx, req, func(ctx context.Context, client csi.NodeClient) {
		res, err = client.NodeGetInfo(ctx, req)
	}); e != nil {
		return nil, e
	}

	return
}

func (s *NodeService) NodeGetCapabilities(ctx context.Context, req *csi.NodeGetCapabilitiesRequest) (res *csi.NodeGetCapabilitiesResponse, err error) {
	if e := s.redirect(ctx, req, func(ctx context.Context, client csi.NodeClient) {
		res, err = client.NodeGetCapabilities(ctx, req)
	}); e != nil {
		return nil, e
	}

	return
}

func (s *NodeService) NodeGetVolumeStats(ctx context.Context, req *csi.NodeGetVolumeStatsRequest) (res *csi.NodeGetVolumeStatsResponse, err error) {
	if e := s.redirect(ctx, req, func(ctx context.Context, client csi.NodeClient) {
		res, err = client.NodeGetVolumeStats(ctx, req)
	}); e != nil {
		return nil, e
	}

	return
}

func (s *NodeService) NodeExpandVolume(ctx context.Context, req *csi.NodeExpandVolumeRequest) (res *csi.NodeExpandVolumeResponse, err error) {
	if e := s.redirect(ctx, req, func(ctx context.Context, client csi.NodeClient) {
		res, err = client.NodeExpandVolume(ctx, req)
	}); e != nil {
		return nil, e
	}

	return
}

func (s *NodeService) SyncHandler(peerPodVolume *peerpodvolumeV1alpha1.PeerpodVolume) {
	if peerPodVolume.Spec.NodeName != os.Getenv("POD_NODE_NAME") {
		// Only handle the PeerpodVolume CRD which is assigned to the same compute node
		glog.Infof("Only handle the PeerpodVolume CRD which is assigned to %v", os.Getenv("POD_NODE_NAME"))
		return
	}
	glog.Infof("syncHandler from nodeService: %v ", peerPodVolume)
	switch peerPodVolume.Status.State {
	case peerpodvolumeV1alpha1.PeerPodVSIRunning:
		// The podName and podNamespace MUST be there when it's PeerPodVSIRunning status
		glog.Infof("Getting vmID for podName:%v, podNamespace:%v", peerPodVolume.Spec.PodName, peerPodVolume.Spec.PodNamespace)
		req := &podvminfo.GetInfoRequest{
			PodName:      peerPodVolume.Spec.PodName,
			PodNamespace: peerPodVolume.Spec.PodNamespace,
			Wait:         false,
		}
		glog.Infof("The get VM ID information request is: %v", req)
		conn, err := net.Dial("unix", s.VMIDInformationEndpoint)
		if err != nil {
			glog.Fatalf("Connect to vm id information service failed: %v", err)
		}
		ttrpcClient := ttrpc.NewClient(conn)
		defer ttrpcClient.Close()
		podVMInfoClient := podvminfo.NewPodVMInfoClient(ttrpcClient)
		res, err := podVMInfoClient.GetInfo(context.Background(), req)
		if err != nil {
			glog.Errorf("Error happens while get VM ID information, err: %v", err.Error())
		} else {
			vmID := res.VMID
			glog.Infof("Got the vm instance id from cloud-api-adaptor podVMInfoService vmID:%v", vmID)
			peerPodVolume.Spec.VMID = vmID
			peerPodVolume.Labels["vmID"] = utils.NormalizeVMID(vmID)
			updatedPeerPodVolume, err := s.PeerpodvolumeClient.ConfidentialcontainersV1alpha1().PeerpodVolumes(s.Namespace).Update(context.Background(), peerPodVolume, metav1.UpdateOptions{})
			if err != nil {
				glog.Errorf("Error happens while Update vmID to PeerpodVolume, err: %v", err.Error())
			}
			updatedPeerPodVolume.Status = peerpodvolumeV1alpha1.PeerpodVolumeStatus{
				State: peerpodvolumeV1alpha1.PeerPodVSIIDReady,
			}
			_, err = s.PeerpodvolumeClient.ConfidentialcontainersV1alpha1().PeerpodVolumes(s.Namespace).UpdateStatus(context.Background(), updatedPeerPodVolume, metav1.UpdateOptions{})
			if err != nil {
				glog.Errorf("Error happens while Update PeerpodVolume status to PeerPodVSIIDReady, err: %v", err.Error())
			}
			glog.Infof("The PeerpodVolume status updated to PeerPodVSIIDReady")
		}
	}
}

func (s *NodeService) DeleteFunction(peerPodVolume *peerpodvolumeV1alpha1.PeerpodVolume) {
	glog.Infof("deleteFunction from nodeService: %v ", peerPodVolume)
}
