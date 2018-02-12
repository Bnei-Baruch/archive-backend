package events

import (
	"encoding/json"
	"fmt"
	"os"
	"os/signal"

	log "github.com/Sirupsen/logrus"
	"github.com/nats-io/go-nats-streaming"
	"github.com/spf13/viper"

	"github.com/Bnei-Baruch/archive-backend/es"
	"github.com/Bnei-Baruch/archive-backend/mdb"
	"github.com/Bnei-Baruch/archive-backend/utils"
	"reflect"
	"runtime"
	"strings"
)

var indexer *es.Indexer

func RunListener() {
	log.SetLevel(log.InfoLevel)

	// routine to run indexer functions from channel
	go func() {
		for {
			a1 := <-ChanIndexFuncs
			a1.F(a1.S)
			currentFunc := strings.Split(runtime.FuncForPC(reflect.ValueOf(a1.F).Pointer()).Name(), ".")
			lastElement := currentFunc[len(currentFunc)-1]
			log.Infof("running indexer function \"%+v\", with parameter %s\n", lastElement, a1.S)
			log.Infof("*******number of elements on Indexer channel is %d", len(ChanIndexFuncs))
		}
	}()

	var err error
	log.Info("Initialize connections to MDB and elasticsearch")
	mdb.Init()
	defer mdb.Shutdown()

	log.Info("Initialize connection to nats")
	natsURL := viper.GetString("nats.url")
	natsClientID := viper.GetString("nats.client-id")
	natsClusterID := viper.GetString("nats.cluster-id")
	natsSubject := viper.GetString("nats.subject")
	sc, err := stan.Connect(natsClusterID, natsClientID, stan.NatsURL(natsURL))
	utils.Must(err)
	defer sc.Close()
	log.Printf("Connected to %s clusterID: [%s] clientID: [%s]\n", natsURL, natsClusterID, natsClientID)
	log.Info("Subscribing to nats")

	var startOpt stan.SubscriptionOption
	if viper.GetBool("nats.durable") == true {
		startOpt = stan.DurableName(viper.GetString("nats.durable-name"))
	} else {
		startOpt = stan.DeliverAllAvailable()
	}

	_, err = sc.Subscribe(natsSubject, msgHandler, startOpt)
	utils.Must(err)

	// to disbable indexing set this in config
	if viper.GetBool("server.fake-indexer") {
		indexer = es.MakeFakeIndexer()
	} else {
		indexer = es.MakeProdIndexer()
	}

	log.Info("Press Ctrl+C to terminate")
	signalChan := make(chan os.Signal, 1)
	cleanupDone := make(chan bool)
	signal.Notify(signalChan, os.Interrupt)
	go func() {
		for range signalChan {
			fmt.Printf("\nReceived an interrupt, unsubscribing and closing connection...\n\n")
			// Do not unsubscribe a durable on exit, except if asked to.
			sc.Close()
			cleanupDone <- true
		}
	}()
	<-cleanupDone
}

// Data struct for unmarshaling data from nats
type Data struct {
	ID      string                 `json:"id"`
	Type    string                 `json:"type"`
	Payload map[string]interface{} `json:"payload"`
}

// ChannelForIndexers for putting indexer funcs on nats
type ChannelForIndexers struct {
	F func(s string) error
	S string
}

// ChanIndexFuncs channel to pass indexer functions
var ChanIndexFuncs = make(chan ChannelForIndexers, 100000)

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

	var d Data
	err := json.Unmarshal(msg.Data, &d)
	if err != nil {
		log.Errorf("json.Unmarshal error: %s\n", err)
	}

	handler, ok := messageHandlers[d.Type]
	if !ok {
		log.Errorf("Unknown event type: %v", d)
	}

	log.Debugf("Handling %+v", d)
	handler(d)

}
