package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"strconv"
	"strings"

	"github.com/uis-dat520-s18/glabs/grouplab1/network"
	"github.com/uis-dat520-s18/glabs/grouplab2/singlepaxos"
)

var (
	help = flag.Bool(
		"help",
		false,
		"Show usage help",
	)
	ip = flag.String(
		"ip",
		"localhost",
		"What IP address to to dial from",
	)
	portseed = flag.Int(
		"portseed",
		6000,
		"What port to dial from",
	)
	proposers = flag.String(
		"proposers",
		"1,2,3,4",
		"The node ids of the systems proposers",
	)
	mode = flag.String(
		"mode",
		"development",
		"Whether to start the application in production or development mode",
	)
	rChan        = make(chan network.Message, 16)
	sChan        = make(chan network.Message, 16)
	conns        = map[int]net.Conn{}
	proposersMap = map[int]Node{}
)

type Node struct {
	IP    string
	Port  int
	RAddr *net.TCPAddr
	LAddr *net.TCPAddr
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
	signal.Notify(osSignalChan, os.Interrupt)

	//get configuration from json
	config, err := loadconfig()
	if err != nil {
		fmt.Println("Failed loading config with error: ", err)
		os.Exit(0)
	}

	proposers := stringToSliceOfInts(*proposers)

	buildProposersMap(proposers, config)

	// Connect to proposers:
	connId := 0
	for _, id := range proposers {
		rAddr := proposersMap[id].RAddr
		lAddr := proposersMap[id].LAddr
		tcpConn, err := net.DialTCP("tcp", lAddr, rAddr)
		if err != nil {
			fmt.Printf("\n#Error: Dial failed with error: %v", err.Error())
			continue
		}
		fmt.Printf("\nConnected to node: " + tcpConn.RemoteAddr().String())
		connId++
		conns[connId] = tcpConn
		go listenOnConn(tcpConn, connId)
	}

	go func() {
		for {
			fmt.Print("\nEnter text: ")
			var input string
			fmt.Scanln(&input)
			msg := network.Message{
				Type:  "value",
				Value: singlepaxos.Value(input),
			}
			sChan <- msg
		}
	}()

	for {
		select {
		case msg := <-rChan:
			switch msg.Type {
			case "value":
				fmt.Printf("\n#Client: Got value: %+v\nEnter text: ", msg.Value)
			}
		case msg := <-sChan:
			//fmt.Printf("\n#Client: Sending message: %+v", msg)
			fmt.Printf("\n#Client: Sending message...\nEnter text: ")
			err := send(msg)
			if err != nil {
				fmt.Printf("\n%v", err.Error())
			}
		case <-osSignalChan:
			fmt.Println("#Application: Exiting...")
			fmt.Println("#Application: Cleaning up network layer...")
			err := cleanUpConns()
			if err != nil {
				fmt.Println("#Application: Error from cleanUpConns(): " + err.Error())
			}
			fmt.Println("#Application: Exiting!")
			os.Exit(0)
		}
	}
}

func send(message network.Message) error {
	jsonByteSlice, err := json.Marshal(message)
	if err != nil {
		return errors.New("Failed to MARSHAL message. Aborting sending of message. Error: " + err.Error())
	}
	length := len(jsonByteSlice)
	for _, conn := range conns {
		_, err = conn.Write(jsonByteSlice[0:length])
		if err != nil {
			fmt.Println("Failed to write/send message to: "+conn.RemoteAddr().String()+". Aborting sending of message with error: ", err.Error())
		}
	}
	return nil
}

func listenOnConn(tcpConn net.Conn, connId int) {
	defer func() {
		delete(conns, connId)
		tcpConn.Close()
	}()

	for {
		var buf [20480]byte
		message := new(network.Message)
		num, err := tcpConn.Read(buf[0:])
		if err != nil {
			fmt.Println("#Network: Failed to read message from tcpConn, exiting goroutine and cleaning up connection. The error was: " + err.Error())
			fmt.Println("#Network: " + fmt.Sprintf("%+v", err))
			break
		}

		err = json.Unmarshal(buf[0:num], &message)
		if err != nil {
			fmt.Println("#Network: Failed to UNMARSHAL message. Error: " + err.Error())
		}
		//fmt.Println("#Network: SUCCESS, the buf is: " + string(buf[0:]))

		rChan <- *message
	}
}

func cleanUpConns() error {
	for _, conn := range conns {
		conn.Close()
	}
	return nil
}

func buildProposersMap(proposers []int, config network.ConfigInstance) {
	localPort := *portseed
	for _, id := range proposers {
		for _, node := range config.Nodes {
			if node.Id == id {
				rIp := node.Ip
				rPort := node.ServerPort
				rAddr, err := net.ResolveTCPAddr("tcp", rIp+":"+strconv.Itoa(rPort))
				if err != nil {
					fmt.Printf("#Error: Unable to resolve TCP address from IP: %v and Port: %v. With error: %v", rIp, rPort, err.Error())
					os.Exit(1)
				}
				localPort++
				lAddr, err := net.ResolveTCPAddr("tcp", *ip+":"+strconv.Itoa(localPort))
				if err != nil {
					fmt.Printf("#Error: Unable to resolve TCP address from IP: %v and Port: %v. With error: %v", *ip, localPort, err.Error())
					os.Exit(1)
				}
				proposersMap[id] = Node{
					IP:    rIp,
					Port:  rPort,
					RAddr: rAddr,
					LAddr: lAddr,
				}
			}
		}
	}
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
