/*
Copyright 2021.

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

package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	RaidLinear RaidLevel = "linear"
	Raid0      RaidLevel = "raid0"
	Raid1      RaidLevel = "raid1"
	Raid5      RaidLevel = "raid5"
	Raid6      RaidLevel = "raid6"
)

type RaidLevel string

type LVMControl struct {
	// +optional
	VG string `json:"vg"`
	// +optional
	LVol string `json:"lvol"`
	// +optional
	PVs []LVMControlPV `json:"pvs"`
}

type LVMControlPV struct {
	// +optional
	DevPath string `json:"devPath"`
	// +optional
	VolId EntityIdentity `json:"volId"`
	// +optional
	TargetInfo SpdkTarget `json:"target"`
}

type EntityIdentity struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	UUID      string `json:"uuid"`
}

type Raid struct {
	Level RaidLevel `json:"level"`
	// TODO: other raid params
}

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// AntstorVolumeGroupSpec defines the desired state of AntstorVolumeGroup
type AntstorDataControlSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	UUID string `json:"uuid"`

	TotalSize int64 `json:"totalSize"`

	// TODO: SpdkControl

	// LVMControl uses LVM as IO controller
	// +optional
	LVM *LVMControl `json:"lvm,omitempty"`

	// type of data control
	EngineType PoolMode `json:"engineType"`

	// raid level
	Raid Raid `json:"raid"`

	// +optional
	HostNode NodeInfo `json:"hostNode"`

	// +optional
	TargetNodeId string `json:"targetNodeId"`

	// +optional
	VolumeGroups []EntityIdentity `json:"volumeGroups"`
}

// AntstorDataControlStatus defines the observed state of AntstorDataControl
type AntstorDataControlStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// +optional
	// +kubebuilder:default=creating
	Status VolumeStatus `json:"status,omitempty"`

	// +optional
	CSINodePubParams *CSINodePubParams `json:"csiNodePubParams,omitempty"`

	// +optional
	Message string `json:"message,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="type",type=string,JSONPath=`.spec.engineType`
// +kubebuilder:printcolumn:name="raid",type=string,JSONPath=`.spec.raid.level`
// +kubebuilder:printcolumn:name="host",type=string,JSONPath=`.spec.hostNode.ip`
// +kubebuilder:printcolumn:name="status",type=string,JSONPath=`.status.status`
// +kubebuilder:printcolumn:name="age",type="date",JSONPath=".metadata.creationTimestamp"
// AntstorDataControl is the Schema for the AntstorDataControl API
type AntstorDataControl struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec AntstorDataControlSpec `json:"spec,omitempty"`

	// +optional
	Status AntstorDataControlStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
// AntstorDataControlList contains a list of AntstorDataControl
type AntstorDataControlList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []AntstorDataControl `json:"items"`
}

func init() {
	SchemeBuilder.Register(&AntstorDataControl{}, &AntstorDataControlList{})
}
