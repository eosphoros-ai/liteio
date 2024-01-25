package plugin

import (
	"context"

	"lite.io/liteio/pkg/controller/manager/state"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Context struct {
	// per reconciler
	KubeCli kubernetes.Interface
	Client  client.Client
	State   state.StateIface
	Log     logr.Logger
	// per request
	ReqCtx RequestContent
}

type RequestContent struct {
	Ctx     context.Context
	Request ctrl.Request
	Object  runtime.Object
}

type Result struct {
	Result ctrl.Result
	Error  error
	Break  bool
}

func (r *Result) NeedBreak() bool {
	return r.Error != nil || r.Break
}

type Plugable interface {
	RegisterPlugin(...Plugin)
	Plugins() []Plugin
}

type Plugin interface {
	Name() string
	Reconcile(ctx *Context) (result Result)
	HandleDeletion(ctx *Context) (err error)
}

type PluginList struct {
	plugins []Plugin
}

func NewPluginList() Plugable {
	return &PluginList{
		plugins: []Plugin{},
	}
}

func (pe *PluginList) RegisterPlugin(p ...Plugin) {
	if len(p) > 0 {
		pe.plugins = append(pe.plugins, p...)
	}
}

func (pe *PluginList) Plugins() []Plugin {
	return pe.plugins
}
