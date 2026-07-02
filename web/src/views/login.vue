<script setup lang="ts">
import { nextTick, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import api from '../api/index'
import logo from '../assets/logo.png'
import furinaBg from '../assets/login/furina.png'
import { sha256 } from '../utils/crypto'
import { useI18n } from '../utils/i18n'
import { applyTheme, persistTheme, readStoredTheme, type AppTheme } from '../utils/theme'

const { t, text, toggleLocale, nextLocaleTitle } = useI18n()
const router = useRouter()
const loading = ref(false)
const passwordType = ref<'password' | 'text'>('password')
const rememberMe = ref(false)
const theme = ref<AppTheme>(readStoredTheme())
const usernameInputRef = ref<HTMLInputElement | null>(null)
const form = ref({
  username: '',
  password: '',
})

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

function getBubbleStyle(index: number) {
  const leftMap = [
    1, 3, 5, 7, 9, 12, 15, 18,
    92, 94, 95, 96, 97, 98, 99, 99.5,
    2, 6, 11, 16, 20, 23, 93, 97,
    4, 10, 14, 22, 28, 34, 60, 66,
    0.5, 8, 19, 24,
  ]
  const left = leftMap[index - 1] || 50
  const isSide = left <= 24 || left >= 76
  const driftMap = [-18, 24, -12, 18, -26, 14, -20, 22, -16, 26, -24, 18]
  const drift = driftMap[(index - 1) % driftMap.length] ?? 12
  const duration = isSide ? 11 + (index % 7) * 1.3 : 14 + (index % 4) * 1.6
  const delay = -1 * ((index * 0.72) % 10)
  const size = isSide ? 20 + (index % 6) * 8 : 10 + (index % 4) * 5
  const opacity = isSide ? 0.36 + (index % 5) * 0.08 : 0.2 + (index % 4) * 0.05
  const scale = 0.82 + (index % 5) * 0.08

  return {
    left: `${left}%`,
    width: `${size}px`,
    height: `${size}px`,
    animationDuration: `${duration}s`,
    animationDelay: `${delay}s`,
    '--bubble-opacity': `${opacity}`,
    '--bubble-soft-drift': `${Math.round(drift * 0.25)}px`,
    '--bubble-mid-drift': `${Math.round(drift / 2)}px`,
    '--bubble-wide-drift': `${Math.round(drift * 0.72)}px`,
    '--bubble-end-drift': `${drift}px`,
    '--bubble-scale': `${scale}`,
    '--bubble-scale-small': `${scale * 0.93}`,
    '--bubble-scale-large': `${scale * 1.12}`,
  }
}

function toggleTheme() {
  theme.value = theme.value === 'dark' ? 'light' : 'dark'
  applyTheme(theme.value)
  persistTheme(theme.value)
}

onMounted(() => {
  theme.value = readStoredTheme()
  applyTheme(theme.value)
  nextTick(() => usernameInputRef.value?.focus())
})
</script>

<template>
  <div class="mihoyo-login-page" :class="`theme-${theme}`">
    <div class="background-layer" aria-hidden="true">
      <img :src="furinaBg" alt="" class="bg-image" />
      <div class="bg-overlay"></div>
      <div class="bubbles">
        <div v-for="i in 36" :key="i" class="bubble" :style="getBubbleStyle(i)"></div>
      </div>
    </div>

    <header class="top-nav">
      <div class="nav-container">
        <div class="nav-left">
          <a href="#" class="brand-link" :aria-label="t.common.title" @click.prevent>
            <img :src="logo" :alt="t.common.title" class="logo-icon" />
            <span class="logo-text">{{ t.common.title }}</span>
          </a>
          <span class="nav-divider" aria-hidden="true"></span>
          <nav class="nav-menu" :aria-label="text('\u9876\u90e8\u5bfc\u822a', 'Top navigation')">
            <button type="button" class="nav-item active">{{ text('\u9996\u9875', 'Home') }}</button>
            <a
              href="https://mengxiangge.netlify.app"
              class="nav-item"
              target="_blank"
              rel="noopener noreferrer"
            >
              {{ text('\u7f51\u7ad9', 'Website') }}
            </a>
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

    <main class="login-main-content">
      <section class="game-info-section">
        <div class="game-logo">
          <div class="logo-symbol">
            <img :src="logo" :alt="t.common.title" class="game-logo-img" />
          </div>
        </div>
        <h2 class="game-title">{{ t.common.title }}</h2>

        <div class="hero-login-panel" role="form" :aria-label="text('\u767b\u5f55', 'Login')">
          <div class="hero-login-form">
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
      </section>
    </main>
  </div>
</template>

<style scoped lang="scss">
.mihoyo-login-page {
  --login-theme-text-color: #ffbf24;
  --login-theme-accent-color: #ffbf24;
  --login-theme-text-glow: #ffbf24;
  --login-page-bg: #061326;
  --login-scene-bg:
    radial-gradient(circle at 18% 18%, rgba(125, 211, 252, 0.34), transparent 30%),
    radial-gradient(circle at 82% 78%, rgba(255, 191, 36, 0.18), transparent 26%),
    linear-gradient(135deg, #0b1d33 0%, #12395f 46%, #07101f 100%);
  --login-bg-filter: none;
  --login-bg-opacity: 1;
  --login-bg-width: 100%;
  --login-bg-transform: translateX(-6%);
  --login-bg-position: center center;
  --login-bg-overlay:
    linear-gradient(90deg, rgba(8, 15, 28, 0.42) 0%, rgba(10, 24, 42, 0.18) 42%, rgba(4, 10, 24, 0.74) 100%),
    linear-gradient(180deg, rgba(4, 10, 24, 0.16) 0%, rgba(4, 10, 24, 0.5) 100%),
    rgba(0, 0, 0, 0.42);
  --login-nav-bg: rgba(12, 27, 43, 0.68);
  --login-nav-border: rgba(255, 255, 255, 0.14);
  --login-nav-text: rgba(255, 255, 255, 0.82);
  --login-nav-text-active: #ffffff;
  --login-nav-icon: rgba(255, 255, 255, 0.86);
  --login-brand-text: linear-gradient(90deg, #dff8ff, #dff8ff);
  --login-brand-shadow: none;
  --login-hero-title-color: #ffffff;
  --login-hero-title-shadow: none;
  --login-form-text: #102a45;
  --login-form-muted: rgba(239, 248, 255, 0.88);
  --login-field-bg: rgba(246, 251, 255, 0.78);
  --login-field-border: rgba(255, 255, 255, 0.62);
  --login-field-placeholder: rgba(82, 106, 136, 0.68);
  --login-field-shadow: none;
  --login-field-focus-shadow: none;
  --login-button-bg: #ffc247;
  --login-button-hover-bg: #ffd06a;
  --login-button-text: #102a45;
  --login-button-shadow: none;
  --login-close-color: #94a3b8;
  --login-close-hover: #475569;
  --login-disabled-bg: #e8edf5;
  --login-disabled-text: #94a3b8;
  min-height: 100vh;
  position: relative;
  overflow: hidden;
  font-family: 'Microsoft YaHei', 'PingFang SC', 'Helvetica Neue', Arial, sans-serif;
  background: var(--login-page-bg);
}

.mihoyo-login-page.theme-light {
  --login-page-bg: #ffffff;
  --login-scene-bg: #ffffff;
  --login-bg-filter: none;
  --login-bg-width: 100%;
  --login-bg-transform: translateX(-6%);
  --login-bg-position: center center;
  --login-bg-overlay:
    linear-gradient(90deg, rgba(247, 253, 255, 0.08) 0%, rgba(247, 253, 255, 0) 48%, rgba(6, 19, 38, 0.44) 100%),
    linear-gradient(180deg, rgba(255, 255, 255, 0.04) 0%, rgba(8, 28, 52, 0.1) 100%);
  --login-nav-bg: rgba(255, 255, 255, 0.68);
  --login-nav-border: rgba(15, 23, 42, 0.08);
  --login-nav-text: rgba(30, 41, 59, 0.72);
  --login-nav-text-active: #0f172a;
  --login-nav-icon: rgba(30, 41, 59, 0.78);
  --login-brand-text: linear-gradient(90deg, #006f93, #006f93);
  --login-brand-shadow: none;
  --login-hero-title-color: #006f93;
  --login-hero-title-shadow: none;
  --login-form-muted: rgba(45, 71, 96, 0.82);
  --login-field-bg: rgba(248, 252, 255, 0.86);
  --login-field-border: rgba(255, 255, 255, 0.96);
  --login-field-placeholder: rgba(60, 83, 108, 0.68);
  --login-field-shadow:
    0 14px 30px rgba(30, 69, 106, 0.18),
    inset 0 1px 0 rgba(255, 255, 255, 0.72);
  --login-field-focus-shadow:
    0 16px 34px rgba(30, 69, 106, 0.24),
    0 0 0 3px rgba(255, 191, 36, 0.18);
  --login-button-bg: #ffc247;
  --login-button-hover-bg: #ffd06a;
  --login-button-shadow: 0 16px 34px rgba(190, 118, 12, 0.24);
}

.mihoyo-login-page.theme-dark {
  --login-page-bg: #061326;
  --login-scene-bg:
    radial-gradient(circle at 16% 18%, rgba(37, 99, 235, 0.32), transparent 32%),
    radial-gradient(circle at 78% 78%, rgba(245, 158, 11, 0.2), transparent 28%),
    linear-gradient(135deg, #061326 0%, #0f2f55 46%, #030712 100%);
  --login-bg-filter: none;
  --login-bg-width: 100%;
  --login-bg-transform: translateX(-6%);
  --login-bg-position: center center;
  --login-bg-overlay:
    linear-gradient(90deg, rgba(3, 7, 18, 0.24) 0%, rgba(8, 18, 34, 0.18) 48%, rgba(3, 7, 18, 0.56) 100%),
    linear-gradient(180deg, rgba(3, 7, 18, 0.1) 0%, rgba(3, 7, 18, 0.28) 100%),
    rgba(0, 0, 0, 0.18);
  --login-nav-bg: rgba(4, 14, 28, 0.58);
  --login-nav-border: rgba(255, 255, 255, 0.16);
  --login-form-text: #e5f0ff;
  --login-form-muted: rgba(213, 230, 255, 0.78);
  --login-field-bg: rgba(38, 59, 89, 0.72);
  --login-field-border: rgba(139, 186, 225, 0.38);
  --login-field-placeholder: #7f93ad;
  --login-button-bg: #ffc247;
  --login-button-hover-bg: #ffd06a;
  --login-close-color: #9fb1c9;
  --login-close-hover: #e5f0ff;
  --login-disabled-bg: rgba(148, 163, 184, 0.14);
  --login-disabled-text: #7f93ad;
}

.background-layer {
  position: fixed;
  inset: 0;
  z-index: 1;
  background: var(--login-scene-bg);
}

.bg-image {
  width: var(--login-bg-width);
  height: 100%;
  max-width: none;
  object-fit: cover;
  object-position: var(--login-bg-position);
  opacity: var(--login-bg-opacity);
  filter: var(--login-bg-filter);
  transform: var(--login-bg-transform);
  transform-origin: center center;
  transition: filter 0.3s ease, opacity 0.3s ease;
}

.bg-overlay {
  position: absolute;
  inset: 0;
  background: var(--login-bg-overlay);
}

.bubbles {
  position: absolute;
  inset: 0;
  pointer-events: none;
  overflow: hidden;
}

.bubble {
  position: absolute;
  bottom: -84px;
  border: 1px solid rgba(235, 250, 255, 0.52);
  border-radius: 999px;
  background:
    radial-gradient(circle at 28% 24%, rgba(255, 255, 255, 0.9) 0 5%, rgba(255, 255, 255, 0.34) 6% 10%, transparent 12%),
    radial-gradient(ellipse at 72% 78%, rgba(111, 212, 255, 0.16), transparent 42%),
    radial-gradient(circle at 50% 50%, rgba(255, 255, 255, 0.05) 0 54%, rgba(136, 219, 255, 0.2) 72%, rgba(255, 255, 255, 0.34) 100%);
  box-shadow:
    inset 2px 3px 8px rgba(255, 255, 255, 0.34),
    inset -6px -9px 16px rgba(28, 87, 132, 0.2),
    0 8px 20px rgba(16, 56, 92, 0.14);
  mix-blend-mode: normal;
  opacity: 0;
  user-select: none;
  transform-origin: center;
  animation: bubbleRise infinite ease-in-out;
}

.bubble::before {
  position: absolute;
  left: 22%;
  top: 19%;
  width: 18%;
  height: 18%;
  border-radius: 50%;
  content: '';
  background: rgba(255, 255, 255, 0.78);
  filter: blur(0.6px);
}

.bubble::after {
  position: absolute;
  right: 15%;
  bottom: 14%;
  width: 34%;
  height: 24%;
  border-radius: 50%;
  content: '';
  background: radial-gradient(ellipse at center, rgba(255, 255, 255, 0.22), transparent 68%);
  filter: blur(1px);
  transform: rotate(-22deg);
}

@keyframes bubbleRise {
  0% {
    transform: translate3d(0, 12vh, 0) scale(0.56);
    opacity: 0;
  }
  10% {
    opacity: var(--bubble-opacity);
  }
  26% {
    transform: translate3d(var(--bubble-mid-drift), -24vh, 0) scale(var(--bubble-scale-large));
    opacity: var(--bubble-opacity);
  }
  44% {
    transform: translate3d(var(--bubble-soft-drift), -44vh, 0) scale(var(--bubble-scale-small));
  }
  62% {
    transform: translate3d(var(--bubble-wide-drift), -68vh, 0) scale(var(--bubble-scale-large));
    opacity: var(--bubble-opacity);
  }
  82% {
    transform: translate3d(var(--bubble-end-drift), -92vh, 0) scale(var(--bubble-scale));
    opacity: var(--bubble-opacity);
  }
  100% {
    transform: translate3d(var(--bubble-end-drift), -116vh, 0) scale(0.74);
    opacity: 0;
  }
}

.top-nav {
  position: relative;
  z-index: 10;
  background: var(--login-nav-bg);
  border-bottom: 1px solid var(--login-nav-border);
  backdrop-filter: blur(10px);
}

.nav-container {
  height: 64px;
  min-height: 64px;
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
  gap: clamp(18px, 1.7vw, 24px);
}

.brand-link {
  flex: 0 1 auto;
  min-width: 0;
  height: 64px;
  gap: 10px;
  text-decoration: none;
}

.logo-icon {
  width: 42px;
  height: 42px;
  object-fit: contain;
  filter: none;
}

.logo-text {
  position: relative;
  display: inline-flex;
  align-items: center;
  min-height: 24px;
  color: transparent;
  background: var(--login-brand-text);
  background-clip: text;
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
  font-size: 24px;
  font-weight: 820;
  line-height: 1;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  filter: var(--login-brand-shadow);
}

.nav-divider {
  width: 1px;
  height: 32px;
  flex: 0 0 auto;
  background: var(--login-nav-border);
}

.nav-menu {
  gap: clamp(20px, 2.1vw, 34px);
  height: 64px;
  min-width: 0;
}

.nav-item {
  position: relative;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  height: 64px;
  padding: 1px 0 0;
  border: 0;
  background: transparent;
  color: var(--login-nav-text);
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
  color: var(--login-nav-text-active);
}

.nav-item::after {
  position: absolute;
  left: 50%;
  bottom: 6px;
  width: 26px;
  height: 2px;
  border-radius: 999px;
  content: '';
  background: #ffbf24;
  box-shadow: none;
  opacity: 0;
  transform: translateX(-50%) scaleX(0.55);
  transition: opacity 0.25s ease, transform 0.25s ease;
}

.nav-item:hover::after {
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
  color: var(--login-nav-icon);
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

.login-main-content {
  position: relative;
  z-index: 2;
  min-height: calc(100vh - 64px);
  width: 100%;
  display: flex;
  flex-direction: row;
  align-items: center;
  justify-content: space-between;
  padding: 0 clamp(32px, 4vw, 76px);
  overflow: visible;
  background: transparent;
}

.game-info-section {
  position: absolute;
  right: clamp(92px, 6.8vw, 140px);
  top: 54%;
  width: clamp(300px, 20vw, 360px);
  display: flex;
  flex-direction: column;
  align-items: center;
  transform: translateY(-50%);
  text-align: center;
  color: white;
  isolation: isolate;
}

.game-logo {
  margin-bottom: 14px;
  opacity: 0;
  animation: magicItemReveal 0.76s cubic-bezier(0.19, 1, 0.22, 1) 0.18s both;
}

.logo-symbol {
  width: 96px;
  height: 96px;
  display: grid;
  place-items: center;
}

.game-logo-img {
  width: 100%;
  height: 100%;
  object-fit: contain;
  filter: none;
}

.game-title {
  max-width: 100%;
  margin: 0 0 28px;
  color: var(--login-hero-title-color);
  font-size: clamp(34px, 2.7vw, 48px);
  line-height: 1.12;
  font-weight: 800;
  white-space: nowrap;
  text-shadow: var(--login-hero-title-shadow);
  opacity: 0;
  animation: magicItemReveal 0.72s cubic-bezier(0.19, 1, 0.22, 1) 0.34s both;
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

.hero-login-panel {
  width: 100%;
  padding: 0;
  border: 0;
  background: transparent;
  box-shadow: none;
  opacity: 0;
  animation: magicItemReveal 0.72s cubic-bezier(0.19, 1, 0.22, 1) 0.46s both;
}

.hero-login-form {
  padding: 0;
  border: 0;
  background: transparent;
  box-shadow: none;
  backdrop-filter: none;
}

.hero-login-form .form-group {
  margin-bottom: 14px;
}

.input-wrapper {
  position: relative;
  display: flex;
  align-items: center;
  min-height: 48px;
  border: 1px solid var(--login-field-border);
  border-radius: 12px;
  background: var(--login-field-bg);
  box-shadow: var(--login-field-shadow);
  overflow: hidden;
  transition: border-color 0.25s ease, background 0.25s ease, box-shadow 0.25s ease;
}

.input-wrapper:focus-within {
  border-color: var(--login-theme-text-color);
  box-shadow: var(--login-field-focus-shadow);
}

.form-input {
  flex: 1;
  min-width: 0;
  height: 48px;
  padding: 0 46px 0 16px;
  border: 0;
  outline: none;
  background: transparent;
  color: var(--login-form-text);
  font-size: 14px;
}

.form-input::placeholder {
  color: var(--login-field-placeholder);
}

.clear-btn,
.toggle-password-btn {
  position: absolute;
  right: 12px;
  top: 50%;
  transform: translateY(-50%);
  border: 0;
  background: none;
  color: var(--login-field-placeholder);
  cursor: pointer;
}

.clear-btn {
  width: 20px;
  height: 20px;
  border-radius: 50%;
  background: rgba(148, 163, 184, 0.62);
  color: white;
  font-size: 12px;
}

.toggle-password-btn {
  font-size: 16px;
}

.form-options {
  margin: 4px 0 18px;
}

.remember-label {
  display: flex;
  align-items: flex-start;
  gap: 10px;
  color: var(--login-form-muted);
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
  border-radius: 12px;
  background: var(--login-button-bg);
  color: var(--login-button-text);
  font-size: 16px;
  font-weight: 800;
  cursor: pointer;
  box-shadow: var(--login-button-shadow);
  transition: background 0.25s ease, color 0.25s ease, box-shadow 0.25s ease;
}

.login-btn:hover:not(:disabled) {
  background: var(--login-button-hover-bg);
  box-shadow: var(--login-button-shadow);
}

.login-btn:disabled {
  background: var(--login-disabled-bg);
  color: var(--login-disabled-text);
  cursor: not-allowed;
}

@media (prefers-reduced-motion: reduce) {
  .bubble,
  .game-logo,
  .game-title,
  .hero-login-panel {
    animation: none;
  }

  .game-logo,
  .game-title,
  .hero-login-panel {
    opacity: 1;
  }
}

@media (max-width: 1024px) {
  .login-main-content {
    flex-direction: column;
    justify-content: center;
    padding: 40px 20px;
    text-align: center;
  }

  .game-info-section {
    position: static;
    width: min(360px, 90vw);
    margin-bottom: 0;
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

  .brand-link {
    height: 56px;
  }

  .logo-icon {
    width: 28px;
    height: 28px;
  }

  .logo-text {
    max-width: 44vw;
    font-size: 16px;
  }

  .nav-menu {
    display: none;
  }

  .nav-divider {
    display: none;
  }

  .nav-right {
    gap: 8px;
  }

  .header-icon-btn {
    width: 32px;
    height: 32px;
  }

  .login-main-content {
    min-height: calc(100vh - 56px);
    justify-content: flex-end;
    padding: 0 20px 42px;
  }

  .bg-image {
    width: 100%;
    object-position: 50% center;
    transform: none;
  }

  .game-info-section {
    width: min(340px, 92vw);
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

  .hero-login-panel {
    padding: 0;
  }
}
</style>
