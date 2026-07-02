<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { 
  User,
  Lock,
  Setting,
  EditPen,
  Iphone,
  Message,
  Sunny,
  Check,
  CircleCheckFilled,
  RefreshRight
} from '@element-plus/icons-vue'
import { useI18n } from '../utils/i18n'
import { sha256 } from '../utils/crypto'
import { applyTheme, normalizeTheme, persistTheme, readStoredTheme, type AppTheme } from '../utils/theme'

const { locale, t, text, setLocale, supportedLocales, localeLabels } = useI18n()

// Current values in components (controlled state)
const username = ref(localStorage.getItem('username') || '')
const nickname = ref(localStorage.getItem('nickname') || '')
const role = ref(localStorage.getItem('user_role') || '')
const theme = ref<AppTheme>(readStoredTheme())
const phone = ref(localStorage.getItem('user_phone') || '')
const email = ref(localStorage.getItem('user_email') || '')

// Password State
const pwdForm = ref({
  old: '',
  new: '',
  confirm: ''
})

// Visual only preview for theme
function previewTheme(val: string | number | boolean | undefined) {
  const nextTheme = normalizeTheme(String(val))
  theme.value = nextTheme
  applyTheme(nextTheme)
}

async function saveAllSettings() {
  const token = localStorage.getItem('token')
  
  try {
    // 1. Save Basic Profile to Backend
    const profileRes = await fetch('/v1/auth/profile', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'Authorization': `Bearer ${token}`
      },
      body: JSON.stringify({
        nickname: nickname.value,
        phone: phone.value,
        email: email.value
      })
    })

    if (!profileRes.ok) {
        throw new Error(await profileRes.text())
    }

    // 2. Persist Local Preferences
    localStorage.setItem('nickname', nickname.value)
    localStorage.setItem('user_phone', phone.value)
    localStorage.setItem('user_email', email.value)
    persistTheme(theme.value)
    
    setLocale(locale.value)
    
    ElMessage.success({
      message: text('设置已持久化保存', 'Settings persisted successfully'),
      type: 'success'
    })
    
    window.dispatchEvent(new Event('storage'))
  } catch (err: any) {
    ElMessage.error(text(`保存失败: ${err.message}`, `Save failed: ${err.message}`))
  }
}

async function handleUpdatePassword() {
  if (!pwdForm.value.old || !pwdForm.value.new || !pwdForm.value.confirm) {
    ElMessage.warning(text('请填写完整密码信息', 'Please fill all password fields'))
    return
  }
  if (pwdForm.value.new !== pwdForm.value.confirm) {
    ElMessage.error(text('两次输入的密码不一致', 'Passwords do not match'))
    return
  }
  
  try {
    await ElMessageBox.confirm(
      text('确定要修改密码吗？修改后将强制重新登录', 'Are you sure to change password? You will need to re-login.'),
      text('提示', 'Tip'),
      { type: 'warning', confirmButtonText: text('确定修改', 'Confirm') }
    )
    
    const token = localStorage.getItem('token')
    const hashedOld = await sha256(pwdForm.value.old)
    const hashedNew = await sha256(pwdForm.value.new)
    
    const res = await fetch('/v1/auth/password', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'Authorization': `Bearer ${token}`
      },
      body: JSON.stringify({
        old: hashedOld,
        new: hashedNew
      })
    })

    if (!res.ok) {
        throw new Error(await res.text())
    }

    ElMessage.success(text('密码修改成功，正在退出...', 'Password changed successfully, logging out...'))
    
    setTimeout(() => {
        localStorage.removeItem('token')
        window.location.href = '/login'
    }, 1000)
    
  } catch (err: any) {
      if (err !== 'cancel') {
          ElMessage.error(text(`修改失败: ${err.message}`, `Update failed: ${err.message}`))
      }
  }
}

async function fetchProfile() {
  const token = localStorage.getItem('token')
  try {
    const res = await fetch('/v1/auth/profile', {
      headers: { 'Authorization': `Bearer ${token}` }
    })
    if (res.ok) {
      const data = await res.json()
      nickname.value = data.nickname || ''
      phone.value = data.phone || ''
      email.value = data.email || ''
      role.value = data.role || ''
      username.value = data.username || ''
      
      // Update local storage for consistency if needed
      localStorage.setItem('nickname', data.nickname || '')
      localStorage.setItem('user_role', data.role || '')
    }
  } catch (err) {
    console.error('Failed to fetch profile:', err)
  }
}

onMounted(() => {
  const savedTheme = readStoredTheme()
  theme.value = savedTheme
  applyTheme(savedTheme)
  fetchProfile()
})
</script>

<template>
  <div class="profile-dashboard">
    <el-row :gutter="20">
      <!-- Left Column: Unified Settings Card -->
      <el-col :xs="24" :sm="24" :md="14" :lg="15">
        <el-card class="dashboard-card" shadow="never">
          <template #header>
            <div class="card-header">
              <div class="header-left">
                <el-icon><Setting /></el-icon>
                <span>{{ text('常规配置与偏好', 'General Settings & Preferences') }}</span>
              </div>
              <el-button type="primary" size="small" @click="saveAllSettings">
                  <el-icon><Check /></el-icon>
                  <span>{{ text('保存更改', 'Save Changes') }}</span>
              </el-button>
            </div>
          </template>
          
          <div class="card-body">
            <!-- Section 1: Basic Info -->
            <div class="settings-section">
              <h4 class="section-title">
                  <el-icon><User /></el-icon>
                  {{ text('账号基础资料', 'Basic Account Profile') }}
              </h4>
              <div class="form-grid">
                <div class="form-item">
                  <label>{{ text('登录名称', 'Username') }}</label>
                  <el-input v-model="username" disabled />
                </div>
                <div class="form-item">
                  <label>{{ text('显示昵称', 'Nickname') }}</label>
                  <el-input v-model="nickname" :prefix-icon="EditPen" />
                </div>
                <div class="form-item">
                  <label>{{ text('手机号码', 'Phone') }}</label>
                  <el-input v-model="phone" :prefix-icon="Iphone" placeholder="138****0000" />
                </div>
                <div class="form-item">
                  <label>{{ text('电子邮箱', 'Email') }}</label>
                  <el-input v-model="email" :prefix-icon="Message" placeholder="user@example.com" />
                </div>
              </div>
            </div>

            <el-divider class="tight-divider" />

            <!-- Section 2: Personalization - Horizontal Layout to save height -->
            <div class="settings-section pref-section-combined">
              <h4 class="section-title">
                  <el-icon><Sunny /></el-icon>
                  {{ text('个性化设定', 'Personalization') }}
              </h4>
              <div class="pref-grid-horizontal">
                <div class="pref-box compact">
                  <div class="pref-info">
                    <div class="sub-label">{{ t.settings.theme }}</div>
                    <div class="sub-desc">{{ text('深浅模式', 'Theme mode') }}</div>
                  </div>
                  <el-radio-group v-model="theme" @change="previewTheme" size="small">
                    <el-radio-button label="light">{{ text('明亮', 'Light') }}</el-radio-button>
                    <el-radio-button label="dark">{{ text('暗黑', 'Dark') }}</el-radio-button>
                  </el-radio-group>
                </div>
                <div class="pref-box compact">
                  <div class="pref-info">
                    <div class="sub-label">{{ t.settings.language }}</div>
                    <div class="sub-desc">{{ text('语言选择', 'Language') }}</div>
                  </div>
                  <el-radio-group v-model="locale" size="small">
                    <el-radio-button v-for="item in supportedLocales" :key="item" :label="item">
                      {{ localeLabels[item] }}
                    </el-radio-button>
                  </el-radio-group>
                </div>
              </div>
            </div>

            <div class="profile-info-footer mt-20">
               <div class="role-hint">
                  <span class="dot"></span>
                  {{ text('身份：', 'Role: ') }}
                  <el-tag size="small" :type="role === 'admin' ? 'danger' : 'success'" effect="plain" class="role-tag">{{ role.toUpperCase() }}</el-tag>
               </div>
               <span class="info-text">{{ text('已开启服务端实时同步', 'Cloud sync active') }}</span>
            </div>
          </div>
        </el-card>
      </el-col>

      <!-- Right Column: Security -->
      <el-col :xs="24" :sm="24" :md="10" :lg="9">
        <el-card class="dashboard-card security-card-full" shadow="never">
          <template #header>
            <div class="card-header">
              <div class="header-left">
                <el-icon><Lock /></el-icon>
                <span>{{ text('安全中心', 'Security') }}</span>
              </div>
              <el-button type="primary" size="small" @click="handleUpdatePassword">
                  <el-icon><RefreshRight /></el-icon>
                  <span>{{ text('立即更新', 'Update') }}</span>
              </el-button>
            </div>
          </template>
          <div class="card-body">
            <div class="security-full-form">
              <div class="form-item">
                <label>{{ text('旧密码', 'Current Password') }}</label>
                <el-input v-model="pwdForm.old" type="password" show-password />
              </div>
              <div class="form-item mt-16">
                <label>{{ text('设定新密码', 'New Password') }}</label>
                <el-input v-model="pwdForm.new" type="password" show-password />
              </div>
              <div class="form-item mt-16">
                <label>{{ text('确认新密码', 'Confirm New Password') }}</label>
                <el-input v-model="pwdForm.confirm" type="password" show-password />
              </div>
              
              <div class="security-footer mt-20">
                 <div class="security-tip">
                    <el-icon><CircleCheckFilled /></el-icon>
                    <span>{{ text('修改后将强制注销会话', 'Forced logout on change') }}</span>
                 </div>
              </div>
            </div>
          </div>
        </el-card>
      </el-col>
    </el-row>
  </div>
</template>

<style scoped lang="scss">
.profile-dashboard {
  animation: fadeIn 0.4s ease-out;
}

.dashboard-card {
  border: 1px solid var(--border-color);
  background: var(--bg-card);
  border-radius: 12px;
}

.mt-16 { margin-top: 16px; }
.mt-20 { margin-top: 20px; }

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.header-left {
  display: flex;
  align-items: center;
  gap: 10px;
  font-weight: 700;
  font-size: 15px;
  color: var(--text-primary);
}

.header-left .el-icon {
  font-size: 18px;
  color: var(--accent-blue);
}

.card-body {
  padding: 12px 16px;
}

/* Section Styling */
.settings-section {
  margin-bottom: 20px;
}

.section-title {
  display: flex;
  align-items: center;
  gap: 8px;
  margin: 0 0 16px 0;
  font-size: 14px;
  color: var(--text-primary);
  font-weight: 600;
}

.form-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 12px 24px;
}

.form-item label {
  display: block;
  font-size: 13px;
  color: var(--text-secondary);
  margin-bottom: 6px;
  font-weight: 500;
}

:deep(.el-input__wrapper) {
  background: var(--bg-primary) !important;
  border: 1px solid var(--border-color) !important;
  box-shadow: none !important;
}

/* PREFERENCES: Horizontal to save massive height */
.pref-grid-horizontal {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 12px;
}

.pref-box.compact {
    padding: 10px 14px;
    background: var(--bg-primary);
    border-radius: 8px;
    border: 1px solid var(--border-color);
    display: flex;
    justify-content: space-between;
    align-items: center;
}

.sub-label {
    font-size: 13px;
    font-weight: 600;
}

.sub-desc {
    font-size: 10px;
    color: var(--text-muted);
}

.profile-info-footer {
  padding-top: 12px;
  border-top: 1px dashed var(--border-color);
  display: flex;
  justify-content: space-between;
  align-items: center;
  font-size: 12px;
}

.role-hint {
  display: flex;
  align-items: center;
  gap: 8px;
}

.dot {
  width: 6px; height: 6px;
  background: var(--accent-green);
  border-radius: 50%;
}

/* Security */
.security-card-full {
  height: 100%;
}

.security-tip {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 12px;
  color: var(--accent-blue);
  background: rgba(59, 130, 246, 0.05);
  padding: 10px 14px;
  border-radius: 6px;
}

/* Final Color & UI Correction */
:deep(.el-radio-button__inner) {
  background: var(--bg-primary) !important;
  color: var(--text-primary) !important; /* Ensure visibility */
  border: 1px solid var(--border-color) !important;
  padding: 8px 16px;
  font-size: 12px;
}

:deep(.el-radio-button__original-radio:checked + .el-radio-button__inner) {
  background: var(--accent-blue) !important;
  color: #ffffff !important;
  border-color: var(--accent-blue) !important;
}

.tight-divider {
    margin: 20px 0 !important;
}

@media (max-width: 1200px) {
  .pref-grid-horizontal {
    grid-template-columns: 1fr;
  }
}

@keyframes fadeIn {
  from { opacity: 0; transform: translateY(10px); }
  to { opacity: 1; transform: translateY(0); }
}
</style>
