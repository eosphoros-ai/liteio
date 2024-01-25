module lite.io/liteio

go 1.16

require (
	github.com/container-storage-interface/spec v1.5.0
	github.com/didi/gendry v1.4.0
	github.com/go-logr/logr v1.2.0
	github.com/go-sql-driver/mysql v1.4.1
	github.com/go-xorm/xorm v0.7.9
	github.com/golang/protobuf v1.5.2
	github.com/kubernetes-csi/csi-lib-utils v0.9.0
	github.com/prometheus/client_golang v1.12.1
	github.com/satori/go.uuid v1.2.0
	github.com/spf13/cobra v1.4.0
	github.com/spf13/pflag v1.0.5
	github.com/stretchr/testify v1.8.1
	github.com/toolkits/file v0.0.0-20160325033739-a5b3c5147e07 // indirect
	github.com/toolkits/nux v0.0.0-20200401110743-debb3829764a
	github.com/toolkits/slice v0.0.0-20141116085117-e44a80af2484 // indirect
	github.com/toolkits/sys v0.0.0-20170615103026-1f33b217ffaf // indirect
	go.uber.org/zap v1.19.1
	golang.org/x/crypto v0.2.0 // indirect
	golang.org/x/net v0.4.0
	golang.org/x/sys v0.3.0
	golang.org/x/time v0.2.0 // indirect
	google.golang.org/grpc v1.40.0
	google.golang.org/protobuf v1.27.1
	gopkg.in/yaml.v2 v2.4.0
	k8s.io/api v0.24.7
	k8s.io/apimachinery v0.24.7
	k8s.io/client-go v0.24.7
	k8s.io/component-base v0.24.7
	k8s.io/component-helpers v0.24.7
	k8s.io/klog/v2 v2.60.1
	k8s.io/kubernetes v1.24.7
	k8s.io/mount-utils v0.24.7
	k8s.io/utils v0.0.0-20220210201930-3a6ce19ff2f9
	sigs.k8s.io/controller-runtime v0.12.3
)

replace (
	k8s.io/api => k8s.io/api v0.24.7
	k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.24.7
	k8s.io/apimachinery => k8s.io/apimachinery v0.24.7
	k8s.io/apiserver => k8s.io/apiserver v0.24.7
	k8s.io/cli-runtime => k8s.io/cli-runtime v0.24.7
	k8s.io/client-go => k8s.io/client-go v0.24.7
	k8s.io/cloud-provider => k8s.io/cloud-provider v0.24.7
	k8s.io/cluster-bootstrap => k8s.io/cluster-bootstrap v0.24.7
	k8s.io/code-generator => k8s.io/code-generator v0.24.7
	k8s.io/component-base => k8s.io/component-base v0.24.7
	k8s.io/component-helpers => k8s.io/component-helpers v0.24.7
	k8s.io/controller-manager => k8s.io/controller-manager v0.24.7
	k8s.io/cri-api => k8s.io/cri-api v0.24.7
	k8s.io/csi-translation-lib => k8s.io/csi-translation-lib v0.24.7
	k8s.io/kube-aggregator => k8s.io/kube-aggregator v0.24.7
	k8s.io/kube-controller-manager => k8s.io/kube-controller-manager v0.24.7
	k8s.io/kube-proxy => k8s.io/kube-proxy v0.24.7
	k8s.io/kube-scheduler => k8s.io/kube-scheduler v0.24.7
	k8s.io/kubectl => k8s.io/kubectl v0.24.7
	k8s.io/kubelet => k8s.io/kubelet v0.24.7
	k8s.io/legacy-cloud-providers => k8s.io/legacy-cloud-providers v0.24.7
	k8s.io/metrics => k8s.io/metrics v0.24.7
	k8s.io/mount-utils => k8s.io/mount-utils v0.24.7
	k8s.io/pod-security-admission => k8s.io/pod-security-admission v0.24.7
	k8s.io/sample-apiserver => k8s.io/sample-apiserver v0.24.7
)
