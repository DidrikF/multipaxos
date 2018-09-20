<template>
    <div class="container" style="padding: 20px">
        <h4 class="title is-4">Create Account</h4>
        <div class="field">
            <label class="label">Account Name</label>
            <input class="input inline-block" style="width: 200px" type="text" placeholder="Savings Account" v-model="newAccountName">
            <button class="button is-link inline-block" @click="createAccount()">Create Account</button>
        </div>


        <!--____________________________________ --> 
        <h4 class="title is-4">Your Accounts</h4>
        <table class="table is-striped is-narrow">
            <thead>
                <tr>
                    <th>Index</th>
                    <th>Account Number</th>
                    <th>Account Name</th>
                    <th>Balance</th>
                    <button class="button is-link is-small" style="top: 10px; left:10px" @click="getAccounts()">Get Accounts</button>
                </tr>
            </thead>
            <tbody>
            <tr :key="index" v-for="(account, index) in sortedAccounts">
                    <td>{{ index }}</td>
                    <td>{{ account.number }}</td>
                    <td>{{ account.name }}</td>
                    <td>{{ account.balance ? account.balance : 0 }}</td>
                </tr>
            </tbody>
        </table>
        <!--____________________________________ --> 
        <h4 class="title is-4">Perform Transaction</h4>
        <div class="field">
            <div class="control">
                <div class="select is-info">
                <select v-model="transaction.op">
                    <option value="" disabled selected>Operation</option>
                    <option value="0">Balance</option>
                    <option value="1">Deposit</option>
                    <option value="2">Withdrawal</option>
                    <option value="3">Transfer</option>
                </select>
                </div>
            </div>
        </div>

        <div class="field">
            <div class="control">
                <div class="select is-info">
                <select v-model="accountNum" >
                    <option value="" disabled selected>Account</option>
                    <option v-bind:key="index" v-for="(account, index) in usersAccounts" :value="account.number">{{ account.name }}</option>
                </select>
                </div>
            </div>
        </div>

        <div class="field">
            <div class="control">
                <input class="input is-info" type="text" placeholder="Amount" v-model="transaction.amount">
            </div>
        </div>

        <div class="field">
            <div class="control">
                <input class="input" type="text" placeholder="Receiver Account" v-model="transaction.receiver">
            </div>
        </div>
        <a class="button is-primary"  @click="performTransaction()">Process</a>


        <!-- Form to submit transactions -->
        <h4 class="title is-4">Transaction Histories of Own Accounts</h4>
        


        <table class="table is-striped is-narrow">
            <thead>
                <tr>
                    <th>Index</th>
                    <th>Account Number</th>
                    <th>Balance</th>
                    <th>Receiver Account</th>
                    <th>Receiver Balance</th>
                    <th>Error</th>
                    <a class="button is-link is-small" style="top: 10px; left:10px" @click="getTransactionHistory()">Get Account Histories</a>
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
        
</template>

<script>

export default {
    data () {
        return {
            // Related to creation and viewing of accounts
            newAccountName: "", 
            usersAccounts: [], // {number: 42, name: "Savings Account", balance: 100}, {number: 130, name: "Bills and business", balance: 100}, {number: 124612, name: "Credit card", balance: 100}
            
            // Related to performing transactions
            transaction: {
                op: "", // make numeric before sending
                amount: "",
                receiver: "",
            },
            accountNum: "",


            // Related to viewing account histories
            viewHistoryOfAccount: null, // select the account you want to view the history of
            transactionHistory: [],

            // Related to displayinig various status messages
            accountMessage: "",
            transactionMessage: "",
            historyMessage: "",

            // hack
            expectingResponse: false,
        }
    },
    props: ['user'],
    computed: {
        sortedAccounts () {
            // Sort transactions based on slotIDs
            return this.usersAccounts.sort((a,b) => {
                return a.number -b.number
            })
        }
    },
    methods: {
        createAccount () {
            this.wsSendValue({
                type: "create-account",
                data: {
                    createAccount: true,
                    account: {
                        name: this.newAccountName,
                        owner: this.user
                    }
                },
                auth: this.user,
            })
            this.expectingResponse = true
        },
        performTransaction () {
            console.log(this.transaction)

            const numericTransaction = {
                op: parseInt(this.transaction.op, 10),
                amount: parseInt(this.transaction.amount, 10),
                receiver: parseInt(this.transaction.receiver, 10),
            }

            this.wsSendValue({
                type: "transaction",
                data: {
                    accountNum: this.accountNum,
                    txn: numericTransaction,
                },
                auth: this.user
            })
            this.expectingResponse = true
        },

        getAccounts () {
            this.usersAccounts = [];
            // based on account number
            this.wsSend({
                type: "account-status",
                auth: this.user,
            });
            
        },
        getTransactionHistory () {
            this.transactionHistory = [];
            // based on account number
            this.wsSend({
                type: "account-history",
                auth: this.user,
            })
        },
        
    },
    created () {
        // register listeners
        this.wsListen("create-account", (message) => {
            // only deal with if if its your account // benchmark mode will also use this
            if (this.expectingResponse == false) return
            this.getAccounts()

            this.expectingResponse = false
        })

        this.wsListen("transaction", (message) => {
            if (!message.data) return
            // only deal with if if its your account // benchmark mode will also use this
            // if (this.expectingResponse == false) return
            this.transactionHistory.push(message.data.txnRes)
            this.getAccounts()

            // this.expectingResponse = false
        })

        this.wsListen("account-status", (message) => {
            if (message.status === 200) {
                // message.data // bank4.Account
                this.usersAccounts = message.data
            } else {
                console.log("Failed to get account status. ", message)
            }
        })

        this.wsListen("account-history", (message) => {
            if (message.status === 200) {
                // message.data // bank4.TransactionResult
                this.transactionHistory = message.data ? message.data : []
            } else {
                console.log("Failed to get account history. ", message)
            }
        })
        
        // this.getTransactionHistory()
        // this.getAccounts()

    }
}
</script>

<style lang="sass">
.field 
    display: inline-block !important
</style>
