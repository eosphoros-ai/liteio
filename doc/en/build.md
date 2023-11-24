# Build 


## Build LiteIO

### Prerequisites

- linux or MacOS
- golang >= 1.17


### AMD64

```
# build disk-controller and disk-agent
make controller

# build CSI Driver
make csi

# build scheduler plugin
make scheduler
```

### ARM64

the following command will build agents (build disk-agent and CSI Driver) for linux/arm64 and push the image to myregistry.com/LiteIO/node-disk-controller:latest

```
PLATFORMS=linux/arm64 IMAGE_ORG=myregistry.com/LiteIO TAG=latest make docker.buildx.agent
```


## Build SPDK

Recommend building SPDK on CentOS 7.9, which is well supported by SPDK community

```
git clone https://github.com/spdk/spdk.git
cd spdk
git checkout v22.05
git submodule update --init

# install dependencies
./scripts/pkgdep.sh
yum install python3-pyelftools meson -y
 
# remove libssl.so.1.1 if you don't want this runtime dependency
yum remove openssl11-devel
mv /lib64/libssl.so.1.1 /lib64/libssl.so.1.1-backup

# install numa libs if you want DPDK to support NUMA
yum install numactl-devel -y

# compile
./configure
make
```
