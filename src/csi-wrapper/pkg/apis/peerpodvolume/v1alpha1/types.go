// Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// PeerpodVolume is a specification for a PeerpodVolume resource
type PeerpodVolume struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PeerpodVolumeSpec   `json:"spec"`
	Status PeerpodVolumeStatus `json:"status,omitempty"`
}

// PeerpodVolumeSpec is the spec for a PeerpodVolume resource
type PeerpodVolumeSpec struct {
	PodName                           string `json:"podName"`
	PodNamespace                      string `json:"podNamespace"`
	PodUID                            string `json:"podUid"`
	NodeID                            string `json:"nodeID"`
	NodeName                          string `json:"nodeName"`
	VolumeID                          string `json:"volumeID"`
	VolumeName                        string `json:"volumeName"`
	VMID                              string `json:"vmID"`
	VMName                            string `json:"vmName"`
	DevicePath                        string `json:"devicePath"`
	StagingTargetPath                 string `json:"stagingTargetPath"`
	TargetPath                        string `json:"targetPath"`
	WrapperControllerPublishVolumeReq string `json:"wrapperControllerPublishVolumeReq"`
	WrapperControllerPublishVolumeRes string `json:"wrapperControllerPublishVolumeRes"`
	WrapperNodeStageVolumeReq         string `json:"wrapperNodeStageVolumeReq"`
	WrapperNodePublishVolumeReq       string `json:"wrapperNodePublishVolumeReq"`
	WrapperNodeUnpublishVolumeReq     string `json:"wrapperNodeUnpublishVolumeReq"`
	WrapperNodeUnstageVolumeReq       string `json:"wrapperNodeUnstageVolumeReq"`
}

type PeerpodVolumeState string

const (
	// CSI-requests be cached to crd
	ControllerPublishVolumeCached PeerpodVolumeState = "controllerPublishVolumeCached"
	NodeStageVolumeCached         PeerpodVolumeState = "nodeStageVolumeCached"
	NodePublishVolumeCached       PeerpodVolumeState = "nodePublishVolumeCached"
	NodeUnpublishVolumeCached     PeerpodVolumeState = "nodeUnpublishVolumeCached"
	NodeUnstageVolumeCached       PeerpodVolumeState = "nodeUnstageVolumeCached"
	// The VSI instance id MUST be set when update the status to `peerPodVSIIDReady`
	PeerPodVSIIDReady PeerpodVolumeState = "peerPodVSIIDReady"
	// We can get the VSI instance from cloud-api-adaptor podVMInfoService when update the status to `peerPodVSIRunning`
	PeerPodVSIRunning PeerpodVolumeState = "peerPodVSIRunning"
	// The cached ControllerPublishVolume, NodeStageVolume, NodePublishVolume will be reproduced with replace peer-pod instance-id
	// before peer-pod container is created
	ControllerPublishVolumeApplied PeerpodVolumeState = "controllerPublishVolumeApplied"
	NodeStageVolumeApplied         PeerpodVolumeState = "nodeStageVolumeApplied"
	NodePublishVolumeApplied       PeerpodVolumeState = "nodePublishVolumeApplied"
	// csi-wrapper plugins will call original csi-driver to release volumes when peer-pod be deleted
	NodeUnpublishVolumeApplied       PeerpodVolumeState = "nodeUnpublishVolumeApplied"
	NodeUnstageVolumeApplied         PeerpodVolumeState = "nodeUnstageVolumeApplied"
	ControllerUnpublishVolumeApplied PeerpodVolumeState = "controllerUnpublishVolumeApplied"
)

// PeerpodVolumeStatus is the status for a PeerpodVolume resource
type PeerpodVolumeStatus struct {
	State PeerpodVolumeState `json:"state"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// PeerpodVolumeList is a list of PeerpodVolume resources
type PeerpodVolumeList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []PeerpodVolume `json:"items"`
}
