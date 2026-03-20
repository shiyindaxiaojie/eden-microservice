<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import * as echarts from 'echarts'
import type { TopologyGraph, TopologyNode } from '../api/registry'

const props = defineProps<{
  graph: TopologyGraph
  loading?: boolean
  selectedNode?: string
}>()

const emit = defineEmits<{
  (e: 'select', nodeId: string): void
}>()

const chartEl = ref<HTMLElement | null>(null)
let chart: echarts.ECharts | null = null

const graphData = computed(() => props.graph || { namespace: 'default', nodes: [], edges: [] })

function nodeTone(node: TopologyNode) {
  if (node.instance_count === 0) {
    return {
      halo: 'rgba(148, 163, 184, 0.18)',
      border: '#94a3b8',
      fill: '#ffffff',
      accent: '#64748b',
    }
  }
  if (node.healthy_count === node.instance_count) {
    return {
      halo: 'rgba(37, 99, 235, 0.16)',
      border: '#22c55e',
      fill: '#ffffff',
      accent: '#2563eb',
    }
  }
  if (node.healthy_count > 0) {
    return {
      halo: 'rgba(245, 158, 11, 0.18)',
      border: '#f59e0b',
      fill: '#ffffff',
      accent: '#d97706',
    }
  }
  return {
    halo: 'rgba(239, 68, 68, 0.18)',
    border: '#ef4444',
    fill: '#ffffff',
    accent: '#dc2626',
  }
}

function buildOption() {
  const nodes = graphData.value.nodes.map((node) => {
    const tone = nodeTone(node)
    const preview = node.instances
      .slice(0, 2)
      .map((instance) => `${instance.host}:${instance.port} ${instance.status === 'passing' ? 'UP' : 'DOWN'}`)
      .join('\n')

    return {
      id: node.id,
      name: node.name,
      value: node.name,
      symbolSize: node.instance_count > 0 ? 94 : 82,
      draggable: true,
      itemStyle: {
        color: tone.fill,
        borderColor: tone.border,
        borderWidth: props.selectedNode === node.id ? 3 : 2,
        shadowColor: tone.halo,
        shadowBlur: props.selectedNode === node.id ? 28 : 18,
      },
      label: {
        show: true,
        formatter: `{title|${node.name}}\n{meta|${node.healthy_count}/${node.instance_count} healthy}${preview ? `\n{inst|${preview}}` : ''}`,
        rich: {
          title: {
            fontSize: 15,
            fontWeight: 700,
            color: '#0f172a',
            lineHeight: 22,
          },
          meta: {
            fontSize: 11,
            color: tone.accent,
            lineHeight: 18,
          },
          inst: {
            fontSize: 10,
            color: '#64748b',
            lineHeight: 16,
          },
        },
      },
    }
  })

  const links = graphData.value.edges.map((edge) => ({
    source: edge.source,
    target: edge.target,
    lineStyle: {
      color: 'rgba(37, 99, 235, 0.38)',
      curveness: 0.16,
      width: 1.6,
    },
    emphasis: {
      lineStyle: {
        width: 2.4,
        color: 'rgba(37, 99, 235, 0.7)',
      },
    },
  }))

  return {
    tooltip: {
      trigger: 'item',
      backgroundColor: 'rgba(15, 23, 42, 0.92)',
      borderWidth: 0,
      textStyle: { color: '#e2e8f0' },
      formatter(params: any) {
        if (params.dataType === 'edge') {
          return `${params.data.source} -> ${params.data.target}`
        }
        const node = graphData.value.nodes.find((item) => item.id === params.data.id)
        if (!node) return params.data.name
        const instances = node.instances.length
          ? node.instances
              .map((instance) => `${instance.host}:${instance.port} · ${instance.status === 'passing' ? 'healthy' : 'critical'}`)
              .join('<br/>')
          : 'No live instances'
        return `<strong>${node.name}</strong><br/>${node.healthy_count}/${node.instance_count} healthy<br/><br/>${instances}`
      },
    },
    animationDuration: 400,
    animationDurationUpdate: 400,
    series: [
      {
        type: 'graph',
        layout: 'force',
        data: nodes,
        links,
        roam: true,
        zoom: 1,
        label: {
          position: 'inside',
        },
        edgeSymbol: ['none', 'arrow'],
        edgeSymbolSize: [0, 8],
        force: {
          repulsion: 860,
          edgeLength: [120, 220],
          gravity: 0.06,
        },
      },
    ],
  }
}

function ensureChart() {
  if (!chart && chartEl.value) {
    chart = echarts.init(chartEl.value)
    chart.on('click', (params: any) => {
      if (params.dataType === 'node' && params.data?.id) {
        emit('select', params.data.id)
      }
    })
  }
}

async function render() {
  await nextTick()
  ensureChart()
  chart?.setOption(buildOption(), true)
}

function resize() {
  chart?.resize()
}

function zoom(delta: number) {
  if (!chart) return
  const option = chart.getOption() as any
  const currentZoom = option.series?.[0]?.zoom || 1
  chart.setOption({
    series: [{ zoom: Math.max(0.45, Math.min(2.4, currentZoom + delta)) }],
  })
}

watch(graphData, render, { deep: true })
watch(() => props.selectedNode, render)

onMounted(async () => {
  await render()
  window.addEventListener('resize', resize)
})

onBeforeUnmount(() => {
  window.removeEventListener('resize', resize)
  chart?.dispose()
  chart = null
})
</script>

<template>
  <div class="topology-board">
    <div class="board-toolbar">
      <button type="button" @click="zoom(-0.15)">-</button>
      <button type="button" @click="zoom(0.15)">+</button>
    </div>

    <div v-if="!loading && graphData.nodes.length === 0" class="board-empty">
      <el-empty description="当前命名空间暂无拓扑关系" :image-size="84" />
    </div>

    <div ref="chartEl" class="graph-canvas" :class="{ hidden: !loading && graphData.nodes.length === 0 }"></div>
  </div>
</template>

<style scoped>
.topology-board {
  position: relative;
  min-height: 640px;
  border-radius: 26px;
  background:
    radial-gradient(circle at 1px 1px, rgba(37, 99, 235, 0.12) 1px, transparent 0),
    linear-gradient(180deg, rgba(255, 255, 255, 0.98), rgba(244, 247, 252, 0.98));
  background-size: 18px 18px, auto;
  border: 1px solid rgba(148, 163, 184, 0.24);
  overflow: hidden;
}

.board-toolbar {
  position: absolute;
  top: 22px;
  left: 22px;
  z-index: 2;
  display: inline-flex;
  border-radius: 14px;
  overflow: hidden;
  box-shadow: 0 18px 46px rgba(15, 23, 42, 0.08);
}

.board-toolbar button {
  width: 44px;
  height: 44px;
  border: 0;
  background: rgba(255, 255, 255, 0.95);
  color: #2563eb;
  font-size: 24px;
  cursor: pointer;
}

.board-toolbar button + button {
  border-left: 1px solid rgba(148, 163, 184, 0.2);
}

.graph-canvas {
  width: 100%;
  height: 640px;
}

.graph-canvas.hidden {
  display: none;
}

.board-empty {
  display: grid;
  min-height: 640px;
  place-items: center;
}

@media (max-width: 920px) {
  .topology-board,
  .graph-canvas,
  .board-empty {
    min-height: 520px;
    height: 520px;
  }
}
</style>
