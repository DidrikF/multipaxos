package network2

import (
	"bufio"
	"encoding/gob"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/uis-dat520-s18/glabs/grouplab3/logger2"
	"github.com/uis-dat520-s18/glabs/grouplab3/multipaxos"
)

type ConfigInstance struct {
	self struct {
		Id         int
		Ip         string
		ServerPort int
	}
	Nodes []struct {
		Id         int
		Ip         string
		ServerPort int
	}
}

type Config struct {
	Development ConfigInstance
	Production  ConfigInstance
}

type Message struct {
	//Cant find any way to simplify this.
	To       int
	From     int
	Type     string
	ClientID string
	Leader   int
	Request  string
	Value    multipaxos.Value
	Response multipaxos.Response
	Prepare  multipaxos.Prepare
	Promise  multipaxos.Promise
	Accept   multipaxos.Accept
	Learn    multipaxos.Learn
	Svalue   Svalue
}

type Svalue struct {
	ClientID   string
	ClientSeq  int
	Noop       bool
	AccountNum int
	Txn        Stransaction
}

type Stransaction struct {
	Op     string
	Amount int
}

type SimpleMessage struct {
	To      int
	From    int
	Type    string
	Payload interface{}
}

type Node struct {
	// If I have nodes advertice their identity, I dont need to know to map ip/ports. I could force a strict order, but I dont like that.
	Id   int
	Addr *net.TCPAddr
}

type Network struct {
	//with new requiremetns I may need to add something, but also look for things to take out/make simpler
	Self              Node
	Nodes             []Node
	Connections       map[int]net.Conn
	Config            ConfigInstance
	SChan             chan Message
	RChan             chan Message
	ServerListener    *net.TCPListener //need reference to do clean up, I think...
	ClientIDChan      chan string      //dont know exacly how this should be yet
	L                 *logger2.Logger
	ClientHandlerChan chan ClientInfo
}

type ClientInfo struct {
	Action   string
	NetID    int
	ClientID string
}

//Metex locks around shared state:
var netMutex sync.Mutex

//Use a work group to know when no more connections are open? I dont think so

func CreateNetwork(config ConfigInstance, selfID int, log *logger2.Logger) (*Network, error) {
	sChan := make(chan Message, 32) //here you send messages out on the network
	rChan := make(chan Message, 32) //here you get messages from the network
	network := &Network{
		Config:            config,
		SChan:             sChan,
		RChan:             rChan,
		Nodes:             []Node{},
		Connections:       map[int]net.Conn{},
		L:                 log,
		ClientHandlerChan: make(chan ClientInfo, 8),
	}

	for _, node := range config.Nodes {
		address := node.Ip + ":" + strconv.Itoa(node.ServerPort)
		tcpAddr, err := net.ResolveTCPAddr("tcp", address)
		if err != nil {
			network.L.Log(logger2.LogMessage{D: "system", S: "debug", M: "#Network: Failed to resolve TCPAddr for address: " + address + ", with error: " + err.Error()})
			continue
		}
		if node.Id == selfID {
			network.Self = Node{
				Id:   node.Id,
				Addr: tcpAddr,
			}
		} else {
			network.Nodes = append(network.Nodes, Node{
				Id:   node.Id,
				Addr: tcpAddr,
			})
		}
	}
	if network.Self.Id != selfID {
		return &Network{}, errors.New("Failed to set self from configuration. Can not proceed with creation of Network")
	}

	return network, nil
}

func (n *Network) StartNetwork() error {
	fmt.Println("Started network!!!")
	err := n.StartServer()
	if err != nil {
		return err
	}
	for _, node := range n.Nodes {
		err := n.ConnectToNode(node)
		if err != nil {
			fmt.Println("connectToNode failed with error: " + err.Error())
			n.L.Log(logger2.LogMessage{D: "system", S: "debug", M: "#Network: Failed to connect to peer when establishing peer2peer network. Error: " + err.Error()})
		}
	}

	//Listen on n.SChan for messages to send over the network to the node specified in Message.To
	go func() {
		fmt.Println("Started send go routine")
		for {
			message := <-n.SChan //The applications entry point into the network
			err := n.Send(message)
			if err != nil {
				n.L.Log(logger2.LogMessage{D: "system", S: "debug", M: "#Network: Failed to send message - Error: " + err.Error()})
				continue
			}
			if message.Type != "heartbeat" {
				n.L.Log(logger2.LogMessage{D: "network", S: "debug", M: fmt.Sprintf("Sent message: %+v", n.MakeSimpleMessage(message))})
			}
			//n.L.Log(logger2.LogMessage{D: "network", S: "debug", M: fmt.Sprintf("Sent message: %+v", n.MakeSimpleMessage(message))})
		}
	}()

	/*
		go func() {
			for {
				<-time.After(5 * time.Second)
				n.PurgeAndAttemptReconnect()
			}
		}()


		//maybe have a command that cuases network to attempt reconnect, to avoid restart.
			go func() {
				for {
					<-time.After(5 * time.Second)
					nodesIdsToSendConnectMessageTo := []int{}
					netMutex.Lock()
					for nodeId, _ := range n.Connections {
						nodesIdsToSendConnectMessageTo = append(nodesIdsToSendConnectMessageTo, nodeId)
					}
					netMutex.Unlock()
					for _, nodeId := range nodesIdsToSendConnectMessageTo {
						msg := Message{
							To:   nodeId,
							From: n.Self.Id,
							Type: "connect",
						}
						n.SChan <- msg
					}
				}
			}()
	*/

	return nil
}

func (n *Network) ConnectToNode(node Node) error {
	if n.Connections[node.Id] != nil {
		return errors.New("Allready connected to node " + strconv.Itoa(node.Id))
	}
	useAddr, err := n.getClientAddr()
	if err != nil {
		return err
	}

	tcpConn, err := net.DialTCP("tcp", useAddr, node.Addr)
	if err != nil {
		return errors.New("Dial failed to node " + strconv.Itoa(node.Id) + " with error: " + err.Error())
	}
	n.L.Log(logger2.LogMessage{D: "system", S: "debug", M: "#Network: Successfully connected to Node " + strconv.Itoa(node.Id)})
	//tcpConn.SetKeepAlive(true)
	netMutex.Lock()
	n.Connections[node.Id] = tcpConn
	netMutex.Unlock()

	connectMsg := Message{
		To:   node.Id,
		From: n.Self.Id,
		Type: "connect",
	}

	err = n.Send(connectMsg) //OBS: maybe I sould retry ???
	if err != nil {
		n.L.Log(logger2.LogMessage{D: "system", S: "err", M: "#Network: Failed to send 'connect' message to node " + strconv.Itoa(node.Id)})
	}

	//one goroutine per tcpConn
	go n.ReadFromConn(tcpConn) //pass by value

	return nil
}

func (n *Network) StartServer() error {
	listener, err := net.ListenTCP("tcp", n.Self.Addr)
	if err != nil {
		return errors.New("Failed to make listener with error: " + err.Error())
	}
	n.ServerListener = listener

	go func(listener *net.TCPListener) {
		defer listener.Close()
		n.L.Log(logger2.LogMessage{D: "system", S: "debug", M: "#Network: Server running, waiting to accept new connections..."})
		for {
			conn, err := listener.AcceptTCP()
			if err != nil {
				n.L.Log(logger2.LogMessage{D: "system", S: "debug", M: "#Network: Server listener.Accept() error: " + err.Error()})
				break
			}
			//conn.SetKeepAlive(true)
			n.L.Log(logger2.LogMessage{D: "system", S: "debug", M: "#Network: Accepted connection from unknown it. The remote node ID must be advertised in a 'connect' message."})
			go n.ReadFromConn(conn)
		}
	}(listener)

	return nil
}

func (n *Network) ReadFromConn(tcpConn net.Conn) { //passed by value
	rw := bufio.NewReadWriter(bufio.NewReader(tcpConn), bufio.NewWriter(tcpConn))
	remoteNodeID := -1
	defer n.cleanupConn(tcpConn, &remoteNodeID) //OBS: does this work?
	var message Message

	for {
		//message = Message{}
		decoder := gob.NewDecoder(rw)
		err := decoder.Decode(&message) // <-- blocking
		switch {
		case err == io.EOF:
			n.L.Log(logger2.LogMessage{D: "system", S: "err", M: "#Network: Reached EOF - close this connection. Error: " + err.Error()})
			return
		case err != nil:
			n.L.Log(logger2.LogMessage{D: "system", S: "err", M: "#Network: Failed to decode message from tcpConn, exiting goroutine and cleaning up connection. The error from the gob decoder: " + err.Error()})
			//n.Logger.LogChan <- logger2.LogMessage{D: "system", S: "err", M: "#Network: " + fmt.Sprintf("%+v", err)}
			n.L.Log(logger2.LogMessage{D: "system", S: "err", M: "#Network: " + fmt.Sprintf("Message that cause an error: %+v", message)})
			return
		}

		if message.Type == "connect" {
			remoteNodeID = message.From
			netMutex.Lock()
			n.Connections[message.From] = tcpConn //OBS: should tcpConn be a pointer: NO, could cause nil pointer trouble
			n.L.Log(logger2.LogMessage{D: "system", S: "debug", M: "A connect message received. Connection map: " + fmt.Sprintf("%+v", n.Connections)})
			netMutex.Unlock()
			if message.ClientID != "" {
				defer func() {
					n.ClientHandlerChan <- ClientInfo{
						Action:   "remove",
						NetID:    message.From,
						ClientID: message.ClientID,
					}
				}()
				n.ClientHandlerChan <- ClientInfo{
					Action:   "add",
					NetID:    message.From,
					ClientID: message.ClientID,
				}
			}
			continue
		}

		/*
			if message.Type == "value" {
				var operation bank.Operation
				switch message.Svalue.Txn.Op {
				case "balance":
					operation = 0
				case "deposit":
					operation = 1
				case "withdraw":
					operation = 2
				}
				message.Value = multipaxos.Value{
					ClientID:   message.Svalue.ClientID,
					ClientSeq:  message.Svalue.ClientSeq,
					Noop:       message.Svalue.Noop,
					AccountNum: message.Svalue.AccountNum,
					Txn: bank.Transaction{
						Op:     operation,
						Amount: message.Svalue.Txn.Amount,
					},
				}
			}*/

		//Testing response timeout on client
		/*
			if message.Type == "value" {
				continue
			}
		*/

		//n.Logger.LogChan <- logger2.LogMessage{D: "system", S: "debug", M: "#Network: SUCCESS, the buf is: "+string(buf[0:])}
		if message.Type != "heartbeat" {
			n.L.Log(logger2.LogMessage{D: "network", S: "debug", M: "#Network: Got message: " + fmt.Sprintf("%+v", n.MakeSimpleMessage(message))})
		}
		//n.L.Log(logger2.LogMessage{D: "network", S: "debug", M: "#Network: Got message: " + fmt.Sprintf("%+v", n.MakeSimpleMessage(message))})
		//n.Logger.LogChan <- logger2.LogMessage{D: "system", S: "debug", M: "Network: n.Connections: "+fmt.Sprintf("%+v", n.Connections)}
		n.RChan <- message
	}
}

func (n *Network) Send(message Message) error { //OBS: Should I do some clean up here if you cannot sent to a client, I want to keep the client handler in sync and remove dead connections as soon as possible
	//sendMutex.Lock()
	//defer sendMutex.Unlock()
	if message.To == n.Self.Id {
		if message.Type != "heartbeat" {
			n.L.Log(logger2.LogMessage{D: "network", S: "debug", M: "#Network: Got message: " + fmt.Sprintf("%+v", n.MakeSimpleMessage(message))})
		}
		n.RChan <- message
		return nil
	}
	//get connection if available
	netMutex.Lock()
	receiverConn := n.Connections[message.To] //if receiverConn, foundIt := ...; foundIt { ... }
	netMutex.Unlock()
	if receiverConn == nil {
		return errors.New("Connection to node: " + strconv.Itoa(message.To) + " is not present in n.Connections. Aborting sending of message.")
	}

	if message.Type == "redirect" {
		defer func() {
			n.cleanupConn(receiverConn, &message.To)
		}()
	}

	rw := bufio.NewReadWriter(bufio.NewReader(receiverConn), bufio.NewWriter(receiverConn))
	encoder := gob.NewEncoder(rw) // Will write to network.
	err := encoder.Encode(message)
	if err != nil {
		return errors.New("Failed to gob encode/send message to node: " + strconv.Itoa(message.To) + ". Aborting sending of message.")
	}
	err = rw.Flush()
	if err != nil {
		return errors.New("Flush failed. error: " + err.Error())
	}

	return nil
}

func (n *Network) Broadcast(message Message) error {
	netMutex.Lock()
	for _, node := range n.Nodes {
		message.To = node.Id
		n.SChan <- message
	}
	netMutex.Unlock()
	return nil
}

func (n *Network) BroadcastTo(nodeIds []int, message Message) error { //use this most of the time
	for _, nodeId := range nodeIds {
		message.To = nodeId
		n.SChan <- message
	}
	return nil
}

func (n *Network) BroadcastToConnected(message Message) error {
	netMutex.Lock()
	for nodeId, _ := range n.Connections {
		message.To = nodeId
		n.SChan <- message
	}
	netMutex.Unlock()
	return nil
}

func (n *Network) getClientAddr() (*net.TCPAddr, error) {
	port, err := getFreePort() //OBS: maybe I should retry?
	if err != nil {
		return nil, errors.New("Failed to get free port with error: " + err.Error())
	}

	addr, err := net.ResolveTCPAddr("tcp", n.Self.Addr.IP.String()+":"+strconv.Itoa(port))
	if err != nil {
		return nil, err
	}
	return addr, nil
}

func (n *Network) PurgeAndAttemptReconnect() {
	n.Purge()
	nodesToConnectTo := []Node{}
	netMutex.Lock()
	for _, node := range n.Nodes {
		if n.Connections[node.Id] != nil {
			continue
		}
		nodesToConnectTo = append(nodesToConnectTo, node)
	}
	netMutex.Unlock()
	for _, node := range nodesToConnectTo {
		err := n.ConnectToNode(node)
		if err != nil {
			n.L.Log(logger2.LogMessage{D: "system", S: "debug", M: "Reconnect failed to node " + strconv.Itoa(node.Id) + " with error: " + err.Error()})
		}
	}
}

//Purge closes and removes inactive connections from n.Connections.
func (n *Network) Purge() {
	netMutex.Lock()
	for key, conn := range n.Connections {
		if IsConnClosed(conn) == true {
			conn.Close()
			delete(n.Connections, key)
		}
	}
	netMutex.Unlock()
}

func (n *Network) cleanupConn(tcpConn net.Conn, remoteNodeID *int) {
	netMutex.Lock()
	if *remoteNodeID > -1 {
		delete(n.Connections, *remoteNodeID)
	}
	netMutex.Unlock()
	tcpConn.Close()

	n.L.Log(logger2.LogMessage{D: "system", S: "debug", M: "Network: n.Connections after a cleanup of a conn: " + fmt.Sprintf("%+v", n.Connections)})
}

func (n *Network) CleanupNetwork() error {
	for _, conn := range n.Connections {
		conn.Close()
	}
	n.ServerListener.Close() //returns an error
	netMutex.Lock()
	n.Connections = map[int]net.Conn{}
	n.ServerListener = nil
	netMutex.Unlock()
	return nil
}

func (n *Network) MakeSimpleMessage(message Message) SimpleMessage {
	simpleMessage := SimpleMessage{To: message.To, From: message.From, Type: message.Type}
	switch message.Type {
	case "value":
		simpleMessage.Payload = message.Value
	case "promise":
		simpleMessage.Payload = message.Promise
	case "accept":
		simpleMessage.Payload = message.Accept
	case "prepare":
		simpleMessage.Payload = message.Prepare
	case "response":
		simpleMessage.Payload = message.Response
	case "learn":
		simpleMessage.Payload = message.Learn
	case "heartbeat":
		simpleMessage.Payload = message.Request
	default:
		simpleMessage.Payload = false
	}

	return simpleMessage
}

//Helper functions
func IsConnClosed(c net.Conn) bool {
	one := make([]byte, 1)
	c.SetReadDeadline(time.Time{})
	_, err := c.Read(one)
	if err == io.EOF {
		return true
	}
	var zero time.Time
	c.SetReadDeadline(zero) //Make future reads not time out
	return false
}

// GetFreePort Gets an available port by asking the kernal for a random port
// ready and available for use.
func getFreePort() (int, error) {
	addr, err := net.ResolveTCPAddr("tcp", "localhost:0")
	if err != nil {
		return 0, err
	}

	l, err := net.ListenTCP("tcp", addr)
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

/*
	for {
		port, err := getFreePort()
		if err != nil {
			n.Logger.Log("system", "notice", "Network: Failed to get free port with error: "+err.Error())

		} else {
			break
		}
	}

			//err = json.Unmarshal(buf[0:num], &message)
		//if err != nil {
		//n.Logger.Log("system", "err", "#Network: Failed to UNMARSHAL message. Error: "+err.Error())
		//n.Logger.Log("system", "err", "#Network: The buf was: "+string(buf[0:]))
		//n.Logger.Log("system", "err", "#Network: The message became: "+fmt.Sprintf("%+v", message))
		//}

*/
