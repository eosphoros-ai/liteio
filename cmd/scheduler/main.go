package main

import (
	"os"

	"lite.io/liteio/pkg/controller/manager/scheduler/plugin"
	"lite.io/liteio/pkg/version"
	"k8s.io/component-base/logs"
)

func main() {
	logs.InitLogs()
	defer logs.FlushLogs()

	version.PrintVersionInfo()

	cmd := plugin.NewSchedulerPluginCmd()
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
