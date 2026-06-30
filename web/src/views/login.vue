<script setup lang="ts">
import { nextTick, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import api from '../api/index'
import logo from '../assets/logo.png'
import furinaBg from '../assets/login/furina.png'
import { sha256 } from '../utils/crypto'
import { useI18n } from '../utils/i18n'

const { t, text, toggleLocale, nextLocaleTitle } = useI18n()
const router = useRouter()
const loading = ref(false)
const showLoginModal = ref(false)
const passwordType = ref<'password' | 'text'>('password')
const rememberMe = ref(false)
const theme = ref(getCookie('theme') || localStorage.getItem('theme') || 'light')
const usernameInputRef = ref<HTMLInputElement | null>(null)
const form = ref({
  username: '',
  password: '',
})

function openLoginModal() {
  showLoginModal.value = true
  nextTick(() => usernameInputRef.value?.focus())
}

function closeLoginModal() {
  showLoginModal.value = false
}

function clearUsername() {
  form.value.username = ''
  nextTick(() => usernameInputRef.value?.focus())
}

function togglePassword() {
  passwordType.value = passwordType.value === 'password' ? 'text' : 'password'
}

async function handleLogin() {
  if (!form.value.username || !form.value.password) {
    ElMessage.warning(text('\u8bf7\u8f93\u5165\u8d26\u53f7\u548c\u5bc6\u7801', 'Please enter username and password'))
    return
  }

  loading.value = true
  try {
    const hashedPassword = await sha256(form.value.password)
    const res = await api.post('/v1/auth/login', {
      username: form.value.username,
      password: hashedPassword,
    })

    localStorage.setItem('token', res.data.token)
    localStorage.setItem('user_role', res.data.role)
    localStorage.setItem('username', form.value.username)
    localStorage.setItem('nickname', res.data.nickname || form.value.username)

    router.push('/')
  } catch (e: any) {
    const errorMsg = e.response?.data?.error
    if (errorMsg === 'invalid credentials') {
      ElMessage.error(text('\u8d26\u53f7\u6216\u5bc6\u7801\u9519\u8bef', 'Invalid username or password'))
    } else {
      ElMessage.error(errorMsg || text('\u767b\u5f55\u5931\u8d25', 'Login failed'))
    }
  } finally {
    loading.value = false
  }
}

function showDocsComingSoon() {
  ElMessage.info(text('\u6587\u6863\u5165\u53e3\u767b\u5f55\u540e\u53ef\u7528', 'Documentation is available after login'))
}

function getParticleStyle(index: number) {
  const left = [9, 18, 28, 36, 44, 56, 64, 72, 81, 88, 94, 6][index - 1] || 50
  const drift = [-16, 20, -26, 18, -10, 24, -18, 16, -22, 12, -14, 20][index - 1] || 12
  const duration = 9 + (index % 5) * 1.4
  const delay = -1 * (index % 8)
  const size = 14 + (index % 4) * 5

  return {
    left: `${left}%`,
    animationDuration: `${duration}s`,
    animationDelay: `${delay}s`,
    fontSize: `${size}px`,
    '--snow-opacity': `${0.3 + (index % 5) * 0.08}`,
    '--snow-mid-drift': `${Math.round(drift / 2)}px`,
    '--snow-end-drift': `${drift}px`,
    '--snow-mid-rotation': `${60 + index * 13}deg`,
    '--snow-end-rotation': `${140 + index * 17}deg`,
  }
}

function getParticleGlyph(index: number) {
  const glyphs = ['*', '\u2736', '\u2737', '\u2726']
  return glyphs[index % glyphs.length]
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
  localStorage.setItem('theme', theme.value)
}

function setCookie(name: string, value: string, daysUp: number) {
  const expires = new Date()
  expires.setTime(expires.getTime() + daysUp * 24 * 60 * 60 * 1000)
  document.cookie = `${name}=${value};expires=${expires.toUTCString()};path=/`
}

function getCookie(name: string) {
  if (typeof document === 'undefined') return null
  const match = document.cookie.match(new RegExp('(^| )' + name + '=([^;]+)'))
  return match ? match[2] : null
}

onMounted(() => {
  applyTheme(theme.value)
})
</script>

<template>
  <div class="mihoyo-login-page" :class="{ 'modal-open': showLoginModal }">
    <div class="background-layer" aria-hidden="true">
      <img :src="furinaBg" alt="" class="bg-image" />
      <div class="bg-overlay"></div>
      <div class="particles">
        <div v-for="i in 12" :key="i" class="particle" :style="getParticleStyle(i)">
          {{ getParticleGlyph(i) }}
        </div>
      </div>
    </div>

    <header class="top-nav">
      <div class="nav-container">
        <div class="nav-left">
          <a href="#" class="brand-link" :aria-label="t.common.title" @click.prevent="closeLoginModal">
            <img :src="logo" :alt="t.common.title" class="logo-icon" />
            <span class="logo-text">{{ t.common.title }}</span>
          </a>
          <span class="brand-divider" aria-hidden="true"></span>
          <nav class="nav-menu" :aria-label="text('\u9876\u90e8\u5bfc\u822a', 'Top navigation')">
            <button type="button" class="nav-item active" @click="closeLoginModal">{{ text('\u9996\u9875', 'Home') }}</button>
            <button type="button" class="nav-item nav-link-button" @click="showDocsComingSoon">{{ text('\u6587\u6863', 'Docs') }}</button>
            <a
              href="https://github.com/shiyindaxiaojie/eden-registry/issues/new"
              class="nav-item"
              target="_blank"
              rel="noopener noreferrer"
            >
              {{ text('\u53cd\u9988', 'Feedback') }}
            </a>
          </nav>
        </div>

        <div class="nav-right">
          <a
            class="header-icon-btn"
            href="https://github.com/shiyindaxiaojie/eden-registry"
            target="_blank"
            rel="noopener noreferrer"
            aria-label="GitHub"
            title="GitHub"
          >
            <svg class="icon-svg" viewBox="0 0 24 24" aria-hidden="true">
              <path
                fill="currentColor"
                d="M12 2C6.48 2 2 6.58 2 12.24c0 4.52 2.86 8.35 6.84 9.7.5.1.68-.22.68-.49 0-.24-.01-1.04-.01-1.88-2.78.62-3.37-1.22-3.37-1.22-.45-1.19-1.11-1.5-1.11-1.5-.91-.64.07-.63.07-.63 1 .07 1.53 1.06 1.53 1.06.9 1.57 2.35 1.12 2.93.86.09-.67.35-1.12.63-1.38-2.22-.26-4.55-1.14-4.55-5.07 0-1.12.39-2.03 1.03-2.75-.1-.26-.45-1.3.1-2.71 0 0 .84-.28 2.75 1.05A9.36 9.36 0 0 1 12 6.94c.85 0 1.7.12 2.5.34 1.9-1.33 2.74-1.05 2.74-1.05.55 1.41.2 2.45.1 2.71.64.72 1.03 1.63 1.03 2.75 0 3.94-2.34 4.8-4.57 5.06.36.32.68.95.68 1.92 0 1.39-.01 2.51-.01 2.85 0 .27.18.6.69.49A10.08 10.08 0 0 0 22 12.24C22 6.58 17.52 2 12 2Z"
              />
            </svg>
          </a>

          <button
            class="header-icon-btn"
            type="button"
            :aria-label="nextLocaleTitle"
            :title="nextLocaleTitle"
            @click="toggleLocale"
          >
            <svg class="icon-svg" viewBox="0 0 24 24" aria-hidden="true">
              <path d="M4 5h9M9 3v2m2.5 0c-.68 2.98-2.44 5.25-5.5 7m1.5-4.5c.86 1.65 2.13 2.95 3.9 3.9M14 19l4-9 4 9m-6.75-3h5.5M4 19h7" />
            </svg>
          </button>

          <button
            class="header-icon-btn theme-toggle-btn"
            type="button"
            :aria-label="theme === 'dark' ? t.settings.light : t.settings.dark"
            :title="theme === 'dark' ? t.settings.light : t.settings.dark"
            @click="toggleTheme"
          >
            <svg v-if="theme === 'dark'" class="icon-svg" viewBox="0 0 24 24" aria-hidden="true">
              <path d="M12 4V2m0 20v-2M4.93 4.93 3.51 3.51m16.98 16.98-1.42-1.42M4 12H2m20 0h-2M4.93 19.07l-1.42 1.42M20.49 3.51l-1.42 1.42M12 8a4 4 0 1 0 0 8 4 4 0 0 0 0-8Z" />
            </svg>
            <svg v-else class="icon-svg" viewBox="0 0 24 24" aria-hidden="true">
              <path d="M21 12.8A8.2 8.2 0 1 1 11.2 3a6.5 6.5 0 0 0 9.8 9.8Z" />
            </svg>
          </button>
        </div>
      </div>
    </header>

    <main class="main-content">
      <section class="game-info-section">
        <div class="game-logo">
          <div class="logo-symbol">
            <img :src="logo" :alt="t.common.title" class="game-logo-img" />
          </div>
        </div>
        <h2 class="game-title">{{ t.common.title }}</h2>
        <p class="game-subtitle">{{ text('\u8f7b\u91cf\u7ea7\u5fae\u670d\u52a1\u63a7\u5236\u9762', 'Lightweight microservice control plane') }}</p>
        <button class="start-game-btn" type="button" @click="openLoginModal">
          <span>{{ text('\u5f00\u59cb\u4f53\u9a8c', 'Enter Console') }}</span>
        </button>
      </section>

      <div v-show="showLoginModal" class="login-modal" @click.self="closeLoginModal">
        <div class="modal-content" role="dialog" aria-modal="true" :aria-label="text('\u767b\u5f55', 'Login')">
          <button class="close-btn" type="button" :aria-label="text('\u5173\u95ed', 'Close')" @click="closeLoginModal">x</button>

          <div class="modal-header">
            <div class="mihoyo-brand">
              <div class="brand-logo">
                <img :src="logo" :alt="t.common.title" class="brand-logo-img" />
              </div>
              <div class="brand-info">
                <div class="brand-name">{{ t.common.title }}</div>
                <div class="brand-slogan">{{ text('\u5fae\u670d\u52a1\u63a7\u5236\u9762', 'Microservice control plane') }}</div>
              </div>
            </div>
          </div>

          <div class="login-tabs">
            <button class="tab-btn active" type="button">{{ text('\u5bc6\u7801\u767b\u5f55', 'Password Login') }}</button>
          </div>

          <div class="login-body">
            <div class="modal-login-form">
              <div class="form-group">
                <div class="input-wrapper">
                  <input
                    ref="usernameInputRef"
                    v-model="form.username"
                    class="form-input"
                    type="text"
                    :placeholder="text('\u8d26\u53f7', 'Username')"
                    autocomplete="username"
                    @keyup.enter="handleLogin"
                  />
                  <button v-if="form.username" class="clear-btn" type="button" tabindex="-1" @click="clearUsername">x</button>
                </div>
              </div>

              <div class="form-group">
                <div class="input-wrapper">
                  <input
                    v-model="form.password"
                    class="form-input"
                    :type="passwordType"
                    :placeholder="text('\u5bc6\u7801', 'Password')"
                    autocomplete="current-password"
                    @keyup.enter="handleLogin"
                  />
                  <button class="toggle-password-btn" type="button" tabindex="-1" @click="togglePassword">
                    {{ passwordType === 'password' ? '\u25c9' : '\u25cc' }}
                  </button>
                </div>
              </div>

              <div class="form-options">
                <label class="remember-label">
                  <input v-model="rememberMe" type="checkbox" class="remember-checkbox" />
                  <span>{{ text('\u8bb0\u4f4f\u767b\u5f55\u8d26\u53f7', 'Remember account') }}</span>
                </label>
              </div>

              <button class="login-btn" type="button" :disabled="loading" @click="handleLogin">
                {{ loading ? text('\u767b\u5f55\u4e2d...', 'Logging in...') : text('\u767b\u5f55', 'Login') }}
              </button>
            </div>
          </div>
        </div>
      </div>
    </main>
  </div>
</template>

<style scoped>
.mihoyo-login-page {
  --login-theme-text-color: #ffbf24;
  --login-theme-accent-color: #ffbf24;
  --login-theme-text-glow: #ffbf24;
  min-height: 100vh;
  position: relative;
  overflow: hidden;
  font-family: 'Microsoft YaHei', 'PingFang SC', 'Helvetica Neue', Arial, sans-serif;
  background: #061326;
}

.background-layer {
  position: fixed;
  inset: 0;
  z-index: 1;
}

.bg-image {
  width: 100%;
  height: 100%;
  object-fit: cover;
  object-position: 58% center;
  filter: brightness(0.52) saturate(0.92);
}

.bg-overlay {
  position: absolute;
  inset: 0;
  background:
    linear-gradient(90deg, rgba(8, 15, 28, 0.4) 0%, rgba(10, 24, 42, 0.2) 42%, rgba(4, 10, 24, 0.72) 100%),
    linear-gradient(180deg, rgba(4, 10, 24, 0.18) 0%, rgba(4, 10, 24, 0.48) 100%),
    rgba(0, 0, 0, 0.5);
}

.particles {
  position: absolute;
  inset: 0;
  pointer-events: none;
  overflow: hidden;
}

.particle {
  position: absolute;
  top: -48px;
  color: rgba(255, 255, 255, 0.88);
  font-family: Georgia, 'Times New Roman', serif;
  line-height: 1;
  user-select: none;
  text-shadow: 0 0 6px rgba(255, 255, 255, 0.28), 0 0 12px rgba(167, 214, 255, 0.18);
  transform-origin: center;
  animation: snowFall infinite ease-in-out;
}

@keyframes snowFall {
  0% {
    transform: translate3d(0, -12vh, 0) rotate(0deg) scale(0.86);
    opacity: 0;
  }
  14% {
    opacity: var(--snow-opacity);
  }
  50% {
    transform: translate3d(var(--snow-mid-drift), 48vh, 0) rotate(var(--snow-mid-rotation)) scale(1);
  }
  86% {
    opacity: var(--snow-opacity);
  }
  100% {
    transform: translate3d(var(--snow-end-drift), 112vh, 0) rotate(var(--snow-end-rotation)) scale(0.92);
    opacity: 0;
  }
}

.top-nav {
  position: relative;
  z-index: 10;
  background: rgba(12, 27, 43, 0.5);
  border-bottom: 1px solid rgba(255, 255, 255, 0.12);
  backdrop-filter: blur(10px);
}

.nav-container {
  min-height: 56px;
  width: 100%;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 24px;
  padding: 0 clamp(16px, 2.2vw, 30px);
}

.nav-left,
.nav-right,
.brand-link,
.nav-menu {
  display: flex;
  align-items: center;
}

.nav-left {
  min-width: 0;
  gap: clamp(14px, 1.8vw, 22px);
}

.brand-link {
  flex: 0 1 auto;
  min-width: 0;
  gap: 12px;
  text-decoration: none;
}

.logo-icon {
  width: 34px;
  height: 34px;
  object-fit: contain;
  filter: drop-shadow(0 2px 5px rgba(0, 0, 0, 0.32));
}

.logo-text {
  position: relative;
  display: inline-flex;
  align-items: center;
  min-height: 24px;
  color: transparent;
  background: linear-gradient(110deg, #ffffff 0%, #d9fbff 32%, #8eeaff 66%, #ffffff 100%);
  background-clip: text;
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
  font-size: 20px;
  font-weight: 820;
  line-height: 1;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  filter: drop-shadow(0 2px 5px rgba(0, 0, 0, 0.38)) drop-shadow(0 0 7px rgba(108, 202, 255, 0.2));
}

.brand-divider {
  width: 1px;
  height: 24px;
  flex: 0 0 auto;
  background: rgba(255, 255, 255, 0.22);
}

.nav-menu {
  gap: clamp(20px, 2.1vw, 34px);
  min-width: 0;
}

.nav-item {
  position: relative;
  display: inline-flex;
  align-items: center;
  padding: 20px 0;
  border: 0;
  background: transparent;
  color: rgba(255, 255, 255, 0.82);
  font: inherit;
  font-size: 15px;
  font-weight: 700;
  line-height: 1;
  text-decoration: none;
  white-space: nowrap;
  cursor: pointer;
  transition: color 0.25s ease;
}

.nav-item:hover,
.nav-item.active {
  color: #ffffff;
}

.nav-item::after {
  position: absolute;
  left: 50%;
  bottom: 6px;
  width: 26px;
  height: 2px;
  border-radius: 999px;
  content: '';
  background: linear-gradient(90deg, transparent, #ffbf24, transparent);
  box-shadow: 0 0 10px rgba(255, 191, 36, 0.44);
  opacity: 0;
  transform: translateX(-50%) scaleX(0.55);
  transition: opacity 0.25s ease, transform 0.25s ease;
}

.nav-item:hover::after,
.nav-item.active::after {
  opacity: 1;
  transform: translateX(-50%) scaleX(1);
}

.nav-right {
  gap: 18px;
}

.header-icon-btn {
  width: 32px;
  height: 32px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  padding: 0;
  border: 0;
  background: transparent;
  color: rgba(255, 255, 255, 0.86);
  cursor: pointer;
  text-decoration: none;
  transition: color 0.25s ease, transform 0.25s ease;
}

.header-icon-btn:hover {
  color: var(--login-theme-text-color);
  transform: translateY(-1px);
}

.icon-svg {
  width: 20px;
  height: 20px;
  display: block;
}

.header-icon-btn .icon-svg path:not([fill]) {
  fill: none;
  stroke: currentColor;
  stroke-width: 1.8;
  stroke-linecap: round;
  stroke-linejoin: round;
}

.main-content {
  position: relative;
  z-index: 2;
  min-height: calc(100vh - 56px);
  width: 100%;
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 clamp(32px, 4vw, 76px);
}

.game-info-section {
  position: absolute;
  right: clamp(64px, 5vw, 108px);
  top: 56%;
  width: clamp(500px, 35vw, 660px);
  display: flex;
  flex-direction: column;
  align-items: center;
  transform: translateY(-50%);
  text-align: center;
  color: white;
  isolation: isolate;
}

.game-info-section::before,
.game-info-section::after {
  position: absolute;
  z-index: -1;
  pointer-events: none;
  content: '';
}

.game-info-section::before {
  inset: -42px -34px -34px;
  background:
    radial-gradient(circle at 20% 24%, rgba(245, 251, 255, 0.9) 0 1.5px, transparent 2px),
    radial-gradient(circle at 78% 18%, rgba(214, 229, 255, 0.82) 0 1.5px, transparent 2.5px),
    radial-gradient(circle at 88% 68%, rgba(255, 210, 87, 0.78) 0 1px, transparent 2px);
  filter: drop-shadow(0 0 12px rgba(148, 213, 255, 0.42));
  animation: magicAuraReveal 1.25s cubic-bezier(0.19, 1, 0.22, 1) 0.1s both;
}

.game-info-section::after {
  top: 50%;
  left: 50%;
  width: min(430px, 76%);
  aspect-ratio: 1;
  border-radius: 50%;
  background:
    conic-gradient(from 18deg, transparent 0 12deg, rgba(255, 221, 111, 0.78) 18deg 26deg, transparent 32deg 88deg, rgba(255, 255, 255, 0.68) 96deg 102deg, transparent 108deg 176deg, rgba(255, 191, 36, 0.34) 184deg 194deg, transparent 204deg 360deg);
  mask: radial-gradient(circle, transparent 0 58%, #000 59% 61%, transparent 62% 100%);
  filter: blur(0.2px) drop-shadow(0 0 18px rgba(255, 191, 36, 0.26));
  animation: magicCircleManifest 1.1s cubic-bezier(0.16, 1, 0.3, 1) both;
}

.game-logo {
  margin-bottom: 24px;
  opacity: 0;
  animation: magicItemReveal 0.76s cubic-bezier(0.19, 1, 0.22, 1) 0.18s both;
}

.logo-symbol {
  width: 118px;
  height: 118px;
  display: grid;
  place-items: center;
}

.game-logo-img {
  width: 100%;
  height: 100%;
  object-fit: contain;
  filter: drop-shadow(0 12px 30px rgba(0, 0, 0, 0.36));
}

.game-title {
  max-width: 100%;
  margin: 0 0 18px;
  color: white;
  font-size: clamp(38px, 3vw, 56px);
  line-height: 1.12;
  font-weight: 800;
  white-space: nowrap;
  text-shadow: 0 3px 5px rgba(0, 0, 0, 0.38);
  opacity: 0;
  animation: magicItemReveal 0.72s cubic-bezier(0.19, 1, 0.22, 1) 0.34s both;
}

.game-subtitle {
  max-width: 100%;
  margin: 0 0 42px;
  color: rgba(255, 255, 255, 0.9);
  font-size: 18px;
  line-height: 1.45;
  white-space: nowrap;
  text-shadow: 0 2px 4px rgba(0, 0, 0, 0.38);
  opacity: 0;
  animation: magicItemReveal 0.72s cubic-bezier(0.19, 1, 0.22, 1) 0.46s both;
}

.start-game-btn {
  min-width: 198px;
  height: 52px;
  padding: 0 36px;
  position: relative;
  overflow: hidden;
  border: 0;
  border-radius: 999px;
  background: #ffb52e;
  color: #263247;
  font-size: 18px;
  font-weight: 800;
  cursor: pointer;
  box-shadow: 0 10px 28px rgba(255, 181, 46, 0.18);
  opacity: 0;
  animation: magicItemReveal 0.72s cubic-bezier(0.19, 1, 0.22, 1) 0.58s both;
}

.start-game-btn:hover {
  background: #ffc247;
  box-shadow: 0 0 28px rgba(255, 181, 46, 0.36), 0 10px 30px rgba(255, 181, 46, 0.22);
}

@keyframes magicItemReveal {
  0% {
    opacity: 0;
    transform: scale(0.985);
    filter: blur(14px) brightness(1.28);
  }
  58% {
    opacity: 1;
    filter: blur(0);
  }
  100% {
    opacity: 1;
    transform: scale(1);
    filter: none;
  }
}

@keyframes magicAuraReveal {
  0% {
    opacity: 0;
    transform: translateX(-18px) scale(0.96);
  }
  45% {
    opacity: 0.9;
  }
  82% {
    opacity: 0.42;
    transform: translateX(0) scale(1);
  }
  100% {
    opacity: 0;
    transform: translateX(0) scale(1);
  }
}

@keyframes magicCircleManifest {
  0% {
    opacity: 0;
    transform: translate(-50%, -50%) scale(0.72) rotate(-24deg);
  }
  34% {
    opacity: 0.62;
  }
  78% {
    opacity: 0.18;
    transform: translate(-50%, -50%) scale(1.04) rotate(8deg);
  }
  100% {
    opacity: 0;
    transform: translate(-50%, -50%) scale(1.04) rotate(8deg);
  }
}

.login-modal {
  position: fixed;
  inset: 0;
  z-index: 1000;
  display: flex;
  align-items: center;
  justify-content: center;
  background: rgba(0, 0, 0, 0.5);
  backdrop-filter: blur(5px);
  animation: fadeIn 0.3s ease;
}

@keyframes fadeIn {
  from { opacity: 0; }
  to { opacity: 1; }
}

.modal-content {
  width: 480px;
  max-width: calc(100vw - 40px);
  max-height: calc(100vh - 40px);
  position: relative;
  overflow: hidden;
  padding: 0;
  border-radius: 12px;
  background: white;
  box-shadow: 0 8px 32px rgba(0, 0, 0, 0.1);
  animation: slideUp 0.3s ease;
}

@keyframes slideUp {
  from { transform: translateY(50px); opacity: 0; }
  to { transform: translateY(0); opacity: 1; }
}

.close-btn {
  position: absolute;
  top: 18px;
  right: 18px;
  z-index: 10;
  width: 32px;
  height: 32px;
  display: flex;
  align-items: center;
  justify-content: center;
  border: 0;
  background: transparent;
  color: #999;
  font-size: 24px;
  cursor: pointer;
}

.close-btn:hover {
  color: #666;
}

.modal-header {
  padding: 34px 40px 18px;
  text-align: center;
}

.mihoyo-brand {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 10px;
}

.brand-logo {
  width: 60px;
  height: 60px;
  display: flex;
  align-items: center;
  justify-content: center;
}

.brand-logo-img {
  width: 60px;
  height: 60px;
  object-fit: contain;
}

.brand-info {
  text-align: left;
}

.brand-name {
  margin-bottom: 2px;
  color: var(--login-theme-text-color);
  font-size: 32px;
  font-weight: 700;
  white-space: nowrap;
}

.brand-slogan {
  color: #999;
  font-size: 12px;
  letter-spacing: 0.3px;
}

.login-tabs {
  display: flex;
  padding: 0 40px;
  margin-bottom: 24px;
}

.tab-btn {
  flex: 1;
  padding: 12px 0;
  border: 0;
  border-bottom: 2px solid var(--login-theme-text-color);
  background: none;
  color: var(--login-theme-text-color);
  font-size: 16px;
  font-weight: 700;
}

.login-body {
  padding: 0 40px 32px;
}

.modal-login-form .form-group {
  margin-bottom: 18px;
}

.input-wrapper {
  position: relative;
  display: flex;
  align-items: center;
  min-height: 46px;
  border: 1px solid #e0e0e0;
  border-radius: 4px;
  background: white;
  transition: border-color 0.25s ease;
}

.input-wrapper:focus-within {
  border-color: var(--login-theme-text-color);
}

.form-input {
  flex: 1;
  min-width: 0;
  height: 46px;
  padding: 0 16px;
  border: 0;
  outline: none;
  background: transparent;
  color: #333;
  font-size: 14px;
}

.form-input::placeholder {
  color: #bbb;
}

.clear-btn,
.toggle-password-btn {
  position: absolute;
  right: 12px;
  top: 50%;
  transform: translateY(-50%);
  border: 0;
  background: none;
  color: #bbb;
  cursor: pointer;
}

.clear-btn {
  width: 20px;
  height: 20px;
  border-radius: 50%;
  background: #ccc;
  color: white;
  font-size: 12px;
}

.toggle-password-btn {
  font-size: 16px;
}

.form-options {
  margin: 2px 0 22px;
}

.remember-label {
  display: flex;
  align-items: flex-start;
  gap: 10px;
  color: #666;
  font-size: 12px;
  line-height: 20px;
  cursor: pointer;
}

.remember-checkbox {
  width: 14px;
  height: 14px;
  flex-shrink: 0;
  margin: 3px 0 0;
}

.login-btn {
  width: 100%;
  height: 50px;
  padding: 0 14px;
  border: 0;
  border-radius: 4px;
  background: #4a4a4a;
  color: var(--login-theme-text-color);
  font-size: 16px;
  font-weight: 600;
  cursor: pointer;
  transition: background 0.25s ease, color 0.25s ease;
}

.login-btn:hover:not(:disabled) {
  background: #333;
}

.login-btn:disabled {
  background: #e8e8e8;
  color: #bbb;
  cursor: not-allowed;
}

@media (prefers-reduced-motion: reduce) {
  .particle,
  .game-info-section::before,
  .game-info-section::after,
  .game-logo,
  .game-title,
  .game-subtitle,
  .start-game-btn,
  .login-modal,
  .modal-content {
    animation: none;
  }

  .game-logo,
  .game-title,
  .game-subtitle,
  .start-game-btn {
    opacity: 1;
  }
}

@media (max-width: 1024px) {
  .main-content {
    flex-direction: column;
    justify-content: center;
    padding: 40px 20px;
    text-align: center;
  }

  .game-info-section {
    position: static;
    width: min(620px, 90vw);
    margin-bottom: 40px;
    transform: none;
  }

  .game-title {
    font-size: clamp(30px, 5vw, 42px);
  }
}

@media (max-width: 768px) {
  .nav-container {
    min-height: 56px;
    padding: 0 16px;
  }

  .nav-left {
    gap: 12px;
  }

  .logo-icon {
    width: 28px;
    height: 28px;
  }

  .logo-text {
    max-width: 44vw;
    font-size: 16px;
  }

  .brand-divider,
  .nav-menu {
    display: none;
  }

  .nav-right {
    gap: 8px;
  }

  .header-icon-btn {
    width: 32px;
    height: 32px;
  }

  .main-content {
    min-height: calc(100vh - 56px);
    justify-content: flex-end;
    padding: 0 20px 42px;
  }

  .bg-image {
    object-position: 50% center;
  }

  .game-info-section {
    width: min(520px, 92vw);
    margin: 0;
  }

  .logo-symbol {
    width: 88px;
    height: 88px;
  }

  .game-logo {
    margin-bottom: 18px;
  }

  .game-title {
    font-size: clamp(30px, 9vw, 42px);
  }

  .game-subtitle {
    margin-bottom: 30px;
    font-size: clamp(13px, 3.7vw, 16px);
  }

  .start-game-btn {
    min-width: 188px;
    height: 50px;
    font-size: 18px;
  }

  .modal-content {
    width: 90%;
    max-width: 400px;
    margin: 20px;
  }

  .modal-header {
    padding: 28px 24px 16px;
  }

  .brand-name {
    font-size: 28px;
  }

  .login-tabs {
    padding: 0 24px;
    margin-bottom: 20px;
  }

  .login-body {
    padding: 0 24px 28px;
  }
}
</style>
