# 架构简介

LiteIO 包含以下6个组件:

1. **Disk-Agent**: 负责管理单机节点上的存储池化(StoragePool), 卷管理，快照管理，上报心跳，采集监控信息
2. **Disk-Controller**: 负责管理集群内 StoragePool 状态，调度 Volume 到 StoragePool
3. **nvmf_tgt**: 用户态存储引擎，部署在存储节点，负责管理 SPDK LVS, LVOL, 提供NoF后端存储
4. **nvme-tcp**: 内核模块，部署在计算节点，
5. **CSI-Driver**: 对接了 K8S CSI 接口的组件，部署在前端计算节点， 使用 nvme-tcp 工具连接后端 Volume
6. **CSI-Controller**: K8S CSI 组件，中心化部署, 负责实际创建删除 PV

## Concepts

### StoragePool

StoragePool 表示在单个节点上的存储逻辑池。因此，每个节点都有其对应的 StoragePool。目前，StoragePool 支持两种池化类型：LVM VG 和 SPDK LVS。
Disk-Agent 发现本地池化，并创建一个 StoragePool 自定义资源（CR），并通过更新 Kubernetes（K8s）中租约的时间戳来定期执行心跳检测。
Disk-Controller 定期检查 StoragePools 的心跳，并更新它们的状态。


### AntstorVolume

AntstorVolume 是 PersistentVolume 的具体实体，根据需要动态分配。AntstorVolume 是从 StoragePool 分配的逻辑卷。Disk-Controller 负责将新的 AntstorVolume 调度到特定的 StoragePool，而 Disk-Agent 管理 AntstorVolume 的资源创建和删除。

### AntstorSnapshot

AntstorSnapshot 表示卷的快照实体。两个数据引擎（LVM 和 SPDK LVS）具有不同的快照实现。两个数据引擎都不是分布式系统，因此，快照必须在与卷相同的节点上。


## Lifecycle of a Volume

### Creation

1. 用户通过 Kubernetes API 创建一个 Pod 和 PVC，并等待 Pod 被调度。
2. Pod 成功调度后，CSI-Controller 收到 PV 创建请求并调用 CreateVolume，在请求的参数下创建一个新的 AntstorVolume。
3. Disk-Controller 找到一个新的 AntstorVolume 并将其调度到适当的 StoragePool。
4. Disk-Agent 收到通知，将新的 AntstorVolume 安排在其节点上，并开始创建逻辑卷。如果 Pod 在另一个节点上，则 Disk-Agent 可能通过 NoF 协议公开该卷。
5. CSI-Driver 发现卷已准备就绪，并尝试连接卷并将块设备挂载到请求的路径。


### Deletion

1. 用户通过 Kubernetes API 提交删除 Pod 和 PVC 的请求。
2. Kubelet 调用 CSI-Driver 卸载和断开连接卷。
3. CSI-Controller 收到 PV 删除请求并调用 DeleteVolume 标记 AntstorVolume 的删除时间戳。
4. Disk-Agent 回收该卷的所有资源并清除 AntstorVolume 的 finalizers。