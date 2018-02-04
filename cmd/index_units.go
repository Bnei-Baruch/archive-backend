package cmd

import (
	"github.com/spf13/cobra"

	"github.com/Bnei-Baruch/archive-backend/consts"
	"github.com/Bnei-Baruch/archive-backend/es"
)

var indexUnits = &cobra.Command{
	Use:   "units",
	Short: "Index content units in ES",
	Run:   indexUnitsFn,
}

func init() {
	indexCmd.AddCommand(indexUnits)
}

func indexUnitsFn(cmd *cobra.Command, args []string) {
	es.IndexCmd(consts.ES_UNITS_INDEX)
}
