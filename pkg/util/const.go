package util

const Int64Max = int64(^uint64(0) >> 1)

const DefaultTimeFormat = "2006-01-02 15:04:05"

const (
	KiB uint64 = 1024
	MiB uint64 = KiB * 1024
	GiB uint64 = MiB * 1024
	TiB uint64 = GiB * 1024
)

const (
	FileSystemExt4    string = "ext4"
	FileSystemXfs     string = "xfs"
	DefaultFileSystem string = FileSystemExt4
)

const (
	KubeConfigUserAgent = "obnvmf-node-disk/v0.0.1"
	KubeCfgUserAgentCSI = "obnvmf-csi/v0.0.1"
)
