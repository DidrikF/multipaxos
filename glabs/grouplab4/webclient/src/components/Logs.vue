<template>
    <div style="padding: 20px">
        <h4 class="title is-4">Logs</h4>
        <div class="box container" :key="index" v-for="(node, index) in nodes">
            <h2 class="title is-2">Node {{ node.id }}</h2>
            <h4 class="title is-5">Web Server</h4>
            <div class="box container" style="max-height: 400px; overflow: scroll;"> <!-- WebServer -->
                <p :key="index" v-for="(logMessage, index) in node.webServer" v-bind:class="logMessage.severity">{{ logMessage.message }}</p>
            </div>
            <h4 class="title is-5">Web Socket Client</h4>
            <div class="box container" style="max-height: 400px; overflow: scroll;"> <!-- Web Socket Client -->
                <p :key="index" v-for="(logMessage, index) in node.client" v-bind:class="logMessage.severity">{{ logMessage.message }}</p>
            </div>
            <h4 class="title is-5">Bank Manager</h4>
            <div class="box container" style="max-height: 400px; overflow: scroll;"> <!-- BankManager -->
                <p :key="index" v-for="(logMessage, index) in node.bankmanager" v-bind:class="logMessage.severity">{{ logMessage.message }}</p>
            </div>
            <h4 class="title is-5">Network</h4>
            <div class="box container" style="max-height: 400px; overflow: scroll;"> <!-- Network -->
                <p :key="index" v-for="(logMessage, index) in node.network" v-bind:class="logMessage.severity">{{ logMessage.message }}</p>
            </div>
            <!--
             <div class="box container">
                <h3>Unknown</h3>
                <p :key="index" v-for="(logMessage, index) in node.unknown" v-bind:class="logMessage.severity">{{ logMessage.message }}</p>
            </div>
            -->
        </div> 
    </div>
</template>

<script>
    export default {
        data() {
            return {
                nodes: []
            }
        },
        methods: {

        },
        created() {
            this.wsListen('log-message', (message) => {
                let node = this.nodes.find((node) => {
                    if (node.id === message.data.id) {
                        return node
                    } 
                })

                if (!node) {
                    let newNode = {
                        id: message.data.id,
                        status: {

                        },
                        application: [],
                        webServer: [],
                        client: [],
                        bankmanager: [],
                        network: [],
                        unknown: [],
                    }
                    this.nodes.push(newNode)
                    
                    node = newNode
                }


                if (message.data.message.origin) {
                    node[message.data.message.origin].push(message.data.message)
                } else {
                    node['unknown'].push(message.data.message)
                }
            })
        }
    }
</script>

<style lang="sass" scoped>

p
    display: block
    font-size: 10px;

.emerg
    color: #ff0000
.alert
    color: #df4600
.crit
    color: #c05e00 
.err   
    color: #a46a00
.warning
    color: #7f7500
.notice
    color: #547c00
.info  
    color: #008000
.debug 
    color: black

</style>