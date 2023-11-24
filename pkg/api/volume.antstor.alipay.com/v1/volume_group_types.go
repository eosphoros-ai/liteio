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
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// Label key
	DataControlNameKey = "obnvmf/data-control-name"
)

const (
	Asymmetric SymmetryValue = "Asymmetric"
	Symmetric  SymmetryValue = "Symmetric"

	// StragetyBestFit searches smallest number of nodes to provide volumes
	StragetyBestFit = "BestFit"
	// StragetyNonEmptyBestFit filters out empty node, and searches smallest number of nodes to provide volumes
	StragetyNonEmptyBestFit = "NonEmptyBestFit"
)

type SymmetryValue string

type IntRange struct {
	// +optional
	Min int `json:"min,omitempty"`
	// +optional
	Max int `json:"max,omitempty"`
}

type QuantityRange struct {
	// +optional
	Min resource.Quantity `json:"min,omitempty"`
	// +optional
	Max resource.Quantity `json:"max,omitempty"`
}

type DesiredVolumeSpec struct {
	// Annotatioins for volumes
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`

	// Labels for volumes
	// +optional
	Labels map[string]string `json:"labels,omitempty"`

	// +optional
	DesiredCount int `json:"desiredCount,omitempty"`

	// +optional
	CountRange IntRange `json:"countRange,omitempty"`

	// +optional
	SizeRange QuantityRange `json:"sizeRange,omitempty"`

	SizeSymmetry SymmetryValue `json:"sizeSymmetry"`
}

type VolumeGroupStrategy struct {
	// +optional
	Name string `json:"name"`

	// EmptyThreashold defines the usage percentage of storagepool being considered empty.
	// +optional
	EmptyThreasholdPct int `json:"emptyThreasholdPct"`

	// +optional
	AllowEmptyNode bool `json:"allowEmptyNode"`
}

type VolumeMeta struct {
	VolId          EntityIdentity `json:"volId"`
	TargetNodeName string         `json:"targetNodeName"`
	Size           int64          `json:"size"`
}

type VolumeTargetStatus struct {
	// UUID of volume
	UUID string `json:"uuid"`

	// +optional
	SpdkTarget *SpdkTarget `json:"spdkTarget,omitempty"`

	// status of volume
	// +optional
	// +kubebuilder:default=creating
	Status VolumeStatus `json:"status,omitempty"`

	// message of volume
	// +optional
	Message string `json:"message"`
}

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// AntstorVolumeGroupSpec defines the desired state of AntstorVolumeGroup
type AntstorVolumeGroupSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// ID is uuid generated by controller for each volume
	Uuid string `json:"uuid,omitempty"`

	// TotalSize in bytes
	TotalSize int64 `json:"totalSize"`

	// DesiredVolumeSpec contains specifications of volumes
	// +optional
	DesiredVolumeSpec DesiredVolumeSpec `json:"desiredVolumeSpec,omitempty"`

	// Stragety of scheduling volumes
	// +optional
	Stragety VolumeGroupStrategy `json:"stragety"`

	// Volumes in the VolumeGroup
	// +optional
	Volumes []VolumeMeta `json:"volumes,omitempty"`
}

// AntstorVolumeGroupStatus defines the observed state of AntstorVolumeGroup
type AntstorVolumeGroupStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// +optional
	VolumeStatus []VolumeTargetStatus `json:"volumeStatus,omitempty"`

	// +optional
	// +kubebuilder:default=creating
	Status VolumeStatus `json:"status,omitempty"`

	// +optional
	Message string `json:"message,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="symmetry",type=string,JSONPath=`.spec.desiredVolumeSpec.sizeSymmetry`
// +kubebuilder:printcolumn:name="size",type=integer,JSONPath=`.spec.totalSize`
// +kubebuilder:printcolumn:name="status",type=string,JSONPath=`.status.status`
// +kubebuilder:printcolumn:name="age",type="date",JSONPath=".metadata.creationTimestamp"
// AntstorVolumeGroup is the Schema for the AntstorVolumeGroup API
type AntstorVolumeGroup struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec AntstorVolumeGroupSpec `json:"spec,omitempty"`

	// +optional
	Status AntstorVolumeGroupStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
// AntstorVolumeGroupList contains a list of AntstorVolumeGroup
type AntstorVolumeGroupList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []AntstorVolumeGroup `json:"items"`
}

func init() {
	SchemeBuilder.Register(&AntstorVolumeGroup{}, &AntstorVolumeGroupList{})
}
