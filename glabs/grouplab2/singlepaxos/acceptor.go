// +build !solution

package singlepaxos

// Acceptor represents an acceptor as defined by the single-decree Paxos
// algorithm.
type Acceptor struct {
	//TODO(student): Task 2 and 3 - algorithm and distributed implementation
	Id             int
	PromiseOut     chan<- Promise
	LearnOut       chan<- Learn
	CurrentRound   Round
	LastVotedRound Round
	Value          Value
	prepareChan    chan Prepare
	acceptChan     chan Accept
	stop           chan int
}

type Node struct {
	Id int
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
	//TODO(student): Task 2 and 3 - algorithm and distributed implementation
	acceptor := &Acceptor{
		Id:             id,
		PromiseOut:     promiseOut,
		LearnOut:       learnOut,
		LastVotedRound: NoRound,
		Value:          ZeroValue,
		CurrentRound:   NoRound,
		prepareChan:    make(chan Prepare, 16),
		acceptChan:     make(chan Accept, 16),
		stop:           make(chan int),
	}

	// set proposers

	//set learners

	return acceptor

}

// Start starts a's main run loop as a separate goroutine. The main run loop
// handles incoming prepare and accept messages.
func (a *Acceptor) Start() {
	go func() {
		for {
			//TODO(student): Task 3 - distributed implementation
			select {
			case prp := <-a.prepareChan:
				promise, ok := a.handlePrepare(prp)
				if ok == true {
					a.PromiseOut <- promise
				}
				//fmt.Printf("Acceptor %v: promise ok? %v", a.Id, ok)
			case acc := <-a.acceptChan:
				if learn, ok := a.handleAccept(acc); ok == true {
					a.LearnOut <- learn
				}
			case <-a.stop:
				break
			}
		}
	}()
}

// Stop stops a's main run loop.
func (a *Acceptor) Stop() {
	//TODO(student): Task 3 - distributed implementation
	a.stop <- 0

}

// DeliverPrepare delivers prepare prp to acceptor a.
func (a *Acceptor) DeliverPrepare(prp Prepare) {
	//TODO(student): Task 3 - distributed implementation
	a.prepareChan <- prp
}

// DeliverAccept delivers accept acc to acceptor a.
func (a *Acceptor) DeliverAccept(acc Accept) {
	//TODO(student): Task 3 - distributed implementation
	a.acceptChan <- acc
}

// Internal: handlePrepare processes prepare prp according to the single-decree
// Paxos algorithm. If handling the prepare results in acceptor a emitting a
// corresponding promise, then output will be true and prm contain the promise.
// If handlePrepare returns false as output, then prm will be a zero-valued
// struct.
func (a *Acceptor) handlePrepare(prp Prepare) (prm Promise, output bool) {
	if prp.Crnd > a.CurrentRound {
		a.CurrentRound = prp.Crnd
		return Promise{To: prp.From, From: a.Id, Rnd: a.CurrentRound, Vrnd: a.LastVotedRound, Vval: a.Value}, true
	}

	return Promise{}, false
}

// Internal: handleAccept processes accept acc according to the single-decree
// Paxos algorithm. If handling the accept results in acceptor a emitting a
// corresponding learn, then output will be true and lrn contain the learn.  If
// handleAccept returns false as output, then lrn will be a zero-valued struct.
func (a *Acceptor) handleAccept(acc Accept) (lrn Learn, output bool) {
	if acc.Rnd >= a.CurrentRound && acc.Rnd != a.LastVotedRound {
		a.CurrentRound = acc.Rnd
		a.LastVotedRound = acc.Rnd
		a.Value = acc.Val
		return Learn{From: a.Id, Rnd: acc.Rnd, Val: acc.Val}, true
	}

	return Learn{}, false
}
