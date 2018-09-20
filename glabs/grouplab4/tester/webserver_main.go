package main

import (
	"github.com/uis-dat520-s18/glabs/grouplab1/detector"
	"github.com/uis-dat520-s18/glabs/grouplab4/bankmanager4"
	"github.com/uis-dat520-s18/glabs/grouplab4/logger4"
	"github.com/uis-dat520-s18/glabs/grouplab4/multipaxos4"
	"github.com/uis-dat520-s18/glabs/grouplab4/webserver"
)

func main() {
	//The web server:
	clientValueOut := make(chan multipaxos4.Value)
	responseIn := make(chan multipaxos4.Response)
	responseOut := make(chan multipaxos4.Response)

	ld := detector.NewMonLeaderDetector([]int{1, 2, 3})
	//logger := logger4.
	logger := logger4.Logger{}
	proposer := multipaxos4.Proposer{}

	bm := bankmanager4.NewbankManager(responseOut, &proposer, &logger)

	webServer := webserver.NewWebserver(1, bm, clientValueOut, responseIn, ld)

	//webServer := &webserver.WebServer{}
	webServer.Start()
}

/*
package main

import (
	"net/http"
)

func ping(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("pong"))
}
func main() {
	http.Handle("/", http.FileServer(http.Dir("./public")))
	http.HandleFunc("/ping", ping)
	if err := http.ListenAndServe(":8080", nil); err != nil {
		panic(err)
	}
}
*/
