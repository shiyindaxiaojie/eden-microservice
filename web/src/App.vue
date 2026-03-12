<script setup lang="ts">
import { useRoute, useRouter } from 'vue-router'
import { computed } from 'vue'

const route = useRoute()
const router = useRouter()

const navItems = [
  { path: '/', label: '仪表盘', icon: 'Odometer' },
  { path: '/services', label: '服务列表', icon: 'Grid' },
  { path: '/cluster', label: '集群节点', icon: 'Connection' },
]

const currentTitle = computed(() => {
  return (route.meta.title as string) || '注册中心'
})

function isActive(path: string) {
  if (path === '/') return route.path === '/'
  return route.path.startsWith(path)
}
</script>

<template>
  <div class="app-layout dark">
    <!-- Sidebar -->
    <aside class="sidebar">
      <div class="sidebar-header">
        <div class="sidebar-logo">
          <div class="logo-icon">E</div>
          <span class="logo-text">Mini Registry</span>
        </div>
      </div>
      <nav class="sidebar-nav">
        <div
          v-for="item in navItems"
          :key="item.path"
          class="nav-item"
          :class="{ active: isActive(item.path) }"
          @click="router.push(item.path)"
        >
          <el-icon><component :is="item.icon" /></el-icon>
          <span>{{ item.label }}</span>
        </div>
      </nav>
    </aside>

    <!-- Main -->
    <div class="main-content">
      <header class="main-header">
        <h1>{{ currentTitle }}</h1>
      </header>
      <div class="page-body">
        <router-view />
      </div>
    </div>
  </div>
</template>
