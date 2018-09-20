// +build !solution

package singlepaxos

// Learner represents a learner as defined by the single-decree Paxos
// algorithm.
type Learner struct {
	Id             int
	NrOfNodes      int
	ValueOut       chan<- Value
	CurrentRound   Round
	Value          Value
	AcceptResponse []RoundValuePair
	learnChan      chan Learn
	stopChan       chan int

	//TODO(student): Task 2 and 3 - algorithm and distributed implementation
	// Add needed fields
}

// NewLearner returns a new single-decree Paxos learner. It takes the
// following arguments:
//
// id: The id of the node running this instance of a Paxos learner.
//
// nrOfNodes: The total number of Paxos nodes.
//
// valueOut: A send only channel used to send values that has been learned,
// i.e. decided by the Paxos nodes.
func NewLearner(id int, nrOfNodes int, valueOut chan<- Value) *Learner {
	//TODO(student): Task 2 and 3 - algorithm and distributed implementation
	return &Learner{
		Id:             id,
		NrOfNodes:      nrOfNodes,
		ValueOut:       valueOut,
		CurrentRound:   NoRound,
		Value:          ZeroValue,
		AcceptResponse: []RoundValuePair{},
		learnChan:      make(chan Learn, 16),
		stopChan:       make(chan int, 1),
	}
}

// Start starts l's main run loop as a separate goroutine. The main run loop
// handles incoming learn messages.
func (l *Learner) Start() {
	go func() {
		for {
			select {
			case lrn := <-l.learnChan:
				if value, ok := l.handleLearn(lrn); ok == true {
					l.ValueOut <- value
				}
			case <-l.stopChan:
				break
			}
			//TODO(student): Task 3 - distributed implementation

		}
	}()
}

// Stop stops l's main run loop.
func (l *Learner) Stop() {
	//TODO(student): Task 3 - distributed implementation
	l.stopChan <- 0
}

// DeliverLearn delivers learn lrn to learner l.
func (l *Learner) DeliverLearn(lrn Learn) {
	//TODO(student): Task 3 - distributed implementation
	l.learnChan <- lrn
}

// Internal: handleLearn processes learn lrn according to the single-decree
// Paxos algorithm. If handling the learn results in learner l emitting a
// corresponding decided value, then output will be true and val contain the
// decided value. If handleLearn returns false as output, then val will have
// its zero value.
func (l *Learner) handleLearn(learn Learn) (val Value, output bool) {
	//TODO(student): Task 2 - algorithm implementation
	if learn.Rnd < l.CurrentRound || learn.Rnd == NoRound {
		return "", false
	}
	if learn.Rnd > l.CurrentRound {
		l.CurrentRound = learn.Rnd
		l.AcceptResponse = nil
	}
	for _, i := range l.AcceptResponse {
		if i.From == learn.From {
			return "", false
		}
	}
	l.AcceptResponse = append(l.AcceptResponse, RoundValuePair{From: learn.From, VotedRound: learn.Rnd, VotedValue: learn.Val})
	if len(l.AcceptResponse) >= l.NrOfNodes/2+1 {
		l.Value = learn.Val
		return l.Value, true
	}
	return "", false
}

//TODO(student): Add any other unexported methods needed.
