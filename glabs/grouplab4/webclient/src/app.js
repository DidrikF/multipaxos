//Bank client SPA
import Vue from 'vue';
import localforage from 'localforage';
import router from './router';
import PAXOSClientHandler from './ws-event-handler';
import 'bulma/css/bulma.css'

require('./styles/style.sass');

import ApplicationComponent from './components/Application.vue';

localforage.config({
    driver: localforage.LOCALSTORAGE,
    storeName: 'paxos-bank',
});

const developmentConf = new Map()
    .set(1, { url: 'ws://localhost:8001/ws', active: false })
    .set(2, { url: 'ws://localhost:8002/ws', active: false })
    .set(3, { url: 'ws://didrikfleischer.com/livedemo/multipaxos/ws', active: true })
    .set(4, { url: 'ws://localhost:8004/ws', active: false });


const paxosClientHandler = new PAXOSClientHandler(developmentConf);


Vue.use({
    install(Vue, webSocket) { 
        Vue.prototype.$paxosClientHandler = paxosClientHandler;
        Vue.mixin({
            methods: {
                wsListen(eventName, handler) {
                    this.$paxosClientHandler.on(eventName, handler)
                },
                wsSendValue(valueMessage) {
                    this.$paxosClientHandler.queueValueMessage(valueMessage)
                },
                wsSend(JSONMessage) {
                    this.$paxosClientHandler.sendMessageToLeader(JSONMessage)
                },
                wsBroadcast(JSONMessage) {
                    this.$paxosClientHandler.broadcast(JSONMessage)
                },
                wsSendArray(valueMessageArray) {
                    this.$paxosClientHandler.sendValueArray(valueMessageArray)
                }
            }
        })
    }
});

window.vm = new Vue({
    el: '#app',  
    router,
    components: {
        application: ApplicationComponent,
    }
})


