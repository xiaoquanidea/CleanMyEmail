import { createRouter, createWebHashHistory } from 'vue-router'
import type { RouteRecordRaw } from 'vue-router'

const routes: RouteRecordRaw[] = [
  {
    path: '/',
    name: 'Home',
    component: () => import('../views/HomeView.vue')
  },
  {
    path: '/account/add',
    name: 'AddAccount',
    component: () => import('../views/AddAccountView.vue')
  },
  {
    path: '/clean/:accountId',
    name: 'Clean',
    component: () => import('../views/CleanView.vue'),
    props: true
  },
  {
    path: '/settings',
    name: 'Settings',
    component: () => import('../views/SettingsView.vue')
  },
  {
    path: '/history',
    name: 'History',
    component: () => import('../views/HistoryView.vue')
  }
]

const router = createRouter({
  history: createWebHashHistory(),
  routes
})

export default router

