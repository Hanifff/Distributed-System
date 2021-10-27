package failuredetector

import (
	"sync"
	"time"
)

// EvtFailureDetector represents a Eventually Perfect Failure Detector as
// described at page 53 in:
// Christian Cachin, Rachid Guerraoui, and Luís Rodrigues: "Introduction to
// Reliable and Secure Distributed Programming" Springer, 2nd edition, 2011.
type EvtFailureDetector struct {
	id        int          // the id of this node
	nodeIDs   []int        // node ids for every node in cluster
	alive     map[int]bool // map of node ids considered alive
	lockAlive map[int]*sync.Mutex

	suspected map[int]bool // map of node ids  considered suspected
	lockSus   map[int]*sync.Mutex

	sr SuspectRestorer // Provided SuspectRestorer implementation

	delay         time.Duration // the current delay for the timeout procedure
	delta         time.Duration // the delta value to be used when increasing delay
	timeoutSignal *time.Ticker  // the timeout procedure ticker

	hbSend chan<- Heartbeat // channel for sending outgoing heartbeat messages
	hbIn   chan Heartbeat   // channel for receiving incoming heartbeat messages
	stop   chan struct{}    // channel for signaling a stop request to the main run loop

	increseNodes chan int

	testingHook func() // DO NOT REMOVE THIS LINE. A no-op when not testing.
	started     bool

	m sync.Mutex
}

// NewEvtFailureDetector returns a new Eventual Failure Detector. It takes the
// following arguments:
//
// id: The id of the node running this instance of the failure detector.
//
// nodeIDs: A list of ids for every node in the cluster (including the node
// running this instance of the failure detector).
//
// ld: A leader detector implementing the SuspectRestorer interface.
//
// delta: The initial value for the timeout interval. Also the value to be used
// when increasing delay.
//
// hbSend: A send only channel used to send heartbeats to other nodes.
func NewEvtFailureDetector(id int, nodeIDs []int, sr SuspectRestorer, delta time.Duration, hbSend chan<- Heartbeat) *EvtFailureDetector {
	alive := make(map[int]bool)
	lockAlive := make(map[int]*sync.Mutex)

	suspected := make(map[int]bool)
	lockSus := make(map[int]*sync.Mutex)
	for n := range nodeIDs {
		alive[n] = true
	}
	return &EvtFailureDetector{
		id:        id,
		nodeIDs:   nodeIDs,
		alive:     alive,
		lockAlive: lockAlive,
		suspected: suspected,
		lockSus:   lockSus,
		sr:        sr,

		delay: delta,
		delta: delta,

		hbSend:  hbSend,
		hbIn:    make(chan Heartbeat, 8),
		stop:    make(chan struct{}),
		started: false,

		increseNodes: make(chan int),

		testingHook: func() {}, // DO NOT REMOVE THIS LINE. A no-op when not testing.

	}
}

// Start starts e's main run loop as a separate goroutine. The main run loop
// handles incoming heartbeat requests and responses. The loop also trigger e's
// timeout procedure at an interval corresponding to e's internal delay
// duration variable.
func (e *EvtFailureDetector) Start() {
	e.started = true
	e.timeoutSignal = time.NewTicker(e.delay)
	go func() {
		for {
			var msg Heartbeat
			e.testingHook() // DO NOT REMOVE THIS LINE. A no-op when not testing.
			select {
			case msg = <-e.hbIn:
				if msg.Request {
					hbReply := makeHeratbeat(e.id, msg.From, false)
					e.hbSend <- hbReply
				} else {
					e.setAlive(msg.From)
				}
			case <-e.timeoutSignal.C:
				e.timeout()
			case <-e.stop:
				e.started = false
				return
			}
		}
	}()
}

// DeliverHeartbeat delivers heartbeat hb to failure detector e.
func (e *EvtFailureDetector) DeliverHeartbeat(hb Heartbeat) {
	e.hbIn <- hb
}

// Started checks if the start method is called
func (e *EvtFailureDetector) Started() bool {
	return e.started
}

// Stop stops e's main run loop.
func (e *EvtFailureDetector) Stop() {
	e.stop <- struct{}{}
}

// Internal: timeout runs e's timeout procedure.
func (e *EvtFailureDetector) timeout() {
	for p := range e.alive {
		if _, ok := e.suspected[p]; ok {
			e.delay += e.delta
			e.timeoutSignal = time.NewTicker(e.delay)
			break
		}
	}
	for _, p := range e.nodeIDs {
		if !e.alive[p] && !e.suspected[p] {
			if _, exist := e.lockSus[p]; !exist {
				e.lockSus[p] = &sync.Mutex{}
				e.lockSus[p].Lock()
				e.suspected[p] = true
				e.sr.Suspect(p)
				e.lockSus[p].Unlock()
			} else {
				e.lockSus[p].Lock()
				e.suspected[p] = true
				e.sr.Suspect(p)
				e.lockSus[p].Unlock()
			}
		} else if e.alive[p] && e.suspected[p] {
			delete(e.suspected, p)
			delete(e.lockSus, p)
			e.sr.Restore(p)
		}
		hbRequst := makeHeratbeat(e.id, p, true)
		e.hbSend <- hbRequst
	}
	e.alive = make(map[int]bool)
	e.lockAlive = make(map[int]*sync.Mutex)
}

// AddNode notifies the failuredetector to scale up its set of nodes
func (e *EvtFailureDetector) AddNode(id int) {
	if !e.continasNode(id) {
		//e.increseNodes <- id
		e.nodeIDs = append(e.nodeIDs, id)
		e.setAlive(id)
	}
}

// continasNode checks if a node is presented in the list of nodes
func (e *EvtFailureDetector) continasNode(id int) bool {
	for _, n := range e.nodeIDs {
		if n == id {
			return true
		}
	}
	return false
}

// setAlive sets a node to alive set
func (e *EvtFailureDetector) setAlive(node int) {
	/* if _, exist := e.lockAlive[node]; exist {
		e.lockAlive[node].Lock()
		e.alive[node] = true
		e.lockAlive[node].Unlock()
	} else {
		e.lockAlive[node] = &sync.Mutex{}
		e.lockAlive[node].Lock()
		e.alive[node] = true
		e.lockAlive[node].Unlock()
	} */
	e.m.Lock()
	e.alive[node] = true
	e.m.Unlock()
}

// makeHeratbeat makes a Hearbeat object with the given paramaters.
func makeHeratbeat(fromID int, toID int, Request bool) Heartbeat {
	hb := Heartbeat{
		From:    fromID,
		To:      toID,
		Request: Request,
	}
	return hb
}
