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
}

// PeerPodStatus defines the observed state of PeerPod
type PeerPodStatus struct {
	Cleaned bool `json:"cleand,omitempty"`
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

func init() {
	SchemeBuilder.Register(&PeerPod{}, &PeerPodList{})
}
