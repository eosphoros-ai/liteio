package csicmd

import (
	"context"
	"fmt"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/spf13/cobra"
)

func init() {
	nodeUnstageVolumeCmd.Flags().StringVar(
		&nodeUnstageReq.stagingTargetPath,
		"staging-target-path",
		"",
		"The path to which to stage or unstage the volume")
}

var (
	nodeUnstageReq nodeUnstageVolume
)

type nodeUnstageVolume struct {
	stagingTargetPath string
}

var nodeUnstageVolumeCmd = &cobra.Command{
	Use:   "unstage",
	Short: `invokes the rpc "NodeUnstageVolume"`,
	Example: `
USAGE
	/BIN csi unstage [flags] VOLUME_ID [VOLUME_ID...]
`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {

		req := csi.NodeUnstageVolumeRequest{
			StagingTargetPath: nodeUnstageReq.stagingTargetPath,
		}

		for i := range args {
			ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
			defer cancel()

			// Set the volume ID for the current request.
			req.VolumeId = args[i]

			fmt.Println("nodeUnstageVolume request", req)
			csiNode, err := newCSINodeClient()
			if err != nil {
				exit(err)
			}

			_, err = csiNode.NodeUnstageVolume(ctx, &req)
			if err != nil {
				return err
			}

			fmt.Println(args[i])
		}

		return nil
	},
}
