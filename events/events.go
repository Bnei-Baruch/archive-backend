package events

import (
	"fmt"
	"log"
	"os"
	"os/signal"


	"github.com/nats-io/go-nats-streaming"
	"github.com/spf13/viper"

)


func printMsg(m *stan.Msg, i int) {
	log.Printf("[#%d] Received on [%s]: '%s'\n", i, m.Subject, m)
}




func RunLListener() {



	// TODO:
	// 1. read from config and setup connctions to:
	// MDB
	// ES
	// NATS streaming server
	// config for assets api

	// First task - connect to nats and print all events to log
	// connect to nats

	// Unfortunately, there is an open issue regarding connection failures on startup.
	// see https://github.com/nats-io/go-nats/issues/195
	// we should upgrade as soon as it's fixed !

	natsUrl := viper.GetString("nats.url")
	natsClientId := viper.GetString("nats.client-id")
	natsClusterId := viper.GetString("nats.cluster-id")
	natsSubject := viper.GetString("nats.subject")

	sc, err := stan.Connect(natsClusterId, natsClientId, stan.NatsURL(natsUrl))
	if err != nil {
		log.Fatalf("Can't connect: %v.\nMake sure a NATS Streaming Server is running at: %s", err, natsUrl)
	}


	log.Printf("Connected to %s clusterID: [%s] clientID: [%s]\n", natsUrl, natsClusterId, natsClientId)

	subj := natsSubject
	i := 0

	mcb := func(msg *stan.Msg) {
		i++
		printMsg(msg, i)
	}

	startOpt := stan.DeliverAllAvailable()

	sc.QueueSubscribe(subj, "", mcb, startOpt)

	log.Printf("Listening on [%s], clientID=[%s], qgroup=[%s] durable=[%s]\n", subj, natsUrl)




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
