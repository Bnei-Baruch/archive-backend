package cmd

import (
	"context"

	log "github.com/Sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/Bnei-Baruch/archive-backend/common"
	es9common "github.com/Bnei-Baruch/archive-backend/es9/common"
	"github.com/Bnei-Baruch/archive-backend/events"
)

// es9UpdateCmd is a throwaway proof for ES9 incremental indexing via the real event facade.
// Usage: ./archive-backend es9-update <cu|collection|source> <uid>
var es9UpdateCmd = &cobra.Command{
	Use:   "es9-update <cu|collection|source> <uid>",
	Short: "ES9 incremental proof: remove+re-add one entity by uid via the event facade",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		kind, uid := args[0], args[1]
		indexNameBase := cmd.Flag("index").Value.String()

		common.InitWithOptions(nil, nil, false)
		defer common.Shutdown()

		es9URL := viper.GetString("elasticsearch9.url")
		if es9URL == "" {
			log.Fatal("elasticsearch9.url not configured")
		}
		manager := es9common.MakeES9Manager(es9URL)
		defer manager.Stop()
		if err := manager.Ping(context.Background()); err != nil {
			log.Fatalf("Failed to connect to ES9: %v", err)
		}

		idx := events.MakeES9Indexer(common.DB, manager, indexNameBase, viper.GetString("elasticsearch.unzip-url"))

		var err error
		switch kind {
		case "cu":
			err = idx.ContentUnitUpdate(uid)
		case "collection":
			err = idx.CollectionUpdate(uid)
		case "source":
			err = idx.SourceUpdate(uid)
		case "tag":
			err = idx.TagUpdate(uid)
		case "blog":
			err = idx.BlogPostUpdate(uid)
		case "tweet":
			err = idx.TweetUpdate(uid)
		default:
			log.Fatalf("unknown kind %q (want cu|collection|source|tag|blog|tweet)", kind)
		}
		if err != nil {
			log.Fatalf("es9-update %s %s failed: %v", kind, uid, err)
		}
		log.Infof("✓ es9-update %s %s done", kind, uid)
	},
}

func init() {
	RootCmd.AddCommand(es9UpdateCmd)
	es9UpdateCmd.Flags().StringP("index", "i", "results", "Base index name (e.g. 'results')")
}
