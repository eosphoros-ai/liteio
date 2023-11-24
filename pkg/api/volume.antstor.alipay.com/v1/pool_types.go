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
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// StorageClass Provisioner name
	StorageClassProvisioner = "antstor.csi.alipay.com"
	// StorageClass Parameters key
	StorageClassParamPositionAdvice = "positionAdvice"
	// PVC Annotation key
	PVCAnnotationSnapshotReservedSize = "obnvmf/snapshot-reserved-bytes"
	// tgt spdk version Annotatioin key
	AnnotationTgtSpdkVersion = "obnvmf/tgt-version"
	// hostnqn Annotation key
	AnnotationHostNQN = "obnvmf/hostnqn"
)

const (
	DefaultNamespace = "obnvmf"

	PoolModeKernelLVM   PoolMode = "KernelLVM"
	PoolModeSpdkLVStore PoolMode = "SpdkLVStore"

	PoolConditionSpkdHealth PoolConditionType = "Spdk"
	PoolConditionLvmHealth  PoolConditionType = "Lvm"
	PoolConditionKubeNode   PoolConditionType = "KubeNode"

	KubeNodeMsgNcOffline = "NC_OFFLINE"

	StatusOK    ConditionStatus = "OK"
	StatusError ConditionStatus = "Error"

	ResourceDiskPoolByte corev1.ResourceName = corev1.ResourceStorage
	ResourceVolumesCount corev1.ResourceName = "volumes"
	// ResourceLvmFreeByte indicates the real free bytes left in the VG
	// ResourceLvmFreeByte corev1.ResourceName = "storage/vg-free"
	// ResourceSchedFreeByte indicates the real free bytes in VG. It is reported by agent.
	// ResourceSchedFreeByte corev1.ResourceName = "sched/vg-free"

	StoragePoolFinalizer = "antstor.alipay.com/storage-pool"
	InStateFinalizer     = "antstor.alipay.com/in-state"

	// KernelLVolFinalizer indicates that there are LVM resources left on Object. Usually for DataControl
	KernelLVolFinalizer = "antstor.alipay.com/lvm"
	// deprecated
	SpdkLvolFinalizer = "antstor.alipay.com/spdk-lvol"

	// LogicVolumeFinalizer is added after LVM or SpdkLvol is created on StoragePool
	LogicVolumeFinalizer = "antstor.alipay.com/logic-volume"

	// SpdkTargetFinalizer is added after Spdk Target is created on StoragePool
	SpdkTargetFinalizer = "antstor.alipay.com/spdk-tgt"

	// if Snapshot lvm is created, then this key is added to Finalizer.
	SnapshotFinalizer = "antstor.alipay.com/snapshot"

	// VolumesFinalizer is added, if VolumeGroup owns volumes.
	VolumesFinalizer = "antstor.alipay.com/volumes"

	// update local storage in node capacity
	// PoolEventSyncNodeLocalStorageKey = "obnvmf/event-node-local-storage"

	// PoolLocalStorageBytesKey value represents the capacity of local storage
	PoolLocalStorageBytesKey = "obnvmf/local-storage-bytes"

	PoolLabelsNodeSnKey = "obnvmf/node-sn"

	// static local storage
	// PoolStaticLocalStoragePercentageKey = "obnvmf/static-local-storage-pct"
	// PoolStaticLocalStorageSizeKey       = "obnvmf/static-local-storage-size"
	// PoolAdjustLocalStorageTimestampKey  = "obnvmf/adjust-local-storage-ts"

	// pool scheduling status
	PoolSchedulingStatusLabelKey = "obnvmf/pool-scheduling-status"
	PoolSchedulingStatusLocked   = PoolStatusLocked

	// lvol type
	LVLayoutLinear   LVLayout = "linear"
	LVLayoutStriped  LVLayout = "striped"
	LVLayoutThinPool LVLayout = "thin,pool"
)

type PoolConditionType string
type ConditionStatus string
type PoolLabelEvent string
type LVLayout string
type PoolMode string

type KernelLVol struct {
	Name     string `json:"name"`
	VGName   string `json:"vgName"`
	DevPath  string `json:"devPath"`
	SizeByte uint64 `json:"sizeByte"`
	// striped or linear or thin,pool
	LvLayout string `json:"lvLayout"`
}

type KernelLVM struct {
	Name        string `json:"name,omitempty"`
	VgUUID      string `json:"vgUUID,omitempty"`
	Bytes       uint64 `json:"bytes,omitempty"`
	ExtendSize  uint64 `json:"extendSize,omitempty"`
	ExtendCount int    `json:"extendCount,omitempty"`
	PVCount     int    `json:"pvCount,omitempty"`

	// +patchStrategy=merge
	// +optional
	ReservedLVol []KernelLVol `json:"reservedLVol,omitempty" patchStrategy:"merge" patchMergeKey:"name"`
}

type SpdkLVStore struct {
	Name             string `json:"name,omitempty"`
	UUID             string `json:"uuid,omitempty"`
	BaseBdev         string `json:"baseBdev,omitempty"`
	Bytes            uint64 `json:"bytes,omitempty"`
	ClusterSize      int    `json:"clusterSize,omitempty"`
	TotalDataCluster int    `json:"totalDataClusters,omitempty"`
	BlockSize        int    `json:"blockSize,omitempty"`
}

type PoolCondition struct {
	Type    PoolConditionType `json:"type,omitempty"`
	Status  ConditionStatus   `json:"status,omitempty"`
	Message string            `json:"message,omitempty"`
}

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// StoragePoolSpec defines the desired state of StoragePool
type StoragePoolSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// KernelLVM contains info of VG
	// +optional
	KernelLVM KernelLVM `json:"kernelLvm,omitempty"`

	// SpdkLVStore contains info of lvstore in spdk
	// +optional
	SpdkLVStore SpdkLVStore `json:"spdkLVStore,omitempty"`

	// Addresses at which this pool can be accessed
	// +patchStrategy=merge
	// +optional
	Addresses []corev1.NodeAddress `json:"addresses,omitempty" patchStrategy:"merge" patchMergeKey:"address"`

	// NodeInfo contains info of node
	// +optional
	NodeInfo NodeInfo `json:"nodeInfo,omitempty"`
}

// StoragePoolStatus defines the observed state of StoragePool
type StoragePoolStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// +optional
	Capacity corev1.ResourceList `json:"capacity,omitempty"`

	// free space of VG
	// +optional
	VGFreeSize resource.Quantity `json:"vgFreeSize,omitempty"`

	// 子系统的状态，例如 SpkdTarget 状态(json rpc是否正常)， LVM VG 状态(接口调用是否正常)
	// +patchStrategy=merge
	// +optional
	Conditions []PoolCondition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`

	// Status of Pool
	// +optional
	// +kubebuilder:default=ready
	Status PoolStatus `json:"status,omitempty"`

	// +optional
	Message string `json:"message,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="ip",type=string,JSONPath=`.spec.nodeInfo.ip`
// +kubebuilder:printcolumn:name="hostname",type=string,JSONPath=`.spec.nodeInfo.hostname`
// +kubebuilder:printcolumn:name="storage",type=string,JSONPath=`.status.capacity.storage`
// +kubebuilder:printcolumn:name="free",type=string,JSONPath=`.status.vgFreeSize`
// +kubebuilder:printcolumn:name="status",type=string,JSONPath=`.status.status`
// +kubebuilder:printcolumn:name="age",type="date",JSONPath=".metadata.creationTimestamp"
// StoragePool is the Schema for the storagepools API
type StoragePool struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec StoragePoolSpec `json:"spec,omitempty"`

	// +optional
	Status StoragePoolStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
// StoragePoolList contains a list of StoragePool
type StoragePoolList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []StoragePool `json:"items"`
}

func init() {
	SchemeBuilder.Register(&StoragePool{}, &StoragePoolList{})
}
