<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import api from '../api/index'
import { useI18n } from '../utils/i18n'
import { User, Lock } from '@element-plus/icons-vue'
import logo from '../assets/logo.png'

import { sha256 } from '../utils/crypto'

const { t } = useI18n()
const router = useRouter()
const loading = ref(false)
const form = ref({
  username: '',
  password: ''
})

async function handleLogin() {
  if (!form.value.username || !form.value.password) {
    ElMessage.warning('Please enter username and password')
    return
  }
  
  loading.value = true
  try {
    const hashedPassword = await sha256(form.value.password)
    const res = await api.post('/v1/auth/login', {
      username: form.value.username,
      password: hashedPassword
    })
    
    localStorage.setItem('token', res.data.token)
    localStorage.setItem('user_role', res.data.role)
    localStorage.setItem('username', form.value.username)
    localStorage.setItem('nickname', res.data.nickname || form.value.username)
    
    router.push('/')
  } catch (e: any) {
    const errorMsg = e.response?.data?.error
    if (errorMsg === 'invalid credentials') {
      ElMessage.error(t.value.common.loginError)
    } else {
      ElMessage.error(errorMsg || 'Login failed')
    }
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div class="login-container">
    <div class="login-card glass-card">
      <div class="login-header">
        <div class="logo-icon">
          <img :src="logo" alt="Logo" style="width: 64px; height: 64px;" />
        </div>
        <h2>{{ t.common.title }}</h2>
        <p>{{ t.common.login }}</p>
      </div>

      <el-form :model="form" class="login-form">
        <el-form-item>
          <el-input 
            v-model="form.username" 
            :placeholder="t.common.username"
            :prefix-icon="User"
            size="large"
          />
        </el-form-item>
        <el-form-item>
          <el-input 
            v-model="form.password" 
            :placeholder="t.common.password"
            :prefix-icon="Lock"
            type="password" 
            show-password
            size="large"
            @keyup.enter="handleLogin"
          />
        </el-form-item>
        <el-button 
          type="primary" 
          class="login-button" 
          :loading="loading" 
          @click="handleLogin"
          size="large"
        >
          {{ t.common.login }}
        </el-button>
      </el-form>
    </div>
  </div>
</template>

<style scoped>
.login-container {
  height: 100vh;
  width: 100vw;
  display: flex;
  justify-content: center;
  align-items: center;
  background: var(--bg-body);
  background-image: 
    radial-gradient(at 0% 0%, rgba(59, 130, 246, 0.15) 0px, transparent 50%),
    radial-gradient(at 100% 100%, rgba(6, 182, 212, 0.15) 0px, transparent 50%);
}

.login-card {
  width: 420px;
  padding: 48px 40px;
  text-align: center;
  background: var(--bg-card);
  backdrop-filter: blur(20px);
  border: 1px solid var(--border-color);
  border-radius: 24px;
  box-shadow: 0 20px 40px rgba(0, 0, 0, 0.08);
}

.login-header {
  margin-bottom: 40px;
}

.login-header .logo-icon {
  width: 64px;
  height: 64px;
  display: flex;
  justify-content: center;
  align-items: center;
  margin: 0 auto 20px;
}

.login-header h2 {
  font-size: 28px;
  font-weight: 700;
  margin: 0;
  letter-spacing: -0.5px;
  background: linear-gradient(135deg, var(--accent-blue), var(--accent-cyan));
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
  background-clip: text;
}

.login-header p {
  color: var(--text-secondary);
  font-size: 14px;
  margin-top: 10px;
}

.login-form {
  margin-top: 24px;
}

.login-form :deep(.el-input__wrapper) {
  background: var(--bg-body);
  border: 1px solid var(--border-color);
  box-shadow: none !important;
  border-radius: 12px;
  padding: 4px 16px;
  transition: all 0.2s;
}

.login-form :deep(.el-input__wrapper.is-focus) {
  border-color: var(--accent-blue);
  box-shadow: 0 0 0 2px rgba(59, 130, 246, 0.1) !important;
}

.login-button {
  width: 100%;
  margin-top: 20px;
  height: 52px;
  font-size: 16px;
  font-weight: 600;
  border-radius: 14px;
  background: linear-gradient(135deg, var(--accent-blue), var(--accent-cyan));
  border: none;
  transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
}

.login-button:hover {
  transform: translateY(-2px);
  box-shadow: 0 8px 20px rgba(59, 130, 246, 0.3);
}
</style>
