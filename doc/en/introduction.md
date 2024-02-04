# Ant Group's decentralized high-performance storage service LiteIO is officially open-source

In the era when traditional distributed storage was prevalent, LiteIO, as a representative of point-to-point block device services, has brought great business and technical benefits in Ant Group's internal practices. Recently, the paper "LightPool: A NVMe-oF-based High-performance and Lightweight Storage Pool Architecture for Cloud-Native Distributed Database", jointly written by the Alibaba Cloud software and hardware research and development team and Ant Group's database technology team, was accepted by the HPCA '24 paper review process. The technical advancements of LiteIO were also recognized by the CCF-A class paper. 

Taking this opportunity, **we are very honored to announce that Ant Group's decentralized high-performance storage service LiteIO is officially open source.** By sharing our technology with developers worldwide, we aim to inspire more creativity and ideas. We believe that products with statefulness and multi-replica capabilities at the product layer, such as databases and storage products, need point-to-point technologies like LiteIO to adapt to current FinOps.

We hope to attract more community ideas, only through openness and collaboration can LiteIO overcome more challenges, solve more problems, and bring more value to users, which makes the LiteIO project go further and become a standard paradigm for storage usage in the cloud-native era.

- Open Source project warehouse: https://github.com/eosphoros-ai/liteio

# What Is LiteIO?

![横版组合标.png](https://intranetproxy.alipay.com/skylark/lark/0/2024/png/162424/1706753759142-85c7d65b-c479-458e-b321-936851f874a7.png#clientId=uc6b5bffa-c969-4&from=ui&id=u328b3307&originHeight=1000&originWidth=2000&originalType=binary&ratio=1&rotation=0&showTitle=false&size=86246&status=done&style=none&taskId=u3fdb5574-b547-4b69-86d6-964c3769987&title=)

LiteIO is a **high-performance and scalable cloud-native block device services**, specifically designed for Kubernetes in hyper-converged architecture. It has been incubated within Ant Group for 3 years and has been widely used in the production environment, providing stable, efficient and scalable disk services for Ant Group's entire data-based and storage-based products.

LiteIO is a common technology that shares local disks and logical volumes to other remote servers through the network, combining Kubernetes scheduling in cloud-native environments to manage and pool a series of disks uniformly. Compared with traditional distributed storage, the point-to-point technology design effectively controls the explosion radius caused by hardware failures while eliminating storage redundancy, thus providing more usable space.

## **Design Background**

In the era of cost reduction and efficiency enhancement, FinOps is particularly important, especially for large-scale systems like Ant Group, which has a vast storage server infrastructure. Even a mere 1% increase in global storage utilization can yield substantial cost-economical benefits. Therefore, it is necessary to ensure stability without compromising on cost optimization and generalizability.

Databases are IO-intensive software systems that require exceptional stability and performance in terms of IO. Generally, production systems deploy databases on local disk servers, which will cause two issues:

- **Uneven Utilization**: I/O-intensive and compute-intensive workload vary, leading to scenarios where one machine may be fully utilized for computation while storage remains underutilized, or vice versa. Moreover, attaining a globally optimal solution through scheduling is a considerable challenge.
- **Poor Scalability**: When the storage is insufficient and the storage needs to be scale up, it becomes essential to migrate to a server with a larger storage capacity, which takes a long time to copy data.



Traditional distributed storage represents a decent solution, but within the domain of databases, it introduces several problems:

- **Ascension In Replication Count**(Cost): The advantage of distributed storage lies in the pooling storage through erasure coding (EC) and multi-replica techniques, which offer robust protection against single hardware failures. However, under this architecture, the application of EC and multi-replica results in a replication factor greater than 1 for each data segment, usually between 1.375 and 2. As an important component of business services, databases often necessitate geo-redundancy and disaster recovery across different availability zones (AZ) at the upper layer, with a backup replication already existing in another AZ. The total number of data replicas is set to rise.
- **Large Explosion Radius**(Stability): Distributed storage typically features a centralized metadata layer, when subject to failure, can lead to global exceptions.

## **Design Ideas**

LiteIO adopts a decentralized design paradigm, utilizing the SPDK data engine and the high-performance NVMe-over-Fabric protocol to connect computing nodes directly to remote storage nodes. Through efficient protocol and backend I/O polling, it provides high performance close to local disks. Point-to-point design, in conjunction with Kubernetes' scheduling control, effectively mitigates the impact of single hardware failures on services.

![liteio1.png](https://intranetproxy.alipay.com/skylark/lark/0/2024/png/162424/1706753898600-dc3c0ed5-6296-49b2-8b1d-4a86b81dd5d0.png#clientId=uc6b5bffa-c969-4&from=ui&id=u90530bb2&originHeight=412&originWidth=750&originalType=binary&ratio=1&rotation=0&showTitle=false&size=101068&status=done&style=none&taskId=uf1e7890a-301e-446a-a37a-7c6af3abec2&title=)

## **FinOps**

Based on LiteIO, unallocated storage within servers can be dynamically distributed to remote compute nodes on demand, while globally coordinated scheduling pools global storage resources, thereby enhancing overall storage efficiency.

For example, there are two types of servers: compute-intensive 96C 4T and storage-intensive 64C 20T. Assuming that the CPU of the storage model has been allocated, with 5T disk space left, while the compute model still has avaiable CPU but no disks to allocate. Using LiteIO, it is possible to combine the CPU of the compute model with the remaining disk space of the storage model into a new container to provide services, thus enhancing the efficiency of both computational and storage capacities.

## General Storage And Computing Separation

LiteIO is a general storage service technology, which acts on storage logic volumes, in conjunction with K8S, the storage perceived by upper-layer containers or applications is indistinguishable from that of local disks. Whether it is direct read/write to block devices (bdev) or formatting the block devices into any file system, no modifications are required from upper-layer services. Databases such as OceanBase, MySQL, PostgreSQL, or applications written in Java, Python, or Go can utilize it as a regular disk.

## **Serverless**

LiteIO's general storage and computing separation capability simplifies scaling dramatically. With the perception and scheduling system, deploying a MySQL instance inherently gains serverless capability. When the computing power of MySQL is insufficient, one can rapidly achieve scale-up by attaching MySQL storage to a more powerful container via LiteIO. When storage space of MySQL is insufficient, simply mounting an additional disk from another storage node allows for expansion without data loss.

# **Technical Features**

## **High Performance Protocol**
LitelO uses the NVMe-oF protocol to improve performance. NVME-oF protocol can take full advantage of the inherent parallelism and multi-queue functions of cutting-edge NVMe devices. In contrast, when accessing NVMe SSDs, iSCSI incurs a performance loss of up to 40%, and additional operations such as protocol conversion consume over 30% of CPU resources. NVMe-oF outperforms iSCSI, offering performance that is comparable to locally connected NVMe SSDs. Consequently, the adoption of NVMe-oF in LiteIO minimizes overhead when accessing storage resources within a pool, delivering high-performance akin to that of native disks. LiteIO utilizes NVMe over Fabric (TCP) as its remote storage protocol, facilitating access to storage resources by other nodes within the cluster.

## **Simplified IO Pipeline**
In the traditional distributed storage architecture, a Write IO operation involves three steps: querying metadata, writing metadata, and writing multiple data replicas, which requires numerous network interactions. In the LitelO architecture, due to the single-replica mechanism, point-to-point access, and one-to-one mapping between frontend bdevs and backend volumes, no additional rootserver or metaserver is required to manage global metadata. The IO path entails only a single network traverse, eliminating the data transmission delays and amplification issues associated with multiple replica writes. which makes LitelO have higher I/O throughput and lower I/O latency.

## **Zero Copy**

When accessing local disks, I/O requests and data are transmitted between NoF-Initiator and NoF-Engine through tcp-loopback, but this process involves several redundant data copies. To eliminate these copies and reduce CPU overhead, we propose an innovative zero copy transmission method for local access to LiteIO. For I/O requests, zero copy transmission uses shared memory between NoF-Initiator and NoF-Engine. For data, we propose a DMA remapping mechanism that allows local storage devices to directly access the application buffers. Zero copy transmission discards the TCP stack, eliminates redundant data copies between the user buffers and the kernel, and achieves near-native performance when accessing local storage resources.

![liteio2.png](https://intranetproxy.alipay.com/skylark/lark/0/2024/png/162424/1706754019764-d9f8ab39-468d-40aa-b3cd-97a24213fdca.png#clientId=uc6b5bffa-c969-4&from=ui&id=u5c769f88&originHeight=726&originWidth=868&originalType=binary&ratio=1&rotation=0&showTitle=false&size=79032&status=done&style=none&taskId=uef8910e5-404d-4a48-b35b-57d46e71c55&title=)

## **Hot Upgrade**

Fully considering that LiteIO, as a key link in the data pipeline, will also require functional upgrades, we have implemented a seamless upgrade process for LiteIO that ensures the frontend business remains unaware of the change, with minimal IO disruption (<100ms), and nvme disk symbol mounted on the host does not experience any alterations.

The overall framework of Target, as depicted below, necessitates the uninterrupted maintenance of the NVMe over Fabrics (nvmf) network connection during hot upgrades. Otherwise, the host side will perceive and reconnect or delete the disk symbol. The hot upgrade is implemented by forking a new target process from the existing one and loading the new binary, ensuring that no IO is lost throughout the process. The switchover between the old and new processes should be swift. Based on the simplicity design principle of the hot upgrade framework, during the hot upgrade, the green TCP or RDMA connections illustrated in the diagram below represent the context that must be maintained. Other modules do not need to save the context state. The maintenance of network connections is achieved through the inheritance of file descriptors by the parent and child processes.

![liteio3.png](https://intranetproxy.alipay.com/skylark/lark/0/2024/png/162424/1706754053014-8a6ccb5c-6ce8-4daf-89e1-9c424b3e521f.png#clientId=uc6b5bffa-c969-4&from=ui&id=uc041224e&originHeight=772&originWidth=1406&originalType=binary&ratio=1&rotation=0&showTitle=false&size=313927&status=done&style=none&taskId=u5a5c8d03-03ae-4bb0-b688-0d5cd928728&title=)

## **Hot Migration**

The volume hot migration feature is designed to transfer volume data from the original Target to a new Target without affecting the business. Once the migration is complete, the Host completes the link switching, thereby enabling a seamless transition of the business to the new Target without any perceived interruption.

During hot migration, the approach employed to transmit data from the original Target to the new Target is through a multi-phase iterative cycle. Prior to each iteration, a data map is extracted from the volume, and data is copied in accordance with this map. During the first iteration, a considerable data may need to be copied. In subsequent iterations, only the data that has been newly modified (write or discard) during the preceding migration phase needs to be copied. Providing that the migration bandwidth exceeds the bandwidth of new write data, after several rounds of replication, the data discrepancy between the original Target and the new Target will progressively diminish, enabling the final round of copying to be performed with IO operations halted.

## **Snapshot**

LiteIO has interfaced with CSI's snapshot-related APIs, allowing users to create snapshots of LiteIO volumes using the Kubernetes community's Snapshot resources. The snapshot capability is dependent on the underlying data engine, and LiteIO supports two types of engines: LVM and SPDK. The snapshot functionality for LVM is provided by VG/LV, while the NoF-Engine's snapshot capability is offered by SPDK LVS. Both LVM and SPDK are stand-alone engines, which necessitate that the Snapshot and the original LV reside on the same machine. This implies that when creating the original LV, it is essential to reserve a certain amount of space for the snapshot. If no space is reserved, there is no guarantee that the snapshot creation will succeed.

LiteIO has integrated with CSI's `ExpandVolume` interface, enabling users to implement online disk expansion by modifying the PVC disk space. For LVM engine, LV expansion can be achieved without alterations. In the process of exposing remote disks for the NoF-Engine, a new RPC call named `bdev_aio_resize` has been added, facilitating online expansion of remote disks. There are also limitations to expansion, similar to snapshots, due to the fact that both LVM and SPDK are stand-alone engines, which do not guarantee sufficient space on the node for expansion.

## **Multi-Disk**

In the point-to-point data link mode, it is inevitable that some storage resource fragments will arise. LiteIO is capable of aggregating these fragments into a single volume for business utilization. This approach introduces a higher failure rate issue: if any node providing the fragments fails, the volume becomes unavailable. Fortunately, there is an internal business, LDG, which can tolerate such a failure rate, thus making the most of the resources. LDG (Logic Data Guard) is designed to construct a routine logical primary-standby database, offering a one-stop lifecycle management and application control platform for the primary-standby databases. To enhance stability and mitigate data risks during operations such as upgrades, maintenance, and accidents, LDG also aims to avert these risks while enhancing data manipulation capabilities.

## **Thin Provisioning**

LitelO also offers Thin Provisioning capabilities, which enable over-provisioning of storage at the single-machine dimension, making it suitable for storage products like MySQL that do not pre-allocate storage space. Coupled with hot migration capabilities, Thin Provisioning allows for rapid and seamless data migration to nodes with available space when the storage capacity of a single machine is insufficient. Since LitelO is not a distributed storage architecture, the use of Thin Provisioning requires precise control of the over-provisioning ratio and the total amount of over-provisioned resources to ensure that data can be quickly migrated in the event of space constraints, thus preventing damage to business.

# **Practice**

LiteIO is extensively utilized across tens of thousands of production servers at Ant Group, enhancing storage efficiency by 25% and significantly optimizing resource service costs. Compared to local storage, the additional IO latency introduced by LiteIO is merely around 2.1 microseconds. Its general storage and computing separation architecture not only services database products but also extends storage computing separation and Serverless capabilities to other computational products and application services within Ant Group. Coupled with features such as hot upgrades, hot migration, and the Kubernetes ecosystem, it ensures that daily operations do not entail any additional maintenance burdens. Capabilities like snapshot and multi-disk aggregation provide greater flexibility and a variety of usage scenarios. Stay tuned for a series of articles sharing the best practices of LiteIO at Ant Group.

# **Join us**

Are you contemplating the FinOps project aimed at reducing costs and enhancing efficiency? Are you also considering the design of a general storage computing separation architecture? Are you an enthusiast of storage technology? Welcome to join the LiteIO open-source community, where your participation is eagerly anticipated.

- Open Source project warehouse: https://github.com/eosphoros-ai/liteio
