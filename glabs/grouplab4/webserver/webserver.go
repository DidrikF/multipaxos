package webserver

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/uis-dat520-s18/glabs/grouplab4/bank4"
	"github.com/uis-dat520-s18/glabs/grouplab4/bankmanager4"
	"github.com/uis-dat520-s18/glabs/grouplab4/detector4"

	"github.com/uis-dat520-s18/glabs/grouplab4/logger4"
	"github.com/uis-dat520-s18/glabs/grouplab4/multipaxos4"
)

type WebServer struct {
	Id          int
	leader      int
	bm          *bankmanager4.Bankmanager
	L           *logger4.Logger
	ld          detector4.LeaderDetector
	proposer    *multipaxos4.Proposer
	websrv      *http.Server
	webserverwg *sync.WaitGroup
	stop        chan struct{}

	websocketwg   sync.WaitGroup
	clients       []*Client
	responseIn    chan multipaxos4.Response //for å takle forbindelse fra bank
	valueIn       chan multipaxos4.Value
	unregister    chan *Client
	register      chan *Client
	webClientChan chan logger4.LogMessage
	Crypt         bool
}

//Metex locks around shared state:
var webMutex sync.Mutex

//
func NewWebServer(id int, bm *bankmanager4.Bankmanager, proposer *multipaxos4.Proposer, ld detector4.LeaderDetector, logger *logger4.Logger, wg *sync.WaitGroup, webClientChan chan logger4.LogMessage, crypt bool) *WebServer {
	return &WebServer{
		Id:          id,
		leader:      ld.Leader(),
		bm:          bm,
		ld:          ld,
		L:           logger,
		proposer:    proposer,
		webserverwg: wg,
		stop:        make(chan struct{}),

		websocketwg: sync.WaitGroup{},
		clients:     []*Client{},
		responseIn:  make(chan multipaxos4.Response), // response from learner responseOut
		valueIn:     make(chan multipaxos4.Value),    // values from web socket clients
		unregister:  make(chan *Client),
		register:    make(chan *Client),

		webClientChan: webClientChan,
		Crypt:         crypt,
	}
}

// Define our message object DETTE SKAL BORT
type Message struct {
	Type   string      `json:"type"`
	Data   interface{} `json:"data"`
	Auth   bank4.User  `json:"auth"`
	Status int         `json:"status"`
	Error  []string    `json:"error"`
}

type NodeLogMessage struct {
	Id      int                `json:"id"`
	Message logger4.LogMessage `json:"message"`
}

/*
type Response struct {
	Data  string   `json:"data"`
	Error []string `json:"error"`
}
*/

func (w *WebServer) startHttpServer() *http.Server {
	port := ":" + strconv.Itoa(8000+w.Id)
	srv := &http.Server{Addr: port}

	// Create a simple file server
	fs := http.FileServer(http.Dir("../webclient")) // ../webclient/public, but then I need to reorder files (not optimal when working with webpack-dev-server)
	http.Handle("/", fs)

	http.HandleFunc("/bank", w.serveBank) // not donig anything atm, maybe not care about authentacation for getting the bank application.

	// Configure from http to a websocket
	http.HandleFunc("/ws", func(wr http.ResponseWriter, req *http.Request) {
		serveWs(w, w.bm, w.L, wr, req)
	})

	w.webserverwg.Add(1)
	go func() {
		defer func() {
			w.L.Log(logger4.LogMessage{O: "webServer", D: "system", S: "debug", M: fmt.Sprintf("Closing web server ListenAndServe go-routine")})
			w.webserverwg.Done()
		}()
		//if !w.Crypt {
		if err := srv.ListenAndServe(); err != nil {
			// cannot panic, because this probably is an intentional close
			w.L.Log(logger4.LogMessage{O: "webServer", D: "system", S: "debug", M: fmt.Sprintf("error from ListenAndServe() (probably intentional): %s", err)})
			return
		}
		/*} else {
			fmt.Printf("Secure server\n")
			if err := srv.ListenAndServeTLS("../certs/server.crt", "../certs/server.key"); err != nil {
				// cannot panic, because this probably is an intentional close
				fmt.Printf("%v", err)
				w.L.Log(logger4.LogMessage{O: "webServer", D: "system", S: "debug", M: fmt.Sprintf("error from ListenAndServeTLS() (probably intentional): %s", err)})
				return
			}
		}*/

	}()

	// returning reference so caller can call Shutdown()
	return srv
}

func (w *WebServer) Start() {

	// Start the server on localhost port 8000 and log any errors
	w.websrv = w.startHttpServer()

	// Manager of Web Sockets
	w.webserverwg.Add(1)
	go w.handleClients()
}

// The Web Socket Client Manager
func (w *WebServer) handleClients() {
	defer func() {
		w.L.Log(logger4.LogMessage{O: "webServer", D: "system", S: "debug", M: fmt.Sprintf("Closing handleClients go-routine")})
		w.webserverwg.Done()
	}()

	trustMsgs := w.ld.Subscribe()
	for {
		select {
		// Regiter and unregister clients
		// The register channel allows you to send the same client multiple times, updating the entry when an old one arrives (conenct message -> updating User and ClientID)
		case client := <-w.register: // pointer to client
			// Attempt to update
			updated := false
			for index, c := range w.clients {
				if c == client {
					// update
					w.clients = append(w.clients[:index], append([]*Client{client}, w.clients[index+1:]...)...)
					updated = true
					break
				}
			}
			if updated == true {
				continue
			}
			// Client was not present from before, so we add it
			w.clients = append(w.clients, client)

		case client := <-w.unregister: // pointer to client
			for index, c := range w.clients {
				if c == client {
					w.L.Log(logger4.LogMessage{O: "webServer", D: "system", S: "debug", M: fmt.Sprintf("Unregistrering client: %v", client.clientID)})
					w.clients = append(w.clients[:index], w.clients[index+1:]...)
				}
			}

		case value := <-w.valueIn:
			w.proposer.DeliverClientValue(value)

		case response := <-w.responseIn: //from bank manager
			if w.leader != w.Id {
				continue
			}

			// w.L.Log(logger4.LogMessage{O: "webServer", D: "system", S: "debug", M: fmt.Sprintf("Building message to send to client via web socket")})
			message := buildMessageFromResponse(response)

			// just for debugging purposes, may be removed when you know it works as intended.
			//connectedClients := w.GetConnectedClients()
			//w.L.Log(logger4.LogMessage{O: "webServer", D: "system", S: "debug", M: fmt.Sprintf("Current clients: %+v", connectedClients)})

			for _, client := range w.clients {
				if client.clientID == response.ClientID {
					// w.L.Log(logger4.LogMessage{O: "webServer", D: "system", S: "debug", M: fmt.Sprintf("sending message on send channel to writer method of web socket client: %+v", message)})
					client.Send <- message
				}
			}

		case newLeader := <-trustMsgs:
			// w.L.Log(logger4.LogMessage{O: "webServer", D: "system", S: "debug", M: fmt.Sprintf("Got trust message. Old leader: %v, New leader: %v", w.leader, newLeader)})
			w.leader = newLeader

			// Update connected clients about who the leader is:
			leaderUpdateMessage := Message{
				Type: "leader-update",
				Data: w.leader,
			}
			for _, client := range w.clients {
				// send to clients
				client.Send <- leaderUpdateMessage
			}
		case <-time.NewTicker(5 * time.Second).C:
			if w.leader == w.Id {
				leaderUpdateMessage := Message{
					Type: "leader-update",
					Data: w.leader,
				}
				for _, client := range w.clients {
					// send to clients
					client.Send <- leaderUpdateMessage
				}
			}
		case logMessage := <-w.webClientChan:
			message := Message{
				Type: "log-message",
				Data: NodeLogMessage{
					Id:      w.Id,
					Message: logMessage,
				},
			}
			for _, client := range w.clients {
				// send to clients
				client.Send <- message
			}
		case <-w.stop:

			return
		}

	}
}

func buildMessageFromResponse(response multipaxos4.Response) Message {
	message := Message{}
	data, _ := json.Marshal(response)

	if response.User.Username != "" {
		message.Type = "register"
		if response.Error != "" {
			message.Status = 299 // successfull paxos round, but not successful register.

		} else {
			message.Status = 201
		}

		message.Data = (json.RawMessage)(data)
		return message
	} else if response.Account.Name != "" {
		message.Type = "create-account"
		if response.Error != "" {
			message.Status = 299
		} else {
			message.Status = 201
		}

		message.Data = (json.RawMessage)(data)
		return message

	} else {
		message.Type = "transaction"
		if response.Error != "" {
			message.Status = 299
		} else {
			message.Status = 200
		}

		message.Data = (json.RawMessage)(data)
	}
	return message
}

func (w *WebServer) GetConnectedClients() string {
	var clientIDs string
	for _, client := range w.clients {
		clientIDs += fmt.Sprintf("%v, ", client.clientID)
	}
	return clientIDs
}

func (w *WebServer) GetConnectedClientsChanLength() string {
	var lengths string
	for _, client := range w.clients {
		lengths += fmt.Sprintf("%v: %v\n", client.clientID, len(client.Send))
	}
	return lengths
}

// need to send auth in the header if this is to be userfull, may not bother with this.
func (w *WebServer) serveBank(wr http.ResponseWriter, req *http.Request) {
}

func (w *WebServer) DeliverResponse(response multipaxos4.Response) {
	w.responseIn <- response
}

func (w *WebServer) Stop() {
	// Close all clients
	for _, client := range w.clients {
		client.conn.Close() // this will cause the client.reader() to fail, which in turn also stops the client.writer() using close(client.Send).
		// close events is only picked up by the reader, so when the reader fail, we need to ensure that the writer also gets closed.
	}
	w.websocketwg.Wait()
	w.L.Log(logger4.LogMessage{O: "webServer", D: "system", S: "debug", M: "Web sockets and their go routines are closed"})

	// close the web server
	w.websrv.Close() // shutdown

	// stop handleClients()
	w.stop <- struct{}{}
}
