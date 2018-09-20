package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/uis-dat520-s18/glabs/grouplab1/detector"
	"github.com/uis-dat520-s18/glabs/grouplab3/bankmanager"
	"github.com/uis-dat520-s18/glabs/grouplab3/clienthandler2"
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
	mode = flag.String(
		"mode",
		"development",
		"Wheter to run application in development, production or testing mode",
	)
	self = flag.Int(
		"self",
		-1,
		"Node ID of this node, can not be negative",
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
	nodes = flag.String(
		"nodes",
		"1,2,3,4",
		"What nodes are part of the system",
	)
)

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
	if *self < 0 {
		flag.Usage()
		os.Exit(0)
	}

	var wg sync.WaitGroup
	osSignalChan := make(chan os.Signal, 1)
	signal.Notify(osSignalChan, os.Interrupt) //^C

	nodeIDs := stringToSliceOfInts(*nodes)

	//_____Init Logger _______
	logger, err := logger2.CreateAndStartLogger(*self, "./logs", "systemLog", "networkLog", *severity, *loud, *save, *netonly, &wg) //func CreateLogger(logDir string, systemLog string, networkLog string, severity string, loud bool) (Logger, error) {
	if err != nil {
		fmt.Println("Creating logger failed with error: ", err.Error())
		logger.Stop()
		wg.Wait()
		os.Exit(0)
	}
	logger.Log(logger2.LogMessage{D: "system", S: "debug", M: "#Application: Started at " + time.Now().Format(time.RFC3339)})
	logger.Log(logger2.LogMessage{D: "network", S: "debug", M: "#Application: Started at " + time.Now().Format(time.RFC3339)})

	//_____Load Config_______
	config, err := loadconfig()
	if err != nil {
		logger.Log(logger2.LogMessage{D: "system", S: "crit", M: "#Application: Failed loading config with error: " + err.Error()})
		logger.Stop()
		wg.Wait()
		os.Exit(0)
	}

	//______Init Network ________
	net, err := network2.CreateNetwork(config, *self, logger)
	if err != nil {
		logger.Log(logger2.LogMessage{D: "system", S: "crit", M: "#Application: Failed to create network with error: " + err.Error()})
		logger.Stop()
		wg.Wait()
		os.Exit(0)
	}
	err = net.StartNetwork()
	if err != nil {
		fmt.Println("Error starting network: " + err.Error())
	}
	fmt.Println("Started network")
	//_______Initialize and start all the relevant parts your application.______

	//create leader detector
	ld := detector.NewMonLeaderDetector(nodeIDs) //*MonLeaderDetector

	//create failure detector
	hbChan := make(chan detector.Heartbeat, 16)
	fd := detector.NewEvtFailureDetector(*self, nodeIDs, ld, 1*time.Second, hbChan) //how to get things sent on the hbChan onto the network???

	//THE PAXOS RELATED CODE:
	prepareOut := make(chan multipaxos.Prepare, 8)
	acceptOut := make(chan multipaxos.Accept, 8)
	promiseOut := make(chan multipaxos.Promise, 8)
	learnOut := make(chan multipaxos.Learn, 8)
	decidedOut := make(chan multipaxos.DecidedValue, 8)

	proposer := multipaxos.NewProposer(*self, len(nodeIDs), -1, ld, prepareOut, acceptOut)
	acceptor := multipaxos.NewAcceptor(*self, promiseOut, learnOut)
	learner := multipaxos.NewLearner(*self, len(nodeIDs), decidedOut)
	proposer.Start()
	acceptor.Start()
	learner.Start()

	//Start the failure detector:
	fd.Start()

	//The client handler:
	clientHandler := clienthandler2.NewClientHandler(*self, net, proposer, ld, logger)
	clientHandler.Start()

	//System startup is a special case where no slots have been decided from the point of view of the application/proposer...
	//	When fist starting a replica, the adu is -1 in both the proposer and bankmanager. This is the slotID communicated in the Prepare message, which results in Promises -> Accepts -> Learns -> DecidedValues which finally is used to update the Accounts.
	responseOut := make(chan multipaxos.Response, 8)
	bankManager := bankmanager.NewBankManager(responseOut, proposer, logger) //Notify the Proposer when a new slot has been decided. -> done by the bank manager

	//______	//Multiplex and demultiplex message to and from the network layer.________
	go func() {
		for {
			select { // I DONT WANT ANY OF THIS TO BLOCK!
			case hb := <-hbChan:
				hbMsg := network2.Message{
					Type:    "heartbeat",
					To:      hb.To,
					From:    hb.From,
					Request: hb.Request,
				}
				//logger.Log(logger2.LogMessage{D: "system", S: "debug", M: "HEARTBEAR FROM HBCHAN: " + fmt.Sprintf("%+v", hbMsg)})
				//fmt.Printf("#1 - Heartbeat: %+v\n", hb)
				net.SChan <- hbMsg //what if n.SChan blocks? The channel is buffered.
			case prepare := <-prepareOut:
				prepareMsg := network2.Message{
					Type:    "prepare",
					From:    prepare.From,
					Prepare: prepare,
				}
				//fmt.Printf("Prepare message out: %+v\n", prepare)
				net.BroadcastTo(nodeIDs, prepareMsg)
				//n.SChan <- prepareMsg
			case accept := <-acceptOut:
				acceptMsg := network2.Message{
					Type:   "accept",
					From:   accept.From,
					Accept: accept,
				}
				//fmt.Printf("AcceptMessage message out: %+v\n", accept)
				net.BroadcastTo(nodeIDs, acceptMsg)
				//n.SChan <- acceptMsg
			case promise := <-promiseOut:
				promiseMsg := network2.Message{
					Type:    "promise",
					To:      promise.To,
					From:    promise.From,
					Promise: promise,
				}
				//fmt.Printf("Promise message out: %+v\n", promise)
				net.SChan <- promiseMsg
			case learn := <-learnOut:
				learnMsg := network2.Message{
					Type:  "learn",
					From:  learn.From,
					Learn: learn,
				}
				//fmt.Printf("#Learn message out: %+v\n", learn)
				net.BroadcastTo(nodeIDs, learnMsg)
				//n.SChan <- learnMsg
			case decidedValue := <-decidedOut:
				//fmt.Printf("DecidedValue from decidedOut: %v - about to be given to bankManager.HandleDecideValue()\n", decidedValue)
				logger.Log(logger2.LogMessage{D: "system", S: "debug", M: fmt.Sprintf("#Application: DecidedValue from Learner: %+v", decidedValue)})
				bankManager.HandleDecidedValue(decidedValue)
			case response := <-responseOut: //bank manager pushes responses out here
				logger.Log(logger2.LogMessage{D: "system", S: "debug", M: fmt.Sprintf("#Application: Response from BankManager: %+v", response)})
				logger.Log(logger2.LogMessage{D: "system", S: "debug", M: bankManager.GetStatus()})
				//fmt.Printf("Response from resonseOut: %v\n", response)
				clientHandler.DeliverResponse(response) //Route client requests from the client handling module to the Proposer (if the client handling module does not interact directly with the Proposer).

			//Receive Message:
			case msg := <-net.RChan:
				//logger.Log("network", "debug", fmt.Sprintf("Got message: %+v\n", msg))
				switch {
				case msg.Type == "heartbeat":
					hb := detector.Heartbeat{
						To:      msg.To,
						From:    msg.From,
						Request: msg.Request,
					}
					//logger.Log(logger2.LogMessage{D: "system", S: "debug", M: "Heartbeat from RCHAN" + fmt.Sprintf("%+v", msg)})
					//logger.Log(logger2.LogMessage{D: "system", S: "debug", M: "Heartbeat from RCHAN" + fmt.Sprintf("%+v", hb)})
					//fmt.Printf("\nHeartbeat message in: %+v", hb)
					fd.DeliverHeartbeat(hb)
				case msg.Type == "prepare":
					fmt.Printf("Prepare message in: %+v\n", msg.Prepare)
					acceptor.DeliverPrepare(msg.Prepare)
				case msg.Type == "accept":
					fmt.Printf("Accept message in: %+v\n", msg.Accept)
					acceptor.DeliverAccept(msg.Accept)
				case msg.Type == "promise":
					fmt.Printf("Promise message in: %+v\n", msg.Promise)
					proposer.DeliverPromise(msg.Promise)
				case msg.Type == "learn":
					fmt.Printf("Learn message in: %+v\n", msg.Learn)
					learner.DeliverLearn(msg.Learn)
				case msg.Type == "value":
					fmt.Printf("Value message in: %+v\n", msg.Value)
					clientHandler.DeliverClientValue(msg.Value)
				default:
					fmt.Printf("Message of unknown type: %+v\n", msg)
				}
			}
		}
	}()

	go func() {
		scanner := bufio.NewScanner(os.Stdin)
		for {
			fmt.Println("Enter command: ")
			scanner.Scan()
			input := scanner.Text()
			switch input {
			case "bm-history":
				fmt.Println(bankManager.GetHistory())
			case "bm-accounts":
				fmt.Println(bankManager.GetStatus())
			case "leader":
				fmt.Println("Current leader: " + strconv.Itoa(ld.Leader()))
			}
		}
	}()

	for {
		select {
		case <-osSignalChan: //Less frequent, but SOMETIMES THIS FAIL?
			logger.Log(logger2.LogMessage{D: "system", S: "debug", M: "#Application: Exiting..."})
			logger.Log(logger2.LogMessage{D: "system", S: "debug", M: "#Application: Stopping proposer, acceptor and learner..."})
			proposer.Stop()
			acceptor.Stop()
			learner.Stop()

			logger.Log(logger2.LogMessage{D: "system", S: "debug", M: "#Application: Cleaning up network layer..."})
			err := net.CleanupNetwork()
			if err != nil {
				logger.Log(logger2.LogMessage{D: "system", S: "debug", M: "#Application: Error from n.CleanupNetwork(): " + err.Error()})
			}
			logger.Log(logger2.LogMessage{D: "system", S: "debug", M: "#Application: Stoping and draining logger..."})
			logger.Stop()
			wg.Wait()
			fmt.Println("#Clean up done! Exiting.")
			os.Exit(0)
		}
	}
}

func loadconfig() (network2.ConfigInstance, error) {
	file, err := os.Open("../config.json")
	defer file.Close()
	if err != nil {
		return network2.ConfigInstance{}, err
	}
	decoder := json.NewDecoder(file)
	config := network2.Config{}
	err = decoder.Decode(&config) //invalid argument
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
