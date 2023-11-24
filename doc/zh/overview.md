# LiteIO 

**LiteIO** 是一个高性能,云原生的块设备服务，使用多种存储引擎，包括 SPDK 和 LVM。它专为超融合架构下的 Kubernetes 设计，允许在整个集群中进行块设备配置。

## Features

1. **高性能**: LiteIO 的数据引擎基于 SPDK 构建，并使用 NVMe-over-Fabric 协议将计算节点直接连接到存储节点。通过高效的协议和后端 I/O 轮询，LiteIO 提供接近本地磁盘的高性能。
2. **云原生**: LiteIO 通过 CSI 控制器和驱动程序与 Kubernetes 集成，提供云原生用户界面。用户可以使用 PVC 动态分配或销毁 LiteIO 卷。
3. **易于搭建**: 只需少量依赖项，就可以使用命令行快速搭建。
4. **超融合架构**: LiteIO 遵循超融合架构，其中单个节点可以同时充当前端和后端。初始化新集群时不存在最小节点数限制。

---

## Architecture

LiteIO 由六个主要组件组成:

1. **Disk-Agent**: Disk-Agent 安装在每个后端节点上，并管理该节点上的 StoragePool。它与数据引擎交互以创建和删除卷和快照。此外，Disk-Agent 报告 StoragePool 的状态给中央控制，并收集可以作为 Prometheus 出口器公开的卷指标。
2. **Disk-Controller**: Disk-Controller 管理集群中的所有 StoragePools 和 Volumes。它的主要责任是将请求的卷安排到适当的 StoragePool。
3. **nvmf_tgt**: nvmf_tgt 是基于 SPDK 的数据引擎，提供存储抽象和功能，例如 LVS（逻辑卷存储），LVOL（逻辑卷），aio_bdev，NoF over TCP 传输和 NoF 子系统。虽然 nvmf_tgt 是可选的，但如果应用程序需要超出本地磁盘的存储，则必须使用它。LiteIO 还支持 Linux LVM 作为数据引擎，这足以满足本地存储方案。
4. **nvme-tcp**: nvme-tcp 是提供 NVMe over fabrics 的 TCP 传输的内核模块。它安装在计算节点上。
5. **CSI-Driver**: LiteIO 的 CSI-Driver 实现了 K8S CSI，并作为 DaemonSet pod 部署在计算节点上。它利用 nvme-cli 工具连接到后端存储。
6. **CSI-Controller**: CSI-Controller 是一个中心服务，处理 PV 的创建和删除。

总体而言，LiteIO 的架构提供了一种可扩展和高效的云原生块存储方法。通过利用多个组件和接口，LiteIO 为各种存储场景提供了灵活和可配置的解决方案。

![](doc/image/architecture.png)

## Quick Start

TODO

## Performance Benchmark

### LiteIO vs Native Disk

The Performance Results of FIO with 1 Disk of Native Disks, LiteIO NoF, and OpenEBS Mayastor: (a) IOPS (b) Bandwidth.

Unit: IOPS(K)

|                        | Native-Disk | LiteIO | Mayastor |
|------------------------|-------------|----------|----------|
| 4k-rand w-dq16 4jobs   | 356.2       | 317.0    | 218.0    |
| 4k-rand w-dq1 1jobs    | 62          | 18       | 15       |
| 4k-rand r-dq128 8jobs  | 617.0       | 614.6    | 243.8    |
| 4k-rand r-dq1 1jobs    | 11.7        | 8.5      | 7.6      |
| 128k-seq r-dq128 4jobs | 24.9        | 24.8     | 19.7     |
| 128k-seq w-dq128 4jobs | 15.6        | 15.5     | 15.4     |


Unit: Bandwidth(MB/s)

|                        | Native-Disk | LiteIO | Mayastor |
|------------------------|-------------|----------|----------|
| 4k-rand w-dq16 4jobs   | 1459.6      | 1299.2   | 896.4    |
| 4k-rand w-dq1 1jobs    | 255.6       | 76.1     | 63.1     |
| 4k-rand r-dq128 8jobs  | 2528.0      | 2516.4   | 998.0    |
| 4k-rand r-dq1 1jobs    | 47.8        | 34.6     | 31.1     |
| 128k-seq r-dq128 4jobs | 3263.0      | 3271.0   | 2585.6   |
| 128k-seq w-dq128 4jobs | 2037.6      | 2030.0   | 2021.4   |

### LiteIO vs ESSD-PL3

4K Mixed Random Read/Write (70%/30%) IOPS with 1 Job

Unit: IOPS(K)

| Queue Depth | ESSD-PL3 | LiteIO |
|-------------|----------|----------|
| 1           | 5.0      | 6.0      |
| 4           | 20.9     | 23.4     |
| 16          | 83.3     | 84.9     |
| 128         | 206.1    | 333.9    |
| 256         | 206.4    | 426.2    |


## 目标场景

LiteIO 不是一种传统的通用分布式存储系统。它最适合需要类似本地磁盘的高 IO 性能的用户。例如，分布式数据库和 AI 训练作业受益于 LiteIO 提供本地和远程卷的能力。
LiteIO 专为 Kubernetes 设计，并允许用户利用所有节点上的存储。这使得它非常适合需要在 K8S 环境中运行应用程序的用户。
然而，需要注意的是，LiteIO 目前不支持数据复制。如果您的应用程序需要数据复制，请注意它在我们未来的开发计划中。与此同时，建议您的应用程序具有数据副本，并可以自行保证数据安全，或者您可以容忍数据丢失。

## Advanced Topics

- [Build Guide](doc/build.md)
- [How to Customize Plugins](doc/develop-plugins.md)


## Roadmap

- [x] Disk-Agent exposes metric service
- [ ] SPDK volume replica