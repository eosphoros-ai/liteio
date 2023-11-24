package misc

import (
	"io"
	"log"
	"net/rpc"
	"sync"
)

type RetryableRPCClient struct {
	ServerAddr string
	RpcCli     *rpc.Client
	lock       sync.Mutex
}

func NewRetryableRPCClient(addr string) (rpcCli *RetryableRPCClient, err error) {
	cli, err := rpc.DialHTTP("tcp", addr)
	if err != nil {
		return
	}

	rpcCli = &RetryableRPCClient{
		RpcCli:     cli,
		ServerAddr: addr,
	}
	return
}

func (cli *RetryableRPCClient) connectServer() (err error) {
	cli.lock.Lock()
	defer cli.lock.Unlock()

	cli.RpcCli, err = rpc.DialHTTP("tcp", cli.ServerAddr)
	if err != nil {
		log.Println(err)
	}
	return
}

func (cli *RetryableRPCClient) CallMethodOrReconnectServer(funcName string, args interface{}, reply interface{}) (err error) {
	if cli.RpcCli == nil {
		err = cli.connectServer()
		if err != nil {
			return
		}
	}

	err = cli.RpcCli.Call(funcName, args, reply)

	// reflect.TypeOf(err) == reflect.TypeOf((*rpc.ServerError)(nil)).Elem() ???
	if err == rpc.ErrShutdown || err == io.ErrUnexpectedEOF {
		// ShutDown error should be returned.
		log.Printf("Restarting RPC Connection due to error %+v", err)
		// re-connect server, so next Call may be successful.
		errReConn := cli.connectServer()
		if errReConn != nil {
			log.Printf("re-connect rpc server error: %+v", errReConn)
		}
	}

	return
}
