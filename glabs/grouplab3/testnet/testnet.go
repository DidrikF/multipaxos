package testnet

import (
	"bufio"
	"encoding/gob"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"sync"

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
	Value    multipaxos.Value
	Response multipaxos.Response
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
	ClientHandlerChan chan ClientInfo
	port              int
}

type ClientInfo struct {
	Action   string
	NetID    int
	ClientID string
}

//Metex locks around shared state:
var netMutex sync.Mutex

//Use a work group to know when no more connections are open? I dont think so

func CreateNetwork(config ConfigInstance, selfID int) (*Network, error) {
	sChan := make(chan Message, 32) //here you send messages out on the network
	rChan := make(chan Message, 32) //here you get messages from the network
	network := &Network{
		Config:            config,
		SChan:             sChan,
		RChan:             rChan,
		Nodes:             []Node{},
		Connections:       map[int]net.Conn{},
		ClientHandlerChan: make(chan ClientInfo, 8),
		port:              62000, //62000
	}

	for _, node := range config.Nodes {
		address := node.Ip + ":" + strconv.Itoa(node.ServerPort)
		tcpAddr, err := net.ResolveTCPAddr("tcp", address)
		if err != nil {
			fmt.Println("#Network: Failed to resolve TCPAddr for address")
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
	err := n.StartServer()
	if err != nil {
		return err
	}
	for _, node := range n.Nodes {
		err := n.ConnectToNode(node)
		if err != nil {
			fmt.Printf("\nFailed to connect to node %v, with error: %v", node, err.Error())
		}
	}

	//Listen on n.SChan for messages to send over the network to the node specified in Message.To
	go func() {
		for {
			message := <-n.SChan //The applications entry point into the network
			err := n.Send(message)
			if err != nil {
				fmt.Println("#Network: Failed to send message - Error: " + err.Error())
			}
		}
	}()

	return nil
}

func (n *Network) ConnectToNode(node Node) error {
	netMutex.Lock()
	if n.Connections[node.Id] != nil {
		return errors.New("Allready connected to node " + strconv.Itoa(node.Id))
	}
	netMutex.Unlock()
	useAddr, err := net.ResolveTCPAddr("tcp", "localhost:"+strconv.Itoa(n.port))
	if err != nil {
		return err
	}
	if err != nil {
		return err
	}

	tcpConn, err := net.DialTCP("tcp", useAddr, node.Addr)
	if err != nil {
		return errors.New("Dial failed to node " + strconv.Itoa(node.Id) + " with error: " + err.Error())
	}
	fmt.Println("Connected successfully to node")

	netMutex.Lock()
	n.port++
	n.Connections[node.Id] = tcpConn
	netMutex.Unlock()

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
		fmt.Println("Ready to accept clients...")
		defer listener.Close()
		for {
			conn, err := listener.Accept()
			if err != nil {
				fmt.Println("Failed to accept a connection with error: " + err.Error())
				break
			}
			go n.ReadFromConn(conn)
		}
	}(listener)

	return nil
}

func (n *Network) ReadFromConn(tcpConn net.Conn) { //passed by value
	fmt.Println("Ready to read from connection")
	rw := bufio.NewReadWriter(bufio.NewReader(tcpConn), bufio.NewWriter(tcpConn))
	remoteNodeID := -1
	defer n.cleanupConn(tcpConn, &remoteNodeID) //OBS: does this work?

	var message Message
	for {
		decoder := gob.NewDecoder(rw)
		err := decoder.Decode(&message)
		switch {
		case err == io.EOF:
			fmt.Println("Reached EOF - close this connection.\n   ---")
			return
		case err != nil:
			fmt.Println("\nError decoding: " + err.Error())
			return
		}
		fmt.Printf("Got message (before Rchan): %v", message)
		n.RChan <- message
	}
}

func (n *Network) Send(message Message) error { //OBS: Should I do some clean up here if you cannot sent to a client, I want to keep the client handler in sync and remove dead connections as soon as possible
	//sendMutex.Lock()
	//defer sendMutex.Unlock()

	//get connection if available
	netMutex.Lock()
	receiverConn := n.Connections[message.To] //if receiverConn, foundIt := ...; foundIt { ... }
	netMutex.Unlock()
	if receiverConn == nil {
		return errors.New("Connection to node: " + strconv.Itoa(message.To) + " is not present in n.Connections. Aborting sending of message.")
	}

	rw := bufio.NewReadWriter(bufio.NewReader(receiverConn), bufio.NewWriter(receiverConn))

	for i := 0; i < 1000; i++ {
		enc := gob.NewEncoder(rw) // Will write to network.
		err := enc.Encode(message)
		if err != nil {
			return errors.New("Failed to gob encode/send message to node: " + strconv.Itoa(message.To) + ". Error: " + err.Error())
		}

		err = rw.Flush()
		if err != nil {
			return errors.New("Flush failed. error: " + err.Error())
		}

		fmt.Println("Successfully sent message")
	}

	return nil
}

func (n *Network) BroadcastTo(nodeIds []int, message Message) error { //use this most of the time
	for _, nodeId := range nodeIds {
		message.To = nodeId
		n.SChan <- message
	}
	return nil
}

func (n *Network) cleanupConn(tcpConn net.Conn, remoteNodeID *int) {
	netMutex.Lock()
	if *remoteNodeID > -1 {
		delete(n.Connections, *remoteNodeID)
	}
	netMutex.Unlock()
	tcpConn.Close()

	fmt.Println("Network: n.Connections after a cleanup of a conn: " + fmt.Sprintf("%+v", n.Connections))
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
