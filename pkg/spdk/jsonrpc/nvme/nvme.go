package nvme

import (
	"encoding/json"
	"fmt"
	"log"
	"os/exec"
	"strconv"
	"strings"

	"code.alipay.com/dbplatform/node-disk-controller/pkg/util/misc"
)

const (
	DefaultCmdName = "nvme"
)

type DisconnectTargetRequest struct {
	NQN string
	// optional, for multi-path
	TrAddr, SvcID string
}

type CmdClient struct {
	NvmeCmdPath string
}

// NewCmdClient finds the full file paht of the command nvme
func NewCmdClient() (cli *CmdClient, err error) {
	path, err := exec.LookPath(DefaultCmdName)
	if err != nil {
		return
	}

	cli = &CmdClient{
		NvmeCmdPath: path,
	}
	return
}

// NewClientWithCmdPath use file as the full path of command nvme
func NewClientWithCmdPath(file string) (cli *CmdClient) {
	var nvmePath = file
	if has, err := misc.FileExists(nvmePath); !has {
		log.Println("ERROR: ", err)
		cli, err = NewCmdClient()
		if err == nil {
			return
		} else {
			log.Println("ERROR: ", err)
			nvmePath = DefaultCmdName
		}
	}

	return &CmdClient{
		NvmeCmdPath: nvmePath,
	}
}

func (cli *CmdClient) ListNvmeDisk() (list []NvmeDevice, err error) {
	bs, err := exec.Command(cli.NvmeCmdPath, "list", "-o", "json").Output()
	if err != nil {
		return
	}

	// no nvme devices
	if len(bs) == 0 {
		return
	}

	var resp = make(map[string][]NvmeDevice)
	err = json.Unmarshal(bs, &resp)
	if err != nil {
		return
	}

	list = resp["Devices"]
	return
}

type ConnectTargetOpts struct {
	// nvme client reconnect time interval in seconds
	ReconnectDelaySec int
	CtrlLossTMO       int
	// hostTrAddr: only used in VFIOUSER mode, INTRA_HOST or LOCAL_COPY(set in opts)
	HostTransAddr string
}

// nvme connect -t tcp -a 100.100.100.1 -s 4450 -n nqn.2021-03.com.alipay.ob:test-aio2
// opts: --reconnect-delay 2 --ctrl-loss-tmo 10
// transType: tcp, vfio-user
// transAddr: target ip for remote voluem, socket path for local vfio volume
func (cli *CmdClient) ConnectTarget(transType, transAddr, svcID, nqn string, opt ConnectTargetOpts) (output []byte, err error) {

	args := []string{
		"connect", "-t", transType, "-a", transAddr, "-s", svcID, "-n", nqn,
	}
	if opt.ReconnectDelaySec > 0 {
		args = append(args, "--reconnect-delay", strconv.Itoa(opt.ReconnectDelaySec))
	}
	if opt.CtrlLossTMO > 0 {
		args = append(args, "--ctrl-loss-tmo", strconv.Itoa(opt.CtrlLossTMO))
	}
	if len(opt.HostTransAddr) > 0 {
		args = append(args, "-w", opt.HostTransAddr)
	}

	output, err = exec.Command(cli.NvmeCmdPath, args...).CombinedOutput()
	fmt.Println("connect command: ", args)
	return
}

func (cli *CmdClient) DisconnectTarget(req DisconnectTargetRequest) (output []byte, err error) {
	// nvme disconnect -n nqn.2021-03.com.alipay.ob:test-aio2
	if req.NQN == "" {
		err = fmt.Errorf("invalid param: %#v", req)
		return
	}

	var cmdArgs = []string{"disconnect", "-n", req.NQN}
	if req.TrAddr != "" {
		cmdArgs = append(cmdArgs, "-a", req.TrAddr)
	}
	if req.SvcID != "" {
		cmdArgs = append(cmdArgs, "-s", req.SvcID)
	}

	fmt.Printf("DisconnectTarget: %s %+v \n", cli.NvmeCmdPath, cmdArgs)

	output, err = exec.Command(cli.NvmeCmdPath, cmdArgs...).CombinedOutput()

	return
}

func (cli *CmdClient) ListSubsystems() (list SubsystemList, err error) {
	var (
		output  []byte
		rawList []SubsystemItem
	)
	output, err = exec.Command(cli.NvmeCmdPath, "list-subsys", "-o", "json").Output()
	if err != nil {
		return
	}

	err = json.Unmarshal(output, &list)
	if err != nil {
		return
	}
	rawList = list.Subsystems

	list.Subsystems = formatSybsysList(rawList)

	return
}

func formatSybsysList(rawList []SubsystemItem) (list []SubsystemItem) {
	for i := 0; i < len(rawList); i++ {
		firstOne := rawList[i]
		if firstOne.NQN != "" && i+1 < len(rawList) {
			paths := rawList[i+1]
			firstOne.Paths = paths.Paths
			list = append(list, firstOne)
		}
	}
	return
}

// ParseNvmePathAddress handles addr in format of "traddr=100.100.100.1 trsvcid=20002"
func ParseNvmePathAddress(addr string) (trAddr, svcId string) {
	kvs := strings.Split(addr, " ")
	for _, item := range kvs {
		kv := strings.SplitN(item, "=", 2)
		if len(kv) == 2 {
			switch kv[0] {
			case "traddr":
				trAddr = kv[1]
			case "trsvcid":
				svcId = kv[1]
			}
		}
	}
	return
}
