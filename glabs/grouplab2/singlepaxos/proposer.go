// +buiLeaderDetector !solution

package singlepaxos

import (
	"fmt"

	"github.com/uis-dat520-s18/glabs/grouplab1/detector"
)

//RoundValuePair represents the promise&accept response messages
type RoundValuePair struct {
	From       int
	VotedRound Round
	VotedValue Value
}

// Proposer represents a proposer as defined by the single-decree Paxos
// algorithm.
type Proposer struct {
	crnd            Round
	clientValue     Value
	Id              int
	NrOfNodes       int
	LeaderDetector  detector.LeaderDetector
	PrepareOut      chan<- Prepare
	AcceptOut       chan<- Accept
	PromiseResponse []RoundValuePair
	promiseChan     chan Promise
	clientValueChan chan Value
	stopChan        chan int

	//TODO(student): Task 2 and 3 - algorithm and distributed implementation
	// Add other needed fields
}

// NewProposer returns a new single-decree Paxos proposer.
// It takes the following arguments:
//
// id: The id of the node running this instance of a Paxos proposer.
//
// nrOfNodes: The total number of Paxos nodes.
//
// ld: A leader detector implementing the detector.LeaderDetector interface.
//
// prepareOut: A send only channel used to send prepares to other nodes.
//
// The proposer's internal crnd field should initially be set to the value of
// its id.
func NewProposer(id int, nrOfNodes int, ld detector.LeaderDetector, prepareOut chan<- Prepare, acceptOut chan<- Accept) *Proposer {
	//TODO(student): Task 2 and 3 - algorithm and distributed implementation
	return &Proposer{
		crnd:            Round(id), //bug, bør være NoRound,
		clientValue:     ZeroValue,
		Id:              id,
		NrOfNodes:       nrOfNodes,
		LeaderDetector:  ld,
		PrepareOut:      prepareOut,
		AcceptOut:       acceptOut,
		PromiseResponse: []RoundValuePair{},
		promiseChan:     make(chan Promise, 16),
		clientValueChan: make(chan Value, 16),
		stopChan:        make(chan int, 1),
	}
}

// Start starts p's main run loop as a separate goroutine. The main run loop
// handles incoming promise messages and leader detector trust messages.
func (p *Proposer) Start() {
	go func() {
		for {
			select {
			case prm := <-p.promiseChan:
				if accept, ok := p.handlePromise(prm); ok == true {
					p.AcceptOut <- accept
				}
			case val := <-p.clientValueChan:
				if prepare, ok := p.handleClient(val); ok == true {
					p.PrepareOut <- prepare
				}
			case <-p.stopChan:
				break
			}
			//TODO(student): Task 3 - distributed implementation
		}
	}()
}

// Stop stops p's main run loop.
func (p *Proposer) Stop() {
	//TODO(student): Task 3 - distributed implementation
	p.stopChan <- 0
}

// DeliverPromise delivers promise prm to proposer p.
func (p *Proposer) DeliverPromise(prm Promise) {
	//TODO(student): Task 3 - distributed implementation
	p.promiseChan <- prm
}

// DeliverClientValue delivers client value val from to proposer p.
func (p *Proposer) DeliverClientValue(val Value) {
	//TODO(student): Task 3 - distributed implementation
	if val == ZeroValue {
		p.RandomValue()
	} else {
		p.clientValue = val
	}
	fmt.Printf("\n%v", p.crnd) //________________________________________________________________________________________________________________
	p.clientValueChan <- val
}

// Internal: handlePromise processes promise prm according to the single-decree
// Paxos algorithm. If handling the promise results in proposer p emitting a
// corresponding accept, then output will be true and acc contain the promise.
// If handlePromise returns false as output, then acc will be a zero-valued
// struct.
func (p *Proposer) handlePromise(prm Promise) (acc Accept, output bool) {
	//TODO(student): Task 2 - algorithm implementation
	if prm.Rnd < p.crnd || prm.Rnd == NoRound {
		return Accept{}, false
	}
	if prm.Rnd > p.crnd {
		p.crnd = prm.Rnd
		p.PromiseResponse = nil
		return Accept{}, false
	}
	//prm.Rnd == p.crnt

	//check if recieved previous, then place answer in PromiseResponse
	for _, i := range p.PromiseResponse {
		if i.From == prm.From {
			return Accept{}, false
		}
	}
	p.PromiseResponse = append(p.PromiseResponse, RoundValuePair{From: prm.From, VotedRound: prm.Vrnd, VotedValue: prm.Vval})

	if len(p.PromiseResponse) > p.NrOfNodes/2 {
		largest := RoundValuePair{}
		for _, i := range p.PromiseResponse {
			if i.VotedRound > largest.VotedRound {
				largest = i
			}
		}
		if largest.VotedRound == NoRound || largest.VotedValue == ZeroValue {
			if p.clientValue == ZeroValue {
				p.RandomValue()
			}
			return Accept{From: p.Id, Rnd: p.crnd, Val: p.clientValue}, true
		} else {
			p.clientValue = largest.VotedValue
			return Accept{From: p.Id, Rnd: p.crnd, Val: p.clientValue}, true
		}
	}
	return Accept{}, false
}

// Internal: increaseCrnd increases proposer p's crnd field by the total number
// of Paxos nodes.
func (p *Proposer) increaseCrnd() {
	//TODO(student): Task 2 - algorithm implementation
	p.crnd += Round(p.NrOfNodes)
	p.PromiseResponse = nil
	return
}

//TODO(student): Add any other unexported methods needed.

//RandomValue is a set of predefined values assigned to self id
func (p *Proposer) RandomValue() {
	//TODO(student): Task 2 - algorithm implementation
	randomValue := []string{"ValueFrom0", "ValueFrom1", "ValueFrom2", "ValueFrom3", "ValueFrom4"}
	p.clientValue = Value(randomValue[p.Id])
	return
}

func (p *Proposer) handleClient(val Value) (prp Prepare, output bool) {
	leader := p.LeaderDetector.Leader()
	p.increaseCrnd()
	if leader == p.Id {
		fmt.Printf("\n#PROPOSER TRUE, IM LEADER") //________________________________________________________________________________________________________________
		return Prepare{From: p.Id, Crnd: p.crnd}, true
	} else {
		fmt.Printf("\n#PROPOSER FALSE, Im not leader") //________________________________________________________________________________________________________________
		return Prepare{}, false
	}
}
