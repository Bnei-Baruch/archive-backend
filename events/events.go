package events

import (
	"context"
	"encoding/json"
	"os"
	"os/signal"
	"runtime/debug"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"

	"github.com/spf13/viper"

	"github.com/Bnei-Baruch/archive-backend/common"
	"github.com/Bnei-Baruch/archive-backend/es"
	"github.com/Bnei-Baruch/archive-backend/utils"
)

var indexer *es.Indexer
var indexerQueue WorkQueue

func shutDown(signalChan chan os.Signal, el *EventListener, indexerQueue WorkQueue, cleanupDone chan bool) {
	for _ = range signalChan {
		log.Info("Shutting down...")

		log.Info("Closing connection to nats")

		// Do not unsubscribe a durable on exit, except if asked to.
		el.Close()

		log.Info("Closing indexer queue")
		indexerQueue.Close()

		cleanupDone <- true
	}
}

// Nats
type EventListener struct {
	nc         *nats.Conn
	js         jetstream.JetStream
	consumer   jetstream.Consumer
	consumeCtx jetstream.ConsumeContext
}

func (el *EventListener) Close() {
	el.consumeCtx.Stop()
	el.nc.Close()
}

func (el *EventListener) handleMessage(msg jetstream.Msg) {
	log.Debugf("EventListener.handleMessage: %+v", msg.Data())

	// don't panic !
	defer func() {
		if rval := recover(); rval != nil {
			log.Errorf("handleMessage panic: %v while handling %v", rval, msg)
			debug.PrintStack()
		}
	}()

	var e Event
	if err := json.Unmarshal(msg.Data(), &e); err != nil {
		log.Errorf("EventListener.handleMessage json.Unmarshal: %w", err)
	}

	handler, ok := messageHandlers[e.Type]
	if !ok {
		log.Errorf("Unknown event type: %v", e)

		// Acknowledge the message so we won't stuck on it
		msg.Ack()
	}

	if e.ReplicationLocation != "" {
		log.Infof("Replication location: %s", e.ReplicationLocation)
		var synced bool
		err := common.DB.QueryRow("SELECT pg_last_wal_replay_lsn() >= $1", e.ReplicationLocation).Scan(&synced)
		if err != nil {
			log.Errorf("Check replica is synced: %+v", err)
			return
		}

		if !synced {
			log.Infof("Replica not synced: %s", e.ReplicationLocation)
			// sleep maybe ?
			//time.Sleep(500 * time.Millisecond)
			return
		}
	}

	// Acknowledge the message
	msg.Ack()

	log.Infof("Handling %+v", e)
	handler(e)
}

func RunListener() {
	log.SetLevel(log.InfoLevel)

	log.Info("Initialize data stores")
	common.Init()
	defer common.Shutdown()

	natsURL := viper.GetString("nats.url")
	log.Infof("Initialize connection to nats: %s", natsURL)
	var err error
	el := new(EventListener)
	el.nc, err = nats.Connect(natsURL)
	if err != nil {
		log.Errorf("nats.Connect: %w", err)
		utils.Must(err)
	}

	el.js, err = jetstream.New(el.nc)
	if err != nil {
		log.Errorf("jetstream.New: %w", err)
		utils.Must(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	el.consumer, err = el.js.CreateOrUpdateConsumer(ctx, "MDB", jetstream.ConsumerConfig{
		Name:        "Archive-Backend",
		Durable:     "Archive-Backend",
		Description: "Events listener of MDB",
	})
	if err != nil {
		log.Errorf("jetstream.CreateOrUpdateConsumer: %w", err)
		utils.Must(err)
	}

	el.consumeCtx, err = el.consumer.Consume(el.handleMessage)
	if err != nil {
		log.Errorf("jetstream consumer.Consume: %w", err)
		utils.Must(err)
	}

	log.Info("Initialize search engine indexer")
	esc, err := common.ESC.GetClient()
	if err != nil {
		log.Fatalf("Elastic is not available in RunListener():  %+v", err)
	}
	if viper.GetBool("server.fake-indexer") {
		indexer, err = es.MakeFakeIndexer(common.DB, esc)
		utils.Must(err)
	} else {
		err, date := es.ProdIndexDate(esc)
		utils.Must(err)
		indexer, err = es.MakeProdIndexer(date, common.DB, esc)
		utils.Must(err)
	}

	log.Info("Initialize indexer queue")
	indexerQueue = new(IndexerQueue)
	indexerQueue.Init()

	// wait for kill
	signalChan := make(chan os.Signal, 1)
	cleanupDone := make(chan bool)
	signal.Notify(signalChan, os.Interrupt)
	go func() { shutDown(signalChan, el, indexerQueue, cleanupDone) }()

	log.Info("Press Ctrl+C to terminate")
	<-cleanupDone
}

// Event data struct for unmarshaling data from nats
type Event struct {
	ID                  string                 `json:"id"`
	Type                string                 `json:"type"`
	ReplicationLocation string                 `json:"rloc"`
	Payload             map[string]interface{} `json:"payload"`
}

type MessageHandler func(e Event)

var messageHandlers = map[string]MessageHandler{
	E_COLLECTION_CREATE:               CollectionCreate,
	E_COLLECTION_DELETE:               CollectionDelete,
	E_COLLECTION_UPDATE:               CollectionUpdate,
	E_COLLECTION_PUBLISHED_CHANGE:     CollectionPublishedChange,
	E_COLLECTION_CONTENT_UNITS_CHANGE: CollectionContentUnitsChange,

	E_CONTENT_UNIT_CREATE:             ContentUnitCreate,
	E_CONTENT_UNIT_DELETE:             ContentUnitDelete,
	E_CONTENT_UNIT_UPDATE:             ContentUnitUpdate,
	E_CONTENT_UNIT_PUBLISHED_CHANGE:   ContentUnitPublishedChange,
	E_CONTENT_UNIT_DERIVATIVES_CHANGE: ContentUnitDerivativesChange,
	E_CONTENT_UNIT_SOURCES_CHANGE:     ContentUnitSourcesChange,
	E_CONTENT_UNIT_TAGS_CHANGE:        ContentUnitTagsChange,
	E_CONTENT_UNIT_PERSONS_CHANGE:     ContentUnitPersonsChange,
	E_CONTENT_UNIT_PUBLISHERS_CHANGE:  ContentUnitPublishersChange,

	E_FILE_PUBLISHED: FilePublished,
	E_FILE_REPLACE:   FileReplace,
	E_FILE_INSERT:    FileInsert,
	E_FILE_UPDATE:    FileUpdate,
	E_FILE_REMOVE:    FileUpdate,

	E_SOURCE_CREATE: SourceCreate,
	E_SOURCE_UPDATE: SourceUpdate,

	E_TAG_CREATE: TagCreate,
	E_TAG_UPDATE: TagUpdate,

	E_PERSON_CREATE: PersonCreate,
	E_PERSON_DELETE: PersonDelete,
	E_PERSON_UPDATE: PersonUpdate,

	E_PUBLISHER_CREATE: PublisherCreate,
	E_PUBLISHER_UPDATE: PublisherUpdate,

	E_BLOG_POST_CREATE: BlogPostCreate,
	E_BLOG_POST_UPDATE: BlogPostUpdate,
	E_BLOG_POST_DELETE: BlogPostDelete,

	E_TWEET_CREATE: TweetCreate,
	E_TWEET_UPDATE: TweetUpdate,
	E_TWEET_DELETE: TweetDelete,
}
