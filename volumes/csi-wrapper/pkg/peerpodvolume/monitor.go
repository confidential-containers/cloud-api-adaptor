package peerpodvolume

import (
	"context"
	"sync"
	"time"

	peerpodvolumeV1alpha1 "github.com/confidential-containers/cloud-api-adaptor/volumes/csi-wrapper/pkg/apis/peerpodvolume/v1alpha1"
	clientset "github.com/confidential-containers/cloud-api-adaptor/volumes/csi-wrapper/pkg/generated/peerpodvolume/clientset/versioned"
	informers "github.com/confidential-containers/cloud-api-adaptor/volumes/csi-wrapper/pkg/generated/peerpodvolume/informers/externalversions"
)

// var logger = log.New(log.Writer(), "[peerpodvolume/monitor] ", log.LstdFlags|log.Lmsgprefix)

type CsiPodVolumeMonitor interface {
	Start(context.Context) error
	Shutdown() error
	Ready() chan struct{}
}

type csiPodVolumeMonitor struct {
	controller      *PeerpodvolumeController
	informerFactory informers.SharedInformerFactory
	stopCh          chan struct{}
	stopOnce        sync.Once

	readyCh chan struct{}
}

func NewPodVolumeMonitor(
	client *clientset.Clientset,
	namespace string,
	syncFunction func(peerPodVolume *peerpodvolumeV1alpha1.PeerpodVolume),
	deleteFunction func(peerPodVolume *peerpodvolumeV1alpha1.PeerpodVolume),
) (CsiPodVolumeMonitor, error) {

	informerFactory := informers.NewSharedInformerFactory(client, time.Second*30)
	informer := informerFactory.Peerpod().V1alpha1().PeerpodVolumes()

	controller := newPeerpodvolumeController(
		client,
		informer,
		namespace,
		syncFunction,
		deleteFunction,
	)

	m := &csiPodVolumeMonitor{
		controller:      controller,
		informerFactory: informerFactory,
		stopCh:          make(chan struct{}),
		readyCh:         make(chan struct{}),
	}

	return m, nil
}

func (m *csiPodVolumeMonitor) Start(ctx context.Context) error {

	m.informerFactory.Start(m.stopCh)
	go m.controller.Run(2, m.stopCh)

	close(m.readyCh)

	select {
	case <-ctx.Done():
		_ = m.Shutdown() // TODO: error check
	case <-m.stopCh:
	}

	return nil
}

func (m *csiPodVolumeMonitor) Shutdown() error {
	m.stopOnce.Do(func() {
		close(m.stopCh)
	})
	return nil
}

func (m *csiPodVolumeMonitor) Ready() chan struct{} {
	return m.readyCh
}
