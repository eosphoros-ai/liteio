# Architecture

## Components

LiteIO consists of six main components:

1. **Disk-Agent**: The Disk-Agent is installed on each backend node and manages the StoragePool on that node. It interacts with the data engine to create and delete volumes and snapshots. Additionally, the Disk-Agent reports the status of the StoragePool to the central control and collects volume metrics, which can be exposed as a Prometheus exporter.
2. **Disk-Controller**: The Disk-Controller is aware of all the StoragePools and Volumes in the cluster. Its primary responsibility is to schedule a requested volume to a suitable StoragePool.
3. **nvmf_tgt**: nvmf_tgt is the data engine based on SPDK, which provides storage abstraction and capabilities such as LVS (Logical Volume Store), LVOL (Logical Volume), aio_bdev, NoF over TCP transport, and NoF subsystems. While nvmf_tgt is optional, it is required if applications need storage beyond local disk. LiteIO also supports Linux LVM as a data engine, which is sufficient for local storage scenarios.
4. **nvme-tcp**: nvme-tcp is a kernel module that provides TCP transport for NVMe over fabrics. It must be installed on computing nodes.
5. **CSI-Driver**: LiteIO's CSI-Driver implements K8S CSI and is deployed as a DaemonSet pod on computing nodes. It utilizes nvme-cli tools to connect to backend storage.
6. **CSI-Controller**: The CSI-Controller is a central service that handles the creation and deletion of PVs.

Overall, LiteIO's architecture provides a scalable and efficient approach to cloud-native block storage. By utilizing multiple components and interfaces, LiteIO offers a flexible and configurable solution for various storage scenarios.

TODO IMAGE HERE


## Concepts

### StoragePool

A StoragePool represents the logical pooling of storage on a single node. Therefore, each node has its corresponding StoragePool. At present, StoragePool supports two pooling types: LVM VG and SPDK LVS.
The Disk-Agent discovers local pooling and creates a StoragePool custom resource (CR) and periodically performs heartbeating by updating the timestamp of a Lease in Kubernetes (K8s).
The Disk-Controller periodically checks heartbeats of StoragePools and updates their status.


### AntstorVolume

An AntstorVolume is the concrete entity of a PersistentVolume and is dynamically provisioned as demanded. An AntstorVolume is a logical volume allocated from a StoragePool. The Disk-Controller is responsible for scheduling a new AntstorVolume to a certain StoragePool, while the Disk-Agent manages the resource creation and deletion of the AntstorVolume.


### AntstorSnapshot

An AntstorSnapshot represents the snapshot entity of a volume. Two data engines (LVM and SPDK LVS) have different implementations of snapshots. Neither data engine is a distributed system; therefore, the snapshot has to be on the same node as the volume.

## Lifecycle of a Volume

### Creation

1. The user creates a Pod and PVC through the Kubernetes API and waits for the Pod to be scheduled.
2. After the Pod is successfully scheduled, the CSI-Controller receives a request of PV creation and invokes CreateVolume, which creates a new AntstorVolume according to the request's parameters.
3. The Disk-Controller finds a new AntstorVolume and schedules it to a suitable StoragePool.
4. The Disk-Agent is notified that a new AntstorVolume is scheduled on its node, and it starts to create a logical volume. If the pod is on another node, the Disk-Agent may expose the volume by NoF protocol.
5. CSI-Driver finds that the volume is ready and tries to connect the volume and mount the block device to the requested path.


### Deletion

1. The user submits a deletion request of Pod and PVC through the Kubernetes API.
2. Kubelet calls CSI-Driver to unmount and disconnect the volume.
3. The CSI-Controller receives a request of PV deletion and invokes DeleteVolume to mark a deletion timestamp of the AntstorVolume.
4. The Disk-Agent recycles all the resources of the volume and cleans AntstorVolume's finalizers.