package cmd

import (
	"github.com/spf13/cobra"

	"fmt"
)

var indexCmd = &cobra.Command{
	Use:   "index",
	Short: "Import MDB to ElasticSearch",
	Run:   indexFn,
}

func init() {
	RootCmd.AddCommand(indexCmd)
}

func indexFn(cmd *cobra.Command, args []string) {
	fmt.Println("Use one of the subcommands.")
}
