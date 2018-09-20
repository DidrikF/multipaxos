## Answers to Paxos Questions 


#1) Is it possible that Paxos enters an infinite loop? Explain.
Yes, it is possible if conflicting Proposers exist in the system which all try to propose a new round with ever increasing round numbers. With one always upping the others last proposed round number, the algorithm cannot make any progress. It never gets to phase 2, because the acceptors cannot Accept a value with a round number lower than the last round number it sent a promise for.
For this to happen the leaderdetector has to fail.

#2) Is the value to agree on included in the Prepare message?
No, the value to agree on is not included in the Prepare messages. Only the current round number (proposal number) is included in the Prepare message from the Proposer.

#3) Does Paxos rely on an increasing proposal/round number in order to work? Explain.
Paxos rely on increasing round numbers to work because Paxos facilitates a distributed state machine. If the "current state" of each replica of the distributed state machine is to be the same, the transitions between states needs to happen in the same order in all the replicas. Without an increasing round number, the algorithm would not be able to perform state transitions in the same order across the system.

#4) Look at this description for Phase 1B: If the proposal number N is larger than any previous proposal, then each Acceptor promises not to accept proposals less than N, and sends the value it last accepted for this instance to the Proposer. What is meant by “the value it last accepted”? And what is an “instance” in this case?
The value it last accepted is the value in an ACCEPT message from a Proposer, which was last received by the Acceptor. For single-decree Paxos, the value is the Acceptor's current state of the distributed state machine.

An instance, in this case, refers to a running copy of the Paxos consensus algorithm. In other words; it is an instance of a process running the algorithm.

#5) Explain, with an example, what will happen if there are multiple proposers.
Time —>
Proposer 1      Prepare(5)                   Accept(5, “foo”)                   Prepare(7)    —>
Acceptor 1        Promise(5, -)        Promise(6, -)    <ignore>    Accepted(6, “bar”) —>
Acceptor 2        Promise(5, -)        Promise(6, -)    <ignore>    Accepted(6, “bar”) —>
Proposer 2            Prepare(4)    Prepare(6)        Accept(6, “bar”)    —>
--Additional details: For this to happen, the leader detectors has to fail deciding on one leader.

Even though multiple proposers could exist in the system at the same time, no proposer would choose the same round number, because this would be coded into the system. 
Above we have a system with two proposers and two acceptors. Three sencarios for how the message flow could be in such a configuration is illustrated.
In this partucular example, having two proposers does delay system progress, but eventually the algorithm completes a round  successfully. 
We make the following key observations regarding the message flow:
1) Prepare(4) will be ignored, because Accepter 1 & 2 has sent Promise(5, -)
2) Accept(5, “foo”) is ignored, because Acceptor 1 & 2 has sent Promise(6) to Proposer 2.
3) Prepare(7) does not interfere with round 6, because it is sent after Accept(6, “bar”) and Accepted(6, “bar”) messages is sent. 

Although not explicit by the diagram:  A proposer will never go ahead and send an Accept message until it get a promise from more than the half of the acceptors.

It is also possible for the proposers to continually send Prepare messages with ever increasing round numbers, which could make it so that the Acceptors never get to "accept" an Accept message. This would happen when it gets the Accept message for round x (from Proposer A), but has sendy a Promise on round x+1 (as a response to a Prepare from Proposer B). This could theoretically continue to happen indefinitely.

#6) What happens if two proposers both believe themselves to be the leader and send Prepare messages simultaneously?
In the end, the massages do arrive in some order at each Acceptor, but this order might differ between Acceptors. If the Prepare message with the higher round number reaches an Acceptor first, then a Promise is sent and the other Prepare message is ignored. If the Prepare message with the lower round number is processed first, then a Promise is sent for both Prepare messages. After the first is sent, for the lower round number, nothing stops the Acceptor from sending a new Promise, as long as the round number for it is higher.

#7) What can we say about system synchrony if there are multiple proposers (or leaders)?
The system itself operates in an asynchronous manner, meaning that it does not rely on a global clock to orchestrate the order of events. The system has built in tolerance for message transmissions and processing to take arbitrary long time. The synchronous properties of the Paxos algorithm, that messages are processed in the same order in all parts on the system, is not compromised by the presens of multiple Proposers. Only the performance of the algorithm stands to fall.

#8) Can an acceptor accept one value in round 1 and another value in round 2? Explain.
From the Acceptors perspective, the algorithm does not prevent it from assuming a new value if an Accept message is received with a higher round number and the round number not being equal to the last accepted value.
But in normal operation this would not happen because once one Acceptor has Accepted a value, this value is sent together with all future Promise messages it sends. The Proposer receiving the Promise will then use this value (if it has the highest round number among the received Promises) in its Accept message. Once a value has been agreed upon in the system, it will not change.


#9) What must happen for a value to be “chosen”? What is the connection between chosen values and learned values?
For an value to be chosen a Proposer must receive promises and send an Accept message to Acceptors, where Acceptors "choose" this value if it has not been any other Propose messages with an higher round number. The acceptors broadcast their "chosen" value in a Learn message.
Learners listen after Learn messages from Acceptors. If they get Learn messages from more than half of the acceptors, on the same value, the value is “Learned”.
A "chosen" value can potentially be forgotten.
A "learned" value is a final state for the distributed system and can not be forgotten or changed.