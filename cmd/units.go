package cmd

import (
	"github.com/spf13/cobra"

	"github.com/Bnei-Baruch/archive-backend/es"
)

var unitsCmd = &cobra.Command{
	Use:   "units",
	Short: "Index content units in ES",
	Run:   etlUnitsFn,
}

func init() {
	etlCmd.AddCommand(unitsCmd)
}

func etlUnitsFn(cmd *cobra.Command, args []string) {
	es.IndexUnits()
}
