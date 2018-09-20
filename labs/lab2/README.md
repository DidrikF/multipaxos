![UiS](http://www.ux.uis.no/~telea/uis-logo-en.png)

# Lab 2: Network Programming in Go

| Lab 2:				| Network Programming in Go				|
| -------------------- 	| ------------------------------------- |
| Subject: 				| DAT520 Distributed Systems 			|
| Deadline:				| Friday Jan 26 2015 12:00			|
| Expected effort:		| 10-15 hours			 				|
| Grading: 				| Pass/fail 							|
| Submission: 			| Individually							|

### Table of Contents

1. [Introduction](https://github.com/uis-dat520-s18/labs/blob/master/lab2/README.md#introduction)
2. [UDP Echo Server](https://github.com/uis-dat520-s18/labs/blob/master/lab2/README.md#udp-echo-server)

5. [Lab Approval](https://github.com/uis-dat520-s18/labs/blob/master/lab2/README.md#lab-approval)

## Introduction

The goal of this lab assignment is to get you started with network programming
in Go. The overall aim of the lab project is to implement a fault tolerant
distributed application, specifically a reliable bulletin board. Knowledge of
network programming in Go is naturally a prerequisite for accomplishing this.

This lab assignment consist of two parts. In the first part you are expected to
implement a simple echo server that is able to respond to different commands
specified as text. In the second part you will be implementing a simple web
server. You will need this latter skill, together with other web-design skills,
to construct a web-based user interface/front-end for your reliable bulletin
board.

The most important packages in the Go standard library that you will use in
this assignment is the [`net`](http://golang.org/pkg/net) and
[`net/http`](http://golang.org/pkg/net/http) packages. It is recommended that
you actively use the documentation available for these packages during your
work on this lab assignment. You will also find this [web
tutorial](https://golang.org/doc/articles/wiki/) highly useful.

## Tasks
1. [UDP Echo Server](https://github.com/uis-dat520-s18/labs/blob/master/lab2/uecho/README.md)

2. [Web Server](https://github.com/uis-dat520-s18/labs/blob/master/lab2/web/README.md)
3. [Remote Procedure Call](https://github.com/uis-dat520-s18/labs/blob/master/lab2/grpc/README.md)

4. [Questions](https://github.com/uis-dat520-s18/labs/blob/master/lab2/networking/README.md)

## Lab Approval

To have your lab assignment approved, you must come to the lab during lab hours
and present your solution. This lets you present the thought process behind
your solution, and gives us more information for grading purposes. When you are
ready to show your solution, reach out to a member of the teaching staff.  It
is expected that you can explain your code and show how it works. You may show
your solution on a lab workstation or your own computer. The results from
Autograder will also be taken into consideration when approving a lab. At least
60% of the Autograder tests should pass for the lab to be approved. A lab needs
to be approved before Autograder will provide feedback on the next lab
assignment.

Also see the [Grading and Collaboration
Policy](https://github.com/uis-dat520-s18/course-info/policy.md) document for
additional information.
