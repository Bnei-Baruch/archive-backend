package cmd

import (
	"database/sql"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/stvp/rollbar"
	"github.com/vattle/sqlboiler/boil"
	"gopkg.in/gin-contrib/cors.v1"
	"gopkg.in/gin-gonic/gin.v1"

	"github.com/Bnei-Baruch/archive-backend/links"
	"github.com/Bnei-Baruch/archive-backend/utils"
	"github.com/Bnei-Baruch/archive-backend/version"
)

var linksCmd = &cobra.Command{
	Use:   "links",
	Short: "A backend service for serving links to file backend",
	Run:   linksFn,
}

func init() {
	RootCmd.AddCommand(linksCmd)
}

func linksFn(cmd *cobra.Command, args []string) {
	log.SetFormatter(&log.TextFormatter{FullTimestamp: true})

	log.Info("Setting up connection to MDB")
	mdbDB, err := sql.Open("postgres", viper.GetString("mdb.url"))
	utils.Must(err)
	defer mdbDB.Close()
	boil.DebugMode = viper.GetString("links.mode") == "debug"

	// Setup Rollbar
	rollbar.Token = viper.GetString("links.rollbar-token")
	rollbar.Environment = viper.GetString("links.rollbar-environment")
	rollbar.CodeVersion = version.Version

	// Setup gin
	gin.SetMode(viper.GetString("links.mode"))
	router := gin.New()
	router.Use(
		utils.DataStoresMiddleware(mdbDB, nil),
		utils.ErrorHandlingMiddleware(),
		cors.New(cors.Config{
			AllowMethods:     []string{"GET"},
			AllowHeaders:     []string{"Origin", "Content-Length", "Content-Type"},
			AllowCredentials: false,
			AllowAllOrigins:  true,
			MaxAge:           12 * time.Hour,
		}),
		utils.RecoveryMiddleware())

	links.SetupRoutes(router)

	log.Infoln("Running application")
	if cmd != nil {
		router.Run(viper.GetString("links.bind-address"))
	}
}
