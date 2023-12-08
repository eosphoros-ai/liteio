package main

import (
	"os"

	antplugin "code.alipay.com/dbplatform/node-disk-controller/cmd/controller/antplugins"
	antfilter "code.alipay.com/dbplatform/node-disk-controller/cmd/controller/antplugins/filter"
	"code.alipay.com/dbplatform/node-disk-controller/pkg/controller/manager/controllers"
	"code.alipay.com/dbplatform/node-disk-controller/pkg/controller/manager/scheduler/filter"
	"code.alipay.com/dbplatform/node-disk-controller/pkg/csi"
)

func main() {
	// add plugins
	controllers.RegisterPlugins([]controllers.PluginFactoryFunc{
		antplugin.NewReportLocalStoragePlugin,
	}, []controllers.PluginFactoryFunc{
		antplugin.NewReportLocalStoragePlugin,
		antplugin.NewPatchPVPlugin,
	})

	// add filters
	filter.RegisterFilter("ObReplica", antfilter.ObReplicaFilterFunc)

	cmd := controllers.NewApplicationCmd()
	// add CSI command
	cmd.AddCommand(csi.NewCSICommand())

	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}

}
