package client

import "encoding/json"

const (
	RaidBdevCategoryAll = "all"
)

type SpdkBdevRaidIface interface {
	// bdev_raid_create
	CreateBdevRaid(req CreateBdevRaidRequest) (ok bool, err error)
	// bdev_raid_get_bdevs
	ListBdevRaid(req ListBdevRaidRequest) (names []string, err error)
}

type CreateBdevRaidRequest struct {
	Name        string   `json:"name"`
	RaidLevel   string   `json:"raid_level"`
	StripSizeKB int      `json:"strip_size_kb"`
	BaseBdevs   []string `json:"base_bdevs"`
}

type ListBdevRaidRequest struct {
	Category string `json:"category"`
}

func (s *SPDK) CreateBdevRaid(req CreateBdevRaidRequest) (ok bool, err error) {
	bs, err := s.rawCli.Call("bdev_raid_create", req)
	if err != nil {
		return
	}
	err = json.Unmarshal(bs, &ok)
	return
}

func (s *SPDK) ListBdevRaid(req ListBdevRaidRequest) (names []string, err error) {
	bs, err := s.rawCli.Call("bdev_raid_get_bdevs", req)
	if err != nil {
		return
	}
	err = json.Unmarshal(bs, &names)
	return
}
