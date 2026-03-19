<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { getServices, getSubscribers, type ServiceSummary } from '../api/registry'
import { useI18n } from '../utils/i18n'
import { Grid, Menu, Monitor, CircleCheck, User, Search } from '@element-plus/icons-vue'

const { t } = useI18n()
const router = useRouter()
const services = ref<ServiceSummary[]>([])
const search = ref('')
const loading = ref(true)
const viewMode = ref<'grid' | 'list'>('grid')

// Subscribers dialog
const subDialogVisible = ref(false)
const subLoading = ref(false)
const currentService = ref('')
const subscribersList = ref<string[]>([])

const filtered = computed(() => {
  if (!search.value) return services.value
  const q = search.value.toLowerCase()
  return services.value.filter(s => s.name.toLowerCase().includes(q))
})

async function fetchServices() {
  try {
    const res = await getServices()
    services.value = res.data || []
  } catch (e) {
    console.error('fetch services failed', e)
  } finally {
    loading.value = false
  }
}

function healthRate(s: ServiceSummary) {
  if (s.instance_count === 0) return 0
  return (s.healthy_count / s.instance_count) * 100
}

function barColor(rate: number) {
  if (rate >= 80) return 'var(--accent-green)'
  if (rate >= 50) return 'var(--accent-orange)'
  return 'var(--accent-red)'
}

function goDetail(name: string) {
  router.push(`/services/${encodeURIComponent(name)}`)
}

async function showSubscribers(name: string) {
  currentService.value = name
  subscribersList.value = []
  subDialogVisible.value = true
  subLoading.value = true
  try {
    const res = await getSubscribers(name)
    subscribersList.value = res.data || []
  } catch (e: any) {
    console.error('Failed to fetch subscribers', e)
  } finally {
    subLoading.value = false
  }
}

onMounted(fetchServices)
</script>

<template>
  <div>
    <!-- Search & Toggle -->
    <div class="search-bar" style="display: flex; gap: 16px; margin-bottom: 24px;">
      <el-input
        v-model="search"
        :placeholder="t.services.searchPlaceholder"
        size="large"
        clearable
        :prefix-icon="Search"
        style="flex: 1;"
      />
      <el-radio-group v-model="viewMode" size="large">
        <el-radio-button value="grid"><el-icon><Menu /></el-icon></el-radio-button>
        <el-radio-button value="list"><el-icon><Grid /></el-icon></el-radio-button>
      </el-radio-group>
    </div>

    <!-- Empty -->
    <div v-if="!loading && filtered.length === 0" class="empty-state">
      <el-icon><Grid /></el-icon>
      <p>{{ t.services.noServices }}</p>
    </div>

    <!-- Grid View -->
    <div v-if="viewMode === 'grid'" class="service-grid">
      <div
        v-for="(svc, idx) in filtered"
        :key="svc.name"
        class="service-card"
        :style="{ animationDelay: `${idx * 0.05}s` }"
      >
        <div class="svc-name" @click="goDetail(svc.name)" style="cursor: pointer;">{{ svc.name }}</div>
        <div class="svc-stats">
          <span><el-icon><Monitor /></el-icon> {{ svc.instance_count }} {{ t.services.instances }}</span>
          <span><el-icon><CircleCheck /></el-icon> {{ svc.healthy_count }} {{ t.services.healthy }}</span>
          <span class="sub-link" @click.stop="showSubscribers(svc.name)">
            <el-icon><User /></el-icon> {{ svc.subscriber_count || 0 }} {{ t.services.subscribers || 'Subscribers' }}
          </span>
        </div>
        <div class="health-bar">
          <div
            class="fill"
            :style="{
              width: healthRate(svc) + '%',
              background: barColor(healthRate(svc))
            }"
          ></div>
        </div>
      </div>
    </div>

    <!-- List View -->
    <div v-else class="glass-table-wrapper">
      <el-table :data="filtered" style="width: 100%" v-loading="loading">
        <el-table-column prop="name" :label="t.services.service" min-width="200">
          <template #default="{ row }">
            <span class="view-link" @click="goDetail(row.name)" style="color: var(--accent-blue); cursor: pointer; font-weight: 500;">
              {{ row.name }}
            </span>
          </template>
        </el-table-column>
        <el-table-column prop="instance_count" :label="t.services.instances" min-width="120" />
        <el-table-column prop="healthy_count" :label="t.services.healthy" min-width="120">
          <template #default="{ row }">
            <span :style="{ color: barColor(healthRate(row)) }">{{ row.healthy_count }}</span>
          </template>
        </el-table-column>
        <el-table-column prop="subscriber_count" :label="t.services.subscribers || 'Subscribers'" min-width="120">
          <template #default="{ row }">
            <el-button link type="primary" @click="showSubscribers(row.name)">
              {{ row.subscriber_count || 0 }}
            </el-button>
          </template>
        </el-table-column>
        <el-table-column label="Health Rate" min-width="150">
          <template #default="{ row }">
            <el-progress :percentage="Number(healthRate(row).toFixed(1))" :color="barColor(healthRate(row))" />
          </template>
        </el-table-column>
      </el-table>
    </div>

    <!-- Subscribers Dialog -->
    <el-dialog
      v-model="subDialogVisible"
      :title="`${currentService} Subscribers`"
      width="500px"
      append-to-body
      class="glass-dialog"
    >
      <div v-loading="subLoading" style="min-height: 100px;">
        <div v-if="subscribersList.length === 0 && !subLoading" style="text-align: center; color: var(--text-muted); margin-top: 40px;">
          No subscribers found
        </div>
        <div v-else class="subscribers-list">
          <div v-for="sub in subscribersList" :key="sub" class="subscriber-item" style="padding: 10px; border-bottom: 1px solid var(--border-color);">
            <el-icon><User /></el-icon> <span style="margin-left: 8px;">{{ sub }}</span>
          </div>
        </div>
      </div>
    </el-dialog>
  </div>
</template>

<style scoped>
.sub-link {
  cursor: pointer;
  transition: opacity 0.2s;
}
.sub-link:hover {
  opacity: 0.8;
  color: var(--accent-blue) !important;
}
.view-link:hover {
  text-decoration: underline;
}
</style>
