package main

import (
	"encoding/gob"
	"fmt"
	"net"
	"strings"
	"time"

	"dnb/bank_app/multipaxos"
)

// MPaxosClient handles client communicatations with the paxos server
type MPaxosClient struct {
	clientSeq     int
	clientID      string
	currentPort   string
	totalSevers   int
	currentServer int
	paxosServers  []string
	paxosConn     *net.TCPConn
	portIdx       int

	responseChan    chan multipaxos.Response
	requestChan     chan multipaxos.Value
	requestReconfig chan multipaxos.Value
	//scaleUpConn     chan bool
	timeOut <-chan time.Time
}

// NewMPaxosClient creates a new ..
func NewMPaxosClient(nrOfServers int) *MPaxosClient {
	paxosServers := []string{"localhost:1234", "localhost:1235", "localhost:1236"}
	/* ,
	"localhost:1237", "localhost:1238", "localhost:1239", "localhost:1240"} */
	//paxosServers := []string{"172.28.1.2:1234", "172.28.1.3:1235", "172.28.1.4:1236",
	//	"172.28.1.5:1237", "172.28.1.6:1238", "172.28.1.7:1239", "172.28.1.8:1240"} // docker netwrok
	//paxosServers := []string{"152.94.1.110:1234", "152.94.1.119:1235", "152.94.1.115:1236"}
	return &MPaxosClient{
		clientSeq:       0,
		clientID:        "",
		portIdx:         0,
		totalSevers:     nrOfServers,
		currentServer:   nrOfServers - 1,
		paxosServers:    paxosServers,
		responseChan:    make(chan multipaxos.Response),
		requestChan:     make(chan multipaxos.Value),
		requestReconfig: make(chan multipaxos.Value),
		//scaleUpConn:     make(chan bool),
		timeOut: make(chan time.Time),
	}
}

func (pc *MPaxosClient) increaseSeq() {
	pc.clientSeq++
}

func (pc *MPaxosClient) changePort() {
	port := converter(strings.Split(pc.currentPort, ":")[1])
	port--
	pc.currentPort = fmt.Sprintf(":%d", port)
}

func (pc *MPaxosClient) changeConnection() bool {
	if pc.currentServer > 0 {
		pc.currentServer--
		return true
	}
	return false
}

func (pc *MPaxosClient) intiGobs() {
	gob.Register(multipaxos.Response{})
	gob.Register(multipaxos.Value{})
	gob.Register(PaxosMsg{})
}
