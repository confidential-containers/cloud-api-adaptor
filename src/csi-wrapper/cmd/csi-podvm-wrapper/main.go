// Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"flag"
	"os"

	"github.com/confidential-containers/cloud-api-adaptor/src/csi-wrapper/pkg/apis/peerpodvolume/v1alpha1"
	"github.com/confidential-containers/cloud-api-adaptor/src/csi-wrapper/pkg/config"
	peerpodvolumeV1alpha1 "github.com/confidential-containers/cloud-api-adaptor/src/csi-wrapper/pkg/generated/peerpodvolume/clientset/versioned"
	"github.com/confidential-containers/cloud-api-adaptor/src/csi-wrapper/pkg/peerpodvolume"
	"github.com/confidential-containers/cloud-api-adaptor/src/csi-wrapper/pkg/wrapper"
	"github.com/golang/glog"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/clientcmd"
)

func init() {
	_ = flag.Set("logtostderr", "true") // TODO: error check
}

func main() {
	cfg := config.Endpoints{}

	flag.StringVar(&cfg.Endpoint, "endpoint", "/csi/csi-podvm-wrapper.sock", "Wrapper CSI Node service endpoint path")
	flag.StringVar(&cfg.Namespace, "namespace", "default", "The namespace where the peer pod volume crd object will be created")
	flag.StringVar(&cfg.TargetEndpoint, "target-endpoint", "/csi/csi.sock", "Target CSI Node service endpoint path")

	flag.Parse()

	glog.Infof("Endpoint: %v ", cfg.Endpoint)
	glog.Infof("TargetEndpoint: %s", cfg.TargetEndpoint)
	podName := os.Getenv("POD_NAME")
	podNamespace := os.Getenv("POD_NAME_SPACE")
	podUID := os.Getenv("POD_UID")
	podNodeName := os.Getenv("POD_NODE_NAME")

	glog.Infof("POD_NAME: %v ", podName)
	glog.Infof("POD_NAME_SPACE: %s", podNamespace)
	glog.Infof("POD_UID: %v ", podUID)
	glog.Infof("POD_NODE_NAME: %v ", podNodeName)

	k8sconfig, err := clientcmd.BuildConfigFromFlags("", "")
	if err != nil {
		glog.Fatalf("Build kubeconfig failed: %v", err)
	}
	peerPodVolumeClient := peerpodvolumeV1alpha1.NewForConfigOrDie(k8sconfig)

	identityService := wrapper.NewIdentityService(cfg.TargetEndpoint)
	podvmService := wrapper.NewPodVMNodeService(cfg.TargetEndpoint, cfg.Namespace, peerPodVolumeClient)

	podVolumeMonitor, err := peerpodvolume.NewPodVolumeMonitor(
		peerPodVolumeClient,
		cfg.Namespace,
		podvmService.SyncHandler,
		podvmService.DeleteFunction,
	)
	if err != nil {
		glog.Fatalf("Initialize peer pod Volume Node monitor failed: %v", err)
	}
	go func() {
		if err := podVolumeMonitor.Start(context.Background()); err != nil {
			glog.Fatalf("Running peer pod Volume Node monitor failed: %v", err)
		}
	}()

	labelSelector := labels.SelectorFromSet(map[string]string{"podUid": string(podUID)})
	options := metav1.ListOptions{
		LabelSelector: labelSelector.String(),
	}
	peerpodVolumes, err := peerPodVolumeClient.ConfidentialcontainersV1alpha1().PeerpodVolumes(cfg.Namespace).List(context.Background(), options)
	if err != nil {
		glog.Fatalf("Failed to get peerpodVolume crd object by podUid: %v, err: %v", podUID, err)
	}
	glog.Infof("peerpodVolume crd object number is: %v ", len(peerpodVolumes.Items))
	for idx, savedPeerpodvolume := range peerpodVolumes.Items {
		glog.Infof("Index of peerpodVolumes.Items: %v ", idx)
		glog.Infof("peerpodVolumes detail: %v ", savedPeerpodvolume)
		savedPeerpodvolume.Spec.PodName = podName
		savedPeerpodvolume.Spec.PodNamespace = podNamespace
		savedPeerpodvolume.Spec.NodeName = podNodeName
		savedPeerpodvolume.Labels["podName"] = podName
		savedPeerpodvolume.Labels["podNamespace"] = podNamespace
		savedPeerpodvolume.Labels["podNodeName"] = podNodeName
		updatedPeerpodvolume, err := peerPodVolumeClient.ConfidentialcontainersV1alpha1().PeerpodVolumes(cfg.Namespace).Update(context.Background(), &savedPeerpodvolume, metav1.UpdateOptions{})
		if err != nil {
			glog.Fatalf("Error happens while Update podName and podNamespace to PeerpodVolume, err: %v", err.Error())
		}
		updatedPeerpodvolume.Status = v1alpha1.PeerpodVolumeStatus{
			State: v1alpha1.PeerPodVSIRunning,
		}
		_, err = peerPodVolumeClient.ConfidentialcontainersV1alpha1().PeerpodVolumes(cfg.Namespace).UpdateStatus(context.Background(), updatedPeerpodvolume, metav1.UpdateOptions{})
		if err != nil {
			glog.Fatalf("Error happens while Update PeerpodVolume status to PeerPodVSIRunning, err: %v", err.Error())
		}
	}
	if err := wrapper.Run(cfg.Endpoint, identityService, nil, podvmService); err != nil {
		glog.Fatalf("Failed to run csi podvm plugin wrapper: %s", err.Error())
	}
}
