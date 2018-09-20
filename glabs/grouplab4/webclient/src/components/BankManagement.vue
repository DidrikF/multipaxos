<template>
    <div style="padding: 20px">
        <h4 class="title is-3">Bank Management</h4>
        <button class="button is-primary inline-block" @click="getStatuses()">Get Account Statuses</button>
        <button class="button is-primary inline-block" @click="getHistories()">Get Account Histories</button>
        <button class="button is-primary inline-block" @click="toggleLogging()">Toggle Logging of WebSocket Messages to Console</button>
        
        <div>
            <h4 class="title is-5" style="margin-bottom: 4px; margin-top: 5px;">Leader Updates</h4>
            <span class="tag is-info" style="margin-right: 2px;" :key="index" v-for="(leader, index) in leaders">{{ leader }}</span> 
        </div>
        
        <!-- Accounts -->
        <div>
            <h4 class="title is-4" style="margin-bottom: 5px; margin-top: 20px;">Account Statuses</h4>
            <table class="table is-striped is-narrow">
                <thead>
                    <tr>
                        <th>Index</th>
                        <th>Account Number</th>
                        <th>Account Name</th>
                        <th>Balance</th>
                    </tr>
                </thead>
                <tbody>
                    <tr :key="index" v-for="(account, index) in accounts">
                        <td>{{ index }}</td>
                        <td>{{ account.number }}</td>
                        <td>{{ account.name }}</td>
                        <td>{{ account.balance ? account.balance : 0 }}</td>
                    </tr>
                </tbody>
            </table>
        </div>

        <!-- Histories -->
        <div>
            <h4 class="title is-4" style="margin-bottom: 5px; margin-top: 20px;">Transaction Histories</h4>
             <table class="table is-striped is-narrow">
                <thead>
                    <tr>
                        <th>Index</th>
                        <th>Account Number</th>
                        <th>Balance</th>
                        <th>Receiver Account</th>
                        <th>Receiver Balance</th>
                        <th>Error</th>
                    </tr>
                </thead>
                <tbody>
                    <tr :key="index" v-for="(txnRes, index) in transactionHistory">
                        <td>{{ index }}</td>
                        <td>{{ txnRes.accountNum ? txnRes.accountNum : 0 }}</td>
                        <td>{{ txnRes.balance ? txnRes.balance : 0 }}</td>
                        <td>{{ txnRes.receiverAccountNum }}</td>
                        <td>{{ txnRes.receiverBalance ? txnRes.receiverBalance : 0 }}</td>
                        <td>{{ txnRes.errorString }}</td>
                    </tr>
                </tbody>
            </table>
        </div>

         <!-- Decided Values -->
        <div>
            <h4 class="title is-4" style="margin-bottom: 5px; margin-top: 20px;">Decided Values</h4>
            <table class="table is-striped is-narrow">
                <thead>
                    <tr>
                        <th>Index</th>
                        <th>ClientID</th>
                        <th>Client Sequence</th>
                        <th>Noop</th>
                        <th>Account Number</th>
                        <th>Operation</th>
                        <th>Amount</th>
                        <th>Receiver</th>
                        <th>Create User</th>
                        <th>Username</th>
                        <th>Password</th>
                        <th>Create Account</th>
                        <th>Account Name</th>
                    </tr>
                </thead>
                <tbody>
                    <tr :key="index" v-for="(value, index) in decidedValues">
                        <td>{{ index }}</td>
                        <td>...{{ value.clientID.slice(24) }}</td>
                        <td>{{ value.clientSeq }}</td>
                        <td>{{ value.noop ? value.noop : false }}</td>
                        <td>{{ value.accountNum }}</td>
                        <td>{{ value.txn.op }}</td>
                        <td>{{ value.txn.amount }}</td>
                        <td>{{ value.txn.receiver }}</td>
                        <td>{{ value.createUser }}</td>
                        <td>{{ value.user.username }}</td>
                        <td>{{ value.user.password }}</td>
                        <td>{{ value.createAccount }}</td>
                        <td>{{ value.account.name }}</td>
                    </tr>
                </tbody>
            </table>
        </div>

    </div>
</template>

<script>
export default {
    data () {
        return {
            command: "",
            response: "",

            leaders: [],

            nodes: [],

            accounts: [],
            transactionHistory: [],
            decidedValues: [],

            toggleLog: false,
        }
    },

    methods: {
        getStatuses() {
            this.wsSend({
                type: "status"
            })
        },
        getHistories() {
            this.wsSend({
                type: "history"
            })
        },
        toggleLogging () {
            this.toggleLog = !this.toggleLog
            console.log("Toggle Log: " + this.toggleLog)
            this.$paxosClientHandler.log = this.toggleLog
        }
    },
    created () {
        this.wsListen('status', (message) => {
            if (message.status === 200) {
                this.accounts = message.data
            } else {
                console.log("Error from status message: ", message.status, message.error)
            }
        })

        this.wsListen('history', (message) => {
            if (message.status === 200) {
                this.decidedValues = message.data.values
                this.transactionHistory = message.data.transactionResults
            } else {
                console.log("Error from status message: ", message.status, message.error)
            }
        })

        this.wsListen('leader-update', (message) => {
            this.leaders.push(message.data)
        })
        /*
        this.wsListen('onmessage', (message) => {
            this.response = JSON.stringify(message)
        })
        */
    }
}
</script>
