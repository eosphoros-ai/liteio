package state

import (
	"encoding/json"
	"net/http"

	v1 "code.alipay.com/dbplatform/node-disk-controller/pkg/api/volume.antstor.alipay.com/v1"
)

type NodeStateAPI struct {
	Name       string            `json:"name"`
	PoolLabels map[string]string `json:"poolLabels"`
	KernelLVM  *v1.KernelLVM     `json:"kernelLVM,omitempty"`
	SpdkLVS    *v1.SpdkLVStore   `json:"spdkLVS,omitempty"`
	// Volumes breif info
	Volumes []VolumeBrief `json:"volumes"`
	// FreeSize of the pool
	FreeSize int64 `json:"freeSize"`
	// Conditions of the pool status
	Conditions map[v1.PoolConditionType]v1.ConditionStatus `json:"conditions"`
	// Resvervations on the node
	Resvervations []ReservationBreif `json:"reservations"`
}

type VolumeBrief struct {
	Namespace  string `json:"ns"`
	Name       string `json:"name"`
	DataHolder string `json:"dataHolder"`
	Size       int64  `json:"size"`
}

type ReservationBreif struct {
	ID   string `json:"id"`
	Size int64  `json:"size"`
}

func NewStateHandler(s StateIface) *StateHandler {
	return &StateHandler{state: s}
}

type StateHandler struct {
	state StateIface
}

func (h *StateHandler) ServeHTTP(writer http.ResponseWriter, req *http.Request) {
	var spName = req.URL.Query().Get("name")
	if spName == "" {
		writer.Write([]byte("query param name is empty"))
		return
	}

	var node, err = h.state.GetNodeByNodeID(spName)
	if err != nil {
		writer.Write([]byte(err.Error()))
		return
	}

	var api = NodeStateAPI{
		Name:       spName,
		PoolLabels: node.Pool.Labels,
		KernelLVM:  &node.Pool.Spec.KernelLVM,
		SpdkLVS:    &node.Pool.Spec.SpdkLVStore,
		FreeSize:   node.Pool.Status.VGFreeSize.Value(),
		Conditions: make(map[v1.PoolConditionType]v1.ConditionStatus),
	}

	for _, item := range node.Pool.Status.Conditions {
		api.Conditions[item.Type] = item.Status
	}

	for _, vol := range node.Volumes {
		api.Volumes = append(api.Volumes, VolumeBrief{
			Namespace:  vol.Namespace,
			Name:       vol.Name,
			Size:       int64(vol.Spec.SizeByte),
			DataHolder: vol.Labels[v1.VolumeDataHolderKey],
		})
	}

	if node.resvSet != nil {
		for _, resv := range node.resvSet.Items() {
			api.Resvervations = append(api.Resvervations, ReservationBreif{
				ID:   resv.ID(),
				Size: resv.Size(),
			})
		}
	}

	bs, err := json.Marshal(api)
	if err != nil {
		writer.Write([]byte(err.Error()))
		return
	}
	writer.Write(bs)
}
