// +build !solution

package multipaxos4

// Learner represents a learner as defined by the Multi-Paxos algorithm.
type Learner struct {
	// TODO(student)
	id     int
	quorum int

	rnd          Round
	learnHistory map[SlotID][]Learn

	decidedOut chan<- DecidedValue
	learnIn    chan Learn

	stop chan struct{}
}

// NewLearner returns a new single-decree Paxos learner. It takes the
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
		// TODO(student)
		id:     id,
		quorum: (nrOfNodes / 2) + 1,

		rnd:          NoRound,
		learnHistory: map[SlotID][]Learn{},

		decidedOut: decidedOut,
		learnIn:    make(chan Learn, 8),

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
				if value, slot, ok := l.handleLearn(lrn); ok == true {
					//fmt.Printf("DecidedValue out, slot: %v, len(l.decidedOut): %v\n", slot, len(l.decidedOut))
					l.decidedOut <- DecidedValue{SlotID: slot, Value: value}
				}
			case <-l.stop:
				break
			}
		}
	}()
}

// Stop stops l's main run loop.
func (l *Learner) Stop() {
	// TODO(student)
	l.stop <- struct{}{}
}

// DeliverLearn delivers learn lrn to learner l.
func (l *Learner) DeliverLearn(lrn Learn) {
	// TODO(student)
	l.learnIn <- lrn
}

// Internal: handleLearn processes learn lrn according to the Multi-Paxos
// algorithm. If handling the learn results in learner l emitting a
// corresponding decided value, then output will be true, sid the id for the
// slot that was decided and val contain the decided value. If handleLearn
// returns false as output, then val and sid will have their zero value.
func (l *Learner) handleLearn(learn Learn) (val Value, sid SlotID, output bool) {
	//fmt.Printf("HandleLearn(), Learn: %v, l.rnd: %v, l.learnHistory: %v\n", learn, l.rnd, l.learnHistory)
	if learn.Rnd < l.rnd {
		return val, sid, false
	} else if learn.Rnd == l.rnd {
		if l.learnHistory[learn.Slot] == nil {
			l.learnHistory[learn.Slot] = []Learn{}
		}
		if inSlice(l.learnHistory[learn.Slot], learn) == true {
			//fmt.Printf("HandleLearn(): SlotID %v was already in learnHistory\n", learn.Slot)

			return val, sid, false
		}
		l.learnHistory[learn.Slot] = append(l.learnHistory[learn.Slot], learn)
		////fmt.Printf("Learn History: %+v", l.learnHistory[learn.Slot])
		//fmt.Printf("HandleLearn(): len(learnHistory[learn.Slot]): %v, quorum: %v\n", len(l.learnHistory[learn.Slot]), l.quorum)
		if len(l.learnHistory[learn.Slot]) == l.quorum {
			return learn.Val, learn.Slot, true
		}

	} else {
		//zero out
		l.rnd = learn.Rnd
		l.learnHistory = map[SlotID][]Learn{}
		l.learnHistory[learn.Slot] = []Learn{learn}
	}
	return val, sid, false
}

func inSlice(learnSlice []Learn, learn Learn) bool {
	for _, l := range learnSlice {
		if l.Rnd == learn.Rnd && l.Slot == learn.Slot && l.From == learn.From {
			return true
		}
	}
	return false
}
