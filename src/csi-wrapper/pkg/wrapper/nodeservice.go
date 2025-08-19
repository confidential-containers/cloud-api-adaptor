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
	"strings"

	podvminfo "github.com/confidential-containers/cloud-api-adaptor/src/cloud-api-adaptor/proto/podvminfo"
	"github.com/confidential-containers/cloud-api-adaptor/src/csi-wrapper/pkg/apis/peerpodvolume/v1alpha1"
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
)

type NodeService struct {
	TargetEndpoint          string
	Namespace               string
	PeerpodvolumeClient     *peerpodvolume.Clientset
	VMIDInformationEndpoint string
}

// Leverage Kata’s mechanism of direct block device assignment to prevent CSI mount source from replacing by Kata shim code
func addKataDirectVolume(volumePath string) {
	err := volume.Add(volumePath, DefaultMountInfo)
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
func (s *NodeService) getPodUIDandVolumeName(targetPath string) (podUID, volumeName string) {
	// /var/lib/kubelet/pods/69576836-28c2-447e-a726-fdf8866a0622/volumes/kubernetes.io~csi/pvc-e9d79b06-fd06-487f-ac93-ea6424819a7d/mount
	paths := strings.Split(targetPath, "/")
	glog.Infof("split paths is :%v", paths)
	podUID = paths[5]
	volumeName = paths[8]
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
		osErr := os.MkdirAll(targetPath, os.FileMode(0755))
		if osErr != nil {
			glog.Warningf("Failed to create fake targetPath, err is: %v", osErr)
		}
		glog.Infof("The stagingTargetPath is :%v", stagingTargetPath)
		glog.Infof("The targetPath is :%v", targetPath)
		glog.Infof("The publishContext is :%v", publishContext)

		addKataDirectVolume(targetPath)

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
		savedPeerpodvolume.Status = v1alpha1.PeerpodVolumeStatus{
			State: v1alpha1.NodePublishVolumeCached,
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
		updatedPeerpodvolume.Status = v1alpha1.PeerpodVolumeStatus{
			State: v1alpha1.NodeUnpublishVolumeCached,
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
		savedPeerpodvolume.Status = v1alpha1.PeerpodVolumeStatus{
			State: v1alpha1.NodeStageVolumeCached,
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
		updatedPeerpodvolume.Status = v1alpha1.PeerpodVolumeStatus{
			State: v1alpha1.NodeUnstageVolumeCached,
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

func (s *NodeService) SyncHandler(peerPodVolume *v1alpha1.PeerpodVolume) {
	if peerPodVolume.Spec.NodeName != os.Getenv("POD_NODE_NAME") {
		// Only handle the PeerpodVolume CRD which is assigned to the same compute node
		glog.Infof("Only handle the PeerpodVolume CRD which is assigned to %v", os.Getenv("POD_NODE_NAME"))
		return
	}
	glog.Infof("syncHandler from nodeService: %v ", peerPodVolume)
	switch peerPodVolume.Status.State {
	case v1alpha1.PeerPodVSIRunning:
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
			updatedPeerPodVolume.Status = v1alpha1.PeerpodVolumeStatus{
				State: v1alpha1.PeerPodVSIIDReady,
			}
			_, err = s.PeerpodvolumeClient.ConfidentialcontainersV1alpha1().PeerpodVolumes(s.Namespace).UpdateStatus(context.Background(), updatedPeerPodVolume, metav1.UpdateOptions{})
			if err != nil {
				glog.Errorf("Error happens while Update PeerpodVolume status to PeerPodVSIIDReady, err: %v", err.Error())
			}
			glog.Infof("The PeerpodVolume status updated to PeerPodVSIIDReady")
		}
	}
}

func (s *NodeService) DeleteFunction(peerPodVolume *v1alpha1.PeerpodVolume) {
	glog.Infof("deleteFunction from nodeService: %v ", peerPodVolume)
}
