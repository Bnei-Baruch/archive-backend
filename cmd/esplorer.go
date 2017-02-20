package cmd

import (
	log "github.com/Sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	elastic "gopkg.in/olivere/elastic.v5"
	"time"
	"net/http"
	"context"
	"encoding/json"
	"github.com/rs/cors"
)

var esplorerCmd = &cobra.Command{
	Use:   "esplorer",
	Short: "Expose query interface to elasticsearch via HTTP",
	Run:   esplorerFn,
}

func init() {
	RootCmd.AddCommand(esplorerCmd)
}

func configDefaults() {
	viper.SetDefault("elasticsearch", map[string]interface{}{
		"url": "http://127.0.0.1:9200",
	})
	viper.SetDefault("server", map[string]interface{}{
		"bind-address": ":8080",
	})
}

func esplorerFn(cmd *cobra.Command, args []string) {
	configDefaults()

	// Setup logging
	log.SetFormatter(&log.TextFormatter{FullTimestamp: true})
	log.Infoln("MDB ES Explorer started")

	// Setup ES connection
	url := viper.GetString("elasticsearch.url")
	es, err := elastic.NewClient(
		elastic.SetURL(viper.GetString("elasticsearch.url")),
		elastic.SetSniff(false),
		elastic.SetHealthcheckInterval(10*time.Second),
		elastic.SetMaxRetries(5),
		elastic.SetErrorLog(log.StandardLogger()),
		elastic.SetInfoLog(log.StandardLogger()),
	)
	if err != nil {
		log.Fatal(err)
	}

	// Getting the ES version number is quite common, so there's a shortcut
	esversion, err := es.ElasticsearchVersion(url)
	if err != nil {
		log.Fatal(err)
	}
	log.Infof("Elasticsearch version %s", esversion)


	// Setup HTTP Handlers
	mux := http.NewServeMux()
	mux.HandleFunc("/query", func(w http.ResponseWriter, r *http.Request) {
		text := r.URL.Query().Get("text")
		q := elastic.NewBoolQuery().Should(elastic.NewMatchQuery("containers.description", text),
			elastic.NewMatchQuery("containers.full_description", text))

		//q := elastic.NewMatchQuery("containers.description", text)
		res, err := es.Search().Index("mdb*").Query(q).Do(context.Background())
		if err != nil {
			log.Fatal(err)
		}

		w.Header().Set("Content-Type", "application/json; charset=UTF-8")
		w.WriteHeader(http.StatusOK)
		err = json.NewEncoder(w).Encode(res)
		if err != nil {
			log.Fatal(err)
		}
	})

	// setup CORS handler
	handler := cors.Default().Handler(mux)

	log.Infoln("Running application")
	log.Fatal(http.ListenAndServe(viper.GetString("server.bind-address"), handler))
}
