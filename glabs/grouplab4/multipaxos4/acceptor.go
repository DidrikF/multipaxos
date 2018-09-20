// +build !solution

package multipaxos4

import "sort"

// Acceptor represents an acceptor as defined by the Multi-Paxos algorithm.
type Acceptor struct {
	// TODO(student)
	id int

	rnd Round

	acceptSlots []PromiseSlot

	promiseOut chan<- Promise
	learnOut   chan<- Learn
	prepareIn  chan Prepare
	acceptIn   chan Accept

	stop chan struct{}
}

// NewAcceptor returns a new single-decree Paxos acceptor.
// It takes the following arguments:
//
// id: The id of the node running this instance of a Paxos acceptor.
//
// promiseOut: A send only channel used to send promises to other nodes.
//
// learnOut: A send only channel used to send learns to other nodes.
func NewAcceptor(id int, promiseOut chan<- Promise, learnOut chan<- Learn) *Acceptor {
	return &Acceptor{
		id: id,

		rnd: NoRound,

		acceptSlots: []PromiseSlot{},

		promiseOut: promiseOut,
		learnOut:   learnOut,
		prepareIn:  make(chan Prepare, 8),
		acceptIn:   make(chan Accept, 8),

		stop: make(chan struct{}),
	}
}

// Start starts a's main run loop as a separate goroutine. The main run loop
// handles incoming prepare and accept messages.
func (a *Acceptor) Start() {
	go func() {
		for {
			// TODO(student)
			select {
			case prp := <-a.prepareIn:
				promise, ok := a.handlePrepare(prp)
				if ok == true {
					a.promiseOut <- promise
				}
				//////fmt.Printf("Acceptor %v: promise ok? %v", a.id, ok)
			case acc := <-a.acceptIn:
				if learn, ok := a.handleAccept(acc); ok == true {
					a.learnOut <- learn
				}
			case <-a.stop:
				break
			}
		}
	}()
}

// Stop stops a's main run loop.
func (a *Acceptor) Stop() {
	// TODO(student)
	a.stop <- struct{}{}
}

// DeliverPrepare delivers prepare prp to acceptor a.
func (a *Acceptor) DeliverPrepare(prp Prepare) {
	// TODO(student)
	a.prepareIn <- prp
}

// DeliverAccept delivers accept acc to acceptor a.
func (a *Acceptor) DeliverAccept(acc Accept) {
	// TODO(student)
	a.acceptIn <- acc
}

// Internal: handlePrepare processes prepare prp according to the Multi-Paxos
// algorithm. If handling the prepare results in acceptor a emitting a
// corresponding promise, then output will be true and prm contain the promise.
// If handlePrepare returns false as output, then prm will be a zero-valued
// struct.
func (a *Acceptor) handlePrepare(prp Prepare) (prm Promise, output bool) {
	// TODO(student)
	if prp.Crnd >= a.rnd {
		var slotresponse []PromiseSlot
		for _, slot := range a.acceptSlots {
			if slot.ID >= prp.Slot {
				slotresponse = append(slotresponse, slot)
			}
		}
		a.rnd = prp.Crnd
		return Promise{To: prp.From, From: a.id, Rnd: a.rnd, Slots: slotresponse}, true
	}
	return prm, false
}

// Internal: handleAccept processes accept acc according to the Multi-Paxos
// algorithm. If handling the accept results in acceptor a emitting a
// corresponding learn, then output will be true and lrn contain the learn.  If
// handleAccept returns false as output, then lrn will be a zero-valued struct.
func (a *Acceptor) handleAccept(acc Accept) (lrn Learn, output bool) {
	// TODO(student)
	if acc.Rnd >= a.rnd {
		a.rnd = acc.Rnd
		for i, slot := range a.acceptSlots {
			if slot.ID == acc.Slot {
				slot.Vrnd = acc.Rnd
				slot.Vval = acc.Val
				a.acceptSlots = append(a.acceptSlots[:i], append([]PromiseSlot{slot}, a.acceptSlots[i+1:]...)...) //fancy func
				return Learn{From: a.id, Slot: slot.ID, Rnd: slot.Vrnd, Val: slot.Vval}, true
			}
		}
		//sort by slotID
		a.acceptSlots = append(a.acceptSlots, PromiseSlot{ID: acc.Slot, Vrnd: acc.Rnd, Vval: acc.Val})
		//a.acceptSlots = MergeSort(a.acceptSlots)

		sort.SliceStable(a.acceptSlots, func(i int, j int) bool {
			return a.acceptSlots[i].ID < a.acceptSlots[j].ID
		})

		return Learn{From: a.id, Slot: acc.Slot, Rnd: acc.Rnd, Val: acc.Val}, true
	}
	return lrn, false
}

//OBS: To create and append the correct slots (if any) to the slice, an acceptor (or proposer) need to keep track of the highest seen slot
//it has sent an accept for. This can for example be done by maintaining a maxSlot variable of type SlotID.
