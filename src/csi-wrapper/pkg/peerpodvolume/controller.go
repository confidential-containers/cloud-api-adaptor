package peerpodvolume

import (
	"encoding/json"
	"fmt"
	"time"

	peerpodvolumeV1alpha1 "github.com/confidential-containers/cloud-api-adaptor/src/csi-wrapper/pkg/apis/peerpodvolume/v1alpha1"
	clientset "github.com/confidential-containers/cloud-api-adaptor/src/csi-wrapper/pkg/generated/peerpodvolume/clientset/versioned"
	informersv1alpha1 "github.com/confidential-containers/cloud-api-adaptor/src/csi-wrapper/pkg/generated/peerpodvolume/informers/externalversions/peerpodvolume/v1alpha1"
	listers "github.com/confidential-containers/cloud-api-adaptor/src/csi-wrapper/pkg/generated/peerpodvolume/listers/peerpodvolume/v1alpha1"
	"github.com/golang/glog"
	kubeErrors "k8s.io/apimachinery/pkg/api/errors"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

// PeerpodvolumeController is the controller implementation for peerpodvolume resources
type PeerpodvolumeController struct {
	namespace      string
	clientset      clientset.Interface
	lister         listers.PeerpodVolumeLister
	synced         cache.InformerSynced
	queue          workqueue.RateLimitingInterface
	syncFunction   func(peerPodVolume *peerpodvolumeV1alpha1.PeerpodVolume)
	deleteFunction func(peerPodVolume *peerpodvolumeV1alpha1.PeerpodVolume)
}

// newPeerpodvolumeController returns a new sample controller
func newPeerpodvolumeController(
	clientset clientset.Interface,
	informer informersv1alpha1.PeerpodVolumeInformer,
	namespace string,
	syncFunction func(peerPodVolume *peerpodvolumeV1alpha1.PeerpodVolume),
	deleteFunction func(peerPodVolume *peerpodvolumeV1alpha1.PeerpodVolume),
) *PeerpodvolumeController {

	controller := &PeerpodvolumeController{
		namespace:      namespace,
		clientset:      clientset,
		lister:         informer.Lister(),
		synced:         informer.Informer().HasSynced,
		queue:          workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "Peerpodvolumes"),
		syncFunction:   syncFunction,
		deleteFunction: deleteFunction,
	}

	// TODO: error check
	_, _ = informer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: controller.enqueuePeerpodvolume,
		UpdateFunc: func(old, new interface{}) {
			oldPeerpodvolume := old.(*peerpodvolumeV1alpha1.PeerpodVolume)
			newPeerpodvolume := new.(*peerpodvolumeV1alpha1.PeerpodVolume)
			if oldPeerpodvolume.ResourceVersion == newPeerpodvolume.ResourceVersion {
				return
			}
			controller.enqueuePeerpodvolume(new)
		},
		DeleteFunc: controller.handleDeletedPeerpodvolume,
	})

	return controller
}

func (c *PeerpodvolumeController) handleDeletedPeerpodvolume(obj interface{}) {
	var peerPodVolume *peerpodvolumeV1alpha1.PeerpodVolume

	peerPodVolume, ok := obj.(*peerpodvolumeV1alpha1.PeerpodVolume)
	if !ok {
		glog.Infof("Not a Peerpodvolume object: %v", obj)
		return
	}
	volumeID := peerPodVolume.Spec.VolumeID
	glog.Infof("Got deleted csi peer pod volume Info for %s", volumeID)
	// call deleteFunction from node service or controller service
	if c.deleteFunction != nil {
		c.deleteFunction(peerPodVolume)
	}
}

// Run sets up event handlers
func (c *PeerpodvolumeController) Run(threadiness int, stopCh chan struct{}) {
	defer utilruntime.HandleCrash()
	defer c.queue.ShutDown()

	if ok := cache.WaitForCacheSync(stopCh, c.synced); !ok {
		glog.Infof("Failed to wait for caches to sync")
		return
	}
	for i := 0; i < threadiness; i++ {
		go wait.Until(c.runWorker, time.Second, stopCh)
	}

	<-stopCh
	glog.Infof("Shutting down controller")
}

func (c *PeerpodvolumeController) runWorker() {
	for c.processNextWorkItem() {
	}
}

func (c *PeerpodvolumeController) processNextWorkItem() bool {
	key, quit := c.queue.Get()
	if quit {
		return false
	}
	defer c.queue.Done(key)

	err := c.syncHandler(key.(string))
	if err == nil {
		c.queue.Forget(key)
		return true
	}

	utilruntime.HandleError(fmt.Errorf("%v failed with: %w", key, err))
	c.queue.AddRateLimited(key)

	return true
}

func (c *PeerpodvolumeController) syncHandler(key string) error {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)

	if err != nil {
		utilruntime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return nil
	}
	if namespace != c.namespace {
		glog.Infof("Detected a out of scope Peerpodvolume object: %s \n, only handle objects under namespace: %s", key, c.namespace)
		return nil
	}
	peerPodVolume, err := c.lister.PeerpodVolumes(namespace).Get(name)
	if err != nil {
		if kubeErrors.IsNotFound(err) {
			utilruntime.HandleError(fmt.Errorf("peerpodvolume %q in work queue no longer exists", key))
			return nil
		}

		return err
	}

	if len(peerPodVolume.Spec.VolumeID) == 0 {
		return nil
	}

	glog.Infof("Detected a Peerpodvolume object: %s \n", key)
	objJSONString, _ := json.Marshal(peerPodVolume)
	objString := string(objJSONString)
	glog.Infof("Detected Peerpodvolume json.Marshal.string: %s\n", objString)
	// call the syncFunction from node service or controller service
	if c.syncFunction != nil {
		c.syncFunction(peerPodVolume)
	}
	return nil
}

func (c *PeerpodvolumeController) enqueuePeerpodvolume(obj interface{}) {
	var key string
	var err error
	if key, err = cache.MetaNamespaceKeyFunc(obj); err != nil {
		utilruntime.HandleError(err)
		return
	}
	c.queue.Add(key)
}
