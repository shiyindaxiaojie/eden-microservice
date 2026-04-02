<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ArrowDown, Guide, Moon, QuestionFilled, Sunny, User, SwitchButton } from '@element-plus/icons-vue'
import { useI18n } from './utils/i18n'
import UiGuide, { type GuideStep } from './components/ui-guide.vue'
import { getGuideStatus, updateGuideStatus } from './api/registry'
import logo from './assets/logo.png'

const GUIDE_STORAGE_KEY = 'eden-ui-guide-completed-v1'

const { locale, t, toggleLocale } = useI18n()
const route = useRoute()
const router = useRouter()

const userRole = ref(localStorage.getItem('user_role') || '')
const username = ref(localStorage.getItem('username') || '')
const nickname = ref(localStorage.getItem('nickname') || '')
const theme = ref(localStorage.getItem('theme') || 'light')
const showGuide = ref(false)

const guideSteps: GuideStep[] = [
  {
    id: 'sidebar-nav',
    route: '/',
    selector: '[data-guide="sidebar-nav"]',
    placement: 'right',
    title: {
      zh: '从侧边栏进入核心模块',
      en: 'Use the sidebar to switch modules',
    },
    description: {
      zh: '这里集中放了概览、服务、命名空间和集群等入口，日常操作基本都从这里开始。',
      en: 'The sidebar is your main entry point for overview, services, namespaces, and cluster operations.',
    },
  },
  {
    id: 'header-actions',
    route: '/',
    selector: '[data-guide="header-actions"]',
    placement: 'bottom',
    title: {
      zh: '顶部放的是全局操作',
      en: 'The header contains global actions',
    },
    description: {
      zh: '你可以在这里打开文档、切换语言和主题、查看账户菜单，也能随时重新打开这套界面导览。',
      en: 'From here you can open docs, switch language and theme, access your account menu, or reopen this product tour anytime.',
    },
  },
  {
    id: 'dashboard-metrics',
    route: '/',
    selector: '[data-guide="dashboard-metrics"]',
    placement: 'bottom',
    title: {
      zh: '先看整体健康度',
      en: 'Start with system health at a glance',
    },
    description: {
      zh: '概览页会快速展示节点、服务、命名空间和资源使用情况，适合先判断注册中心是否稳定。',
      en: 'The overview page gives you a quick read on nodes, services, namespaces, and resource usage so you can assess registry health fast.',
    },
  },
  {
    id: 'dashboard-activity',
    route: '/',
    selector: '[data-guide="dashboard-activity"]',
    placement: 'top',
    title: {
      zh: '事件和日志在这里追踪',
      en: 'Track events and logs here',
    },
    description: {
      zh: '出现异常时，可以先看事件流和日志面板，快速定位是注册、心跳还是节点同步出了问题。',
      en: 'When something goes wrong, this is the fastest place to inspect events and logs for registration, heartbeat, or node sync issues.',
    },
  },
  {
    id: 'services-toolbar',
    route: '/services',
    query: { view: 'catalog' },
    selector: '[data-guide="services-toolbar"]',
    placement: 'bottom',
    title: {
      zh: '服务页支持快速筛选',
      en: 'Filter services quickly from the toolbar',
    },
    description: {
      zh: '你可以按命名空间、服务名、IP 和健康状态过滤结果，适合在服务很多时快速缩小范围。',
      en: 'Filter by namespace, service name, IP, and health state to narrow down large service lists quickly.',
    },
  },
  {
    id: 'services-topology-switch',
    route: '/services',
    query: { view: 'catalog' },
    selector: '[data-guide="services-topology-switch"]',
    placement: 'bottom',
    title: {
      zh: '这里可以切换到拓扑视图',
      en: 'Switch to topology view here',
    },
    description: {
      zh: '服务页支持列表、卡片和拓扑三种视图，排查依赖调用关系时建议直接切到拓扑。',
      en: 'The services page supports list, card, and topology views. Use topology when you need to inspect service dependencies.',
    },
  },
  {
    id: 'services-topology-canvas',
    route: '/services',
    query: { view: 'topology' },
    selector: '[data-guide="services-topology-canvas"]',
    placement: 'right',
    title: {
      zh: '拓扑里可以查看依赖链路',
      en: 'Explore dependency links in topology view',
    },
    description: {
      zh: '在这里可以看到服务上下游关系，选中节点后还能查看实例状态和依赖详情。',
      en: 'The topology graph shows upstream and downstream relationships, and selecting a node reveals instance state and dependency details.',
    },
  },
  {
    id: 'cluster-toolbar',
    route: '/cluster',
    selector: '[data-guide="cluster-toolbar"]',
    placement: 'bottom',
    title: {
      zh: '节点页用于管理集群成员',
      en: 'Use the cluster page to manage nodes',
    },
    description: {
      zh: '你可以在这里搜索节点、切换展示方式，并在允许的模式下添加或移除集群成员。',
      en: 'Search nodes, switch layouts, and add or remove cluster members from this page when your mode allows it.',
    },
  },
]

watch(
  () => route.path,
  () => {
    userRole.value = localStorage.getItem('user_role') || ''
    username.value = localStorage.getItem('username') || ''
    nickname.value = localStorage.getItem('nickname') || ''
  },
)

const navItems = computed(() => {

  const baseItems = [
    { path: '/', label: t.value.nav.dashboard, icon: 'Odometer' },
    { path: '/services', label: t.value.nav.services, icon: 'Grid' },
    { path: '/namespaces', label: t.value.nav.namespaces, icon: 'Collection' },
    { path: '/cluster', label: t.value.nav.cluster, icon: 'Connection' },
  ]
  const adminItems = [
    { path: '/rbac', label: t.value.nav.accessControl, icon: 'Lock' },
    { path: '/settings', label: t.value.nav.settings, icon: 'Setting' },
  ]

  if (userRole.value === 'admin') return [...baseItems, ...adminItems]
  if (userRole.value === 'developer') return baseItems
  return baseItems
})

const currentTitle = computed(() => {
  if (route.path === '/') return t.value.nav.dashboard
  if (route.path.startsWith('/services')) return t.value.nav.services
  if (route.path.startsWith('/namespaces')) return t.value.nav.namespaces
  if (route.path.startsWith('/cluster')) return t.value.nav.cluster
  if (route.path.startsWith('/rbac')) return t.value.nav.accessControl
  if (route.path.startsWith('/settings')) return t.value.nav.settings
  if (route.path.startsWith('/docs')) return t.value.nav.docs
  if (route.path.startsWith('/profile')) return locale.value === 'zh' ? '个人中心' : 'Profile'
  return 'Eden Registry'
})

watch(
  [() => route.fullPath, locale],
  () => {
    document.title = `${currentTitle.value} - Eden Registry`
  },
  { immediate: true },
)

watch(
  () => route.fullPath,
  async () => {
    // If not public, not already showing, and not specifically marked as completed in localStorage
    if (route.meta.public || showGuide.value || localStorage.getItem(GUIDE_STORAGE_KEY) === '1') {
      return
    }
    await nextTick()
    showGuide.value = true
  },
)

function handleLogout() {
  const guideVal = localStorage.getItem(GUIDE_STORAGE_KEY)
  const themeVal = localStorage.getItem('theme')
  localStorage.clear()
  if (guideVal) localStorage.setItem(GUIDE_STORAGE_KEY, guideVal)
  if (themeVal) localStorage.setItem('theme', themeVal)
  router.push('/login')
}

function isActive(path: string) {
  if (path === '/') return route.path === '/'
  return route.path.startsWith(path)
}

function applyTheme(nextTheme: string) {
  document.documentElement.setAttribute('data-theme', nextTheme)
  if (nextTheme === 'dark') {
    document.documentElement.classList.add('dark')
  } else {
    document.documentElement.classList.remove('dark')
  }
}

function toggleTheme() {
  theme.value = theme.value === 'dark' ? 'light' : 'dark'
  applyTheme(theme.value)
  setCookie('theme', theme.value, 365)
}

function restartGuide() {
  showGuide.value = true
}

function finishGuide() {
  localStorage.setItem(GUIDE_STORAGE_KEY, '1')
  showGuide.value = false
  // Sync to server
  updateGuideStatus(true).catch(err => console.error('Failed to sync guide status:', err))
}

function setCookie(name: string, value: string, daysUp: number) {
  const expires = new Date()
  expires.setTime(expires.getTime() + daysUp * 24 * 60 * 60 * 1000)
  document.cookie = `${name}=${value};expires=${expires.toUTCString()};path=/`
}

function getCookie(name: string) {
  const match = document.cookie.match(new RegExp('(^| )' + name + '=([^;]+)'))
  return match ? match[2] : null
}

function handleStorageChange() {
  username.value = localStorage.getItem('username') || ''
  nickname.value = localStorage.getItem('nickname') || ''
}

onMounted(async () => {
  const savedTheme = getCookie('theme') || 'light'
  theme.value = savedTheme
  applyTheme(savedTheme)

  window.addEventListener('storage', handleStorageChange)

  if (!route.meta.public) {
    const localCompleted = localStorage.getItem(GUIDE_STORAGE_KEY) === '1'
    try {
      const res = await getGuideStatus()
      if (res.data.guide_completed) {
        localStorage.setItem(GUIDE_STORAGE_KEY, '1')
      } else if (!localCompleted) {
        await nextTick()
        showGuide.value = true
      }
    } catch (err) {
      if (!localCompleted) {
        await nextTick()
        showGuide.value = true
      }
    }
  }
})

onBeforeUnmount(() => {
  window.removeEventListener('storage', handleStorageChange)
})
</script>

<template>
  <div v-if="route.meta.public" class="public-layout" :class="theme">
    <router-view />
  </div>

  <div v-else class="app-layout" :class="theme">
    <aside class="sidebar">
      <div class="sidebar-header">
        <div class="sidebar-logo">
          <div class="logo-icon">
            <img :src="logo" alt="Logo" style="width: 36px; height: 36px;" />
          </div>
          <span class="logo-text">{{ t.common.title }}</span>
        </div>
      </div>
      <nav class="sidebar-nav" data-guide="sidebar-nav">
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

    <div class="main-content">
      <header class="main-header">
        <h1>{{ currentTitle }}</h1>
        <div class="header-actions" data-guide="header-actions">
          <a href="https://github.com/shiyindaxiaojie/eden-go-registry" target="_blank" class="header-btn" title="GitHub">
            <svg viewBox="0 0 16 16" fill="currentColor"><path d="M8 0C3.58 0 0 3.58 0 8c0 3.54 2.29 6.53 5.47 7.59.4.07.55-.17.55-.38 0-.19-.01-.82-.01-1.49-2.01.37-2.53-.49-2.69-.94-.09-.23-.48-.94-.82-1.13-.28-.15-.68-.52-.01-.53.63-.01 1.08.58 1.23.82.72 1.21 1.87.87 2.33.66.07-.52.28-.87.51-1.07-1.78-.2-3.64-.89-3.64-3.95 0-.87.31-1.59.82-2.15-.08-.2-.36-1.02.08-2.12 0 0 .67-.21 2.2.82.64-.18 1.32-.27 2-.27.68 0 1.36.09 2 .27 1.53-1.04 2.2-.82 2.2-.82.44 1.1.16 1.92.08 2.12.51.56.82 1.27.82 2.15 0 3.07-1.87 3.75-3.65 3.95.29.25.54.73.54 1.48 0 1.07-.01 1.93-.01 2.2 0 .21.15.46.55.38A8.013 8.013 0 0016 8c0-4.42-3.58-8-8-8z"></path></svg>
          </a>

          <div
            class="header-btn"
            role="button"
            tabindex="0"
            @click="restartGuide"
            @keydown.enter="restartGuide"
            @keydown.space.prevent="restartGuide"
            :title="locale === 'zh' ? '界面导览' : 'Product tour'"
          >
            <el-icon><Guide /></el-icon>
          </div>

          <router-link to="/docs" class="header-btn" :title="t.nav.docs">
            <el-icon><QuestionFilled /></el-icon>
          </router-link>

          <div class="header-btn" @click="toggleLocale" :title="locale === 'en' ? t.common.switchToChinese : t.common.switchToEnglish">
            <span class="lang-text">{{ locale === 'en' ? '中' : 'EN' }}</span>
          </div>

          <div class="header-btn" @click="toggleTheme" :title="theme === 'dark' ? t.settings.light : t.settings.dark">
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
                <el-dropdown-item @click="router.push('/profile')">
                  <el-icon><User /></el-icon>
                  {{ t.common.profile || '个人中心' }}
                </el-dropdown-item>
                <el-dropdown-item divided @click="handleLogout" style="color: var(--el-color-danger)">
                  <el-icon><SwitchButton /></el-icon>
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

    <UiGuide v-model="showGuide" :steps="guideSteps" @complete="finishGuide" @dismiss="finishGuide" />
  </div>
</template>
