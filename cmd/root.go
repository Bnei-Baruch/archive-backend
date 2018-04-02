package cmd

import (
	"fmt"
	"os"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	log "github.com/Sirupsen/logrus"

	"github.com/Bnei-Baruch/archive-backend/utils"
)

var cfgFile string

var RootCmd = &cobra.Command{
	Use:   "archive-backend",
	Short: "Backend for new archive site",
}

func Execute() {
	if err := RootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(-1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)
	RootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "", "config file (default is config.toml)")
}

func initConfig() {
    log.Infof("Debug! initConfig.")
	if err := utils.InitConfig(cfgFile, ""); err != nil {
		panic(errors.Wrapf(err, "Could not read config, using: %s", viper.ConfigFileUsed()))
	}
}
