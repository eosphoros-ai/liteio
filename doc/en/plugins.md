# Plugins

The Disk-Controller is primarily built using the controller-runtime library. In order to effectively schedule volumes, the controller must have knowledge of all StoragePools and Volumes across the entire cluster. To achieve this, there are two reconcilers in place: the StoragePoolReconciler and the AntstorVolumeReconciler.

Since users may be running LiteIO in different environments, they may have specific requirements for reconciling StoragePools and Volumes. To address this issue, LiteIO offers a plugin mechanism that includes reconciler and scheduler plugins.

## Import liteio to your project

`go.mod` example:

```
module your-project

go 1.17

require (
    code.alipay.com/dbplatform/node-disk-controller v1.0.0
)


replace code.alipay.com/dbplatform/node-disk-controller => github.com/eosphoros-ai/liteio v1.0.0
```

The above config will download mod from URL `github.com/eosphoros-ai/liteio@v1.0.0` and use it as mod `code.alipay.com/dbplatform/node-disk-controller`.
Remember to replace mod version `v1.0.0` with a real version name.


## Reconciler Plugin

The Plugin includes three methods:
- Name: which returns the name of the plugin
- Reconcile: which is called during the reconciling cycle
- HandleDeletion: which is called when a resource is being deleted

The Context object contains a field called Object, which represents the object being reconciled. This object can be a pointer to either a StoragePool or an AntstorVolume.

By leveraging the plugin mechanism, users can easily customize the behavior of the Disk-Controller to meet their specific needs. This makes LiteIO a highly flexible and adaptable solution for storage management.


```

type Context struct {
	KubeCli kubernetes.Interface
	Client  client.Client
	Ctx     context.Context
	Request ctrl.Request
	Object  runtime.Object
	State   state.StateIface
	Log     logr.Logger
}

type Plugin interface {
	Name() string
	Reconcile(ctx *Context) (result Result)
	HandleDeletion(ctx *Context) (err error)
}

```

### Example: Metadata Syncer

It is often necessary to export metadata to a relational database such as MySQL. LiteIO recognizes this need and has included a built-in MetaSyncPlugin that synchronizes StoragePools and Volumes to MySQL.

The MetaSyncPlugin is automatically loaded in the Disk-Controller by default. To use this feature, simply set the `--dbInfo` flag of the node-disk-controller's operator command and provide the base64-encoded DB connection info in MySQL Go driver's format. For example: base64(user:passwd@tcp(ip_address:port)/dbname?charset=utf8&interpolateParams=true).

### Develop Reconciler Plugin

1. Make a new custom Plugin struct

```
type CustomPlugin struct {}

func NewCustomPlugin(h *controllers.PluginHandle) (p plugin.Plugin, err error) {
    return &CustomPlugin{}, nil
}

func (p *CustomPlugin) Name() string {
	return "Custom"
}

func (p *CustomPlugin) Reconcile(ctx *plugin.Context) (result plugin.Result) {
	return plugin.Result{}
}

func (p *CustomPlugin) HandleDeletion(ctx *plugin.Context) (err error) {
	return
}

```

2. Register it in main function

```
controllers.RegisterPlugins([]controllers.PluginFactoryFunc{
		myplugin.NewCustomPlugin,
	}, nil)
```


## Scheduler Plugin

LiteIO's scheduler is based on Kubernetes' scheduler and leverages filtering and scoring mechanisms to simplify and extend the scheduling process.


### Filtering

When a new volume is being scheduled, it goes through the FilterChain as its first step. The FilterChain consists of the current cluster state and a series of filtering operators known as PredicateFunc. The PredicateFunc is defined as follows:
```
type PredicateFunc func(*Context, *state.Node, *v1.AntstorVolume) bool
```

There are two built-in PredicateFuncs:

- Basic: ensures that the StoragePool has enough resources for the incoming volume and that its status is healthy.
- Affinity: matches the required affinity of the volume.

Affinity usage example:

A volume has an annotation obnvmf/pool-label-selector="A=B", which indicates its affinity to the StoragePool. The qualified StoragePool must have a Label with the key "A" and value "B". The syntax follows Kubernetes' [LabelSelector](https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/) . There is also another annotation key, obnvmf/node-label-selector, which is used to match the labels of Kubernetes Nodes.

Users can customize volume scheduling by developing and configuring their own PredicateFunc using the following three steps.

1. Add a new PredicateFunc. e.g.

```
func CustomFilterFunc(ctx *Context, n *state.Node, vol *v1.AntstorVolume) bool {
    // custom filtering logic
    return false
}
```

2. Register the new filter in main function.

```
filter.RegisterFilter("Custom", myfilter.CustomFilterFunc)
```

3. Configure the filters in controller config

```
scheduler:
  maxRemoteVolumeCount: 3
  filters:
  - Basic
  - Affinity
  - Custom
```