package multipaxos

import (
	"container/list"
	"fmt"
	"sort"
	"sync"
	"time"

	"bank_app/leaderdetector"
)

// Proposer represents a proposer as defined by the Multi-Paxos algorithm.
type Proposer struct {
	id     int
	quorum int
	n      int

	crnd     Round
	adu      SlotID
	nextSlot SlotID

	promises []*Promise
	//promiseCount int

	phaseOneDone           bool
	phaseOneProgressTicker *time.Ticker

	acceptsOut *list.List
	requestsIn *list.List

	ld     leaderdetector.LeaderDetector
	leader int

	prepareOut chan<- Prepare
	acceptOut  chan<- Accept
	promiseIn  chan Promise
	cvalIn     chan Value

	incrNrNodes chan int

	incDcd chan struct{}
	stop   chan struct{}
}

// NewProposer returns a new Multi-Paxos proposer. It takes the following
// arguments:
//
// id: The id of the node running this instance of a Paxos proposer.
//
// nrOfNodes: The total number of Paxos nodes.
//
// adu: all-decided-up-to. The initial id of the highest _consecutive_ slot
// that has been decided. Should normally be set to -1 initially, but for
// testing purposes it is passed in the constructor.
//
// ld: A leader detector implementing the detector.LeaderDetector interface.
//
// prepareOut: A send only channel used to send prepares to other nodes.
//
// The proposer's internal crnd field should initially be set to the value of
// its id.
func NewProposer(id, nrOfNodes, adu int, ld leaderdetector.LeaderDetector, prepareOut chan<- Prepare, acceptOut chan<- Accept) *Proposer {
	return &Proposer{
		id:     id,
		quorum: (nrOfNodes / 2) + 1,
		n:      nrOfNodes,

		crnd:     Round(id),
		adu:      SlotID(adu),
		nextSlot: 0,

		promises: make([]*Promise, nrOfNodes),

		phaseOneProgressTicker: time.NewTicker(time.Second),

		acceptsOut: list.New(),
		requestsIn: list.New(),

		ld:     ld,
		leader: ld.Leader(),

		prepareOut: prepareOut,
		acceptOut:  acceptOut,
		promiseIn:  make(chan Promise, 8),
		cvalIn:     make(chan Value, 8),

		incrNrNodes: make(chan int),

		incDcd: make(chan struct{}),
		stop:   make(chan struct{}),
	}
}

// Start starts p's main run loop as a separate goroutine.
func (p *Proposer) Start() {
	trustMsgs := p.ld.Subscribe()
	go func() {
		for {
			select {
			case prm := <-p.promiseIn:
				accepts, output := p.handlePromise(prm)
				if !output {
					continue
				}
				p.nextSlot = p.adu + 1
				p.acceptsOut.Init()
				p.phaseOneDone = true
				for _, acc := range accepts {
					p.acceptsOut.PushBack(acc)
				}
				p.sendAccept()
				fmt.Println("**Phase one is set to: ", p.phaseOneDone)
			case cval := <-p.cvalIn:
				if p.id != p.leader {
					continue
				}
				p.requestsIn.PushBack(cval)
				if !p.phaseOneDone {
					continue
				}
				p.sendAccept()
			case <-p.incDcd:
				p.adu++
				//fmt.Println("ds.adu :", p.adu)
				if p.id != p.leader {
					continue
				}
				if !p.phaseOneDone {
					continue
				}
				p.sendAccept()
			case <-p.phaseOneProgressTicker.C:
				if p.id == p.leader && !p.phaseOneDone {
					p.startPhaseOne()
				}
			case leader := <-trustMsgs:
				p.leader = leader
				if leader == p.id {
					p.startPhaseOne()
				}
			case incTo := <-p.incrNrNodes:
				p.n = incTo
				p.quorum = (p.n / 2) + 1
			case <-p.stop:
				return
			}
		}
	}()
}

// Stop stops p's main run loop.
func (p *Proposer) Stop() {
	p.stop <- struct{}{}
}

// IncreaseNrOfNodes increases the number of nodes while reconfiguering
func (p *Proposer) IncreaseNrOfNodes(incTo int) {
	if incTo != p.n {
		p.incrNrNodes <- incTo
	}
}

// DeliverPromise delivers promise prm to proposer p.
func (p *Proposer) DeliverPromise(prm Promise) {
	p.promiseIn <- prm
}

// DeliverClientValue delivers client value cval from to proposer p.
func (p *Proposer) DeliverClientValue(cval Value) {
	p.cvalIn <- cval
}

// IncrementAllDecidedUpTo increments the Proposer's adu variable by one.
func (p *Proposer) IncrementAllDecidedUpTo() {
	p.incDcd <- struct{}{}
}

// Internal: handlePromise processes promise prm according to the Multi-Paxos
// algorithm. If handling the promise results in proposer p emitting a
// corresponding accept slice, then output will be true and accs contain the
// accept messages. If handlePromise returns false as output, then accs will be
// a nil slice.
func (p *Proposer) handlePromise(prm Promise) (accs []Accept, output bool) {
	alreadyRec := p.continasPromise(prm)
	if alreadyRec || prm.Rnd != p.crnd {
		return nil, false
	}
	p.promises = append(p.promises, &prm)
	if p.isQurom() {
		var accepts []Accept
		for i := range p.promises {
			if p.promises[i] != nil {
				for j := range p.promises[i].Slots {
					currID := p.promises[i].Slots[j].ID
					ignore := p.ignoreSlot(currID) // *
					if !ignore {
						if !p.continasSlot(accepts, currID) {
							accepts = append(accepts, Accept{
								From: p.id,
								Slot: currID,
								Rnd:  p.crnd,
								Val:  p.promises[i].Slots[j].Vval,
							})
						}
						if p.continasSlot(accepts, currID) {
							slot, maxed := p.maxVrand(p.promises[i].Slots[j])
							if maxed {
								accepts = p.replaceSlot(slot, accepts)
							}
						}
					}
				}
			}
		}
		accepts = p.findGap(accepts)
		if accepts == nil {
			accepts = make([]Accept, 0)
		}
		return accepts, true
	}
	return nil, false
}

// Internal: increaseCrnd increases proposer p's crnd field by the total number
// of Paxos nodes.
func (p *Proposer) increaseCrnd() {
	p.crnd = p.crnd + Round(p.n)
}

// Internal: startPhaseOne resets all Phase One data, increases the Proposer's
// crnd and sends a new Prepare with Slot as the current adu.
func (p *Proposer) startPhaseOne() {
	p.phaseOneDone = false
	p.promises = make([]*Promise, p.n)
	fmt.Println("number of node in handling promisses : ", p.n)
	p.increaseCrnd()
	p.prepareOut <- Prepare{From: p.id, Slot: p.adu, Crnd: p.crnd}
}

// Internal: sendAccept sends an accept from either the accept out queue
// (generated after Phase One) if not empty, or, it generates an accept using a
// value from the client request queue if not empty. It does not send an accept
// if the previous slot has not been decided yet.
func (p *Proposer) sendAccept() {
	const alpha = 1
	if !(p.nextSlot <= p.adu+alpha) {
		// We must wait for the next slot to be decided before we can
		// send an accept.
		//
		// For Lab 6: Alpha has a value of one here. If you later
		// implement pipelining then alpha should be extracted to a
		// proposer variable (alpha) and this function should have an
		// outer for loop.
		fmt.Printf("I (Proposer) am stuck. My nextslot is : %d, and adu+1 is: %d", p.nextSlot, p.adu+alpha)
		return
	}

	// Pri 1: If bounded by any accepts from Phase One -> send previously
	// generated accept and return.
	if p.acceptsOut.Len() > 0 {
		acc := p.acceptsOut.Front().Value.(Accept)
		p.acceptsOut.Remove(p.acceptsOut.Front())
		p.acceptOut <- acc
		p.nextSlot++
		return
	}

	// Pri 2: If any client request in queue -> generate and send
	// accept.
	if p.requestsIn.Len() > 0 {
		cval := p.requestsIn.Front().Value.(Value)
		p.requestsIn.Remove(p.requestsIn.Front())
		acc := Accept{
			From: p.id,
			Slot: p.nextSlot,
			Rnd:  p.crnd,
			Val:  cval,
		}
		p.nextSlot++
		p.acceptOut <- acc
	}
}

// continasPromise checks if the proposer has recieved a promise with
// the same rnd from the same node previously.
func (p *Proposer) continasPromise(prm Promise) bool {
	for i := range p.promises {
		if p.promises[i] != nil {
			if p.promises[i].From == prm.From &&
				p.promises[i].Rnd == prm.Rnd {
				return true
			}
		}
	}
	return false
}

// continasSlot checks if the accepted list has already inserted a similar slot.
func (p *Proposer) continasSlot(acc []Accept, id SlotID) bool {
	for i := range acc {
		accepted := acc[i]
		if accepted.Slot == id {
			return true
		}
	}
	return false
}

// sortAccepts sorts accepts list in ascending order based on the slots ids.
func (p *Proposer) sortAccepts(acc []Accept) []Accept {
	sort.SliceStable(acc, func(i, j int) bool {
		return acc[i].Slot < acc[j].Slot
	})
	return acc
}

// findGap finds gaps between slot ids.
// If there is a gap, then it will add a accept with a no-op value.
func (p *Proposer) findGap(accepts []Accept) []Accept {
	accepts = p.sortAccepts(accepts)
	if len(accepts) > 0 {
		missingSlot := accepts[0].Slot
		for i, a := range accepts {
			if i == 0 {
				continue
			}
			if a.Slot != missingSlot+1 {
				accepts = append(accepts, Accept{
					From: p.id,
					Slot: missingSlot + 1,
					Rnd:  p.crnd,
					Val:  Value{Noop: true},
				})
				accepts = p.sortAccepts(accepts)
			}
			missingSlot++
		}
		return accepts
	}
	return accepts
}

// isQurom checks the number of promises in order to find if there is a
// majority of recieved unique promises.
func (p *Proposer) isQurom() bool {
	count := 0
	for i := range p.promises {
		if p.promises[i] != nil {
			count++
		}
	}
	return count >= p.quorum
}

// maxVrand finds a maximum vrand from set of promises and returns if there is any.
func (p *Proposer) maxVrand(slot *PromiseSlot) (*PromiseSlot, bool) {
	maxSlot := slot
	maxVrnd := slot.Vrnd
	updated := false
	for i := range p.promises {
		if p.promises[i] != nil {
			for j := range p.promises[i].Slots {
				if maxVrnd < p.promises[i].Slots[j].Vrnd &&
					p.promises[i].Slots[j].ID == slot.ID {
					maxVrnd = p.promises[i].Slots[j].Vrnd
					maxSlot = p.promises[i].Slots[j]
					updated = true
				}
			}
		}
	}
	return maxSlot, updated
}

// replaceSlot replaces a slot that have a higher vrand with the other slots that has the same ID.
func (p *Proposer) replaceSlot(slot *PromiseSlot, acc []Accept) []Accept {
	for i, a := range acc {
		if a.Slot == slot.ID {
			acc[i] = Accept{From: p.id, Slot: slot.ID, Rnd: p.crnd, Val: slot.Vval}
		}
	}
	return acc
}

// ignoreSlot filters slots that apears less than quorum while have vrand less than the adu.
// see test 9- action 2
func (p *Proposer) ignoreSlot(currentSlotID SlotID) bool {
	filter := make(map[SlotID]int)
	lockFilter := make(map[SlotID]*sync.Mutex)

	for i := range p.promises {
		if p.promises[i] != nil {
			for j := range p.promises[i].Slots {
				if _, ok := lockFilter[p.promises[i].Slots[j].ID]; !ok {
					lockFilter[p.promises[i].Slots[j].ID] = &sync.Mutex{}
					lockFilter[p.promises[i].Slots[j].ID].Lock()
					filter[p.promises[i].Slots[j].ID] = 1
					lockFilter[p.promises[i].Slots[j].ID].Unlock()
				} else {
					lockFilter[p.promises[i].Slots[j].ID].Lock()
					filter[p.promises[i].Slots[j].ID]++
					lockFilter[p.promises[i].Slots[j].ID].Unlock()
				}
			}
		}
	}
	apeared := filter[currentSlotID]
	if apeared < p.quorum {
		for i := range p.promises {
			if p.promises[i] != nil {
				for j := range p.promises[i].Slots {
					if p.promises[i].Slots[j].ID == currentSlotID &&
						p.promises[i].Slots[j].Vrnd < Round(p.adu) {
						return true
					}
				}
			}
		}
	}
	return false
}
