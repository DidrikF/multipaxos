// +build !solution

// Leave an empty line above this comment.
package main

import (
	"fmt"
	"net"
	"strings"
	"unicode"
)

// UDPServer implements the UDP server specification found at
// https://github.com/uis-dat520-s18/labs/blob/master/lab2/README.md#udp-server
type UDPServer struct {
	conn *net.UDPConn

	// TODO(student): Add fields if needed

}

// NewUDPServer returns a new UDPServer listening on addr. It should return an
// error if there was any problem resolving or listening on the provided addr.

func NewUDPServer(addr string) (*UDPServer, error) {
	//resolve address
	UDPAddrPointer, err := net.ResolveUDPAddr("udp", addr)

	//return error if failed to resolve UDP address
	if err != nil {
		return nil, err
	}

	//create UDP server
	UPDConnPointer, err := net.ListenUDP("udp", UDPAddrPointer)

	//return error if failed to listen to addr
	if err != nil {
		return nil, err
	}

	//Create UDPServer value
	server := UDPServer{
		conn: UPDConnPointer,
	}

	//return pointer to server, and nil error
	return &server, nil

}

// ServeUDP starts the UDP server's read loop. The server should read from its
// listening socket and handle incoming client requests as according to the
// the specification.
func (u *UDPServer) ServeUDP() {
	//Start the UPD serveres read loop on listening socket
	connection := *((*u).conn)
	defer connection.Close()

	for {
		buf := make([]byte, 1024, 1024)
		//fmt.Println(len(buf))

		//rn, rerr := connection.Read(buf[0:]) // "cmd|:|txt"
		rn, src, rerr := connection.ReadFromUDP(buf[0:])

		//fmt.Println(string(buf))
		if rerr != nil {
			// ???
		}

		input := strings.Split(string(buf[0:rn]), "|:|")

		cmd := input[0]
		txt := input[1]

		//handle requests according to specification
		response, err := handleRequest(cmd, txt)

		if err != nil {
			// ???
		}

		//_, werr := connection.Write(response)
		_, werr := connection.WriteToUDP(response, src)

		if werr != nil {
			fmt.Println("Error when writing back to client. Error: ", werr)
		}

		//Make sure that your server continues to function even if one client's connection or datagram packet caused an error.

	}
}

func handleRequest(cmd string, txt string) ([]byte, error) {
	var response string
	var err error

	if txt == "" {
		return []byte("Unknown command"), err
	}

	switch { //break is implicit
	case cmd == "UPPER":
		response = strings.ToUpper(txt)
	case cmd == "LOWER":
		response = strings.ToLower(txt)
	case cmd == "CAMEL":
		response, err = camelCase(txt)
	case cmd == "ROT13":
		response, err = rot13(txt)
	case cmd == "SWAP":
		response, err = swap(txt)
	default:
		response = "Unknown command"
	}

	return []byte(response), err
}

func camelCase(txt string) (string, error) {

	words := strings.Split(txt, " ") // []string

	for i, word := range words { // a string is a slice of bytes
		runes := []rune(word)
		for j, r := range runes {
			if j == 0 {
				runes[0] = unicode.ToUpper(r)
				continue
			}
			runes[j] = unicode.ToLower(r)
		}

		words[i] = string(runes)
	}

	result := strings.Join(words, " ")

	return result, nil
}

func rot13(txt string) (string, error) {
	buf := []byte(txt)

	for i := 0; i < len(buf); i++ {
		switch {
		case 'a' <= buf[i] && buf[i] <= 'z':
			buf[i] = 'a' + (buf[i]-'a'+13)%26
		case 'A' <= buf[i] && buf[i] <= 'Z':
			buf[i] = 'A' + (buf[i]-'A'+13)%26
		}
	}

	return string(buf), nil
}

func swap(txt string) (string, error) {

	swapFunc := func(c rune) rune {
		if unicode.IsUpper(c) {
			c = unicode.ToLower(c)
		} else if unicode.IsLower(c) {
			c = unicode.ToUpper(c)
		}
		return c
	}

	result := strings.Map(swapFunc, txt) //map must convert the runes back into a string

	return result, nil
}

// socketIsClosed is a helper method to check if a listening socket has been
// closed.
func socketIsClosed(err error) bool {
	if strings.Contains(err.Error(), "use of closed network connection") {
		return true
	}
	return false
}
