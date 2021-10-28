package main

import (
	"bufio"
	"encoding/gob"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strings"
	"time"

	"dnb/bank_app/bank"
	"dnb/bank_app/multipaxos"
)

// paxosConnection connects a client with one of the running paxos nodes.
// mpaxoscli: A Paxos client instance
// Returnes: A TCP connection to Paxos leader
func paxosConnection(mpaxoscli *MPaxosClient) *net.TCPConn {
	paxosServer := mpaxoscli.paxosServers[mpaxoscli.currentServer]
	master, err := net.ResolveTCPAddr("tcp4", paxosServer)
	if err != nil {
		fmt.Println("Could not connect to node with address: ", paxosServer, err)
		return nil
	}
	//cliPort, err := net.ResolveTCPAddr("tcp4", cliPorts[mpaxoscli.portIdx])
	cliPort, err := net.ResolveTCPAddr("tcp4", mpaxoscli.currentPort)
	if err != nil {
		fmt.Println("Could not connect to node with address: ", paxosServer, err)
		return nil
	}
	c, err := net.DialTCP("tcp4", cliPort, master)
	if err != nil {
		fmt.Println("Could not connect to node with address: ", paxosServer, err)
		return nil
	}
	fmt.Printf("Connected to paxos node with address: %s, from port: %d\n", paxosServer, cliPort.Port)
	return c
}

// response decodes the incomming response from a paxos server.
// It signals the main loop with an unbufferend channel.
// mpaxoscli: A Paxos client instance
func response(mpaxoscli *MPaxosClient) {
	for {
		rw := bufio.NewReader(mpaxoscli.paxosConn)
		dec := gob.NewDecoder(rw)
		var resp multipaxos.Response
		err := dec.Decode(&resp)
		if err == io.EOF {
			mpaxoscli.changePort()
			changed := mpaxoscli.changeConnection()
			if !changed {
				fmt.Println("There is NO MORE Paxos server that we can connect to!")
				return
			}
			mpaxoscli.paxosConn = paxosConnection(mpaxoscli)
			continue
		}
		if err != nil {
			log.Println("Node failed to decode! ", err)
			continue
		}
		mpaxoscli.responseChan <- resp
	}
}

// request asks a client to enter its request in command line and process it.
// It signals the main loop with an unbufferend channel.
// mpaxoscli: A Paxos client instance
// scanner: A Scan inctance to read the stdin from command line
func request(mpaxoscli *MPaxosClient, scanner *bufio.Scanner) {
	for {
		fmt.Println("Enter 1 to send a Transaction requests and 2 to send a Reconfiguration request:")
		scanner.Scan()
		cmd := scanner.Text()
		input := strings.Split(cmd, ",")
		if err := scanner.Err(); err != nil || cmd == "" || len(input) != 1 {
			fmt.Println("Error reading your input, please try again!")
			continue
		}
		switch cmd {
		case "1":
			// (account number, transaction type and amount).
			fmt.Println("Enter account number, transaction type, and amount:")
			scanner.Scan()
			cmd := scanner.Text()
			input := strings.Split(cmd, ",")
			if err := scanner.Err(); err != nil || cmd == "" || len(input) != 3 {
				fmt.Println("Error reading your input, please try again!")
				continue
			}
			// increase the sequence number
			mpaxoscli.increaseSeq()
			request := multipaxos.Value{
				ClientID:   mpaxoscli.clientID,
				ClientSeq:  mpaxoscli.clientSeq,
				AccountNum: converter(input[0]),
				Txn: bank.Transaction{
					Op:     bank.Operation(converter(input[1])),
					Amount: converter(input[2]),
				},
				RC: multipaxos.ReConfiguration{
					Reconfig: false,
				},
			}
			mpaxoscli.requestChan <- request
		case "2":
			fmt.Println("Enter the number of paxos nodes you want to scale up to:")
			scanner.Scan()
			cmd := scanner.Text()
			input := strings.Split(cmd, ",")
			if err := scanner.Err(); err != nil || cmd == "" || len(input) != 1 {
				fmt.Println("Error reading your input, please try again!")
				continue
			}
			// increase the sequence number
			mpaxoscli.increaseSeq()
			mpaxoscli.totalSevers = converter(input[0])
			request := multipaxos.Value{
				ClientID:   mpaxoscli.clientID,
				ClientSeq:  mpaxoscli.clientSeq,
				AccountNum: 0,
				Txn:        bank.Transaction{},
				RC: multipaxos.ReConfiguration{
					Reconfig:      true,
					NumberOfNodes: mpaxoscli.totalSevers,
				},
			}
			mpaxoscli.requestReconfig <- request
		}
	}
}

// sendRequest sends a request from a client to a paxos node.
// It signals the main loop with an unbufferend channel.
// mpaxoscli: A Paxos client instance.
// request: A multipaxos instance of Value.
func sendRequest(mpaxoscli *MPaxosClient, request multipaxos.Value) bool {
	rw := bufio.NewWriter(mpaxoscli.paxosConn)
	e := gob.NewEncoder(rw)
	pxCli := PaxosMsg{
		Mod: "CLIENTREQ",
		Msg: request,
	}
	err := e.Encode(&pxCli)
	if err == io.EOF {
		log.Println("Client connection closed! ", err)
		mpaxoscli.changePort()
		changed := mpaxoscli.changeConnection()
		if !changed {
			fmt.Println("There is NO MORE Paxos server that we can connect to!")
			return false
		}
		mpaxoscli.paxosConn = paxosConnection(mpaxoscli)
		return true
	}
	if err != nil {
		log.Println("Client failed to encode! ", err)
	}
	err = rw.Flush()
	if err != nil {
		fmt.Println("Flush failed", err)
	}
	return true
}

// paxosClient is a multipaxos client that communicates with paxos servers.
// nodeID: The tcp port which the client use to connect to the server
// nrOfServers: the number of existing servers.
func paxosClient(nodeID string, nrOfServers int) {
	time.Sleep(time.Second)
	mpaxoscli := NewMPaxosClient(nrOfServers)
	mpaxoscli.intiGobs()
	mpaxoscli.clientID = nodeID
	//mpaxoscli.setStartPort(nodeID)
	mpaxoscli.currentPort = nodeID
	// connect to a paxos server
	mpaxoscli.paxosConn = paxosConnection(mpaxoscli)
	scanner := bufio.NewScanner(os.Stdin)
	// start the reciever and sender go routines
	go response(mpaxoscli)
	go request(mpaxoscli, scanner)
mainLoop:
	for {
		select {
		case request := <-mpaxoscli.requestChan:
			isServer := sendRequest(mpaxoscli, request)
			if !isServer {
				mpaxoscli.paxosConn.Close()
				break mainLoop
			}
			mpaxoscli.timeOut = time.After(30 * time.Second)
		case reconf := <-mpaxoscli.requestReconfig:
			isServer := sendRequest(mpaxoscli, reconf)
			if !isServer {
				mpaxoscli.paxosConn.Close()
				break mainLoop
			}
		case resp := <-mpaxoscli.responseChan:
			fmt.Printf("%s\n", resp.TxnRes.String())
			mpaxoscli.timeOut = make(chan time.Time) // set to nil after response
			if resp.ScaledUp {
				mpaxoscli.totalSevers = resp.NrOfNodes
				mpaxoscli.currentServer = mpaxoscli.totalSevers - 1
				mpaxoscli.changePort()
				mpaxoscli.paxosConn = paxosConnection(mpaxoscli)
			}
		case <-mpaxoscli.timeOut:
			fmt.Printf("Paxos server did not repsond!\n***Time out!!!\n")
			mpaxoscli.changePort()
			changed := mpaxoscli.changeConnection()
			if !changed {
				fmt.Println("There is NO MORE Paxos server that we can connect to!")
				mpaxoscli.paxosConn.Close()
				break mainLoop
			}
			mpaxoscli.paxosConn = paxosConnection(mpaxoscli)
		}
	}
}
