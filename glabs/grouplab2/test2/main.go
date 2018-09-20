package main

import (
	"fmt"
	"os"
	"os/signal"
	"time"

	"github.com/uis-dat520-s18/glabs/grouplab1/detector"
	"github.com/uis-dat520-s18/glabs/grouplab1/network"
	"github.com/uis-dat520-s18/glabs/grouplab2/singlepaxos"
)

func main() {
	ch1 := make(chan network.Message, 16)
	ch2 := make(chan network.Message, 16)
	ch3 := make(chan network.Message, 16)
	clientCh := make(chan singlepaxos.Value, 16)
	//var proposer *singlepaxos.Proposer

	testValue := singlepaxos.Value("test value")

	//Go routine 1
	go func() {
		ld := detector.NewMonLeaderDetector([]int{1})

		prepareOut := make(chan singlepaxos.Prepare)
		acceptOut := make(chan singlepaxos.Accept)
		proposer := singlepaxos.NewProposer(1, 3, ld, prepareOut, acceptOut, []int{1, 2, 3})

		promiseOut := make(chan singlepaxos.Promise)
		learnOut := make(chan singlepaxos.Learn)
		acceptor := singlepaxos.NewAcceptor(1, promiseOut, learnOut, []int{1}, []int{1})

		valueOut := make(chan singlepaxos.Value)
		learner := singlepaxos.NewLearner(1, 3, valueOut, []int{1, 2, 3})

		proposer.Start()
		acceptor.Start()
		learner.Start()

		for {
			select {
			case clientValue := <-clientCh:
				fmt.Printf("\n#%d - Deliver Client Value: %+v", proposer.Id, clientValue)
				proposer.DeliverClientValue(clientValue)
			case prepare := <-prepareOut:
				prepareMsg := network.Message{
					Type:    "prepare",
					From:    prepare.From,
					Prepare: prepare,
				}
				fmt.Printf("\n#%d - Prepare message out: %+v", proposer.Id, prepare)
				ch1 <- prepareMsg
				ch2 <- prepareMsg
				ch3 <- prepareMsg

			case accept := <-acceptOut:
				acceptMsg := network.Message{
					Type:   "accept",
					From:   accept.From,
					Accept: accept,
				}
				fmt.Printf("\n#%d - AcceptMessage message out: %+v", acceptor.Id, accept)
				ch1 <- acceptMsg
				ch2 <- acceptMsg
				ch3 <- acceptMsg

			case promise := <-promiseOut:
				promiseMsg := network.Message{
					Type:    "promise",
					To:      promise.To,
					From:    promise.From,
					Promise: promise,
				}
				fmt.Printf("\n#%d - Promise message out: %+v", acceptor.Id, promise)
				ch1 <- promiseMsg
			case learn := <-learnOut:
				learnMsg := network.Message{
					Type:  "learn",
					From:  learn.From,
					Learn: learn,
				}
				fmt.Printf("\n#%d - Learn message out: %+v", acceptor.Id, learn)
				ch1 <- learnMsg
			case value := <-valueOut:
				fmt.Printf("\n#1 - #Learner: %v\n", value)
				//TEST:
				if value != testValue {
					fmt.Printf("\n#1 - #Learner: VALUE WAS WRONG\n")
					//t.Errorf("Got wrong value. Expected: test, Got: %v", value)
				}

			//INPUT
			case msg := <-ch1:
				switch {
				case msg.Type == "prepare":
					fmt.Printf("\n#1 - Prepare message in: %+v", msg.Prepare)
					acceptor.DeliverPrepare(msg.Prepare)
				case msg.Type == "accept":
					fmt.Printf("\n#1 - Accept message in: %+v", msg.Accept)
					acceptor.DeliverAccept(msg.Accept)
				case msg.Type == "promise":
					fmt.Printf("\n#1 - Promise message in: %+v", msg.Promise)
					proposer.DeliverPromise(msg.Promise)
				case msg.Type == "learn":
					fmt.Printf("\n#1 - Learn message in: %+v", msg.Learn)
					learner.DeliverLearn(msg.Learn)
				}
			}
		}

	}()

	//Go routine 2
	go func() {
		promiseOut := make(chan singlepaxos.Promise)
		learnOut := make(chan singlepaxos.Learn)
		acceptor := singlepaxos.NewAcceptor(2, promiseOut, learnOut, []int{1}, []int{1})

		acceptor.Start()

		for {
			select {
			case promise := <-promiseOut:
				promiseMsg := network.Message{
					Type:    "promise",
					To:      promise.To,
					From:    promise.From,
					Promise: promise,
				}
				//fmt.Printf("\n#2 - Promise message out: %+v", promise)
				ch1 <- promiseMsg
			case learn := <-learnOut:
				learnMsg := network.Message{
					Type:  "learn",
					From:  learn.From,
					Learn: learn,
				}
				//fmt.Printf("\n#2 - Learn message out: %+v", learn)
				ch1 <- learnMsg
			case msg := <-ch2:
				switch {
				case msg.Type == "prepare":
					fmt.Printf("\n#2 - Prepare message in: %+v", msg.Prepare)
					acceptor.DeliverPrepare(msg.Prepare)
				case msg.Type == "accept":
					acceptor.DeliverAccept(msg.Accept)
					fmt.Printf("\n#2 - Accept message in: %+v", msg.Accept)
				}
			}
		}
	}()

	//Go routine 3
	go func() {
		promiseOut := make(chan singlepaxos.Promise)
		learnOut := make(chan singlepaxos.Learn)
		acceptor := singlepaxos.NewAcceptor(3, promiseOut, learnOut, []int{1}, []int{1})

		acceptor.Start()

		for {
			select {
			case promise := <-promiseOut:
				promiseMsg := network.Message{
					Type:    "promise",
					To:      promise.To,
					From:    promise.From,
					Promise: promise,
				}
				//fmt.Printf("\n#3 - Promise message out: %+v", promise)
				ch1 <- promiseMsg
			case learn := <-learnOut:
				learnMsg := network.Message{
					Type:  "learn",
					From:  learn.From,
					Learn: learn,
				}
				//fmt.Printf("\n#3 - Learn message out: %+v", learn)
				ch1 <- learnMsg
			case msg := <-ch3:
				switch {
				case msg.Type == "prepare":
					fmt.Printf("\n#3 - Prepare message in: %+v", msg.Prepare)
					acceptor.DeliverPrepare(msg.Prepare)
				case msg.Type == "accept":
					acceptor.DeliverAccept(msg.Accept)
					fmt.Printf("\n#3 - Accept message in: %+v", msg.Accept)
				}
			}
		}
	}()

	clientCh <- testValue
	time.Sleep(5 * time.Second)
	clientCh <- singlepaxos.Value("foobar2")
	osSignalChan := make(chan os.Signal, 1)
	signal.Notify(osSignalChan, os.Interrupt) //^C

	for {
		select {
		//This code is supposed to kill a lot of goroutines, and sometimes the scheduler gives control to a goroutine that is supposed to be killed, is causes some strange behavior, like looping many times over the server listener, which is closed in CleanupNetwork.
		case <-osSignalChan: //Less frequent, but SOMETIMES THIS FAIL?
			fmt.Println("\nEXITING")

			os.Exit(0)
		}
	}

}
