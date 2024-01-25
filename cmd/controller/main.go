package main

import (
	"os"

	antplugin "lite.io/liteio/cmd/controller/antplugins"
	antfilter "lite.io/liteio/cmd/controller/antplugins/filter"
	"lite.io/liteio/pkg/controller/manager/controllers"
	"lite.io/liteio/pkg/controller/manager/scheduler/filter"
	"lite.io/liteio/pkg/csi"
)

func main() {
	// add plugins
	controllers.RegisterPluginsInPoolReconciler([]controllers.PluginFactoryFunc{
		antplugin.NewReportLocalStoragePlugin,
	})
	controllers.RegisterPluginsInVolumeReconciler([]controllers.PluginFactoryFunc{
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
