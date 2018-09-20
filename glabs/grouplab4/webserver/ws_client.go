package webserver

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/uis-dat520-s18/glabs/grouplab4/logger4"

	"github.com/uis-dat520-s18/glabs/grouplab4/bankmanager4"
	"github.com/uis-dat520-s18/glabs/grouplab4/multipaxos4"

	"github.com/gorilla/websocket"
	"github.com/uis-dat520-s18/glabs/grouplab4/bank4"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(req *http.Request) bool {
		return true
	},
}

type Client struct {
	bm        *bankmanager4.Bankmanager
	L         *logger4.Logger
	webServer *WebServer
	conn      *websocket.Conn
	Send      chan Message
	user      bank4.User
	clientID  string
}

type HistoryReponse struct {
	Values             []multipaxos4.Value       `json:"values"`
	TransactionResults []bank4.TransactionResult `json:"transactionResults"`
}

func (c *Client) reader() {
	defer func() {
		c.L.Log(logger4.LogMessage{O: "client", D: "system", S: "debug", M: fmt.Sprintf("reader of client %v is exiting", c.clientID)})
		c.webServer.unregister <- c
		close(c.Send) // panic due to closing of closed channel
		c.conn.Close()
		c.webServer.websocketwg.Done()
	}()
	var message Message

	for {
		var data json.RawMessage
		message = Message{
			Data: &data,
		}
		err := c.conn.ReadJSON(&message)
		if err != nil {
			c.L.Log(logger4.LogMessage{O: "client", D: "system", S: "debug", M: fmt.Sprintf("conn.ReadJSON had an error:  %s", err)})

			/* Maybe just ignore this type of failure.
			errorResponse := Message{
				Type:  "error",
				Error: []string{"Failed to decode message from web socket"},
			}
			c.Send <- errorResponse // dont know if this message gets sent before closing the network connection
			*/
			return
		}

		// c.L.Log(logger4.LogMessage{O: "client", D: "system", S: "debug", M: fmt.Sprintf("Got message and decoded it into: %+v", message)})

		// Message Handling:
		switch message.Type {
		case "connect":
			var clientID string
			err := json.Unmarshal(data, &clientID)
			if err != nil {
				errorResponse := Message{
					Type:  "error",
					Error: []string{"Failed to connect to web socket server with 'connect' message, could not decode clientID"},
				}
				c.Send <- errorResponse
				continue
			}
			// c.L.Log(logger4.LogMessage{O: "client", D: "system", S: "debug", M: fmt.Sprintf("Handling 'connect' message, and decoded data to: %+v", clientID)})

			c.clientID = clientID
			c.user = message.Auth // set it, but there is no auth, so not safe
			c.webServer.register <- c

			response := Message{
				Type:   "connect",
				Status: 200,
				Data:   c.webServer.leader, // concurrent read of leader
			}
			// c.L.Log(logger4.LogMessage{O: "client", D: "system", S: "debug", M: fmt.Sprintf("Handling 'connect' message, and are finally sending response: %+v", response)})

			c.Send <- response
		case "register":
			value := multipaxos4.Value{}
			response := Message{
				Type: "register",
			}
			err := json.Unmarshal(data, &value) // NO BUENO
			if err != nil {
				response.Status = 400
				response.Error = []string{"Failed to decode value in register request", err.Error()}

				c.Send <- response
				continue
			}

			// c.L.Log(logger4.LogMessage{O: "client", D: "system", S: "debug", M: fmt.Sprintf("Handling 'register' message, and decoded data to: %+v", value)})

			if len(value.User.Username) < 4 {
				response.Error = append(response.Error, fmt.Sprintf("Username '%v' is to short, it needs to be at least 4 characters long", value.User.Username))
			}
			if len(value.User.Password) < 4 {
				response.Error = append(response.Error, "Password was too short, it needs to be at least 4 characters long")
			}

			if taken := c.bm.IsUsernameTaken(value.User.Username); taken == true {
				response.Error = append(response.Error, fmt.Sprintf("Username '%v' is allready taken", value.User.Username))
			}

			if len(response.Error) > 0 {
				response.Status = 400
				c.Send <- response
				continue
			}

			//c.L.Log(logger4.LogMessage{O: "client", D: "system", S: "debug", M: fmt.Sprintf("Handling 'register' message, and are finally passing the value to the proposer")})

			c.webServer.valueIn <- value
		case "login":
			response := Message{
				Type: "login",
			}
			if authenticated := c.bm.AuthenticateUser(message.Auth); authenticated != true {
				response.Status = 401
				response.Error = []string{"Failed to connect to web socket server with 'connect' message, user authentication failed"}
				c.Send <- response
				continue
			}
			c.user = message.Auth
			c.webServer.register <- c
			response.Status = 200
			c.Send <- response

		case "transaction":
			response := Message{
				Type: "transaction",
			}
			var value multipaxos4.Value
			err := json.Unmarshal(data, &value)
			if err != nil {
				response.Status = 400
				response.Error = []string{"Failed to decode value in transaction request"}
				c.Send <- response
				continue
			}
			// authorize user
			if authorized := c.bm.AuthorizeUser(value.AccountNum, message.Auth); authorized != true {
				response.Status = 403
				response.Error = []string{fmt.Sprintf("User %s is not authorized to access account %d", message.Auth.Username, value.AccountNum)}
				c.Send <- response
				continue
			}

			// remember that the value needs to go through PAXOS, the response is dealt with when the response comes for the bankmanager.
			c.webServer.valueIn <- value

		case "create-account":
			response := Message{
				Type: "create-account",
			}
			var value multipaxos4.Value
			err := json.Unmarshal(data, &value)
			if err != nil {
				response.Status = 400
				response.Error = []string{"Failed to decode value in 'create-account' request"}
				c.Send <- response
				continue
			}
			// authorize user (meaning in this case that the user exists)
			if authorized := c.bm.AuthenticateUser(message.Auth); authorized != true {
				response.Status = 403
				response.Error = []string{fmt.Sprintf("User %s, is not authorized to create account", message.Auth.Username)}
				c.Send <- response
				continue
			}

			// Send value to proposer
			c.webServer.valueIn <- value

		case "account-status":
			response := Message{
				Type: "account-status",
			}

			usersAccounts := c.bm.GetUsersAccounts(message.Auth)

			response.Status = 200
			response.Data = usersAccounts
			c.Send <- response

		case "account-history":
			response := Message{
				Type: "account-history",
			}

			transactionResults := c.bm.GetUsersAccountHistories(message.Auth)

			response.Status = 200
			response.Data = transactionResults

			c.Send <- response

		case "status":
			accounts := c.bm.GetAllAccountStatuses()
			response := Message{
				Type:   "status",
				Data:   accounts,
				Status: 200,
			}

			c.Send <- response
		case "history":
			values, transactionResults := c.bm.GetAllAccountHistories()

			historyResponse := HistoryReponse{
				Values:             values,
				TransactionResults: transactionResults,
			}

			response := Message{
				Type:   "history",
				Data:   historyResponse,
				Status: 200,
			}

			c.Send <- response
		default:
			response := Message{
				Type:   "error",
				Status: 400,
				Error:  []string{fmt.Sprintf("Unknown request type: %s", message.Type)},
			}
			c.Send <- response
		}
	}
}

func (c *Client) writer() {
	defer func() {
		c.L.Log(logger4.LogMessage{O: "client", D: "system", S: "debug", M: fmt.Sprintf("writer of client %v is exiting", c.clientID)})
		c.webServer.unregister <- c
		c.conn.Close()
		c.webServer.websocketwg.Done()
	}()
	for {
		message, ok := <-c.Send
		if ok != true {
			c.conn.WriteMessage(websocket.CloseMessage, []byte{})
			return
		}

		err := c.conn.WriteJSON(message)
		if err != nil {
			c.L.Log(logger4.LogMessage{O: "client", D: "system", S: "err", M: fmt.Sprintf("Failed to write json to websocket. Message was: %v", message)})
		}
		// c.L.Log(logger4.LogMessage{O: "client", D: "system", S: "debug", M: fmt.Sprintf("Wrote message to webScoket! Message: %v", message)})

	}
}

// serveWs handles websocket requests from the peer.
func serveWs(webServer *WebServer, bankmanager *bankmanager4.Bankmanager, logger *logger4.Logger, wr http.ResponseWriter, req *http.Request) {
	conn, err := upgrader.Upgrade(wr, req, nil)
	if err != nil {
		fmt.Printf("Failed to upgrade to WebSocket with error: %v", err.Error())
		return
	}

	client := &Client{webServer: webServer, bm: bankmanager, L: logger, conn: conn, Send: make(chan Message)}

	client.webServer.register <- client

	// Allow collection of memory referenced by the caller by doing all work in
	// new goroutines.
	webServer.websocketwg.Add(1)
	go client.reader()
	webServer.websocketwg.Add(1)
	go client.writer()
}
