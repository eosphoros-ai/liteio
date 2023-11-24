# CentOS Kernel Upgrade

## Check cgroup v2 support

Check if the kernel support cgroup2:
```
grep cgroup /proc/filesystems
```

If your system supports cgroupv2, you would see:
```
nodev   cgroup
nodev   cgroup2
```
On a system with only cgroupv1, you would only see:
```
nodev   cgroup
```


## Install Kernel
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
sudo yum --enablerepo=elrepo-kernel install kernel-lt

```


## Set grub2

```
# Get installed  kernel versions
awk -F\' /^menuentry/{print\$2} /etc/grub2.cfg

CentOS Linux 7 Rescue 3fbdb8b24a17e67dc83d346877013e3e (5.4.249-1.el7.elrepo.x86_64)
CentOS Linux (5.4.249-1.el7.elrepo.x86_64) 7 (Core)
CentOS Linux (3.10.0-1160.15.2.el7.x86_64) 7 (Core)
CentOS Linux (3.10.0-1160.el7.x86_64) 7 (Core)
CentOS Linux (0-rescue-cc2c86fe566741e6a2ff6d399c5d5daa) 7 (Core)


# Check grubenv  file
cat /boot/grub2/grubenv |grep saved
saved_entry=CentOS Linux (3.10.0-1160.15.2.el7.x86_64) 7 (Core)

# Set default kernel
grub2-set-default "CentOS Linux (5.4.249-1.el7.elrepo.x86_64) 7 (Core)"

# Rebuild Grub.cfg file
BIOS-Based:
grub2-mkconfig -o /boot/grub2/grub.cfg

UEFI-BASED:
grub2-mkconfig -o /boot/efi/EFI/redhat/grub.cfg

# restart
reboot
```

## Install Docker

```
sudo yum install -y yum-utils
sudo yum-config-manager --add-repo https://download.docker.com/linux/centos/docker-ce.repo
sudo yum install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin
sudo systemctl start docker
```

## Instal kind

```
# For AMD64 / x86_64
[ $(uname -m) = x86_64 ] && curl -Lo ./kind https://kind.sigs.k8s.io/dl/v0.20.0/kind-linux-amd64
chmod +x ./kind
sudo mv ./kind /usr/bin/kind

# create cluster
sudo kind create cluster --config kind-cluster.yaml
```

cluster config 
```
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
- role: control-plane
  # add a mount from /path/to/my/files on the host to /files on the node
  extraMounts:
  - hostPath: /local-storage
    containerPath: /local-storage
  - hostPath: /dev
    containerPath: /dev
```

## Install kubectl

```
curl -LO "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl"
chmod +x ./kubectl
sudo mv ./kubectl /usr/local/bin/kubectl
```

