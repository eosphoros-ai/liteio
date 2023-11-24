package nvme

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseNvmePath(t *testing.T) {
	tests := []struct {
		Addr        string
		ExpectAddr  string
		ExpectSvcId string
	}{
		{
			Addr:        "traddr=100.100.100.1 trsvcid=20002",
			ExpectAddr:  "100.100.100.1",
			ExpectSvcId: "20002",
		},
		{
			Addr:        "traddr=100.100.100.1",
			ExpectAddr:  "100.100.100.1",
			ExpectSvcId: "",
		},
		{
			Addr:        "traddr=",
			ExpectAddr:  "",
			ExpectSvcId: "",
		},
		{
			Addr:        "trsvcid=xxx",
			ExpectAddr:  "",
			ExpectSvcId: "xxx",
		},
		{
			Addr:        "",
			ExpectAddr:  "",
			ExpectSvcId: "",
		},
		{
			Addr:        "traddr=100.100.100.1 trsvcid=20002 xxx=yyy",
			ExpectAddr:  "100.100.100.1",
			ExpectSvcId: "20002",
		},
	}

	for _, item := range tests {
		addr, svc := ParseNvmePathAddress(item.Addr)
		assert.Equal(t, item.ExpectAddr, addr)
		assert.Equal(t, item.ExpectSvcId, svc)
	}
}

func TestFormatSubsysList(t *testing.T) {
	rawList := []SubsystemItem{
		{
			Name: "nvme0",
			NQN:  "nqn0",
		},
		{
			Name: "nvme1",
			NQN:  "nqn1",
		},
		{
			Paths: []Path{
				{
					Name:      "path1",
					Transport: "tcp",
					Address:   "addr",
					State:     "live",
					PathState: "working",
				},
				{
					Name:      "path2",
					Transport: "tcp",
					Address:   "addr",
					State:     "live",
					PathState: "standby",
				},
			},
		},
		{
			Name: "nvme2",
			NQN:  "nqn2",
		},
		{
			Name: "nvme3",
			NQN:  "nqn3",
		},
	}

	list := formatSybsysList(rawList)
	t.Logf("%+v", list)
	assert.Equal(t, 3, len(list))
	assert.Equal(t, 0, len(list[0].Paths))
	assert.Equal(t, 2, len(list[1].Paths))
	assert.Equal(t, 0, len(list[2].Paths))
}

func TestGetNvmeTcpVersion(t *testing.T) {
	tests := []struct {
		rawVersion string
		expect     NvmfVersion
	}{
		{
			"alinvme v0.0.8 a4e115c 2022-11-17 18:44:03",
			NvmfVersion{
				Version: "v0.0.8",
				Commit:  "a4e115c",
				Time:    "2022-11-17 18:44:03",
			},
		},
		{
			"alinvme v0.0.6",
			NvmfVersion{
				Version: "v0.0.6",
			},
		},
	}

	for _, test := range tests {
		ver := parseNVMeTCPVersion(test.rawVersion)
		assert.Equal(t, test.expect, ver)
	}
}
