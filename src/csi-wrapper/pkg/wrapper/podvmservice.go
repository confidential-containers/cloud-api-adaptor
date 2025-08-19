// Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package wrapper

import (
	"bytes"
	"context"
	"fmt"
	"os"

	"github.com/confidential-containers/cloud-api-adaptor/src/csi-wrapper/pkg/apis/peerpodvolume/v1alpha1"
	peerpodvolume "github.com/confidential-containers/cloud-api-adaptor/src/csi-wrapper/pkg/generated/peerpodvolume/clientset/versioned"
	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/golang/glog"
	"github.com/golang/protobuf/jsonpb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type PodVMNodeService struct {
	TargetEndpoint      string
	Namespace           string
	PeerpodvolumeClient *peerpodvolume.Clientset
}

func NewPodVMNodeService(targetEndpoint, namespace string, peerpodvolumeClientSet *peerpodvolume.Clientset) *PodVMNodeService {
	return &PodVMNodeService{
		Namespace:           namespace,
		TargetEndpoint:      fmt.Sprintf("unix://%s", targetEndpoint),
		PeerpodvolumeClient: peerpodvolumeClientSet,
	}
}

func (s *PodVMNodeService) redirect(ctx context.Context, req interface{}, fn func(context.Context, csi.NodeClient)) error {
	// grpc.Dial is deprecated and supported only with grpc 1.x
	//nolint:staticcheck
	conn, err := grpc.Dial(s.TargetEndpoint, grpc.WithBlock(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		glog.Errorf("failed to connect s.TargetEndpoint: %v, err:%v", s.TargetEndpoint, err)
		return err
	}
	defer conn.Close()

	client := csi.NewNodeClient(conn)
	glog.Infof("NewNodeClient client: %v", client)
	fn(ctx, client)

	return nil
}

func (s *PodVMNodeService) NodePublishVolume(ctx context.Context, req *csi.NodePublishVolumeRequest) (res *csi.NodePublishVolumeResponse, err error) {
	if e := s.redirect(ctx, req, func(ctx context.Context, client csi.NodeClient) {
		res, err = client.NodePublishVolume(ctx, req)
	}); e != nil {
		return nil, e
	}

	return
}

func (s *PodVMNodeService) NodeUnpublishVolume(ctx context.Context, req *csi.NodeUnpublishVolumeRequest) (res *csi.NodeUnpublishVolumeResponse, err error) {
	if e := s.redirect(ctx, req, func(ctx context.Context, client csi.NodeClient) {
		res, err = client.NodeUnpublishVolume(ctx, req)
	}); e != nil {
		return nil, e
	}

	return
}

func (s *PodVMNodeService) NodeStageVolume(ctx context.Context, req *csi.NodeStageVolumeRequest) (res *csi.NodeStageVolumeResponse, err error) {
	if e := s.redirect(ctx, req, func(ctx context.Context, client csi.NodeClient) {
		res, err = client.NodeStageVolume(ctx, req)
	}); e != nil {
		return nil, e
	}

	return
}

func (s *PodVMNodeService) NodeUnstageVolume(ctx context.Context, req *csi.NodeUnstageVolumeRequest) (res *csi.NodeUnstageVolumeResponse, err error) {
	if e := s.redirect(ctx, req, func(ctx context.Context, client csi.NodeClient) {
		res, err = client.NodeUnstageVolume(ctx, req)
	}); e != nil {
		return nil, e
	}

	return
}

func (s *PodVMNodeService) NodeGetInfo(ctx context.Context, req *csi.NodeGetInfoRequest) (res *csi.NodeGetInfoResponse, err error) {
	if e := s.redirect(ctx, req, func(ctx context.Context, client csi.NodeClient) {
		res, err = client.NodeGetInfo(ctx, req)
	}); e != nil {
		return nil, e
	}

	return
}

func (s *PodVMNodeService) NodeGetCapabilities(ctx context.Context, req *csi.NodeGetCapabilitiesRequest) (res *csi.NodeGetCapabilitiesResponse, err error) {
	if e := s.redirect(ctx, req, func(ctx context.Context, client csi.NodeClient) {
		res, err = client.NodeGetCapabilities(ctx, req)
	}); e != nil {
		return nil, e
	}

	return
}

func (s *PodVMNodeService) NodeGetVolumeStats(ctx context.Context, req *csi.NodeGetVolumeStatsRequest) (res *csi.NodeGetVolumeStatsResponse, err error) {
	if e := s.redirect(ctx, req, func(ctx context.Context, client csi.NodeClient) {
		res, err = client.NodeGetVolumeStats(ctx, req)
	}); e != nil {
		return nil, e
	}

	return
}

func (s *PodVMNodeService) NodeExpandVolume(ctx context.Context, req *csi.NodeExpandVolumeRequest) (res *csi.NodeExpandVolumeResponse, err error) {
	if e := s.redirect(ctx, req, func(ctx context.Context, client csi.NodeClient) {
		res, err = client.NodeExpandVolume(ctx, req)
	}); e != nil {
		return nil, e
	}

	return
}

func (s *PodVMNodeService) ReproduceNodeStageVolume(peerPodVolume *v1alpha1.PeerpodVolume) {
	glog.Infof("Reproducing NodeStageVolumeRequest for peer pod")
	wrapperRequest := peerPodVolume.Spec.WrapperNodeStageVolumeReq
	var modifiedRequest csi.NodeStageVolumeRequest
	if err := (&jsonpb.Unmarshaler{}).Unmarshal(bytes.NewReader([]byte(wrapperRequest)), &modifiedRequest); err != nil {
		glog.Errorf("Failed to convert to NodeStageVolumeRequest, err: %v", err.Error())
	} else {
		// The cached NodeStageVolumeRequest contains a faked PublishContext from [ControllerService.ControllerPublishVolume].
		// Since a CSI driver may depend on PublishContext to pass required information from ControllerPublishVolume to NodeStageVolume,
		// we need to replace the PublishContext in the cached NodeStageVolumeRequest with the real one from
		// the cached ControllerPublishVolumeResponse.
		publishContext := make(map[string]string)
		controllerPublishVolumeResJSON := peerPodVolume.Spec.WrapperControllerPublishVolumeRes
		var controllerPublishVolumeRes csi.ControllerPublishVolumeResponse
		if err := (&jsonpb.Unmarshaler{}).Unmarshal(bytes.NewReader([]byte(controllerPublishVolumeResJSON)), &controllerPublishVolumeRes); err != nil {
			glog.Errorf("Failed to convert to ControllerPublishVolumeResponse, err: %s", err)
		}
		for k, v := range controllerPublishVolumeRes.PublishContext {
			publishContext[k] = v
		}
		publishContext["device-path"] = peerPodVolume.Spec.DevicePath
		modifiedRequest.PublishContext = publishContext

		glog.Infof("The modified NodeStageVolumeRequest is :%v", modifiedRequest)
		ctx := context.Background()
		count := 0
		reproduced := false
		for {
			glog.Infof("start to Reproducing NodeStageVolumeRequest for peer pod (retrying... %d/%d)", count, 20)
			// TODO: error check
			_ = s.redirect(ctx, modifiedRequest, func(ctx context.Context, client csi.NodeClient) {
				response, err := client.NodeStageVolume(ctx, &modifiedRequest)
				glog.Infof("The NodeStageVolumeResponse for peer pod is :%v", response)
				if err != nil {
					glog.Errorf("Failed to reproduce NodeStageVolume with modified NodeStageVolumeRequest, err: %v", err.Error())
				} else {
					peerPodVolume.Status = v1alpha1.PeerpodVolumeStatus{
						State: v1alpha1.NodeStageVolumeApplied,
					}
					_, err := s.PeerpodvolumeClient.ConfidentialcontainersV1alpha1().PeerpodVolumes(s.Namespace).UpdateStatus(context.Background(), peerPodVolume, metav1.UpdateOptions{})
					if err != nil {
						glog.Errorf("Error happens while Update PeerpodVolume status to NodeStageVolumeApplied, err: %v", err.Error())
					} else {
						reproduced = true
					}
				}
			})
			if count == 20 {
				glog.Error("reaches max retry count. gave up Reproducing NodeStageVolumeRequest for peer pod")
				break
			}
			if reproduced {
				break
			}
			glog.Infof("failed to Reproducing NodeStageVolumeRequest for peer pod (retrying... %d/%d)", count, 20)
			count++
		}

	}
}

func (s *PodVMNodeService) ReproduceNodePublishVolume(peerPodVolume *v1alpha1.PeerpodVolume) {
	glog.Infof("Reproducing nodePublishVolumeRequest for peer pod")
	wrapperRequest := peerPodVolume.Spec.WrapperNodePublishVolumeReq
	var nodePublishVolumeRequest csi.NodePublishVolumeRequest
	if err := (&jsonpb.Unmarshaler{}).Unmarshal(bytes.NewReader([]byte(wrapperRequest)), &nodePublishVolumeRequest); err != nil {
		glog.Errorf("Failed to convert to NodePublishVolumeRequest, err: %v", err.Error())
	} else {
		glog.Infof("The NodePublishVolumeRequest is :%v", nodePublishVolumeRequest)
		ctx := context.Background()
		count := 0
		reproduced := false
		for {
			glog.Infof("start to Reproducing nodePublishVolumeRequest for peer pod (retrying... %d/%d)", count, 20)
			// TODO: error check
			_ = s.redirect(ctx, nodePublishVolumeRequest, func(ctx context.Context, client csi.NodeClient) {
				response, err := client.NodePublishVolume(ctx, &nodePublishVolumeRequest)
				glog.Infof("The NodePublishVolumeResponse for peer pod is :%v", response)
				if err != nil {
					glog.Errorf("Failed to reproduce NodePublishVolume with the NodePublishVolumeRequest, err: %v", err.Error())
				} else {
					peerPodVolume.Status = v1alpha1.PeerpodVolumeStatus{
						State: v1alpha1.NodePublishVolumeApplied,
					}
					_, err := s.PeerpodvolumeClient.ConfidentialcontainersV1alpha1().PeerpodVolumes(s.Namespace).UpdateStatus(context.Background(), peerPodVolume, metav1.UpdateOptions{})
					if err != nil {
						glog.Errorf("Error happens while Update PeerpodVolume status to NodePublishVolumeApplied, err: %v", err.Error())
					} else {
						reproduced = true
					}
				}
			})
			if count == 20 {
				glog.Error("reaches max retry count. gave up Reproducing nodePublishVolumeRequest for peer pod")
				break
			}
			if reproduced {
				break
			}
			glog.Infof("failed to Reproducing nodePublishVolumeRequest for peer pod (retrying... %d/%d)", count, 20)
			count++
		}
	}
}

func (s *PodVMNodeService) ReproduceNodeUnpublishVolume(peerPodVolume *v1alpha1.PeerpodVolume) {
	glog.Infof("Reproducing nodeUnPublishVolumeRequest for peer pod")
	wrapperRequest := peerPodVolume.Spec.WrapperNodeUnpublishVolumeReq
	var nodeUnpublishVolumeRequest csi.NodeUnpublishVolumeRequest
	if err := (&jsonpb.Unmarshaler{}).Unmarshal(bytes.NewReader([]byte(wrapperRequest)), &nodeUnpublishVolumeRequest); err != nil {
		glog.Errorf("Failed to convert to NodeUnpublishVolumeRequest, err: %v", err.Error())
	} else {
		glog.Infof("The NodeUnpublishVolumeRequest is :%v", nodeUnpublishVolumeRequest)
		ctx := context.Background()
		// TODO: error check
		_ = s.redirect(ctx, nodeUnpublishVolumeRequest, func(ctx context.Context, client csi.NodeClient) {
			response, err := client.NodeUnpublishVolume(ctx, &nodeUnpublishVolumeRequest)
			if err != nil {
				glog.Errorf("Failed to reproduce NodeUnpublishVolume with the NodeUnpublishVolumeRequest, err: %v", err.Error())
			} else {
				peerPodVolume.Status = v1alpha1.PeerpodVolumeStatus{
					State: v1alpha1.NodeUnpublishVolumeApplied,
				}
				_, err := s.PeerpodvolumeClient.ConfidentialcontainersV1alpha1().PeerpodVolumes(s.Namespace).UpdateStatus(context.Background(), peerPodVolume, metav1.UpdateOptions{})
				if err != nil {
					glog.Errorf("Error happens while Update PeerpodVolume status to NodeUnpublishVolumeApplied, err: %v", err.Error())
				}
				glog.Infof("The NodeUnpublishVolumeResponse for peer pod is :%v", response)
			}
		})
	}
}

func (s *PodVMNodeService) ReproduceNodeUnstageVolume(peerPodVolume *v1alpha1.PeerpodVolume) {
	glog.Infof("Reproducing nodeUnstageVolumeRequest for peer pod")
	wrapperRequest := peerPodVolume.Spec.WrapperNodeUnstageVolumeReq
	var nodeUnstageVolumeRequest csi.NodeUnstageVolumeRequest
	if err := (&jsonpb.Unmarshaler{}).Unmarshal(bytes.NewReader([]byte(wrapperRequest)), &nodeUnstageVolumeRequest); err != nil {
		glog.Errorf("Failed to convert to NodeUnstageVolumeRequest, err: %v", err.Error())
	} else {
		glog.Infof("The NodeUnstageVolumeRequest is :%v", nodeUnstageVolumeRequest)
		ctx := context.Background()
		// TODO: error check
		_ = s.redirect(ctx, nodeUnstageVolumeRequest, func(ctx context.Context, client csi.NodeClient) {
			response, err := client.NodeUnstageVolume(ctx, &nodeUnstageVolumeRequest)
			if err != nil {
				glog.Errorf("Failed to reproduce NodeUnstageVolume with the NodeUnstageVolumeRequest, err: %v", err.Error())
			} else {
				peerPodVolume.Status = v1alpha1.PeerpodVolumeStatus{
					State: v1alpha1.NodeUnstageVolumeApplied,
				}
				_, err := s.PeerpodvolumeClient.ConfidentialcontainersV1alpha1().PeerpodVolumes(s.Namespace).UpdateStatus(context.Background(), peerPodVolume, metav1.UpdateOptions{})
				if err != nil {
					glog.Errorf("Error happens while Update PeerpodVolume status to NodeUnstageVolumeApplied, err: %v", err.Error())
				}
				glog.Infof("The NodeUnstageVolumeResponse for peer pod is :%v", response)
			}
		})
	}
}

func (s *PodVMNodeService) SyncHandler(peerPodVolume *v1alpha1.PeerpodVolume) {
	if peerPodVolume.Spec.PodName != os.Getenv("POD_NAME") || peerPodVolume.Spec.PodNamespace != os.Getenv("POD_NAME_SPACE") {
		// Only handle the podvm related PeerpodVolume CRD
		glog.Infof("Only handle the PeerpodVolume crd object for POD_NAME:%v, POD_NAME_SPACE:%v", os.Getenv("POD_NAME"), os.Getenv("POD_NAME_SPACE"))
		return
	}
	glog.Infof("syncHandler from podvm nodeService: %v ", peerPodVolume)
	switch peerPodVolume.Status.State {
	case v1alpha1.ControllerPublishVolumeApplied:
		s.ReproduceNodeStageVolume(peerPodVolume)
	case v1alpha1.NodeStageVolumeApplied:
		s.ReproduceNodePublishVolume(peerPodVolume)
	case v1alpha1.NodeUnpublishVolumeCached:
		s.ReproduceNodeUnpublishVolume(peerPodVolume)
	case v1alpha1.NodeUnstageVolumeCached:
		s.ReproduceNodeUnstageVolume(peerPodVolume)
	}
}

func (s *PodVMNodeService) DeleteFunction(peerPodVolume *v1alpha1.PeerpodVolume) {
	glog.Infof("deleteFunction from podvm nodeService: %v ", peerPodVolume)
}
