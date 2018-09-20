package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/uis-dat520-s18/glabs/grouplab4/bankmanager4"
	"github.com/uis-dat520-s18/glabs/grouplab4/detector4"
	"github.com/uis-dat520-s18/glabs/grouplab4/logger4"
	"github.com/uis-dat520-s18/glabs/grouplab4/multipaxos4"
	"github.com/uis-dat520-s18/glabs/grouplab4/network4"
	"github.com/uis-dat520-s18/glabs/grouplab4/webserver"
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
	crypt = flag.Bool(
		"crypt",
		false,
		"Whether to encrypt traffic or not",
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

	var logwg sync.WaitGroup
	var netwg sync.WaitGroup
	var mainwg sync.WaitGroup
	var webserverwg sync.WaitGroup

	webClientChan := make(chan logger4.LogMessage, 100)

	var stopMainLoop = make(chan struct{})
	osSignalChan := make(chan os.Signal, 1)
	signal.Notify(osSignalChan, os.Interrupt) //^C

	nodeIDs := stringToSliceOfInts(*nodes)

	//_____Init Logger _______
	logger, err := logger4.CreateAndStartLogger(*self, "./logs", "systemLog", "networkLog", *severity, *loud, *save, *netonly, &logwg, webClientChan) //func CreateLogger(logDir string, systemLog string, networkLog string, severity string, loud bool) (Logger, error) {
	if err != nil {
		fmt.Println("Creating logger failed with error: ", err.Error())
		logger.Stop()
		logwg.Wait()
		os.Exit(0)
	}
	logger.Log(logger4.LogMessage{O: "application", D: "system", S: "debug", M: "Application Started at " + time.Now().Format(time.RFC3339)})
	logger.Log(logger4.LogMessage{O: "application", D: "network", S: "debug", M: "Application Started at " + time.Now().Format(time.RFC3339)})

	//_____Load Config_______
	config, err := loadconfig()
	if err != nil {
		logger.Log(logger4.LogMessage{O: "application", D: "system", S: "crit", M: "Application failed loading config with error: " + err.Error()})
		logger.Stop()
		logwg.Wait()
		os.Exit(0)
	}

	//______Init Network ________
	net, err := network4.CreateNetwork(config, *self, logger, &netwg, *crypt)
	if err != nil {
		logger.Log(logger4.LogMessage{O: "application", D: "system", S: "crit", M: "Application failed to create network with error: " + err.Error()})
		logger.Stop()
		// netwg.Wait() ??
		logwg.Wait()
		os.Exit(0)
	}
	err = net.StartNetwork()
	if err != nil {
		fmt.Println("Error starting network: " + err.Error())
	}
	fmt.Println("Started network")
	//_______Initialize and start all the relevant parts your application.______

	//create leader detector
	ld := detector4.NewMonLeaderDetector(nodeIDs) //*MonLeaderDetector

	//create failure detector
	hbChan := make(chan detector4.Heartbeat, 16)
	fd := detector4.NewEvtFailureDetector(*self, nodeIDs, ld, 1*time.Second, hbChan) //how to get things sent on the hbChan onto the network???

	//THE PAXOS RELATED CODE:
	prepareOut := make(chan multipaxos4.Prepare, 8)
	acceptOut := make(chan multipaxos4.Accept, 8)
	promiseOut := make(chan multipaxos4.Promise, 8)
	learnOut := make(chan multipaxos4.Learn, 8)
	decidedOut := make(chan multipaxos4.DecidedValue, 8)

	proposer := multipaxos4.NewProposer(*self, len(nodeIDs), -1, ld, prepareOut, acceptOut)
	acceptor := multipaxos4.NewAcceptor(*self, promiseOut, learnOut)
	learner := multipaxos4.NewLearner(*self, len(nodeIDs), decidedOut)
	proposer.Start()
	acceptor.Start()
	learner.Start()

	//Start the failure detector:
	fd.Start()

	leaderUpdates := ld.Subscribe()

	//System startup is a special case where no slots have been decided from the point of view of the application/proposer...	//	When fist starting a replica, the adu is -1 in both the proposer and bankmanager4. This is the slotID communicated in the Prepare message, which results in Promises -> Accepts -> Learns -> DecidedValues which finally is used to update the Accounts.
	responseOut := make(chan multipaxos4.Response, 8)
	bankmanager := bankmanager4.NewbankManager(responseOut, proposer, logger) //Notify the Proposer when a new slot has been decided. -> done by the bankmanager

	//The web server:
	webServer := webserver.NewWebServer(*self, bankmanager, proposer, ld, logger, &webserverwg, webClientChan, *crypt)
	webServer.Start()

	//______	//Multiplex and demultiplex message to and from the network layer.________
	stopHbLoop := make(chan struct{})
	mainwg.Add(1)
	go func() {
		defer func() {
			logger.Log(logger4.LogMessage{O: "application", D: "system", S: "debug", M: fmt.Sprintf("Heart Beat handler stoped")})
			mainwg.Done()
		}()
		for {
			select { // I DONT WANT ANY OF THIS TO BLOCK!
			case hb := <-hbChan: // From Eventual Leader Detector
				hbMsg := network4.Message{
					Type:    "heartbeat",
					To:      hb.To,
					From:    hb.From,
					Request: hb.Request,
				}
				//logger.Log(logger4.LogMessage{D: "system", S: "debug", M: "HEARTBEAR FROM HBCHAN: " + fmt.Sprintf("%+v", hbMsg)})
				//fmt.Printf("#1 - Heartbeat: %+v\n", hb)
				net.SChan <- hbMsg
			case <-stopHbLoop:
				return
			}
		}
	}()

	stopResponseLoop := make(chan struct{})
	mainwg.Add(1)
	go func() {
		defer func() {
			logger.Log(logger4.LogMessage{O: "application", D: "system", S: "debug", M: fmt.Sprintf("Heart Beat handler stoped")})
			mainwg.Done()
		}()
		for {
			select { // I DONT WANT ANY OF THIS TO BLOCK!
			case response := <-responseOut: // From BankManager
				logger.Log(logger4.LogMessage{O: "application", D: "system", S: "debug", M: fmt.Sprintf("Response from bankmanager: %+v", response)})
				// logger.Log(logger4.LogMessage{D: "system", S: "debug", M: bankmanager.GetStatus()})
				//fmt.Printf("Response from resonseOut: %v\n", response)

				webServer.DeliverResponse(response)
				logger.Log(logger4.LogMessage{O: "application", D: "system", S: "debug", M: fmt.Sprintf("Delivered response via webserver! ClientSeq %v", response.ClientSeq)})
			case <-stopResponseLoop:
				return
			}
		}
	}()

	mainwg.Add(1)
	go func() {
		defer func() {
			logger.Log(logger4.LogMessage{O: "application", D: "system", S: "debug", M: fmt.Sprintf("Application mux stoped")})
			mainwg.Done()
		}()
		for {
			select { // I DONT WANT ANY OF THIS TO BLOCK!
			case newLeader := <-leaderUpdates:
				logger.Log(logger4.LogMessage{O: "application", D: "system", S: "debug", M: fmt.Sprintf("Leader update, new leader: %v", newLeader)})

			case prepare := <-prepareOut: // From Proposer
				prepareMsg := network4.Message{
					Type:    "prepare",
					From:    prepare.From,
					Prepare: prepare,
				}
				//fmt.Printf("Prepare message out: %+v\n", prepare)
				net.BroadcastTo(nodeIDs, prepareMsg)

			case accept := <-acceptOut: // From Proposer
				acceptMsg := network4.Message{
					Type:   "accept",
					From:   accept.From,
					Accept: accept,
				}
				//fmt.Printf("AcceptMessage message out: %+v\n", accept)
				net.BroadcastTo(nodeIDs, acceptMsg)

			case promise := <-promiseOut: // From Acceptor
				promiseMsg := network4.Message{
					Type:    "promise",
					To:      promise.To,
					From:    promise.From,
					Promise: promise,
				}
				//fmt.Printf("Promise message out: %+v\n", promise)
				net.SChan <- promiseMsg

			case learn := <-learnOut: // From Acceptor
				learnMsg := network4.Message{
					Type:  "learn",
					From:  learn.From,
					Learn: learn,
				}
				//fmt.Printf("#Learn message out: %+v\n", learn)
				net.BroadcastTo(nodeIDs, learnMsg)

			case decidedValue := <-decidedOut: // From Learner
				//fmt.Printf("DecidedValue from decidedOut: %v - about to be given to bankmanager.HandleDecideValue()\n", decidedValue)
				logger.Log(logger4.LogMessage{O: "application", D: "system", S: "debug", M: fmt.Sprintf("DecidedValue from Learner: %+v", decidedValue)})
				bankmanager.HandleDecidedValue(decidedValue)

			//Receive Messages:
			case msg := <-net.RChan:
				//logger.Log("network", "debug", fmt.Sprintf("Got message: %+v\n", msg))
				switch {
				case msg.Type == "heartbeat":
					hb := detector4.Heartbeat{
						To:      msg.To,
						From:    msg.From,
						Request: msg.Request,
					}
					//logger.Log(logger4.LogMessage{O: "application", D: "system", S: "debug", M: "Heartbeat from RCHAN" + fmt.Sprintf("%+v", msg)})
					//logger.Log(logger4.LogMessage{O: "application", D: "system", S: "debug", M: "Heartbeat from RCHAN" + fmt.Sprintf("%+v", hb)})
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
				default:
					fmt.Printf("Message of unknown type: %+v\n", msg)
				}
			case <-stopMainLoop:
				logger.Log(logger4.LogMessage{O: "application", D: "system", S: "notice", M: fmt.Sprintf("Application mux was stoped by stopMainLoop channel!")})
				return
			}
		}
	}()
	// Go-routine to read commands from stdin
	/*
		mainwg.Add(1)
		go func() {
			defer mainwg.Done()

			scanner := bufio.NewScanner(os.Stdin)
			for {
				fmt.Println("Enter command: ")
				scanner.Scan()
				input := scanner.Text()
				switch input {
				case "bm-history":
					fmt.Println(bankmanager.GetHistory())
				case "bm-accounts":
					fmt.Println(bankmanager.GetStatus())
				case "leader":
					fmt.Printf("Current leader: %v\n", strconv.Itoa(ld.Leader()))
				case "test-send":
					net.Broadcast(network4.Message{
						From: *self,
						Type: "test",
					})
				case "nodes":
					fmt.Printf("%v", net.Connections)
				case "clients":
					fmt.Printf("%v", webServer.GetConnectedClients())
				case "client-chans":
					fmt.Printf("%v", webServer.GetConnectedClientsChanLength())
				case "schan":
					fmt.Printf("Number of queued messages in SChan: %v\n", len(net.SChan))
				case "hbchan":
					fmt.Printf("Number of queued messages in hbChan: %v\n", len(hbChan))
				case "channels":
					fmt.Printf("prepareOut: %v, acceptOut: %v, promiseOut: %v, learnOut: %v, decidedOut: %v, responseOut: %v, Rchan: %v\n", len(prepareOut), len(acceptOut), len(promiseOut), len(learnOut), len(decidedOut), len(responseOut), len(net.RChan))
				case "q":
					osSignalChan <- os.Interrupt
					return
				}
			}
		}()

	*/

	// For-select loop to block until the program should clean up and exit
	for {
		select {
		case <-osSignalChan: //Less frequent
			logger.Log(logger4.LogMessage{O: "application", D: "system", S: "debug", M: "Exiting..."})
			logger.Log(logger4.LogMessage{O: "application", D: "system", S: "debug", M: "Stopping proposer, acceptor and learner..."})
			proposer.Stop()
			acceptor.Stop()
			learner.Stop()
			fd.Stop()

			logger.Log(logger4.LogMessage{O: "application", D: "system", S: "debug", M: "Cleaning up web server..."})
			webServer.Stop()
			logger.Log(logger4.LogMessage{O: "application", D: "system", S: "debug", M: "Waiting for webServer to finish cleaning up..."})
			webserverwg.Wait()

			logger.Log(logger4.LogMessage{O: "application", D: "system", S: "debug", M: "Cleaning up network layer..."})
			err := net.CleanupNetwork() // should eventually stop server go routine and readFromConn go routines...
			if err != nil {
				logger.Log(logger4.LogMessage{O: "application", D: "system", S: "debug", M: "Error from n.CleanupNetwork(): " + err.Error()})
			}
			logger.Log(logger4.LogMessage{O: "application", D: "system", S: "debug", M: "Stoping the networks send go routine..."})
			net.Stop() // stop send message go routine
			netwg.Wait()
			logger.Log(logger4.LogMessage{O: "application", D: "system", S: "debug", M: "Stoping main application loop..."})
			stopHbLoop <- struct{}{}
			stopMainLoop <- struct{}{}
			stopResponseLoop <- struct{}{}
			mainwg.Wait()

			logger.Log(logger4.LogMessage{O: "application", D: "system", S: "debug", M: "Stoping and draining logger..."})
			logger.Stop()
			logwg.Wait()
			fmt.Println("#Clean up done! Exiting.")
			os.Exit(0)
		}
	}
	fmt.Printf("$$$ THE VERY END OF MAIN HAS BEEN EXECUTED $$$")
}

func loadconfig() (network4.ConfigInstance, error) {
	file, err := os.Open("../config.json")
	defer file.Close()
	if err != nil {
		return network4.ConfigInstance{}, err
	}
	decoder := json.NewDecoder(file)
	config := network4.Config{}
	err = decoder.Decode(&config) //invalid argument
	if err != nil {
		return network4.ConfigInstance{}, err
	}
	var conf network4.ConfigInstance
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
