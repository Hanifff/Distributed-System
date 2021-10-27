package main

import (
	"fmt"
	"net"
	"strconv"
	"strings"
)

var (
	count      = 0
	ds         *DistributedServer
	allNodeIDs = []int{1234, 1235, 1236 /* , 1237, 1238, 1239, 1240 */}
)

// convertID convertes string flag used to start each node to int
func convertID(node string) int {
	id, err := strconv.Atoi(strings.Split(node, ":")[1])
	if err != nil {
		fmt.Println("Could not convert!")
	}
	return id
}

// converter convertes any string to int
func converter(id string) int {
	node, err := strconv.Atoi(id)
	if err != nil {
		fmt.Println("Could not convert!")
	}
	return node
}

// removeNode romeves a disconnected node from the master node
func removeNode(svs []*net.TCPConn, s *net.TCPConn) []*net.TCPConn {
	node := s.RemoteAddr().String()
	id := convertID(node)
	for i, n := range svs {
		remoteAdd := n.RemoteAddr().String()
		nid := convertID(remoteAdd)
		if nid == id {
			svs = removeElement(svs, i)
			fmt.Println("Removed node!", n.RemoteAddr().String())
			return svs
		}
	}
	return svs
}

// removeElement removes an element of a list of tcp connection
func removeElement(s []*net.TCPConn, idx int) []*net.TCPConn {
	s[idx] = s[len(s)-1]
	return s[:len(s)-1]
}
