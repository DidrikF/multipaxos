package main

import (
	"bufio"
	"encoding/gob"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"math"
	"math/rand"
	"net"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/uis-dat520-s18/glabs/grouplab3/bank"
	"github.com/uis-dat520-s18/glabs/grouplab3/logger2"
	"github.com/uis-dat520-s18/glabs/grouplab3/multipaxos"
	"github.com/uis-dat520-s18/glabs/grouplab3/network2"
)

var (
	help = flag.Bool(
		"help",
		false,
		"Show usage help",
	)
	self = flag.Int(
		"self",
		-1,
		"The network id used in the 'From' field of network messages",
	)
	severity = flag.String(
		"severity",
		"debug",
		"The minimum severity level of messages to log. Options: emerg, alert, crit, err, warning, notice, info and debug",
	)
	loud = flag.Bool(
		"loud",
		true,
		"Whether to print log messages to stdout",
	)
	netonly = flag.Bool(
		"netonly",
		false,
		"Whether to only print network messages, not system log",
	)
	save = flag.Bool(
		"save",
		true,
		"Whether to save log messages to file",
	)
	ip = flag.String(
		"ip",
		"localhost",
		"What IP address to to dial from",
	)
	nodes = flag.String(
		"nodes",
		"1,2,3,4",
		"The node ids of the systems nodes",
	)
	mode = flag.String(
		"mode",
		"development",
		"Whether to start the application in production or development mode",
	)
	benchmark = flag.Bool(
		"benchmark",
		false,
		"Whether to run in bechmark mode",
	)
	randoms = flag.Int(
		"randoms",
		500,
		"Specify number of random request in bechmark mode",
	)
	rChan         = make(chan network2.Message, 16)
	sChan         = make(chan network2.Message, 16)
	reconnectChan = make(chan struct{})
	connection    Connection
	nodesMap      = map[int]Node{}
	logger        *logger2.Logger
	leader        = -1
	clientID      string
	operations    = map[string]int{
		"balance":  1,
		"deposit":  2,
		"withdraw": 3,
	}
	sequence        = 0
	seqRespondedTo  = 0
	valueBuffer     = []multipaxos.Value{}
	responsesGotten = []multipaxos.Response{}
	responseTimer   *time.Ticker
	timerchan       = make(chan int)
	nodeIDs         []int
)

var mu sync.Mutex

//_____________TODO______________________
//RECONNECT TO ANOTHER NODE SCENARIOS (Redirect (X), timout (X), EOF (X), err (X)) OBS

//something wrong with setting the leader on the client.

/*
Redirect on error/EOF (TEST)
Causes chaos when no one to connect to (DONT SEE WHY)
	Works sometimes - needs more testing

 -	Setting who is leader and updating this on the client

Multiple clients

Look over the use of mutexes around shared data (client, bankmanager and clienthandler especially)

*/

type Node struct {
	IP    string
	Port  int
	RAddr *net.TCPAddr
}

type Connection struct {
	To   int
	Conn *net.TCPConn
}

func usage() {
	fmt.Fprintf(os.Stderr, "Usage: %s [OPTIONS]\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "\nOptions:\n")
	flag.PrintDefaults()
}

func main() {
	flag.Usage = usage
	flag.Parse()
	if *help {
		flag.Usage()
		os.Exit(0)
	}
	responseTimer = time.NewTicker(5 * time.Second)
	responseTimer.Stop()
	var wg sync.WaitGroup
	osSignalChan := make(chan os.Signal, 1)
	signal.Notify(osSignalChan, os.Interrupt)

	//get configuration from json
	config, err := loadconfig()
	if err != nil {
		fmt.Println("Failed loading config with error: ", err)
		os.Exit(0)
	}

	nodeIDs = stringToSliceOfInts(*nodes)
	err = buildNodesMap(nodeIDs, config)
	if err != nil {
		fmt.Println("Failed building nodesMap with error: ", err)
		os.Exit(0)
	}

	//_____Init Logger _______
	logger, err = logger2.CreateAndStartLogger(*self, "./logs", "systemLog", "networkLog", *severity, *loud, *save, *netonly, &wg) //func CreateLogger(logDir string, systemLog string, networkLog string, severity string, loud bool) (Logger, error) {
	if err != nil {
		fmt.Println("Creating logger failed with error: ", err.Error())
		logger.Stop()
		wg.Wait()
		os.Exit(0)
	}

	logger.Log(logger2.LogMessage{D: "system", S: "debug", M: "#Application: Started at " + time.Now().Format(time.RFC3339)})
	logger.Log(logger2.LogMessage{D: "network", S: "debug", M: "#Application: Started at " + time.Now().Format(time.RFC3339)})

	//OBS: Create random client ID
	clientID = generateRandomString(10)

	// Connect to node
	err = attemptConnectToBank(nodeIDs)
	if err != nil {
		logger.Log(logger2.LogMessage{D: "system", S: "debug", M: "#Application: Failed to connect to bank with error: " + err.Error()})
	}

	//___________Command line input loops___________

	//___Manual/Normal Client mode____
	if *benchmark == false {
		go func() {
			scanner := bufio.NewScanner(os.Stdin)
			for {
				fmt.Println("Enter command: ")
				scanner.Scan()
				input := scanner.Text()
				//look for special commands:
				runes := []rune(input)
				firstCharacter := string(runes[0])
				if firstCharacter == "#" {
					input = string(runes[1:])
					switch input {
					case "quit":
						fmt.Println("Quitting...")
						break
					case "reconnect":
						err = attemptConnectToBank(nodeIDs)
						if err != nil {
							logger.Log(logger2.LogMessage{D: "system", S: "debug", M: "#Application: Failed to connect to bank with error: " + err.Error()})
							break
						}
						fmt.Printf("\nConnected to node: " + strconv.Itoa(connection.To))
					case "reconnect-leader":
						//connect to leader if any
						if leader < 0 {
							fmt.Println("Error: Don't know who the leader is, aborting reconnect to leader")
							break
						}
						fmt.Println("Client: Attempting to connect to the leader -> node " + strconv.Itoa(leader))
						err := connectToLeader()
						if err != nil {
							fmt.Println("Error: " + err.Error())
							break
						}
						fmt.Println("\nClient: Connected to leader! ")
					case "status":
						fmt.Printf("Status Report:\n - Network Status: %v\n - Current Leader: %v\n - Current ClientSeq: %v\n - Sequence responded to: %v\n - ClientID: %v\n - Time waited for response: %v\n", connection, leader, sequence, seqRespondedTo, clientID, "NOT IMPLEMENTED")
					case "history":
						fmt.Printf("Value and Response History:\n - Value Buffer: %v\n - Responses Gotten: %v\n", valueBuffer, responsesGotten)
					}
					continue
				}

				//parse input
				value, err := parseInputToValue(input)
				if err != nil {
					fmt.Println("Error: " + err.Error())
					continue
				}
				handleSendingOfValues(value)
			}
		}()
	} else if *benchmark == true {

		//___Benchmark mode____
		go func() {
			responsetimes := []int{}
			vals := []multipaxos.Value{}
			for sequence < *randoms { //Check -1
				input := randomRequest()
				value, err := parseInputToValue(input)
				if err != nil {
					fmt.Println("Error: " + err.Error())
					continue
				}
				vals = append(vals, value)
			}
			handleSendingOfValues(vals...)

			//Gather data from response times
			for seqRespondedTo < sequence-1 {
				starttime := time.Now() // nano seconds in int64
				_ = <-timerchan
				responsetimes = append(responsetimes, int(time.Since(starttime)/time.Microsecond)) //OBS
			}

			responsetimes = MergeSort(responsetimes)
			sum := 0
			for _, time := range responsetimes {
				sum = sum + time
			}
			mean := sum / len(responsetimes)
			minimum := responsetimes[0]
			maximum := responsetimes[len(responsetimes)-1]
			var median int
			if len(responsetimes)%2 != 0 {
				median = responsetimes[(len(responsetimes)+1)/2]
			} else {
				median = (responsetimes[len(responsetimes)/2] + responsetimes[(len(responsetimes)+2)/2]) / 2
			}

			var sumSquaredDifference float64
			for _, time := range responsetimes {
				sumSquaredDifference += math.Pow(float64(time-mean), 2)
			}
			sampleVariance := sumSquaredDifference / float64(len(responsetimes)-1)
			sampleStdev := int(math.Sqrt(sampleVariance))
			fmt.Printf("\n#RTT statistics from %v transactions:\nMean: %vµs\nMin: %vµs\nMax: %vµs\nMedian: %vµs\nSDeviation: %vµs\n", *randoms, mean, minimum, maximum, median, sampleStdev)
			//fmt.Printf("TIems: %v\n", responsetimes)
		}()
	}

	//_____Message handling_____
	for {
		select {
		case msg := <-rChan:
			//fmt.Printf("\nGot message: %+v", msg)
			switch msg.Type {
			case "redirect":
				fmt.Println("Client: Got redirected to node " + strconv.Itoa(msg.Leader))
				leader = msg.Leader
				err := connectToLeader()
				if err != nil {
					fmt.Println("Error: " + err.Error())
					break
				}
				fmt.Println("Client: Connected to new leader!")
				handleSendingOfValues()
			case "response":
				responseTimer.Stop()
				if *benchmark == true {
					timerchan <- 1
				}
				response := msg.Response
				fmt.Printf("\n#Client: Got response to ClientSeq %v - Transaction Result: %+v\n", response.ClientSeq, response.TxnRes.String())
				responsesGotten = append(responsesGotten, response)
				// remove values that has gotten a response from the value buffer
				for i, value := range valueBuffer {
					if response.ClientSeq == value.ClientSeq {
						valueBuffer = append(valueBuffer[:i], valueBuffer[i+1:]...)
					}
				}
				seqRespondedTo++
				handleSendingOfValues()
			}
		case <-responseTimer.C: //Client value request timed out
			fmt.Println("Error: value-request timed out, response was not received in time. Attempting to reconnect to the bank.")
			err = attemptConnectToBank(nodeIDs)
			if err != nil {
				logger.Log(logger2.LogMessage{D: "system", S: "debug", M: "#Application: Failed to connect to bank with error: " + err.Error()})
				break
			}
			fmt.Printf("Connected to node: " + strconv.Itoa(connection.To) + "\n")
		case <-reconnectChan:
			err := attemptConnectToBank(nodeIDs)
			if err != nil {
				logger.Log(logger2.LogMessage{D: "system", S: "debug", M: "#Application: Failed to connect to bank with error: " + err.Error()})
			}
		case msg := <-sChan:
			//fmt.Printf("#Client: Sending message: %+v\n", msg)
			//fmt.Printf("#Client: Sending message...\nEnter text: ")
			err := send(msg)
			if err != nil {
				fmt.Println("Error: " + err.Error())
			}
		case <-osSignalChan:
			fmt.Println("#Application: Exiting...")
			fmt.Println("#Application: Cleaning up network layer...")
			cleanUpConn()
			fmt.Println("#Application: Exiting!")
			os.Exit(0)
		}
	}

}

func attemptConnectToBank(nodeIDs []int) error {
	//clean up
	leader = -1
	if connection.Conn != nil {
		cleanUpConn()
	}

	//get new local address
	lAddr, err := getLocalAddress()
	if err != nil {
		return errors.New("Failed to resolve a local net.TCPAddr with error: " + err.Error())
	}
	//attempt to connect to a node one by one
	for index := range nodeIDs { //OBS: TEST REDIRECT!
		id := nodeIDs[len(nodeIDs)-1-index]
		rAddr := nodesMap[id].RAddr
		tcpConn, err := net.DialTCP("tcp", lAddr, rAddr)
		if err != nil {
			fmt.Printf("#Error: Dial failed with error: %v\n", err.Error()) //OBS: log instead
			continue
		}
		//tcpConn.SetKeepAlive(true)

		connection = Connection{To: id, Conn: tcpConn}
		sChan <- network2.Message{
			To:       connection.To,
			From:     *self,
			Type:     "connect",
			ClientID: clientID,
		}
		go listenOnConn(tcpConn)
		break
	}
	if connection.Conn == nil {
		return errors.New("AttemptConnectToBank Failed to connect to any node")
	}
	return nil
}

func connectToLeader() error { //should be safe to call multiple times
	if leader < 0 {
		return errors.New("don't know who the leader is, aborting connecting to leader")
	}
	if connection.Conn != nil {
		if connection.To == leader {
			return errors.New("Already connected to the leader, which is node: " + strconv.Itoa(leader))
		}
		cleanUpConn()
	}
	lAddr, err := getLocalAddress()
	if err != nil {
		return errors.New("Failed to resolve a local net.TCPAddr with error: " + err.Error())
	}
	rAddr := nodesMap[leader].RAddr
	tcpConn, err := net.DialTCP("tcp", lAddr, rAddr)
	if err != nil {
		fmt.Printf("#Error: Dial failed with error: %v\n", err.Error())
		return errors.New("Failed to dial leader, with error: " + err.Error())
	}
	//tcpConn.SetKeepAlive(true)

	connection = Connection{To: leader, Conn: tcpConn}

	go listenOnConn(tcpConn)

	sChan <- network2.Message{
		To:       connection.To,
		From:     *self,
		Type:     "connect",
		ClientID: clientID,
	}

	return nil
}

func listenOnConn(tcpConn *net.TCPConn) {
	defer func() {
		mu.Lock()
		if connection.To == leader {
			leader = -1
		}
		connection = Connection{}
		mu.Unlock()
		(*tcpConn).Close()
		reconnectChan <- struct{}{}
	}()

	rw := bufio.NewReadWriter(bufio.NewReader(tcpConn), bufio.NewWriter(tcpConn))

	for {
		decoder := gob.NewDecoder(rw)
		var message network2.Message

		err := decoder.Decode(&message)
		switch {
		case err == io.EOF:
			logger.Log(logger2.LogMessage{D: "system", S: "err", M: "#Network: Reached EOF - close this connection. Error: " + err.Error()})
			return
		case err != nil:
			logger.Log(logger2.LogMessage{D: "system", S: "err", M: "#Network: Failed to decode message from tcpConn, exiting goroutine and cleaning up connection. The error from the gob decoder: " + err.Error()})
			//n.Logger.LogChan <- logger2.LogMessage{D: "system", S: "err", M: "#Network: " + fmt.Sprintf("%+v", err)}
			logger.Log(logger2.LogMessage{D: "system", S: "err", M: "#Network: " + fmt.Sprintf("Message that cause an error: %+v", message)})
			return
		}

		if message.Type == "response" {
			leader = message.From
		}
		//n.Logger.LogChan <- logger2.LogMessage{D: "system", S: "debug", M: "#Network: SUCCESS, the buf is: "+string(buf[0:])}
		logger.Log(logger2.LogMessage{D: "network", S: "debug", M: "#Network: Got message: " + fmt.Sprintf("%+v", makeSimpleMessage(message))})
		//n.Logger.LogChan <- logger2.LogMessage{D: "system", S: "debug", M: "Network: n.Connections: "+fmt.Sprintf("%+v", n.Connections)}

		rChan <- message
	}
}

//deposit 100 to account 42
//withdraw 50 from account 62
//balance of account 42
func parseInputToValue(input string) (val multipaxos.Value, err error) {
	mu.Lock()
	defer mu.Unlock()
	inputSlice := strings.Split(input, " ")
	account, err := strconv.ParseInt(inputSlice[len(inputSlice)-1], 10, 0)
	if err != nil {
		return val, errors.New("Account number needs to be an int, " + inputSlice[len(inputSlice)-1] + " was given.")
	}
	if op, ok := operations[inputSlice[0]]; ok == true {
		if op == 1 { //balance
			sequence++
			return multipaxos.Value{
				ClientID:   clientID,
				ClientSeq:  sequence,
				Noop:       "false",
				AccountNum: int(account),
				Txn: bank.Transaction{
					Op:     bank.Operation(op),
					Amount: 0,
				},
			}, nil
		} else if op == 2 { //deposit
			amount, err := strconv.ParseInt(inputSlice[1], 10, 0)
			if err != nil {
				return val, errors.New("Amount needs to be an int, " + inputSlice[1] + " was given.")
			}
			sequence++
			return multipaxos.Value{
				ClientID:   clientID,
				ClientSeq:  sequence,
				Noop:       "false",
				AccountNum: int(account),
				Txn: bank.Transaction{
					Op:     bank.Operation(op),
					Amount: int(amount),
				},
			}, nil
		} else if op == 3 { //withdrawel
			amount, err := strconv.ParseInt(inputSlice[1], 10, 0)
			if err != nil {
				return val, errors.New("Amount needs to be an int, " + inputSlice[1] + " was given.")
			}
			sequence++
			return multipaxos.Value{
				ClientID:   clientID,
				ClientSeq:  sequence,
				Noop:       "false",
				AccountNum: int(account),
				Txn: bank.Transaction{
					Op:     bank.Operation(op),
					Amount: int(amount),
				},
			}, nil
		}
	}
	return val, errors.New("Unkown operation " + inputSlice[0])
}

func handleSendingOfValues(values ...multipaxos.Value) { //Wait for response before new request:
	mu.Lock()
	defer mu.Unlock()

	// Debugging info_______
	/*
		vals := ""
		for index, val := range values {
			vals += fmt.Sprintf("%v: %v\n", index, val)
		}
		valueBuf := ""
		for index, val := range valueBuffer {
			valueBuf += fmt.Sprintf("%v: %v\n", index, val)
		}
		fmt.Printf("#Client: Handle sending of value.\n - ClientSeq: %v\n - Sequence Responded to: %v\n - New values:\n%v\n - Value buffer:\n%v", sequence, seqRespondedTo, vals, valueBuf)
	*/
	//_________

	msgSent := false
	//loop through values passed as arguments and buffer or send
	if len(values) > 0 {
		for _, val := range values {
			if val.ClientSeq > seqRespondedTo { //buffer also the Value about to get sent
				valueBuffer = append(valueBuffer, val)
			}
			if val.ClientSeq == seqRespondedTo+1 {
				msg := network2.Message{
					To:    connection.To,
					From:  *self,
					Type:  "value",
					Value: val, //val
				}
				fmt.Printf("putting value message on SChan in handleSendingOfValue() - value: %v\n", val) //correct value message

				sChan <- msg
				responseTimer = time.NewTicker(5 * time.Second)
				msgSent = true
			}
		}
	}
	if msgSent == true {
		return
	}

	//loop through buffered values and send value if its predecessor has been responded to
	for index, val := range valueBuffer {
		if val.ClientSeq == seqRespondedTo+1 {
			msg := network2.Message{
				To:    connection.To,
				From:  *self,
				Type:  "value",
				Value: val, //val
			}
			fmt.Printf("putting value message on SChan in handleSendingOfValue() - value: %v\n", val)
			sChan <- msg
			responseTimer = time.NewTicker(10 * time.Second)
			valueBuffer = append(valueBuffer[:index], valueBuffer[index+1:]...)
			break
		}
	}
}

func send(message network2.Message) error {
	mu.Lock()
	if connection.Conn == nil || connection.To != message.To {
		return errors.New("Do not have a connection to node: " + strconv.Itoa(message.To) + ", aborting sending of message.")
	}
	rw := bufio.NewReadWriter(bufio.NewReader(connection.Conn), bufio.NewWriter(connection.Conn))
	mu.Unlock()
	encoder := gob.NewEncoder(rw)
	err := encoder.Encode(message)
	if err != nil {
		return errors.New("Failed to gob encode/send message to node: " + strconv.Itoa(message.To) + ". Aborting sending of message.")
	}
	err = rw.Flush()
	if err != nil {
		return errors.New("Flush failed. error: " + err.Error())
	}

	//OBS: log the message
	logger.Log(logger2.LogMessage{D: "network", S: "debug", M: fmt.Sprintf("Sent message: %+v", makeSimpleMessage(message))})
	return nil
}

func cleanUpConn() {
	if connection.Conn != nil {
		connection.Conn.Close() //should end the listeningOnConn go routine
	}
	connection = Connection{}
}

func loadconfig() (network2.ConfigInstance, error) {
	file, err := os.Open("../config.json")
	defer file.Close()
	if err != nil {
		return network2.ConfigInstance{}, err
	}
	decoder := json.NewDecoder(file)
	config := network2.Config{}
	err = decoder.Decode(&config)
	if err != nil {
		return network2.ConfigInstance{}, err
	}
	var conf network2.ConfigInstance
	if *mode == "production" {
		conf = config.Production
	} else {
		conf = config.Development
	}
	return conf, nil
}

func buildNodesMap(nodeIDs []int, config network2.ConfigInstance) error {
	for _, id := range nodeIDs {
		for _, node := range config.Nodes {
			if node.Id == id {
				rIp := node.Ip
				rPort := node.ServerPort
				rAddr, err := net.ResolveTCPAddr("tcp", rIp+":"+strconv.Itoa(rPort))
				if err != nil {
					fmt.Printf("#Error: Unable to resolve TCP address from IP: %v and Port: %v. With error: %v\n", rIp, rPort, err.Error())
					return err
				}
				nodesMap[id] = Node{
					IP:    rIp,
					Port:  rPort,
					RAddr: rAddr,
				}
			}
		}
	}
	return nil
}

func getLocalAddress() (*net.TCPAddr, error) {
	port, err := getFreePort()
	if err != nil {
		getLocalAddress()
	}
	return net.ResolveTCPAddr("tcp", *ip+":"+strconv.Itoa(port))
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

func makeSimpleMessage(message network2.Message) network2.SimpleMessage {
	simpleMessage := network2.SimpleMessage{To: message.To, From: message.From, Type: message.Type}
	switch message.Type {
	case "value":
		simpleMessage.Payload = message.Value
	case "response":
		simpleMessage.Payload = message.Response
	case "connect":
		simpleMessage.Payload = message.ClientID
	default:
		simpleMessage.Payload = false
	}

	return simpleMessage
}

func stringToSliceOfInts(s string) []int {
	arr := strings.Split(s, ",")
	result := []int{}
	for _, val := range arr {
		newvalue, _ := strconv.Atoi(val)
		result = append(result, newvalue)
	}
	return result
}

func generateRandomString(length int) string {
	charset := "abcdefghijklmnopqrstuvwxzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	seededRandomGenerator := rand.New(rand.NewSource(time.Now().UnixNano())) //New returns a new Rand that uses random values from src to generate other random values.
	bytes := make([]byte, length)
	for i := range bytes {
		bytes[i] = charset[seededRandomGenerator.Intn(len(charset))] //Intn returns, as an int, a non-negative pseudo-random number in [0,n). It panics if n <= 0.
	}
	return string(bytes)
}

func randomRequest() string {
	account := rand.Intn(50)
	category := rand.Intn(2)
	if category == 2 {
		return "status " + strconv.Itoa(account)
	}
	amount := rand.Intn(2000)
	if category == 0 {
		return "deposit " + strconv.Itoa(amount) + " " + strconv.Itoa(account)
	}
	return "withdraw " + strconv.Itoa(amount) + " " + strconv.Itoa(account)
}

// Source https://austingwalters.com/merge-sort-in-go-golang/ for MergeSort + Merge
// Runs MergeSort algorithm on a slice single
func MergeSort(slice []int) []int {

	if len(slice) < 2 {
		return slice
	}
	mid := (len(slice)) / 2
	return Merge(MergeSort(slice[:mid]), MergeSort(slice[mid:]))
}

// Merges left and right slice into newly created slice
func Merge(left, right []int) []int {

	size, i, j := len(left)+len(right), 0, 0
	slice := make([]int, size, size)

	for k := 0; k < size; k++ {
		if i > len(left)-1 && j <= len(right)-1 {
			slice[k] = right[j]
			j++
		} else if j > len(right)-1 && i <= len(left)-1 {
			slice[k] = left[i]
			i++
		} else if left[i] < right[j] {
			slice[k] = left[i]
			i++
		} else {
			slice[k] = right[j]
			j++
		}
	}
	return slice
}
