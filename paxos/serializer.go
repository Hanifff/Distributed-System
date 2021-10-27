package main

import (
	"bufio"
	"context"
	"encoding/gob"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net"

	"github.com/Bank_app/failuredetector"
	"github.com/bank_app/multipaxos"
)

// forwadrdClientResp looks up for client id from the response struct and connected port.
// Then forwards response if there are any.
func forwadrdClientResp(resp *multipaxos.Response, stop chan struct{}) {
	if ds.clients != nil {
		for _, c := range ds.clients {
			rw := bufio.NewReadWriter(bufio.NewReader(c), bufio.NewWriter(c))
			enc := gob.NewEncoder(rw)
			err := enc.Encode(&resp)
			if err == io.EOF {
				fmt.Println("Connection to client is closed: ", c.RemoteAddr().String())
				ds.clients = removeNode(ds.clients, c)
				stop <- struct{}{}
				break
			}
			if err != nil {
				fmt.Println("Could not send response to: ", c.RemoteAddr().String())
			}
			err = rw.Flush()
			if err != nil {
				fmt.Println("Flush failed.", err)
				ds.clients = removeNode(ds.clients, c)
				stop <- struct{}{}
				continue
			}
		}
	}
}

// sendToLeader sends paxos messages to the leader
func sendToLeader(msg PaxosMsg, leaderID int, stop chan struct{}) { // from leader to every one
	for _, conn := range ds.servers {
		node := conn.RemoteAddr().String()
		id := convertID(node)
		if id == leaderID {
			rw := bufio.NewWriter(conn)
			enc := gob.NewEncoder(rw)
			err := enc.Encode(&msg)
			if err != nil && err != io.EOF {
				log.Println("Error while encoding Paxos messages!", err)
			}
			if err == io.EOF {
				//fmt.Println("Connection to the client is closed!")
				ds.servers = removeNode(ds.servers, conn)
				stop <- struct{}{}
				continue
			}
			err = rw.Flush()
			if err != nil {
				ds.servers = removeNode(ds.servers, conn)
				stop <- struct{}{}
				fmt.Println("Flush failed", err)
				continue
			}
		}
	}
}

// broadCasetPxmsg broadcastes paxos messages to other paxos nodes
func broadCasetPxmsg(msg PaxosMsg, stop chan struct{}) { // from leader to every one
	for _, c := range ds.servers {
		rw := bufio.NewWriter(c)
		enc := gob.NewEncoder(rw)
		err := enc.Encode(&msg)
		if err != nil && err != io.EOF {
			log.Println("Error while encoding Paxos messages!", err)
		}
		if err == io.EOF {
			//fmt.Println("Connection to the paxos server is closed!")
			ds.servers = removeNode(ds.servers, c)
			stop <- struct{}{}
			continue
		}
		err = rw.Flush()
		if err != nil {
			ds.servers = removeNode(ds.servers, c)
			stop <- struct{}{}
			fmt.Println("Flush failed", err)
			continue
		}
	}
}

// forwardReplyHB sends hb messages in form of replay
func forwardReplyHB(hb failuredetector.Heartbeat, stop chan struct{}) {
	for _, n := range ds.servers {
		node := n.RemoteAddr().String()
		id := convertID(node)
		if id == hb.To { // only send to who it belongs to ***
			rw := bufio.NewWriter(n)
			enc := gob.NewEncoder(rw)
			pxmsg := PaxosMsg{
				Mod: "HB",
				Msg: hb,
			}
			err := enc.Encode(&pxmsg)
			if err != nil {
				if err == io.EOF {
					ds.servers = removeNode(ds.servers, n)
					stop <- struct{}{}
					return
				}
			}
			err = rw.Flush()
			if err != nil {
				ds.servers = removeNode(ds.servers, n)
				stop <- struct{}{}
				fmt.Println("Flush failed", err)
				continue
			}
		}
	}
}

// broadcastHB broadcasts hb messages to all paxos servers
func broadcastHB(myID int, ctx context.Context, stop chan struct{}) {
	for _, c := range ds.servers {
		n := rand.Intn(ds.nrOfServers)
		rw := bufio.NewWriter(c)
		enc := gob.NewEncoder(rw)
		hbReq := failuredetector.Heartbeat{To: ds.nodes[n], From: myID, Request: true}
		pxmsg := PaxosMsg{
			Mod: "HB",
			Msg: hbReq,
		}
		err := enc.Encode(&pxmsg)
		if err == io.EOF {
			//fmt.Println("Connection to the paxos server is closed!")
			ds.servers = removeNode(ds.servers, c)
			stop <- struct{}{}
			return
		}
		if err != nil && err != io.EOF {
			log.Println("Error while encoding hb!", err)
		}

		err = rw.Flush()
		if err != nil {
			ds.servers = removeNode(ds.servers, c)
			stop <- struct{}{}
			fmt.Println("Flush failed", err)
			return
		}
	}
}

// reciever recieves hb messages from all nodes
func reciever(conn *net.TCPConn) (PaxosMsg, error) {
	rw := bufio.NewReader(conn)
	dec := gob.NewDecoder(rw)
	var pxmsg PaxosMsg
	err := dec.Decode(&pxmsg)
	if err == io.EOF {
		fmt.Println("Connection to the paxos server is closed!")
		//ds.servers = removeNode(detector.servers, conn)
		return pxmsg, err
	}
	if err == io.ErrClosedPipe {
		//log.Println("Error while decoding hb, closed pipe.", err)
		return pxmsg, err
	}
	if err != nil && err != io.EOF && err != io.ErrClosedPipe {
		//log.Println("Error while decoding hb.", err)
		return pxmsg, nil
	}
	return pxmsg, nil
}
