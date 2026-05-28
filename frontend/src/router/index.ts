import { createRouter, createWebHashHistory } from 'vue-router'
import WelcomeView from '../views/WelcomeView.vue'

const router = createRouter({
  history: createWebHashHistory(),
  routes: [
    {
      path: '/',
      name: 'welcome',
      component: WelcomeView,
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
