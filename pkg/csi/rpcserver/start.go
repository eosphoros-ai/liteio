package rpcserver

import (
	"time"

	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/mount-utils"

	"lite.io/liteio/pkg/csi/client"
	"lite.io/liteio/pkg/csi/driver"
)

var DefaultBackOff = wait.Backoff{
	Duration: time.Second,
	Factor:   1.5,
	Steps:    20,
	Cap:      time.Minute * 2,
}

func StartServer(endpoint string, driver *driver.CSIDriver, mounter *mount.SafeFormatAndMount, cloudMgr client.AntstorClientIface, kubeCli kubernetes.Interface) {

	idendity := NewIdentityServer(driver)
	controller := NewControllerServer(driver, cloudMgr, kubeCli)
	node := NewNodeServer(driver, mounter, cloudMgr)

	s := NewGRPCServer()
	s.Start(endpoint, idendity, controller, node)
	s.Wait()
}
