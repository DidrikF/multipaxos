package clienthandler

import (
	"net"

	"github.com/uis-dat520-s18/group122/grouplab1/logger"
	"github.com/uis-dat520-s18/group122/grouplab1/network"
	"github.com/uis-dat520-s18/group122/grouplab2/singlepaxos"
)

type ClientHandler struct {
	Id             int
	Connections    []net.Conn
	Logger         logger.Logger
	ClientValueIn  chan singlepaxos.Value
	LearnerValueIn chan singlepaxos.Value
	network        *network.Network
	proposer       *singlepaxos.Proposer
}

func NewClientHandler(id int, network *network.Network, proposer *singlepaxos.Proposer) *ClientHandler {
	return &ClientHandler{
		Id:             id,
		Connections:    []net.Conn{},
		ClientValueIn:  make(chan singlepaxos.Value, 16),
		LearnerValueIn: make(chan singlepaxos.Value, 16),
		network:        network,
		proposer:       proposer,
	}
}

func (h *ClientHandler) Start() {
	go func() {
		for {
			select {
			case clientValue := <-h.ClientValueIn:
				h.proposer.DeliverClientValue(clientValue)
			case learnerValue := <-h.LearnerValueIn:
				for _, conn := range h.Connections {
					msg := network.Message{
						Type:  "value",
						From:  h.Id,
						Value: learnerValue,
					}
					h.network.SendToConn(conn, msg)
				}
			case conn := <-h.network.AddClientConn:
				h.Connections = append(h.Connections, conn)
			case rConn := <-h.network.RemoveClientConn:
				for i, conn := range h.Connections {
					if rConn.RemoteAddr().String() == conn.RemoteAddr().String() {
						h.Connections = append(h.Connections[:i], h.Connections[i+1:]...)
					}
				}

			}
		}
	}()
}

func (h *ClientHandler) DeliverClientValue(val singlepaxos.Value) {
	h.ClientValueIn <- val
}

func (h *ClientHandler) DeliverValueFromLearner(val singlepaxos.Value) {
	h.LearnerValueIn <- val
}
