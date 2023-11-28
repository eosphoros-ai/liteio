# Quick Start 

The Quick Start Guide helps you to seup a local K8S cluster and deploy LiteIO in it. It is only for testing purpose.

## Preparing Node

### Lightsail

Create a AWS Lightsail instance, which at least has `4 Cores`. OS should be `CentOS 7 2009-01`.

### Aliyun ECS

ECS Spec: 
```
CPU/Memory: 4C8G at least 
Image: CentOS 7.9 64bit
```


## Install dependencies

### cgroupv2

upgrade kernel to enable cgroupv2

```
# Enable the ELRepo Repository
sudo rpm --import https://www.elrepo.org/RPM-GPG-KEY-elrepo.org
sudo rpm -Uvh https://www.elrepo.org/elrepo-release-7.0-3.el7.elrepo.noarch.rpm

# List Available Kernels
yum list available --disablerepo='*' --enablerepo=elrepo-kernel
Available Packages
elrepo-release.noarch                                                       7.0-6.el7.elrepo                                               elrepo-kernel
kernel-lt.x86_64                                                            5.4.249-1.el7.elrepo                                           elrepo-kernel
...
kernel-ml.x86_64                                                            6.4.1-1.el7.elrepo                                             elrepo-kernel
...

# install the latest long-term support kernel
sudo yum -y --enablerepo=elrepo-kernel install kernel-lt
```

Set grub2 to use the new kernel

```
# Set default kernel
sudo grub2-set-default "CentOS Linux (5.4.249-1.el7.elrepo.x86_64) 7 (Core)"

# Rebuild Grub.cfg file
sudo grub2-mkconfig -o /boot/grub2/grub.cfg

sudo reboot
```

### Docker

```
sudo yum install -y yum-utils
sudo yum-config-manager --add-repo https://download.docker.com/linux/centos/docker-ce.repo
sudo yum install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin
sudo systemctl start docker
```

### kind

```
# For AMD64 / x86_64
[ $(uname -m) = x86_64 ] && curl -Lo ./kind https://kind.sigs.k8s.io/dl/v0.20.0/kind-linux-amd64
chmod +x ./kind
sudo mv ./kind /usr/bin/kind
```

### kubectl

```
curl -LO "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl"
chmod +x ./kubectl
sudo mv ./kubectl /usr/local/bin/kubectl
```

## Setup K8S Cluster

### K8S in docker container
```
# create cluster
sudo kind create cluster --config kind-cluster.yaml
```

kind-cluster.yaml file content:
```
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
- role: control-plane
  extraMounts:
  - hostPath: /local-storage
    containerPath: /local-storage
  - hostPath: /dev
    containerPath: /dev
```
cluster config need mapping `/dev` into kind's container, so that losetup is able to create loop device.


### K8S from kubeadm

Doc: [Setup K8S cluster by kubeadm](kubeadm-install.md)


## Install LiteIO


### Data Engine: LVM

```
# create components
kubectl create -f hack/deploy/base/

# create configmap
kubectl create -f hack/deploy/base/lvm

# list pods
kubectl -n obnvmf get pods
NAME                                      READY   STATUS    RESTARTS   AGE
csi-antstor-controller-79b44f5ccb-xr6nk   4/4     Running   0          13s
node-disk-controller-545c9b877f-vlzjq     1/1     Running   0          16s
obnvmf-disk-agent-app-hzpsg               1/1     Running   0          8m54s

# list storagepools
kubectl -n obnvmf get storagepool
NAME                 IP           HOSTNAME             STORAGE   FREE    STATUS   AGE
kind-control-plane   172.18.0.2   kind-control-plane   996Mi     996Mi   ready    19m

```

The deploying example uses LVM VG as pooling engine. The pooling engine could be set by agent-config.yaml in ConfigMap storage-setting.
```
agent-config.yaml: |
  storage:
    pooling:
      name: test-vg
      mode: KernelLVM
    pvs:
    - filePath: /data/pv01
      size: 1048576000 # 1GiB
```
The above config will create file /data/pv01 is it does not exist, create a loop device based on the file, and setup a VG named test-vg. 


#### Deploy Pod and PVC

```
kubectl create -f pod.yaml
pod/test-pod created
persistentvolumeclaim/pvc-obnvmf-test created
```

Verify the Pod and PVC are sucessfully created.

```
# verify that PVC is Bound
kubectl -n obnvmf get pvc
NAME              STATUS   VOLUME                                     CAPACITY   ACCESS MODES   STORAGECLASS   AGE
pvc-obnvmf-test   Bound    pvc-85d89012-e1d9-43bb-93f4-49cfbf5e71df   100Mi      RWO            antstor-nvmf   12s

# check antstorvolume status
kubectl -n obnvmf get antstorvolume
NAME                                       UUID                                   SIZE        TARGETID             HOST_IP      STATUS   AGE
pvc-85d89012-e1d9-43bb-93f4-49cfbf5e71df   f72b66d6-ff67-4123-8071-432005685a5d   104857600   kind-control-plane   172.18.0.2   ready    15s

# list pods
kubectl -n obnvmf get pods
NAME                                      READY   STATUS    RESTARTS      AGE
test-pod                                  1/1     Running   0             24s
csi-antstor-controller-79b44f5ccb-xr6nk   4/4     Running   0             106m
node-disk-controller-545c9b877f-vlzjq     1/1     Running   0             106m
obnvmf-csi-node-g2jx9                     3/3     Running   3 (93m ago)   94m
obnvmf-disk-agent-app-hzpsg               1/1     Running   0             115m

# list mounts in container
kubectl -n obnvmf exec -it test-pod sh
df -h
Filesystem                                                          Size  Used Avail Use% Mounted on
/dev/mapper/test--vg-pvc--85d89012--e1d9--43bb--93f4--49cfbf5e71df   95M  6.0M   89M   7% /test-data
```

#### Delete Pod and PVC

```
kubectl delete -f pod.yaml
pod "test-pod" deleted
persistentvolumeclaim "pvc-obnvmf-test" deleted

kubectl -n obnvmf get antstorvolume
No resources found in obnvmf namespace.

kubectl -n obnvmf get pvc
No resources found in obnvmf namespace.

kubectl get pv
No resources found
```


### Data Engine: SPDK LVS


```
# create components
kubectl create -f hack/deploy/base/

# create nvmf_tgt daemonset and configmap
kubectl create -f hack/deploy/base/aio-lvs
```

The disk-agent config will create a normal file `/local-storage/aio-lvs` of 1GiB and create an AIOBdev, which is used to create a LVS.
```
storage:
  pooling:
    name: aio-lvs
    mode: SpdkLVStore
  bdev:
    type: aioBdev
    name: test-aio-bdev
    size: 1048576000 # 1GiB
    filePath: /local-storage/aio-lvs
```

Verify the pods and storagepool are in correct status.
```
$kubectl -n obnvmf get storagepool
NAME                                              IP             HOSTNAME                                          STORAGE   FREE    STATUS   AGE
ip-172-26-10-67.ap-northeast-1.compute.internal   172.26.10.67   ip-172-26-10-67.ap-northeast-1.compute.internal   998Mi     998Mi   ready    64m

$kubectl -n obnvmf get storagepool ip-172-26-10-67.ap-northeast-1.compute.internal -oyaml
# we could see the LVS is correctly reported
spec:
  spdkLVStore:
    baseBdev: test-aio-bdev
    blockSize: 512
    bytes: 1046478848
    clusterSize: 2097152
    name: aio-lvs
    totalDataClusters: 499
    uuid: 1fe3f466-ef0c-4e1f-91c7-6499d3ec0bcd
status:
  capacity:
    storage: 998Mi
  conditions:
  - status: OK
    type: Spdk
  status: ready
  vgFreeSize: 998Mi
```

#### Deploy Pod and PVC

```
$ kubectl create -f pod.yaml
pod/test-pod created
persistentvolumeclaim/pvc-obnvmf-test created

$ kubectl -n obnvmf get pods
NAME                                      READY   STATUS    RESTARTS      AGE
csi-antstor-controller-79b44f5ccb-vv4gl   4/4     Running   0             111m
node-disk-controller-7d5d7d79f4-vw8lh     1/1     Running   0             108m
nvmf-tgt-gcr7g                            1/1     Running   0             54m
obnvmf-csi-node-4cn77                     3/3     Running   0             43m
obnvmf-disk-agent-app-h4v5z               1/1     Running   0             4s
test-pod                                  1/1     Running   0             14m

$ kubectl -n obnvmf get storagepool
NAME                                              IP             HOSTNAME                                          STORAGE   FREE    STATUS   AGE
ip-172-26-10-67.ap-northeast-1.compute.internal   172.26.10.67   ip-172-26-10-67.ap-northeast-1.compute.internal   998Mi     898Mi   ready    62m

$ kubectl -n obnvmf get antstorvolume
NAME                                       UUID                                   SIZE        TARGETID                                          HOST_IP        STATUS   AGE
pvc-71c89b02-651f-46da-aa68-78ee56e379e4   4cc0cb4f-8849-454f-a571-26e311d2ef25   104857600   ip-172-26-10-67.ap-northeast-1.compute.internal   172.26.10.67   ready    117s
```

Verify the volume is writable in the Pod.
```
kubectl -n obnvmf exec -it test-pod -- df -h
Filesystem      Size  Used Avail Use% Mounted on
overlay         320G  8.9G  312G   3% /
tmpfs            64M     0   64M   0% /dev
tmpfs           7.8G     0  7.8G   0% /sys/fs/cgroup
/dev/nvme1n1     95M  6.0M   89M   7% /test-data
/dev/nvme0n1p1  320G  8.9G  312G   3% /etc/hosts
shm              64M     0   64M   0% /dev/shm
tmpfs           1.0G   12K  1.0G   1% /run/secrets/kubernetes.io/serviceaccount
tmpfs           7.8G     0  7.8G   0% /proc/acpi
tmpfs           7.8G     0  7.8G   0% /proc/scsi
tmpfs           7.8G     0  7.8G   0% /sys/firmware

kubectl -n obnvmf exec -it test-pod -- sh -c "echo xxx > /test-data/testfile && cat /test-data/testfile && ls -alh /test-data/testfile"
xxx
-rw-r--r--. 1 root root 4 Jul 20 09:10 /test-data/testfile
```

#### Delete Pod and PVC

```
kubectl delete -f pod.yaml
pod "test-pod" deleted
persistentvolumeclaim "pvc-obnvmf-test" deleted

kubectl -n obnvmf get antstorvolume
No resources found in obnvmf namespace.

kubectl -n obnvmf get pvc
No resources found in obnvmf namespace.

kubectl get pv
No resources found
```
