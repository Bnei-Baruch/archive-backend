package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/Bnei-Baruch/mdb2es/utils"
	"github.com/pkg/errors"
)

var cfgFile string

var RootCmd = &cobra.Command{
	Use:   "mdb2es",
	Short: "MDB to Elasticsearch tools belt",
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
	if err := utils.InitConfig(cfgFile, ""); err != nil {
		panic(errors.Wrapf(err, "Could not read config, using: %s", viper.ConfigFileUsed()))
	}
}
