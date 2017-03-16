package cmd

import (
	"github.com/spf13/cobra"

	"github.com/Bnei-Baruch/archive-backend/es"
)

var etlCmd = &cobra.Command{
	Use:   "etl",
	Short: "Import MDB to ElasticSearch",
	Run:   etlFn,
}

func init() {
	RootCmd.AddCommand(etlCmd)
}

func etlFn(cmd *cobra.Command, args []string) {
	es.ImportMDB()
}
