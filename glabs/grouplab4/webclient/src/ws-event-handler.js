import uuid from 'uuid';

// Leader may be reported to be -1

export default class PAXOSClientManager {
    constructor (config) {
        this.config = config;
        
        let leader = 0
        this.config.forEach((nodeConf, nodeID) => {
            if (nodeConf.active && nodeID > leader) {
                leader = nodeID
            }
        })

        this.leader = leader
        this.clientID = uuid();
        this.clientSeq = 0;
        this.seqRespondedTo = -1;
        this.sockets = new Map();
        this.listeners = {};
        this.valueMessageQueue = [];
        this.lastValueSentTimeStamp = null;
        this.log = false
        this.registerCommonHandlers()
        this.connectToAll();
    }

    
    registerCommonHandlers () {
        this.on('connect', (message) => { //OBS: is this correct
            const newLeader = message.data
            if (newLeader === -1) {
                return
            }
            this.leader = newLeader
        })
       
        this.on('leader-update', (message) => {
            const newLeader = message.data
            const currentLeader = this.leader
            
            if (newLeader === -1) { // ignore it when a node does not know how the leader is.
                return
            }
            
            if (newLeader === currentLeader) {
                return
            } 
        
            this.leader = newLeader
            const newLeaderSocket = this.sockets.get(newLeader)
            
            if (!newLeaderSocket || (newLeaderSocket.readyState === newLeaderSocket.CLOSED) || (newLeaderSocket.readyState === newLeaderSocket.CLOSING)) {
                const socket = new WebSocket(this.config.get(newLeader).url)
                this.registerBasicHandlers(socket)
                this.sockets.set(newLeader, socket)
            } else {
                this.handleSendingOfValue()
            }
            
        })
    }
    
    connectToAll() {
        this.config.forEach((socketConf, nodeID) => {
            if (socketConf.active == false) {
                return
            }
            const ws = new WebSocket(socketConf.url)
            this.registerBasicHandlers(ws)
            this.sockets.set(nodeID, ws)
        })
    }
    
    registerBasicHandlers (ws) {
        ws.onmessage = (event) => {
            const message = JSON.parse(event.data)
            if (message.type !== 'log-message' && this.log) {
                console.log("Got message: ", message)
            }
            
            this.emit('onmessage', event)

            if (message.type === 'register' || message.type === 'create-account' || message.type === 'transaction') {
                if (message.data && (message.data.clientSeq === this.seqRespondedTo + 1)) {
                    this.seqRespondedTo++
                    this.recordResponseTime()
                    this.handleSendingOfValue()
                    this.emit(message.type, message)
                    return
                } else {
                    //console.log("Message of type "+message.type+" was received, but the client sequence number was not seqRespondedTo + 1. No more actions will be taken")
                    return
                }
            } else if (message.type === 'connect') {
                this.handleSendingOfValue()
                this.emit(message.type, message)
            } else {
                this.emit(message.type, message)
            }

        }
        ws.onopen = (event) => {
            console.log('Websocket connection opened!' )
            this.sendConnectMessageOnWebSocket(ws)
            this.emit('onopen', event)
        }
        ws.onclose = (event) => {
            console.log('Websocket closed!')
            this.emit('onclose', event)
        }

        ws.onerror = (event) => {
            console.log("Wesocket error!")
            this.emit('onerror', event)
        }
    }

    sendConnectMessageOnWebSocket (ws) {
        ws.send(JSON.stringify({
            "type": "connect",
            "data": this.clientID,
        }))
    }
    
    reconnectToLeader () { // reconnection should happen automatically if the server is the one who went down and later come back up again.
        console.log("reconnectToLeader() called")
        const leaderSocket = this.sockets.get(this.leader)
        if (!leaderSocket || (leaderSocket.readyState === leaderSocket.CLOSED) || (leaderSocket.readyState === leaderSocket.CLOSING)) {
            console.log("socket to leader was not existing, was closed or closing, so a new websocket was created in an attempt to reconnect.")
            this.sockets.delete(this.leader)
            const ws = new WebSocket(this.config.get(this.leader).url)
            this.registerBasicHandlers(ws)
            this.sockets.set(this.leader, ws)
        }
    }
   
    handleSendingOfValue() {
       if (this.valueMessageQueue.length > 0) {

            this.valueMessageQueue = this.valueMessageQueue.filter(message => message.data.clientSeq > this.seqRespondedTo)

            let nextValueMessage = this.valueMessageQueue[this.valueMessageQueue.length-1]
           
            if (nextValueMessage.data.clientSeq === this.seqRespondedTo + 1) {
                nextValueMessage = this.valueMessageQueue.pop()
                this.sendMessageToLeader(nextValueMessage)
                
                setTimeout((nextValueMessage, oldSeqRespondedTo) => {
                    //console.log("seqRespondedTo: ",this.seqRespondedTo, ", oldSeqRespondedTo: ", oldSeqRespondedTo)
                    if (this.seqRespondedTo === oldSeqRespondedTo) { // seqRespondedTo has not been incremented
                        console.log("pushing message back on queue")
                        this.valueMessageQueue.push(nextValueMessage)
                        this.handleSendingOfValue()
                    }
                }, 2000, nextValueMessage, this.seqRespondedTo)

           }
       }
   }

    queueValueMessage(valueMessage) {
        valueMessage.data.clientID = this.clientID
        valueMessage.data.clientSeq = this.clientSeq
        this.clientSeq++

        this.valueMessageQueue.unshift(valueMessage)
        this.handleSendingOfValue()
    }

    sendValueArray(valueMessageArray) {
        valueMessageArray.forEach(valueMessage => {
            valueMessage.data.clientID = this.clientID
            valueMessage.data.clientSeq = this.clientSeq
            this.clientSeq++
            this.valueMessageQueue.unshift(valueMessage)
        })
        this.handleSendingOfValue()
    }
    
    recordResponseTime() {
        const responseTime = Date.now() - this.lastValueSentTimeStamp
        this.lastValueSentTimeStamp = null
        this.emit("response-time", responseTime)
    }


    sendMessageToLeader (message) {
        if (this.log) {
            console.log("Message being sent: ", message)
        }
        if (message.type === 'register' || message.type === 'create-account' || message.type === 'transaction') {
            this.lastValueSentTimeStamp = Date.now()
        }

        const leaderSocket = this.sockets.get(this.leader) 

        if (this.sockets.has(this.leader) && (leaderSocket.readyState !== leaderSocket.CLOSED) && (leaderSocket.readyState !== leaderSocket.CLOSING)) {
            leaderSocket.send(JSON.stringify(message))
        } else {
            console.log("Failed sending message to leader, becuase no socket to the leader was present.")
            this.reconnectToLeader()
        }
    }

    on(eventName, handler) {
        if(!Array.isArray(this.listeners[eventName])) {
            this.listeners[eventName] = []
        }

        this.listeners[eventName].push(handler)
    }

    emit(eventName, ...args) {
        const listeners = this.listeners[eventName]
        if (listeners && listeners.length) {
            listeners.forEach(handler => {
                handler.call(null, ...args)
            })
        }
    }
}

/*
connectToMissing () {
    this.config.forEach((nodeConf, nodeID, map) => {
        if (nodeConf.active == true) {
            if (this.sockets.get(nodeID) === undefined) {
                const ws = new WebSocket(nodeConf.url)
                this.registerBasicHandlers(ws)
                this.sockets.set(nodeID, ws)
            }
        }
    })
}
*/