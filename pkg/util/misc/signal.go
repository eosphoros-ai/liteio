package misc

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"k8s.io/klog/v2"
)

var onlyOneSignalHandler = make(chan struct{})

// SetupSignalHandler registered for syscall.SIGQUIT, syscall.SIGTERM, syscall.SIGSTOP, syscall.SIGINT which closing the stopSignal
func SetupSignalHandler(onExit func()) (stopCh chan struct{}) {
	close(onlyOneSignalHandler) // panics when called twice
	var (
		signalChan chan os.Signal
		s          os.Signal
	)
	stopCh = make(chan struct{})

	signalChan = make(chan os.Signal, 10)
	signal.Notify(signalChan, syscall.SIGHUP, syscall.SIGQUIT, syscall.SIGTERM,
		syscall.SIGINT, syscall.SIGSTOP, syscall.SIGPIPE, syscall.SIGCHLD, syscall.SIGUSR2)

	go func() {
		defer close(stopCh)
		defer close(signalChan)
		for {
			s = <-signalChan
			switch s {
			case syscall.SIGQUIT, syscall.SIGTERM, syscall.SIGSTOP, syscall.SIGINT:
				// close
				klog.Infof("Get a signal %+v to quit", s)
				fmt.Printf("Quiting process for signal=%+v, PID=%d \n", s, os.Getpid())
				if onExit != nil {
					onExit()
				}
				return
			case syscall.SIGHUP, syscall.SIGPIPE, syscall.SIGCHLD:
				klog.Infof("Got a signal %s", s.String())
				// reload ?
			case syscall.SIGUSR2:
				klog.Infof("Get a signal %+v to quit", s)
				fmt.Printf("Quiting process, PID=%d \n", os.Getpid())
				return
			}
		}
	}()

	return stopCh
}
