package v1

import (
	// corev1 "k8s.io/api/core/v1"
	// "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	MigrationPhasePending        MigrationPhase = "Pending"
	MigrationPhaseCreatingVolume MigrationPhase = "CreatingVolume"
	MigrationPhaseSetupPipe      MigrationPhase = "SetupPipe"
	MigrationPhaseSyncing        MigrationPhase = "Syncing"
	MigrationPhaseCleaning       MigrationPhase = "Cleaning"
	MigrationPhaseFinished       MigrationPhase = "Finished"

	MigrationStatusError   string = "Error"
	MigrationStatusWorking string = "Working"

	ConnectStatusUnknown      ConnectStatus = "Unknown"
	ConnectStatusConnected    ConnectStatus = "Connected"
	ConnectStatusDisconnected ConnectStatus = "Disconnected"

	ResultStatusUnknown ResultStatus = "Unknown"
	ResultStatusSuccess ResultStatus = "Success"
	ResultStatusError   ResultStatus = "Error"

	MigrationLabelKeyMigrationName    = "migrate.obnvmf/migration-name"
	MigrationLabelKeySourceVolumeName = "migrate.obnvmf/source-volume-name"
	MigrationLabelKeySourceNodeId     = "migrate.obnvmf/source-node-id"
	MigrationLabelKeyHostNodeId       = "migrate.obnvmf/host-node-id"

	MigrationFinalizerPipeConnected = "obnvmf/migrate-pipe-connected"
	MigrationFinalizerHostConnected = "obnvmf/migrate-host-connected"
)

// +kubebuilder:validation:Enum=Pending;CreatingVolume;SetupPipe;Syncing;Cleaning;Finished
type MigrationPhase string

// +kubebuilder:validation:Enum=Unknown;Connected;Disconnected
type ConnectStatus string

// +kubebuilder:validation:Enum=Unknown;Success;Error
type ResultStatus string

type VolumeInfo struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`

	// +optional
	TargetNodeId string `json:"targetNodeId"`

	// +optional
	HostNodeId string `json:"hostNodeId"`

	// +optional
	Spdk SpdkTarget `json:"spdk"`
}

type MigrationPipe struct {
	DestBdevName string `json:"destBdevName,omitempty"`
	// +kubebuilder:default=Unknown
	Status ConnectStatus `json:"status,omitempty"`
}

type HostConnectDestVolume struct {
	Status string `json:"status,omitempty"`
}

type AutoSwitch struct {
	Enabled bool `json:"enabled,omitempty"`

	// +kubebuilder:default=Unknown
	Status ResultStatus `json:"status,omitempty"`
}

type JobProgress struct {
	SrcBdev         string `json:"srcBdev,omitempty"`
	DstBdev         string `json:"dstBdev,omitempty"`
	Status          string `json:"status,omitempty"`
	TotalWritePages int    `json:"total_write_pages,omitempty"`
	TotalReadPages  int    `json:"total_read_pages,omitempty"`
	RoundPassed     int    `json:"roundPassed,omitempty"`
	ElapsedTimeMs   int    `json:"ms_elapsed,omitempty"`
	// working_round
	IsLastRound string `json:"is_last_round,omitempty"`
}

type MigrationInfo struct {
	// +optional
	MigrationPipe MigrationPipe `json:"migrationPipe,omitempty"`

	// +optional
	// +kubebuilder:default=Unknown
	HostConnectStatus ConnectStatus `json:"hostConnectStatus,omitempty"`

	// +optional
	StartTimestamp int `json:"startTs,omitempty"`

	// +optional
	AutoSwitch AutoSwitch `json:"autoSwitch,omitempty"`

	// +optional
	JobProgress JobProgress `json:"jobProgress,omitempty"`
}

type VolumeMigrationSpec struct {
	// SourceVolume is the source volume, which has the original data
	SourceVolume VolumeInfo `json:"sourceVolume"`

	// +optional
	// DestVolume is the destination volume, where data will flow
	DestVolume VolumeInfo `json:"destVolume,omitempty"`

	// +optional
	// MigrationInfo has detailed information about migration
	MigrationInfo MigrationInfo `json:"migrationInfo,omitempty"`
}

type VolumeMigrationStatus struct {
	// +optional
	// +kubebuilder:default=Pending
	Phase MigrationPhase `json:"phase,omitempty"`

	// +optional
	Status string `json:"status,omitempty"`

	// +optional
	Message string `json:"message,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:name="source",type=string,JSONPath=`.spec.sourceVolume.name`
//+kubebuilder:printcolumn:name="target",type=string,JSONPath=`.spec.destVolume.name`
//+kubebuilder:printcolumn:name="phase",type=string,JSONPath=`.status.phase`
//+kubebuilder:printcolumn:name="status",type=string,JSONPath=`.status.status`
//+kubebuilder:printcolumn:name="age",type="date",JSONPath=".metadata.creationTimestamp"
// VolumeMigration is the Schema for the VolumeMigration API
type VolumeMigration struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec VolumeMigrationSpec `json:"spec,omitempty"`

	// +optional
	Status VolumeMigrationStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
//+kubebuilder:object:root=true
// VolumeMigrationList contains a list of VolumeMigration
type VolumeMigrationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []VolumeMigration `json:"items"`
}

func init() {
	SchemeBuilder.Register(&VolumeMigration{}, &VolumeMigrationList{})
}
