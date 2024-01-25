package client

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	v1 "lite.io/liteio/pkg/api/volume.antstor.alipay.com/v1"
	"lite.io/liteio/pkg/util/misc"
	uuid "github.com/satori/go.uuid"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
)

const (
	DataControlUuidPrefix = "dc-"

	PvTypeVolume      = "Volume"
	PvTypeVolumeGroup = "VolumeGroup"
)

func (cm *KubeAPIClient) CreatePV(opt PVCreateOption) (volId string, err error) {
	if opt.PvName == "" || opt.Size == 0 {
		err = fmt.Errorf("invalid request %+v", opt)
		return
	}
	klog.Info("create pv arguments: ", opt)

	// determine pv type
	if opt.PvType == "" {
		opt.PvType = PvTypeVolume
	}
	if opt.RaidLevel != "" {
		opt.PvType = PvTypeVolumeGroup
	}
	var (
		uuid = uuid.NewV4().String()
		size = uint64(opt.Size)
	)

	// requestSize MUST align to 4MiB
	if ret := size % fourMiB; ret > 0 {
		size = (size / fourMiB) * fourMiB
	}
	if size < fourMiB {
		err = fmt.Errorf("volume size too small, should be bigger than 4MiB")
		return
	}
	klog.Infof("Creating vol %s with size %d", opt.PvName, size)

	if opt.Labels == nil {
		opt.Labels = make(map[string]string)
	}
	opt.Labels[v1.VolumePVNameLabelKey] = opt.PvName

	switch opt.PvType {
	case PvTypeVolume:
		// check dupliate volume
		cli := cm.cli.VolumeV1().AntstorVolumes(defaultNamespace)
		tmpVol, err := cli.Get(context.Background(), opt.PvName, metav1.GetOptions{})
		if err != nil && !errors.IsNotFound(err) {
			klog.Error(err)
			return "", err
		}
		// found volume
		if err == nil && tmpVol != nil {
			return tmpVol.Spec.Uuid, nil
		}

		// create new Volume
		vol := v1.AntstorVolume{
			ObjectMeta: metav1.ObjectMeta{
				Namespace:   v1.DefaultNamespace,
				Name:        opt.PvName,
				Labels:      misc.CopyLabel(opt.Labels),
				Annotations: misc.CopyLabel(opt.Annotations),
			},
			Spec: v1.AntstorVolumeSpec{
				Uuid:           uuid,
				Type:           opt.VolumeType,
				SizeByte:       uint64(opt.Size),
				HostNode:       &opt.HostNode,
				PositionAdvice: v1.VolumePosition(opt.PositionAdvice),
			},
			Status: v1.AntstorVolumeStatus{
				Status: v1.VolumeStatusCreating,
			},
		}
		vol.Labels[v1.UuidLabelKey] = uuid

		// CreateVolume is idempotent in node-disk-controller RPC
		tmpVol, err = cli.Create(context.Background(), &vol, metav1.CreateOptions{})
		if err != nil {
			klog.Error(err)
			return "", err
		}
		return tmpVol.Spec.Uuid, nil
	case PvTypeVolumeGroup:
		// check and create volgroup and datacontrol
		var (
			dcUUID          = DataControlUuidPrefix + uuid
			volGroupName    = opt.PvName
			dataControlName = opt.PvName

			dcCli = cm.cli.VolumeV1().AntstorDataControls(defaultNamespace)
			vgCli = cm.cli.VolumeV1().AntstorVolumeGroups(defaultNamespace)

			volGroup *v1.AntstorVolumeGroup
			dataCtrl *v1.AntstorDataControl
		)
		// check volgroup
		volGroup, err = vgCli.Get(context.Background(), volGroupName, metav1.GetOptions{})
		if errors.IsNotFound(err) {
			// create volgroup
			volGroup = &v1.AntstorVolumeGroup{
				ObjectMeta: metav1.ObjectMeta{
					Namespace:   defaultNamespace,
					Name:        volGroupName,
					Labels:      misc.CopyLabel(opt.Labels),
					Annotations: misc.CopyLabel(opt.Annotations),
				},
				Spec: v1.AntstorVolumeGroupSpec{
					Uuid:      uuid,
					TotalSize: opt.Size,
					Stragety: v1.VolumeGroupStrategy{
						Name:           v1.StragetyBestFit,
						AllowEmptyNode: opt.AllowEmptyNode,
					},
					DesiredVolumeSpec: v1.DesiredVolumeSpec{
						Annotations: misc.CopyLabel(opt.Annotations),
						Labels:      misc.CopyLabel(opt.Labels),
						CountRange: v1.IntRange{
							Min: 1,
							Max: opt.MaxVolumes,
						},
						SizeRange: v1.QuantityRange{
							// TODO: validate opt.MinVolumeSize
							Min: resource.MustParse(opt.MinVolumeSize),
							Max: resource.MustParse(opt.MaxVolumeSize),
						},
						SizeSymmetry: v1.SymmetryValue(opt.SizeSymmetry),
					},
				},
			}
			volGroup.Labels[v1.DataControlNameKey] = dataControlName

			volGroup, err = vgCli.Create(context.Background(), volGroup, metav1.CreateOptions{})
			if err != nil {
				klog.Error(err)
				return
			}
		} else if err != nil {
			klog.Error(err)
			return
		}
		klog.Infof("successfully created AntstorVolumeGroup %+v", *volGroup)

		// check data control
		dataCtrl, err = dcCli.Get(context.Background(), dataControlName, metav1.GetOptions{})
		if errors.IsNotFound(err) {
			// create data control
			dataCtrl = &v1.AntstorDataControl{
				ObjectMeta: metav1.ObjectMeta{
					Namespace:   defaultNamespace,
					Name:        volGroupName,
					Labels:      misc.CopyLabel(opt.Labels),
					Annotations: misc.CopyLabel(opt.Annotations),
				},
				Spec: v1.AntstorDataControlSpec{
					UUID:       dcUUID,
					TotalSize:  opt.Size,
					EngineType: v1.PoolModeKernelLVM,
					Raid: v1.Raid{
						Level: v1.RaidLevel(opt.RaidLevel),
					},
					HostNode: opt.HostNode,
					VolumeGroups: []v1.EntityIdentity{
						{
							Namespace: defaultNamespace,
							Name:      volGroupName,
							UUID:      uuid,
						},
					},
				},
			}
			dataCtrl.Labels[v1.UuidLabelKey] = dcUUID

			dataCtrl, err = dcCli.Create(context.Background(), dataCtrl, metav1.CreateOptions{})
			if err != nil {
				klog.Error(err)
				return
			}
		} else if err != nil {
			klog.Error(err)
			return
		}
		klog.Infof("successfully created AntstorDataControl %+v", *dataCtrl)
		return dataCtrl.Spec.UUID, nil
	}

	return
}

func (cm *KubeAPIClient) GetPvByID(id string) (pv PV, err error) {
	labelSelector := fmt.Sprintf("%s=%s", v1.UuidLabelKey, id)

	if strings.HasPrefix(id, DataControlUuidPrefix) {
		var list *v1.AntstorDataControlList
		list, err = cm.cli.VolumeV1().AntstorDataControls(defaultNamespace).List(context.Background(), metav1.ListOptions{
			LabelSelector: labelSelector,
		})
		if err != nil {
			klog.Error(err)
			return
		}

		if len(list.Items) == 0 {
			err = ErrorNotFoundResource
			return
		}

		pv.Type = PvTypeVolumeGroup
		pv.DataContrl = &list.Items[0]
		pv.Name = pv.DataContrl.Name
		pv.Namespace = pv.DataContrl.Namespace
		pv.UUID = pv.DataContrl.Spec.UUID

	} else {
		var list *v1.AntstorVolumeList
		list, err = cm.cli.VolumeV1().AntstorVolumes(defaultNamespace).List(context.Background(), metav1.ListOptions{
			LabelSelector: labelSelector,
		})
		if err != nil {
			klog.Error(err)
			return
		}

		if len(list.Items) == 0 {
			err = ErrorNotFoundResource
			return
		}

		pv.Type = PvTypeVolume
		pv.Volume = &list.Items[0]
		pv.Name = pv.Volume.Name
		pv.Namespace = pv.Volume.Namespace
		pv.UUID = pv.Volume.Spec.Uuid
	}

	return
}

func (cm *KubeAPIClient) DeletePV(id string) (err error) {
	if id == "" {
		err = fmt.Errorf("invalid empty volID")
		return
	}

	var pv PV
	pv, err = cm.GetPvByID(id)
	if err == ErrorNotFoundResource {
		klog.Info("not found pv, consider it deleted. ", err)
		return nil
	} else if err != nil {
		klog.Error(err)
		return err
	}

	switch pv.Type {
	case PvTypeVolume:
		volCli := cm.cli.VolumeV1().AntstorVolumes(defaultNamespace)
		err = volCli.Delete(context.Background(), pv.Name, metav1.DeleteOptions{})
		if err != nil {
			klog.Error(err)
			return
		}
	case PvTypeVolumeGroup:
		// check and delete voluegroup
		var vgCli = cm.cli.VolumeV1().AntstorVolumeGroups(defaultNamespace)
		for _, item := range pv.DataContrl.Spec.VolumeGroups {
			_, err = vgCli.Get(context.Background(), item.Name, metav1.GetOptions{})
			if err == nil {
				delErr := vgCli.Delete(context.Background(), item.Name, metav1.DeleteOptions{})
				if delErr != nil {
					klog.Error(delErr)
					return delErr
				}
			} else if errors.IsNotFound(err) {
				klog.Infof("not found AntstorVolumeGroups %s, consider it deleted", item.Name)
			} else if err != nil {
				klog.Error(err)
				return
			}
		}

		// delete DataControl
		dcCli := cm.cli.VolumeV1().AntstorDataControls(defaultNamespace)
		err = dcCli.Delete(context.Background(), pv.Name, metav1.DeleteOptions{})
		if err != nil {
			klog.Error(err)
			return
		}
	}

	return
}

func (cm *KubeAPIClient) ResizePV(id string, requestSize int64) (err error) {
	var pv PV
	pv, err = cm.GetPvByID(id)
	if err != nil {
		klog.Error(err)
		return err
	}

	var originalSize = pv.GetSize()

	switch pv.Type {
	case PvTypeVolume:
		vol := pv.Volume
		vol.Spec.SizeByte = uint64(requestSize)
		if vol.Labels == nil {
			vol.Labels = make(map[string]string)
		}
		vol.Labels["obnvmf/expansion-original-size"] = strconv.Itoa(int(originalSize))
		_, err = cm.cli.VolumeV1().AntstorVolumes(pv.Namespace).Update(context.Background(), vol, metav1.UpdateOptions{})
		return
	case PvTypeVolumeGroup:
		// TODO:
		return fmt.Errorf("not support resizing VoluemGroup yet")
	}

	return
}

func (cm *KubeAPIClient) SetNodePublishParameters(req SetNodePublishParamRequest) (err error) {
	var pv PV
	pv, err = cm.GetPvByID(req.ID)
	if err != nil {
		klog.Error(err)
		return err
	}

	var csiNodePubParam v1.CSINodePubParams
	csiNodePubParam.CSIVolumeContext = make(map[string]string)
	for key, val := range req.CSIVolumeContext {
		csiNodePubParam.CSIVolumeContext[key] = val
	}
	csiNodePubParam.StagingTargetPath = req.StagingTargetPath
	csiNodePubParam.TargetPath = req.TargetPath

	switch pv.Type {
	case PvTypeVolume:
		volume := pv.Volume
		volume.Status.CSINodePubParams = &csiNodePubParam
		_, err = cm.cli.VolumeV1().AntstorVolumes(volume.Namespace).UpdateStatus(context.Background(), volume, metav1.UpdateOptions{})
		return
	case PvTypeVolumeGroup:
		dataControl := pv.DataContrl
		dataControl.Status.CSINodePubParams = &csiNodePubParam
		_, err = cm.cli.VolumeV1().AntstorDataControls(dataControl.Namespace).UpdateStatus(context.Background(), dataControl, metav1.UpdateOptions{})
		return
	}

	err = fmt.Errorf("not supported pv type")

	return
}
