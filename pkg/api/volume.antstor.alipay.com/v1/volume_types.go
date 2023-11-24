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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// Labels key
	// TargetNodeIdLabelKey specify target node id. It is used for list by label filter
	TargetNodeIdLabelKey = "obnvmf/target-node-id"
	// UuidLabelKey is the key of volume's uuid. It is used for list by label filter
	UuidLabelKey = "obnvmf/vol-uuid"
	// PVTargetNodeNameLabelKey is a key for PV labels, indicating the node id where the PV resides.
	PVTargetNodeNameLabelKey = "obnvmf/pv-target-node"
	// VolumePVNameLabelKey is a label key to record the coresponding PV name
	VolumePVNameLabelKey = "obnvmf/pv-name"
	// ExpansionOriginalSize is a label key, whose value represents the original size (in byte) of a volume in expansion
	ExpansionOriginalSize = "obnvmf/expansion-original-size"
	// VolumeGroupNameLabelKey is a label key of volumegroup name
	VolumeGroupNameLabelKey = "obnvmf/vol-group-name"
)

const (
	// Annotations key
	// key of spdk connecting mode
	SpdkConnectModeKey = "obnvmf/spdk-conn-mode"
	// value indicates that guest kernel directly connect spdk target
	SpdkConnectModeGuestKernelDirect = "guest-direct"

	// specify filesystem type
	FsTypeLabelKey = "obnvmf/fs-type"

	// specify the lv layout
	LvLayoutAnnoKey = "obnvmf/lv-layout"
	// LV's allocated size in VG
	AllocatedSizeAnnoKey = "obnvmf/allocated-bytes"

	// volume label for scheduling
	NodeLabelSelectorKey = "obnvmf/node-label-selector"
	PoolLabelSelectorKey = "obnvmf/pool-label-selector"

	// snapshot reserved space key
	SnapshotReservedSpaceAnnotationKey = "obnvmf/snapshot-reserved-bytes"
	// content source info
	VolumeSourceSnapNameLabelKey      = "obnvmf/volume-source-snap-name"
	VolumeSourceSnapNamespaceLabelKey = "obnvmf/volume-source-snap-ns"

	// key of reservation id
	ReservationIDKey = "obnvmf/reservation-id"
	// key of selected target node
	SelectedTgtNodeKey = "obnvmf/selected-tgt-node"

	K8SAnnoSelectedNode = "volume.kubernetes.io/selected-node"

	// key of VFIOUSER mode(INTRA_HOST or LOCAL_COPY)
	VfiouserModeKey = "obnvmf/volume-vfiouser-mode"
)

const (
	// volume and pod must on the same node
	MustLocal VolumePosition = "MustLocal"
	// no ensurance of the position of the volume
	PreferLocal  VolumePosition = "PreferLocal"
	PreferRemote VolumePosition = "PreferRemote"
	// volume and pod must not on the same node
	MustRemote VolumePosition = "MustRemote"
	// no preference for the position of volume
	NoPreference VolumePosition = ""
)

const (
	// index key of volume
	IndexKeyUUID         = ".spec.uuid"
	IndexKeyTargetNodeID = ".spec.targetNodeId"

	StoragePoolTypeKernelVGroup StoragePoolType = "KernelVGroup"
	StoragePoolTypeSpdkLVStore  StoragePoolType = "SpdkLVStore"

	VolumeTypeKernelLVol VolumeType = "KernelLVol"
	VolumeTypeSpdkLVol   VolumeType = "SpdkLVol"
	VolumeTypeFlexible   VolumeType = "Flexible"

	PoolStatusReady PoolStatus = "ready"
	// 人工锁定调度
	PoolStatusLocked PoolStatus = "locked"
	// 正常下线
	PoolStatusOffline PoolStatus = "offline"
	// 意外情况, agent心跳停止
	PoolStatusUnknown PoolStatus = "unknown"

	// vg, target 全部正常创建后的状态
	VolumeStatusCreating VolumeStatus = "creating"
	VolumeStatusReady    VolumeStatus = "ready"
	VolumeStatusDeleted  VolumeStatus = "deleted"

	PendingPhase PhaseType = "Pending"
	ReadyPhase   PhaseType = "Ready"

	// data-holder key for volume
	VolumeDataHolderKey = "antstor.csi.alipay.com/data-holder"

	// volume context key
	VolumeContextKeyPodName = "csi.storage.k8s.io/pod.name"
	VolumeContextKeyPodNS   = "csi.storage.k8s.io/pod.namespace"
	VolumeContextKeyPvcName = "csi.storage.k8s.io/pvc-name"
	VolumeContextKeyPvcNS   = "csi.storage.k8s.io/pvc-namespace"
)

// +kubebuilder:validation:Enum=KernelVGroup;SpdkLVStore
type StoragePoolType string

// +kubebuilder:validation:Enum=Flexible;KernelLVol;SpdkLVol
type VolumeType string

// +kubebuilder:validation:Enum=creating;ready;deleted
type VolumeStatus string

// +kubebuilder:validation:Enum=MustLocal;PreferLocal;PreferRemote;MustRemote;""
type VolumePosition string

// +kubebuilder:validation:Enum=locked;ready;offline;unknown
type PoolStatus string

// +kubebuilder:validation:Enum=Pending;Ready
type PhaseType string

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

type NodeInfo struct {
	// +optional
	ID string `json:"id"`
	// +optional
	IP string `json:"ip"`
	// +optional
	Hostname string `json:"hostname"`
	// +optional
	Labels map[string]string `json:"labels,omitempty"`
}

type SpdkTarget struct {
	BdevName  string `json:"bdevName"`
	SubsysNQN string `json:"subsysNqn"`
	SvcID     string `json:"svcID"`
	SerialNum string `json:"sn"`
	TransType string `json:"transType"`
	NSUUID    string `json:"nsUuid"`
	Address   string `json:"address"`
	AddrFam   string `json:"addrFam"`
}

type KernelLvol struct {
	Name    string `json:"name"`
	DevPath string `json:"devPath"`
}

type CSINodePubParams struct {
	StagingTargetPath string `json:"stagingTargetPath"`
	TargetPath        string `json:"targetPath"`
	// volume_context in NodePublishVolume request
	CSIVolumeContext map[string]string `json:"volumeContext,omitempty"`
}

type HostAttachment struct {
	// +optional
	HostDevPath string `json:"hostDevPath,omitempty"`
}

type SpdkLvol struct {
	Name    string `json:"name"`
	LvsName string `json:"lvsName"`
	Thin    bool   `json:"thin,omitempty"`
}

// AntstorVolumeSpec defines the desired state of AntstorVolume
type AntstorVolumeSpec struct {
	// ID is uuid generated by controller for each volume
	Uuid string `json:"uuid,omitempty"`

	// +optional
	// +kubebuilder:default=Flexible
	Type VolumeType `json:"type,omitempty"`

	// SizeByte is size of volume
	SizeByte uint64 `json:"sizeByte"`

	// +optional
	PositionAdvice VolumePosition `json:"positionAdvice,omitempty"`

	// Specify volume is solid or thin
	// +optional
	//+kubebuilder:default=false
	IsThin bool `json:"isThin,omitempty"`

	// +optional
	// StopReconcile is true, reconcile will not process this volume
	StopReconcile bool `json:"stopReconcile,omitempty"`

	// NodeAffinity
	// +optional
	// +nullable
	NodeAffinity *corev1.NodeAffinity `json:"nodeAffinity,omitempty"`

	// PoolAffinity
	// +optional
	// +nullable
	PoolAffinity *corev1.NodeAffinity `json:"poolAffinity,omitempty"`

	// +optional
	TargetNodeId string `json:"targetNodeId"`

	// +optional
	// +nullable
	HostNode *NodeInfo `json:"hostNode,omitempty"`

	// +optional
	// +nullable
	KernelLvol *KernelLvol `json:"kernelLvol,omitempty"`

	// +optional
	// +nullable
	SpdkLvol *SpdkLvol `json:"spdkLvol,omitempty"`

	// +optional
	// +nullable
	SpdkTarget *SpdkTarget `json:"spdkTarget,omitempty"`
}

// AntstorVolumeStatus defines the observed state of AntstorVolume
type AntstorVolumeStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// +optional
	// +kubebuilder:default=creating
	Status VolumeStatus `json:"status"`

	// +optional
	CSINodePubParams *CSINodePubParams `json:"csiNodePubParams,omitempty"`

	// +optional
	HostAttachment *HostAttachment `json:"hostAttachment,omitempty"`

	// +optional
	Message string `json:"msg,omitempty"`
}

/*
if enable status subresource, kubectl will not be able to edit status manually.

https://github.com/kubernetes/kubectl/issues/564
Design Proposal: https://github.com/kubernetes/community/blob/master/contributors/design-proposals/api-machinery/customresources-subresources.md#status-behavior
If the /status subresource is enabled, the following behaviors change:
The main resource endpoint will ignore all changes in the status subpath. (note: it will not reject requests which try to change the status, following the existing semantics of other resources).

Improvement:
https://github.com/kubernetes/enhancements/tree/master/keps/sig-cli/2590-kubectl-subresource
*/

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="uuid",type=string,JSONPath=`.spec.uuid`
// +kubebuilder:printcolumn:name="size",type=integer,JSONPath=`.spec.sizeByte`
// +kubebuilder:printcolumn:name="targetId",type=string,JSONPath=`.spec.targetNodeId`
// +kubebuilder:printcolumn:name="host_ip",type=string,JSONPath=`.spec.hostNode.ip`
// +kubebuilder:printcolumn:name="status",type=string,JSONPath=`.status.status`
// +kubebuilder:printcolumn:name="age",type="date",JSONPath=".metadata.creationTimestamp"
// AntstorVolume is the Schema for the antstorvolumes API
type AntstorVolume struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec AntstorVolumeSpec `json:"spec,omitempty"`

	Status AntstorVolumeStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
// AntstorVolumeList contains a list of AntstorVolume
type AntstorVolumeList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []AntstorVolume `json:"items"`
}

func init() {
	SchemeBuilder.Register(&AntstorVolume{}, &AntstorVolumeList{})
}
