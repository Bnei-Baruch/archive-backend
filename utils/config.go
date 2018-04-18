package utils

import (
	"strings"

	"github.com/spf13/viper"
)

func InitConfig(cfgFile string, cfgPath string) error {
	if cfgFile == "" {
		viper.SetConfigName("config")
		if cfgPath == "" {
			viper.AddConfigPath(".")
		} else {
			viper.AddConfigPath(cfgPath)
		}
	} else {
		viper.SetConfigFile(cfgFile)
	}
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))
	viper.AutomaticEnv()
	return viper.ReadInConfig()
}
