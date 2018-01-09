package events

import (
	"fmt"
	"log"
	"os"
	"os/signal"

	"encoding/json"
	"github.com/Bnei-Baruch/archive-backend/es"
	"github.com/nats-io/go-nats-streaming"
	"github.com/spf13/viper"
)

// struct for the messages from bus
type Payload struct {
	Id  int64  `json:"id"`
	Uid string `json:"uid"`
}

type Data struct {
	Id      string  `json:"id"`
	Type    string  `json:"type"`
	Payload Payload `json:"payload"`
}

//RunLListener runs listener for nats server and reacts to new events
func RunLListener() {

	//
	//
	//
	// TODO:
	// run Kolman's stuff
	// see https://github.com/nats-io/go-nats/issues/195
	// we should upgrade as soon as it's fixed !

	natsUrl := viper.GetString("nats.url")
	natsClientId := viper.GetString("nats.client-id")
	natsClusterId := viper.GetString("nats.cluster-id")
	natsSubject := viper.GetString("nats.subject")

	// connect to nats server
	sc, err := stan.Connect(natsClusterId, natsClientId, stan.NatsURL(natsUrl))
	if err != nil {
		log.Fatalf("Can't connect: %v.\nMake sure a NATS Streaming Server is running at: %s\n retrying:", err, natsUrl)
		//retry somehow
	}

	log.Printf("Connected to %s clusterID: [%s] clientID: [%s]\n", natsUrl, natsClusterId, natsClientId)

	//mcb := func(msg *stan.Msg) {
	//	var b Data
	//
	//	err := json.Unmarshal(msg.Data, &b)
	//	if err != nil {
	//		fmt.Printf("json.Unmarshal error: %s\n",err)
	//	}
	//	fmt.Printf("%v\n",b.Payload.Uid)
	//
	//}

	startOpt := stan.DeliverAllAvailable()

	sc.QueueSubscribe(natsSubject, "", msgHandler, startOpt)

	log.Printf("Listening on [%s], clientID=[%s], qgroup=[%s] durable=[%s]\n", natsSubject, natsUrl)

	signalChan := make(chan os.Signal, 1)
	cleanupDone := make(chan bool)
	signal.Notify(signalChan, os.Interrupt)
	go func() {
		for _ = range signalChan {
			fmt.Printf("\nReceived an interrupt, unsubscribing and closing connection...\n\n")
			// Do not unsubscribe a durable on exit, except if asked to.
			sc.Close()
			cleanupDone <- true
		}
	}()
	<-cleanupDone
}

func msgHandler(msg *stan.Msg) {
	var data Data

	err := json.Unmarshal(msg.Data, &data)
	if err != nil {
		fmt.Printf("json.Unmarshal error: %s\n", err)
	}

	switch data.Type {
	case E_COLLECTION_CREATE:
		es.CollectionAdd(data.Payload.Uid)
	case E_COLLECTION_DELETE:
		es.CollectionDelete(data.Payload.Uid)
	case E_COLLECTION_UPDATE,
		E_COLLECTION_PUBLISHED_CHANGE,
		E_COLLECTION_CONTENT_UNITS_CHANGE:
		es.CollectionUpdate(data.Payload.Uid)
	case E_CONTENT_UNIT_CREATE:
		es.ContentUnitAdd(data.Payload.Uid)
	case E_CONTENT_UNIT_DELETE:
		es.ContentUnitDelete(data.Payload.Uid)
	case E_CONTENT_UNIT_UPDATE,
		E_CONTENT_UNIT_PUBLISHED_CHANGE,
		E_CONTENT_UNIT_DERIVATIVES_CHANGE,
		E_CONTENT_UNIT_SOURCES_CHANGE,
		E_CONTENT_UNIT_TAGS_CHANGE,
		E_CONTENT_UNIT_PERSONS_CHANGE,
		E_CONTENT_UNIT_PUBLISHERS_CHANGE:
		es.ContentUnitUpdate(data.Payload.Uid)
	case E_FILE_PUBLISHED:
		es.FileAdd(data.Payload.Uid)
	case E_FILE_REPLACE:
		// ???
	case E_FILE_INSERT:
		// ???
	case E_FILE_UPDATE:
		es.FileUpdate(data.Payload.Uid)
	case E_SOURCE_CREATE:
		es.SourceAdd(data.Payload.Uid)
	case E_SOURCE_UPDATE:
		es.SourceUpdate(data.Payload.Uid)
	case E_TAG_CREATE:
		es.TagAdd(data.Payload.Uid)
	case E_TAG_UPDATE:
		es.TagUpdate(data.Payload.Uid)
	case E_PERSON_CREATE:
		es.PersonAdd(data.Payload.Uid)
	case E_PERSON_DELETE:
		es.PersonDelete(data.Payload.Uid)
	case E_PERSON_UPDATE:
		es.PersonUpdate(data.Payload.Uid)
	case E_PUBLISHER_CREATE:
		es.PublisherAdd(data.Payload.Uid)
	case E_PUBLISHER_UPDATE:
		es.PublisherUpdate(data.Payload.Uid)
	default:
		fmt.Println("unknown event type", data.Type, data.Payload.Uid)
	}

}
