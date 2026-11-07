import { createRouter, createWebHistory, type RouteRecordRaw } from 'vue-router'
import HostStorePage from '../features/host-store/views/HostStorePage.vue'

const routes: RouteRecordRaw[] = [
  { path: '/', redirect: '/host-store' },
  { path: '/host-store', name: 'host-store', component: HostStorePage },
  { path: '/:pathMatch(.*)*', redirect: '/host-store' }
]

export const router = createRouter({ history: createWebHistory(), routes })
