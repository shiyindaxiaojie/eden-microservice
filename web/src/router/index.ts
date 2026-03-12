import { createRouter, createWebHistory } from 'vue-router'

const routes = [
  {
    path: '/',
    name: 'Dashboard',
    component: () => import('../views/dashboard.vue'),
    meta: { title: '仪表盘' }
  },
  {
    path: '/services',
    name: 'Services',
    component: () => import('../views/services.vue'),
    meta: { title: '服务列表' }
  },
  {
    path: '/services/:name',
    name: 'ServiceDetail',
    component: () => import('../views/service-detail.vue'),
    meta: { title: '服务详情' }
  },
  {
    path: '/cluster',
    name: 'Cluster',
    component: () => import('../views/cluster.vue'),
    meta: { title: '集群节点' }
  }
]

const router = createRouter({
  history: createWebHistory(),
  routes,
})

router.beforeEach((to) => {
  document.title = `${to.meta.title || 'Overview'} - 注册中心`
})

export default router
