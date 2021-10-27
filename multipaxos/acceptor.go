package multipaxos

// Acceptor represents an acceptor as defined by the Multi-Paxos algorithm.
type Acceptor struct {
	id        int
	rnd       Round
	vrand     Round
	vval      Value
	Slot      SlotID
	promSlots []*PromiseSlot

	promiseOut chan<- Promise
	learnOut   chan<- Learn
	acceptIn   chan Accept
	prepareIn  chan Prepare
	stop       chan struct{}
}

// NewAcceptor returns a new Multi-Paxos acceptor.
// It takes the following arguments:
//
// id: The id of the node running this instance of a Paxos acceptor.
//
// promiseOut: A send only channel used to send promises to other nodes.
//
// learnOut: A send only channel used to send learns to other nodes.
func NewAcceptor(id int, promiseOut chan<- Promise, learnOut chan<- Learn) *Acceptor {
	return &Acceptor{
		id:    id,
		rnd:   NoRound,
		vrand: NoRound,
		vval:  Value{},
		Slot:  SlotID(NoRound),

		promiseOut: promiseOut,
		learnOut:   learnOut,

		acceptIn:  make(chan Accept, 8),
		prepareIn: make(chan Prepare, 8),
		stop:      make(chan struct{}),
	}
}

// Start starts a's main run loop as a separate goroutine. The main run loop
// handles incoming prepare and accept messages.
func (a *Acceptor) Start() {
	go func() {
		for {
			select {
			case prep := <-a.prepareIn:
				prm, output := a.handlePrepare(prep)
				if output {
					a.promiseOut <- prm
				}
			case acc := <-a.acceptIn:
				lrn, output := a.handleAccept(acc)
				if output {
					a.learnOut <- lrn
				}
			case <-a.stop:
				return
			}
		}
	}()
}

// Stop stops a's main run loop.
func (a *Acceptor) Stop() {
	a.stop <- struct{}{}
}

// DeliverPrepare delivers prepare prp to acceptor a.
func (a *Acceptor) DeliverPrepare(prp Prepare) {
	a.prepareIn <- prp
}

// DeliverAccept delivers accept acc to acceptor a.
func (a *Acceptor) DeliverAccept(acc Accept) {
	a.acceptIn <- acc
}

// Internal: handlePrepare processes prepare prp according to the Multi-Paxos
// algorithm. If handling the prepare results in acceptor a emitting a
// corresponding promise, then output will be true and prm contain the promise.
// If handlePrepare returns false as output, then prm will be a zero-valued
// struct.
func (a *Acceptor) handlePrepare(prp Prepare) (prm Promise, output bool) {
	if prp.Crnd > a.rnd {
		if a.Slot != prp.Slot {
			a.promSlots = a.filterSlots(a.Slot, a.promSlots)
		}
		a.promSlots = a.maxVrandDuplicates(a.promSlots)
		a.changeHistory()
		a.rnd = prp.Crnd
		a.Slot = prp.Slot
		return Promise{
			To:    prp.From,
			From:  a.id,
			Rnd:   a.rnd,
			Slots: a.promSlots,
		}, true
	}
	return Promise{}, false
}

// Internal: handleAccept processes accept acc according to the Multi-Paxos
// algorithm. If handling the accept results in acceptor a emitting a
// corresponding learn, then output will be true and lrn contain the learn.  If
// handleAccept returns false as output, then lrn will be a zero-valued struct.
func (a *Acceptor) handleAccept(acc Accept) (lrn Learn, output bool) {

	if acc.Rnd >= a.rnd && acc.Rnd >= a.vrand {
		a.rnd, a.vrand, a.vval = acc.Rnd, acc.Rnd, acc.Val
		a.promSlots = append(a.promSlots, &PromiseSlot{
			ID:   acc.Slot,
			Vrnd: a.vrand,
			Vval: a.vval,
		})
		return Learn{From: a.id, Slot: acc.Slot, Rnd: acc.Rnd, Val: acc.Val}, true
	}
	return Learn{}, false
}

// filter slots remeoves slots that has same id as the first promissed slot
func (a *Acceptor) filterSlots(currSlot SlotID, slots []*PromiseSlot) []*PromiseSlot {
	removed := false
	for i := 0; i < len(slots); i++ {
		if removed {
			i--
		}
		if currSlot == slots[i].ID {
			slots = removeSlot(slots, i)
			removed = true
			continue
		}
		removed = false
	}
	return slots
}

// changeHistory changes the history in a way that the RC request is unset
func (a *Acceptor) changeHistory() {
	for i, p := range a.promSlots {
		if p.Vval.RC.Reconfig {
			p.Vval.RC.Reconfig = false
			a.promSlots[i] = p
		}
	}
}

// maxVrandDuplicates removes a slot with the same id but lower round number
func (a *Acceptor) maxVrandDuplicates(slots []*PromiseSlot) []*PromiseSlot {
	for i, s1 := range slots {
		for j, s2 := range slots {
			if i != j {
				if s1.Vrnd < s2.Vrnd && s1.ID == s2.ID {
					slots = removeSlot(slots, i)
				}
			}
		}
	}
	return slots
}

// removeSlot removes a slot from the list of slots
func removeSlot(slots []*PromiseSlot, index int) []*PromiseSlot {
	refreshedSlots := append(slots[:index], slots[index+1:]...)
	return refreshedSlots
}
