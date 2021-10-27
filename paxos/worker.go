package main

import (
	"fmt"
	"net"
	"time"

	"bank_app/failuredetector"
	"bank_app/leaderdetector"
	"bank_app/multipaxos"
)

// initInstances initializes intances of all paxos components and leader detector
func initInstances() (*failuredetector.EvtFailureDetector, *leaderdetector.MonLeaderDetector,
	<-chan int, *multipaxos.Proposer, *multipaxos.Acceptor, *multipaxos.Learner) {
	// initilize detectors
	ld := ds.LdInstnace()
	fd := ds.FdInstance(ld)
	subscriber := ld.Subscribe()
	// initilize paxos compononets
	proposer := ds.proposerIns(ld)
	acceptor := ds.acceptorIns()
	learner := ds.learnIns()
	return fd, ld, subscriber, proposer, acceptor, learner
}

// connect makes TCP connection to other servers
func connect(server string) error {
	time.Sleep(7 * time.Second)
	for i, ip := range ds.serverIPs[:ds.nrOfServers] {
		master, _ := net.ResolveTCPAddr("tcp4", ip)
		c, err := net.DialTCP("tcp4", nil, master)
		if err != nil {
			fmt.Println("Could not connect to the Master node! ", err)
			return err
		}
		ds.servers[i] = c
	}
	return nil
}

// connectReconf makes a TCP connection with the new server
func connectReconf() error {
	newNode := ds.serverIPs[ds.nrOfServers-1]
	master, _ := net.ResolveTCPAddr("tcp4", newNode)
	c, err := net.DialTCP("tcp4", nil, master)
	if err != nil {
		fmt.Println("Could not connect to the Master node! ", err)
		return err
	}
	ds.servers = append(ds.servers, c)
	return nil
}
