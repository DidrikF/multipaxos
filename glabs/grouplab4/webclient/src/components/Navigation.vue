<template>
    <div class="Navigation">
            <div class="Navigation__header">
                <span><i class="fas fa-university"></i>PAXOS-Bank Client</span>
            </div>
            <div class="Navigation__section">
                <ul class="Navigation__list">
                    <li class="item1"><router-link class="Navigation__item current-link" :to="{ name: 'bank' }"><i class="fas fa-dollar-sign"></i><span class="Navigation__item-text">Bank</span></router-link></li>
                    <li class="item1"><router-link class="Navigation__item" :to="{ name: 'management' }"><i class="fas fa-book"></i><span class="Navigation__item-text">Bank Management</span></router-link></li>
                    <li class="item1"><router-link class="Navigation__item" :to="{ name: 'benchmark' }"><i class="fas fa-stopwatch"></i><span class="Navigation__item-text">Benchmark</span></router-link></li>
                    <li class="item1"><router-link class="Navigation__item" :to="{ name: 'logs' }"><i class="fas fa-server"></i><span class="Navigation__item-text">Logs</span></router-link></li>                    
                </ul>
            </div>
            <div class="Navigation__userinfo">
                <a href="http://didrikfleischer.com">DidrikFleischer.com</a>
            </div>
            <div class="Navigation__userinfo">
                <span>User: {{ user.username }}</span>
                <a class="Logout" @click="$emit('logout')">Logout</a>
            </div>
    </div>
</template>

<script>


export default {
    props: ['user'],
}


</script>



<style lang="sass" scoped>

@import '~styles/style.sass'

.Navigation
    display: flex
    background: $darkGrey
    color: $veryLightGrey 
    width: 100%
    height: 50px
    align-items: center
    min-width: 750px

.Navigation__header
    margin-left: 20px
    padding-right: 20px
    border-right: 1px solid $veryLightGrey
    height: 50px
    display: flex
    align-items: center
    justify-content: center // dont think i need this
    i
        margin-right: 5px

.Navigation__section

.Navigation__list
    display: flex
    list-style: none

.Navigation__item
    display: block
    height: 50px
    text-decoration: none
    padding-right: 20px
    padding-left: 10px
    cursor: pointer
    display: flex
    align-items: center
    justify-content: center // dont think i need this
    color: $veryLightGrey
    transition: color 0.1s linear 0s,background-color 0.1s linear 0s,opacity 0.1s linear 0s
    box-sizing: border-box

.Navigation__item-text
    margin-left: 5px

    
.Navigation__item:hover, .Navigation__item:active // .Navigation__item:focus, 
    color: $almostWhite
    border-bottom: 3px solid $green

.Navigation__userinfo
    margin-left: auto
    margin-right: 20px
    font-size: 12px

.material-icons
    font-size: 15px
    cursor: pointer
    vertical-align: middle;
    padding-bottom: 2px

.router-link-exact-active
    border-bottom: 3px solid $green

.Logout
    display: block
    cursor: pointer
    text-decoration: underline

.Logout:hover, .Logout:active, .Logout:focus
        color: $almostWhite

</style>
