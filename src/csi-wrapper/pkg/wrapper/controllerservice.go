// Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package wrapper

import (
	"bytes"
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/confidential-containers/cloud-api-adaptor/src/csi-wrapper/pkg/apis/peerpodvolume/v1alpha1"
	peerpodvolume "github.com/confidential-containers/cloud-api-adaptor/src/csi-wrapper/pkg/generated/peerpodvolume/clientset/versioned"
	"github.com/confidential-containers/cloud-api-adaptor/src/csi-wrapper/pkg/utils"
	"github.com/container-storage-interface/spec/lib/go/csi"
	uid "github.com/gofrs/uuid"
	"github.com/golang/glog"
	"github.com/golang/protobuf/jsonpb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// PublishInfoVolumeID ...
	PublishInfoVolumeID = "volume-id"

	// PublishInfoNodeID ...
	PublishInfoNodeID = "node-id"

	// PublishInfoStatus ...
	PublishInfoStatus = "attach-status"

	// PublishInfoDevicePath ...
	PublishInfoDevicePath = "device-path"

	// PublishInfoRequestID ...
	PublishInfoRequestID = "request-id"

	// Parameter key for Peer Pod from StorageClass
	PeerpodParamKey = "peerpod"

	// peerpodVolumeNamePlaceholder is used when creating a peerpodvolume from an existing PV.
	// For existing PVs, we have to create the peerpodvolume CR without knowing the real volume name in [ControllerService.ControllerPublishVolume].
	// We only later know the real volume name when we process the CR again in [NodeService.NodePublishVolume].
	peerpodVolumeNamePlaceholder = "peerpod-volume-name-placeholder"
)

// azureVMRegexp checks if used to validate an Azure resource ID for a VM, or scale set VM.
var azureVMRegexp = regexp.MustCompile(`(?i)^/subscriptions/[^/]+/resourceGroups/[^/]+/providers/Microsoft\.Compute/(virtualMachines|virtualMachineScaleSets/[^/]+/virtualMachines)/[^/]+$`)

type ControllerService struct {
	TargetEndpoint      string
	Namespace           string
	PeerpodvolumeClient *peerpodvolume.Clientset
}

func NewControllerService(targetEndpoint, namespace string, peerpodvolumeClientSet *peerpodvolume.Clientset) *ControllerService {
	return &ControllerService{
		Namespace:           namespace,
		TargetEndpoint:      fmt.Sprintf("unix://%s", targetEndpoint),
		PeerpodvolumeClient: peerpodvolumeClientSet,
	}
}

func (s *ControllerService) redirect(ctx context.Context, req interface{}, fn func(context.Context, csi.ControllerClient)) error {
	// grpc.Dial is deprecated and supported only with grpc 1.x
	//nolint:staticcheck
	conn, err := grpc.Dial(s.TargetEndpoint, grpc.WithBlock(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return err
	}
	defer conn.Close()

	client := csi.NewControllerClient(conn)

	fn(ctx, client)

	return nil
}

func (s *ControllerService) CreateVolume(ctx context.Context, req *csi.CreateVolumeRequest) (res *csi.CreateVolumeResponse, err error) {
	if e := s.redirect(ctx, req, func(ctx context.Context, client csi.ControllerClient) {
		peerpod := req.Parameters[PeerpodParamKey]
		if peerpod != "" {
			// Delete peerpod key from req parameters because csi driver may check parameters strictly.
			delete(req.Parameters, PeerpodParamKey)
		}

		volumeName := req.Name
		res, err = client.CreateVolume(ctx, req)
		glog.Infof("Created volume response: %s\n", res)

		// Create PeerpodVolume CRD object only when peerpod parameter is found in request
		if peerpod != "" {
			volumeID := res.GetVolume().VolumeId
			normalizedVolumeID := utils.NormalizeVolumeID(volumeID)
			_, _ = s.createPeerpodVolume(normalizedVolumeID, volumeName)
		}
	}); e != nil {
		return nil, e
	}

	return
}

func (s *ControllerService) DeleteVolume(ctx context.Context, req *csi.DeleteVolumeRequest) (res *csi.DeleteVolumeResponse, err error) {
	if e := s.redirect(ctx, req, func(ctx context.Context, client csi.ControllerClient) {
		res, err = client.DeleteVolume(ctx, req)

		volumeID := utils.NormalizeVolumeID(req.GetVolumeId())
		_, err = s.PeerpodvolumeClient.ConfidentialcontainersV1alpha1().PeerpodVolumes(s.Namespace).Get(context.Background(), volumeID, metav1.GetOptions{})
		if err != nil {
			glog.Infof("Not found PeerpodVolume with volumeID: %v, err: %v", volumeID, err.Error())
		} else {
			ppErr := s.PeerpodvolumeClient.ConfidentialcontainersV1alpha1().PeerpodVolumes(s.Namespace).Delete(context.Background(), volumeID, metav1.DeleteOptions{})
			if ppErr != nil {
				glog.Warningf("Failed to delete to Peerpodvolume by volumeID: %v, err: %v", volumeID, ppErr.Error())
			} else {
				glog.Infof("The peerPodVolume is deleted, volumeID: %v", volumeID)
			}
		}
	}); e != nil {
		return nil, e
	}

	return
}

func (s *ControllerService) ControllerPublishVolume(ctx context.Context, req *csi.ControllerPublishVolumeRequest) (res *csi.ControllerPublishVolumeResponse, err error) {
	volumeID := utils.NormalizeVolumeID(req.GetVolumeId())
	savedPeerpodvolume, err := s.PeerpodvolumeClient.ConfidentialcontainersV1alpha1().PeerpodVolumes(s.Namespace).Get(context.Background(), volumeID, metav1.GetOptions{})
	if err != nil {
		glog.Infof("Not found PeerpodVolume with volumeID: %v, err: %v", volumeID, err.Error())

		// ControllerPublishVolume was called without a matching peerpod volume object existing.
		// This can happen when the persistent volume was manually created, e.g. to consume an existing storage backend.
		// In this case, we need to create the peerpod volume object here.
		peerPod := req.VolumeContext[PeerpodParamKey]
		if peerPod == "" {
			if e := s.redirect(ctx, req, func(ctx context.Context, client csi.ControllerClient) {
				res, err = client.ControllerPublishVolume(ctx, req)
			}); e != nil {
				return nil, e
			}
		}
		glog.Info("PeerPod parameter found in ControllerPublishVolumeRequest. Creating a new PeerpodVolume object")

		// Delete peerpod key from req parameters because csi driver may check parameters strictly.
		delete(req.VolumeContext, PeerpodParamKey)

		volumeName := peerpodVolumeNamePlaceholder
		savedPeerpodvolume, err = s.createPeerpodVolume(volumeID, volumeName)
		if err != nil {
			return nil, err
		}
	}

	nodeID := req.GetNodeId()
	uuid, _ := uid.NewV4() // #nosec G104: Attempt to randomly generate uuid
	requestID := uuid.String()

	fakePublishContext := map[string]string{
		PublishInfoVolumeID:   volumeID,
		PublishInfoNodeID:     "",
		PublishInfoStatus:     "attached",
		PublishInfoDevicePath: "/dev/fff",
		PublishInfoRequestID:  requestID,
	}
	res = &csi.ControllerPublishVolumeResponse{PublishContext: fakePublishContext}

	glog.Infof("The fake ControllerPublishVolumeResponse is :%v", res)
	var reqBuf bytes.Buffer
	if err := (&jsonpb.Marshaler{}).Marshal(&reqBuf, req); err != nil {
		glog.Error(err, "Error happens while Marshal ControllerPublishVolumeRequest")
	}
	reqJSONString := reqBuf.String()
	glog.Infof("ControllerPublishVolumeRequest JSON string: %s\n", reqJSONString)

	var resBuf bytes.Buffer
	if err := (&jsonpb.Marshaler{}).Marshal(&resBuf, res); err != nil {
		glog.Error(err, "Error happens while Marshal ControllerPublishVolumeResponse")
	}
	resJSONString := resBuf.String()
	glog.Infof("ControllerPublishVolumeResponse JSON string: %s\n", resJSONString)

	savedPeerpodvolume.Labels["nodeID"] = nodeID
	savedPeerpodvolume.Spec.NodeID = nodeID
	savedPeerpodvolume.Spec.WrapperControllerPublishVolumeReq = string(reqJSONString)
	savedPeerpodvolume.Spec.WrapperControllerPublishVolumeRes = string(resJSONString)
	savedPeerpodvolume, err = s.PeerpodvolumeClient.ConfidentialcontainersV1alpha1().PeerpodVolumes(s.Namespace).Update(context.Background(), savedPeerpodvolume, metav1.UpdateOptions{})
	if err != nil {
		glog.Errorf("Error happens while Update PeerpodVolume in ControllerPublishVolume, err: %v", err.Error())
		return
	}
	savedPeerpodvolume.Status = v1alpha1.PeerpodVolumeStatus{
		State: v1alpha1.ControllerPublishVolumeCached,
	}
	_, err = s.PeerpodvolumeClient.ConfidentialcontainersV1alpha1().PeerpodVolumes(s.Namespace).UpdateStatus(context.Background(), savedPeerpodvolume, metav1.UpdateOptions{})
	if err != nil {
		glog.Errorf("Error happens while Update PeerpodVolume status to ControllerPublishVolumeCached, err: %v", err.Error())
	}

	return
}

func (s *ControllerService) ControllerUnpublishVolume(ctx context.Context, req *csi.ControllerUnpublishVolumeRequest) (res *csi.ControllerUnpublishVolumeResponse, err error) {
	volumeID := utils.NormalizeVolumeID(req.GetVolumeId())
	savedPeerpodvolume, err := s.PeerpodvolumeClient.ConfidentialcontainersV1alpha1().PeerpodVolumes(s.Namespace).Get(context.Background(), volumeID, metav1.GetOptions{})
	if err != nil {
		glog.Infof("Not found PeerpodVolume with volumeID: %v, err: %v", volumeID, err.Error())
		if e := s.redirect(ctx, req, func(ctx context.Context, client csi.ControllerClient) {
			res, err = client.ControllerUnpublishVolume(ctx, req)
		}); e != nil {
			return nil, e
		}
	} else {
		statusString := string(savedPeerpodvolume.Status.State)
		if strings.Contains(statusString, "Applied") {
			// volume is attached to peer pod vm if the status.state end with `Applied`
			req.NodeId = savedPeerpodvolume.Spec.VMID
			glog.Infof("The modified ControllerUnpublishVolumeRequest is :%v", req)
			ctx := context.Background()
			// TODO: error check
			_ = s.redirect(ctx, req, func(ctx context.Context, client csi.ControllerClient) {
				response, err := client.ControllerUnpublishVolume(ctx, req)
				if err != nil {
					glog.Errorf("Failed to run ControllerUnpublishVolume with modified ControllerUnpublishVolume, err: %v", err.Error())
				} else {
					glog.Infof("The ControllerUnpublishVolumeResponse for peer pod is :%v", response)
				}
			})
		}

		labels := map[string]string{
			"volumeName": savedPeerpodvolume.Spec.VolumeName,
		}
		savedPeerpodvolume.Labels = labels
		savedPeerpodvolume.Spec.NodeID = ""
		savedPeerpodvolume.Spec.DevicePath = ""
		savedPeerpodvolume.Spec.NodeName = ""
		savedPeerpodvolume.Spec.PodName = ""
		savedPeerpodvolume.Spec.PodNamespace = ""
		savedPeerpodvolume.Spec.PodUID = ""
		savedPeerpodvolume.Spec.StagingTargetPath = ""
		savedPeerpodvolume.Spec.TargetPath = ""
		savedPeerpodvolume.Spec.VMID = ""
		savedPeerpodvolume.Spec.VMName = ""
		savedPeerpodvolume.Spec.WrapperControllerPublishVolumeReq = ""
		savedPeerpodvolume.Spec.WrapperControllerPublishVolumeRes = ""
		savedPeerpodvolume.Spec.WrapperNodePublishVolumeReq = ""
		savedPeerpodvolume.Spec.WrapperNodeStageVolumeReq = ""
		savedPeerpodvolume.Spec.WrapperNodeUnpublishVolumeReq = ""
		savedPeerpodvolume.Spec.WrapperNodeUnstageVolumeReq = ""
		updatedSavedPeerpodvolume, err := s.PeerpodvolumeClient.ConfidentialcontainersV1alpha1().PeerpodVolumes(s.Namespace).Update(context.Background(), savedPeerpodvolume, metav1.UpdateOptions{})
		if err != nil {
			glog.Errorf("Error happens while clean PeerpodVolume specs, err: %v", err.Error())
		}

		updatedSavedPeerpodvolume.Status = v1alpha1.PeerpodVolumeStatus{
			State: "",
		}
		_, err = s.PeerpodvolumeClient.ConfidentialcontainersV1alpha1().PeerpodVolumes(s.Namespace).UpdateStatus(context.Background(), updatedSavedPeerpodvolume, metav1.UpdateOptions{})
		if err != nil {
			glog.Errorf("Error happens while Update PeerpodVolume status to ControllerUnpublishVolumeApplied, err: %v", err.Error())
		}

		res = &csi.ControllerUnpublishVolumeResponse{}
	}

	return
}

func (s *ControllerService) ValidateVolumeCapabilities(ctx context.Context, req *csi.ValidateVolumeCapabilitiesRequest) (res *csi.ValidateVolumeCapabilitiesResponse, err error) {
	if e := s.redirect(ctx, req, func(ctx context.Context, client csi.ControllerClient) {
		res, err = client.ValidateVolumeCapabilities(ctx, req)
	}); e != nil {
		return nil, e
	}

	return
}

func (s *ControllerService) ListVolumes(ctx context.Context, req *csi.ListVolumesRequest) (res *csi.ListVolumesResponse, err error) {
	if e := s.redirect(ctx, req, func(ctx context.Context, client csi.ControllerClient) {
		res, err = client.ListVolumes(ctx, req)
	}); e != nil {
		return nil, e
	}

	return
}

func (s *ControllerService) GetCapacity(ctx context.Context, req *csi.GetCapacityRequest) (res *csi.GetCapacityResponse, err error) {
	if e := s.redirect(ctx, req, func(ctx context.Context, client csi.ControllerClient) {
		res, err = client.GetCapacity(ctx, req)
	}); e != nil {
		return nil, e
	}

	return
}

func (s *ControllerService) ControllerGetCapabilities(ctx context.Context, req *csi.ControllerGetCapabilitiesRequest) (res *csi.ControllerGetCapabilitiesResponse, err error) {
	if e := s.redirect(ctx, req, func(ctx context.Context, client csi.ControllerClient) {
		res, err = client.ControllerGetCapabilities(ctx, req)
	}); e != nil {
		return nil, e
	}

	return
}

func (s *ControllerService) CreateSnapshot(ctx context.Context, req *csi.CreateSnapshotRequest) (res *csi.CreateSnapshotResponse, err error) {
	if e := s.redirect(ctx, req, func(ctx context.Context, client csi.ControllerClient) {
		res, err = client.CreateSnapshot(ctx, req)
	}); e != nil {
		return nil, e
	}

	return
}

func (s *ControllerService) DeleteSnapshot(ctx context.Context, req *csi.DeleteSnapshotRequest) (res *csi.DeleteSnapshotResponse, err error) {
	if e := s.redirect(ctx, req, func(ctx context.Context, client csi.ControllerClient) {
		res, err = client.DeleteSnapshot(ctx, req)
	}); e != nil {
		return nil, e
	}

	return
}

func (s *ControllerService) ListSnapshots(ctx context.Context, req *csi.ListSnapshotsRequest) (res *csi.ListSnapshotsResponse, err error) {
	if e := s.redirect(ctx, req, func(ctx context.Context, client csi.ControllerClient) {
		res, err = client.ListSnapshots(ctx, req)
	}); e != nil {
		return nil, e
	}

	return
}

func (s *ControllerService) ControllerExpandVolume(ctx context.Context, req *csi.ControllerExpandVolumeRequest) (res *csi.ControllerExpandVolumeResponse, err error) {
	if e := s.redirect(ctx, req, func(ctx context.Context, client csi.ControllerClient) {
		res, err = client.ControllerExpandVolume(ctx, req)
	}); e != nil {
		return nil, e
	}

	return
}

func (s *ControllerService) ControllerGetVolume(ctx context.Context, req *csi.ControllerGetVolumeRequest) (res *csi.ControllerGetVolumeResponse, err error) {
	if e := s.redirect(ctx, req, func(ctx context.Context, client csi.ControllerClient) {
		res, err = client.ControllerGetVolume(ctx, req)
	}); e != nil {
		return nil, e
	}

	return
}

func (s *ControllerService) SyncHandler(peerPodVolume *v1alpha1.PeerpodVolume) {
	glog.Infof("syncHandler from ControllerService: %v ", peerPodVolume)
	if peerPodVolume.Status.State == v1alpha1.PeerPodVSIIDReady && peerPodVolume.Spec.DevicePath == "" {
		// After peerpod vsi id is ready in crd object, we can reproduce the ControllerPublishVolumeRequest
		vsiID := peerPodVolume.Spec.VMID

		// The azure csi driver requires the nodeID to be just the name of the VM,
		// not the full Azure Resource ID, as it is saved in the PeerpodVolume object
		if azureVMRegexp.MatchString(vsiID) {
			vsiID = filepath.Base(vsiID)
		}

		// Replace the nodeID with peerpod vsi instance id in ControllerPublishVolumeRequest and pass
		// the modified ControllerPublishVolumeRequest to original controller service
		wrapperRequest := peerPodVolume.Spec.WrapperControllerPublishVolumeReq
		var modifiedRequest csi.ControllerPublishVolumeRequest
		if err := (&jsonpb.Unmarshaler{}).Unmarshal(bytes.NewReader([]byte(wrapperRequest)), &modifiedRequest); err != nil {
			glog.Errorf("Failed to convert to ControllerPublishVolumeRequest, err: %v", err.Error())
		} else {
			modifiedRequest.NodeId = vsiID
			glog.Infof("The modified ControllerPublishVolumeRequest is :%v", modifiedRequest)
			ctx := context.Background()
			// TODO: error check
			_ = s.redirect(ctx, modifiedRequest, func(ctx context.Context, client csi.ControllerClient) {
				response, err := client.ControllerPublishVolume(ctx, &modifiedRequest)
				if err != nil {
					glog.Errorf("Failed to reproduce ControllerPublishVolume with modified ControllerPublishVolumeRequest, err: %v", err.Error())
				} else {
					glog.Infof("The ControllerPublishVolumeResponse for peer pod is :%v", response)
					var resBuf bytes.Buffer
					if err := (&jsonpb.Marshaler{}).Marshal(&resBuf, response); err != nil {
						glog.Error(err, "Error happens while Marshal ControllerPublishVolumeResponse")
					}
					resJSONString := resBuf.String()
					glog.Infof("ControllerPublishVolumeResponse for peer pod JSON string: %s\n", resJSONString)
					peerPodVolume.Spec.WrapperControllerPublishVolumeRes = resJSONString
					devicePath := response.PublishContext["device-path"]
					glog.Infof("device-path for peer pod VM: %s\n", devicePath)
					peerPodVolume.Spec.DevicePath = devicePath
					updatedPeerPodVolume, err := s.PeerpodvolumeClient.ConfidentialcontainersV1alpha1().PeerpodVolumes(s.Namespace).Update(context.Background(), peerPodVolume, metav1.UpdateOptions{})
					if err != nil {
						glog.Errorf("Error happens while Update PeerpodVolume with ControllerPublishVolumeResponse for peer pod, err: %v", err.Error())
						return
					}
					updatedPeerPodVolume.Status = v1alpha1.PeerpodVolumeStatus{
						State: v1alpha1.ControllerPublishVolumeApplied,
					}
					_, err = s.PeerpodvolumeClient.ConfidentialcontainersV1alpha1().PeerpodVolumes(s.Namespace).UpdateStatus(context.Background(), updatedPeerPodVolume, metav1.UpdateOptions{})
					if err != nil {
						glog.Errorf("Error happens while Update PeerpodVolume status to ControllerPublishVolumeApplied, err: %v", err.Error())
					}
				}
			})
		}
	}
}

func (s *ControllerService) DeleteFunction(peerPodVolume *v1alpha1.PeerpodVolume) {
	glog.Infof("deleteFunction from controllerService: %v ", peerPodVolume)
}

func (s *ControllerService) createPeerpodVolume(volumeID, volumeName string) (*v1alpha1.PeerpodVolume, error) {
	labels := map[string]string{
		"volumeName": volumeName,
	}
	newPeerpodvolume := &v1alpha1.PeerpodVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name:      volumeID,
			Namespace: s.Namespace,
			Labels:    labels,
		},
		Spec: v1alpha1.PeerpodVolumeSpec{
			VolumeID:   volumeID,
			VolumeName: volumeName,
		},
	}
	peerpodVolume, err := s.PeerpodvolumeClient.ConfidentialcontainersV1alpha1().PeerpodVolumes(s.Namespace).Create(context.Background(), newPeerpodvolume, metav1.CreateOptions{})
	if err != nil {
		glog.Errorf("Error happens while creating peerPodVolume with volumeID: %v, err: %v", volumeID, err.Error())
	} else {
		glog.Infof("Peerpodvolume object is created")
	}
	return peerpodVolume, err
}
