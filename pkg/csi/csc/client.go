package csicmd

import (
	"fmt"
	"os"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"google.golang.org/grpc"
)

func newCSINodeClient() (client csi.NodeClient, err error) {
	var opts []grpc.DialOption
	opts = append(opts, grpc.WithBlock(), grpc.WithInsecure())
	conn, err := grpc.Dial("unix://"+CSISocketFile, opts...)
	if err != nil {
		return
	}

	client = csi.NewNodeClient(conn)
	return
}

func exit(err error) {
	fmt.Printf("Error: %+v \n", err)
	os.Exit(1)
}

func getNodeID() string {
	if NodeID == "" {
		NodeID = os.Getenv("NODE_ID")
	}

	return NodeID
}
