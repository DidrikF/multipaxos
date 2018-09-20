package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"time"

	"github.com/uis-dat520-s18/group122/grouplab1/detector"
	"github.com/uis-dat520-s18/group122/grouplab1/logger"
	"github.com/uis-dat520-s18/group122/grouplab1/network"
	"github.com/uis-dat520-s18/group122/grouplab2/clienthandler"
	"github.com/uis-dat520-s18/group122/grouplab2/singlepaxos"
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
	roles = flag.String(
		"roles",
		"proposer|acceptor|learner",
		"What roles the node should take on. Example: proposer|acceptor|learner",
	)
	proposers = flag.String(
		"proposers",
		"1,2,3,4",
		"The node ids of the proposers in the system",
	)

	acceptors = flag.String(
		"acceptors",
		"1,2,3,4",
		"The node ids of the acceptors in the system",
	)

	learners = flag.String(
		"learners",
		"1,2,3,4",
		"The node ids of the learners in the system",
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
	//create a channel to receive OS signals. Used to clean up the network layer and other parts of the application
	osSignalChan := make(chan os.Signal, 1)
	signal.Notify(osSignalChan, os.Interrupt) //^C
	//create logger
	logger, err := logger.CreateLogger("./logs", "systemLog", "networkLog", *severity, *loud, *self, *netonly) //func CreateLogger(logDir string, systemLog string, networkLog string, severity string, loud bool) (Logger, error) {
	if err != nil {
		fmt.Println("Creating logger failed with error: ", err.Error())
		os.Exit(0)
	}
	logger.Log("system", "debug", "\n\n\n\n#Application: Started at "+time.Now().Format(time.RFC3339))
	logger.Log("network", "debug", "\n\n\n\n#Application: Started at "+time.Now().Format(time.RFC3339))

	//get configuration from json
	config, err := loadconfig()
	if err != nil {
		//fmt.Println("Failed loading config with error: ", err)
		logger.Log("system", "crit", "#Application: Failed loading config with error: "+err.Error())
		os.Exit(0)
	}

	//create network
	n, err := network.CreateNetwork(config, *self, *mode, logger) //Maybe create sChan and rChan inside an only use these channels direclyt of an instance of a network.
	if err != nil {
		//fmt.Printf("Failed to create network with error %v", err)
		logger.Log("system", "crit", "#Application: Failed to create network with error: "+err.Error())
		os.Exit(0)
	}

	//Get nodeIDs for creation of leader detector
	nodeIDs := []int{n.Self.Id}
	for _, node := range n.Nodes {
		nodeIDs = append(nodeIDs, node.Id)
	}
	//identify roles
	roles := strings.Split(*roles, "|")
	acceptors := stringToSliceOfInts(*acceptors)
	proposers := stringToSliceOfInts(*proposers)
	learners := stringToSliceOfInts(*learners)
	//create leader detector
	ld := detector.NewMonLeaderDetector(proposers) //*MonLeaderDetector

	//create failure detector
	hbChan := make(chan detector.Heartbeat, 16)
	fd := detector.NewEvtFailureDetector(n.Self.Id, nodeIDs, ld, 1*time.Second, hbChan) //how to get things sent on the hbChan onto the network???

	//Connect to peers and start local TCP server
	err = n.EstablishPeer2Peer()
	if err != nil {
		//fmt.Println(err.Error())
		n.Logger.Log("system", "err", "#Application: Error when establishing peer to peer network. Error: "+err.Error())
	}

	//THE PAXOS RELATED CODE:
	var prepareOut chan singlepaxos.Prepare
	var acceptOut chan singlepaxos.Accept
	var promiseOut chan singlepaxos.Promise
	var learnOut chan singlepaxos.Learn
	var valueOut chan singlepaxos.Value
	var proposer *singlepaxos.Proposer
	var acceptor *singlepaxos.Acceptor
	var learner *singlepaxos.Learner

	if stringInSlice("proposer", roles) == true {
		prepareOut = make(chan singlepaxos.Prepare)
		acceptOut = make(chan singlepaxos.Accept)
		proposer = singlepaxos.NewProposer(n.Self.Id, len(acceptors), ld, prepareOut, acceptOut)
		proposer.Start()
	}

	if stringInSlice("acceptor", roles) == true {
		promiseOut = make(chan singlepaxos.Promise)
		learnOut = make(chan singlepaxos.Learn)
		acceptor = singlepaxos.NewAcceptor(n.Self.Id, promiseOut, learnOut)
		acceptor.Start()
	}

	if stringInSlice("learner", roles) == true {
		valueOut = make(chan singlepaxos.Value)
		learner = singlepaxos.NewLearner(n.Self.Id, len(acceptors), valueOut)
		learner.Start()
	}

	//Start the failure detector:
	fd.Start()

	//The client handler:
	clienthandler := clienthandler.NewClientHandler(n.Self.Id, &n, proposer)
	clienthandler.Start()

	//Message Middleware. Transforming and multiplexing messages onto the correct channels based on message type.
	go func() {
		for {
			select { // I DONT WANT ANY OF THIS TO BLOCK!
			case hb := <-hbChan:
				netMsg := network.Message{
					Type:    "heartbeat",
					To:      hb.To,
					From:    hb.From,
					Msg:     "",
					Request: hb.Request,
				}
				//fmt.Printf("\n#1 - Heartbeat: %+v\n", hb)
				n.SChan <- netMsg //what if n.SChan blocks? The channel is buffered.
			case prepare := <-prepareOut:
				prepareMsg := network.Message{
					Type:    "prepare",
					From:    prepare.From,
					Prepare: prepare,
				}
				fmt.Printf("\n#%d - Prepare message out: %+v", proposer.Id, prepare)
				n.BroadcastTo(acceptors, prepareMsg)
				//n.SChan <- prepareMsg
			case accept := <-acceptOut:
				acceptMsg := network.Message{
					Type:   "accept",
					From:   accept.From,
					Accept: accept,
				}
				fmt.Printf("\n#%d - AcceptMessage message out: %+v\n", proposer.Id, accept)
				n.BroadcastTo(acceptors, acceptMsg)
				//n.SChan <- acceptMsg
			case promise := <-promiseOut:
				promiseMsg := network.Message{
					Type:    "promise",
					To:      promise.To,
					From:    promise.From,
					Promise: promise,
				}
				fmt.Printf("\n#%d - Promise message out: %+v\n", acceptor.Id, promise)
				n.SChan <- promiseMsg
			case learn := <-learnOut:
				learnMsg := network.Message{
					Type:  "learn",
					From:  learn.From,
					Learn: learn,
				}
				fmt.Printf("\n#%d - Learn message out: %+v\n", acceptor.Id, learn)
				n.BroadcastTo(learners, learnMsg)
				//n.SChan <- learnMsg
			case value := <-valueOut:
				fmt.Printf("\n#Learner: %v\n", value)
				clienthandler.DeliverValueFromLearner(value)

			//Receive Message:
			case msg := <-n.RChan:
				logger.Log("network", "debug", fmt.Sprintf("Got message: %+v\n", msg))
				switch {
				case msg.Type == "heartbeat":
					hb := detector.Heartbeat{
						To:      msg.To,
						From:    msg.From,
						Request: msg.Request,
					}
					fd.DeliverHeartbeat(hb)
				case msg.Type == "prepare":
					fmt.Printf("\n#%d - Prepare message in: %+v", acceptor.Id, msg.Prepare)
					acceptor.DeliverPrepare(msg.Prepare)
				case msg.Type == "accept":
					fmt.Printf("\n#%d - Accept message in: %+v", acceptor.Id, msg.Accept)
					acceptor.DeliverAccept(msg.Accept)
				case msg.Type == "promise":
					fmt.Printf("\n#%d - Promise message in: %+v", proposer.Id, msg.Promise)
					proposer.DeliverPromise(msg.Promise)
				case msg.Type == "learn":
					fmt.Printf("\n#%d - Learn message in: %+v", learner.Id, msg.Learn)
					learner.DeliverLearn(msg.Learn)
				case msg.Type == "value":
					fmt.Printf("\n#%d - Value message in: %+v", clienthandler.Id, msg.Value)
					clienthandler.DeliverClientValue(msg.Value)
				default:
					n.UnmodRChan <- msg
				}
			}
		}
	}()

	//TEST CLIENT
	/*
		if stringInSlice("proposer", roles) == true {
			go func() {
				for {
					time.Sleep(5 * time.Second)
					proposer.DeliverClientValue(singlepaxos.ZeroValue)
				}
			}()
		}
	*/

	for {
		select {
		//This code is supposed to kill a lot of goroutines, and sometimes the scheduler gives control to a goroutine that is supposed to be killed, is causes some strange behavior, like looping many times over the server listener, which is closed in CleanupNetwork.
		case <-osSignalChan: //Less frequent, but SOMETIMES THIS FAIL?
			logger.Log("system", "debug", "#Application: Exiting...")
			logger.Log("system", "debug", "#Application: Stopping proposer, acceptor and learner...")
			proposer.Stop()
			acceptor.Stop()
			learner.Stop()
			logger.Log("system", "debug", "#Application: Cleaning up network layer...")
			err := n.CleanupNetwork()
			if err != nil {
				logger.Log("system", "debug", "#Application: Error from n.CleanupNetwork(): "+err.Error())
			}
			logger.Log("system", "debug", "#Application: Exiting!")
			os.Exit(0)
		}
	}

}

func loadconfig() (network.ConfigInstance, error) {
	file, err := os.Open("../config.json")
	defer file.Close()
	if err != nil {
		return network.ConfigInstance{}, err
	}
	decoder := json.NewDecoder(file)
	config := network.Config{}
	err = decoder.Decode(&config) //invalid argument
	if err != nil {
		return network.ConfigInstance{}, err
	}
	var conf network.ConfigInstance
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
