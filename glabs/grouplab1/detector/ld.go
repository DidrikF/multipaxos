// +build !solution

package detector

import (
	"time"
)

// A MonLeaderDetector represents a Monarchical Eventual Leader Detector as
// described at page 53 in:
// Christian Cachin, Rachid Guerraoui, and Luís Rodrigues: "Introduction to
// Reliable and Secure Distributed Programming" Springer, 2nd edition, 2011.
type MonLeaderDetector struct {
	leader             int
	suspected          map[int]bool
	nodeIDs            []int
	subscribers        []chan int
	recoverSubscribers []chan int
	// TODO(student): Add needed fields
}

// NewMonLeaderDetector returns a new Monarchical Eventual Leader Detector
// given a list of node ids.
func NewMonLeaderDetector(nodeIDs []int) *MonLeaderDetector {
	m := &MonLeaderDetector{
		leader:             UnknownID,
		suspected:          map[int]bool{},
		nodeIDs:            nodeIDs,
		subscribers:        []chan int{},
		recoverSubscribers: []chan int{},
	}
	m.selectLeader()

	//For debugging purposes, can/should remove
	/*
		go func() {
			for {
				time.Sleep(2 * time.Second)
				m.publish()
			}
		}()
	*/
	//__________________________________________

	return m
}

// Leader returns the current leader. Leader will return UnknownID if all nodes
// are suspected.
func (m *MonLeaderDetector) Leader() int {
	// TODO(student): Implement
	for _, valID := range m.nodeIDs {
		if m.suspected[valID] == false {
			return m.leader
		}
	}
	return UnknownID
}

// Suspect instructs m to consider the node with matching id as suspected. If
// the suspect indication result in a leader change the leader detector should
// this publish this change its subscribers.
func (m *MonLeaderDetector) Suspect(id int) {
	// TODO(student): Implement
	if m.suspected[id] == true {
		return
	}
	m.suspected[id] = true
	leaderchange := m.selectLeader()
	if leaderchange == true {
		m.publish()
	}
}

func (m *MonLeaderDetector) SubscribeToRecoverEvents() <-chan int {
	channel := make(chan int, 1)

	m.recoverSubscribers = append(m.recoverSubscribers, channel)

	return channel
}

// Restore instructs m to consider the node with matching id as restored. If
// the restore indication result in a leader change the leader detector should
// this publish this change its subscribers.
func (m *MonLeaderDetector) Restore(id int) {
	m.publishRecovered(id)
	// TODO(student): Implement
	if m.suspected[id] == false {
		return
	}
	m.suspected[id] = false
	leaderchange := m.selectLeader()
	if leaderchange == true {
		m.publish()
	}
}

// Subscribe returns a buffered channel that m on leader change will use to
// publish the id of the highest ranking node. The leader detector will publish
// UnknownID if all nodes become suspected. Subscribe will drop publications to
// slow subscribers. Note: Subscribe returns a unique channel to every
// subscriber; it is not meant to be shared.
func (m *MonLeaderDetector) Subscribe() <-chan int {
	// TODO(student): Implement
	channel := make(chan int, 1)

	m.subscribers = append(m.subscribers, channel)

	return channel
}

// TODO(student): Add other unexported functions or methods if needed.

// Select Leader
func (m *MonLeaderDetector) selectLeader() bool {
	candidates := []int{}
	for _, valID := range m.nodeIDs {
		if m.suspected[valID] == false {
			candidates = append(candidates, valID)
		}
	}

	pleader := m.leader
	m.leader = UnknownID
	for _, valID := range candidates { // if no candidates, leader will be -1
		if valID > m.leader {
			m.leader = valID
		}
	}

	if pleader == m.leader {
		return false
	}
	return true
}

func (m *MonLeaderDetector) publish() {
	//go func() {
	for _, channel := range m.subscribers {
		select {
		case channel <- m.leader:
			break
		case <-time.After(100 * time.Millisecond):
			continue
		}
	}
	//}()
}

func (m *MonLeaderDetector) publishRecovered(recovered int) {
	//go func() {
	for _, channel := range m.recoverSubscribers {
		select {
		case channel <- recovered:
			break
		case <-time.After(100 * time.Millisecond):
			continue
		}
	}
	//}()
}
