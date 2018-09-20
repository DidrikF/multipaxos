<template>
    <div>
        <navigation :user="user" v-on:logout="logout()"></navigation>
        <div class="page">
            <keep-alive> <!-- The Keep-alive component wrapper keeps the containing components from being destroyed when switching between pages, allows us to keep the state of our components -->
                <router-view v-bind:user="user">
                    <!-- vue component currenly routed to is displayed here -->
                </router-view>
            </keep-alive>
        </div>


    </div>

</template>

<script>

import NavigationComponent from './Navigation.vue';
import localforage from 'localforage';

export default {
    data () {
        return {
            user: {
                username: "", // "Didrik Fleischer"
                password: "", // "password123"
            },
        }
    },
    computed: {

    },
    methods: {
        logout() {
            // log user out
            localforage.removeItem('user')
                .then(() => {
                    this.user = null
                    console.log('Logout')
                    //redirect to homepage
                    document.location.replace('/')
                
                }).catch((error) => {
                    console.log("Localforage error when logging out: ", error)
                })
           
        }
    },
    created() {
        localforage.getItem('user')
        .then(user => {
            this.user = user
        }).catch(error => {
            console.log("Localforage error when getting 'user': " + error)
        })

        
    },
    components: {
        navigation: NavigationComponent,
    }
}
</script>

<style lang="sass" scoped>

</style>
