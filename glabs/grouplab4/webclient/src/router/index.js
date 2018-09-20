import Vue from 'vue'
import Router from 'vue-router'

import BankComponent from '../components/Bank.vue';
import BankManagementComponent from '../components/BankManagement.vue';
import BenchmarkComponent from '../components/Benchmark.vue';
import LogsComponent from '../components/Logs.vue';

const routes = [
      {
        path: '/bank',
        component: BankComponent,
        name: 'bank',
        meta: {
          needsAuth: true,
        },
      },
      {
        path: '/management',
        component: BankManagementComponent,
        name: 'management',
        meta: {
          needsAuth: false,
        },
      },
      {
        path: '/benchmark',
        component: BenchmarkComponent,
        name: 'benchmark',
        meta: {
          needsAuth: false,
        },
      },
      {
        path: '/logs',
        component: LogsComponent,
        name: 'logs',
        meta: {
          needsAuth: false,
        },
      },
]

Vue.use(Router)

const router = new Router({
	routes
})

export default router