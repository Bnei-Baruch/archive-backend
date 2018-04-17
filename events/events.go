package events

import (
	"encoding/json"
	"os"
	"os/signal"
	"runtime/debug"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/nats-io/go-nats-streaming"
	"github.com/spf13/viper"

	"github.com/Bnei-Baruch/archive-backend/common"
	"github.com/Bnei-Baruch/archive-backend/es"
	"github.com/Bnei-Baruch/archive-backend/utils"
)

var indexer *es.Indexer
var indexerQueue WorkQueue

func shutDown(signalChan chan os.Signal, sc stan.Conn, indexerQueue WorkQueue, cleanupDone chan bool) {
    for _ = range signalChan {
        log.Info("Shutting down...")

        log.Info("Closing connection to nats")
        // Do not unsubscribe a durable on exit, except if asked to.
        sc.Close()

        log.Info("Closing indexer queue")
        indexerQueue.Close()

        cleanupDone <- true
    }
}

func RunListener() {
	log.SetLevel(log.InfoLevel)

	var err error

	log.Info("Initialize data stores")
	common.Init()
	defer common.Shutdown()

	log.Info("Initialize connection to nats")
	natsURL := viper.GetString("nats.url")
	natsClientID := viper.GetString("nats.client-id")
	natsClusterID := viper.GetString("nats.cluster-id")
	natsSubject := viper.GetString("nats.subject")
	sc, err := stan.Connect(natsClusterID, natsClientID, stan.NatsURL(natsURL))
	utils.Must(err)
	defer sc.Close()

	log.Info("Subscribing to nats subject")
	var startOpt stan.SubscriptionOption
	if viper.GetBool("nats.durable") == true {
		startOpt = stan.DurableName(viper.GetString("nats.durable-name"))
	} else {
		startOpt = stan.DeliverAllAvailable()
	}
	_, err = sc.Subscribe(natsSubject, msgHandler, startOpt)
	utils.Must(err)

	log.Info("Initialize search engine indexer")
	if viper.GetBool("server.fake-indexer") {
		indexer = es.MakeFakeIndexer(common.DB, common.ESC)
	} else {
		indexer = es.MakeProdIndexer(common.DB, common.ESC)
	}

	log.Info("Initialize indexer queue")
	indexerQueue = new(IndexerQueue)
	indexerQueue.Init()

	// wait for kill
	signalChan := make(chan os.Signal, 1)
	cleanupDone := make(chan bool)
	signal.Notify(signalChan, os.Interrupt)
	go func() { shutDown(signalChan, sc, indexerQueue, cleanupDone ) }()

	log.Info("Press Ctrl+C to terminate")
	<-cleanupDone
}

// Data struct for unmarshaling data from nats
type Data struct {
	ID      string                 `json:"id"`
	Type    string                 `json:"type"`
	Payload map[string]interface{} `json:"payload"`
}

type MessageHandler func(d Data)

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

	E_SOURCE_CREATE: SourceCreate,
	E_SOURCE_UPDATE: SourceUpdate,

	E_TAG_CREATE: TagCreate,
	E_TAG_UPDATE: TagUpdate,

	E_PERSON_CREATE: PersonCreate,
	E_PERSON_DELETE: PersonDelete,
	E_PERSON_UPDATE: PersonUpdate,

	E_PUBLISHER_CREATE: PublisherCreate,
	E_PUBLISHER_UPDATE: PublisherUpdate,
}

// msgHandler checks message type and calls "eventHandler"
func msgHandler(msg *stan.Msg) {
	// don't panic !
	defer func() {
		if rval := recover(); rval != nil {
			log.Errorf("msgHandler panic: %v while handling %v", rval, msg)
			debug.PrintStack()
		}
	}()

	var d Data
	err := json.Unmarshal(msg.Data, &d)
	if err != nil {
		log.Errorf("json.Unmarshal error: %s\n", err)
	}

	handler, ok := messageHandlers[d.Type]
	if !ok {
		log.Errorf("Unknown event type: %v", d)
	}

	// DIRTY HACK !!!
	// delay handler until our read replica will be synced (hopefully)
	time.Sleep(500 * time.Millisecond)

	log.Infof("Handling %+v", d)
	handler(d)
}
