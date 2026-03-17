<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { ElMessageBox, ElMessage } from 'element-plus'
import api from '../api/index'
import { useI18n } from '../utils/i18n'
import { 
  Setting, 
  Key, 
  Message, 
  Bell, 
  Operation, 
  CopyDocument, 
  RefreshRight, 
  Delete,
  Sunny,
  Moon,
  Timer,
  Share,
  Monitor,
  Grid,
  Plus
} from '@element-plus/icons-vue'

const { locale, t, toggleLocale } = useI18n()

// Tab state
const activeTab = ref('basic')

// View state
const credentialView = ref('grid') // 'grid' | 'list'

// Settings Data
const keys = ref<any[]>([])
const loading = ref(false)
const modeLoading = ref(false)
const currentEnv = ref('standalone')
const currentMode = ref('ap')
const showDialog = ref(false)
const keyForm = ref({ key: '', description: '', expDays: 0 })

// Preferences
const theme = ref(document.documentElement.getAttribute('data-theme') || 'light')
const eventRetention = ref(30)

// Advanced Settings States
const webhookForm = ref({
  url: '',
  events: ['register', 'deregister', 'health_change']
})
const alarmForm = ref({
  enabled: false,
  threshold: 80,
  contact: 'email'
})
const smtpForm = ref({
  host: 'smtp.example.com',
  port: 465,
  user: '',
  pass: '',
  from: ''
})

async function fetchKeys() {
  loading.value = true
  try {
    const res = await api.get('/v1/settings/apikeys')
    keys.value = res.data || []
  } catch (e) {
    console.error('fetch keys failed', e)
  } finally {
    loading.value = false
  }
}

async function fetchMode() {
  try {
    const res = await api.get('/v1/settings/mode')
    currentMode.value = res.data.mode || 'ap'
    currentEnv.value = res.data.env || 'standalone'
  } catch (e) {
    console.error('fetch mode failed', e)
  }
}

async function handleEnvChange(targetEnv: string) {
  if (currentEnv.value === targetEnv) return
  
  modeLoading.value = true
  try {
    await api.post(`/v1/settings/mode?env=${targetEnv}`)
    const label = targetEnv === 'cluster' ? t.value.settings.clusterTitle : t.value.settings.singleTitle
    ElMessage.success(t.value.settings.envSwitchSuccess.replace('{env}', label))
    
    // Force local state update immediately to avoid UI lag/flicker
    currentEnv.value = targetEnv
    
    // Wait a bit before fetching to allow backend state to propagate
    setTimeout(() => {
      fetchMode()
    }, 1000)
  } catch (e: any) {
    const errorMsg = e.response?.data?.error || 'Switch failed'
    ElMessage.error(errorMsg)
    fetchMode()
  } finally {
    modeLoading.value = false
  }
}

async function handleModeChange(targetMode: string) {
  if (currentMode.value === targetMode) return
  
  modeLoading.value = true
  try {
    await api.post(`/v1/settings/mode?mode=${targetMode}`)
    const modeLabel = targetMode === 'cp' ? '强一致性' : '最终一致性'
    ElMessage.success(t.value.settings.modeSwitchSuccess.replace('{mode}', modeLabel))
    
    currentMode.value = targetMode
    setTimeout(() => {
      fetchMode()
    }, 1000)
  } catch (e: any) {
    const errorMsg = e.response?.data?.error || 'Switch failed'
    ElMessage.error(errorMsg)
    fetchMode()
  } finally {
    modeLoading.value = false
  }
}

function handleThemeChange(val: string) {
  theme.value = val
  document.documentElement.setAttribute('data-theme', val)
  if (val === 'dark') {
    document.documentElement.classList.add('dark')
  } else {
    document.documentElement.classList.remove('dark')
  }
  document.cookie = `theme=${val};path=/;max-age=31536000`
}

function handleLangChange(val: string) {
  if (locale.value !== val) {
    toggleLocale()
  }
}

function generateRandKey() {
  const chars = 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789'
  let res = 'eden_'
  for (let i = 0; i < 24; i++) {
    res += chars.charAt(Math.floor(Math.random() * chars.length))
  }
  keyForm.value.key = res
}

async function handleGenerate() {
  try {
    const payload = {
      key: keyForm.value.key,
      description: keyForm.value.description,
      expires_at: keyForm.value.expDays > 0 ? Math.floor(Date.now() / 1000) + keyForm.value.expDays * 86400 : 0
    }
    await api.post('/v1/settings/apikey', payload)
    ElMessage.success(t.value.common.success)
    showDialog.value = false
    fetchKeys()
  } catch (e: any) {
    ElMessage.error(e.response?.data?.error || 'Failed')
  }
}

async function deleteKey(key: string) {
  try {
    await ElMessageBox.confirm(
      t.value.settings.deleteConfirm,
      t.value.common.warning,
      { type: 'warning' }
    )
    await api.post(`/v1/settings/apikey/delete?key=${key}`)
    ElMessage.success(t.value.common.success)
    fetchKeys()
  } catch {}
}

function copyKey(key: string) {
  navigator.clipboard.writeText(key)
  ElMessage.success(t.value.settings.copySuccess)
}

function handleSaveGeneral() {
  ElMessage.success(t.value.settings.saveSettings)
}

function formatTime(ts: number) {
  if (!ts) return t.value.settings.neverExpire
  return new Date(ts * 1000).toLocaleDateString()
}

onMounted(() => {
  fetchKeys()
  fetchMode()
})
</script>

<template>
  <div class="settings-container">
    <el-tabs v-model="activeTab" class="settings-tabs">
      <!-- 1. Basic Settings -->
      <el-tab-pane name="basic">
        <template #label>
          <span class="tab-label"><el-icon><Setting /></el-icon>{{ t.settings.basic }}</span>
        </template>
        
        <div class="tab-content grid-layout">
          <!-- Consistency Mode -->
          <div class="settings-section glass-card mode-section">
            <div class="section-header">
              <el-icon><Operation /></el-icon>
              <h4>运行模式</h4>
              <div class="status-tags">
                <div class="status-indicator">
                  <span class="indicator-dot" :class="currentEnv === 'cluster' ? 'active' : 'idle'"></span>
                  <span class="dot-label">当前运行:</span>
                  <el-tag :type="currentEnv === 'cluster' ? 'primary' : 'info'" size="small" effect="dark">
                    {{ currentEnv === 'cluster' ? '集群模式' : '单机模式' }}
                  </el-tag>
                </div>
                <!-- Only show protocol if in cluster mode -->
                <div class="status-indicator" v-if="currentEnv === 'cluster'">
                  <span class="dot-label">一致性:</span>
                  <el-tag :class="currentMode === 'cp' ? 'tag-cp' : 'tag-ap'" size="small" effect="dark">
                    {{ currentMode === 'cp' ? '强一致性' : '最终一致性' }}
                  </el-tag>
                </div>
              </div>
            </div>
            
            <div class="consistency-wrapper-v7" v-loading="modeLoading">
              <!-- Grid Selection -->
              <div class="main-mode-grid">
                <!-- Standalone Mode Card -->
                <div 
                  class="mode-card-v7" 
                  :class="{ active: currentEnv === 'standalone' }"
                  @click="handleEnvChange('standalone')"
                >
                  <div class="card-glow"></div>
                  <div class="card-inner">
                    <div class="mode-icon-v7"><el-icon><Monitor /></el-icon></div>
                    <div class="mode-content-v7">
                      <div class="mode-title-v7">单机模式</div>
                      <div class="mode-desc-v7">本地自闭环，轻量化部署，不涉及分布式一致性协议。</div>
                    </div>
                  </div>
                </div>
                
                <!-- Cluster Mode Card with Integrated Switch -->
                <div 
                  class="mode-card-v7 cluster" 
                  :class="{ active: currentEnv === 'cluster' }"
                  @click="handleEnvChange('cluster')"
                >
                  <div class="card-glow"></div>
                  <div class="card-inner">
                    <div class="mode-icon-v7 cluster"><el-icon><Share /></el-icon></div>
                    <div class="mode-content-v7">
                      <div class="mode-title-v7">分布式集群模式</div>
                      
                      <div class="integrated-toggle-v7" :class="[currentMode]" v-if="currentEnv === 'cluster'" @click.stop>
                        <div 
                          class="toggle-option" 
                          :class="{ selected: currentMode === 'ap' }"
                          @click="handleModeChange('ap')"
                        >
                          <div class="toggle-bg"></div>
                          <div class="toggle-text">
                            <span class="primary">AP 模式</span>
                            <span class="secondary">可用性优先</span>
                          </div>
                        </div>
                        <div 
                          class="toggle-option" 
                          :class="{ selected: currentMode === 'cp' }"
                          @click="handleModeChange('cp')"
                        >
                          <div class="toggle-bg"></div>
                          <div class="toggle-text">
                            <span class="primary">CP 模式</span>
                            <span class="secondary">强一致性</span>
                          </div>
                        </div>
                      </div>
                      <div class="mode-desc-v7" v-else>启用分布式协调，支持 AP / CP 协议切换。</div>
                    </div>
                  </div>
                </div>
              </div>
            </div>

            <!-- Technical Summary -->
            <div class="technical-footer-v7">
              <div class="info-bubble-v7" :class="[currentEnv === 'standalone' ? 'standalone' : currentMode]">
                <el-icon><InfoFilled /></el-icon>
                <div class="bubble-content">
                  <template v-if="currentEnv === 'standalone'">
                    <strong>单机模式：</strong>本地存储，适用于测试或边缘单点场景。
                  </template>
                  <template v-else-if="currentMode === 'ap'">
                    <strong>AP 模式 (Gossip)：</strong>分区容错性高，优先保证系统可用性，节点间数据最终对齐。
                  </template>
                  <template v-else>
                    <strong>CP 模式 (Raft)：</strong>强一致性保证，要求集群多数派存活，适用于核心元数据管理。
                  </template>
                </div>
              </div>
            </div>
          </div>

          <!-- Preferences -->
          <div class="settings-section glass-card">
            <div class="section-header">
              <el-icon><Operation /></el-icon>
              <h4>{{ t.settings.preferences }}</h4>
            </div>
            <el-form label-position="left" label-width="140px">
              <el-form-item :label="t.settings.language">
                <el-radio-group v-model="locale" @change="handleLangChange">
                  <el-radio-button label="zh">简体中文</el-radio-button>
                  <el-radio-button label="en">English</el-radio-button>
                </el-radio-group>
              </el-form-item>
              
              <el-form-item :label="t.settings.theme">
                <el-radio-group v-model="theme" @change="handleThemeChange">
                  <el-radio-button label="light">
                    <el-icon><Sunny /></el-icon> {{ t.settings.light }}
                  </el-radio-button>
                  <el-radio-button label="dark">
                    <el-icon><Moon /></el-icon> {{ t.settings.dark }}
                  </el-radio-button>
                </el-radio-group>
              </el-form-item>

              <el-form-item :label="t.settings.retention">
                <div style="display: flex; align-items: center; gap: 12px; width: 100%;">
                  <el-slider v-model="eventRetention" :min="1" :max="365" style="flex: 1" />
                  <span class="val-text">{{ eventRetention }}d</span>
                </div>
              </el-form-item>

              <div style="margin-top: auto; padding-top: 24px;">
                <el-button type="primary" @click="handleSaveGeneral" style="width: 100%; height: 44px; font-weight: 600;">
                  {{ t.settings.saveSettings }}
                </el-button>
              </div>
            </el-form>
          </div>
        </div>
      </el-tab-pane>

      <!-- 2. Credentials Tab -->
      <el-tab-pane name="credentials">
        <template #label>
          <span class="tab-label"><el-icon><Key /></el-icon>{{ t.settings.credentials }}</span>
        </template>
        
        <div class="tab-content">
          <div class="content-header credentials-header">
            <div class="header-main">
              <div class="title-row">
                <h3 class="tab-content-title">{{ t.settings.credentials }}</h3>
                <el-radio-group v-model="credentialView" size="small" class="view-switch">
                  <el-radio-button label="list"><el-icon><Operation /></el-icon></el-radio-button>
                  <el-radio-button label="grid"><el-icon><Grid /></el-icon></el-radio-button>
                </el-radio-group>
              </div>
              <p class="content-desc">{{ t.settings.credentialsDesc }}</p>
              <div class="action-row">
                <el-button type="primary" @click="showDialog = true; generateRandKey()">
                  <el-icon style="margin-right: 4px;"><Plus /></el-icon>
                  {{ t.settings.generateKey }}
                </el-button>
              </div>
            </div>
          </div>

          <!-- Card/Grid View -->
          <div v-if="credentialView === 'grid'" class="key-grid">
            <div v-for="k in keys" :key="k.key" class="key-card glass-card" :class="{ 'is-expired': k.status === 'expired' }">
              <div class="key-header">
                <code class="key-code" :title="k.key">{{ k.key.substring(0, 24) }}...</code>
                <div class="key-actions">
                  <el-tag v-if="k.status === 'expired'" type="danger" size="small" effect="dark" class="status-tag">
                    {{ t.settings.expired }}
                  </el-tag>
                  <el-button link type="primary" :icon="CopyDocument" @click="copyKey(k.key)" />
                  <el-button link type="danger" :icon="Delete" @click="deleteKey(k.key)" />
                </div>
              </div>
              <div class="key-body">
                <div class="key-desc">{{ k.description || t.settings.noDescription }}</div>
                <div class="key-info">
                  <div class="key-date" :class="{ 'text-danger': k.status === 'expired' }">
                    <el-icon><Timer /></el-icon> {{ t.settings.expiration }}: {{ formatTime(k.expires_at) }}
                  </div>
                  <div class="key-date">
                    <el-icon><Timer /></el-icon> {{ t.settings.created }}: {{ new Date(k.created_at * 1000).toLocaleDateString() }}
                  </div>
                </div>
              </div>
            </div>
          </div>

          <!-- List View -->
          <div v-else class="key-list glass-card">
            <el-table :data="keys" style="width: 100%" class="custom-table">
              <el-table-column prop="key" label="Key" width="380">
                <template #default="scope">
                  <div class="key-with-copy">
                    <code class="key-code-list" :title="scope.row.key">{{ scope.row.key.substring(0, 32) }}...</code>
                    <el-button link type="primary" :icon="CopyDocument" @click="copyKey(scope.row.key)" />
                  </div>
                </template>
              </el-table-column>
              <el-table-column prop="description" :label="t.settings.description" min-width="150" />
              <el-table-column prop="status" :label="t.common.status" width="100">
                <template #default="scope">
                  <el-tag :type="scope.row.status === 'expired' ? 'danger' : 'success'" size="small">
                    {{ scope.row.status === 'expired' ? t.settings.expired : t.settings.activeStatus }}
                  </el-tag>
                </template>
              </el-table-column>
              <el-table-column :label="t.settings.expiration" width="160">
                <template #default="scope">
                  <span :class="{ 'text-danger': scope.row.status === 'expired' }">
                    {{ formatTime(scope.row.expires_at) }}
                  </span>
                </template>
              </el-table-column>
              <el-table-column :label="t.settings.created" width="160">
                <template #default="scope">
                  <span class="text-muted">
                    {{ new Date(scope.row.created_at * 1000).toLocaleDateString() }}
                  </span>
                </template>
              </el-table-column>
              <el-table-column :label="t.common.actions" width="80" align="right">
                <template #default="scope">
                  <el-button link type="danger" :icon="Delete" @click="deleteKey(scope.row.key)" />
                </template>
              </el-table-column>
            </el-table>
          </div>

          <el-empty v-if="keys.length === 0" :description="t.settings.noCredentials" />
        </div>
      </el-tab-pane>

      <!-- 3. Channels Tab (Combined Webhook & SMTP) -->
      <el-tab-pane name="channels">
        <template #label>
          <span class="tab-label"><el-icon><Share /></el-icon>{{ t.settings.channels }}</span>
        </template>
        
        <div class="tab-content two-cols">
          <!-- Webhook Card -->
          <div class="settings-section glass-card">
            <div class="section-header">
              <el-icon><Share /></el-icon>
              <h4>{{ t.settings.webhooks }}</h4>
            </div>
            <el-form label-position="left" label-width="140px">
              <el-form-item :label="t.settings.webhookUrl">
                <el-input v-model="webhookForm.url" placeholder="https://api.example.com/webhook" />
              </el-form-item>
              <el-form-item :label="t.settings.events">
                <el-checkbox-group v-model="webhookForm.events">
                  <el-checkbox label="register">{{ t.settings.eventRegister }}</el-checkbox>
                  <el-checkbox label="deregister">{{ t.settings.eventDeregister }}</el-checkbox>
                  <el-checkbox label="health_change">{{ t.settings.eventHealth }}</el-checkbox>
                </el-checkbox-group>
              </el-form-item>
              <div style="margin-top: auto; padding-top: 16px;">
                <el-button type="primary" @click="handleSaveGeneral" style="width: 100%">{{ t.settings.saveSettings }}</el-button>
              </div>
            </el-form>
          </div>

          <!-- SMTP Card -->
          <div class="settings-section glass-card">
            <div class="section-header">
              <el-icon><Message /></el-icon>
              <h4>{{ t.settings.smtpSettings }}</h4>
            </div>
            <el-form label-position="left" label-width="140px">
              <el-form-item :label="t.settings.smtpHost">
                <div class="form-row">
                  <el-input v-model="smtpForm.host" placeholder="smtp.example.com" style="flex: 3" />
                  <el-input-number v-model="smtpForm.port" :controls="false" placeholder="465" style="flex: 1" />
                </div>
              </el-form-item>
              <el-form-item :label="t.settings.smtpUser">
                <el-input v-model="smtpForm.user" placeholder="user@example.com" />
              </el-form-item>
              <el-form-item :label="t.settings.smtpPass">
                <el-input v-model="smtpForm.pass" type="password" show-password />
              </el-form-item>
              <el-form-item :label="t.settings.smtpFrom">
                <el-input v-model="smtpForm.from" placeholder="Eden Registry <noreply@example.com>" />
              </el-form-item>
              <div style="margin-top: auto; padding-top: 16px;">
                <el-button type="primary" @click="handleSaveGeneral" style="width: 100%">{{ t.settings.saveSettings }}</el-button>
              </div>
            </el-form>
          </div>
        </div>
      </el-tab-pane>

      <!-- 4. Monitoring Tab -->
      <el-tab-pane name="monitoring">
        <template #label>
          <span class="tab-label"><el-icon><Monitor /></el-icon>{{ t.settings.monitoring }}</span>
        </template>
        
        <div class="tab-content">
          <div class="settings-section glass-card">
            <div class="section-header">
              <el-icon><Bell /></el-icon>
              <h4>{{ t.settings.alarms }}</h4>
              <el-switch v-model="alarmForm.enabled" />
            </div>
            <el-form label-position="left" label-width="160px" :disabled="!alarmForm.enabled">
              <el-form-item :label="t.settings.threshold">
                <div class="val-control">
                  <el-slider v-model="alarmForm.threshold" :min="10" :max="99" style="flex: 1" />
                  <span class="val-text">{{ alarmForm.threshold }}%</span>
                </div>
              </el-form-item>
              <el-form-item :label="t.settings.notificationChannel">
                <el-radio-group v-model="alarmForm.contact" class="channel-radio">
                  <el-radio label="email" border>{{ t.settings.email }}</el-radio>
                  <el-radio label="webhook" border>{{ t.settings.webhook }}</el-radio>
                </el-radio-group>
              </el-form-item>
              <div style="margin-top: 16px;">
                <el-button type="primary" @click="handleSaveGeneral" style="width: 100%">{{ t.settings.saveSettings }}</el-button>
              </div>
            </el-form>
          </div>
        </div>
      </el-tab-pane>
    </el-tabs>

    <!-- Premium Square Dialog -->
    <el-dialog 
      v-model="showDialog" 
      :show-close="false"
      width="560px" 
      class="premium-dialog"
    >
      <template #header>
        <div class="dialog-header">
          <div class="header-title">
            <el-icon><Key /></el-icon>
            <span>{{ t.settings.generateKey }}</span>
          </div>
          <el-button link @click="showDialog = false" class="close-btn">
            <el-icon><Delete /></el-icon>
          </el-button>
        </div>
      </template>

      <div class="dialog-body">
        <el-form label-position="left" label-width="120px" class="premium-form">
          <el-form-item :label="t.settings.apiKey">
            <div class="key-input-container">
              <el-input v-model="keyForm.key" readonly class="glass-input" />
              <el-button @click="generateRandKey" class="regen-btn" :icon="RefreshRight" />
            </div>
          </el-form-item>

          <el-form-item :label="t.settings.description">
            <el-input 
              v-model="keyForm.description" 
              type="textarea" 
              :rows="3" 
              :placeholder="t.common.none"
              class="glass-input" 
            />
          </el-form-item>

          <el-form-item :label="t.settings.expiration" style="margin-bottom: 0;">
            <el-select v-model="keyForm.expDays" class="glass-input full-width">
              <el-option :label="t.settings.neverExpire" :value="0" />
              <el-option :label="t.settings.expireIn.replace('{n}', '7')" :value="7" />
              <el-option :label="t.settings.expireIn.replace('{n}', '30')" :value="30" />
              <el-option :label="t.settings.expireIn.replace('{n}', '90')" :value="90" />
              <el-option :label="t.settings.expireIn.replace('{n}', '365')" :value="365" />
            </el-select>
          </el-form-item>
        </el-form>
      </div>

      <template #footer>
        <div class="dialog-footer">
          <el-button @click="showDialog = false" class="square-btn">{{ t.common.cancel }}</el-button>
          <el-button type="primary" @click="handleGenerate" class="square-btn highlight">{{ t.common.confirm }}</el-button>
        </div>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
.settings-container {
  width: 100%;
}

/* Structural elements use clean corners */
.glass-card,
.settings-section,
.key-card,
.key-list {
  border-radius: 8px;
}

.settings-tabs :deep(.el-tabs__header) {
  margin-bottom: 32px;
}

.settings-tabs :deep(.el-tabs__nav-wrap::after) {
  display: none;
}

.tab-label {
  display: flex;
  align-items: center;
  gap: 8px;
  font-weight: 500;
  font-size: 15px;
}

.tab-content {
  animation: fadeIn 0.3s ease;
}

.tab-content.grid-layout {
  display: grid;
  grid-template-columns: 1.25fr 0.75fr;
  gap: 24px;
}
@media (max-width: 1200px) {
  .tab-content.grid-layout {
    grid-template-columns: 1fr;
  }
}

.tab-content.two-cols {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 24px;
}

/* Sections */
.settings-section {
  padding: 32px;
  height: 100%;
  display: flex;
  flex-direction: column;
}

.section-header {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 28px;
}

.section-header h4 {
  margin: 0;
  font-size: 16px;
  font-weight: 700;
  flex: 1;
}

.section-header .el-icon {
  font-size: 20px;
  color: var(--accent-blue);
}

.credentials-header {
  margin-bottom: 32px;
}

.title-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 12px;
}

.tab-content-title {
  margin: 0;
  font-size: 20px;
  font-weight: 700;
}

.content-desc {
  color: var(--text-secondary);
  font-size: 14px;
  margin-bottom: 24px;
  line-height: 1.6;
}

.action-row {
  padding: 24px;
  background: var(--bg-card);
  border: 1px dashed var(--border-color);
  display: flex;
  align-items: center;
}

/* V7 Operation Mode Styles */
.consistency-wrapper-v7 {
  margin-top: 8px;
}

.main-mode-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 16px;
}

.mode-card-v7 {
  position: relative;
  min-height: 160px;
  background: var(--bg-card);
  border: 1px solid var(--border-color);
  border-radius: 12px;
  cursor: pointer;
  transition: all 0.4s cubic-bezier(0.16, 1, 0.3, 1);
  overflow: hidden;
  display: flex;
}

.mode-card-v7:hover {
  border-color: var(--accent-blue);
  box-shadow: 0 12px 30px rgba(0,0,0,0.08);
  transform: translateY(-2px);
}

.mode-card-v7.active {
  border-color: var(--accent-blue);
  background: rgba(59, 130, 246, 0.02);
  box-shadow: 0 8px 24px rgba(59, 130, 246, 0.08);
}

.card-glow {
  position: absolute;
  top: 0; left: 0; width: 100%; height: 100%;
  background: radial-gradient(circle at top right, rgba(59, 130, 246, 0.05), transparent 70%);
  opacity: 0;
  transition: opacity 0.4s;
}

.mode-card-v7.active .card-glow {
  opacity: 1;
}

.card-inner {
  padding: 24px;
  display: flex;
  gap: 20px;
  width: 100%;
  z-index: 1;
}

.mode-icon-v7 {
  width: 56px;
  height: 56px;
  background: rgba(0,0,0,0.03);
  border-radius: 14px;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 26px;
  color: var(--text-muted);
  transition: all 0.3s;
  flex-shrink: 0;
}

.mode-card-v7.active .mode-icon-v7 {
  background: var(--accent-blue);
  color: white;
}

.mode-card-v7.cluster.active .mode-icon-v7 {
  background: var(--accent-blue);
}

.mode-content-v7 {
  flex: 1;
  display: flex;
  flex-direction: column;
}

.mode-title-v7 {
  font-size: 17px;
  font-weight: 800;
  margin-bottom: 8px;
  color: var(--text-color);
}

.mode-desc-v7 {
  font-size: 13px;
  color: var(--text-muted);
  line-height: 1.5;
}

/* Integrated Toggle Style */
.integrated-toggle-v7 {
  display: flex;
  background: var(--bg-card-alt, rgba(0,0,0,0.04));
  padding: 4px;
  border-radius: 10px;
  margin-top: 12px;
  gap: 4px;
  width: fit-content;
  flex-shrink: 0; /* Prevent shrinking */
}

.toggle-option {
  min-width: 100px;
  position: relative;
  padding: 8px 16px;
  cursor: pointer;
  border-radius: 8px;
  transition: all 0.3s;
  text-align: center;
  overflow: visible;
}

.toggle-text {
  position: relative;
  z-index: 1;
  display: flex;
  flex-direction: column;
  white-space: nowrap;
  align-items: center;
}

.toggle-text .primary {
  font-size: 13px;
  font-weight: 800;
  color: var(--text-muted);
}

.toggle-text .secondary {
  font-size: 10px;
  font-weight: 500;
  opacity: 0.7;
  color: var(--text-muted);
}

.toggle-option.selected .primary { color: white; }
.toggle-option.selected .secondary { color: rgba(255,255,255,0.85); }

.integrated-toggle-v7.cp .toggle-bg {
  background: #f97316; /* Orange */
}

.integrated-toggle-v7.ap .toggle-bg {
  background: #10b981; /* Emerald/Green */
}

.tag-cp {
  background-color: #f97316 !important;
  border-color: #f97316 !important;
  color: white !important;
}

.tag-ap {
  background-color: #10b981 !important;
  border-color: #10b981 !important;
  color: white !important;
}

.toggle-bg {
  position: absolute;
  top: 0; left: 0; width: 100%; height: 100%;
  border-radius: 8px;
  opacity: 0;
  transform: scale(0.9);
  transition: all 0.3s cubic-bezier(0.16, 1, 0.3, 1);
}

.toggle-option.selected .toggle-bg {
  transform: scale(1);
  opacity: 1;
}

/* Status Indicators */
.status-tags {
  display: flex;
  gap: 8px;
  align-items: center;
}

.status-indicator {
  display: flex;
  align-items: center;
  gap: 6px;
  background: rgba(0,0,0,0.03);
  padding: 2px 10px;
  border-radius: 6px;
  white-space: nowrap;
}

.indicator-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: #94a3b8;
}

.indicator-dot.active {
  background: #10b981;
  box-shadow: 0 0 8px rgba(16, 185, 129, 0.6);
  animation: pulse-green 2s infinite;
}

@keyframes pulse-green {
  0% { box-shadow: 0 0 0 0 rgba(16, 185, 129, 0.7); }
  70% { box-shadow: 0 0 0 8px rgba(16, 185, 129, 0); }
  100% { box-shadow: 0 0 0 0 rgba(16, 185, 129, 0); }
}

.dot-label {
  font-size: 12px;
  font-weight: 600;
  color: var(--text-muted);
}

/* Technical Footer */
.technical-footer-v7 {
  margin-top: 24px;
}

.info-bubble-v7 {
  padding: 16px 20px;
  border-radius: 12px;
  display: flex;
  gap: 16px;
  align-items: flex-start;
  background: rgba(0,0,0,0.02);
  border: 1px solid var(--border-color);
  transition: all 0.3s;
}

.info-bubble-v7.cp {
  background: rgba(249, 115, 22, 0.05); /* Orange bg */
  border-color: rgba(249, 115, 22, 0.2);
}

.info-bubble-v7.cp .el-icon {
  color: #f97316;
}

.info-bubble-v7.ap {
  background: rgba(16, 185, 129, 0.05);
  border-color: rgba(16, 185, 129, 0.2);
}

.info-bubble-v7.ap .el-icon {
  color: #10b981;
}

.info-bubble-v7 .el-icon {
  font-size: 18px;
  margin-top: 2px;
  color: var(--text-muted);
}

.bubble-content {
  font-size: 13px;
  line-height: 1.6;
  color: var(--text-secondary);
}

.info-bubble-v7.ap { border-left: 4px solid var(--accent-green); background: rgba(16, 185, 129, 0.03); }
.info-bubble-v7.cp { border-left: 4px solid var(--accent-blue); background: rgba(59, 130, 246, 0.03); }
.info-bubble-v7.standalone { border-left: 4px solid #94a3b8; }

.val-control {
  display: flex;
  align-items: center;
  gap: 16px;
}

.val-text {
  font-size: 14px;
  font-weight: 700;
  color: var(--accent-blue);
  min-width: 45px;
  text-align: right;
}

.form-row {
  display: flex;
  gap: 16px;
}

.channel-radio {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 12px;
}

/* Key Cards */
.key-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(320px, 1fr));
  gap: 20px;
}

.key-card {
  padding: 24px;
  transition: all 0.3s ease;
  border: 1px solid var(--border-color);
  position: relative;
}

.key-card.is-expired {
  border-color: var(--el-color-danger-light-3);
  background: rgba(245, 108, 108, 0.02);
}

.key-card:hover {
  border-color: var(--accent-blue);
}

.key-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 16px;
}

.key-code {
  font-family: 'Fira Code', monospace;
  font-size: 13px;
  color: var(--accent-blue);
  background: rgba(59, 130, 246, 0.1);
  padding: 6px 12px;
}

.key-code-list {
  font-family: 'Fira Code', monospace;
  font-size: 12px;
  color: var(--accent-blue);
}

.key-with-copy {
  display: flex;
  align-items: center;
  gap: 8px;
}

.key-body .key-desc {
  font-size: 15px;
  font-weight: 600;
  margin-bottom: 12px;
}

.key-info {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.key-date {
  font-size: 12px;
  color: var(--text-muted);
  display: flex;
  align-items: center;
  gap: 6px;
}

.text-danger {
  color: var(--el-color-danger);
}

.status-tag {
  margin-right: 8px;
}

.key-list {
  border: 1px solid var(--border-color);
}

.custom-table :deep(.el-table__inner-wrapper::before) {
  display: none;
}

.glass-card {
  backdrop-filter: blur(20px);
  background: var(--bg-card);
  border: 1px solid var(--border-color);
}

.key-regen-input {
  display: flex;
  gap: 12px;
  width: 100%;
}

/* Premium Dialog Styling */
:deep(.premium-dialog) {
  background: var(--bg-card) !important;
  border: 1px solid var(--border-color) !important;
  box-shadow: 0 40px 100px rgba(0, 0, 0, 0.4) !important;
  padding: 0;
  border-radius: 0 !important;
}

:deep(.premium-dialog .el-dialog__header) {
  display: none;
}

.dialog-header {
  padding: 32px 40px 24px;
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.header-title {
  display: flex;
  align-items: center;
  gap: 16px;
  font-size: 20px;
  font-weight: 800;
  color: var(--text-color);
}

.header-title .el-icon {
  color: var(--accent-blue);
  font-size: 24px;
}

.close-btn {
  font-size: 20px;
  color: var(--text-muted);
  opacity: 0.6;
}

.close-btn:hover {
  opacity: 1;
  color: var(--el-color-danger);
}

.dialog-body {
  padding: 0 40px 32px;
}

.premium-form :deep(.el-form-item) {
  margin-bottom: 28px;
}

.premium-form :deep(.el-form-item__label) {
  font-weight: 700;
  font-size: 14px;
  color: var(--text-color);
  display: flex;
  align-items: center;
  height: 44px;
}

.glass-input :deep(.el-input__wrapper),
.glass-input :deep(.el-textarea__inner),
.glass-input :deep(.el-select__wrapper) {
  background: var(--bg-card-alt, rgba(0, 0, 0, 0.03)) !important;
  border: 1px solid var(--border-color) !important;
  box-shadow: none !important;
  height: 44px;
  padding: 4px 16px;
}

.glass-input :deep(.el-textarea__inner) {
  height: auto;
  min-height: 100px !important;
}

.regen-btn {
  height: 44px;
  width: 44px;
  border: 1px solid var(--border-color);
  background: transparent;
}

.dialog-footer {
  padding: 24px 40px 32px;
  display: flex;
  gap: 16px;
  justify-content: flex-end;
  border-top: 1px solid var(--border-color);
  background: rgba(0, 0, 0, 0.01);
}

.square-btn {
  height: 44px;
  padding: 0 32px;
  font-weight: 600;
  font-size: 14px;
}

.square-btn.highlight {
  box-shadow: 0 8px 20px rgba(59, 130, 246, 0.25);
}

@keyframes fadeIn {
  from { opacity: 0; transform: translateY(12px); }
  to { opacity: 1; transform: translateY(0); }
}

/* Responsive Overrides */
@media (max-width: 1024px) {
  .tab-content.grid-layout,
  .tab-content.two-cols {
    grid-template-columns: 1fr;
  }
}
</style>
