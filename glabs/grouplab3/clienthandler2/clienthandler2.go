package clienthandler2

import (
	"fmt"

	"github.com/uis-dat520-s18/glabs/grouplab1/detector"
	"github.com/uis-dat520-s18/glabs/grouplab3/logger2"
	"github.com/uis-dat520-s18/glabs/grouplab3/multipaxos"
	"github.com/uis-dat520-s18/glabs/grouplab3/network2"
)

type ClientHandler struct {
	Id            int
	ld            detector.LeaderDetector
	leader        int
	L             *logger2.Logger
	Clients       map[string]int //ClientID -> NodeID
	clientValueIn chan multipaxos.Value
	proposer      *multipaxos.Proposer
	responseIn    chan multipaxos.Response
	network       *network2.Network
}

func NewClientHandler(id int, net *network2.Network, prop *multipaxos.Proposer, ld detector.LeaderDetector, log *logger2.Logger) *ClientHandler {
	return &ClientHandler{
		Id:            id,
		ld:            ld,
		leader:        ld.Leader(),
		L:             log,
		Clients:       map[string]int{},
		clientValueIn: make(chan multipaxos.Value, 8),
		proposer:      prop,
		responseIn:    make(chan multipaxos.Response, 8),
		network:       net,
	}
}

func (h *ClientHandler) Start() {
	trustMsgs := h.ld.Subscribe()
	go func() {
		for {
			select {
			case clientValue := <-h.clientValueIn:
				fmt.Printf("\nClient value in: %v", clientValue)
				if h.Id != h.leader {
					netID := h.Clients[clientValue.ClientID]
					h.network.SChan <- network2.Message{
						To:     netID,
						From:   h.Id,
						Type:   "redirect",
						Leader: h.leader,
					}
					//OBS: delete client from Clients as I have redirected it
					delete(h.Clients, clientValue.ClientID)
					h.L.Log(logger2.LogMessage{D: "system", S: "debug", M: fmt.Sprintf("#Client Handler: Got value: %v, but is not the leader. Sent a redirect and deleted the client from h.Clients", clientValue)})
				}
				h.proposer.DeliverClientValue(clientValue)
			case response := <-h.responseIn: //from bank manager
				if netID, ok := h.Clients[response.ClientID]; ok == true {
					responseMsg := network2.Message{
						To:       netID,
						From:     h.Id,
						Type:     "response",
						Response: response,
					}
					h.network.SChan <- responseMsg
				}

			case clientInfo := <-h.network.ClientHandlerChan:
				if clientInfo.Action == "add" {
					h.Clients[clientInfo.ClientID] = clientInfo.NetID
				} else if clientInfo.Action == "remove" {
					delete(h.Clients, clientInfo.ClientID)
				}
				h.L.Log(logger2.LogMessage{D: "system", S: "debug", M: fmt.Sprintf("#Client Handler: Got client info: %v, h.Clients: %v", clientInfo, h.Clients)})
			case newLeader := <-trustMsgs:
				h.L.Log(logger2.LogMessage{D: "system", S: "debug", M: fmt.Sprintf("#Client Handler: Got trust message. Old leader: %v, New leader: %v", h.leader, newLeader)})
				h.leader = newLeader
				if h.leader != h.Id {
					for _, netID := range h.Clients {
						h.network.SChan <- network2.Message{
							To:     netID,
							From:   h.Id,
							Type:   "redirect",
							Leader: h.leader,
						}
					}
					//OBS: reset Clients as we have redirected all of them
					h.Clients = map[string]int{}
				}
			}
		}
	}()
}

func (h *ClientHandler) DeliverClientValue(val multipaxos.Value) {
	h.clientValueIn <- val
}

func (h *ClientHandler) DeliverResponse(res multipaxos.Response) {
	h.responseIn <- res
}

/*
Client Handling Module:
x 1. Each Paxos node should have a module for receiving client requests and sending client responses. Specification:

2. The client handling module should accept connections from clients. A client should exchange their identifier with the Paxos
	node so that a node can use the id to store and lookup the connection. See also the description of the client application in the next subsection.

3. A client should be redirected if it connect or send a request to a non-leader node. A node should respond with a <REDIRECT, addr> message that indicate
	which node it consider as the current leader.

4. A client response, as described in the previous subsection, should be of type Response found in defs.go from the multipaxos package.

5. Upon receiving a Response value from the server module, the client handling module should attempt to send the response to the client if it is connected.
	It can ignore any errors resulting from trying to send the response to the client.
*/
