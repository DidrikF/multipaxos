package network

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/uis-dat520-s18/glabs/grouplab1/logger"
	"github.com/uis-dat520-s18/glabs/grouplab2/singlepaxos"
)

type ConfigInstance struct {
	self struct {
		Id            int
		Ip            string
		ServerPort    int
		ClientPortMin int
		ClientPortMax int
	}
	Keepalive          bool
	Timeout            int
	Transport_proto    string
	Reconnect_interval int
	Nodes              []struct {
		Id            int
		Ip            string
		ServerPort    int
		ClientPortMin int
		ClientPortMax int
	}
}

type Config struct {
	Development ConfigInstance
	Production  ConfigInstance
}

type Node struct {
	Id            int
	Addr          *net.TCPAddr
	ClientPortMin int
	ClientPortMax int
	UsedPorts     []int
}

type Network struct {
	Self             Node
	Nodes            []Node
	Connections      map[int]net.Conn
	Config           ConfigInstance
	SChan            chan Message
	RChan            chan Message
	UnmodRChan       chan Message
	ServerListener   *net.TCPListener
	Logger           logger.Logger
	AddClientConn    chan net.Conn
	RemoveClientConn chan net.Conn
}

type Message struct {
	To      int
	From    int
	Msg     string
	Type    string //not a problem to add extra fields
	Request bool
	Promise singlepaxos.Promise
	Accept  singlepaxos.Accept
	Prepare singlepaxos.Prepare
	Learn   singlepaxos.Learn
	Value   singlepaxos.Value
}

var mutex = &sync.Mutex{}
var sendMutex = &sync.Mutex{}
var resMutex = &sync.Mutex{}

//CreateNetwork creates a new Network and sets config, self and nodes.
//Once done, you can start creating connections with createPeerToPeer() and connectTo()
func CreateNetwork(config ConfigInstance, self int, mode string, log logger.Logger) (Network, error) {
	sChan := make(chan Message, 16) //here you send messages out on the network
	rChan := make(chan Message, 16) //here you get messages from the network
	addClientConn := make(chan net.Conn, 16)
	removeClientConn := make(chan net.Conn, 16)
	unmodRChan := make(chan Message, 16) //used to push on unmodified messages from the network layer's rChan to the once interested
	network := Network{
		Config:           config,
		SChan:            sChan,
		RChan:            rChan,
		UnmodRChan:       unmodRChan,
		AddClientConn:    addClientConn,
		RemoveClientConn: removeClientConn,
		Nodes:            []Node{},
		Connections:      map[int]net.Conn{},
		Logger:           log,
	}

	for _, node := range config.Nodes {
		address := node.Ip + ":" + strconv.Itoa(node.ServerPort)
		tcpAddr, err := net.ResolveTCPAddr(config.Transport_proto, address)
		if err != nil {
			//fmt.Println("Failed to resolve TCPAddr for address: " + address + " with error: " + err.Error())
			log.Log("system", "debug", "#Network: Failed to resolve TCPAddr for address: "+address+", with error: "+err.Error())
			continue
		}
		if node.Id == self {
			network.Self = Node{
				Id:            node.Id,
				Addr:          tcpAddr,
				ClientPortMin: node.ClientPortMin,
				ClientPortMax: node.ClientPortMax,
				UsedPorts:     []int{},
			}
		} else {
			network.Nodes = append(network.Nodes, Node{
				Id:            node.Id,
				Addr:          tcpAddr,
				ClientPortMin: node.ClientPortMin,
				ClientPortMax: node.ClientPortMax,
				UsedPorts:     []int{},
			})
		}
	}
	if network.Self.Id != self {
		return Network{}, errors.New("Failed to set self from configuration. Can not proceed with creation of Network")
	}

	return network, nil
}

func (n *Network) EstablishPeer2Peer() error {
	for _, node := range n.Nodes {
		err := n.ConnectToPeer(node)
		if err != nil {
			//fmt.Println(err)
			n.Logger.Log("system", "debug", "#Network: Failed to connect to peer when establishing peer2peer network. Error: "+err.Error())
		}
	}
	err := n.StartServer()
	if err != nil {
		return err
	}

	//Listen on n.SChan for messages to send over the network to the node specified in Message.To
	go func() {
		for {
			message := <-n.SChan //The applications entry point into the network
			err := n.Send(message)
			if err != nil {
				//fmt.Println(err.Error())
				n.Logger.Log("system", "debug", "#Network: Failed to send message. Message: "+fmt.Sprintf("%+v", message)+" - Error: "+err.Error())
			}
		}
	}()

	//Periodic purge and attempt reconnect to nodes
	go func() {
		for {
			<-time.After(5 * time.Second)
			n.PurgeAndAttemptReconnect()
		}
	}()

	//fail gracefully (could have log)
	return nil
}

func (n *Network) ConnectToPeer(node Node) error {
	//check if connected
	if n.Connections[node.Id] != nil {
		return errors.New("Allready connected to node " + strconv.Itoa(node.Id))
	}
	useAddr, err := n.getClientAddr()
	if err != nil {
		n.removeFromUsedPorts(useAddr)
		return err
	}
	tcpConn, err := net.DialTCP(n.Config.Transport_proto, useAddr, node.Addr)
	if err != nil {
		//tcpConn.Close() //this causes an unrecoverable error???
		n.removeFromUsedPorts(useAddr) //if I get to this point and the DailTCP() returns an error i cannot remove the port from n.Self.UsedPorts, because the port is still in use.
		return errors.New("Dial failed to node " + strconv.Itoa(node.Id) + " with error: " + err.Error())
	}
	//fmt.Println("Connected to node: " + strconv.Itoa(node.Id))
	n.Logger.Log("system", "debug", "#Network: Successfully connected to Node "+strconv.Itoa(node.Id))
	mutex.Lock()
	n.Connections[node.Id] = tcpConn
	mutex.Unlock()

	//one goroutine per tcpConn
	go n.listenForMessagesOnConn(tcpConn, node.Id)

	return nil
}

func (n *Network) StartServer() error {
	listener, err := net.ListenTCP("tcp", n.Self.Addr)
	if err != nil {
		return errors.New("Failed to make listener with error: " + err.Error())
	}
	n.ServerListener = listener

	go func(listener *net.TCPListener) {
		defer listener.Close() //WILL THIS CLOSE CORRECTLY? //put the listener on the Network struct

		for {
			//fmt.Println("#Network: Server running, waiting to accept new connections...")
			n.Logger.Log("system", "debug", "#Network: Server running, waiting to accept new connections...")
			conn, err := listener.Accept()
			if err != nil {
				//fmt.Println("Server listener.Accept() error: " + err.Error())
				n.Logger.Log("system", "debug", "#Network: Server listener.Accept() error: "+err.Error())
				break //continue
			}
			//fmt.Println("Accepted a client on the server")
			raddr := conn.RemoteAddr()
			nodeID, err := n.getNodeIdFromRemoteAddr(raddr)
			if err != nil {
				n.AddClientConn <- conn
				go n.listenOnClientConn(conn)
				continue
			}
			n.Logger.Log("system", "debug", "#Network: Accepted connection from node "+strconv.Itoa(nodeID))
			mutex.Lock()
			n.Connections[nodeID] = conn
			mutex.Unlock()
			go n.listenForMessagesOnConn(conn, nodeID)
		}
	}(listener)

	return nil
}

func (n *Network) listenOnClientConn(tcpConn net.Conn) {
	defer func() {
		n.RemoveClientConn <- tcpConn
		tcpConn.Close()
	}()

	for {
		var buf [20480]byte
		message := new(Message)
		num, err := tcpConn.Read(buf[0:])
		if err != nil {
			n.Logger.Log("system", "err", "#Network: Failed to decode message from tcpConn, exiting goroutine and cleaning up connection. The error from the json decoder: "+err.Error())
			n.Logger.Log("system", "err", "#Network: "+fmt.Sprintf("%+v", err))
			break
		}

		err = json.Unmarshal(buf[0:num], &message)
		if err != nil {
			n.Logger.Log("system", "err", "#Network: Failed to UNMARSHAL message. Error: "+err.Error())
			//n.Logger.Log("system", "err", "#Network: The buf was: "+string(buf[0:]))
			//n.Logger.Log("system", "err", "#Network: The message became: "+fmt.Sprintf("%+v", message))
		}
		n.Logger.Log("system", "debug", "#Network: SUCCESS, the buf is: "+string(buf[0:]))

		n.RChan <- *message
	}
}

func (n *Network) SendToConn(conn net.Conn, message Message) error {
	jsonByteSlice, err := json.Marshal(message)
	if err != nil {
		return errors.New("Failed to MARSHAL message to node: " + strconv.Itoa(message.To) + ". Aborting sending of message. Error: " + err.Error())
	}
	length := len(jsonByteSlice)
	_, err = conn.Write(jsonByteSlice[0:length])
	if err != nil {
		return errors.New("Failed to write/send message to node: " + strconv.Itoa(message.To) + ". Aborting sending of message.")
	}

	n.Logger.Log("network", "debug", fmt.Sprintf("Sent message: %+v", message))
	return nil
}

func (n *Network) listenForMessagesOnConn(tcpConn net.Conn, nodeID int) {
	defer n.cleanupConn(tcpConn, nodeID) //Assuming that this work! but we also have a purge and attempt reconnect.

	for {

		//THE DECODER STILL FAIL SOME TIMES!! WHY?? it also causes the connection to be dropped!

		//decoder := json.NewDecoder(tcpConn) //WHAT HAPPENS WHEN THIS TCPCONN IS CLOSED? WILL THIS GOROUTINE CLOSE SUCCESSFULLY
		//resMutex.Lock()
		var buf [20480]byte
		message := new(Message) //META MESSAGE that can be used with all messages sent by the system
		//decoder := json.NewDecoder(tcpConn)
		//err := decoder.Decode(&message) //"use of closed network connection" returned as error when tring to decode from a closed connection!!!

		//I also get "extra data in buffer" from the Decode call (if I dont create a new decoder for every itteration)

		num, err := tcpConn.Read(buf[0:])

		if err != nil {
			n.Logger.Log("system", "err", "#Network: Failed to decode message from tcpConn, exiting goroutine and cleaning up connection. The error from the json decoder: "+err.Error())
			n.Logger.Log("system", "err", "#Network: "+fmt.Sprintf("%+v", err))
			//n.Logger.Log("system", "err", "#Network: "+fmt.Sprintf("%+v", message))
			break //end goroutine/function which calls cleanupConn()

			//This is not covering every case I want to end the goroutine and connection...
			//continue
			if err == io.EOF {
				break
			}

		}
		//HACK
		/*
			arr := strings.Split(string(buf[:num]), "")
			if arr[0] != "{" {
				arr = append([]string{"{"}, arr...)
			}
			newBuf := []byte(strings.Join(arr, ""))
		*/
		err = json.Unmarshal(buf[0:num], &message)
		if err != nil {
			n.Logger.Log("system", "err", "#Network: Failed to UNMARSHAL message. Error: "+err.Error())
			//n.Logger.Log("system", "err", "#Network: The buf was: "+string(buf[0:]))
			//n.Logger.Log("system", "err", "#Network: The message became: "+fmt.Sprintf("%+v", message))
		}
		n.Logger.Log("system", "debug", "#Network: SUCCESS, the buf is: "+string(buf[0:]))

		/*
			msgSlice := strings.Split(string(buf[:num]), ",")
			to, _ := strconv.Atoi(msgSlice[0])
			from, _ := strconv.Atoi(msgSlice[1])
			req, _ := strconv.ParseBool(msgSlice[2])
			message := Message{
				To:      to,
				From:    from,
				Request: req,
				Type:    msgSlice[3],
			}
		*/
		n.RChan <- *message //this channel is accessible for anyone who have access to the network instance.
		//resMutex.Unlock()
	}
}

func (n *Network) Send(message Message) error { //passed by value!
	//You cannot send messages to yourself
	sendMutex.Lock()
	defer sendMutex.Unlock()
	if message.To == n.Self.Id {
		n.RChan <- message
		return nil
	}
	//get connection if available
	receiverConn := n.Connections[message.To] //if receiverConn, foundIt := ...; foundIt { ... }
	if receiverConn == nil {
		return errors.New("Connection to node: " + strconv.Itoa(message.To) + " is not present in n.Connections. Aborting sending of message.")
	}
	/*
		encoder := json.NewEncoder(receiverConn) //pass by value
		err := encoder.Encode(message)
		if err != nil {
			return errors.New("Failed to encode/send message to node: " + strconv.Itoa(message.To) + ". Aborting sending of message.")
		}
	*/

	jsonByteSlice, err := json.Marshal(message)
	//fmt.Printf("Marshaled JSON: " + string(jsonByteSlice) + "\n")
	if err != nil {
		return errors.New("Failed to MARSHAL message to node: " + strconv.Itoa(message.To) + ". Aborting sending of message. Error: " + err.Error())
	}

	//stringMsg := strconv.Itoa(message.To) + "," + strconv.Itoa(message.From) + "," + strconv.FormatBool(message.Request) + "," + message.Type
	length := len(jsonByteSlice)
	_, err = receiverConn.Write(jsonByteSlice[0:length]) // []byte(stringMsg
	if err != nil {
		return errors.New("Failed to write/send message to node: " + strconv.Itoa(message.To) + ". Aborting sending of message.")
	}

	n.Logger.Log("network", "debug", fmt.Sprintf("Sent message: %+v", message))
	return nil
}

func (n *Network) Broadcast(message Message) error {
	for _, node := range n.Nodes {
		message.To = node.Id
		err := n.Send(message)
		if err != nil {
			n.Logger.Log("system", "notice", fmt.Sprintf("#Network: %+v", err))
		}
	}
	return nil
}

func (n *Network) BroadcastTo(nodeIds []int, message Message) error {
	for _, id := range nodeIds {
		message.To = id
		err := n.Send(message)
		if err != nil {
			n.Logger.Log("system", "notice", fmt.Sprintf("#Network: %+v", err))
		}
	}
	return nil
}

func (n *Network) cleanupConn(tcpConn net.Conn, nodeID int) {
	networkString := tcpConn.LocalAddr().String()
	split := strings.Split(networkString, ":")
	localPort, _ := strconv.Atoi(split[len(split)-1])
	mutex.Lock()
	delete(n.Connections, nodeID)
	for i, usedPort := range n.Self.UsedPorts {
		if localPort == usedPort {
			n.Self.UsedPorts = append(n.Self.UsedPorts[:i], n.Self.UsedPorts[i+1:]...)
		}
	}
	mutex.Unlock()
	tcpConn.Close()
}

func (n *Network) getClientAddr() (*net.TCPAddr, error) {
	usePort := -1
	for i := n.Self.ClientPortMin; i <= n.Self.ClientPortMax; i++ {
		inUsedPorts := false
		for _, port := range n.Self.UsedPorts {
			if port == i {
				inUsedPorts = true
			}
		}
		if inUsedPorts == false {
			usePort = i
			mutex.Lock()
			n.Self.UsedPorts = append(n.Self.UsedPorts, i)
			mutex.Unlock()
			break
		}
	}

	if usePort > -1 {
		useAddr, err := net.ResolveTCPAddr("tcp", n.Self.Addr.IP.String()+":"+strconv.Itoa(usePort))
		if err != nil {
			return nil, err
		}
		return useAddr, nil
	}
	return nil, errors.New("No available client port")
}

func (n *Network) getNodeIdFromRemoteAddr(raddr net.Addr) (int, error) {
	split := strings.Split(raddr.String(), ":")
	remotePort, err := strconv.Atoi(split[len(split)-1])
	if err != nil {
		return -1, errors.New("Failed to identify nodeID from raddr")
	}

	for _, node := range n.Nodes {
		if remotePort >= node.ClientPortMin && remotePort <= node.ClientPortMax {
			return node.Id, nil
		}
	}

	return -1, errors.New("Failed to get nodeId from rAddr port")
}

func (n *Network) PurgeAndAttemptReconnect() {
	n.Purge()
	for _, node := range n.Nodes {

		if n.Connections[node.Id] != nil { //dont attepmt to connect to someone that we are connected to
			continue
		}

		err := n.ConnectToPeer(node)
		if err != nil {
			//fmt.Println("Reconnect failed to node ", node.Id, " with error: ", err.Error())
			n.Logger.Log("system", "debug", "Reconnect failed to node "+strconv.Itoa(node.Id)+" with error: "+err.Error())
		}
	}
}

//Purge closes and removes inactive connections from n.Connections.
func (n *Network) Purge() {
	//check connections
	for key, conn := range n.Connections {
		if IsConnClosed(conn) == true {
			n.removeFromUsedPorts(conn.LocalAddr())
			conn.Close()
			mutex.Lock()
			delete(n.Connections, key)
			mutex.Unlock()
		}
	}
}

func (n *Network) removeFromUsedPorts(addr net.Addr) error { //Currenly not returning errors
	networkString := addr.String()
	split := strings.Split(networkString, ":")
	portToRemove, _ := strconv.Atoi(split[len(split)-1])
	for i, usedPort := range n.Self.UsedPorts {
		if portToRemove == usedPort {
			mutex.Lock()
			n.Self.UsedPorts = append(n.Self.UsedPorts[:i], n.Self.UsedPorts[i+1:]...)
			mutex.Unlock()
		}
	}
	return nil
}

func (n *Network) CleanupNetwork() error {
	for _, conn := range n.Connections {
		n.removeFromUsedPorts(conn.LocalAddr())
		conn.Close() //returns an error
	}
	n.ServerListener.Close() //returns an error
	mutex.Lock()
	n.Connections = map[int]net.Conn{}
	n.ServerListener = nil
	mutex.Unlock()

	return nil
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

/* NOTES:
On timeout/close etc. remove from network.connections

*/

/* OTHER FUNCTION TO IMPLEMENT:
func (n *Network) GetConnection(nodeId int) (net.TCPConn, error) {

}

func (n *Network) GetLiveConnections() (map[int]net.Conn, error) {

}

func validIP4(ipAddress string) bool {
	ipAddress = strings.Trim(ipAddress, " ")

	re, _ := regexp.Compile(`^(([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])\.){3}([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])$`)
	if re.MatchString(ipAddress) {
		return true
	}
	return false
}
*/
