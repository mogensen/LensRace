import { createRouter, createWebHistory } from 'vue-router'
import HomeView from '@/views/HomeView.vue'

const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  routes: [
    { path: '/', name: 'home', component: HomeView },
    {
      path: '/games/:id/lobby',
      name: 'lobby',
      component: () => import('@/views/LobbyView.vue'),
      props: true,
    },
    {
      path: '/games/:id/play',
      name: 'play',
      component: () => import('@/views/PlayView.vue'),
      props: true,
    },
    {
      path: '/games/:id/results',
      name: 'results',
      component: () => import('@/views/ResultsView.vue'),
      props: true,
    },
  ],
})

export default router
