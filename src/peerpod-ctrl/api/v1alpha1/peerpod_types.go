/*
Copyright Confidential Containers Contributors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// PeerPodSpec defines the desired state of PeerPod
type PeerPodSpec struct {
	CloudProvider string `json:"cloudProvider,omitempty"`
	InstanceID    string `json:"instanceID,omitempty"`

	// Pool-specific fields for VM pool management
	// A non-empty PoolAllocationID indicates this is a pooled VM
	PoolAllocationID string `json:"poolAllocationID,omitempty"`
	PoolType         string `json:"poolType,omitempty"`
}

// PeerPodStatus defines the observed state of PeerPod
type PeerPodStatus struct {
	Cleaned bool `json:"cleaned,omitempty"`

	// Pool-specific status fields
	PoolAvailable bool         `json:"poolAvailable,omitempty"`
	AllocatedAt   *metav1.Time `json:"allocatedAt,omitempty"`
	ReturnedAt    *metav1.Time `json:"returnedAt,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// PeerPod is the Schema for the peerpods API
type PeerPod struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PeerPodSpec   `json:"spec,omitempty"`
	Status PeerPodStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// PeerPodList contains a list of PeerPod
type PeerPodList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PeerPod `json:"items"`
}

// Helper methods for pool management

// IsPooledVM returns true if this PeerPod represents a pooled VM
// A PeerPod is considered pooled if it has a non-empty PoolAllocationID
func (p *PeerPod) IsPooledVM() bool {
	return p.Spec.PoolAllocationID != ""
}

// GetPoolAllocationID returns the pool allocation ID if this is a pooled VM
func (p *PeerPod) GetPoolAllocationID() string {
	return p.Spec.PoolAllocationID
}

// GetPoolType returns the pool type if this is a pooled VM
func (p *PeerPod) GetPoolType() string {
	return p.Spec.PoolType
}

// SetPoolMetadata sets pool-related metadata for a PeerPod
func (p *PeerPod) SetPoolMetadata(allocationID, poolType string) {
	p.Spec.PoolAllocationID = allocationID
	p.Spec.PoolType = poolType
}

// SetPoolAllocated marks the PeerPod as allocated from the pool
func (p *PeerPod) SetPoolAllocated() {
	now := metav1.Now()
	p.Status.AllocatedAt = &now
	p.Status.PoolAvailable = false
}

// SetPoolReturned marks the PeerPod as returned to the pool
func (p *PeerPod) SetPoolReturned() {
	now := metav1.Now()
	p.Status.ReturnedAt = &now
	p.Status.PoolAvailable = true
}

func init() {
	SchemeBuilder.Register(&PeerPod{}, &PeerPodList{})
}
