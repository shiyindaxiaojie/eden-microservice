<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { getServices, type ServiceSummary } from '../api/registry'

const router = useRouter()
const services = ref<ServiceSummary[]>([])
const search = ref('')
const loading = ref(true)

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

onMounted(fetchServices)
</script>

<template>
  <div>
    <!-- Search -->
    <div class="search-bar">
      <el-input
        v-model="search"
        placeholder="搜索服务名称..."
        size="large"
        clearable
        prefix-icon="Search"
      />
    </div>

    <!-- Empty -->
    <div v-if="!loading && filtered.length === 0" class="empty-state">
      <el-icon><Grid /></el-icon>
      <p>暂无服务注册</p>
    </div>

    <!-- Service Cards -->
    <div class="service-grid">
      <div
        v-for="(svc, idx) in filtered"
        :key="svc.name"
        class="service-card"
        :style="{ animationDelay: `${idx * 0.05}s` }"
        @click="goDetail(svc.name)"
      >
        <div class="svc-name">{{ svc.name }}</div>
        <div class="svc-stats">
          <span><el-icon><Monitor /></el-icon> {{ svc.instance_count }} 实例</span>
          <span><el-icon><CircleCheck /></el-icon> {{ svc.healthy_count }} 健康</span>
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
  </div>
</template>
