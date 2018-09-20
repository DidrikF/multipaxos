// +build !solution

package main

import (
	"flag"
	"log"
	"net/http"
	"strconv"
)

var httpAddr = flag.String("http", ":8080", "Listen address")

func main() {
	flag.Parse()
	server := NewServer()
	log.Fatal(http.ListenAndServe(*httpAddr, server)) //func ListenAndServe(addr string, handler Handler) error
	//Handler is typically nil, in which case the DefaultServeMux is used.
}

type Server struct {
	handlers map[string]func(http.ResponseWriter, *http.Request, *Server)
	counter  int
}

func NewServer() *Server {

	mux := make(map[string]func(http.ResponseWriter, *http.Request, *Server))
	mux["/"] = homeHandler
	mux["/counter"] = counterHandler
	mux["/lab2"] = lab2Handler
	mux["/fizzbuzz"] = fizzBuzzHandler

	s := &Server{
		handlers: mux,
		counter:  0,
	}

	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	//browser is asking for favicon...
	handlers := s.handlers

	if handler, ok := handlers[r.URL.Path]; ok {
		handler(w, r, s)
		return
	}
	defaultHandler(w, r, s)
}

//Request Handlers
func homeHandler(w http.ResponseWriter, r *http.Request, s *Server) {
	s.counter = s.counter + 1

	res := []byte("Hello World!\n")
	w.WriteHeader(http.StatusOK) //this is called implicitly on first Write() call
	w.Write(res)

}

func counterHandler(w http.ResponseWriter, r *http.Request, s *Server) {
	s.counter = s.counter + 1

	res := []byte("counter: " + strconv.Itoa(s.counter) + "\n")
	w.WriteHeader(http.StatusOK)
	w.Write(res)

}

func lab2Handler(w http.ResponseWriter, r *http.Request, s *Server) {
	s.counter = s.counter + 1

	res := []byte("<a href=\"http://www.github.com/uis-dat520-s18/labs/tree/master/lab2\">Moved Permanently</a>.\n\n")
	w.WriteHeader(301)
	w.Write(res)
}

func fizzBuzzHandler(w http.ResponseWriter, r *http.Request, s *Server) {
	s.counter = s.counter + 1
	var valueParam string

	if keys, ok := r.URL.Query()["value"]; ok {
		valueParam = keys[0]
	}

	if valueParam == "" {
		w.Write([]byte("no value provided\n"))
		return
	}

	value, err := strconv.Atoi(valueParam)
	if err != nil {
		w.Write([]byte("not an integer\n"))
		return
	}

	result := fizzBuzzGame(value)

	if result == "" {
		w.Write([]byte(strconv.Itoa(value) + "\n"))
	}

	w.Write([]byte(result))
}

func defaultHandler(w http.ResponseWriter, r *http.Request, s *Server) {
	s.counter = s.counter + 1
	w.WriteHeader(404)
	w.Write([]byte("404 page not found\n"))
}

func fizzBuzzGame(val int) string {
	divBy3 := (val % 3) == 0
	divBy5 := (val % 5) == 0
	if divBy3 && !divBy5 {
		return "fizz\n"
	}
	if !divBy3 && divBy5 {
		return "buzz\n"
	}
	if divBy3 && divBy5 {
		return "fizzbuzz\n"
	}
	return ""
}
