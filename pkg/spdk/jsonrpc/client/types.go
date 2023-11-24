package client

import (
	"encoding/json"
	"fmt"
)

const (
	JSONRPCVersion = "2.0"

	ClearMethodNone        ClearMethod = "none"
	ClearMethodUnmap       ClearMethod = "unmap"
	ClearMethodWriteZeroes ClearMethod = "write_zeroes"

	// "IPv4", "IPv6", "IB", or "FC"
	AddrFamilyIPv4      AddressFamily = "IPv4"
	AddrFamilyIPv6      AddressFamily = "IPv6"
	AddrFamilyIB        AddressFamily = "IB"
	AddrFamilyFC        AddressFamily = "FC"
	AddrFamilyIntraHost AddressFamily = "INTRA_HOST"
	AddrFamilyLocalCopy AddressFamily = "LOCAL_COPY"

	TransportTypeRDMA     = "RDMA"
	TransportTypeTCP      = "TCP"
	TransportTypeVFIOUSER = "VFIOUSER"
)

type ClearMethod string
type AddressFamily string

type RPCRequest struct {
	RPCVersion string      `json:"jsonrpc"`
	Method     string      `json:"method"`
	ID         uint64      `json:"id"`
	Params     interface{} `json:"params,omitempty"`
}

type RPCResponse struct {
	JsonRPCVersion string          `json:"jsonrpc"`
	ID             uint64          `json:"id"`
	Result         json.RawMessage `json:"result"`
	Error          RPCError        `json:"error"`
}

type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e RPCError) Error() string {
	return fmt.Sprintf("Code=%d Msg=%s", e.Code, e.Message)
}

// BdevAioCreateReq bdev_aio_create
type BdevAioCreateReq struct {
	// required
	BdevName string `json:"name"`
	FileName string `json:"filename"`
	// optional
	BlockSize int `json:"block_size,omitempty"`
}

type NVMFCreateSubsystemReq struct {
	// required
	NQN string `json:"nqn"`

	// optional
	// Parent NVMe-oF target name.
	TargetName string `json:"tgt_name,omitempty"`
	// Serial number of virtual controller
	SerialNumber string `json:"serial_number,omitempty"`
	// Model number of virtual controller
	ModelNumber   string `json:"model_number,omitempty"`
	MaxNamespaces int    `json:"max_namespaces,omitempty"`
	AllowAnyHost  bool   `json:"allow_any_host,omitempty"`
	ANAReporting  bool   `json:"ana_reporting,omitempty"`
}

type NVMFSubsystemAddNSReq struct {
	// required
	NQN       string            `json:"nqn"`
	Namespace NamespaceForAddNS `json:"namespace"`

	// optional
	TargetName string `json:"tgt_name,omitempty"`
}

type NamespaceForAddNS struct {
	// req
	BdevName string `json:"bdev_name"`

	// opt
	NSID     int    `json:"nsid,omitempty"`
	NGUID    string `json:"nguid,omitempty"`
	EUI64    string `json:"eui64,omitempty"`
	UUID     string `json:"uuid,omitempty"`
	PtplFile string `json:"ptpl_file,omitempty"`
}

type ListenAddress struct {
	// required
	TrType string        `json:"trtype"` // RDMA
	AdrFam AddressFamily `json:"adrfam"`
	TrAddr string        `json:"traddr"`
	// opt
	TrSvcID string `json:"trsvcid,omitempty"`
}

type NVMFSubsystemAddListenerReq struct {
	// required
	NQN string `json:"nqn"`
	// opt
	TargetName    string        `json:"tgt_name,omitempty"`
	ListenAddress ListenAddress `json:"listen_address,omitempty"`
}

type NVMFCreateTransportReq struct {
	// required
	TrType string `json:"trtype"`

	// optional
	TargetName          string `json:"tgt_name,omitempty"`
	MaxQueueDepth       int    `json:"max_queue_depth,omitempty"`
	MaxQPairsPerCtrlr   int    `json:"max_qpairs_per_ctrlr,omitempty"`
	MaxIOQPairsPerCtrlr int    `json:"max_io_qpairs_per_ctrlr,omitempty"`
	InCapsuleDataSize   int    `json:"in_capsule_data_size,omitempty"`
	MaxIOSize           int    `json:"max_io_size,omitempty"`
	IOUnitSize          int    `json:"io_unit_size,omitempty"`
	MaxAQDepth          int    `json:"max_aq_depth,omitempty"`
	NumSharedBuffers    int    `json:"num_shared_buffers,omitempty"`
	BufCacheSize        int    `json:"buf_cache_size,omitempty"`
	NumCQE              int    `json:"num_cqe,omitempty"`
	MaxSRQDepth         int    `json:"max_srq_depth,omitempty"`
	SockPriority        int    `json:"sock_priority,omitempty"`
	AcceptorBacklog     int    `json:"acceptor_backlog,omitempty"`
	AbortTimeoutSec     int    `json:"abort_timeout_sec,omitempty"`

	NoWrBatching     bool `json:"no_wr_batching,omitempty"`
	DifInsertOrStrip bool `json:"dif_insert_or_strip,omitempty"`
	C2hSuccess       bool `json:"c2h_success,omitempty"`
	NoSRQ            bool `json:"no_srq,omitempty"`
}

type Transport struct {
	// TCP or RDMA
	TransType     string `json:"trtype"`
	MaxQueueDepth int    `json:"max_queue_depth"`
}

type Bdev struct {
	Name        string             `json:"name"`
	Aliases     []string           `json:"aliases"`
	ProductName string             `json:"product_name"`
	UUID        string             `json:"uuid"`
	BlockSize   int                `json:"block_size"`
	NumBlocks   int                `json:"num_blocks"`
	Driver      DriverSpecific     `json:"driver_specific"`
	RateLimits  AssignedRateLimits `json:"assigned_rate_limits"`
}

type DriverSpecific struct {
	AIO AIODriver `json:"aio"`
}

type AssignedRateLimits struct {
	RWIops   uint64 `json:"rw_ios_per_sec"`
	RWMbytes uint64 `json:"rw_mbytes_per_sec"`
	RMbytes  uint64 `json:"r_mbytes_per_sec"`
	WMbytes  uint64 `json:"w_mbytes_per_sec"`
}

type AIODriver struct {
	Filename string `json:"filename"`
}

type Subsystem struct {
	NQN             string          `json:""`
	ListenAddresses []ListenAddress `json:"listen_addresses"`
	AllowAnyHost    bool            `json:"allow_any_host"`
	SerialNum       string          `json:"serial_number"`
	ModelNum        string          `json:"model_number"`
	Namespaces      []Namespace     `json:"namespaces"`
	Hosts           []SubsysHost    `json:"hosts"`
}

type SubsysHost struct {
	NQN string `json:"nqn"`
}

type Namespace struct {
	NsID     int    `json:"nsid"`
	BdevName string `json:"bdev_Name"`
	Name     string `json:"name"`
	UUID     string `json:"uuid"`
}

type NVMFDeleteSubsystemReq struct {
	NQN string `json:"nqn"`
	// opt
	TgtName string `json:"tgt_name,omitempty"`
}

type BdevAioDeleteReq struct {
	Name string `json:"name"`
}

type BdevAioResizeReq struct {
	Name string `json:"name"`
	Size uint64 `json:"size"`
}

/*
	{
	  "version": "SPDK v21.01.1 Stupa v0.0.8 git sha1 35c4cd3c3 - Nov 17 2022 18:43:48",
	  "fields": {
	    "major": 21,
	    "minor": 1,
	    "patch": 1,
	    "suffix": " Stupa v0.0.8",
	    "commit": "35c4cd3c3"
	  }
	}
*/
type SpdkVersion struct {
	Version string            `json:"version"`
	Fields  SpdkVersionFields `json:"fields"`
}

type SpdkVersionFields struct {
	Major  int    `json:"major"`
	Minor  int    `json:"minor"`
	Patch  int    `json:"patch"`
	Suffix string `json:"suffix"`
	Commit string `json:"commit"`
}

type FrameworkGetSubsystemsItem struct {
	Subsystem string   `json:"subsystem"`
	DependsOn []string `json:"depends_on"`
}

type FrameworkGetConfigReq struct {
	Name string `json:"name"`
}

type FrameworkGetConfigItem map[string]interface{}

type BdevGetBdevsReq struct {
	// optional
	BdevName string `json:"name,omitempty"`
}

type BdevGetIostatReq struct {
	BdevName string `json:"name,omitempty"`
}

type BdevIostat struct {
	Name              string `json:"name"` // bdev uuid
	BytesRead         uint64 `json:"bytes_read"`
	NumReadOps        uint64 `json:"num_read_ops"`
	BytesWritten      uint64 `json:"bytes_written"`
	NumWriteOps       uint64 `json:"num_write_ops"`
	BytesUnmapped     uint64 `json:"bytes_unmapped"`
	NumUnmapOps       uint64 `json:"num_unmap_ops"`
	ReadLatencyTicks  uint64 `json:"read_latency_ticks"`
	WriteLatencyTicks uint64 `json:"write_latency_ticks"`
	UnmapLatencyTicks uint64 `json:"unmap_latency_ticks"`
	TimeInQueue       uint64 `json:"time_in_queue"`
}

type BdevIostats struct {
	TickRate uint64       `json:"tick_rate"`
	Ticks    uint64       `json:"ticks"`
	Bdevs    []BdevIostat `json:"bdevs"`
}

type SubsystemStat struct {
	TickRate   uint64      `json:"tick_rate"`
	Ticks      uint64      `json:"ticks"`
	PollGroups []PoolGroup `json:"poll_groups"`
}

type PoolGroup struct {
	Name          string      `json:"name"`
	AdminQpairs   uint64      `json:"admin_qpairs"`
	IoQpairs      uint64      `json:"io_qpairs"`
	PendingBdevIo uint64      `json:"pending_bdev_io"`
	Transports    []string    `json:"transports"`
	Subsystems    []SubSystem `json:"subsystems"`
}

type SubSystem struct {
	Ios Ios `json:"ios"`
}

type Ios struct {
	SubsysName        string `json:"subsys_name"`
	BytesRead         uint64 `json:"bytes_read"`
	NumReadOps        uint64 `json:"num_read_ops"`
	BytesWritten      uint64 `json:"bytes_written"`
	NumWriteOps       uint64 `json:"num_write_ops"`
	ReadLatencyTicks  uint64 `json:"read_latency_ticks"`
	WriteLatencyTicks uint64 `json:"write_latency_ticks"`
	TimeInQueue       uint64 `json:"time_in_queue"`
}

type NVMFSubsystemAddHostReq struct {
	NQN     string `json:"nqn"`
	HostNQN string `json:"host"`
	// optional
	TargetName  string `json:"tgt_name,omitempty"`
	PSKFilePath string `json:"psk,omitempty"`
}
