package cmd

import (
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/Bnei-Baruch/archive-backend/common"
	"github.com/Bnei-Baruch/archive-backend/es"
	"github.com/Bnei-Baruch/archive-backend/utils"
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
	clock := common.Init()

	utils.Must(es.ConvertDocx(common.DB))

	common.Shutdown()
	log.Info("Success")
	log.Infof("Total run time: %s", time.Now().Sub(clock).String())
}
