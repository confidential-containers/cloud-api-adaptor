// Code generated by lister-gen. DO NOT EDIT.

package v1alpha1

import (
	v1alpha1 "github.com/confidential-containers/cloud-api-adaptor/src/csi-wrapper/pkg/apis/peerpodvolume/v1alpha1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
)

// PeerpodVolumeLister helps list PeerpodVolumes.
// All objects returned here must be treated as read-only.
type PeerpodVolumeLister interface {
	// List lists all PeerpodVolumes in the indexer.
	// Objects returned here must be treated as read-only.
	List(selector labels.Selector) (ret []*v1alpha1.PeerpodVolume, err error)
	// PeerpodVolumes returns an object that can list and get PeerpodVolumes.
	PeerpodVolumes(namespace string) PeerpodVolumeNamespaceLister
	PeerpodVolumeListerExpansion
}

// peerpodVolumeLister implements the PeerpodVolumeLister interface.
type peerpodVolumeLister struct {
	indexer cache.Indexer
}

// NewPeerpodVolumeLister returns a new PeerpodVolumeLister.
func NewPeerpodVolumeLister(indexer cache.Indexer) PeerpodVolumeLister {
	return &peerpodVolumeLister{indexer: indexer}
}

// List lists all PeerpodVolumes in the indexer.
func (s *peerpodVolumeLister) List(selector labels.Selector) (ret []*v1alpha1.PeerpodVolume, err error) {
	err = cache.ListAll(s.indexer, selector, func(m interface{}) {
		ret = append(ret, m.(*v1alpha1.PeerpodVolume))
	})
	return ret, err
}

// PeerpodVolumes returns an object that can list and get PeerpodVolumes.
func (s *peerpodVolumeLister) PeerpodVolumes(namespace string) PeerpodVolumeNamespaceLister {
	return peerpodVolumeNamespaceLister{indexer: s.indexer, namespace: namespace}
}

// PeerpodVolumeNamespaceLister helps list and get PeerpodVolumes.
// All objects returned here must be treated as read-only.
type PeerpodVolumeNamespaceLister interface {
	// List lists all PeerpodVolumes in the indexer for a given namespace.
	// Objects returned here must be treated as read-only.
	List(selector labels.Selector) (ret []*v1alpha1.PeerpodVolume, err error)
	// Get retrieves the PeerpodVolume from the indexer for a given namespace and name.
	// Objects returned here must be treated as read-only.
	Get(name string) (*v1alpha1.PeerpodVolume, error)
	PeerpodVolumeNamespaceListerExpansion
}

// peerpodVolumeNamespaceLister implements the PeerpodVolumeNamespaceLister
// interface.
type peerpodVolumeNamespaceLister struct {
	indexer   cache.Indexer
	namespace string
}

// List lists all PeerpodVolumes in the indexer for a given namespace.
func (s peerpodVolumeNamespaceLister) List(selector labels.Selector) (ret []*v1alpha1.PeerpodVolume, err error) {
	err = cache.ListAllByNamespace(s.indexer, s.namespace, selector, func(m interface{}) {
		ret = append(ret, m.(*v1alpha1.PeerpodVolume))
	})
	return ret, err
}

// Get retrieves the PeerpodVolume from the indexer for a given namespace and name.
func (s peerpodVolumeNamespaceLister) Get(name string) (*v1alpha1.PeerpodVolume, error) {
	obj, exists, err := s.indexer.GetByKey(s.namespace + "/" + name)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.NewNotFound(v1alpha1.Resource("peerpodvolume"), name)
	}
	return obj.(*v1alpha1.PeerpodVolume), nil
}
