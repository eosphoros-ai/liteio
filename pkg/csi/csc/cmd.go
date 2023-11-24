package csicmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	CSISocketFile string
	NodeID        string
)

func NewCSICommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "csi",
		Short: "Run CSI Request",
		Long:  `Run CSI Request`,
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println(`
			Use -h to show help
			Sending CSI request examples:
			BIN csi unstage --staging-target-path=[PATH] [VolID]
			BIN csi stage --staging-target-path=[PATH] [VolID]
			BIN csi publish --vol-context skip-save-context=true,key2=val2 --staging-target-path=[PATH] --target-path=[PATH2] [VolID]
			`)
		},
	}

	cmd.PersistentFlags().StringVarP(&CSISocketFile, "csi-sock", "s", "/plugin/csi.sock", "CSI socket file path")
	cmd.PersistentFlags().StringVarP(&NodeID, "nodeid", "n", "", "Node name in k8s")

	// add sub commands
	cmd.AddCommand(nodeUnpublishVolumeCmd)
	cmd.AddCommand(nodeUnstageVolumeCmd)
	cmd.AddCommand(nodeStageVolumeCmd)
	cmd.AddCommand(nodePublishVolumeCmd)

	return cmd
}
