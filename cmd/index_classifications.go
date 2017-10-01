package cmd

import (
	"github.com/spf13/cobra"

	"github.com/Bnei-Baruch/archive-backend/es"
)

var indexClassificationsCmd = &cobra.Command{
	Use:   "classifications",
	Short: "Index content units classifications in ES",
	Run:   indexClassificationsFn,
}

func init() {
	indexCmd.AddCommand(indexClassificationsCmd)
}

func indexClassificationsFn(cmd *cobra.Command, args []string) {
	es.IndexClassifications()
}
