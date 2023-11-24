package client

import (
	"encoding/json"
	"errors"
)

const (
	MigrateTaskStatusWorking   = "working"
	MigrateTaskStatusCompleted = "completed"
	MigrateTaskStatusSuspend   = "suspend"
	MigrateTaskStatusRemoving  = "removing"
	MigrateTaskStatusFailed    = "failed"
	MigrateTaskStatusUnknown   = "unknown"
)

type SpdkMigrateIface interface {
	// bdev_migrate_query
	BdevMigrateQuery(req BdevMigrateQueryRequest) (list []MigrateTask, err error)
	// bdev_migrate_start
	BdevMigrateStart(req BdevMigrateStartRequest) (err error)
	// bdev_migrate_set_config
	BdevMigrateSetConfig(req BdevMigrateSetConfigRequest) (err error)
	// bdev_migrate_cleanup_task
	BdevMigrateCleanupTask(req BdevMigrateStartRequest) (err error)
}

type MigrateTask struct {
	TaskID           string    `json:"task_id"`
	SrcBdev          string    `json:"src_bdev"`
	DstBdev          string    `json:"dst_bdev"`
	Status           string    `json:"status"`
	EnableSwitch     string    `json:"enable_switch"`
	MaxQueueDepth    int       `json:"max_queue_depth"`
	LastRoundExtends int       `json:"last_round_extents"`
	LastRoundSize    int       `json:"last_round_size"`
	TotalWritePages  int       `json:"total_write_pages"`
	TotalReadPages   int       `json:"total_read_pages"`
	TimeElapsedMS    int       `json:"ms_elapsed"`
	RoundPassed      int       `json:"round_passed"`
	WorkingRound     RoundInfo `json:"working_round"`
}

type RoundInfo struct {
	RoundIndex    int    `json:"round_index"`
	IsLastRound   string `json:"is_last_round"`
	TimeElapsedMS int    `json:"ms_elapsed"`
}

type BdevMigrateQueryRequest struct {
	SrcBdev string `json:"src_bdev"`
	// optional
	ListHistory bool `json:"list_history,omitempty"`
}

type BdevMigrateStartRequest struct {
	SrcBdev string `json:"src_bdev"`
	DstBdev string `json:"dst_bdev"`
	// optional
	AutoSwitch bool `json:"auto_switch,omitempty"`
}

type BdevMigrateSetConfigRequest struct {
	SrcBdev string `json:"src_bdev"`
	DstBdev string `json:"dst_bdev"`
	// optional
	AutoSwitch       bool `json:"enable_switch,omitempty"`
	LastRoundExtends int  `json:"last_round_extents,omitempty"`
	LastRoundSize    int  `json:"last_round_size,omitempty"`
}

func (s *SPDK) BdevMigrateQuery(req BdevMigrateQueryRequest) (tasks []MigrateTask, err error) {
	bs, err := s.rawCli.Call("bdev_migrate_query", req)
	if err != nil {
		return
	}
	err = json.Unmarshal(bs, &tasks)

	// filter by src_bdev, b/c spdk rpc returns all tasks.
	var newtasks []MigrateTask
	for _, item := range tasks {
		if item.SrcBdev == req.SrcBdev {
			newtasks = append(newtasks, item)
		}
	}

	return newtasks, err
}

func (s *SPDK) BdevMigrateStart(req BdevMigrateStartRequest) (err error) {
	bs, err := s.rawCli.Call("bdev_migrate_start", req)
	if err != nil {
		err = errors.New(err.Error() + string(bs))
		return
	}

	return
}

func (s *SPDK) BdevMigrateSetConfig(req BdevMigrateSetConfigRequest) (err error) {
	bs, err := s.rawCli.Call("bdev_migrate_set_config", req)
	if err != nil {
		err = errors.New(err.Error() + string(bs))
		return
	}
	return
}

func (s *SPDK) BdevMigrateCleanupTask(req BdevMigrateStartRequest) (err error) {
	bs, err := s.rawCli.Call("bdev_migrate_cleanup_task", req)
	if err != nil {
		err = errors.New(err.Error() + string(bs))
		return
	}
	return
}
