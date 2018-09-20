package fakenet

import (
	"github.com/uis-dat520-s18/glabs/grouplab3/network2"
)

type Network struct {
	Self              int
	Nodes             []int
	SChan             chan network2.Message
	RChans            map[int]chan network2.Message
	ClientHandlerChan chan network2.ClientInfo
}

func CreateNetwork(nodes []int) *Network {
	network := &Network{
		Nodes: nodes,
		SChan: make(chan network2.Message, 16),
		RChans: map[int]chan network2.Message{
			1: make(chan network2.Message, 8),
			2: make(chan network2.Message, 8),
			3: make(chan network2.Message, 8),
		},
		ClientHandlerChan: make(chan network2.ClientInfo, 8),
	}
	return network
}

func (n *Network) Start() {
	go func() {
		for {
			select {
			case msg := <-n.SChan:
				n.RChans[msg.To] <- msg
				//fmt.Println("pushed message on rchan " + strconv.Itoa(msg.To))
			}
		}
	}()
}

func (n *Network) BroadcastTo(nodes []int, msg network2.Message) {
	for _, nodeID := range nodes {
		msg.To = nodeID
		n.SChan <- msg
	}
}
