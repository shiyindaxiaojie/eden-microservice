<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
import * as echarts from 'echarts'
import { getDependencyGraph, getNamespaces, type Namespace } from '../api/registry'
import { useI18n } from '../utils/i18n'
import { ElMessage } from 'element-plus'
import { Refresh } from '@element-plus/icons-vue'

const { t } = useI18n()
const chartRef = ref<HTMLElement | null>(null)
let chartInstance: echarts.ECharts | null = null

const loading = ref(false)
const currentNamespace = ref('default')
const namespaces = ref<Namespace[]>([])

const fetchNamespaces = async () => {
  try {
    const res = await getNamespaces()
    namespaces.value = res.data || []
  } catch (e) {
    console.error('Failed to fetch namespaces', e)
  }
}

const renderGraph = (graphData: any) => {
  if (!chartInstance) {
    chartInstance = echarts.init(chartRef.value!)
  }
  
  const nodes = graphData.nodes.map((n: any) => ({
    name: n.id,
    value: n.name || n.id,
    symbolSize: 50,
    itemStyle: {
      color: 'var(--accent-blue)',
      borderColor: 'var(--border-color)',
      borderWidth: 2
    },
    label: {
      show: true,
      position: 'bottom',
      color: 'var(--text-primary)'
    }
  }))
  
  const links = graphData.edges.map((e: any) => ({
    source: e.source,
    target: e.target,
    lineStyle: {
      color: '#A3B8CC',
      curveness: 0.2,
      width: 2
    }
  }))

  const option = {
    tooltip: {},
    animationDurationUpdate: 1500,
    series: [
      {
        type: 'graph',
        layout: 'force',
        symbolSize: 50,
        roam: true,
        label: {
          show: true
        },
        edgeSymbol: ['circle', 'arrow'],
        edgeSymbolSize: [4, 10],
        edgeLabel: {
          fontSize: 12
        },
        data: nodes,
        links: links,
        lineStyle: {
          opacity: 0.9,
          width: 2,
          curveness: 0.1
        },
        force: {
          repulsion: 1000,
          edgeLength: [100, 300]
        }
      }
    ]
  }

  chartInstance.setOption(option)
}

const fetchGraph = async () => {
  loading.value = true
  try {
    const res = await getDependencyGraph(currentNamespace.value)
    if (res.data) {
      renderGraph(res.data)
    }
  } catch (err: any) {
    ElMessage.error(err.response?.data?.error || err.message)
  } finally {
    loading.value = false
  }
}

const handleNamespaceChange = () => {
  fetchGraph()
}

onMounted(async () => {
  await fetchNamespaces()
  fetchGraph()
  
  window.addEventListener('resize', () => {
    chartInstance?.resize()
  })
})

onUnmounted(() => {
  chartInstance?.dispose()
})
</script>

<template>
  <div class="dependency-graph-page">
    <div class="panel-header">
      <div class="header-content">
        <h2>{{ t.nav.dependencies || 'Dependency Graph' }}</h2>
      </div>
      <div class="header-actions" style="display: flex; gap: 16px; align-items: center;">
        <span style="font-size: 14px; color: var(--text-muted);">Namespace:</span>
        <el-select 
          v-model="currentNamespace" 
          @change="handleNamespaceChange"
          style="width: 200px;"
        >
          <el-option value="default" label="default" />
          <el-option 
            v-for="ns in namespaces" 
            :key="ns.name" 
            :value="ns.name" 
            :label="ns.name"
            v-show="ns.name !== 'default'"
          />
        </el-select>
        <el-button @click="fetchGraph" type="primary" plain :icon="Refresh">Refresh</el-button>
      </div>
    </div>
    
    <div class="panel-content" v-loading="loading" style="height: calc(100vh - 200px); min-height: 500px;">
      <div ref="chartRef" style="width: 100%; height: 100%;"></div>
    </div>
  </div>
</template>

<style scoped>
.dependency-graph-page {
  animation: fadeIn 0.3s ease-in-out;
}
</style>
