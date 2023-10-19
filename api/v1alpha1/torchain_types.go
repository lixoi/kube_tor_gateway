/*
Copyright 2023 Lixoi.

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

// my custom struct
type TorChainKeepAlive struct {
	// Keep alive is enabled
	Enabled bool `json:"enabled,omitempty"`
	// Keep alive messages interval in seconds
	Interval int `json:"interval,omitempty"`
	// Number of Node
}

// TorChainSpec defines the desired state of TorChain
// ожидаемое состояние кластера (спецификация)
type TorChainSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Foo is an example field of TorChain. Edit torchain_types.go to remove/update
	//Foo string `json:"foo,omitempty"`
	// Deployments count
	Deployments int `json:"deployments,omitempty"`
	// Keepalive configuration
	Keepalive *TorChainKeepAlive `json:"keepalive,omitempty"`
	// Lengh of chain
	LengthChain int `json:"lengthchain,omitempty"`
}

// TorChainStatus defines the observed state of TorChain
// текущее состояние кластера
type TorChainStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// All tor nodes are prepared and ready
	Deployed bool `json:"deployed"`
	// How many tor nodes isn't available
	BrokenNodes int `json:"brokenNodes"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// TorChain is the Schema for the torchains API
type TorChain struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   TorChainSpec   `json:"spec,omitempty"`
	Status TorChainStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// TorChainList contains a list of TorChain
type TorChainList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []TorChain `json:"items"`
}

func init() {
	SchemeBuilder.Register(&TorChain{}, &TorChainList{})
}
