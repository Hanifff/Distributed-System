package multipaxos

import (
	"sync"
)

// Learner represents a learner as defined by the Multi-Paxos algorithm.
type Learner struct {
	// Add needed fields
	id        int
	nrOfNodes int

	recieved map[int][]*Learn
	decided  []Learn
	lockRcv  map[int]*sync.Mutex

	decidedOut chan<- DecidedValue
	learnIn    chan Learn
	stop       chan struct{}
	incrNodes  chan int

	rnd Round

	m sync.Mutex
}

// NewLearner returns a new Multi-Paxos learner. It takes the
// following arguments:
//
// id: The id of the node running this instance of a Paxos learner.
//
// nrOfNodes: The total number of Paxos nodes.
//
// decidedOut: A send only channel used to send values that has been learned,
// i.e. decided by the Paxos nodes.
func NewLearner(id int, nrOfNodes int, decidedOut chan<- DecidedValue) *Learner {

	return &Learner{
		id:        id,
		nrOfNodes: nrOfNodes,

		recieved: make(map[int][]*Learn),
		lockRcv:  make(map[int]*sync.Mutex),

		decidedOut: decidedOut,
		learnIn:    make(chan Learn, 20),
		incrNodes:  make(chan int),

		rnd:  NoRound,
		stop: make(chan struct{}),
	}
}

// Start starts l's main run loop as a separate goroutine. The main run loop
// handles incoming learn messages.
func (l *Learner) Start() {
	go func() {
		for {
			select {
			case lrn := <-l.learnIn:
				val, sid, output := l.handleLearn(lrn)
				if !output {
					continue
				}
				l.decidedOut <- DecidedValue{
					SlotID: sid,
					Value:  val,
				}
			case incr := <-l.incrNodes:
				l.nrOfNodes = incr
			case <-l.stop:
				return
			}
		}
	}()
}

// IncreaseNrOFNodes increases the number of nodes while reconfiguering
func (l *Learner) IncreaseNrOfNodes(recNrofNodes int) {
	if l.nrOfNodes != recNrofNodes {
		l.incrNodes <- recNrofNodes
	}
}

// Stop stops l's main run loop.
func (l *Learner) Stop() {
	l.stop <- struct{}{}
}

// DeliverLearn delivers learn lrn to learner l.
func (l *Learner) DeliverLearn(lrn Learn) {
	l.learnIn <- lrn
}

// Internal: handleLearn processes learn lrn according to the Multi-Paxos
// algorithm. If handling the learn results in learner l emitting a
// corresponding decided value, then output will be true, sid the id for the
// slot that was decided and val contain the decided value. If handleLearn
// returns false as output, then val and sid will have their zero value.
func (l *Learner) handleLearn(learn Learn) (val Value, sid SlotID, output bool) {

	if l.rnd != learn.Rnd {
		// on leader change empty decided value
		l.decided = []Learn{}
	}
	l.rnd = learn.Rnd
	l.m.Lock()
	l.recieved[learn.From] = append(l.recieved[learn.From], &learn)
	countDups := l.majorities(&learn)
	l.m.Unlock()
	if l.aldreadyDecided(learn.Slot) {
		return Value{}, 0, false
	}
	majority := (l.nrOfNodes / 2) + 1
	if countDups >= int(majority) {
		l.decided = append(l.decided, learn)
		return learn.Val, learn.Slot, true
	}
	return Value{}, 0, false
}

// majorities counts the number of occurrences of a learned value.
func (l *Learner) majorities(learn *Learn) int {
	countDups := 1
	// count the number of duplicates
	for nodeID, existedLearn := range l.recieved {
		if nodeID != learn.From {
			for _, e := range existedLearn {
				if learn.Rnd == e.Rnd &&
					learn.Val == e.Val &&
					learn.Slot == e.Slot {
					countDups++
				}
			}
		}
	}
	return countDups
}

// aldreadyDecided checks if a value is already decided
func (l *Learner) aldreadyDecided(slot SlotID) bool {
	for _, lrn := range l.decided {
		if lrn.Slot == slot {
			return true
		}
	}
	return false
}
