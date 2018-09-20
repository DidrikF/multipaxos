![UiS](../img/uis-logo-en.png)

# Lab 6: Extensions and Optimizations

| Lab 6:		| Extensions and Optimizations		|
| -------------------- 	| ------------------------------------- |
| Subject: 		| DAT520 Distributed Systems 		|
| Deadline:		| Friday Apr 20 2017 12:00		|
| Expected effort:	| 30 hours 				|
| Grading: 		| Pass/fail + Lab Exam 			|
| Submission: 		| Group					|

### Table of Contents

1. [Introduction](#introduction)
2. [Task 1: Bank Javascript Client](#task-1-bank-javascript-client)
3. [Task 2: Pipelining, Batching and Performance Evaluation](#task-2-pipelining-batching-and-performance-evaluation)
4. [Task 3: Dynamic Membership through Reconfiguration](#task-3-dynamic-membership-through-reconfiguration)
5. [Lab Exam](#lab-exam)
6. [References](#references)

## Introduction

This assignment concludes the lab project. You will extend your Paxos system by
implementing one of three available tasks. Note that no part of this lab will
be verified by Autograder. The assignment will be verified as a part of the Lab
exam.

_You should implement **one** of the three tasks listed below_.

## Task 1: Bank Javascript Client

This task is to implement a client application with a graphical user interface
for the bank service developed in Lab 5. Specification:

* The client application should have a HTML/CSS user interface and use
  Javascript for any necessary application logic. You may use a framework like
  for example [AngularJS](https://angularjs.org) or
  [React](http://facebook.github.io/react/) if you want.

* Clients and replicas (the bank service) should communicate using Websockets.
  There are several good Websocket packages available for Go, for example
  [Golang Net Repository](https://github.com/golang/net/tree/master/websocket)
  and [Gorilla Web Toolkit](https://github.com/gorilla/websocket).  This
  demands that you adjust your client handling at each replica to also use
  Websockets for communication.

* Messages exchanged between clients and replicas should be encoded using a web
  friendly format such as JSON.

* The client application should at startup prompt the user for an account
  number. The client application does for simplicity not need to perform any
  authentication. The account number given by the user should be used in every
  transaction sent. You may implement some form of simple authentication if you
  want, e.g. add a password field to the `Account` struct from the `bank`
  package.

* A client should be able send transactions of every type specified in the
  `bank` package.

* You should extend the `bank` package with an additional transaction type
  `Transfer`, which lets a client transfer a specified amount from its own
  account to another specified account. You are free the make any change to
  the `bank` package you deem necessary.

* The client user interface should contain at least three main components:
	* The account number and balance for the selected account.
	* A input section where the client can choose/specify a transaction and
	  send it to the bank service.
	* An ordered list of recent transaction requests and responses for the
	  selected account.

* The client application should have be able to perform a failover if a leader
  replica becomes unavailable. You should be able to demonstrate such a
  scenario at the lab exam.

## Task 2: Pipelining, Batching and Performance Evaluation

This task is to implement two optimizations, pipelining and batching, for the
Multi-Paxos protocol and to evaluate their performance impact. We recommended
you to study the relevant parts of [[1]] if you choose this task.

### Batching

Batching is an optimization that can provide large gains in terms of
performance. Usually in Paxos, when the leader receives a request to be
processed, it associates the request with a Paxos instance and then starts
proposing for this instance. What the batching optimization does is to combine
multiple received requests into a single Paxos instance. The leader waits a set
amount of time while it collects received requests. After this period of time,
it will pack all of the received requests in this period of time into a single
Paxos instance.

### Pipelining

In traditional Multi-Paxos when the leader receives a request and proposes it,
he must wait until the instance is finished before he can propose the next
request. Pipelining extends Multi-Paxos to support execution of multiple
instances at the same time. In this way, when the leader receives a request, it
can assign it to a new instance and propose it right away, without waiting for
the previous instances to finish. Similar to the batching optimization, the
main problem with implementing pipelining is figuring out how many instances
the leader can start in parallel. Too many parallel instances being started can
lead to overloading the system. Because of this, the number of parallel
instances needs to be bounded so as not to cause degradations in performance of
the system. An α parameter is usually used to denote the number of allowed
concurrent instances.

### Performance Evaluation

This task also involves doing a performance evaluation of your service and
protocol implementation to see how it performs under different settings.

You should focus on two main metrics:

* **Throughput:** How many requests per second can your service execute on
  average when under full load. Throughput should be measured at the replicas.

* **Latency:** What is the average round trip time (RTT) for client requests
  when the service is under full load? Latency should be measured at the
  clients.

The bank client application (from Lab 5) used in your experiments should send
one request at a time and wait for a reply before sending a new one. Clients
should, as in Lab 5, be automated to send a certain number of requests. You
should try to run enough clients so that your services is completely saturated
during the experiments. You should calculate statistics for both metrics, e.g.
mean, standard deviation, median, minimum, maximum, 90th percentile.  You
should perform experiments using different values for at least the following
variables:

* Cluster size (e.g. three or five replicas).

* Batch timeout.

* Value of the α parameter.

Results should be presented in a report using graphs and/or tables. _You should
be able to present and explain the results from the report during the Lab
exam_.

## Task 3: Dynamic Membership through Reconfiguration

Add dynamic membership into your Multi-Paxos protocol, namely, implement a
reconfiguration command to adjust the set of servers that are executing in the
system, e.g. adding or replacing nodes while the system is running.

The Paxos protocol assumes a fixed configuration, thus we must ensure that each
instance of consensus is executed by a _single configuration_.  For example, in
case of adding a new server at slot ```i```, finish executing every
lower-numbered slot, obtain the state immediately following execution of slot
```i − 1```, transfer this latest state to the new server (including
application state), then start the consensus algorithm again in the new
configuration.

You should be able to trigger a reconfiguration by sending a special
reconfiguration request from the bank client. You will be asked to demonstrate
different types of reconfiguration during the Lab exam.

Note that this task is not as simple at it may first seem. We recommended you
to study the relevant parts both [[2]](#references) and [[3]](#references) for
guidance and to get an overview of the technique.

## Lab Approval

To have your lab assignment approved, you must come to the lab during lab hours
and present your solution. This lets you present the thought process behind
your solution, and gives us more information for grading purposes and we may
also provide feedback on your solution then and there. When you are ready to
show your solution, reach out to a member of the teaching staff. It is expected
that you can explain your code and show how it works. You may show your
solution on a lab workstation or your own computer.

## References

1. Nuno Santos and André Schiper. _Tuning paxos for high-throughput with
   batching and pipelining._ In Proceedings of the 13th international
   conference on Distributed Computing and Networking, ICDCN’12, pages 153–167,
   Berlin, Heidelberg, 2012. Springer-Verlag. [[pdf]](resources/santos12.pdf)

2. Leslie Lamport, Dahlia Malkhi, and Lidong Zhou. _Reconfiguring a state
   machine._ SIGACT News, 41(1):63–73, March 2010.
   [[pdf]](resources/reconfig10.pdf)

3. Jacob R. Lorch, Atul Adya, William J. Bolosky, Ronnie Chaiken, John R.
   Douceur, and Jon Howell. _The smart way to migrate replicated stateful
   services._ In Proceedings of the 1st ACM SIGOPS/EuroSys European Conference
   on Computer Systems 2006, EuroSys ’06, pages 103–115, New York, NY, USA, 2006. ACM.
   [[pdf]](resources/smart06.pdf)
