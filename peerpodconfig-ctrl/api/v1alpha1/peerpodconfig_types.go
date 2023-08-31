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

// PeerPodConfigSpec defines the desired state of PeerPodConfig
type PeerPodConfigSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// InstanceType describes the name of the instance type of the chosen cloud provider
	InstanceType string `json:"instanceType,omitempty"`

	// Limit is the max number of peer pods per node. This is exposed as extended resource on the node
	Limit string `json:"limit,omitempty"`

	// CloudSecretName is the name of the secret that holds the credentials for the cloud provider
	// +kubebuilder:default:=peer-pods-secret
	CloudSecretName string `json:"cloudSecretName"`

	// NodeSelector selects the nodes on which to run the cloud-api-adaptor pods
	// +optional
	NodeSelector map[string]string `json:"nodeSelector"`

	// ConfigMapName is the name of the configmap that holds cloud provider specific environment Variables
	// +kubebuilder:default:=peer-pods-cm
	ConfigMapName string `json:"configMapName"`
}

// PeerPodConfigStatus defines the observed state of PeerPodConfig
type PeerPodConfigStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// SetupCompleted is set to true when all components have been deployed/created
	SetupCompleted bool `json:"setupCompleted,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// PeerPodConfig is the Schema for the peerpodconfigs API
type PeerPodConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PeerPodConfigSpec   `json:"spec,omitempty"`
	Status PeerPodConfigStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// PeerPodConfigList contains a list of PeerPodConfig
type PeerPodConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PeerPodConfig `json:"items"`
}

func init() {
	SchemeBuilder.Register(&PeerPodConfig{}, &PeerPodConfigList{})
}
