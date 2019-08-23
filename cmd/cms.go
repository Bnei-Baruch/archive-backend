package cmd

import (
	"github.com/spf13/cobra"

	"github.com/Bnei-Baruch/archive-backend/cms"
)

var cmsCmd = &cobra.Command{
	Use:   "cms",
	Short: "Sync data from CMS",
	Run: func(cmd *cobra.Command, args []string) {
		cms.SyncCMS()
	},
}

func init() {
	RootCmd.AddCommand(cmsCmd)
}
