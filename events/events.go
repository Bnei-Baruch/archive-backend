package events

import (
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/Bnei-Baruch/archive-backend/mdb/models"
	"github.com/Bnei-Baruch/archive-backend/utils"
	log "github.com/Sirupsen/logrus"
	"github.com/nats-io/go-nats-streaming"
	"github.com/spf13/viper"
	"github.com/volatiletech/sqlboiler/boil"
	"github.com/volatiletech/sqlboiler/queries/qm"
	"os"
	"os/signal"
)

func RunLListener() {

	natsUrl := viper.GetString("nats.url")
	natsClientId := viper.GetString("nats.client-id")
	natsClusterId := viper.GetString("nats.cluster-id")
	natsSubject := viper.GetString("nats.subject")
	mdbUrl := "mdb.url"

	// Open handle to database like normal
	mdb, err := sql.Open("postgres", viper.GetString(mdbUrl))
	utils.Must(err)
	utils.Must(mdb.Ping())
	defer mdb.Close()

	// connect to nats server

		sc, err := stan.Connect(natsClusterId, natsClientId, stan.NatsURL(natsUrl))
		if err != nil {
			log.Fatalf("Can't connect: %v.\nMake sure a NATS Streaming Server is running at: %s\n retrying:", err, natsUrl)
			panic("...")
		}
		log.Printf("Connected to %s clusterID: [%s] clientID: [%s]\n", natsUrl, natsClusterId, natsClientId)

	// connection options
	startOpt := stan.DeliverAllAvailable()

	sc.Subscribe(natsSubject, msgHandler, startOpt)

	log.Printf("Listening on [%s], clientID=[%s], durable=[%s]\n", natsSubject, natsUrl, "false")

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

func fileSecure(s string, mdb *sql.DB) int16 {
	boil.SetDB(mdb)

	file := mdbmodels.Files(mdb, qm.Where("uid=?", s))
	OneFile, _ := file.One()
	println(OneFile.Type)
	return OneFile.Secure
}

//checks message type and calls "callEs"
func msgHandler(msg *stan.Msg) {

	type TypeTest struct {
		Type string `json:"type"`
	}

	var test TypeTest
	err := json.Unmarshal(msg.Data, &test)
	if err != nil {
		fmt.Printf("json.Unmarshal error: %s\n", err)
	}

	if test.Type != E_FILE_REPLACE {

		type SimpleData struct {
			Id      string `json:"id"`
			Type    string `json:"type"`
			Payload struct {
				Id  int64  `json:"id"`
				Uid string `json:"uid"`
			} `json:"payload"`
		}

		var data SimpleData

		err = json.Unmarshal(msg.Data, &data)
		if err != nil {
			fmt.Printf("json.Unmarshal error: %s\n", err)
		}

		callEs(data.Type, data.Payload.Uid, "")
	} else {

		type ReplaceData struct {
			ID      string `json:"id"`
			Type    string `json:"type"`
			Payload struct {
				InsertType string `json:"insert_type"`
				New        struct {
					ID  int    `json:"id"`
					UID string `json:"uid"`
				} `json:"new"`
				Old struct {
					ID  int    `json:"id"`
					UID string `json:"uid"`
				} `json:"old"`
			} `json:"payload"`
		}

		var data ReplaceData
		err = json.Unmarshal(msg.Data, &data)
		if err != nil {
			fmt.Printf("json.Unmarshal error: %s\n", err)
		}
		callEs(data.Type, data.Payload.Old.UID, data.Payload.New.UID)
	}
}

//calls searching functions
func callEs(eventType string, uid string, oldUid string) {
	fmt.Println("old_uid: " + oldUid + ",new_uid: " + uid)
	switch eventType {
	case E_COLLECTION_CREATE:
		CollectionCreate(uid)
	case E_COLLECTION_DELETE:
		CollectionDelete(uid)
	case E_COLLECTION_UPDATE:
		CollectionUpdate(uid)
	case E_COLLECTION_PUBLISHED_CHANGE:
		CollectionPublishedChange(uid)
	case E_COLLECTION_CONTENT_UNITS_CHANGE:
		CollectionContentUnitsChange(uid)
	case E_CONTENT_UNIT_CREATE:
		ContentUnitCreate(uid)
	case E_CONTENT_UNIT_DELETE:
		ContentUnitDelete(uid)
	case E_CONTENT_UNIT_UPDATE:
		ContentUnitUpdate(uid)
	case E_CONTENT_UNIT_PUBLISHED_CHANGE:
		ContentUnitPublishedChange(uid)
	case E_CONTENT_UNIT_DERIVATIVES_CHANGE:
		ContentUnitDerivativesChange(uid)
	case E_CONTENT_UNIT_SOURCES_CHANGE:
		ContentUnitSourcesChange(uid)
	case E_CONTENT_UNIT_TAGS_CHANGE:
		ContentUnitTagsChange(uid)
	case E_CONTENT_UNIT_PERSONS_CHANGE:
		ContentUnitPersonsChange(uid)
	case E_CONTENT_UNIT_PUBLISHERS_CHANGE:
		ContentUnitPublishersChange(uid)

	case E_FILE_PUBLISHED:
		FilePublished(uid)
	case E_FILE_REPLACE:
		FileReplace(uid, oldUid)
	case E_FILE_INSERT:
		FileInsert(uid)
	case E_FILE_UPDATE:
		FileUpdate(uid)
	case E_SOURCE_CREATE:
		SourceCreate(uid)
	case E_SOURCE_UPDATE:
		SourceUpdate(uid)
	case E_TAG_CREATE:
		TagCreate(uid)
	case E_TAG_UPDATE:
		TagUpdate(uid)
	case E_PERSON_CREATE:
		PersonCreate(uid)
	case E_PERSON_DELETE:
		PersonDelete(uid)
	case E_PERSON_UPDATE:
		PersonUpdate(uid)
	case E_PUBLISHER_CREATE:
		PublisherCreate(uid)
	case E_PUBLISHER_UPDATE:
		PublisherUpdate(uid)
	default:
		fmt.Println("unknown event type: " + eventType + " for UID: " + uid)
	}
}
