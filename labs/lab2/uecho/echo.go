package main

import (
	"flag"
	"fmt"
	"os"
)

var (
	help = flag.Bool(
		"help",
		false,
		"Show usage help",
	)
	server = flag.Bool(
		"server",
		false,
		"Start echo server if true; otherwise start the echo client",
	)
	endpoint = flag.String(
		"endpoint",
		"localhost:12110",
		"Endpoint on which server runs or to which client connects",
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
	if *server {
		//fmt.Println(*endpoint, *server) //"locahost:12100" true
		server, err := NewUDPServer(*endpoint)
		check(err)

		server.ServeUDP()
		<-make(chan struct{}) //???
	} else {
		clientLoop(*endpoint)
	}
}

func check(err error) {
	fmt.Println("Checking for errors\n")
	fmt.Println(err)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Fatal error: %s\n", err.Error())
		os.Exit(1)
	}
}
