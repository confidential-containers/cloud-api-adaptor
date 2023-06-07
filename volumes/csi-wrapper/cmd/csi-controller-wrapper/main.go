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

	flag.StringVar(&cfg.Endpoint, "endpoint", "/csi/csi-controller-wrapper.sock", "Wrapper CSI Controller service endpoint path")
	flag.StringVar(&cfg.Namespace, "namespace", "default", "The namespace where the peer pod volume crd object will be created")
	flag.StringVar(&cfg.TargetEndpoint, "target-endpoint", "/csi/csi.sock", "Target CSI Controller service endpoint path")

	flag.Parse()

	if cfg.Endpoint == "" {
		glog.Fatalf("No wrapper csi endpoint provided")
	}

	if cfg.TargetEndpoint == "" {
		glog.Fatalf("No target csi endpoint provided")
	}

	glog.Infof("Endpoint: %s ", cfg.Endpoint)
	glog.Infof("TargetEndpoint: %s", cfg.TargetEndpoint)

	k8sconfig, err := clientcmd.BuildConfigFromFlags("", "")
	if err != nil {
		glog.Fatalf("Build kubeconfig failed: %v", err)
	}
	peerPodVolumeClient := peerpodvolumeV1alpha1.NewForConfigOrDie(k8sconfig)

	identityService := wrapper.NewIdentityService(cfg.TargetEndpoint)
	controllerService := wrapper.NewControllerService(cfg.TargetEndpoint, cfg.Namespace, peerPodVolumeClient)

	podVolumeMonitor, err := peerpodvolume.NewPodVolumeMonitor(
		peerPodVolumeClient,
		cfg.Namespace,
		controllerService.SyncHandler,
		controllerService.DeleteFunction,
	)
	if err != nil {
		glog.Fatalf("Initialize peer pod Volume Controller monitor failed: %v", err)
	}
	go func() {
		if err := podVolumeMonitor.Start(context.Background()); err != nil {
			glog.Fatalf("Running peer pod Volume Controller monitor failed: %v", err)
		}
	}()

	if err := wrapper.Run(cfg.Endpoint, identityService, controllerService, nil); err != nil {
		glog.Fatalf("Failed to run csi controller plugin wrapper: %s", err.Error())
	}
}
