// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package k8sops

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"sync"

	peerPodV1alpha1 "github.com/confidential-containers/cloud-api-adaptor/src/peerpod-ctrl/api/v1alpha1"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
)

var logger = log.New(log.Writer(), "[util/k8sops] ", log.LstdFlags|log.Lmsgprefix)
var ppFinalizer string = "peer.pod/finalizer"

type PeerPodService struct {
	client        *kubernetes.Clientset
	uclient       *rest.RESTClient // use generated client instead
	cloudProvider string
	podToPP       map[string]string // map Pod UID to owned PeerPod Name
	mutex         sync.Mutex
}

func NewPeerPodService() (*PeerPodService, error) {
	cloudProvider := os.Getenv("CLOUD_PROVIDER") // TODO: don't get from env var directly
	if cloudProvider == "" {
		return nil, errors.New("NewPeerPodService: failed to get cloudProvider")
	}

	config, err := getKubeConfig()
	if err != nil {
		return nil, fmt.Errorf("NewPeerPodService: failed to get config: %w", err)
	}

	clientset, err := getClient(config)
	if err != nil {
		return nil, fmt.Errorf("NewPeerPodService: failed to create clientset: %w", err)
	}
	config.ContentConfig.GroupVersion = &schema.GroupVersion{Group: peerPodV1alpha1.GroupVersion.Group, Version: peerPodV1alpha1.GroupVersion.Version}
	config.NegotiatedSerializer = scheme.Codecs.WithoutConversion()
	config.APIPath = "/apis"
	restClient, err := rest.UnversionedRESTClientFor(config)
	if err != nil {
		return nil, fmt.Errorf("NewPeerPodService: failed to create UnversionedRESTClient: %s", err)
	}
	logger.Printf("initialized PeerPodService")
	return &PeerPodService{client: clientset, uclient: restClient, cloudProvider: cloudProvider, podToPP: make(map[string]string)}, nil
}

func (s *PeerPodService) newPeerPod(pod *v1.Pod, instanceId string) *peerPodV1alpha1.PeerPod {
	pp := peerPodV1alpha1.PeerPod{
		TypeMeta: metav1.TypeMeta{
			APIVersion: peerPodV1alpha1.GroupVersion.Group + "/" + peerPodV1alpha1.GroupVersion.Version,
			Kind:       "PeerPod",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:       pod.Name + "-resource-" + rand.String(5),
			Namespace:  pod.Namespace,
			Finalizers: []string{ppFinalizer},
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(pod, v1.SchemeGroupVersion.WithKind("Pod")),
			},
		},
		Spec: peerPodV1alpha1.PeerPodSpec{
			InstanceID:    string(instanceId),
			CloudProvider: s.cloudProvider,
		},
	}
	*pp.ObjectMeta.OwnerReferences[0].BlockOwnerDeletion = true // needed?
	return &pp
}

func (s *PeerPodService) getPod(podname string, podns string) (*v1.Pod, error) {
	pod, err := s.client.CoreV1().Pods(podns).Get(context.TODO(), podname, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	return pod, nil
}

// make the pod an owner of a PeerPod
func (s *PeerPodService) OwnPeerPod(podname string, podns string, instanceID string) error {
	pod, err := s.getPod(podname, podns)
	if err != nil {
		return err
	}
	pp := s.newPeerPod(pod, instanceID)
	result := peerPodV1alpha1.PeerPod{}
	err = s.uclient.Post().Namespace(pod.Namespace).Resource("peerPods").Body(pp).Do(context.TODO()).Into(&result)
	if err != nil {
		return err
	}
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.podToPP[string(pod.UID)] = string(pp.Name)
	logger.Printf("%s is now owning a PeerPod object", podname)
	return nil
}

// remove finalizer from PeerPod
func (s *PeerPodService) ReleasePeerPod(podname string, podns string, instanceID string) error {
	pod, err := s.getPod(podname, podns)
	if err != nil {
		return err
	}

	s.mutex.Lock()
	defer s.mutex.Unlock()
	ownedPPName, ok := s.podToPP[string(pod.UID)]
	if !ok {
		return errors.New("pod to PeerPod mapping not found")
	}
	result := peerPodV1alpha1.PeerPod{}
	patch := []byte(`[{"op": "remove", "path": "/metadata/finalizers"}]`)
	err = s.uclient.Patch(types.JSONPatchType).Name(ownedPPName).Namespace(podns).Resource("peerPods").Body(patch).Do(context.TODO()).Into(&result)
	if err != nil {
		return err
	}
	delete(s.podToPP, string(pod.UID))
	logger.Printf("%s's owned PeerPod object can now be deleted", podname)
	return nil
}
