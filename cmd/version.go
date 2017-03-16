package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/Bnei-Baruch/archive-backend/version"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version number of archive-backend",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("Backend for new archive site version %s\n", version.Version)
	},
}

func init() {
	RootCmd.AddCommand(versionCmd)
}
