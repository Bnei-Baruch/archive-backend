package events

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"


	"github.com/Bnei-Baruch/archive-backend/mdb/models"
	"github.com/Bnei-Baruch/archive-backend/utils"
	log "github.com/Sirupsen/logrus"
	"github.com/nats-io/go-nats-streaming"
	"github.com/spf13/viper"
	"github.com/volatiletech/sqlboiler/boil"
	"github.com/volatiletech/sqlboiler/queries/qm"
)

// MdbConn pointer to connection to db
var MdbConn *sql.DB

// RunLListener function sdfsf sdfsdf
func RunLListener() {
	// variables for connections to sources
	natsURL := viper.GetString("nats.url")
	natsClientID := viper.GetString("nats.client-id")
	natsClusterID := viper.GetString("nats.cluster-id")
	natsSubject := viper.GetString("nats.subject")
	mdbURL := viper.GetString("mdb.url")

	// connect to postgres
	var err error
	MdbConn, err = sql.Open("postgres", mdbURL)
	utils.Must(err)
	utils.Must(MdbConn.Ping())

	log.Infof("Connected to db %v", MdbConn)
	boil.SetDB(MdbConn)
	defer MdbConn.Close()


	// connect to nats server
	sc, err := stan.Connect(natsClusterID, natsClientID, stan.NatsURL(natsURL))
	if err != nil {
		log.Fatalf("Can't connect: %v.\nMake sure a NATS Streaming Server is running at: %s\n retrying:", err, natsURL)
		panic("...")
	}
	defer sc.Close()
	log.Printf("Connected to %s clusterID: [%s] clientID: [%s]\n", natsURL, natsClusterID, natsClientID)


	// connection options
	startOpt := stan.DeliverAllAvailable()
	_, err = sc.Subscribe(natsSubject, msgHandler, startOpt)
	if err != nil {
		log.Fatalln("couldn't subscribe to nats", err)
	}
	log.Printf("Listening on [%s], clientID=[%s], durable=[%s]\n", natsSubject, natsURL, "false")

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

//Data struct for unmarshaling data
type Data struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	Payload map[string]interface{} `json:"payload"`
}

//checks message type and calls "eventHandler"
func msgHandler(msg *stan.Msg) {

	msgSrcData := msg.Data
	var MsgData Data

	err := json.Unmarshal(msgSrcData, &MsgData)
	if err != nil {
		log.Errorf("json.Unmarshal error: %s\n", err)
	}

	switch MsgData.Type {
	case E_COLLECTION_CREATE:
		CollectionCreate(MsgData)
	case E_COLLECTION_DELETE:
		CollectionDelete(MsgData)
	case E_COLLECTION_UPDATE:
		CollectionUpdate(MsgData)
	case E_COLLECTION_PUBLISHED_CHANGE:
		CollectionPublishedChange(MsgData)
	case E_COLLECTION_CONTENT_UNITS_CHANGE:
		CollectionContentUnitsChange(MsgData)
	case E_CONTENT_UNIT_CREATE:
		ContentUnitCreate(MsgData)
	case E_CONTENT_UNIT_DELETE:
		ContentUnitDelete(MsgData)
	case E_CONTENT_UNIT_UPDATE:
		ContentUnitUpdate(MsgData)
	case E_CONTENT_UNIT_PUBLISHED_CHANGE:
		ContentUnitPublishedChange(MsgData)
	case E_CONTENT_UNIT_DERIVATIVES_CHANGE:
		ContentUnitDerivativesChange(MsgData)
	case E_CONTENT_UNIT_SOURCES_CHANGE:
		ContentUnitSourcesChange(MsgData)
	case E_CONTENT_UNIT_TAGS_CHANGE:
		ContentUnitTagsChange(MsgData)
	case E_CONTENT_UNIT_PERSONS_CHANGE:
		ContentUnitPersonsChange(MsgData)
	case E_CONTENT_UNIT_PUBLISHERS_CHANGE:
		ContentUnitPublishersChange(MsgData)
	case E_FILE_PUBLISHED:
		FilePublished(MsgData)
	case E_FILE_REPLACE:
		FileReplace(MsgData)
	case E_FILE_INSERT:
		FileInsert(MsgData)
	case E_FILE_UPDATE:
		FileUpdate(MsgData)
	case E_SOURCE_CREATE:
		SourceCreate(MsgData)
	case E_SOURCE_UPDATE:
		SourceUpdate(MsgData)
	case E_TAG_CREATE:
		TagCreate(MsgData)
	case E_TAG_UPDATE:
		TagUpdate(MsgData)
	case E_PERSON_CREATE:
		PersonCreate(MsgData)
	case E_PERSON_DELETE:
		PersonDelete(MsgData)
	case E_PERSON_UPDATE:
		PersonUpdate(MsgData)
	case E_PUBLISHER_CREATE:
		PublisherCreate(MsgData)
	case E_PUBLISHER_UPDATE:
		PublisherUpdate(MsgData)
	//default:
	//	log.Errorf("unknown event type: %s MsgData: %s", eventType, MsgData)
	}
}

// GetFileObj gets the file object from db
func GetFileObj(uid string) *mdbmodels.File {
	//fmt.Println("uid is: ", uid)
	utils.Must(MdbConn.Ping())
	mdbObj := mdbmodels.Files(MdbConn, qm.Where("uid=?", uid))
	OneFile, err := mdbObj.One()
	if err != nil {
		log.Error(err)
	}

	return OneFile
}

func GetUnitObj(uid string) *mdbmodels.ContentUnit {
	//fmt.Println("uid is: ", uid)
	utils.Must(MdbConn.Ping())
	mdbObj := mdbmodels.ContentUnits(MdbConn, qm.Where("uid=?", uid))
	OneObj, err := mdbObj.One()
	if err != nil {
		log.Error(err)
	}

	return OneObj
}


//func unZipFile(uid string) error {
//
//	file := GetFileObj(uid)
//	if (file.Type == "image" ||
//		strings.HasSuffix(file.Name, ".zip")) &&
//		file.Secure != 1 {
//		fmt.Printf("\n*********************************\n%+v\n", file)
//		fmt.Println("IMAGE!!! ", file.UID)
//
//		resp, err := http.Get(BACKEND_URL + "/" + uid)
//		if err != nil {
//			return err
//		}
//		fmt.Println(resp)
//	}
//	return nil
//}
