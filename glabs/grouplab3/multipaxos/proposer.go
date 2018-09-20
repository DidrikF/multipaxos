// +build !solution

package multipaxos

import (
	"container/list"
	"fmt"
	"sort"
	"time"

	"github.com/uis-dat520-s18/glabs/grouplab1/detector"
)

// Proposer represents a proposer as defined by the Multi-Paxos algorithm.
type Proposer struct {
	Id     int
	quorum int
	n      int

	crnd     Round
	adu      SlotID
	nextSlot SlotID
	//maxSlot SlotID -> the highest slot it has sent an accept on, look at comment at the bottom of the Acceptor

	promises     []Promise //is this where promises are buffered until there is a quorum?
	promiseCount int       //is this used to check if there is a quorum?

	phaseOneDone           bool
	phaseOneProgressTicker *time.Ticker //if we dont get enough promises back, we risk hanging in phase 1, this allows us to increment the round number and attempt phase 1 again.
	phaseTwoProgressTicker *time.Ticker

	acceptsOut *list.List
	requestsIn *list.List

	ld     detector.LeaderDetector
	leader int

	prepareOut chan<- Prepare
	acceptOut  chan<- Accept
	promiseIn  chan Promise
	cvalIn     chan Value

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
//		_						-1
func NewProposer(id, nrOfNodes, adu int, ld detector.LeaderDetector, prepareOut chan<- Prepare, acceptOut chan<- Accept) *Proposer {
	proposer := &Proposer{
		Id:     id,
		quorum: (nrOfNodes / 2) + 1,
		n:      nrOfNodes,

		crnd:     Round(id),
		adu:      SlotID(adu),
		nextSlot: 0, //just one before adu? (in this lab in any case, in lab 6 next slot may be ahead of edu by alpha) ???

		promises: []Promise{},

		phaseOneProgressTicker: time.NewTicker(time.Second),
		phaseTwoProgressTicker: time.NewTicker(time.Second),

		acceptsOut: list.New(), //push in from the back and pop from the front, the order Accepts go in is the order they come out.
		requestsIn: list.New(),

		ld:     ld,
		leader: ld.Leader(),

		prepareOut: prepareOut,
		acceptOut:  acceptOut,
		promiseIn:  make(chan Promise, 8),
		cvalIn:     make(chan Value, 8),

		incDcd: make(chan struct{}), //incrementAllDecidedUpTo method send on this channel
		stop:   make(chan struct{}), //stops go routine invoked in Start()
	}
	proposer.phaseOneProgressTicker.Stop()
	proposer.phaseTwoProgressTicker.Stop()

	return proposer

}

// Start starts p's main run loop as a separate goroutine.
func (p *Proposer) Start() {
	trustMsgs := p.ld.Subscribe()
	go func() {
		p.startPhaseOne()
		for {
			select {
			case prm := <-p.promiseIn:
				accepts, output := p.handlePromise(prm)
				if !output {
					continue
				}
				fmt.Printf("#Proposer: Got qorum of Promises! Accepts: %v\n", accepts)
				p.nextSlot = p.adu + 1 // adu is a slotID!
				p.acceptsOut.Init()    //reset/init the list
				p.phaseOneDone = true  //phase one is done when there is a quorum of promises!
				p.phaseOneProgressTicker.Stop()
				for _, acc := range accepts {
					p.acceptsOut.PushBack(acc) //PushBack inserts a new element e with value v at the back of list acceptsOut and returns the element.
				}
				p.sendAccept() //This just triggers the first sending of Accept messages on the new round after phase 1. Subsequent Accept sendings are triggered by IncrementAllDecidedUpTo() or a client Value comming in.
			case cval := <-p.cvalIn:
				fmt.Printf("\nProposer: client value in - proposer id: %v - leader: %v - phaseOneDone: %v", p.Id, p.leader, p.phaseOneDone)
				if p.Id != p.leader {
					continue
				}
				p.requestsIn.PushBack(cval)
				if !p.phaseOneDone {
					continue
				}
				p.sendAccept()
			case <-p.incDcd: //IncrementAllDecidedUpTo, this is what will trigger sendAccept() most of the time (in normal operation)
				p.adu++
				if p.Id != p.leader {
					continue
				}
				if !p.phaseOneDone { //phase one is done when there is a quorum of promises
					continue
				}
				fmt.Printf("#Proposer: IncrementAllDecidedUpTo! sending accepts because I am the leader and phase one is done.")
				p.sendAccept() //trigger sendAccept() if its the leader and phase one is done.
			case <-p.phaseOneProgressTicker.C: //triggers every one second!
				if p.Id == p.leader && !p.phaseOneDone { //if still leader and phase one is not done... Adresses the case where Acceptors wont reply to Prepare messages becuse the Proposers round is too low.
					p.startPhaseOne()
				}
			case <-p.phaseTwoProgressTicker.C:
				fmt.Printf("Proposer: Phase two progress ticker ticked! Leader: %v - Me: %v\n", p.leader, p.Id)
				if p.Id == p.leader && p.phaseOneDone {
					//it might be cut of from the network, the acceptors may be down/not connected to the proposer....

					//resend saved Accepts or start phase 1 over again.
					p.startPhaseOne()
				}
			case leader := <-trustMsgs: //channel returned by subscribing to leader detector!
				fmt.Printf("#Proposer: New leader: %v - I am node: %v\n", leader, p.Id)
				p.leader = leader
				if leader == p.Id {
					p.startPhaseOne() //start phase 1 if elected as new leader!
				}
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
	//OBS: Stop timer
	p.phaseTwoProgressTicker.Stop() //may need to rethink for multiple inflight Accepts
	p.incDcd <- struct{}{}
}

//____Looks like all up til now is allready implemented

// Internal: handlePromise processes promise prm according to the Multi-Paxos
// algorithm. If handling the promise results in proposer p emitting a
// corresponding accept slice, then output will be true and accs contain the
// accept messages. If handlePromise returns false as output, then accs will be
// a nil slice. See the Lab 5 text for a more complete specification.
func (p *Proposer) handlePromise(prm Promise) (accs []Accept, output bool) { //what about when we had a quorum earlier...
	//If Rnd != Crnd -> ignore promise
	if prm.Rnd != p.crnd {
		return nil, false
	}
	//If promise has been received on Crnd from the same node -> ignore
	//If we have recieved the promise before
	for _, recPrm := range p.promises {
		if recPrm.Rnd == prm.Rnd && recPrm.From == prm.From { //has recieved the promise before
			return nil, false
		}
	}

	//else save the promise
	p.promises = append(p.promises, prm)

	if len(p.promises) < p.quorum { //there is not a quorum of promises
		return nil, false
	}
	/*
		for _, pr := range p.promises {
			fmt.Printf("promise: %+v\n", pr)
			for _, sl := range pr.Slots {
				fmt.Printf(" - slot: %+v\n", sl)
			}
		}
	*/
	registeredSlots := []PromiseSlot{}

	for _, recPrm := range p.promises {
		for _, slotFromPromise := range recPrm.Slots {
			//If SlotID < adu -> ignore (or <= ???)
			if slotFromPromise.ID <= p.adu { //OBS: or just <
				continue
			}
			slotHasBeenSeen := false
			//range over registedSlots and see if the slotFromPromise has been seen before, if so skip that slotFromPromise, if it is not the case that it has a higher round
			for rsIndex, registedSlot := range registeredSlots {
				//if slotID is allready in slots slice, use the slot with the highest round
				if slotFromPromise.ID == registedSlot.ID {
					if slotFromPromise.Vrnd > registedSlot.Vrnd { // I need to remember PromiseSlot.ID
						registeredSlots = append(registeredSlots[:rsIndex], append([]PromiseSlot{slotFromPromise}, registeredSlots[rsIndex+1:]...)...)
					}
					slotHasBeenSeen = true
					break //to save cycles
				}
			}
			//If it was registed before:
			if slotHasBeenSeen == true {
				continue
			}
			//Slot has not be registed before:
			registeredSlots = append(registeredSlots, slotFromPromise)
		}
	}

	//Build accepts from Slots:
	for _, registedSlot := range registeredSlots {
		accept := Accept{
			From: p.Id,
			Slot: registedSlot.ID,
			Rnd:  p.crnd,
			Val:  registedSlot.Vval,
		}
		accs = append(accs, accept)
	}

	//If not bound in any slot, the accs should be an empty slice
	if len(accs) <= 0 {
		return []Accept{}, true
	}

	//sort by slotID
	sort.SliceStable(accs, func(i int, j int) bool {
		return accs[i].Slot < accs[j].Slot
	})
	//fmt.Printf("\n     accs: %+v", accs)
	//generete no-op for gaps
	finalAccs := []Accept{}

	startOfSID := accs[0].Slot
	endOfSID := accs[len(accs)-1].Slot
	accsIndex := 0
	for SID := startOfSID; SID <= endOfSID; SID++ { //proposer is bound by highest vrnd/vval tuple reported in a promise
		if SID == accs[accsIndex].Slot {
			finalAccs = append(finalAccs, accs[accsIndex])
			accsIndex++
			continue
		}
		noopAccept := Accept{
			From: p.Id,
			Slot: SID,
			Rnd:  p.crnd,
			Val: Value{
				Noop: "true",
			},
		}
		finalAccs = append(finalAccs, noopAccept)
	}

	//fmt.Printf("\nfinalAccs: %+v\n\n\n", finalAccs)

	//Return []Accept where slot > nextSlot and accs slice should be in increasing consecutive slot order.
	return finalAccs, true

	//reset p.promises, what to do about more Promises on a round where we allready had a quorum.

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
	p.promises = []Promise{}
	p.increaseCrnd()
	p.phaseOneProgressTicker = time.NewTicker(1 * time.Second)
	p.prepareOut <- Prepare{From: p.Id, Slot: p.adu, Crnd: p.crnd}
	fmt.Printf("\nProposer: phase one started. Crnd: %v", p.crnd)
}

// Internal: sendAccept sends an accept from either the accept out queue
// (generated after Phase One) if not empty, or, it generates an accept using a
// value from the client request queue if not empty. It does not send an accept
// if the previous slot has not been decided yet.

//OBS: What if a slot should not be decided? Then we wont get IncrementAllDecidedUpTo() invoked? I think it will get stuck in the current specification!
//Assuming Learner will allways eventually get a quorum of Learn messages and triger decided. (can be ensured by network layer properties, and if they fail a leader change will happen starting phase one over agina...)
//No! this could actually happen and you can implement a solution if you want.
//Can be solved with a timer started/activated when sending accepts and times out after some time if Learners do not trigger decide. When we do get a decide the timer can be deactivated.

func (p *Proposer) sendAccept() { //Once sendAccept() is called once, it will be retriggered by IncrementAllDecidedUpTo(), which is called when a value is decided (repored by Learners)
	const alpha = 1
	fmt.Printf("#Proposer: Send Accept invoked! NextSlot: %v - Adu: %v - Requests In length: %v - AcceptsOut length: %v\n", p.nextSlot, p.adu, p.requestsIn.Len(), p.acceptsOut.Len())
	//if p.nextSlot > p.adu + alpha {...}
	if !(p.nextSlot <= p.adu+alpha) { //when done with HandlePromise() nextSlot = adu+1 , so next slot will never be lower than adu I think.
		// We must wait for the next slot to be decided before we can
		// send an accept.
		return
	}

	// Pri 1: If bounded by any accepts from Phase One -> send previously
	// generated accept and return.
	if p.acceptsOut.Len() > 0 { //acceptsOut is a List...
		acc := p.acceptsOut.Front().Value.(Accept) //Front returns the first element of list l or nil if the list is empty.
		p.acceptsOut.Remove(p.acceptsOut.Front())  //Remove removes e from l if e is an element of list l. It returns the element value e.Value. The element must not be nil. (ensured by testing for Len > 0)
		p.acceptOut <- acc
		//OBS: start phase two timer
		p.phaseTwoProgressTicker = time.NewTicker(2 * time.Second)
		p.nextSlot++ //we only increment nextSlot when sending an Accept, and due to the incremental ordering of Accepts from HandlePromise and that non bound slots are filled with a No-op we can safel assume that nextSlot and slotID in Accept message matches.
		return
	}

	// Pri 2: If any client request in queue -> generate and send
	// accept.
	if p.requestsIn.Len() > 0 {
		cval := p.requestsIn.Front().Value.(Value) //Front() returns list elements
		p.requestsIn.Remove(p.requestsIn.Front())
		acc := Accept{
			From: p.Id,
			Slot: p.nextSlot,
			Rnd:  p.crnd,
			Val:  cval,
		}
		p.acceptOut <- acc
		//OBS: start phase two timer
		p.phaseTwoProgressTicker = time.NewTicker(2 * time.Second)
		p.nextSlot++
	}
}

// For Lab 6: Alpha has a value of one here. If you later
// implement pipelining then alpha should be extracted to a
// proposer variable (alpha) and this function should have an
// outer for loop.

/*

	if slotFromPromise.ID == registeredSlot.ID { //if the slot has been registed before
		if slotFromPromise.Vrnd > registeredSlot.Vrnd { //Slot round is higher on the slot from the promise rather then that from the regitered slots
			slotsToBuildAccepts = append(slotsToBuildAccepts[:stbaIndex], apppend([]PromiseSlot{slotFromPromise}, slotsToBuildAccepts[stbaIndex+1:]...)...)
		}

	}

*/
