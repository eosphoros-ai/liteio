# 部署与使用

## 部署方法

LiteIO 需要部署在 K8S 环境中，并且有集群的管理员权限，需要至少一台可用节点。 如果希望使用远程后端存储, 节点需要提前安装 nvme-tcp 内核模块。
在项目根目录下，使用以下命令部署组件。

安装 nvme-tcp 模块
```
# for Debian and Ubuntu
apt -y install linux-modules-extra-$(uname -r) kmod

# install mod
modprobe nvme-tcp
```

安装 LiteIO 组件
```
kubectl apply -f hack/deploy/base
```

验证部署是否成功

1. 检查容器是否运行正常
```
kubectl -nobnvmf get pods -owide

显示结果
```

2. 检查 StorageClass 是否正确创建
```
kubectl get sc | grep nvmf

显示结果
```

## 使用方法

使用以下 yaml 文件创建容器和卷

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: obnvmf-test
  namespace: obnvmf
spec:
  volumes:
  - name: vol-pvc-obnvmf
    persistentVolumeClaim:
      claimName: pvc-obnvmf-test
  containers:
  - name: nginx
    image: nginx:latest
    volumeMounts:
    - name: vol-pvc-obnvmf
      mountPath: "/data"

---

apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: pvc-obnvmf-test
  namespace: obnvmf
spec:
  accessModes:
  - ReadWriteOnce
  resources:
    requests:
      storage: 10Gi
  storageClassName: antstor-nvmf

```

检查创建结果

```
显示结果
```