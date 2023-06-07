// Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"flag"

	"github.com/confidential-containers/cloud-api-adaptor/volumes/csi-wrapper/pkg/config"
	peerpodvolumeV1alpha1 "github.com/confidential-containers/cloud-api-adaptor/volumes/csi-wrapper/pkg/generated/peerpodvolume/clientset/versioned"
	"github.com/confidential-containers/cloud-api-adaptor/volumes/csi-wrapper/pkg/peerpodvolume"
	"github.com/confidential-containers/cloud-api-adaptor/volumes/csi-wrapper/pkg/wrapper"
	"github.com/golang/glog"
	"k8s.io/client-go/tools/clientcmd"
)

func init() {
	_ = flag.Set("logtostderr", "true") // TODO: error check
}

func main() {
	cfg := config.Endpoints{}

	flag.StringVar(&cfg.Endpoint, "endpoint", "/csi/csi-node-wrapper.sock", "Wrapper CSI Node service endpoint path")
	flag.StringVar(&cfg.Namespace, "namespace", "default", "The namespace where the peer pod volume crd object will be created")
	flag.StringVar(&cfg.TargetEndpoint, "target-endpoint", "/csi/csi.sock", "Target CSI Node service endpoint path")
	flag.StringVar(&cfg.VMIDInformationEndpoint, "vm-id-information-endpoint", "/run/peerpod/hypervisor.sock", "Unix domain socket path of VM ID information service")

	flag.Parse()

	glog.Infof("Endpoint: %v ", cfg.Endpoint)
	glog.Infof("TargetEndpoint: %s", cfg.TargetEndpoint)
	glog.Infof("VMIDInformationEndpoint: %s", cfg.VMIDInformationEndpoint)

	k8sconfig, err := clientcmd.BuildConfigFromFlags("", "")
	if err != nil {
		glog.Fatalf("Build kubeconfig failed: %v", err)
	}
	peerPodVolumeClient := peerpodvolumeV1alpha1.NewForConfigOrDie(k8sconfig)

	identityService := wrapper.NewIdentityService(cfg.TargetEndpoint)
	nodeService := wrapper.NewNodeService(cfg.TargetEndpoint, cfg.Namespace, peerPodVolumeClient, cfg.VMIDInformationEndpoint)

	podVolumeMonitor, err := peerpodvolume.NewPodVolumeMonitor(
		peerPodVolumeClient,
		cfg.Namespace,
		nodeService.SyncHandler,
		nodeService.DeleteFunction,
	)
	if err != nil {
		glog.Fatalf("Initialize peer pod Volume Node monitor failed: %v", err)
	}
	go func() {
		if err := podVolumeMonitor.Start(context.Background()); err != nil {
			glog.Fatalf("Running peer pod Volume Node monitor failed: %v", err)
		}
	}()

	if err := wrapper.Run(cfg.Endpoint, identityService, nil, nodeService); err != nil {
		glog.Fatalf("Failed to run csi node plugin wrapper: %s", err.Error())
	}
}
