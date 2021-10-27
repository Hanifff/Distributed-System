package main

import (
	"bufio"
	"context"
	"encoding/gob"
	"fmt"
	"io"
	"log"
	"net"
	"strings"
	"sync"
	"time"

	"bgithub.com/Bank_app/leaderdetector"
	"github.com/Bank_app/multipaxos"

	"github.com/Bank_app/failuredetector"
)

// handlePaxosMsg handles the consesus communication messages in phase one of Paxos algorithm.
// conn: A TCP connection
// fd: An instance of failuredetector
// ld: An instance of leaderdetector
// proposer, acceptor and learner: Instance of each Paxos roles
// ctx: Manages the go routine in background.
func handlePhaseOne(conn *net.TCPConn, fd *failuredetector.EvtFailureDetector,
	ld *leaderdetector.MonLeaderDetector, proposer *multipaxos.Proposer,
	acceptor *multipaxos.Acceptor, learner *multipaxos.Learner, ctx context.Context) {
	fmt.Println("> New Connection!")
	stop := make(chan struct{})
	defer conn.Close()
	for {
		if ds.nrOfServers == count {
			count = ds.nrOfServers + 1
			if !fd.Started() {
				fd.Start()
				proposer.Start()
				acceptor.Start()
				learner.Start()
			}
			fmt.Println("All Components Configuered!", count)
		}
		if count > ds.nrOfServers {
			select {
			case prep := <-ds.prepare:
				msg := PaxosMsg{
					Mod: "PREPARE",
					Msg: prep,
				}
				broadCasetPxmsg(msg, stop)
			case prom := <-ds.promiseOut:
				msg := PaxosMsg{
					Mod: "PROMISE",
					Msg: prom,
				}
				sendToLeader(msg, ld.Leader(), stop)
			case <-stop:
				return
			case <-ctx.Done():
				fmt.Println("paxos handler CLOESD!")
				return
			}
		}
	}
}

// handlePhaseTwo handles the phase 2 of communication messages between servers in Paxos algorithm
// conn: A TCP connection
// fd: An instance of failuredetector
// ld: An instance of leaderdetector
// proposer, acceptor and learner: Instance of each Paxos roles
// ctx: Manages the go routine in background.
func handlePhaseTwo(conn *net.TCPConn, fd *failuredetector.EvtFailureDetector,
	ld *leaderdetector.MonLeaderDetector, proposer *multipaxos.Proposer,
	acceptor *multipaxos.Acceptor, learner *multipaxos.Learner, ctx context.Context) {
	stop := make(chan struct{})
	defer conn.Close()
	for {
		select {
		case acc := <-ds.acceptOut:
			msg := PaxosMsg{
				Mod: "ACCEPT",
				Msg: acc,
			}
			broadCasetPxmsg(msg, stop)
		case lrn := <-ds.learnOut:
			msg := PaxosMsg{
				Mod: "LEARN",
				Msg: lrn,
			}
			broadCasetPxmsg(msg, stop)
		case dv := <-ds.decidedOut:
			ds.handleDecideValue(&dv, fd, ld, proposer, acceptor, learner)
		case <-stop:
			return
		case <-ctx.Done():
			fmt.Println("paxos handler CLOESD!")
			return
		}
	}
}

// pxmsgReciver recieves and handles consensus messages from Paxos servers
// conn: A TCP connection
// fd: An instance of failuredetector
// ld: An instance of leaderdetector
// proposer, acceptor and learner: Instance of each Paxos roles
// myID: id of the current server
// ctx: Manages the go routine in background.
func pxmsgReciver(conn *net.TCPConn, proposer *multipaxos.Proposer, fd *failuredetector.EvtFailureDetector,
	acceptor *multipaxos.Acceptor, learner *multipaxos.Learner, myID int, ctx context.Context) {
	defer conn.Close()
	for {
		rw := bufio.NewReader(conn)
		dec := gob.NewDecoder(rw)
		var pxmsg PaxosMsg
		err := dec.Decode(&pxmsg)
		if err == io.EOF {
			fmt.Println("Connection to the paxos server is closed!")
			ds.servers = removeNode(ds.servers, conn)
			return
		}
		if err == io.ErrClosedPipe {
			log.Println("Error while decoding Paxos messages, closed pipe !", err)
			return
		}
		if pxmsg.Msg != nil {
			switch pxmsg.Mod {
			case "PREPARE":
				acceptor.DeliverPrepare(pxmsg.Msg.(multipaxos.Prepare))
			case "PROMISE":
				proposer.DeliverPromise(pxmsg.Msg.(multipaxos.Promise))
			case "ACCEPT":
				fmt.Println("rcv accept")
				acceptor.DeliverAccept(pxmsg.Msg.(multipaxos.Accept))
			case "LEARN":
				fmt.Println("rcv lrn")
				learner.DeliverLearn(pxmsg.Msg.(multipaxos.Learn))
			case "CLIENTREQ":
				proposer.DeliverClientValue(pxmsg.Msg.(multipaxos.Value))
			case "HB":
				if pxmsg.Msg.(failuredetector.Heartbeat).To == myID {
					fd.DeliverHeartbeat(pxmsg.Msg.(failuredetector.Heartbeat))
				}
			}
		}
		select {
		case <-ctx.Done():
			fmt.Println("paxosRec CLOSED!")
			return
		default:
		}
	}
}

// handleHB handles hearbeat messages of each concurrent node's connected to the server.
// conn: A TCP connection
// fd: An instance of failuredetector
// ld: An instance of leaderdetector
// proposer, acceptor and learner: Instance of each Paxos roles
// myID: id of the current server
// ctx: Manages the go routine in background.
func handleHB(conn *net.TCPConn, fd *failuredetector.EvtFailureDetector,
	ld *leaderdetector.MonLeaderDetector, subscriber <-chan int, myID int,
	proposer *multipaxos.Proposer, acceptor *multipaxos.Acceptor,
	learner *multipaxos.Learner, ctx context.Context) {
	ticker := time.NewTicker(40 * time.Millisecond)
	stop := make(chan struct{})
	defer conn.Close()
	for {
		select {
		case <-ticker.C:
			broadcastHB(myID, ctx, stop)
		case reply := <-ds.resp:
			forwardReplyHB(reply, stop)
		case leader := <-subscriber:
			fmt.Printf("Leader changed!\nThe New leader is: %d\n", leader)
		case <-stop:
			return
		case <-ctx.Done():
			fmt.Println("handleHB CLOSED!")
			return
		}
	}
}

// handleClients handles each client connection to the Paxos server.
// conn: A TCP connection
// fd: An instance of failuredetector
// ld: An instance of leaderdetector
// proposer, acceptor and learner: Instance of each Paxos roles
// myID: id of the current server
// ctx: Manages the go routine in background.
func handleClients(conn *net.TCPConn, proposer *multipaxos.Proposer, ld *leaderdetector.MonLeaderDetector,
	myID int, ctx context.Context) {
	fmt.Println("A Client is connected!", conn.RemoteAddr().String())
	stop := make(chan struct{})
	defer conn.Close()
	for {
		r := bufio.NewReader(conn)
		dec := gob.NewDecoder(r)
		var pxmsg PaxosMsg
		err := dec.Decode(&pxmsg)
		if err == io.EOF {
			//fmt.Println("Connection to the paxos server is closed!")
			ds.clients = removeNode(ds.clients, conn)
			return
		}
		if err != nil && err != io.EOF {
			log.Println("Error while decoding client request.", err)
			return
		}
		fmt.Println("rcv :", pxmsg)
		if pxmsg.Msg != nil {
			switch pxmsg.Mod {
			case "CLIENTREQ":
				if myID == ld.Leader() {
					proposer.DeliverClientValue(pxmsg.Msg.(multipaxos.Value))
				} else {
					sendToLeader(pxmsg, ld.Leader(), stop)
				}
			}
		}
		select {
		case <-stop:
			return
		case <-ctx.Done():
			fmt.Println("handleClients CLOSED")
			return
		default:
		}
	}
}

// paxosNode makes connection with other servers and clients.
// This function routes each thread in several go routines.
// Each go routine handles communication messages betweeen paxos instances.
// server: IP Address of the server
// ctx: Keep tracks of each go routine running in background
// wg: An instance of WaitGroup to manage the go routines
func paxosNode(server string, ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()
	connections := make(chan *net.TCPConn)
	s := strings.Split(server, "-")
	master, _ := net.ResolveTCPAddr("tcp4", s[0])
	l, err := net.ListenTCP("tcp4", master)
	if err != nil {
		fmt.Println("error while connecting to the server: ", err)
		return
	}
	fmt.Println("Paxos node running on address: ", server)
	serverID := convertID(s[0])
	nrOfServers := converter(s[1])
	ds = NewDistributedServer(serverID, nrOfServers)
	fd, ld, subscriber, proposer, acceptor, learner := initInstances()
	ds.initGob() // initilize gob with paxos messages

	err = connect(s[0])
	if err != nil {
		fmt.Println("error connecting, ", err)
		return
	}
	defer l.Close()
	go func() {
		for {
			conn, err := l.AcceptTCP()
			if err != nil {
				log.Println("connection, ", err)
				return
			}
			connections <- conn
		}
	}()
	for {
		select {
		case conn := <-connections:
			if ds.isClient(conn) {
				ds.clients = append(ds.clients, conn)
				go handleClients(conn, proposer, ld, serverID, ctx)
			} else {
				count++
				go handlePhaseOne(conn, fd, ld, proposer, acceptor, learner, ctx)
				go handlePhaseTwo(conn, fd, ld, proposer, acceptor, learner, ctx)
				go pxmsgReciver(conn, proposer, fd, acceptor, learner, serverID, ctx)
				go handleHB(conn, fd, ld, subscriber, serverID, proposer, acceptor, learner, ctx)
			}
		case <-ctx.Done():
			fd.Stop()
			proposer.Stop()
			acceptor.Stop()
			learner.Stop()
			fmt.Print("Stoping all go routines!")
			return
		}
	}
}
