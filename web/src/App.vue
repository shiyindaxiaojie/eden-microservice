<script setup lang="ts">
import { useRoute, useRouter } from 'vue-router'
import { computed, onMounted, ref, watch } from 'vue'
import { Moon, Sunny, QuestionFilled, ArrowDown } from '@element-plus/icons-vue'
import { useI18n } from './utils/i18n'
import logo from './assets/logo.png'

const { locale, t, toggleLocale } = useI18n()


const route = useRoute()
const router = useRouter()

const userRole = ref(localStorage.getItem('user_role') || '')
const username = ref(localStorage.getItem('username') || '')
const nickname = ref(localStorage.getItem('nickname') || '')

watch(() => route.path, () => {
  userRole.value = localStorage.getItem('user_role') || ''
  username.value = localStorage.getItem('username') || ''
  nickname.value = localStorage.getItem('nickname') || ''
})

const navItems = computed(() => {
  const items = [
    { path: '/', label: t.value.nav.dashboard, icon: 'Odometer' },
    { path: '/services', label: t.value.nav.services, icon: 'Grid' },
    { path: '/namespaces', label: t.value.nav.namespaces, icon: 'Collection' },
    { path: '/dependency-graph', label: t.value.nav.dependencies, icon: 'Share' },
    { path: '/cluster', label: t.value.nav.cluster, icon: 'Connection' },
    { path: '/rbac', label: t.value.nav.accessControl, icon: 'Lock' },
    { path: '/settings', label: t.value.nav.settings, icon: 'Setting' },
  ]
  
  if (userRole.value === 'admin') return items
  return items.filter(i => i.path === '/' || i.path === '/services' || i.path === '/cluster' || i.path === '/namespaces' || i.path === '/dependency-graph')
})


const currentTitle = computed(() => {
  if (route.path === '/') return t.value.nav.dashboard
  if (route.path.startsWith('/services')) return t.value.nav.services
  if (route.path.startsWith('/cluster')) return t.value.nav.cluster
  if (route.path.startsWith('/rbac')) return t.value.nav.accessControl
  if (route.path.startsWith('/settings')) return t.value.nav.settings
  if (route.path.startsWith('/docs')) return t.value.nav.docs
  return (route.meta.title as string) || 'Registry'
})

function handleLogout() {
  localStorage.clear()
  router.push('/login')
}


function isActive(path: string) {
  if (path === '/') return route.path === '/'
  return route.path.startsWith(path)
}

// Theme logic
const theme = ref('light')

const toggleTheme = () => {
  theme.value = theme.value === 'dark' ? 'light' : 'dark'
  applyTheme(theme.value)
  setCookie('theme', theme.value, 365)
}

const applyTheme = (t: string) => {
  document.documentElement.setAttribute('data-theme', t)
  if (t === 'dark') {
    document.documentElement.classList.add('dark')
  } else {
    document.documentElement.classList.remove('dark')
  }
}

const setCookie = (name: string, value: string, daysUp: number) => {
  const expires = new Date()
  expires.setTime(expires.getTime() + daysUp * 24 * 60 * 60 * 1000)
  document.cookie = `${name}=${value};expires=${expires.toUTCString()};path=/`
}

const getCookie = (name: string) => {
  const match = document.cookie.match(new RegExp('(^| )' + name + '=([^;]+)'))
  return match ? match[2] : null
}

onMounted(() => {
  const savedTheme = getCookie('theme') || 'light'
  theme.value = savedTheme
  applyTheme(savedTheme)
})
</script>

<template>
  <div v-if="route.meta.public" class="public-layout" :class="theme">
    <router-view />
  </div>
  
  <div v-else class="app-layout" :class="theme">
    <!-- Sidebar -->
    <aside class="sidebar">
      <div class="sidebar-header">
        <div class="sidebar-logo">
          <div class="logo-icon">
            <img :src="logo" alt="Logo" style="width: 36px; height: 36px;" />
          </div>
          <span class="logo-text">{{ t.common.title }}</span>
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

    <div v-if="!route.meta.public" class="main-content">
      <header class="main-header">
        <h1>{{ currentTitle }}</h1>
        <div class="header-actions">
          <a href="https://github.com/shiyindaxiaojie/eden-go-registry" target="_blank" class="header-btn" title="GitHub">
            <svg viewBox="0 0 16 16" fill="currentColor"><path d="M8 0C3.58 0 0 3.58 0 8c0 3.54 2.29 6.53 5.47 7.59.4.07.55-.17.55-.38 0-.19-.01-.82-.01-1.49-2.01.37-2.53-.49-2.69-.94-.09-.23-.48-.94-.82-1.13-.28-.15-.68-.52-.01-.53.63-.01 1.08.58 1.23.82.72 1.21 1.87.87 2.33.66.07-.52.28-.87.51-1.07-1.78-.2-3.64-.89-3.64-3.95 0-.87.31-1.59.82-2.15-.08-.2-.36-1.02.08-2.12 0 0 .67-.21 2.2.82.64-.18 1.32-.27 2-.27.68 0 1.36.09 2 .27 1.53-1.04 2.2-.82 2.2-.82.44 1.1.16 1.92.08 2.12.51.56.82 1.27.82 2.15 0 3.07-1.87 3.75-3.65 3.95.29.25.54.73.54 1.48 0 1.07-.01 1.93-.01 2.2 0 .21.15.46.55.38A8.013 8.013 0 0016 8c0-4.42-3.58-8-8-8z"></path></svg>
          </a>

          <router-link to="/docs" class="header-btn" :title="t.nav.docs">
            <el-icon><QuestionFilled /></el-icon>
          </router-link>

          <div class="header-btn" @click="toggleLocale" :title="locale === 'en' ? 'Switch to Chinese' : 'Switch to English'">
            <span class="lang-text">{{ locale === 'en' ? 'En' : '中' }}</span>
          </div>

          <div class="header-btn" @click="toggleTheme" :title="theme === 'dark' ? 'Switch to Light' : 'Switch to Dark'">
            <el-icon v-if="theme === 'dark'"><Sunny /></el-icon>
            <el-icon v-else><Moon /></el-icon>
          </div>

          <el-dropdown trigger="click">
            <div class="user-profile">
              <span class="nickname">{{ nickname || username }}</span>
              <el-icon class="dropdown-icon"><ArrowDown /></el-icon>
            </div>
            <template #dropdown>
              <el-dropdown-menu>
                <el-dropdown-item>{{ t.common.profile }}</el-dropdown-item>
                <el-dropdown-item divided @click="handleLogout" style="color: var(--el-color-danger)">
                  {{ t.common.logout }}
                </el-dropdown-item>
              </el-dropdown-menu>
            </template>
          </el-dropdown>
        </div>
      </header>

      <div class="page-body">
        <router-view />
      </div>
    </div>
  </div>
</template>
