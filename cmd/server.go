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
	"gopkg.in/olivere/elastic.v5"

	"github.com/Bnei-Baruch/archive-backend/api"
	"github.com/Bnei-Baruch/archive-backend/mdb"
	"github.com/Bnei-Baruch/archive-backend/utils"
	"github.com/Bnei-Baruch/archive-backend/version"
)

var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Archive backend server",
	Run:   serverFn,
}

func init() {
	RootCmd.AddCommand(serverCmd)
}

func serverFn(cmd *cobra.Command, args []string) {
	log.SetFormatter(&log.TextFormatter{FullTimestamp: true})

	log.Infof("Starting Archive backend server version %s", version.Version)

	log.Info("Setting up connection to MDB")
	mdbDB, err := sql.Open("postgres", viper.GetString("mdb.url"))
	utils.Must(err)
	defer mdbDB.Close()
	boil.DebugMode = viper.GetString("server.mode") == "debug"

	log.Info("Initializing type registries")
	utils.Must(mdb.InitTypeRegistries(mdbDB))

	log.Info("Setting up connection to ElasticSearch")
	url := viper.GetString("elasticsearch.url")
	esc, err := elastic.NewClient(
		elastic.SetURL(viper.GetString("elasticsearch.url")),
		elastic.SetSniff(false),
		elastic.SetHealthcheckInterval(10*time.Second),
		elastic.SetErrorLog(log.StandardLogger()),
		elastic.SetInfoLog(log.StandardLogger()),
	)
	utils.Must(err)

	esversion, err := esc.ElasticsearchVersion(url)
	utils.Must(err)
	log.Infof("Elasticsearch version %s", esversion)

	// Setup Rollbar
	rollbar.Token = viper.GetString("server.rollbar-token")
	rollbar.Environment = viper.GetString("server.rollbar-environment")
	rollbar.CodeVersion = version.Version

	// Setup gin
	gin.SetMode(viper.GetString("server.mode"))
	router := gin.New()
	router.Use(
		utils.DataStoresMiddleware(mdbDB, esc),
		utils.ErrorHandlingMiddleware(),
		cors.Default(),
		utils.RecoveryMiddleware())

	api.SetupRoutes(router)

	log.Infoln("Running application")
	if cmd != nil {
		router.Run(viper.GetString("server.bind-address"))
	}

	// This would be reasonable once we'll have graceful shutdown implemented
	//if len(rollbar.Token) > 0 {
	//	rollbar.Wait()
	//}
}
