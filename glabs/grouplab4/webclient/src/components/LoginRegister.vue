<template>
    <div class="LoginRegister">
        <div class="Banner">
            <div class="Banner__icon">
                <i class="fas fa-university"></i>
            </div>
            <div>
                <h1 class="Banner__header">PAXOS-Bank Client</h1>
            </div>
            
        </div>
        <div style="text-align: center;">
            <a href="http://didrikfleischer.com">DidrikFleischer.com</a>
        </div>

        <div class="box container" v-if="formToggler === false">
            <h4 class="title is-4">Register</h4>
            <div class="field">
                <p class="control has-icons-left has-icons-right">
                    <input class="input" type="email" placeholder="Email" v-model="registerUsername">
                    <span class="icon is-small is-left">
                    <i class="fas fa-envelope"></i>
                    </span>
                    <span class="icon is-small is-right">
                    <i class="fas fa-check"></i>
                    </span>
                </p>
            </div>
            <div class="field">
                <p class="control has-icons-left">
                    <input class="input" type="password" placeholder="Password" v-model="registerPassword">
                    <span class="icon is-small is-left">
                    <i class="fas fa-lock"></i>
                    </span>
                </p>
            </div>
                <div class="field">
                <p class="control">
                    <button class="button is-success" @click="register()">
                    Register
                    </button>
                </p>
            </div>
            <p class="help is-success" v-if="this.statusMessage">{{ this.statusMessage }}</p>
            <p class="Form__note">Alleady have an account? Go to the <a @click="toggleForms()">login</a> form.</p>
        </div>
            
        <div class="box container" v-if="formToggler === true">
            <h4 class="title is-4">Login</h4>
            <div class="field" >
                <p class="control has-icons-left has-icons-right">
                    <input class="input" type="email" placeholder="Email" v-model="username">
                    <span class="icon is-small is-left">
                    <i class="fas fa-envelope"></i>
                    </span>
                    <span class="icon is-small is-right">
                    <i class="fas fa-check"></i>
                    </span>
                </p>
            </div>
            <div class="field">
                <p class="control has-icons-left">
                    <input class="input" type="password" placeholder="Password" v-model="password">
                    <span class="icon is-small is-left">
                    <i class="fas fa-lock"></i>
                    </span>
                </p>
            </div>
            <div class="field">
                <p class="control">
                    <button class="button is-success" @click="login()">
                    Login
                    </button>
                </p>
            </div>
            <p class="help is-success" v-if="this.statusMessage">{{ this.statusMessage }}</p>
            <p class="Form__note">Alleady have an account? Go to the <a @click="toggleForms()">registration</a> form.</p>
        </div>

    </div>
</template>

<script>
/*  TO DO:
    outputting multiple errors 
    style status message
*/

import localforage from 'localforage';

export default {
    data () {
        return {
            formToggler: false,
            registerUsername: "didrik",
            registerPassword: "password",
            username: "didrik",
            password: "password",
            statusMessage: "",
            ws: null,
        }
    },
    methods: { // NOTIFY vue router that user is logged in!
        toggleForms() {
            this.formToggler = !this.formToggler;
        },
        register () {
            console.log("sending register message ")

            const valueToSend = {
                type: "register",
                data: {
                    createUser: true,
                    user: {
                        username: this.registerUsername,
                        password: this.registerPassword
                    }
                }
            }

            console.log("Value to send ", valueToSend)

            this.wsSendValue(valueToSend)
            
        },
        login () {
            console.log("sending login message")
            this.wsSend({
                "type": "login",
                "auth": {
                    "username": this.username,
                    "password": this.password,
                }
            })
        }
    },
    created() {
        this.wsListen("register", (message) => {
            this.statusMessage = ""
            if (message.status == 201) {
                this.statusMessage = "Successfully registerd user"
            } else {
                message.error.forEach(error => {
                    this.statusMessage += (error + "\n")
                })
            }
        })
        
        //this.ws = new WebSocket("ws://localhost:8080/ws")
        this.wsListen("login", (message) => {
            this.statusMessage = ""
            if (message.status == 200) {
                console.log("Successfully logged in!")
                this.statusMessage = "Successfully logged in!"

                // save user in local storage
                localforage.setItem("user", {
                    username: this.username,
                    password: this.password,
                })
                // redirect to bankApp.html
                window.location.replace("/livedemo/multipaxos/bankApp.html")

            } else {
                message.error.forEach(error => {
                    this.statusMessage += (error + "\n")
                })
            }
        })
        
    }
}


</script>

<style lang="sass" scoped>
@import '~styles/style.sass'
.LoginRegister
    height: 100%

.Banner
    box-sizing: border-box
    *, *:before, *after
        box-sizing: inherit
    
    width: 100%
    height: 200px
    display: flex
    align-items: center
    justify-content: center
    font-family: "Roboto"
    font-size: 30px
    background: $darkGrey
    color: $lightGrey
    h1
        margin: 0
.Banner__icon
    display: block
    color: veryLightGrey
    font-size: 80px
    margin-right: 30px

.Content
    display: flex
    align-items: center
    justify-content: center
    padding-top: 50px

.Container
    font-family: "Roboto"
    border: 1px solid $lightGrey
    border-radius: 5px
    display: inline-block
    width: 270px
    justify-content: center
    padding: 10px
    h3
        margin: 0

.box
    width: 350px
    margin-top: 50px


.Form
    

.Form__group
    label
        display: block
    input
        display:block
        width: 95%
        margin: 0 auto

.Form__status

.Form__note
    font-size: 12px
    a
        text-decoration: underline
        cursor: pointer


.Button

.Button--submit

</style>
