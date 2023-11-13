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

// TorChainSpec defines the desired state of TorChain
// ожидаемое состояние кластера (спецификация)
type TorChainSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// number node of chain
	NumberNode int `json:"numberNode,omitempty"` // 1 or 2 or 3
	// typee of VPN node
	TypeNode string `json:"typeNode,omitempty"` //open_vpn or wireguard
	// unigue tor chain name
	NameTorChain string `json:"nameTorChain,omitempty"`
	// counter of switched to enother VPN Server
	SwitchServer int `json:"switchServer,omitempty"`
	// environments:
	// ip gateway
	// GateWay string `json:"gateway,omitempty"`
	// file name VPN config
	VpnFileConfig string `json:"vpnFileConfig,omitempty"`
	// volumeMounts:
	// path to TMP dir
	TmpDir string `json:"tmpDir,omitempty"`
	// interfaces:
	// input traffic
	InInterface string `json:"inInterface,omitempty"`
	// output traffic
	OutInterface string `json:"outInterface,omitempty"`
	// VpnDirConfig string `json:"vpnDirConfig,omitempty"`
	// image VPN client
	Image string `json:"image,omitempty"`
	// nodeSelector
	NameK8sNode string `json:"nameK8sNode,omitempty"`
}

// TorChainStatus defines the observed state of TorChain
// текущее состояние кластера
type TorChainStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// All tor nodes are prepared and ready
	Connected bool `json:"connected"`
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
