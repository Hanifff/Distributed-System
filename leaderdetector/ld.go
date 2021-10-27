package leaderdetector

import (
	"sync"
)

// A MonLeaderDetector represents a Monarchical Eventual Leader Detector as
// described at page 53 in:
// Christian Cachin, Rachid Guerraoui, and Luís Rodrigues: "Introduction to
// Reliable and Secure Distributed Programming" Springer, 2nd edition, 2011.
type MonLeaderDetector struct {
	nodes     []int
	suspected map[int]bool
	lockSusld map[int]*sync.Mutex

	restorer map[int]bool
	lockRes  map[int]*sync.Mutex

	subscribers []chan int
}

// NewMonLeaderDetector returns a new Monarchical Eventual Leader Detector
// given a list of node ids.
func NewMonLeaderDetector(nodeIDs []int) *MonLeaderDetector {
	m := &MonLeaderDetector{
		nodes:     nodeIDs,
		suspected: make(map[int]bool),
		lockSusld: make(map[int]*sync.Mutex),
		restorer:  make(map[int]bool),
		lockRes:   make(map[int]*sync.Mutex),
	}
	return m
}

// Leader returns the current leader. Leader will return UnknownID if all nodes
// are suspected.
func (m *MonLeaderDetector) Leader() int {
	lead := m.findLeader()
	if lead > -1 {
		return lead
	}
	return UnknownID
}

// AddNode notifies the leaderdetector to scale up its set of nodes
func (m *MonLeaderDetector) AddNode(id int) {
	if !m.continasNode(id) {
		oldLeader := m.Leader()
		m.nodes = append(m.nodes, id)
		newLeader := m.Leader()
		if oldLeader != newLeader { // make the notification here, instead of calling the Restorer method in fd
			m.notify(newLeader)
		}
	}
}

// Suspect instructs the leader detector to consider the node with matching
// id as suspected. If the suspect indication result in a leader change
// the leader detector should publish this change to its subscribers.
func (m *MonLeaderDetector) Suspect(id int) {
	oldLeader := m.Leader()
	if _, exist := m.lockSusld[id]; !exist {
		m.lockSusld[id] = &sync.Mutex{}
		m.lockSusld[id].Lock()
		m.suspected[id] = true
		m.lockSusld[id].Unlock()
	} else {
		m.lockSusld[id].Lock()
		m.suspected[id] = true
		m.lockSusld[id].Unlock()
	}
	newLeader := m.Leader()
	if oldLeader != newLeader {
		m.notify(newLeader)
	}
}

// Restore instructs the leader detector to consider the node with matching
// id as restored. If the restore indication result in a leader change
// the leader detector should publish this change to its subscribers.
func (m *MonLeaderDetector) Restore(id int) {
	oldLeader := m.Leader()
	delete(m.suspected, id)

	if _, exist := m.lockRes[id]; !exist {
		m.lockRes[id] = &sync.Mutex{}
		m.lockRes[id].Lock()
		m.restorer[id] = true
		m.lockRes[id].Unlock()
	} else {
		m.lockRes[id].Lock()
		m.restorer[id] = true
		m.lockRes[id].Unlock()
	}
	newLeader := m.Leader()
	if oldLeader != newLeader {
		m.notify(newLeader)
	}
}

// Subscribe returns a buffered channel which will be used by the leader
// detector to publish the id of the highest ranking node.
// The leader detector will publish UnknownID if all nodes become suspected.
// Subscribe will drop publications to slow subscribers.
// Note: Subscribe returns a unique channel to every subscriber;
// it is not meant to be shared.
func (m *MonLeaderDetector) Subscribe() <-chan int {
	subsChan := make(chan int, 1)
	m.subscribers = append(m.subscribers, subsChan)
	return subsChan
}

// notify notifies all subscribers if a new leader detected.
func (m *MonLeaderDetector) notify(trust int) {
	for _, subscriber := range m.subscribers {
		select {
		case subscriber <- trust:
		default:
		}
	}
}

// continasNode checks if a node is presented in the list of nodes
func (m *MonLeaderDetector) continasNode(id int) bool {
	for _, n := range m.nodes {
		if n == id {
			return true
		}
	}
	return false
}

// findLeader iterates over the list of processes that are not suspected and returns node with the highest ransk.
func (m *MonLeaderDetector) findLeader() int {
	max := -1
	for _, n := range m.nodes {
		if _, exist := m.suspected[n]; !exist {
			if max < n {
				max = n
			}
		}
	}
	return max
}
