package network4

import (
	"bufio"
	"crypto/tls"
	"crypto/x509"
	"encoding/gob"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/uis-dat520-s18/glabs/grouplab4/logger4"
	"github.com/uis-dat520-s18/glabs/grouplab4/multipaxos4"
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
	Value    multipaxos4.Value
	Response multipaxos4.Response
	Prepare  multipaxos4.Prepare
	Promise  multipaxos4.Promise
	Accept   multipaxos4.Accept
	Learn    multipaxos4.Learn
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
	Addr string
}

type Network struct {
	//with new requiremetns I may need to add something, but also look for things to take out/make simpler
	Self              Node
	Nodes             []Node
	Connections       map[int]net.Conn
	Config            ConfigInstance
	SChan             chan Message
	RChan             chan Message
	ServerListener    *net.Listener //need reference to do clean up, I think...
	ClientIDChan      chan string   //dont know exacly how this should be yet
	L                 *logger4.Logger
	ClientHandlerChan chan ClientInfo
	WG                *sync.WaitGroup
	StopMessageChan   chan struct{}
	Crypt             bool
}

type ClientInfo struct {
	Action   string
	NetID    int
	ClientID string
}

//Metex locks around shared state:
var netMutex sync.Mutex

//Use a work group to know when no more connections are open? I dont think so

func CreateNetwork(config ConfigInstance, selfID int, log *logger4.Logger, wg *sync.WaitGroup, crypt bool) (*Network, error) {
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
		StopMessageChan:   make(chan struct{}),
		WG:                wg,
		Crypt:             crypt,
	}

	for _, node := range config.Nodes {
		address := node.Ip + ":" + strconv.Itoa(node.ServerPort)
		tcpAddr := address
		/*tcpAddr, err := net.ResolveTCPAddr("tcp", address)
		if err != nil {
			network.L.Log(logger4.LogMessage{O: "network", D: "system", S: "debug", M: "Failed to resolve Addr for address: " + address + ", with error: " + err.Error()})
			continue
		}*/
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
			n.L.Log(logger4.LogMessage{O: "network", D: "system", S: "debug", M: "Failed to connect to peer when establishing peer2peer network. Error: " + err.Error()})
		}
	}

	//Listen on n.SChan for messages to send over the network to the node specified in Message.To
	n.WG.Add(1)
	go func() {
		defer n.WG.Done()
		fmt.Println("Started send go routine")
		for {
			select {
			case message := <-n.SChan: //The applications entry point into the network
				err := n.Send(message)
				if err != nil {
					n.L.Log(logger4.LogMessage{O: "network", D: "system", S: "debug", M: "Failed to send message - Error: " + err.Error()})
					continue
				}
				if message.Type != "heartbeat" {
					n.L.Log(logger4.LogMessage{O: "network", D: "network", S: "debug", M: fmt.Sprintf("Sent message: %+v", n.MakeSimpleMessage(message))})
				}
				//n.L.Log(logger4.LogMessage{D: "network", S: "debug", M: fmt.Sprintf("Sent message: %+v", n.MakeSimpleMessage(message))})
			case <-n.StopMessageChan:
				n.L.Log(logger4.LogMessage{O: "network", D: "system", S: "debug", M: "Closing SChan listener"})
				return
			}
		}
	}()

	return nil
}

func (n *Network) ConnectToNode(node Node) error {
	if n.Connections[node.Id] != nil {
		return errors.New("Allready connected to node " + strconv.Itoa(node.Id))
	}
	/*useAddr, err := n.getClientAddr()
	if err != nil {
		return err
	}*/
	tcpConn, err := n.MakeConn(node.Addr)
	if err != nil {
		return errors.New("Dial failed to node " + strconv.Itoa(node.Id) + " with error: " + err.Error())
	}
	n.L.Log(logger4.LogMessage{O: "network", D: "system", S: "debug", M: "Successfully connected to Node " + strconv.Itoa(node.Id)})
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
		n.L.Log(logger4.LogMessage{O: "network", D: "system", S: "err", M: "Failed to send 'connect' message to node " + strconv.Itoa(node.Id)})
	}

	//one goroutine per tcpConn
	n.WG.Add(1)
	go n.ReadFromConn(tcpConn) //pass by value

	return nil
}

func (n *Network) StartServer() error {
	listener, err := n.MakeListener()
	if err != nil {
		return errors.New("Failed to make listener with error: " + err.Error())
	}
	n.ServerListener = &listener

	n.WG.Add(1) //
	go func(listener net.Listener) {
		defer func() {
			// n.L.Log(logger4.LogMessage{D: "system", S: "debug", M: "#Network: Calling listener.Close() and WG.Done()"})
			listener.Close()
			n.L.Log(logger4.LogMessage{O: "network", D: "system", S: "debug", M: "Closed networkserver listener for incoming nodes"})
			n.WG.Done()
		}()

		n.L.Log(logger4.LogMessage{O: "network", D: "system", S: "debug", M: "Server running, waiting to accept new connections..."})
		for {
			conn, err := listener.Accept()
			if err != nil {
				n.L.Log(logger4.LogMessage{O: "network", D: "system", S: "debug", M: "Server listener.Accept() error: " + err.Error()})
				break
			}
			//conn.SetKeepAlive(true)
			n.L.Log(logger4.LogMessage{O: "network", D: "system", S: "debug", M: "Accepted connection from unknown it. The remote node ID must be advertised in a 'connect' message."})
			n.WG.Add(1)
			go n.ReadFromConn(conn)
		}
	}(listener)

	return nil
}
func (n *Network) MakeConn(addr string) (net.Conn, error) {
	if !n.Crypt {
		conn, err := net.Dial("tcp", addr)
		if err != nil {
			return nil, errors.New("Dial failed to node " + err.Error())
		}
		return conn, nil
	} else {
		serverCertPool := x509.NewCertPool()
		serverCertBytes, err := ioutil.ReadFile("../certs/serverCert.crt")
		if err != nil {
			return nil, errors.New("Failed to read server TLS cert/key with error: " + err.Error())
		}
		if ok := serverCertPool.AppendCertsFromPEM([]byte(serverCertBytes)); !ok {
			log.Fatalln("Unable to add server certificate to certificate pool")
		}
		cer, err := tls.LoadX509KeyPair("../certs/clientCert.crt", "../certs/clientKey.key")
		if err != nil {
			return nil, errors.New("Failed to parse own as client TLS cert/key with error: " + err.Error())
		}

		config := &tls.Config{
			InsecureSkipVerify:       true, //false //SKIPPER VERIFIKASJON...
			RootCAs:                  serverCertPool,
			MinVersion:               tls.VersionTLS12,
			Certificates:             []tls.Certificate{cer},
			PreferServerCipherSuites: true,
			CurvePreferences:         []tls.CurveID{tls.CurveP521, tls.CurveP384, tls.CurveP256},
			CipherSuites: []uint16{
				tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
				tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
				tls.TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA,
				tls.TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA,
			},
		}
		config.BuildNameToCertificate()
		conn, err := tls.Dial("tcp", addr, config)
		if err != nil {
			return nil, errors.New("Failed to TLS dial with error: " + err.Error())
		}
		return conn, nil
	}
}

func (n *Network) MakeListener() (net.Listener, error) {
	if !n.Crypt {
		listener, err := net.Listen("tcp", n.Self.Addr)
		if err != nil {
			return nil, errors.New("Failed to make listener with error: " + err.Error())
		}
		return listener, nil
	} else {
		clientCertPool := x509.NewCertPool()
		certBytes, err := ioutil.ReadFile("../certs/clientCert.crt")
		if err != nil {
			return nil, errors.New("Failed to read client TLS cert/key with error:  " + err.Error())
		}
		if ok := clientCertPool.AppendCertsFromPEM(certBytes); !ok {
			log.Fatalln("Unable to add certificate to certificate pool")
		}
		cer, err := tls.LoadX509KeyPair("../certs/serverCert.crt", "../certs/serverKey.key")
		if err != nil {
			return nil, errors.New("Failed to parse own as server TLS cert/key with error: " + err.Error())
		}
		config := &tls.Config{
			ClientAuth:               tls.RequestClientCert, //RequireAndVerifyClientCert, //DENNE SKIPPER VERIFIKASJON...
			ClientCAs:                clientCertPool,
			MinVersion:               tls.VersionTLS12,
			Certificates:             []tls.Certificate{cer},
			PreferServerCipherSuites: true,
			CurvePreferences:         []tls.CurveID{tls.CurveP521, tls.CurveP384, tls.CurveP256},
			CipherSuites: []uint16{
				tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
				tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
				tls.TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA,
				tls.TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA,
			},
		}
		config.BuildNameToCertificate()
		listener, err := tls.Listen("tcp", n.Self.Addr, config)
		if err != nil {
			return nil, errors.New("Failed to make TLS listener with error: " + err.Error())
		}
		return listener, nil
	}

}

func (n *Network) ReadFromConn(tcpConn net.Conn) { //passed by value
	remoteNodeID := -1
	defer func() {
		n.cleanupConn(tcpConn, &remoteNodeID) //OBS: does this work?
		n.L.Log(logger4.LogMessage{O: "network", D: "system", S: "debug", M: fmt.Sprintf("Closed listenerconn to %d", remoteNodeID)})
		n.WG.Done()
	}()
	rw := bufio.NewReadWriter(bufio.NewReader(tcpConn), bufio.NewWriter(tcpConn))
	var message Message
	for {
		message = Message{} // OBS, does this fix things
		decoder := gob.NewDecoder(rw)
		err := decoder.Decode(&message) // <-- blocking
		switch {
		case err == io.EOF:
			n.L.Log(logger4.LogMessage{O: "network", D: "system", S: "err", M: "Reached EOF - close this connection. Error: " + err.Error()})
			return
		case err != nil:
			n.L.Log(logger4.LogMessage{O: "network", D: "system", S: "err", M: "Failed to decode message from tcpConn, exiting goroutine and cleaning up connection. The error from the gob decoder: " + err.Error()})
			//n.Logger.LogChan <- logger4.LogMessage{D: "system", S: "err", M: "#Network: " + fmt.Sprintf("%+v", err)}
			n.L.Log(logger4.LogMessage{O: "network", D: "system", S: "err", M: fmt.Sprintf("Message that cause an error: %+v", message)})
			return
		}

		if message.Type == "test" {
			n.L.Log(logger4.LogMessage{O: "network", D: "system", S: "err", M: fmt.Sprintf("Got a test message from %v", message.From)})
			fmt.Printf("test-recieved")
			continue
		}

		if message.Type == "connect" {
			remoteNodeID = message.From
			netMutex.Lock()
			n.Connections[message.From] = tcpConn //OBS: should tcpConn be a pointer: NO, could cause nil pointer trouble
			n.L.Log(logger4.LogMessage{O: "network", D: "system", S: "debug", M: "A connect message received. Connection map: " + fmt.Sprintf("%+v", n.Connections)})
			netMutex.Unlock()
			continue
		}

		//n.Logger.LogChan <- logger4.LogMessage{D: "system", S: "debug", M: "#Network: SUCCESS, the buf is: "+string(buf[0:])}
		if message.Type != "heartbeat" {
			n.L.Log(logger4.LogMessage{O: "network", D: "network", S: "debug", M: "Got message: " + fmt.Sprintf("%+v", n.MakeSimpleMessage(message))})
		}
		//n.L.Log(logger4.LogMessage{D: "network", S: "debug", M: "#Network: Got message: " + fmt.Sprintf("%+v", n.MakeSimpleMessage(message))})
		//n.Logger.LogChan <- logger4.LogMessage{D: "system", S: "debug", M: "Network: n.Connections: "+fmt.Sprintf("%+v", n.Connections)}
		n.RChan <- message
	}
}

func (n *Network) Send(message Message) error { //OBS: Should I do some clean up here if you cannot sent to a client, I want to keep the client handler in sync and remove dead connections as soon as possible
	//sendMutex.Lock()
	//defer sendMutex.Unlock()
	if message.To == n.Self.Id {
		if message.Type != "heartbeat" {
			n.L.Log(logger4.LogMessage{O: "network", D: "network", S: "debug", M: "Got message: " + fmt.Sprintf("%+v", n.MakeSimpleMessage(message))})
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
			n.cleanupConn(receiverConn, &message.To) // does this lead to go routine stopping WG being decremented
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

// OBS: remove the methods we end up not using
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

func (n *Network) getClientAddr() (string, error) {
	port, err := getFreePort() //OBS: maybe I should retry?
	if err != nil {
		return "", errors.New("Failed to get free port with error: " + err.Error())
	}
	addr := n.Self.Addr + ":" + port
	/*
		addr, err := net.ResolveAddr("tcp", n.Self.Addr.IP.String()+":"+strconv.Itoa(port))
		if err != nil {
			return nil, err
		}
	*/
	return addr, nil
}

func (n *Network) cleanupConn(tcpConn net.Conn, remoteNodeID *int) {
	netMutex.Lock()
	if *remoteNodeID > -1 {
		delete(n.Connections, *remoteNodeID)
	}
	netMutex.Unlock()
	tcpConn.Close()

	n.L.Log(logger4.LogMessage{O: "network", D: "system", S: "debug", M: "n.Connections after a cleanup of a conn: " + fmt.Sprintf("%+v", n.Connections)})
}

func (n *Network) CleanupNetwork() error {
	for _, conn := range n.Connections {
		err := conn.Close()
		if err != nil {
			n.L.Log(logger4.LogMessage{O: "network", D: "system", S: "debug", M: "Failed to close connection with error: " + err.Error()})
		}
	}
	(*n.ServerListener).Close() //returns an error
	netMutex.Lock()
	n.Connections = map[int]net.Conn{}
	n.ServerListener = nil
	netMutex.Unlock()
	return nil
}

func (n *Network) Stop() {
	n.StopMessageChan <- struct{}{}
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
func getFreePort() (string, error) {

	/*addr, err := net.ResolveAddr("tcp", "localhost:0")
	if err != nil {
		return 0, err
	}*/

	l, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		return "", err
	}
	defer l.Close()
	l.Addr().String()
	addr := strings.SplitAfter(l.Addr().String(), ":")
	prt := addr[1]
	return prt, nil
}
