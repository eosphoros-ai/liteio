package nvme

type NvmeDevice struct {
	DevicePath   string `json:"DevicePath"`
	ModelNumber  string `json:"ModelNumber"`
	SerialNumber string `json:"SerialNumber"`
	UsedBytes    uint64 `json:"UsedBytes"`
	PhysicalSize uint64 `json:"PhysicalSize"`
	SectorSize   uint64 `json:"SectorSize"`
}

/*
./nvme list-subsys -ojson
{
  "Subsystems" : [
	# 可能因为挂载点没有 umount, 导致信息残留，后面一个Item没有Paths信息
	{
      "Name" : "nvme-subsys2",
      "NQN" : "nqn.2021-03.xuhai:test-2"
    },
	# 活跃中的 subsys 后面一个Item是Paths
    {
      "Name" : "nvme-subsys0",
      "NQN" : "nqn.2021-03.xuhai:test"
    },
    {
      "Paths" : [
        {
          "Name" : "alinvme0",
          "Transport" : "tcp",
          "Address" : "traddr=100.100.100.200 trsvcid=20002",
          "State" : "live",
          "PathState": "working" // 状态有 working || standby ||  degraded
        },
        {
          "Name" : "alinvme1",
          "Transport" : "tcp",
          "Address" : "traddr=100.100.100.210 trsvcid=20002",
          "State" : "live",
          "PathState": "standby"
        }
      ]
    }
  ]
}
*/

type SubsystemList struct {
	Subsystems []SubsystemItem `json:"Subsystems"`
}

type SubsystemItem struct {
	// example: nvme-subsys0
	Name string `json:"Name"`
	// example: nqn.2021-03.com.alipay.ob:uuid:a58665ff-d73a-4d91-9a41-1fe4aac755c4
	NQN string `json:"NQN"`
	// path list
	Paths []Path `json:"Paths"`
}

type Path struct {
	// example: alinvme0
	Name string `json:"Name"`
	// example: tcp
	Transport string `json:"Transport"`
	// example: traddr=100.100.100.1 trsvcid=20002
	Address string `json:"Address"`
	// enum: live
	State string `json:"State"`
	// enum: working || standby ||  degraded
	PathState string `json:"PathState"`
}
