package csicmd

import (
	"context"
	"fmt"
	"time"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/spf13/cobra"
)

func init() {
	nodeUnpublishVolumeCmd.Flags().StringVar(
		&nodeUnpublishReq.targetPath,
		"target-path",
		"",
		"The path to which to mount or unmount the volume")
}

type nodeUnpublishVolume struct {
	targetPath string
}

var (
	defaultTimeout   = 10 * time.Second
	nodeUnpublishReq nodeUnpublishVolume
)

var nodeUnpublishVolumeCmd = &cobra.Command{
	Use:     "unpublish",
	Aliases: []string{"umount", "unmount"},
	Short:   `invokes the rpc "NodeUnpublishVolume"`,
	Example: `
USAGE
    /BIN csi unpublish [flags] VOLUME_ID [VOLUME_ID...]
`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {

		req := csi.NodeUnpublishVolumeRequest{
			TargetPath: nodeUnpublishReq.targetPath,
		}

		for i := range args {
			ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
			defer cancel()

			// Set the volume ID for the current request.
			req.VolumeId = args[i]

			fmt.Println("nodeUnpublishVolume request", req)

			csiNode, err := newCSINodeClient()
			if err != nil {
				exit(err)
			}

			_, err = csiNode.NodeUnpublishVolume(ctx, &req)
			if err != nil {
				return err
			}

			fmt.Println(args[i])
		}

		return nil
	},
}
