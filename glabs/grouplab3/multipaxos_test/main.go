package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/uis-dat520-s18/glabs/grouplab1/detector"
	"github.com/uis-dat520-s18/glabs/grouplab3/bank"
	"github.com/uis-dat520-s18/glabs/grouplab3/bankmanager"
	"github.com/uis-dat520-s18/glabs/grouplab3/fakeclienthandler"
	"github.com/uis-dat520-s18/glabs/grouplab3/fakenet"
	"github.com/uis-dat520-s18/glabs/grouplab3/multipaxos"
	"github.com/uis-dat520-s18/glabs/grouplab3/network2"
)

var (
	help = flag.Bool(
		"help",
		false,
		"Show usage help",
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
	connection Connection
	leader     = -1
	clientID   string
	sequence   = 0
	operations = map[string]int{
		"balance":    0,
		"deposit":    1,
		"withdrawal": 2,
	}
	seqRespondedTo  = -1
	valueBuffer     = []multipaxos.Value{}
	responsesGotten = []multipaxos.Response{}
	responseTimer   *time.Ticker
	network         *fakenet.Network
)

var mu sync.Mutex

type Connection struct {
	To   int
	Conn struct{}
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

	osSignalChan := make(chan os.Signal, 1)
	signal.Notify(osSignalChan, os.Interrupt) //^C

	//_____Init Logger _______

	NODEIDS := []int{1, 2, 3}

	//______Init Network ________
	network = fakenet.CreateNetwork(NODEIDS)
	network.Start()

	//_______Initialize and start all the relevant parts your application.______

	for _, nodeID := range NODEIDS {
		go func(nodeID int) {
			fmt.Println("Started Node " + strconv.Itoa(nodeID))
			//create leader detector
			ld := detector.NewMonLeaderDetector(NODEIDS) //*MonLeaderDetector

			//create failure detector
			hbChan := make(chan detector.Heartbeat, 16)
			fd := detector.NewEvtFailureDetector(nodeID, NODEIDS, ld, 1*time.Second, hbChan) //how to get things sent on the hbChan onto the network???

			//THE PAXOS RELATED CODE:
			prepareOut := make(chan multipaxos.Prepare, 8)
			acceptOut := make(chan multipaxos.Accept, 8)
			promiseOut := make(chan multipaxos.Promise, 8)
			learnOut := make(chan multipaxos.Learn, 8)
			decidedOut := make(chan multipaxos.DecidedValue, 8)

			proposer := multipaxos.NewProposer(nodeID, len(NODEIDS), -1, ld, prepareOut, acceptOut)
			acceptor := multipaxos.NewAcceptor(nodeID, promiseOut, learnOut)
			learner := multipaxos.NewLearner(nodeID, len(NODEIDS), decidedOut)
			proposer.Start()
			acceptor.Start()
			learner.Start()

			//Start the failure detector:
			fd.Start()

			//The client handler:
			clientHandler := fakeclienthandler.NewClientHandler(nodeID, network, proposer, ld)
			clientHandler.Start()

			responseOut := make(chan multipaxos.Response, 8)
			bankManager := bankmanager.NewBankManager(responseOut, proposer)
			mu.Lock()
			rchan := network.RChans[nodeID]
			mu.Unlock()

			//______	//Multiplex and demultiplex message to and from the network layer.________
			for {
				select {
				case hb := <-hbChan:
					hbMsg := network2.Message{
						Type:    "heartbeat",
						To:      hb.To,
						From:    hb.From,
						Request: hb.Request,
					}
					//fmt.Printf("\n#1 - Heartbeat message out: %+v\n", hb)
					network.SChan <- hbMsg //what if n.SChan blocks? The channel is buffered.
				case prepare := <-prepareOut:
					prepareMsg := network2.Message{
						Type:    "prepare",
						From:    prepare.From,
						Prepare: prepare,
					}
					//fmt.Printf("\nPrepare message out: %+v", prepare)
					network.BroadcastTo(NODEIDS, prepareMsg)
					//n.SChan <- prepareMsg
				case accept := <-acceptOut:
					acceptMsg := network2.Message{
						Type:   "accept",
						From:   accept.From,
						Accept: accept,
					}
					//fmt.Printf("\nAcceptMessage message out: %+v\n", accept)
					network.BroadcastTo(NODEIDS, acceptMsg)
					//n.SChan <- acceptMsg
				case promise := <-promiseOut:
					promiseMsg := network2.Message{
						Type:    "promise",
						To:      promise.To,
						From:    promise.From,
						Promise: promise,
					}
					//fmt.Printf("\nPromise message out: %+v\n", promise)
					network.SChan <- promiseMsg
				case learn := <-learnOut:
					learnMsg := network2.Message{
						Type:  "learn",
						From:  learn.From,
						Learn: learn,
					}
					//fmt.Printf("\n#Learn message out: %+v\n", learn)
					network.BroadcastTo(NODEIDS, learnMsg)
				case decidedValue := <-decidedOut:
					//fmt.Printf("\nDecidedValue from decidedOut: %v - about to be given to bankManager.HandleDecideValue()", decidedValue)
					bankManager.HandleDecidedValue(decidedValue)
				case response := <-responseOut: //bank manager pushes responses out here
					fmt.Printf(bankManager.GetStatus())
					//fmt.Printf("\nResponse from resonseOut: %v", response)
					clientHandler.DeliverResponse(response)
				//Receive Message:
				case msg := <-rchan:
					switch {
					case msg.Type == "heartbeat":
						hb := detector.Heartbeat{
							To:      msg.To,
							From:    msg.From,
							Request: msg.Request,
						}
						//fmt.Printf("\nHeart beat message in: %+v", msg.Request)
						fd.DeliverHeartbeat(hb)
					case msg.Type == "prepare":
						fmt.Printf("\nPrepare message in: %+v", msg.Prepare)
						acceptor.DeliverPrepare(msg.Prepare)
					case msg.Type == "accept":
						fmt.Printf("\nAccept message in: %+v", msg.Accept)
						acceptor.DeliverAccept(msg.Accept)
					case msg.Type == "promise":
						fmt.Printf("\nPromise message in: %+v", msg.Promise)
						proposer.DeliverPromise(msg.Promise)
					case msg.Type == "learn":
						fmt.Printf("\nLearn message in: %+v", msg.Learn)
						learner.DeliverLearn(msg.Learn)
					case msg.Type == "value":
						fmt.Printf("\nValue message in: %+v", msg.Value)
						clientHandler.DeliverClientValue(msg.Value)
					default:
						fmt.Printf("\nMessage of unknown type: %+v", msg)
					}
				}
			}
		}(nodeID)
	}

	fmt.Printf("Getting an rchan: %v", network.RChans[2])

	//_____Client Code:_____
	network.RChans[42] = make(chan network2.Message, 8)

	//____Command line input loop______
	go func(self int, leader int) {
		mu.Lock()
		connection.To = leader
		mu.Unlock()

		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			input := scanner.Text()
			if input == "quit" {
				fmt.Println("Quitting...")
				break
			}
			//look for special commands:
			runes := []rune(input)
			firstCharacter := string(runes[0])
			if firstCharacter == "#" {
				input = string(runes[1:])
				switch input {
				case "status":
					fmt.Printf("Status Report:\n - Network Status: %v\n - Current Leader: %v\n - Current ClientSeq: %v\n - seqRespondedTo: %v\n - Time waited for response: %v\n", connection, leader, sequence, seqRespondedTo, "NOT IMPLEMENTED")
				case "history":
					fmt.Printf("Value and Response History:\n - Value Buffer: %v\n - Responses Gotten: %v\n", valueBuffer, responsesGotten)
				}
				continue
			}
			value, err := parseInputToValue(input)
			if err != nil {
				fmt.Println("Error: " + err.Error())
				continue
			}
			mu.Lock()
			sequence++
			mu.Unlock()
			handleSendingOfValues(value)
			fmt.Printf("\nvalueBuffer: %v", valueBuffer)
		}
		if err := scanner.Err(); err != nil {
			fmt.Println("Error encountered:", err)
		}
	}(42, 3)

	//_____ Message handling loop and program exit______
	for {
		select {
		case msg := <-network.RChans[42]:
			switch msg.Type {
			case "redirect":
				fmt.Println("Client: Got redirected to node " + strconv.Itoa(msg.Leader))
				mu.Lock()
				leader = msg.Leader
				connection.To = leader
				mu.Unlock()
				//err := connectToLeader()
				//if err != nil {
				//	fmt.Println("Error: " + err.Error())
				//	break
				//}
				fmt.Println("Client: Connected to new leader!")
				handleSendingOfValues()
			case "response":
				response := msg.Response
				fmt.Printf("\n#Client: Got response to ClientSeq %v - Transaction Result: %+v", response.ClientSeq, response.TxnRes.String())
				//if not seed response before save it and inc seqRespondedTo
				gottenResponseBefore := false
				for _, res := range responsesGotten {
					if response.ClientSeq == res.ClientSeq {
						gottenResponseBefore = true
					}
				}
				if gottenResponseBefore == false {
					responsesGotten = append(responsesGotten, response)
					seqRespondedTo++
					handleSendingOfValues()
				}
			}
		case <-osSignalChan: //Less frequent, but SOMETIMES THIS FAIL?
			fmt.Println("EXITING!")
			os.Exit(0)
		}
	}
}

func stringInSlice(a string, list []string) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
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

/*
Could add some client code...
*/
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
		if op == 0 { //balance
			return multipaxos.Value{
				ClientID:   clientID,
				ClientSeq:  sequence,
				AccountNum: int(account),
				Txn: bank.Transaction{
					Op:     bank.Operation(op),
					Amount: 0,
				},
			}, nil
		} else if op == 1 { //deposit
			amount, err := strconv.ParseInt(inputSlice[1], 10, 0)
			if err != nil {
				return val, errors.New("Amount needs to be an int, " + inputSlice[1] + " was given.")
			}
			return multipaxos.Value{
				ClientID:   clientID,
				ClientSeq:  sequence,
				AccountNum: int(account),
				Txn: bank.Transaction{
					Op:     bank.Operation(op),
					Amount: int(amount),
				},
			}, nil
		} else if op == 2 { //withdrawel
			amount, err := strconv.ParseInt(inputSlice[1], 10, 0)
			if err != nil {
				return val, errors.New("Amount needs to be an int, " + inputSlice[1] + " was given.")
			}
			return multipaxos.Value{
				ClientID:   clientID,
				ClientSeq:  sequence,
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
	fmt.Println("handle sending of value")
	msgSent := false
	//loop through values passed as arguments and buffer or send
	if len(values) > 0 {
		for _, val := range values {
			if val.ClientSeq > seqRespondedTo+1 {
				valueBuffer = append(valueBuffer, val)
				continue
			} else if val.ClientSeq == seqRespondedTo+1 {
				msg := network2.Message{
					To:    connection.To,
					From:  42,
					Type:  "value",
					Value: val,
				}
				fmt.Printf("\nputting value message on SChan in handleSendingOfValue() - value: %v", val)
				network.SChan <- msg
				responseTimer = time.NewTicker(3 * time.Second)
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
				From:  42,
				Type:  "value",
				Value: val,
			}
			fmt.Printf("\nputting value message on SChan in handleSendingOfValue() - value: %v", val)
			network.SChan <- msg
			responseTimer = time.NewTicker(3 * time.Second)
			valueBuffer = append(valueBuffer[:index], valueBuffer[index+1:]...)
			break
		}
	}
}
