// +build !solution

// Leave an empty line above this comment.
package main

import (
	"flag"
	"fmt"
	"net"
	"os"

	"golang.org/x/net/context"
	"google.golang.org/grpc"

	pb "github.com/uis-dat520-s18/labs/lab2/grpc/proto"
)

type keyValueServicesServer struct {
	kv map[string]string
	// TODO (student): Add fields if needed
}

var (
	help = flag.Bool(
		"help",
		false,
		"Show usage help",
	)
	endpoint = flag.String(
		"endpoint",
		"localhost:12111",
		"Endpoint on which server runs or to which client connects",
	)
)

func Usage() {
	fmt.Fprintf(os.Stderr, "Usage: %s [OPTIONS]\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "\nOptions:\n")
	flag.PrintDefaults()
}

//**************************************************************************************************************
// The Insert() gRPC inserts a key/value pair into the map.
// Input:  ctx     The context of the client's request.
//         req     The request from the client. Contains a key/value pair.
// Output: (1)     A response to the client containing whether or not the insert was successful.
//         (2)     An error (if any).
//**************************************************************************************************************
func (s *keyValueServicesServer) Insert(ctx context.Context, req *pb.InsertRequest) (*pb.InsertResponse, error) {
	s.kv[req.Key] = req.Value

	return &pb.InsertResponse{Success: true}, nil
}

//**************************************************************************************************************
// The Lookup() gRPC returns a value corresponding to the key provided in the input.
// Input:  ctx     The context of the client's request.
//         req     The request from the client. Contains a key pair.
// Output: (1)     A response to the client containing the value corresponding to the key.
//         (2)     An error (if any).
//**************************************************************************************************************
func (s *keyValueServicesServer) Lookup(ctx context.Context, req *pb.LookupRequest) (*pb.LookupResponse, error) {
	// TODO (student): Implement function Lookup

	return &pb.LookupResponse{Value: "Initial value"}, nil
}

//**************************************************************************************************************
// The Keys() gRPC returns a slice listing all the keys.
// Input:  ctx     The context of the client's request.
//         req     The request from the client.
// Output: (1)     A response to the client containing a slice of the keys.
//         (2)     An error (if any).
//**************************************************************************************************************
func (s *keyValueServicesServer) Keys(ctx context.Context, req *pb.KeysRequest) (*pb.KeysResponse, error) {
	// TODO (student): Implement function Keys

	return &pb.KeysResponse{Keys: []string{"Initial", "value"}}, nil
}

func main() {
	flag.Usage = Usage
	flag.Parse()
	if *help {
		flag.Usage()
		return
	}

	listener, err := net.Listen("tcp", *endpoint)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	} else {
		fmt.Printf("Listener started on %v\n", *endpoint)
	}

	server := new(keyValueServicesServer)
	server.kv = make(map[string]string)
	grpcServer := grpc.NewServer()
	pb.RegisterKeyValueServiceServer(grpcServer, server)
	fmt.Printf("Preparing to serve incoming requests.\n")
	err = grpcServer.Serve(listener)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	}
}
