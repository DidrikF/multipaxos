<template>
    <div style="padding: 20px">
        <h1 class="title is-3">Benchmark</h1>
        <div class="is-clearfix">
            <h4 class="title is-4" style="margin-bottom: 5px">Generate Random Users and Accounts</h4>
            <div class="field" style="display: inline-block">
                <label class="label">Number of users</label>
                <input class="input inline-block" style="width: 200px" type="number" v-model="amountOfUsers">
            </div>
            <div class="field" style="display: inline-block; margin-right: 35px;">
                <label class="label">Number of accounts per user</label>
                <input class="input inline-block" style="width: 200px" type="number" v-model="accountsPerUser">
                <button class="button is-link inline-block" @click="primeBenchmark()">Generate</button>
            </div>
            <div class="field">
                <h4 class="title is-4" style="margin-bottom: 5px">Run Benchmark</h4>
                <input class="input inline-block" style="width: 200px" type="number" placeholder="Number of transactions" v-model="amount">
                <button class="button is-primary inline-block" @click="runBenchmark()">GO!</button>
            </div>
        </div>
        
        <div class="columns" style="margin-top: 10px;"> 
            <div class="column">
                <div id="RTTchartContainer"></div>
                <div id="sortedChartContainer"></div>
                <div id="bellCurveContainer"></div>
            </div>
            <div class="column">
                <h4 class="title is-5">Created Users</h4>
                <table class="table is-striped is-narrow" v-if="users.length > 0">
                    <thead>
                        <tr>
                            <th>Username</th>
                            <th>Password</th>
                        </tr>
                    </thead>
                    <tbody>
                        <tr :key="index" v-for="(user, index) in users">
                            <td>{{ user.username }}</td>
                            <td>{{ user.password }}</td>
                        </tr>
                    </tbody>
                </table>

                <h4 class="title is-5">Created Accounts</h4>
                <table class="table is-striped is-narrow" v-if="accounts.length > 0">
                    <thead>
                        <tr>
                            <th>Number</th>
                            <th>Name</th>
                            <th>Owner</th>
                            <th>Balance</th>
                        </tr>
                    </thead>
                    <tbody>
                        <tr v-bind:key="index" v-for="(account, index) in accounts">
                            <td>{{ account.number }}</td>
                            <td>{{ account.name }}</td>
                            <td>{{ account.owner.username }}</td>
                            <td>{{ account.balance ? account.balance : 0 }}</td>
                        </tr>
                    </tbody>
                </table>

                <h4 class="title is-5" style="margin-bottom: 5px">Response Time Statistics</h4>
                <table class="table is-bordered is-striped is-narrow is-hoverable">
                    <thead>
                        <tr>
                            <th>Statistic</th>
                            <th>Result (ms)</th>
                        </tr>
                    </thead>
                    <tbody>
                        <tr><td>Mean</td><td>{{ mean }}</td></tr>
                        <tr><td>Minimum</td><td>{{ minimum }}</td></tr>
                        <tr><td>Maximum</td><td>{{ maximum }}</td></tr>
                        <tr><td>Median</td><td>{{ median }}</td></tr>
                        <tr><td>Sample Variance</td><td>{{ sampleVariance }}</td></tr>
                        <tr><td>Sample Standard Deviation</td><td>{{ sampleStdev }}</td></tr>

                    </tbody>
                </table>
            </div>
        </div>
        <div>
            <h4 class="title is-5">Transactions made on last run of benchmark</h4>
            <table class="table is-striped is-narrow" v-if="accounts.length > 0">
                <thead>
                    <tr>
                        <th>Index</th>
                        <th>Account</th>
                        <th>Operation</th>
                        <th>Amount</th>
                        <th>Receiver Account Number</th>
                    </tr>
                </thead>
                <tbody>
                    <tr :key="index" v-for="(message, index) in transactionMessagesFromLastBenchmark">
                        <td>{{ index }}</td>
                        <td>{{ message.data.accountNum }}</td>
                        <td>{{ message.data.txn.op }}</td>
                        <td>{{ message.data.txn.amount }}</td>
                        <td>{{ message.data.txn.receiver }}</td>
                    </tr>
                </tbody>
            </table>


            <h4 class="title is-5">Transaction History</h4>
            <table class="table is-striped is-narrow" v-if="accounts.length > 0">
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

    </div>
</template>

<script>
// mport Highcharts from 'highcharts';
import faker from 'faker';
// require('highcharts-histogram-bellcurve')
// require('../../node_modules/highcharts/modules/histogram-bellcurve')

export default {
    data () {
        return {
            rttChart: null,
            sortedChart: null,
            responseTimes: [], // 23, 234, 23, 23, 43, 45, 4, 45, 87, 67, 45, 23, 77
            runningBenchmark: false,
            
            amount: 100,
            amountOfUsers: 5,
            accountsPerUser: 2,
            accounts: [],
            users: [],

            transactionMessagesFromLastBenchmark: [],
            transactionHistory: [],
            responseTimeNumber: 0,

            benchmarkDone: false,
            mean: 0,
            minimum: 0,
            maximum: 0,
            median: 0,
            sampleVariance: 0,
            sampleStdev: 0,
        }
    },
    computed: {
        sortedResponseTimes () {
            return this.responseTimes.sort((a, b) => {
                return a - b
            })
        }
    },
    props: ['user'],
    methods: {
        primeBenchmark () {
            this.users = [];
            this.accounts = [];
            const registerMessages = this.generateRegisterMessages()
            const accountMessages = this.generateAccountMessages(registerMessages)
            // console.log(registerMessages, accountMessages)
            this.wsSendArray([...registerMessages, ...accountMessages])

        },
        runBenchmark () {
            this.responseTimes = []
            // const randomValueMessage = this.createRandomValueMessages()
            const randomTransactionMessages = this.generateTransactionMessages()
            //console.log(randomTransactionMessages)
            this.responseTimeNumber = 0
            this.runningBenchmark = true
            this.benchmarkDone = false
            this.transactionMessagesFromLastBenchmark = randomTransactionMessages
            this.transactionHistory = []
            this.wsSendArray(randomTransactionMessages)
        },
        createRandomValueMessages (amount) {
            const randomValueMessages = []
            for (let i = 0; i < this.amount; i++) {
                randomValueMessages.push({
                    type: "transaction",
                    data: {
                        accountNum: 1,
                        txn: {
                            op: 1,
                            amount: i,
                        },
                    },
                    auth: this.user,
                })
            }

            return randomValueMessages
        },
        generateRegisterMessages() {
            const registerMessages = []
            for (let i = 0; i < this.amountOfUsers; i++) {
                registerMessages.push({
                    type: "register",
                    data: {
                        createUser: true,
                        user: {
                            username: faker.internet.userName(),
                            password: faker.internet.password(),
                        }
                    }
                })
            }
            return registerMessages
        },
        generateAccountMessages(registerMessages) {
            const accountMessages = []
            registerMessages.forEach((registerMessage) => {
                for (let i = 0; i < this.accountsPerUser; i++) {
                    accountMessages.push({
                        type: "create-account",
                        data: {
                            createAccount: true,
                            account: {
                                name: faker.finance.accountName(),
                                owner: {
                                    username: registerMessage.data.user.username,
                                    password: registerMessage.data.user.password
                                }
                            }
                        },
                        auth: {
                            username: registerMessage.data.user.username,
                            password: registerMessage.data.user.password,
                        }
                    })
                }
            })
            return accountMessages
        },
        generateTransactionMessages() {
            if (this.accounts.length === 0) {
                console.log("No accounts available to generate transaction messages. You need to prime the system before running a benchmark.")
            }
            
            const transactionMessages = []

            for (let i = 0; i < this.amount; i++) {
                let accountIndex = i%this.accounts.length
                let account = this.accounts[accountIndex]
                transactionMessages.push({
                        type: "transaction",
                        data: {
                            accountNum: account.number,
                            txn: {
                                op: Math.floor(Math.random()*4), // random number 0,1,2,3
                                amount: Math.floor(faker.finance.amount()),// ramdon number -5000, 5000
                                receiver: this.accounts[Math.floor(Math.random()*this.accounts.length)].number// random account from this.accounts
                            }
                        },
                        auth: account.owner
                    })
            }  
            return transactionMessages         
        },
        reset () {
            
        },
        drawRTTGraph() {
            this.rttChart = Highcharts.chart('RTTchartContainer', {
                chart: {
                    type: 'line'
                },
                title: {
                    text: 'RTT Chronologically'
                },
                xAxis: {
                    // slotIDs
                    title: {
                        text: 'SlotIDs'
                    }
                },
                yAxis: {
                    title: {
                        text: 'Milli seconds'
                    }
                },
                series: [{
                    name: 'RTT chronologically',
                    data: this.responseTimes
                }]
            });
        },
        drawSortedGraph() {
            this.sortedChart = Highcharts.chart('sortedChartContainer', {
                chart: {
                    type: 'line'
                },
                title: {
                    text: 'RTT Sorted'
                },
                xAxis: {
                    // slotIDs
                    title: {
                        text: 'Responses'
                    }
                },
                yAxis: {
                    title: {
                        text: 'Milli seconds'
                    }
                },
                series: [{
                    name: 'RTT Sorted',
                    data: this.sortedResponseTimes
                }]
            });
        },
        drawBellCurve () {
            this.sortedChart = Highcharts.chart('bellCurveContainer', {
                title: {
                    text: 'Bell curve of Response Times'
                },

                xAxis: [{
                    title: {
                        text: 'Responses'
                    },
                    alignTicks: false
                }, {
                    title: {
                        text: 'Response Times'
                    },
                    alignTicks: false,
                    opposite: true
                }],

                yAxis: [{
                    title: { text: 'Milli Seconds' }
                }, {
                    title: { text: 'Probability' },
                    opposite: true
                }],

                series: [{
                    name: 'Bell curve',
                    type: 'bellcurve',
                    xAxis: 1,
                    yAxis: 1,
                    baseSeries: 1,
                    zIndex: -1
                }, {
                    name: 'Data',
                    type: 'scatter',
                    data: this.responseTimes,
                    marker: {
                        radius: 1.5
                    }
                }]
            });
        },
        calculateStatistics() {
            const responseTimes = this.sortedResponseTimes
            const amount = this.responseTimes.length
            const sum = responseTimes.reduce((sum, value) => {
                return sum + value
            }, 0)
            this.mean = sum / amount
            this.minimum = responseTimes[0]
            this.maximum = responseTimes[amount - 1]
            // median
            if (amount%2 != 0) {
                this.median = responseTimes[(amount+1)/2]
            } else {
                this.median = (responseTimes[amount/2] + responseTimes[(amount+2)/2]) / 2
            }
            // variance -> standard deviation
            let sumSquaredDifference = 0
            responseTimes.forEach((time) => {
                sumSquaredDifference += Math.pow(time-this.mean, 2)
            })

            this.sampleVariance =  sumSquaredDifference / (amount - 1)
            this.sampleStdev = Math.sqrt(this.sampleVariance)
        }
    },
    mounted () {
        this.wsListen('response-time', (responseTime) => {
            this.responseTimeNumber++
            //console.log("response time:", this.responseTimeNumber, this.amount)
            this.responseTimes.push(responseTime)
            if (this.responseTimeNumber === parseInt(this.amount, 10) && this.runningBenchmark) {
                this.responseTimeNumber = 0
                this.benchmarkDone = true
                setTimeout(() => {
                    this.runningBenchmark = false
                }, 1000)
                this.drawRTTGraph()
                this.drawBellCurve()
                this.drawSortedGraph()
                this.calculateStatistics()
            }
            
        })

        // Display the results of the transaction
        this.wsListen("register", (message) => {
            if (message.status === 201) {
                this.users.push(message.data.user)
            } else {
                console.log("Error in register response: ", message.status, message.error)                
            }
        })

        // register listeners
        this.wsListen("create-account", (message) => {
            if (message.status === 201) {
                this.accounts.push(message.data.account)
            } else {
                console.log("Error in create-account response: ", message.status, message.error)
            }
        })

        this.wsListen("transaction", (message) => {
            if (this.runningBenchmark === false) {
                return
            }
            if (message.status === 200) {
                this.transactionHistory.push(message.data.txnRes)
            } else {
                console.log("Error in transaction response: ", message.status, message.error)
            }
        })

        this.drawRTTGraph()
        this.drawBellCurve()
        this.drawSortedGraph()
        this.calculateStatistics()
       
    }
}
</script>
