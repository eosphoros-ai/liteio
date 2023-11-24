package csicmd

import (
	"context"
	"fmt"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/spf13/cobra"
)

func init() {
	nodePublishVolumeCmd.Flags().StringVar(
		&nodePublishReq.stagingTargetPath,
		"staging-target-path",
		"",
		"The path to which to stage or unstage the volume")

	nodePublishVolumeCmd.Flags().StringVar(
		&nodePublishReq.targetPath,
		"target-path",
		"",
		"The path to which to mount or unmount the volume")

	nodePublishVolumeCmd.Flags().Var(
		&nodePublishReq.volCtx,
		"vol-context",
		`One or more key/value pairs may be specified to send with
			the request as its VolumeContext field:
				--vol-context key1=val1,key2=val2 --vol-context=key3=val3`)
}

var (
	nodePublishReq nodePublishVolume
)

type nodePublishVolume struct {
	targetPath        string
	stagingTargetPath string
	pubCtx            mapOfStringArg
	volCtx            mapOfStringArg
	attribs           mapOfStringArg
	readOnly          bool
	caps              volumeCapabilitySliceArg
}

var nodePublishVolumeCmd = &cobra.Command{
	Use:     "publish",
	Aliases: []string{"mnt", "mount"},
	Short:   `invokes the rpc "NodePublishVolume"`,
	Example: `
USAGE
	/BIN csi publish [flags] VOLUME_ID [VOLUME_ID...]
`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {

		req := csi.NodePublishVolumeRequest{
			StagingTargetPath: nodePublishReq.stagingTargetPath,
			TargetPath:        nodePublishReq.targetPath,
			PublishContext:    nodePublishReq.pubCtx.data,
			Readonly:          nodePublishReq.readOnly,
			VolumeContext:     nodePublishReq.volCtx.data,
			VolumeCapability:  &csi.VolumeCapability{},
		}

		if len(nodePublishReq.caps.data) > 0 {
			req.VolumeCapability = nodePublishReq.caps.data[0]
		}

		for i := range args {
			ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
			defer cancel()

			// Set the volume ID for the current request.
			req.VolumeId = args[i]

			fmt.Println("PublishVolume request", req)
			csiNode, err := newCSINodeClient()
			if err != nil {
				exit(err)
			}

			_, err = csiNode.NodePublishVolume(ctx, &req)
			if err != nil {
				return err
			}

			fmt.Println(args[i])
		}

		return nil
	},
}
