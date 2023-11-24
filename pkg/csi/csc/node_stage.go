package csicmd

import (
	"context"
	"fmt"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/spf13/cobra"
)

func init() {
	nodeStageVolumeCmd.Flags().StringVar(&nodeStageReq.stagingTargetPath,
		"staging-target-path",
		"",
		"The path to which to stage or unstage the volume")
}

var (
	nodeStageReq nodeStageVolume
)

type nodeStageVolume struct {
	nodeID            string
	stagingTargetPath string
	pubCtx            mapOfStringArg
	volCtx            mapOfStringArg
	caps              volumeCapabilitySliceArg
}

var nodeStageVolumeCmd = &cobra.Command{
	Use:   "stage",
	Short: `invokes the rpc "NodeStageVolume"`,
	Example: `
USAGE
	/BIN csi stage [flags] VOLUME_ID [VOLUME_ID...]
`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {

		req := csi.NodeStageVolumeRequest{
			StagingTargetPath: nodeStageReq.stagingTargetPath,
			PublishContext:    nodeStageReq.pubCtx.data,
			// Secrets:           root.secrets,
			VolumeContext:    nodeStageReq.volCtx.data,
			VolumeCapability: &csi.VolumeCapability{},
		}

		if len(nodeStageReq.caps.data) > 0 {
			req.VolumeCapability = nodeStageReq.caps.data[0]
		}

		for i := range args {
			ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
			defer cancel()

			// Set the volume ID for the current request.
			req.VolumeId = args[i]

			fmt.Println("StageVolume request", req)
			csiNode, err := newCSINodeClient()
			if err != nil {
				exit(err)
			}

			_, err = csiNode.NodeStageVolume(ctx, &req)
			if err != nil {
				return err
			}

			fmt.Println(args[i])
		}

		return nil
	},
}
