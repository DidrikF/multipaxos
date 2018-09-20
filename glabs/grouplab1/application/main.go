package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"time"

	"github.com/uis-dat520-s18/glabs/grouplab1/detector"
	"github.com/uis-dat520-s18/glabs/grouplab1/logger"
	"github.com/uis-dat520-s18/glabs/grouplab1/network"
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
	//create leader detector
	ld := detector.NewMonLeaderDetector(nodeIDs) //*MonLeaderDetector

	//create failure detector
	hbChan := make(chan detector.Heartbeat, 16)
	fd := detector.NewEvtFailureDetector(n.Self.Id, nodeIDs, ld, 1*time.Second, hbChan) //how to get things sent on the hbChan onto the network???

	//Connect to peers and start local TCP server
	err = n.EstablishPeer2Peer()
	if err != nil {
		//fmt.Println(err.Error())
		n.Logger.Log("system", "err", "#Application: Error when establishing peer to peer network. Error: "+err.Error())
	}

	//subscribe to leader changes
	leaderChangeChan := ld.Subscribe()

	//Message Middleware. Transforming and multiplexing messages onto the correct channels based on message type.
	go func() {
		for {
			select { // I DONT WANT ANY OF THIS TO BLOCK!
			case hb := <-hbChan:
				//fmt.Printf("\n{From: %v, To: %v, Request: %v}\n", hb.From, hb.To, strconv.FormatBool(hb.Request)) captured by network layer loggign

				netMsg := network.Message{
					Type:    "heartbeat",
					To:      hb.To,
					From:    hb.From,
					Msg:     "",
					Request: hb.Request,
				}
				n.SChan <- netMsg //what if n.SChan blocks? The channel is buffered.
			case msg := <-n.RChan:
				logger.Log("network", "debug", fmt.Sprintf("Got message: %+v", msg))
				switch {
				case msg.Type == "heartbeat":
					hb := detector.Heartbeat{
						To:      msg.To,
						From:    msg.From,
						Request: msg.Request,
					}
					fd.DeliverHeartbeat(hb)
				default:
					n.UnmodRChan <- msg
				}
			}
		}
	}()

	//Start the failure detector:
	fd.Start()

	//Print leader changes to stdout
	for {
		select {
		case newLeader := <-leaderChangeChan:
			fmt.Println("\n#Application: New Leader -> ", strconv.Itoa(newLeader), "\n")
			logger.Log("system", "debug", "#Application: New Leader -> "+strconv.Itoa(newLeader))

		//This code is supposed to kill a lot of goroutines, and sometimes the scheduler gives control to a goroutine that is supposed to be killed, is causes some strange behavior, like looping many times over the server listener, which is closed in CleanupNetwork.
		case <-osSignalChan: //Less frequent, but SOMETIMES THIS FAIL?
			//mainMutex.Lock()
			logger.Log("system", "debug", "#Application: Exiting...")
			logger.Log("system", "debug", "#Application: Cleaning up network layer...")
			err := n.CleanupNetwork()
			if err != nil {
				logger.Log("system", "debug", "#Application: Error from n.CleanupNetwork(): "+err.Error())
			}
			logger.Log("system", "debug", "#Application: Exiting!")

			//mainMutex.Unlock()
			os.Exit(0)
			//purge connections, then case not connected to some nodes, try to connect to them -> do this periodically. Maybe in its own goroutine
		}
	}

}

func loadconfig() (network.ConfigInstance, error) {
	file, err := os.Open("../config.json")
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
