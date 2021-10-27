package main

import (
	"encoding/gob"
	"fmt"
	"net"
	"strings"
	"time"

	"dnb/bank_app/bank"
	"dnb/bank_app/failuredetector"
	"dnb/bank_app/leaderdetector"
	"dnb/bank_app/multipaxos"
)

// PaxosMsg is a instance of paxos messages
type PaxosMsg struct {
	Mod string
	Msg interface{}
}

// DistributedServer is a instance of MultiPaxos
type DistributedServer struct {
	id        int
	delta     time.Duration
	resp      chan failuredetector.Heartbeat
	serverIPs []string
	nodes     []int
	servers   []*net.TCPConn
	clients   []*net.TCPConn
	prepare   chan multipaxos.Prepare
	acceptOut chan multipaxos.Accept
	adu       int

	promiseOut chan multipaxos.Promise
	learnOut   chan multipaxos.Learn

	decidedOut     chan multipaxos.DecidedValue
	bufferedValues []*multipaxos.DecidedValue
	hbTicker       *time.Ticker
	accounts       map[int]*bank.Account

	nrOfServers int
	processHB   bool

	reconfigProcces bool
}

// NewDistributedServer create new instainces of a MultiPaxos server
func NewDistributedServer(id int, nrOfPaxosNodes int) *DistributedServer {
	serverIPs := []string{"localhost:1234", "localhost:1235", "localhost:1236"}
	/* , "localhost:1237",
	"localhost:1238", "localhost:1239", "localhost:1240" */
	//serverIPs := []string{"172.28.1.2:1234", "172.28.1.3:1235", "172.28.1.4:1236",
	//	"172.28.1.5:1237", "172.28.1.6:1238", "172.28.1.7:1239", "172.28.1.8:1240"}
	//serverIPs := []string{"152.94.1.110:1234", "152.94.1.119:1235", "152.94.1.115:1236"} //pi11,pi20, pi16
	nodes := []int{1234, 1235, 1236} /* , 1237, 1238, 1239, 1240 */
	return &DistributedServer{
		resp:      make(chan failuredetector.Heartbeat),
		id:        id,
		serverIPs: serverIPs,
		servers:   make([]*net.TCPConn, nrOfPaxosNodes),
		nodes:     nodes[:nrOfPaxosNodes],

		prepare:   make(chan multipaxos.Prepare),
		acceptOut: make(chan multipaxos.Accept),
		adu:       -1,

		promiseOut: make(chan multipaxos.Promise),
		learnOut:   make(chan multipaxos.Learn),
		decidedOut: make(chan multipaxos.DecidedValue),
		hbTicker:   time.NewTicker(100 * time.Millisecond),

		accounts:        make(map[int]*bank.Account),
		nrOfServers:     nrOfPaxosNodes,
		processHB:       false,
		reconfigProcces: false,
	}
}

// FdInstance intilizes a failuredetector instance
func (ds *DistributedServer) FdInstance(ld *leaderdetector.MonLeaderDetector) *failuredetector.EvtFailureDetector {
	ds.delta = 5 * time.Second
	fdInstance := failuredetector.NewEvtFailureDetector(ds.id, ds.nodes[:ds.nrOfServers], ld, ds.delta, ds.resp)
	return fdInstance
}

// LdInstnace initiliazes a leaderdetector instance
func (ds *DistributedServer) LdInstnace() *leaderdetector.MonLeaderDetector {
	return leaderdetector.NewMonLeaderDetector(ds.nodes[:ds.nrOfServers])
}

// proposerIns intilizes a proposer instance
func (ds *DistributedServer) proposerIns(ld *leaderdetector.MonLeaderDetector) *multipaxos.Proposer {
	propIns := multipaxos.NewProposer(ds.id, ds.nrOfServers, ds.adu, ld, ds.prepare, ds.acceptOut)
	return propIns
}

// acceptorIns intilizes an acceptor instance
func (ds *DistributedServer) acceptorIns() *multipaxos.Acceptor {
	accIns := multipaxos.NewAcceptor(ds.id, ds.promiseOut, ds.learnOut)
	return accIns
}

// learnIns intilizes a learner instance
func (ds *DistributedServer) learnIns() *multipaxos.Learner {
	lrnIns := multipaxos.NewLearner(ds.id, ds.nrOfServers, ds.decidedOut)
	return lrnIns
}

// handleDecideValue handles a decided value.
// The method bufferes values not sorted and forward those in order.
func (ds *DistributedServer) handleDecideValue(dcdVal *multipaxos.DecidedValue, fd *failuredetector.EvtFailureDetector,
	ld *leaderdetector.MonLeaderDetector, proposer *multipaxos.Proposer, acceptor *multipaxos.Acceptor,
	learner *multipaxos.Learner) {
	stop := make(chan struct{})
	if dcdVal.Value.RC.Reconfig {
		fmt.Println("Decided! We reconfigure..", dcdVal)
		dcdVal.Value.RC.Reconfig = false
		ds.reconfigProcces = true

		ds.nrOfServers = dcdVal.Value.RC.NumberOfNodes
		// connection processes to the new node
		ds.nodes = append(ds.nodes, allNodeIDs[ds.nrOfServers-1])
		connectReconf()
		ds.notifyClientScaleUp()
		newNodeID := allNodeIDs[ds.nrOfServers-1]
		fd.AddNode(newNodeID)
		ld.AddNode(newNodeID)
		proposer.IncreaseNrOfNodes(ds.nrOfServers)
		learner.IncreaseNrOfNodes(ds.nrOfServers)
		proposer.IncrementAllDecidedUpTo()
		ds.adu++
		fmt.Println("ds.adu :", ds.adu)
		return
	} else {
		fmt.Println("Handling decided value: ", dcdVal)
		if dcdVal.SlotID > multipaxos.SlotID(ds.adu)+1 {
			fmt.Println("buffering values..")
			ds.bufferedValues = append(ds.bufferedValues, dcdVal)
			return
		}
		if !dcdVal.Value.Noop {
			txnRes := ds.handleAccount(dcdVal.Value)
			response := multipaxos.Response{
				ClientID:  dcdVal.Value.ClientID,
				ClientSeq: dcdVal.Value.ClientSeq,
				TxnRes:    txnRes,
				NrOfNodes: ds.nrOfServers,
				ScaledUp:  false,
			}
			forwadrdClientResp(&response, stop)
		}
		if !ds.reconfigProcces {
			proposer.IncrementAllDecidedUpTo()
			ds.adu++
		}
		fmt.Println("ds.adu :", ds.adu)
		for _, dv := range ds.bufferedValues {
			ds.handleDecideValue(dv, fd, ld, proposer, acceptor, learner)
		}
		// goes in loop in case a buffered value
		return
	}
}

// handleAccount handles an account after a transaction request recieved.
// The method will store a new account if it is not stored.
// The result of transaction will be returned.
func (ds *DistributedServer) handleAccount(val multipaxos.Value) bank.TransactionResult {
	if _, exist := ds.accounts[val.AccountNum]; !exist {
		ds.accounts[val.AccountNum] = &bank.Account{
			Number:  val.AccountNum,
			Balance: 0,
		}
	}
	txnRes := ds.accounts[val.AccountNum].Process(val.Txn)
	return txnRes
}

// isClient checks if the new connection is a connection to a bank client or a paxos server
func (ds *DistributedServer) isClient(conn *net.TCPConn) bool {
	port := strings.Split(conn.RemoteAddr().String(), ":")[1]
	thisPort := converter(port)
	if thisPort >= 42220 && thisPort <= 42237 {
		return true
	}
	return false
}

func (ds *DistributedServer) notifyClientScaleUp() {
	stop := make(chan struct{})
	response := multipaxos.Response{
		ClientID:  "Reconfugration",
		NrOfNodes: ds.nrOfServers,
		TxnRes:    bank.TransactionResult{},
		ScaledUp:  true,
	}
	forwadrdClientResp(&response, stop)
}

// initGob initializes Paxos messages in gob
func (ds *DistributedServer) initGob() {
	gob.Register(failuredetector.Heartbeat{})
	gob.Register(multipaxos.Prepare{})
	gob.Register(multipaxos.Promise{})
	gob.Register(multipaxos.Accept{})
	gob.Register(multipaxos.Learn{})
	gob.Register(multipaxos.Value{})
	gob.Register(multipaxos.Response{})
	gob.Register(multipaxos.DecidedValue{})
	gob.Register(multipaxos.PromiseSlot{})
	gob.Register(PaxosMsg{})
}
