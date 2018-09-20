package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"

	"github.com/uis-dat520-s18/glabs/grouplab3/multipaxos"

	"github.com/uis-dat520-s18/glabs/grouplab3/testnet"
)

var (
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
	nodes = flag.String(
		"nodes",
		"1,2,3,4",
		"What nodes are part of the system",
	)
)

func main() {
	flag.Parse()

	osSignalChan := make(chan os.Signal, 1)
	signal.Notify(osSignalChan, os.Interrupt) //^C

	//_____Load Config_______
	config, err := loadconfig()
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(0)
	}

	//______Init Network ________
	net, err := testnet.CreateNetwork(config, *self)
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(0)
	}
	net.StartNetwork()

	go func() {
		for {
			fmt.Println("\nEnter command: ")
			var input string
			fmt.Scanln(&input)
			switch input {
			case "send":
				for i := 0; i < 10; i++ {
					message := testnet.Message{
						To:       1,
						From:     *self,
						Type:     "random",
						Value:    multipaxos.Value{},
						Response: multipaxos.Response{},
					}
					net.SChan <- message
				}
			}
		}
	}()

	for {
		select {
		case response := <-net.RChan:
			fmt.Printf("\nGot message: %v", response)
		case <-osSignalChan: //Less frequent, but SOMETIMES THIS FAIL?
			err := net.CleanupNetwork()
			if err != nil {
				fmt.Println(err.Error())
			}
			fmt.Println("EXITING")
			os.Exit(0)
		}
	}
}

func loadconfig() (testnet.ConfigInstance, error) {
	file, err := os.Open("../config.json")
	defer file.Close()
	if err != nil {
		return testnet.ConfigInstance{}, err
	}
	decoder := json.NewDecoder(file)
	config := testnet.Config{}
	err = decoder.Decode(&config) //invalid argument
	if err != nil {
		return testnet.ConfigInstance{}, err
	}
	var conf testnet.ConfigInstance
	if *mode == "production" {
		conf = config.Production
	} else {
		conf = config.Development
	}
	return conf, nil
}
