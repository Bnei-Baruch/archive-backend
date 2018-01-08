package events

import (
	"fmt"
	"log"
	"os"
	"os/signal"

	"github.com/nats-io/go-nats-streaming"
	"github.com/spf13/viper"
	"encoding/json"
)

func printMsg(m *stan.Msg, i int) {
	log.Printf("[#%d] Received on [%s]: '%v'\n", i, m.Subject, m.Data)
}

// struct for the messages from bus
type Payload struct {
	Id int64 `json:"id"`
	Uid	string `json:"uid"`
}

type Data struct {
	Id string `json:"id"`
	Type string `json:"type"`
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
		log.Fatalf("Can't connect: %v.\nMake sure a NATS Streaming Server is running at: %s", err, natsUrl)
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
		var b Data

		err := json.Unmarshal(msg.Data, &b)
		if err != nil {
			fmt.Printf("json.Unmarshal error: %s\n",err)
		}
		fmt.Printf("%v\n",b.Payload.Uid)

}