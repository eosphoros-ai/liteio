package main

import (
	"os"

	"code.alipay.com/dbplatform/node-disk-controller/pkg/controller/manager/scheduler/plugin"
	"code.alipay.com/dbplatform/node-disk-controller/pkg/version"
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
