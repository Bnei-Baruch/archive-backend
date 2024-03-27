package cmd

import (
	log "github.com/Sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/stvp/rollbar"
	"gopkg.in/gin-contrib/cors.v1"
	"gopkg.in/gin-gonic/gin.v1"

	"github.com/Bnei-Baruch/archive-backend/api"
	"github.com/Bnei-Baruch/archive-backend/common"
	"github.com/Bnei-Baruch/archive-backend/utils"
	"github.com/Bnei-Baruch/archive-backend/version"
)

var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Archive backend server",
	Run:   serverFn,
}

var bindAddress string

func init() {
	serverCmd.PersistentFlags().StringVar(&bindAddress, "bind_address", "", "Bind address for server.")
	viper.BindPFlag("server.bind-address", serverCmd.PersistentFlags().Lookup("bind_address"))
	RootCmd.AddCommand(serverCmd)
}

func serverFn(cmd *cobra.Command, args []string) {
	log.Infof("Starting Archive backend server version %s", version.Version)
	common.Init()
	defer common.Shutdown()

	// Setup Rollbar
	rollbar.Token = viper.GetString("server.rollbar-token")
	rollbar.Environment = viper.GetString("server.rollbar-environment")
	rollbar.CodeVersion = version.Version

	// Setup gin
	gin.SetMode(viper.GetString("server.mode"))
	middleware := []gin.HandlerFunc{
		utils.LoggerMiddleware(),
		utils.DataStoresMiddleware(common.DB, common.ESC, common.CACHE /*common.GRAMMARS,*/, common.TOKENS_CACHE, common.CMS, common.VARIABLES),
		utils.ErrorHandlingMiddleware(),
	}

	// cors
	if viper.GetString("server.enable-cors") == "true" {
		corsConfig := cors.DefaultConfig()
		corsConfig.AllowHeaders = append(corsConfig.AllowHeaders, "Authorization", "X-Request-ID")
		corsConfig.AllowAllOrigins = true
		middleware = append(middleware, cors.New(corsConfig))
	}

	middleware = append(middleware, utils.RecoveryMiddleware())

	router := gin.New()
	router.Use(middleware...)
	api.SetupRoutes(router)

	log.Infoln("Running application")
	if cmd != nil {
		router.Run(viper.GetString("server.bind-address"))
	}

	// This would be reasonable once we'll have graceful shutdown implemented
	// if len(rollbar.Token) > 0 {
	// 	rollbar.Wait()
	// }
}
