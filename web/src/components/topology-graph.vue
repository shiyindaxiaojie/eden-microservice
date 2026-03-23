<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import * as echarts from 'echarts'
import type { TopologyGraph, TopologyNode } from '../api/registry'

const currentZoom = ref(1)

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
      border: '#475569',
      fill: 'rgba(71, 85, 105, 0.06)',
      accent: '#64748b',
      text: '#94a3b8',
    }
  }
  if (node.healthy_count === node.instance_count) {
    return {
      border: '#22c55e',
      fill: 'rgba(34, 197, 94, 0.06)',
      accent: '#10b981',
      text: '#0f172a',
    }
  }
  if (node.healthy_count > 0) {
    return {
      border: '#f59e0b',
      fill: 'rgba(245, 158, 11, 0.06)',
      accent: '#d97706',
      text: '#0f172a',
    }
  }
  return {
    border: '#ef4444',
    fill: 'rgba(239, 68, 68, 0.06)',
    accent: '#dc2626',
    text: '#0f172a',
  }
}

function buildOption() {
  const nodes = graphData.value.nodes.map((node) => {
    const tone = nodeTone(node)
    const isSelected = props.selectedNode === node.id
    const maxShow = 3
    const shownInstances = node.instances.slice(0, maxShow)
    const extraCount = node.instances.length - maxShow
    const moreLabel = extraCount > 0 ? `\n{more|+${extraCount} more}` : ''

    // Dynamic height: base 44 + 15 per IP line + 15 for "+N more"
    const instLines = Math.min(node.instances.length, maxShow)
    const nodeHeight = node.instance_count > 0
      ? 44 + instLines * 15 + (extraCount > 0 ? 15 : 0)
      : 50

    return {
      id: node.id,
      name: node.name,
      value: node.name,
      symbol: 'rect',
      symbolSize: [180, nodeHeight],
      draggable: false,
      itemStyle: {
        color: isSelected ? 'rgba(59, 130, 246, 0.08)' : '#ffffff',
        borderColor: isSelected ? '#3b82f6' : tone.border,
        borderWidth: isSelected ? 2 : 1,
        shadowColor: isSelected ? 'rgba(59, 130, 246, 0.18)' : 'rgba(0, 0, 0, 0.06)',
        shadowBlur: isSelected ? 20 : 8,
        shadowOffsetY: 2,
      },
      label: {
        show: true,
        formatter: [
          `{title|${node.name}}{meta|${node.healthy_count}/${node.instance_count}}`,
          ...shownInstances.map((instance) => {
             const statusIcon = instance.status === 'passing' ? '{upIcon|UP}' : '{downIcon|Down}'
             return `{inst|${instance.host}:${instance.port}}${statusIcon}`
          }),
        ]
          .filter(Boolean)
          .join('\n') + moreLabel,
        rich: {
          title: {
            fontSize: 15,
            fontWeight: 700,
            fontFamily: 'Inter, system-ui, sans-serif',
            color: '#0f172a',
            lineHeight: 20,
            width: 110,
            align: 'left',
          },
          meta: {
            fontSize: 11,
            fontFamily: 'Inter, system-ui, sans-serif',
            fontWeight: 600,
            color: tone.accent,
            lineHeight: 20,
            width: 40,
            align: 'right',
          },
          inst: {
            fontSize: 11,
            fontFamily: 'Inter, system-ui, sans-serif',
            color: '#64748b',
            lineHeight: 18,
            width: 120,
            align: 'left',
          },
          upIcon: {
            fontSize: 10,
            fontWeight: 700,
            color: '#10b981',
            lineHeight: 18,
            width: 30,
            align: 'right',
          },
          downIcon: {
            fontSize: 10,
            fontWeight: 700,
            color: '#ef4444',
            lineHeight: 18,
            width: 30,
            align: 'right',
          },
          more: {
            fontSize: 10,
            fontFamily: 'Inter, system-ui, sans-serif',
            fontWeight: 500,
            color: '#3b82f6',
            lineHeight: 15,
            width: 150,
            align: 'left',
          },
        },
      },
    }
  })

  const links = graphData.value.edges.map((edge) => ({
    source: edge.source,
    target: edge.target,
    lineStyle: {
      color: '#cbd5e1',
      curveness: 0.2,
      width: 1,
    },
    emphasis: {
      lineStyle: {
        width: 1.5,
        color: '#3b82f6',
      },
    },
  }))

  return {
    tooltip: {
      trigger: 'item',
      backgroundColor: '#ffffff',
      borderColor: '#e2e8f0',
      borderWidth: 1,
      padding: [12, 16],
      textStyle: {
        color: '#0f172a',
        fontSize: 12,
        fontFamily: 'Inter, system-ui, sans-serif',
      },
      extraCssText: 'border-radius: 8px; box-shadow: 0 4px 16px rgba(0,0,0,0.08);',
      formatter(params: any) {
        if (params.dataType === 'edge') {
          return `<span style="color:#64748b">${params.data.source}</span> → <span style="color:#64748b">${params.data.target}</span>`
        }
        const node = graphData.value.nodes.find((item) => item.id === params.data.id)
        if (!node) return params.data.name
        const instances = node.instances.length
          ? node.instances
              .map(
                (instance) =>
                  `<span style="font-family:monospace;font-size:11px;color:#475569">${instance.host}:${instance.port}</span> <span style="color:${instance.status === 'passing' ? '#10b981' : '#ef4444'};font-size:10px">${instance.status === 'passing' ? '● healthy' : '● critical'}</span>`,
              )
              .join('<br/>')
          : '<span style="color:#94a3b8">No live instances</span>'
        return `<div style="font-weight:600;font-size:13px;margin-bottom:6px">${node.name}</div><div style="color:#64748b;font-size:11px;margin-bottom:8px">${node.healthy_count}/${node.instance_count} healthy</div>${instances}`
      },
    },
    animationDuration: 500,
    animationDurationUpdate: 300,
    animationEasing: 'cubicOut',
    series: [
      {
        type: 'graph',
        layout: 'force',
        data: nodes,
        links,
        tooltip: { show: false },
        roam: true,
        zoom: 1,
        label: {
          position: 'inside',
        },
        edgeSymbol: ['none', 'arrow'],
        edgeSymbolSize: [0, 10],
        force: {
          layoutAnimation: false,
          repulsion: 1000,
          edgeLength: [160, 280],
          gravity: 0.05,
          friction: 0.6,
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
    chart.on('graphRoam', () => {
      const option = chart?.getOption() as any
      if (option && option.series && option.series[0]) {
        currentZoom.value = option.series[0].zoom || 1
      }
    })
  }
}

async function render() {
  await nextTick()
  ensureChart()
  chart?.setOption(buildOption(), false)
}

function resize() {
  chart?.resize()
}

function zoom(delta: number) {
  if (!chart) return
  const option = chart.getOption() as any
  const z = option.series?.[0]?.zoom || currentZoom.value
  const newZoom = Math.max(0.45, Math.min(2.4, z + delta))
  currentZoom.value = newZoom
  chart.setOption({
    series: [{ zoom: newZoom }],
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
      <button type="button" @click="zoom(0.15)" title="Zoom in">
        <svg width="16" height="16" viewBox="0 0 16 16" fill="none"><path d="M8 4v8M4 8h8" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"/></svg>
      </button>
      <div class="zoom-value">{{ Math.round(currentZoom * 100) }}%</div>
      <button type="button" @click="zoom(-0.15)" title="Zoom out">
        <svg width="16" height="16" viewBox="0 0 16 16" fill="none"><path d="M4 8h8" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"/></svg>
      </button>
    </div>

    <div v-if="!loading && graphData.nodes.length === 0" class="board-empty">
      <div class="empty-state-inner">
        <svg width="40" height="40" viewBox="0 0 40 40" fill="none"><rect x="4" y="8" width="14" height="10" rx="2" stroke="#cbd5e1" stroke-width="1.5"/><rect x="22" y="22" width="14" height="10" rx="2" stroke="#cbd5e1" stroke-width="1.5"/><path d="M18 13h4v4" stroke="#cbd5e1" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"/><path d="M22 27h-4v-4" stroke="#cbd5e1" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"/></svg>
        <p>No topology data</p>
      </div>
    </div>

    <div ref="chartEl" class="graph-canvas" :class="{ hidden: !loading && graphData.nodes.length === 0 }"></div>
  </div>
</template>

<style scoped>
.topology-board {
  position: relative;
  height: 100%;
  min-height: 0;
  background:
    radial-gradient(circle at 1px 1px, rgba(148, 163, 184, 0.08) 1px, transparent 0),
    linear-gradient(180deg, #fafbfd 0%, #f1f5f9 100%);
  background-size: 20px 20px, auto;
  border: 1px solid rgba(148, 163, 184, 0.15);
  border-radius: 10px;
  overflow: hidden;
}

.board-toolbar {
  position: absolute;
  top: 12px;
  left: 12px;
  z-index: 2;
  display: inline-flex;
  flex-direction: column;
  gap: 1px;
  border-radius: 8px;
  overflow: hidden;
  border: 1px solid rgba(148, 163, 184, 0.15);
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.04);
}

.board-toolbar button {
  width: 32px;
  height: 32px;
  border: 0;
  background: rgba(255, 255, 255, 0.92);
  color: #475569;
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  transition: all 0.15s ease;
}

.board-toolbar button:hover {
  background: #ffffff;
  color: #3b82f6;
}

.zoom-value {
  width: 32px;
  height: 24px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: rgba(255, 255, 255, 0.92);
  color: #64748b;
  font-size: 10px;
  font-family: 'Inter', system-ui, sans-serif;
  font-weight: 600;
  border-top: 1px solid rgba(148, 163, 184, 0.12);
  border-bottom: 1px solid rgba(148, 163, 184, 0.12);
}

.graph-canvas {
  width: 100%;
  height: 100%;
  min-height: 500px;
}

.graph-canvas.hidden {
  display: none;
}

.board-empty {
  display: grid;
  height: 100%;
  min-height: 500px;
  place-items: center;
}

.empty-state-inner {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 12px;
}

.empty-state-inner p {
  color: #94a3b8;
  font-size: 13px;
  font-weight: 500;
}

@media (max-width: 920px) {
  .graph-canvas,
  .board-empty {
    min-height: 400px;
  }
}
</style>
