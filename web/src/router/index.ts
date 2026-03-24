import { createRouter, createWebHistory } from 'vue-router'

const routes = [
  {
    path: '/',
    name: 'Dashboard',
    component: () => import('../views/dashboard.vue'),
    meta: { title: 'Overview' },
  },
  {
    path: '/services',
    name: 'Services',
    component: () => import('../views/services.vue'),
    meta: { title: 'Services' },
  },
  {
    path: '/services/:name',
    name: 'ServiceDetail',
    component: () => import('../views/service-detail.vue'),
    meta: { title: 'Service Detail' },
  },
  {
    path: '/cluster',
    name: 'Cluster',
    component: () => import('../views/cluster.vue'),
    meta: { title: 'Nodes' },
  },
  {
    path: '/rbac',
    name: 'RBAC',
    component: () => import('../views/rbac.vue'),
    meta: { title: 'RBAC' },
  },
  {
    path: '/settings',
    name: 'Settings',
    component: () => import('../views/settings.vue'),
    meta: { title: 'Settings' },
  },
  {
    path: '/login',
    name: 'Login',
    component: () => import('../views/login.vue'),
    meta: { title: 'Login', public: true },
  },
  {
    path: '/namespaces',
    name: 'Namespaces',
    component: () => import('../views/namespace.vue'),
    meta: { title: 'Namespaces' },
  },
  {
    path: '/dependency-graph',
    redirect: '/services',
  },
  {
    path: '/docs',
    name: 'Docs',
    component: () => import('../views/docs.vue'),
    meta: { title: 'Documentation' },
  },
  {
    path: '/profile',
    name: 'Profile',
    component: () => import('../views/profile.vue'),
    meta: { title: 'Personal Center' },
  },
]

const router = createRouter({
  history: createWebHistory(),
  routes,
})

router.beforeEach((to) => {
  const token = localStorage.getItem('token')
  if (!to.meta.public && !token) {
    return '/login'
  }
})

export default router

