# Plugins

Disk-Controller 主要是使用 controller-runtime 库构建而成。为了有效地进行卷调度，控制器必须了解整个集群中的所有存储池和卷。为了实现这一点，有两个调解者：StoragePoolReconciler 和 AntstorVolumeReconciler。

由于用户可能在不同的环境中运行 LiteIO，因此他们可能会对存储池和卷的调节有特定的要求。为了解决这个问题，LiteIO 提供了一个插件机制，包括调解者和调度器插件。

## Reconciler Plugin

插件包括三个方法：

- Name：返回插件的名称
- Reconcile：在调解周期中调用
- HandleDeletion：资源正在被删除时调用

Context 对象包含一个名为 Object 的字段，表示正在调解的对象，这个对象可以是存储池或 Antstor 卷的指针。

通过利用插件机制，用户可以轻松地定制 Disk-Controller 的行为，以满足他们的特定需求。这使得 LiteIO 成为一种高度灵活和可适应的存储管理解决方案。

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

一个常见需求是:通常需要将元数据导出到关系数据库，如 MySQL。LiteIO 认识到这个需求，并包括了一个内置的 MetaSyncPlugin，将存储池和卷同步到 MySQL 中。

MetaSyncPlugin 默认情况下会自动加载到 Disk-Controller 中。要使用此功能，只需设置节点磁盘控制器的操作命令的 `--dbInfo` 标志，并提供以 MySQL Go driver 格式编码的数据库连接信息的 base64 编码。例如：base64(user:passwd@tcp(ip_address:port)/dbname?charset=utf8&interpolateParams=true)。

### Develop Reconciler Plugin

1. 创建一个新的自定义插件结构体

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

2. 在主函数中注册它

```
controllers.RegisterPlugins([]controllers.PluginFactoryFunc{
		myplugin.NewCustomPlugin,
	}, nil)
```


## Scheduler Plugin

LiteIO 的调度过程借鉴了 Kubernetes 的调度器，并利用过滤和评分机制来简化和扩展调度过程。

### Filtering

当调度一个新的卷时，它首先通过 FilterChain 进行过滤。FilterChain 包含当前集群状态和一系列过滤器运算符，称为 PredicateFunc。PredicateFunc 定义如下：

```
type PredicateFunc func(*Context, *state.Node, *v1.AntstorVolume) bool
```

有两个内置的 PredicateFunc：

- Basic: 确保存储池有足够的资源来容纳即将到来的卷，并且存储池的状态是健康的。
- Affinity: 匹配卷的所需亲和性。

以下是使用 Affinity PredicateFunc 的示例：

一个卷有一个 Annotation obnvmf/pool-label-selector="A=B"，表示它与存储池具有亲和性。符合条件的存储池必须具有一个键为“A”且值为“B”的标签。语法遵循 Kubernetes 的 [LabelSelector](https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/) 。 还有另一个注解 key obnvmf/node-label-selector，用于匹配 Kubernetes 节点的标签。

用户可以通过以下三个步骤来开发和配置自己的 PredicateFunc 以定制卷调度。

1. 新建一个 PredicateFunc. e.g.

```
func CustomFilterFunc(ctx *Context, n *state.Node, vol *v1.AntstorVolume) bool {
    // custom filtering logic
    return false
}
```

2. 在主函数中注册

```
filter.RegisterFilter("Custom", myfilter.CustomFilterFunc)
```

3. 在 controller 配置中设置新增的 Filter

```
scheduler:
  maxRemoteVolumeCount: 3
  filters:
  - Basic
  - Affinity
  - Custom
```