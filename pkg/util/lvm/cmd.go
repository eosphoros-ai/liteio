package lvm

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"strconv"
	"strings"

	"code.alipay.com/dbplatform/node-disk-controller/pkg/util/osutil"
	"k8s.io/klog/v2"
)

const (
	LvDeviceOpen    = "open"
	LvDeviceNotOpen = ""

	pvLostErrStr = "not found or rejected by a filter"
)

var (
	PvLostErr = fmt.Errorf("LVM_PV_LOST")

	vgsCmd = cmdArgs{
		cmd:  "vgs",
		args: []string{"--units", "B", "-o", "uuid"},
	}

	lvsCmd = cmdArgs{
		cmd:  "lvs",
		args: []string{"--units", "B"},
	}

	pvsCmdJson = cmdArgs{
		cmd:  "pvs",
		args: []string{"--reportformat", "json", "--units", "B"},
	}

	// vgs --options vg_all --reportformat json --units B
	vgsCmdJson = cmdArgs{
		cmd:  "vgs",
		args: []string{"--options", "vg_all", "--reportformat", "json", "--units", "B"},
	}

	// --noheadings -o lv_all,vg_name,segtype --units b --reportformat json
	lvsCmdJson = cmdArgs{
		cmd:  "lvs",
		args: []string{"--noheadings", "--units", "B", "-o", "lv_uuid,lv_name,lv_size,lv_path,lv_full_name,vg_name,lv_layout,lv_attr,lv_device_open,origin,origin_uuid,origin_size,vg_name,segtype", "--reportformat", "json"},
	}
)

type cmdArgs struct {
	cmd  string
	args []string
}

type lvmJSONOutput struct {
	Report []report `json:"report"`
}

type report struct {
	Vg []reportVG `json:"vg"`
	Lv []reportLV `json:"lv"`
	Pv []reportPV `json:"pv"`
}

type reportPV = PV

type PV struct {
	PvName string `json:"pv_name"`
	VgName string `json:"vg_name"`
	PvFmt  string `json:"pv_fmt"`
	PvSize string `json:"pv_size"`
	PvFree string `json:"pv_free"`
}

type reportVG struct {
	UUID string `json:"vg_uuid"`
	Name string `json:"vg_name"`
	Size string `json:"vg_size"` // 104857600B
	Free string `json:"vg_free"` // 104857600B
	// pv
	PVCount string `json:"pv_count"` // 12
	// extends
	ExtendSize  string `json:"vg_extent_size"`  // 4194304B
	ExtendCount string `json:"vg_extent_count"` // 629247
}

type reportLV struct {
	UUID     string `json:"lv_uuid"`
	Name     string `json:"lv_name"`
	Size     string `json:"lv_size"`      // 104857600B
	DevPath  string `json:"lv_path"`      // /dev/obnvmf-vg/test1
	FullName string `json:"lv_full_name"` // obnvmf-vg/test1
	VGName   string `json:"vg_name"`
	LvLayout string `json:"lv_layout"`
	// value example: "-wi-ao----"
	LvAttr string `json:"lv_attr"`
	// value is "open" or ""
	LvDeviceOpen string `json:"lv_device_open"`
	// origin vol
	Origin     string `json:"origin"`
	OriginUUID string `json:"origin_uuid"`
	// value example: "107374182400B"
	OriginSize string `json:"origin_size"`
}

type cmd struct {
	binDir string
	exec   osutil.ShellExec
	// cmd output json format
	jsonFormat bool
}

func (c *cmd) ListPV() (pvs []PV, err error) {
	var out []byte
	var cmd = filepath.Join(c.binDir, pvsCmdJson.cmd)
	out, err = c.exec.ExecCmd(cmd, pvsCmdJson.args)
	if err != nil {
		return
	}

	klog.Infof("ListPV out: %s", string(out))

	var report lvmJSONOutput
	err = json.Unmarshal(out, &report)
	if err != nil {
		return
	}

	if len(report.Report) == 0 {
		return
	}

	if len(report.Report[0].Pv) == 0 {
		return
	}

	pvs = make([]PV, len(report.Report[0].Pv))
	copy(pvs, report.Report[0].Pv)

	return
}

func (c *cmd) ListVG() (vgs []VG, err error) {
	if c.jsonFormat {
		return c.listVGJSON()
	}
	return c.listVG()
}

func (c *cmd) ListLVInVG(vgName string) (lvs []LV, err error) {
	if c.jsonFormat {
		return c.listLVInVGJSON(vgName)
	}
	return c.listLVInVG(vgName)
}

func (c *cmd) CreateVG(name string, pvs []string) (vg VG, err error) {
	var out []byte
	var createCmd = cmdArgs{
		cmd:  "vgcreate",
		args: make([]string, 0, len(pvs)+1),
	}
	createCmd.args = append(createCmd.args, name)
	createCmd.args = append(createCmd.args, pvs...)

	var cmd = filepath.Join(c.binDir, createCmd.cmd)
	out, err = c.exec.ExecCmd(cmd, createCmd.args)
	if err != nil {
		klog.Errorf("err %+v, output: %s", err, string(out))
		return
	}

	klog.Infof("vgcreate %+v, stdout: %s", pvs, string(out))

	var vgs []VG
	vgs, err = c.ListVG()
	if err != nil {
		return
	}
	for _, item := range vgs {
		if item.Name == name {
			vg = item
		}
	}

	return
}

func (c *cmd) RemoveVG(vgName string) (err error) {
	var out []byte
	var rmCmd = cmdArgs{
		cmd:  "vgremove",
		args: []string{vgName, "-y"},
	}
	var cmd = filepath.Join(c.binDir, rmCmd.cmd)
	out, err = c.exec.ExecCmd(cmd, rmCmd.args)
	if err != nil {
		klog.Errorf("err %+v, output: %s", err, string(out))
		return
	}

	klog.Infof("vgremove %+s, stdout: %s", vgName, string(out))

	return
}

func (c *cmd) CreatePV(pvs []string) (err error) {
	var out []byte
	var createCmd = cmdArgs{
		cmd:  "pvcreate",
		args: pvs,
	}
	var cmd = filepath.Join(c.binDir, createCmd.cmd)
	out, err = c.exec.ExecCmd(cmd, createCmd.args)
	if err != nil {
		klog.Errorf("err %+v, output: %s", err, string(out))
		return
	}

	klog.Infof("pvcreate %+v, stdout: %s", pvs, string(out))

	return
}

func (c *cmd) RemovePVs(pvs []string) (err error) {
	var out []byte
	var rmCmd = cmdArgs{
		cmd:  "pvremove",
		args: append(pvs, "-y"),
	}
	var cmd = filepath.Join(c.binDir, rmCmd.cmd)
	out, err = c.exec.ExecCmd(cmd, rmCmd.args)
	if err != nil {
		klog.Errorf("err %+v, output: %s", err, string(out))
		return
	}

	klog.Infof("pvremove %+v, stdout: %s", pvs, string(out))

	return
}

func (c *cmd) listLVInVGJSON(vgName string) (lvs []LV, err error) {
	var out []byte
	var cmd = filepath.Join(c.binDir, lvsCmdJson.cmd)
	out, err = c.exec.ExecCmd(cmd, lvsCmdJson.args)
	if err != nil {
		return
	}

	klog.Infof("ListLV out: %s", string(out))

	var output lvmJSONOutput
	err = json.Unmarshal(out, &output)
	if err != nil {
		return
	}

	if len(output.Report) == 0 {
		return
	}

	if len(output.Report[0].Lv) == 0 {
		return
	}

	var lvListByVGName []reportLV
	for _, item := range output.Report[0].Lv {
		if item.VGName == vgName {
			lvListByVGName = append(lvListByVGName, item)
		}
	}

	lvs = make([]LV, len(lvListByVGName))
	for i, item := range lvListByVGName {
		var total uint64
		total, err = strconv.ParseUint(strings.Trim(item.Size, "B"), 10, 0)
		if err != nil {
			klog.Error(err)
			continue
		}
		lvs[i] = LV{
			Name:     item.Name,
			VGName:   item.VGName,
			DevPath:  item.DevPath,
			SizeByte: total,
			LvLayout: item.LvLayout,
			LvAttr:   item.LvAttr,
			// "open" or ""
			LvDeviceOpen: item.LvDeviceOpen,
			// origin vol
			Origin:     item.Origin,
			OriginUUID: item.OriginUUID,
			OriginSize: item.OriginSize,
		}
	}

	return
}

func (c *cmd) listVGJSON() (vgs []VG, err error) {
	var out, stderr []byte
	var cmd = filepath.Join(c.binDir, vgsCmdJson.cmd)
	out, stderr, err = c.exec.ExecCmdWithError(cmd, vgsCmdJson.args)
	if err != nil {
		return
	}
	klog.Infof("ListVG out: %s", string(out))

	if strings.Contains(string(stderr), pvLostErrStr) {
		err = fmt.Errorf("%w stderr=%s", PvLostErr, string(stderr))
		return
	}

	var report lvmJSONOutput
	err = json.Unmarshal(out, &report)
	if err != nil {
		return
	}

	if len(report.Report) == 0 {
		return
	}

	if len(report.Report[0].Vg) == 0 {
		return
	}

	vgs = make([]VG, len(report.Report[0].Vg))
	for i, item := range report.Report[0].Vg {
		var total, free, extendSize uint64
		var pvCount, extendCount int
		total, err = strconv.ParseUint(strings.Trim(item.Size, "B"), 10, 0)
		if err != nil {
			klog.Error(err)
			continue
		}
		free, err = strconv.ParseUint(strings.Trim(item.Free, "B"), 10, 0)
		if err != nil {
			klog.Error(err)
			continue
		}

		extendSize, err = strconv.ParseUint(strings.Trim(item.ExtendSize, "B"), 10, 0)
		if err != nil {
			klog.Error(err)
			continue
		}

		pvCount, err = strconv.Atoi(item.PVCount)
		if err != nil {
			klog.Error(err)
			continue
		}

		extendCount, err = strconv.Atoi(item.ExtendCount)
		if err != nil {
			klog.Error(err)
			continue
		}

		vgs[i] = VG{
			Name:        item.Name,
			UUID:        item.UUID,
			TotalByte:   total,
			FreeByte:    free,
			PVCount:     pvCount,
			ExtendCount: extendCount,
			ExtendSize:  extendSize,
		}
	}

	return
}

/*
Output:
sudo vgs --units B -o +uuid
  VG        #PV #LV #SN Attr   VSize          VFree          VG UUID
  obnvmf-vg   1   1   0 wz--n- 4000783007744B 4000741064704B TJg2Kd-SYse-5ufS-Qj3r-wmLd-JlXd-r8JSyT
  vg00        2   1   0 wz--n- 8001566015488B 5802542759936B 0DcQZH-YQpg-N02W-o4Th-giBW-xBI2-A8u8oG
*/
// ListVG list all vg on the node
func (c *cmd) listVG() (vgs []VG, err error) {
	var out []byte
	var cmd = filepath.Join(c.binDir, vgsCmd.cmd)
	out, err = c.exec.ExecCmd(cmd, vgsCmd.args)
	if err != nil {
		return
	}

	klog.Infof("ListVG out: %s", string(out))

	err = handleFieldsOfEachLine(out, func(cols []string) error {
		var err error
		if len(cols) >= 8 && cols[0] != "VG" {
			vg := VG{
				Name: cols[0],
				UUID: cols[7],
			}
			totalInt, errStr := strconv.Atoi(strings.TrimRight(cols[5], "B"))
			if errStr != nil {
				err = fmt.Errorf("VSize is %s, err %+v", cols[5], errStr)
				return err
			}
			vg.TotalByte = uint64(totalInt)

			freeInt, errStr := strconv.Atoi(strings.TrimRight(cols[6], "B"))
			if errStr != nil {
				err = fmt.Errorf("VFree is %s, err %+v", cols[6], errStr)
				return err
			}
			vg.FreeByte = uint64(freeInt)
			vgs = append(vgs, vg)
		}
		return err
	})

	if err != nil {
		klog.Errorf("ListVG error %+v, out: %s", err, string(out))
		return
	}

	return
}

/*
sudo lvs  --units B

	LV          VG        Attr       LSize          Pool Origin Data%  Meta%  Move Log Cpy%Sync Convert
	vol-test001 obnvmf-vg -wi-a-----      41943040B
	lvol0       vg00      -wi-a----- 2199023255552B
*/
func (c *cmd) listLVInVG(vgName string) (lvs []LV, err error) {
	var out []byte
	var cmd = filepath.Join(c.binDir, lvsCmd.cmd)
	out, err = c.exec.ExecCmd(cmd, lvsCmd.args)
	if err != nil {
		return
	}

	klog.Infof("ListLVInVG out: %s", string(out))

	var allLvList []LV

	err = handleFieldsOfEachLine(out, func(cols []string) error {
		var err error
		if len(cols) >= 4 && cols[0] != "LV" {
			lv := LV{
				Name:    cols[0],
				VGName:  cols[1],
				DevPath: fmt.Sprintf("/dev/%s/%s", vgName, cols[0]),
			}
			totalInt, errStr := strconv.Atoi(strings.TrimRight(cols[3], "B"))
			if errStr != nil {
				err = fmt.Errorf("VSize is %s, err %+v", cols[3], errStr)
				return err
			}
			lv.SizeByte = uint64(totalInt)
			allLvList = append(allLvList, lv)
		}
		return err
	})

	if err != nil {
		klog.Errorf("ListLVInVG error %+v, out: %s", err, string(out))
		return
	}

	for _, item := range allLvList {
		if item.VGName == vgName {
			lvs = append(lvs, item)
		}
	}

	return
}

func (c *cmd) getPvCount(vgName string) (pvCnt int, vg VG, err error) {
	// get pv number
	vgList, err := c.listVGJSON()
	if err != nil {
		return
	}
	for _, item := range vgList {
		if item.Name == vgName {
			vg = item
			pvCnt = item.PVCount
			break
		}
	}
	return
}

func (c *cmd) CreateStripeLV(vgName, lvName string, sizeByte uint64) (vol LV, err error) {
	var pvNum int
	var out []byte
	var vg VG
	// get pv number
	pvNum, vg, err = c.getPvCount(vgName)
	if err != nil {
		return
	}
	if pvNum < 1 {
		err = fmt.Errorf("cannot found vg by name %s, %+v", vgName, vg)
		return
	}

	var createCmd = getStripeLVCreateCmd(vgName, lvName, sizeByte, pvNum)
	var cmd = filepath.Join(c.binDir, createCmd.cmd)
	out, err = c.exec.ExecCmd(cmd, createCmd.args)
	if err != nil {
		klog.Errorf("err %+v, output: %s", err, string(out))
		return
	}

	vol.Name = lvName
	vol.VGName = vgName
	vol.DevPath = fmt.Sprintf("/dev/%s/%s", vgName, lvName)
	vol.SizeByte = sizeByte
	return
}

// CreateLinearLV
func (c *cmd) CreateLinearLV(vgName, lvName string, opt LvOption) (vol LV, err error) {
	var out []byte
	var sizeByte = opt.Size
	var logicSize = opt.LogicSize
	var createCmd = getLvCreateCmd(vgName, lvName, sizeByte, logicSize)
	var cmd = filepath.Join(c.binDir, createCmd.cmd)
	out, err = c.exec.ExecCmd(cmd, createCmd.args)
	if err != nil {
		klog.Errorf("err %+v, output: %s", err, string(out))
		return
	}

	vol.Name = lvName
	vol.VGName = vgName
	vol.DevPath = fmt.Sprintf("/dev/%s/%s", vgName, lvName)
	vol.SizeByte = sizeByte
	return
}

func (c *cmd) RemoveLV(vgName, lvName string) (err error) {
	var out []byte
	var removeCmd = getLvRemoveCmd(vgName, lvName)
	var cmd = filepath.Join(c.binDir, removeCmd.cmd)
	out, err = c.exec.ExecCmd(cmd, removeCmd.args)
	if err != nil {
		klog.Errorf("err %+v, output: %s", err, string(out))
		return
	}

	return
}

// CreateSnapshotLinear command is lvcreate -L 1GB -s -n name_snap antstore-vg/origin-lv
func (c *cmd) CreateSnapshotLinear(vgName, snapName, originVol string, sizeByte uint64) (err error) {
	var out []byte
	var createCmd = getCreateSnapshotLinearCmd(vgName, snapName, originVol, sizeByte)
	var cmd = filepath.Join(c.binDir, createCmd.cmd)
	out, err = c.exec.ExecCmd(cmd, createCmd.args)
	if err != nil {
		klog.Errorf("err %+v, output: %s", err, string(out))
		return
	}
	return
}

// CreateSnapshotStripe command is lvcreate -i 1 -I 128k -L 1GB -s -n name_snap antstore-vg/origin-lv
func (c *cmd) CreateSnapshotStripe(vgName, snapName, originVol string, sizeByte uint64) (err error) {
	var pvNum int
	var out []byte
	var vg VG
	// get pv number
	pvNum, vg, err = c.getPvCount(vgName)
	if err != nil {
		return
	}
	if pvNum < 1 {
		err = fmt.Errorf("cannot found vg by name %s, %+v", vgName, vg)
		return
	}

	var createCmd = getCreateSnapshotStripeCmd(vgName, snapName, originVol, sizeByte, pvNum)
	var cmd = filepath.Join(c.binDir, createCmd.cmd)
	out, err = c.exec.ExecCmd(cmd, createCmd.args)
	if err != nil {
		klog.Errorf("err %+v, output: %s", err, string(out))
		return
	}

	return
}

// MergeSnapshot command is lvconvert --merge antstore-vg/name_snap
func (c *cmd) MergeSnapshot(vgName, snapName string) (err error) {
	var out []byte
	var mergeCmd = getMergeSnapshotCmd(vgName, snapName)
	var cmd = filepath.Join(c.binDir, mergeCmd.cmd)
	out, err = c.exec.ExecCmd(cmd, mergeCmd.args)
	if err != nil {
		klog.Errorf("err %+v, output: %s", err, string(out))
		return
	}
	return
}

// ExpandVolume command is lvextend --size +104857600B antstore-vg/lvol
// Format of targetVol could be /dev/vg/lvol or vg/lvol
func (c *cmd) ExpandVolume(deltaBytes int64, targetVol string) (err error) {
	var out []byte
	var cmd = filepath.Join(c.binDir, "lvextend")
	out, err = c.exec.ExecCmd(cmd, []string{"--size", fmt.Sprintf("+%dB", deltaBytes), targetVol})
	if err != nil {
		klog.Errorf("err %+v, output: %s", err, string(out))
		return
	}
	return
}

// cmd example: lvcreate -i 1 -I 128k -L 1GB -s -n name_snap antstore-vg/origin-lv
func getCreateSnapshotStripeCmd(vg, snapName, originName string, sizeByte uint64, pvCnt int) cmdArgs {
	return cmdArgs{
		cmd: "lvcreate",
		args: []string{
			"-i", strconv.Itoa(pvCnt),
			"-I", "128k",
			"-L", fmt.Sprintf("%dB", sizeByte),
			"-s",
			"-n", snapName,
			fmt.Sprintf("%s/%s", vg, originName),
		},
	}
}

// cmd example: lvcreate -L 1GB -s -n name_snap antstore-vg/origin-lv
func getCreateSnapshotLinearCmd(vg, snapName, originName string, sizeByte uint64) cmdArgs {
	return cmdArgs{
		cmd: "lvcreate",
		args: []string{
			"-L", fmt.Sprintf("%dB", sizeByte),
			"-s",
			"-n", snapName,
			fmt.Sprintf("%s/%s", vg, originName),
		},
	}
}

func getMergeSnapshotCmd(vg, snapName string) cmdArgs {
	return cmdArgs{
		cmd: "lvconvert",
		args: []string{
			"--merge", fmt.Sprintf("%s/%s", vg, snapName),
		},
	}
}

func getStripeLVCreateCmd(vg, lv string, sizeByte uint64, pvNum int) cmdArgs {
	return cmdArgs{
		cmd: "lvcreate",
		args: []string{
			"-y",
			"-L", fmt.Sprintf("%dB", sizeByte),
			"-i", strconv.Itoa(pvNum),
			"-I", "128k",
			"-n", lv,
			vg,
		},
	}
}

func getLvCreateCmd(vg, lv string, sizeByte uint64, logicSize string) cmdArgs {
	if logicSize != "" {
		return cmdArgs{
			cmd: "lvcreate",
			args: []string{
				"-y",
				"-l", logicSize,
				"-n", lv,
				vg,
			},
		}
	}

	return cmdArgs{
		cmd: "lvcreate",
		args: []string{
			"-y",
			"-L", fmt.Sprintf("%dB", sizeByte),
			"-n", lv,
			vg,
		},
	}
}

func getLvRemoveCmd(vg, lv string) cmdArgs {
	return cmdArgs{
		cmd: "lvremove",
		args: []string{
			"-y",
			fmt.Sprintf("%s/%s", vg, lv),
		},
	}
}

func handleFieldsOfEachLine(out []byte, lineFn func(cols []string) error) (err error) {
	var line string
	scanner := bufio.NewScanner(bytes.NewReader(out))
	// The split function defaults to ScanLines.
	for scanner.Scan() {
		line = scanner.Text()
		stripedLine := strings.TrimSpace(line)
		if len(stripedLine) > 0 {
			cols := strings.Fields(stripedLine)
			err = lineFn(cols)
			if err != nil {
				return err
			}
		}
	}

	var errRead = scanner.Err()
	if errRead != nil && errRead != io.EOF {
		err = errRead
		return
	}

	return
}
