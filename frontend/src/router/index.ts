import { createRouter, createWebHashHistory } from 'vue-router'
import ContainerListView from '../views/ContainerListView.vue'

const router = createRouter({
  history: createWebHashHistory(),
  routes: [
    {
      path: '/',
      name: 'containers',
      component: ContainerListView,
    },
    {
      path: '/sessions/:id',
      name: 'terminal',
      component: () => import('../views/TerminalView.vue'),
    },
    {
      path: '/sessions/:id/replay',
      name: 'replay',
      component: () => import('../views/ReplayView.vue'),
    },
  ],
})

export default router
