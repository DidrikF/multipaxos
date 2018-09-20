// +build !solution

package detector

import (
	"time"
)

// EvtFailureDetector represents a Eventually Perfect Failure Detector as
// described at page 53 in:
// Christian Cachin, Rachid Guerraoui, and Luís Rodrigues: "Introduction to
// Reliable and Secure Distributed Programming" Springer, 2nd edition, 2011.
type EvtFailureDetector struct {
	id        int          // this node's id, does not seem to be used!
	nodeIDs   []int        // node ids for every node in cluster
	alive     map[int]bool // map of node ids considered alive
	suspected map[int]bool // map of node ids  considered suspected

	sr SuspectRestorer // Provided SuspectRestorer implementation

	delay         time.Duration // the current delay for the timeout procedure
	delta         time.Duration // the delta value to be used when increasing delay
	timeoutSignal *time.Ticker  // the timeout procedure ticker

	//write only channel
	hbSend chan<- Heartbeat // channel for sending outgoing heartbeat messages
	hbIn   chan Heartbeat   // channel for receiving incoming heartbeat messages
	stop   chan struct{}    // channel for signaling a stop request to the main run loop

	testingHook func() // DO NOT REMOVE THIS LINE. A no-op when not testing.
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

//                            2, []int{2, 1, 0} ,
func NewEvtFailureDetector(id int, nodeIDs []int, sr SuspectRestorer, delta time.Duration, hbSend chan<- Heartbeat) *EvtFailureDetector {
	suspected := make(map[int]bool)
	alive := make(map[int]bool)

	// TODO(student): perform any initialization necessary
	for _, id := range nodeIDs {
		alive[id] = true //all nodes are alive initially (it says to do so in the algorithm)
		suspected[id] = false
	}

	fd := &EvtFailureDetector{
		id:        id,
		nodeIDs:   nodeIDs, // []int{2, 1, 0}
		alive:     alive,
		suspected: suspected,

		sr: sr, // <-- leader detector

		delay:         delta,
		delta:         delta,
		timeoutSignal: time.NewTicker(delta),
		//timeoutSignal: nil, // type of *time.Ticker , this is initialized with the *time.Ticker's zero values.

		hbSend: hbSend,
		hbIn:   make(chan Heartbeat, 8), //channel with a small buffer
		stop:   make(chan struct{}),

		testingHook: func() {}, // DO NOT REMOVE THIS LINE. A no-op when not testing.
	}
	fd.timeoutSignal.Stop()
	return fd
}

// Start starts e's main run loop as a separate goroutine. The main run loop
// handles incoming heartbeat requests and responses. The loop also trigger e's
// timeout procedure at an interval corresponding to e's internal delay
// duration variable.
func (e *EvtFailureDetector) Start() { //test calls Start()
	e.timeoutSignal = time.NewTicker(e.delay) ////return *Ticker
	go func() {
		for {
			e.testingHook() // DO NOT REMOVE THIS LINE. A no-op when not testing.
			select {
			case hbReq := <-e.hbIn:
				// TODO(student): Handle incoming heartbeat
				//hbReq {To: ourID, From: i, Request: true}
				if hbReq.Request == "request" && hbReq.To == e.id {
					hbReply := Heartbeat{To: hbReq.From, From: e.id, Request: "response"}
					e.hbSend <- hbReply
				} else if hbReq.Request == "response" && hbReq.To == e.id {
					e.alive[hbReq.From] = true
				}
			case <-e.timeoutSignal.C: // <- chan Time
				e.timeout()
			case <-e.stop:
				return
			}
		}
	}()
}

// DeliverHeartbeat delivers heartbeat hb to failure detector e.
func (e *EvtFailureDetector) DeliverHeartbeat(hb Heartbeat) {
	e.hbIn <- hb //hbReq := Heartbeat{To: ourID, From: i, Request: true}
}

// Stop stops e's main run loop.
func (e *EvtFailureDetector) Stop() {
	e.stop <- struct{}{}
}

// Internal: timeout runs e's timeout procedure.
func (e *EvtFailureDetector) timeout() {
	// TODO(student): Implement timeout procedure

	if e.nodeInAliveAndSuspected() == true {
		e.delay = e.delay + e.delta
	}
	for _, nodeId := range e.nodeIDs {
		if e.isAlive(nodeId) == false && e.isSuspected(nodeId) == false {
			e.suspected[nodeId] = true
			//trigger suspected event ???
			e.sr.Suspect(nodeId)
		} else if e.isAlive(nodeId) == true && e.isSuspected(nodeId) == true {
			e.suspected[nodeId] = false
			//Trigger restore event
			e.sr.Restore(nodeId)
		}
		hbRequest := Heartbeat{To: nodeId, From: e.id, Request: "request"}
		e.hbSend <- hbRequest
	}
	e.resetAlive()
	e.trimSuspected()
	e.timeoutSignal.Stop()
	e.timeoutSignal = time.NewTicker(e.delay)
}

// TODO(student): Add other unexported functions or methods if needed.

func intInSlice(i int, s []int) bool {
	for _, element := range s {
		if element == i {
			return true
		}
	}
	return false
}

func (e *EvtFailureDetector) nodeInAliveAndSuspected() bool {
	for i := range e.alive {
		if e.alive[i] == true && e.suspected[i] == true {
			return true
		}
	}
	return false
}

func (e *EvtFailureDetector) isAlive(id int) bool {
	return e.alive[id]
}

func (e *EvtFailureDetector) isSuspected(id int) bool {
	return e.suspected[id]
}
func (e *EvtFailureDetector) resetAlive() {
	e.alive = map[int]bool{}

	//e.alive[e.id] = true //IS THIS NEEDED OR DO I RESPOND TO HEARTBEATS TO MYSELF!
}

//Needed to be done to pass tests, although the logic is the same without. (suspected[2] = false, is the same as not having suspected[2] in the map.)
func (e *EvtFailureDetector) trimSuspected() {
	susp := e.suspected
	e.suspected = map[int]bool{}
	for key, value := range susp {
		if value == true {
			e.suspected[key] = value
		}
	}
}
