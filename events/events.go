package events

import (
	"database/sql"
	"encoding/json"
	"fmt"

	//"github.com/Bnei-Baruch/archive-backend/mdb/models"
	//"github.com/Bnei-Baruch/archive-backend/utils"
	log "github.com/Sirupsen/logrus"
	"github.com/nats-io/go-nats-streaming"
	"github.com/spf13/viper"
	"github.com/volatiletech/sqlboiler/boil"
	"os"
	"os/signal"
	"github.com/Bnei-Baruch/archive-backend/utils"
	"github.com/Bnei-Baruch/archive-backend/mdb/models"
	"github.com/volatiletech/sqlboiler/queries/qm"
	"net/http"
)

// pointer to connection to db
var MdbConn *sql.DB


func RunLListener() {

	natsUrl := viper.GetString("nats.url")
	natsClientId := viper.GetString("nats.client-id")
	natsClusterId := viper.GetString("nats.cluster-id")
	natsSubject := viper.GetString("nats.subject")
	mdbUrl := viper.GetString("mdb.url")

	var err1 error
	MdbConn, err1 = sql.Open("postgres", mdbUrl )
	boil.SetDB(MdbConn)
	utils.Must(err1)
	utils.Must(MdbConn.Ping())
	defer MdbConn.Close()


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
		for  range signalChan {
			fmt.Printf("\nReceived an interrupt, unsubscribing and closing connection...\n\n")
			// Do not unsubscribe a durable on exit, except if asked to.
			sc.Close()
			cleanupDone <- true
		}
	}()
	<-cleanupDone
}



//checks message type and calls "eventHandler"
func msgHandler(msg *stan.Msg) {





	type TypeTest struct {
		Type string `json:"type"`
	}

	var test TypeTest
	err := json.Unmarshal(msg.Data, &test)
	if err != nil {
		fmt.Printf("json.Unmarshal error: %s\n", err)
	}

// check if event is FILE_REPLACE and run function
	if test.Type != E_FILE_REPLACE {

		type SimpleData struct {
			Id      string `json:"id"`
			Type    string `json:"type"`
			Payload struct {
				ID  int64  `json:"id"`
				UID string `json:"uid"`
			} `json:"payload"`
		}

		var PayloadData SimpleData

		err = json.Unmarshal(msg.Data, &PayloadData)
		if err != nil {
			fmt.Printf("json.Unmarshal error: %s\n", err)
		}
		//fmt.Println(PayloadData)

		eventHandler(PayloadData.Type, PayloadData.Payload.UID, "")
		//test1(MdbConn, PayloadData.Payload.UID)
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

		var PayloadData ReplaceData
		err = json.Unmarshal(msg.Data, &PayloadData)
		if err != nil {
			fmt.Printf("json.Unmarshal error: %s\n", err)
		}
		//fmt.Println(PayloadData)
		eventHandler(PayloadData.Type, PayloadData.Payload.New.UID, PayloadData.Payload.Old.UID)
	}

}


//calls searching functions
func eventHandler(eventType string, uid string, oldUid string) {
	fmt.Println("old_uid: " + oldUid + ",new_uid: " + uid + "  eventType: " + eventType)
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
		log.Errorf("unknown event type: %s uid: %s", eventType, uid)
	}
}

func Unzip(db *sql.DB, u string) int16 {
	mdbFile := mdbmodels.Files(db, qm.Where("uid=?", u))
	OneFile, _ := mdbFile.One()
	fmt.Printf("\n*****************%v\n",OneFile.Secure)
	if OneFile.Secure == 0 {
		_, err := http.Get("http://API/" + u)
			if err != nil{
				log.Error()
			}
	}
	return OneFile.Secure
}
