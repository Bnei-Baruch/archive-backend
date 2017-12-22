package events

import (
	"fmt"

	"github.com/nats-io/go-nats-streaming"
)

func RunLListener() {
	fmt.Println("Hello World")

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
	_, err := stan.Connect("clusterID", "clientID")
	if err != nil {
		panic(err)
	}

}
