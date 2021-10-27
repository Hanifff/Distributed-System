package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"sync"
)

/*
var (
	help = flag.Bool(
		"help",
		false,
		"Show usage help",
	)
	paxos = flag.Bool(
		"paxos",
		false,
		"Start paxos server if true; otherwise start the paxos client",
	)
	server = flag.String(
		"server",
		"localhost:1234-3",
		"Start server with a specisified port nummber!",
	)
	nodeID = flag.String(
		"nodeID",
		":42220-3",
		"The ID of current node and Port number to run the client",
	)
)

func usage() {
	fmt.Fprintf(os.Stderr, "Usage: %s [OPTIONS]\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "\nOptions:\n")
	flag.PrintDefaults()
}

// This main function used when run on local host
func main() {
	flag.Usage = usage
	flag.Parse()
	if *help {
		flag.Usage()
		os.Exit(0)
	}
	if *paxos {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		wg := sync.WaitGroup{}
		wg.Add(1)
		go paxosNode(*server, ctx, &wg)
		stopPaxos := make(chan os.Signal, 1)
		signal.Notify(stopPaxos, os.Interrupt)
		stop := <-stopPaxos
		fmt.Printf("\nServer stopping on %s signal\n", stop)
		cancel() // cancell all go routines
		fmt.Println("Main: all goroutines have finished!")
		wg.Wait()

	} else {
		node := strings.Split(*nodeID, "-")
		paxosClient(node[0], converter(node[1]))
	}
}
*/

// This main function is used when running on docker contianers...
func main() {
	arg, err := os.Hostname()
	if err != nil {
		fmt.Println("Could not find the ip address!", err)
	}
	args := strings.Split(arg, ",")
	if len(args) == 1 {
		wg := sync.WaitGroup{}
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		wg.Add(1)
		paxosNode(args[0], ctx, &wg)
		stopPaxos := make(chan os.Signal, 1)
		signal.Notify(stopPaxos, os.Interrupt)
		stop := <-stopPaxos
		fmt.Printf("\nServer stopping on %s signal\n", stop)
		cancel() // cancell all go routines
		fmt.Println("Main: all goroutines have finished!")
		wg.Wait()
	} else {
		nrOfSvs := converter(args[0])
		paxosClient(args[1], nrOfSvs)
	}
}

// run command for paxos servers:
// docker build -t paxos_server -f bank_app/Dockerfile .
// docker run -h 172.28.1.2:1234-3 -it --name paxos1 --net paxos-lab5 --ip 172.28.1.2 --expose=1234 --rm paxos_server
// docker run -h 172.28.1.3:1235-3 -it --name paxos2 --net paxos-lab5 --ip 172.28.1.3 --expose=1235 --rm paxos_server
// docker run -h 172.28.1.4:1236-3 -it --name paxos3 --net paxos-lab5 --ip 172.28.1.4 --expose=1236 --rm paxos_server

// and for clients:
// docker run -h 5,:42227 -it --name cli1 --net paxos-lab5 --ip 172.28.1.9 --expose=42227 --rm paxos_server
// docker run -h 3,:42237 -it --name cli2 --net paxos-lab5 --ip 172.28.1.10 --expose=42237 --rm paxos_server
