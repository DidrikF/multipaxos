package fakeclienthandler

import (
	"fmt"
	"strconv"

	"github.com/uis-dat520-s18/glabs/grouplab1/detector"
	"github.com/uis-dat520-s18/glabs/grouplab3/fakenet"
	"github.com/uis-dat520-s18/glabs/grouplab3/multipaxos"
	"github.com/uis-dat520-s18/glabs/grouplab3/network2"
)

type ClientHandler struct {
	Id            int
	ld            detector.LeaderDetector
	leader        int
	Clients       map[string]int
	clientValueIn chan multipaxos.Value
	proposer      *multipaxos.Proposer
	responseIn    chan multipaxos.Response
	network       *fakenet.Network
}

func NewClientHandler(id int, net *fakenet.Network, prop *multipaxos.Proposer, ld detector.LeaderDetector) *ClientHandler {
	return &ClientHandler{
		Id:            id,
		ld:            ld,
		leader:        ld.Leader(),
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
				fmt.Println("\nclientHandler will deliver value to proposer if it is the leader")
				fmt.Println("\nLeader in clientHandler: " + strconv.Itoa(h.leader) + ", ID of clienthandler: " + strconv.Itoa(h.Id))
				if h.Id != h.leader {
					netID := h.Clients[clientValue.ClientID]
					h.network.SChan <- network2.Message{
						To:     netID,
						From:   h.Id,
						Type:   "redirect",
						Leader: h.leader,
					}
					//OBS: delete client from Clients as I have redirected it
				}
				h.proposer.DeliverClientValue(clientValue)
			case response := <-h.responseIn:
				responseMsg := network2.Message{
					To:       42,
					From:     h.Id,
					Type:     "response",
					Response: response,
				}
				h.network.SChan <- responseMsg
				/*
					if netID, ok := h.Clients[response.ClientID]; ok == true {
						responseMsg := network2.Message{
							To:       netID,
							From:     h.Id,
							Type:     "response",
							Response: response,
						}
						h.network.SChan <- responseMsg
					}
				*/

			case clientInfo := <-h.network.ClientHandlerChan: //NO ClientInfo messages will come here now
				if clientInfo.Action == "add" {
					h.Clients[clientInfo.ClientID] = clientInfo.NetID
				} else if clientInfo.Action == "remove" {
					delete(h.Clients, clientInfo.ClientID)
				}
			case newLeader := <-trustMsgs:
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
