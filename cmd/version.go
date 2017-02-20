package cmd

import (
	"fmt"

	"github.com/Bnei-Baruch/mdb2es/version"
	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version number of MDB2ES",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("MDB to Elasticsearch tools belt version %s\n", version.Version)
	},
}

func init() {
	RootCmd.AddCommand(versionCmd)
}
