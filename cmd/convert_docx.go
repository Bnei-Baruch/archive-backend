package cmd

import (
	"github.com/spf13/cobra"

	"github.com/Bnei-Baruch/archive-backend/es"
)

var convertDocx = &cobra.Command{
	Use:   "convert-docx",
	Short: "Converts all docs to docx for ES",
	Run:   convertDocxFn,
}

func init() {
	RootCmd.AddCommand(convertDocx)
}

func convertDocxFn(cmd *cobra.Command, args []string) {
	es.ConvertDocx()
}
