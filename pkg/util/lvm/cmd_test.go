package lvm

import (
	"testing"

	utilmock "lite.io/liteio/pkg/generated/mocks/util"
	"github.com/stretchr/testify/assert"
)

var (
	vgsResult = `  VG        #PV #LV #SN Attr   VSize          VFree          VG UUID
	obnvmf-vg   1   1   0 wz--n- 4000783007744B 4000741064704B TJg2Kd-SYse-5ufS-Qj3r-wmLd-JlXd-r8JSyT
	vg00        2   1   0 wz--n- 8001566015488B 5802542759936B 0DcQZH-YQpg-N02W-o4Th-giBW-xBI2-A8u8oG`

	lvsResult = `  LV          VG        Attr       LSize          Pool Origin Data%  Meta%  Move Log Cpy%Sync Convert
	vol-test001 obnvmf-vg -wi-a-----      41943040B
	lvol0       vg00      -wi-a----- 2199023255552B`

	vgsResultJSON = `{
		"report": [
			{
				"vg": [
					{"vg_fmt":"lvm2", "vg_uuid":"LGJadr-puup-ECtb-cZf0-JGaE-nsis-txHVyv", "vg_name":"obnvmf-vg", "vg_attr":"wz--n-", "vg_permissions":"writeable", "vg_extendable":"extendable", "vg_exported":"", "vg_partial":"", "vg_allocation_policy":"normal", "vg_clustered":"", "vg_shared":"", "vg_size":"2639253209088B", "vg_free":"2638179467264B", "vg_sysid":"", "vg_systemid":"", "vg_lock_type":"", "vg_lock_args":"", "vg_extent_size":"4194304B", "vg_extent_count":"629247", "vg_free_count":"628991", "max_lv":"0", "max_pv":"0", "pv_count":"1", "vg_missing_pv_count":"0", "lv_count":"1", "snap_count":"0", "vg_seqno":"938", "vg_tags":"", "vg_profile":"", "vg_mda_count":"1", "vg_mda_used_count":"1", "vg_mda_free":"520192B", "vg_mda_size":"1044480B", "vg_mda_copies":"unmanaged"}
				]
			}
		]
	}`
	lvsResultJSON = `{
		"report": [
			{
				"lv": [
					{"lv_uuid":"u94UEL-1O6V-nBXM-sdM8-tdDV-UVTP-OItHe1", "lv_name":"testlv", "lv_full_name":"obnvmf-vg/testlv", "lv_path":"/dev/obnvmf-vg/testlv", "lv_dm_path":"/dev/mapper/obnvmf--vg-testlv", "lv_parent":"", "lv_layout":"linear", "lv_role":"public", "lv_initial_image_sync":"", "lv_image_synced":"", "lv_merging":"", "lv_converting":"", "lv_allocation_policy":"inherit", "lv_allocation_locked":"", "lv_fixed_minor":"", "lv_skip_activation":"", "lv_when_full":"", "lv_active":"active", "lv_active_locally":"active locally", "lv_active_remotely":"", "lv_active_exclusively":"active exclusively", "lv_major":"-1", "lv_minor":"-1", "lv_read_ahead":"auto", "lv_size":"1073741824B", "lv_metadata_size":"", "seg_count":"1", "origin":"", "origin_uuid":"", "origin_size":"", "lv_ancestors":"", "lv_full_ancestors":"", "lv_descendants":"", "lv_full_descendants":"", "raid_mismatch_count":"", "raid_sync_action":"", "raid_write_behind":"", "raid_min_recovery_rate":"", "raid_max_recovery_rate":"", "move_pv":"", "move_pv_uuid":"", "convert_lv":"", "convert_lv_uuid":"", "mirror_log":"", "mirror_log_uuid":"", "data_lv":"", "data_lv_uuid":"", "metadata_lv":"", "metadata_lv_uuid":"", "pool_lv":"", "pool_lv_uuid":"", "lv_tags":"", "lv_profile":"", "lv_lockargs":"", "lv_time":"2021-09-24 12:32:56 +0000", "lv_time_removed":"", "lv_host":"sqaob011163087248.stl", "lv_modules":"", "lv_historical":"", "lv_kernel_major":"252", "lv_kernel_minor":"0", "lv_kernel_read_ahead":"131072B", "lv_permissions":"writeable", "lv_suspended":"", "lv_live_table":"live table present", "lv_inactive_table":"", "lv_device_open":"", "data_percent":"", "snap_percent":"", "metadata_percent":"", "copy_percent":"", "sync_percent":"", "cache_total_blocks":"", "cache_used_blocks":"", "cache_dirty_blocks":"", "cache_read_hits":"", "cache_read_misses":"", "cache_write_hits":"", "cache_write_misses":"", "kernel_cache_settings":"", "kernel_cache_policy":"", "kernel_metadata_format":"", "lv_health_status":"", "kernel_discards":"", "lv_check_needed":"unknown", "lv_merge_failed":"unknown", "lv_snapshot_invalid":"unknown", "lv_attr":"-wi-a-----", "vg_name":"obnvmf-vg", "segtype":"linear"}
				]
			}
		]
	}`
)

func TestLvmCmd(t *testing.T) {
	mockExec := utilmock.NewShellExec(t)
	createCmd := getLvCreateCmd("vg", "lv", 1024, "")
	rmCmd := getLvRemoveCmd("vg", "lv")
	mockExec.On("ExecCmd", vgsCmd.cmd, vgsCmd.args).Return([]byte(vgsResult), nil)
	mockExec.On("ExecCmd", lvsCmd.cmd, lvsCmd.args).Return([]byte(lvsResult), nil)
	mockExec.On("ExecCmd", vgsCmdJson.cmd, vgsCmdJson.args).Return([]byte(vgsResultJSON), nil)
	mockExec.On("ExecCmd", lvsCmdJson.cmd, lvsCmdJson.args).Return([]byte(lvsResultJSON), nil)
	mockExec.On("ExecCmd", createCmd.cmd, createCmd.args).Return([]byte(""), nil)
	mockExec.On("ExecCmd", rmCmd.cmd, rmCmd.args).Return([]byte(""), nil)

	out, err := mockExec.ExecCmd(vgsCmd.cmd, vgsCmd.args)
	assert.NoError(t, err)
	assert.Equal(t, vgsResult, string(out))

	cmdObj := &cmd{
		exec: mockExec,
	}

	vglist, err := cmdObj.ListVG()
	assert.NoError(t, err)
	t.Logf("%+v", vglist)

	assert.Equal(t, uint64(8001566015488), vglist[1].TotalByte)
	assert.Equal(t, uint64(5802542759936), vglist[1].FreeByte)

	lvs, err := cmdObj.ListLVInVG("obnvmf-vg")
	assert.NoError(t, err)
	t.Logf("%+v", lvs)
	assert.Equal(t, uint64(41943040), lvs[0].SizeByte)

	vol, err := cmdObj.CreateLinearLV("vg", "lv", LvOption{Size: 1024})
	assert.NoError(t, err)
	assert.Equal(t, "lv", vol.Name)

	err = cmdObj.RemoveLV("vg", "lv")
	assert.NoError(t, err)

	// test json
	cmdObj.jsonFormat = true
	vglist, err = cmdObj.ListVG()
	assert.NoError(t, err)
	t.Logf("%+v", vglist)
	assert.Equal(t, uint64(2639253209088), vglist[0].TotalByte)
	assert.Equal(t, uint64(2638179467264), vglist[0].FreeByte)

	lvs, err = cmdObj.ListLVInVG("obnvmf-vg")
	assert.NoError(t, err)
	t.Logf("%+v", lvs)
	assert.Equal(t, uint64(1073741824), lvs[0].SizeByte)

}
