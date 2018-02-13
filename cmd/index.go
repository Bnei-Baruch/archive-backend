package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/Bnei-Baruch/archive-backend/consts"
	"github.com/Bnei-Baruch/archive-backend/es"
)

var indexCmd = &cobra.Command{
	Use:   "index",
	Short: "Import MDB to ElasticSearch",
	Run:   indexFn,
}

var indexClassificationsCmd = &cobra.Command{
	Use:   "classifications",
	Short: "Index content units classifications in ES",
	Run:   indexClassificationsFn,
}

var indexUnitsCmd = &cobra.Command{
	Use:   "units",
	Short: "Index content units in ES",
	Run:   indexUnitsFn,
}

var indexCollectionsCmd = &cobra.Command{
	Use:   "collections",
	Short: "Index content collections in ES",
	Run:   indexCollectionsFn,
}

func init() {
	RootCmd.AddCommand(indexCmd)
	indexCmd.AddCommand(indexClassificationsCmd)
	indexCmd.AddCommand(indexUnitsCmd)
	indexCmd.AddCommand(indexCollectionsCmd)
}

func indexFn(cmd *cobra.Command, args []string) {
	fmt.Println("Use one of the subcommands.")
}

func indexClassificationsFn(cmd *cobra.Command, args []string) {
	es.IndexCmd(consts.ES_CLASSIFICATIONS_INDEX)
}

func indexUnitsFn(cmd *cobra.Command, args []string) {
	es.IndexCmd(consts.ES_UNITS_INDEX)
}

func indexCollectionsFn(cmd *cobra.Command, args []string) {
	es.IndexCmd(consts.ES_COLLECTIONS_INDEX)
}
