package client

import (
	"encoding/json"
	"log"
	"net"
	"sync"
)

type JsonRpcClientIface interface {
	Close() (err error)
	Call(method string, params interface{}) (result []byte, err error)
}

type Client struct {
	UnixSocketFile string
	id             uint64
	conn           net.Conn
	lock           sync.Mutex
}

func NewClient(unixSocket string) (cli *Client, err error) {
	cli = &Client{
		UnixSocketFile: unixSocket,
	}
	cli.conn, err = net.Dial("unix", unixSocket)
	if err != nil {
		return
	}

	return
}

func (cli *Client) Close() (err error) {
	if cli.conn != nil {
		err = cli.conn.Close()
	}
	return
}

func (cli *Client) Call(method string, params interface{}) (result []byte, err error) {
	cli.lock.Lock()
	defer cli.lock.Unlock()

	cli.id += 1
	req := RPCRequest{
		RPCVersion: JSONRPCVersion,
		ID:         cli.id,
		Method:     method,
		Params:     params,
	}

	bs, _ := json.Marshal(req)
	log.Println(string(bs))
	_, err = cli.conn.Write(bs)
	if err != nil {
		return
	}

	var resp RPCResponse
	decoder := json.NewDecoder(cli.conn)
	decoder.Decode(&resp)

	result = resp.Result
	log.Println(string(result))
	if resp.Error.Code != 0 {
		err = resp.Error
	}

	return
}
