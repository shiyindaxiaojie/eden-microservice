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

function displayAddresses(node: TopologyNode) {
  const seen = new Set<string>()
  const items: string[] = []

  for (const instance of node.instances) {
    const address = `${instance.host}:${instance.port}`
    if (seen.has(address)) continue
    seen.add(address)
    items.push(address)
    if (items.length >= 3) break
  }

  return items
}

function escapeRichText(value: string) {
  return value.replace(/[{}|]/g, ' ').replace(/\n/g, ' ')
}

function nodeTone(node: TopologyNode) {
  if (node.instance_count === 0) {
    return {
      border: '#94a3b8',
      fillTop: '#ffffff',
      fillBottom: '#f8fafc',
      accent: '#64748b',
      text: '#94a3b8',
      subtext: '#94a3b8',
      badgeBg: 'rgba(148, 163, 184, 0.12)',
      chipBg: 'rgba(255, 255, 255, 0.86)',
      chipBorder: 'rgba(148, 163, 184, 0.18)',
      glow: 'rgba(148, 163, 184, 0.16)',
    }
  }

  if (node.healthy_count === node.instance_count) {
    return {
      border: '#22c55e',
      fillTop: '#ffffff',
      fillBottom: '#f0fdf4',
      accent: '#16a34a',
      text: '#0f172a',
      subtext: '#64748b',
      badgeBg: 'rgba(34, 197, 94, 0.14)',
      chipBg: 'rgba(255, 255, 255, 0.94)',
      chipBorder: 'rgba(134, 239, 172, 0.55)',
      glow: 'rgba(34, 197, 94, 0.14)',
    }
  }

  if (node.healthy_count > 0) {
    return {
      border: '#f59e0b',
      fillTop: '#ffffff',
      fillBottom: '#fffbeb',
      accent: '#d97706',
      text: '#0f172a',
      subtext: '#64748b',
      badgeBg: 'rgba(245, 158, 11, 0.16)',
      chipBg: 'rgba(255, 255, 255, 0.94)',
      chipBorder: 'rgba(253, 230, 138, 0.6)',
      glow: 'rgba(245, 158, 11, 0.16)',
    }
  }

  return {
    border: '#ef4444',
    fillTop: '#ffffff',
    fillBottom: '#fef2f2',
    accent: '#dc2626',
    text: '#0f172a',
    subtext: '#64748b',
    badgeBg: 'rgba(239, 68, 68, 0.14)',
    chipBg: 'rgba(255, 255, 255, 0.94)',
    chipBorder: 'rgba(252, 165, 165, 0.58)',
    glow: 'rgba(239, 68, 68, 0.16)',
  }
}

function buildOption(): echarts.EChartsOption {
  const nodes = graphData.value.nodes.map((node) => {
    const tone = nodeTone(node)
    const isSelected = props.selectedNode === node.id
    const addresses = displayAddresses(node)
    const longestAddress = addresses.reduce((max, address) => Math.max(max, address.length), 0)
    const ratioText = `${node.healthy_count}/${node.instance_count}`
    const badgeWidth = Math.max(56, ratioText.length * 8 + 20)
    const nodeWidth = Math.min(292, Math.max(220, Math.max(node.name.length * 8.5 + 90, longestAddress * 7 + 58)))
    const nodeHeight = 74 + Math.max(1, addresses.length) * 24
    const titleWidth = Math.max(112, nodeWidth - badgeWidth - 42)
    const metaWidth = badgeWidth
    const contentWidth = nodeWidth - 44
    const title = escapeRichText(node.name)
    const addressRows = addresses.length
      ? addresses.map((address) => `{addr|${escapeRichText(address)}}`).join('\n')
      : '{empty|No instance}'

    return {
      id: node.id,
      name: node.name,
      value: node.name,
      symbol: 'rect',
      symbolSize: [nodeWidth, nodeHeight],
      draggable: false,
      itemStyle: {
        color: new echarts.graphic.LinearGradient(0, 0, 1, 1, [
          {
            offset: 0,
            color: isSelected ? '#f8fbff' : tone.fillTop,
          },
          {
            offset: 1,
            color: isSelected ? '#eef5ff' : tone.fillBottom,
          },
        ]),
        borderColor: isSelected ? '#3b82f6' : tone.border,
        borderWidth: isSelected ? 2.4 : 1.4,
        shadowColor: isSelected ? 'rgba(59, 130, 246, 0.18)' : tone.glow,
        shadowBlur: isSelected ? 22 : 14,
        shadowOffsetY: 6,
      },
      label: {
        show: true,
        formatter: `{title|${title}}{meta|${ratioText}}\n{hint|Alive / Registered}\n${addressRows}`,
        rich: {
          title: {
            fontSize: 15,
            fontWeight: 700,
            fontFamily: 'Inter, system-ui, sans-serif',
            color: tone.text,
            lineHeight: 22,
            width: titleWidth,
            align: 'left' as const,
          },
          meta: {
            fontSize: 12,
            fontFamily: 'Inter, system-ui, sans-serif',
            fontWeight: 700,
            color: tone.accent,
            backgroundColor: tone.badgeBg,
            borderRadius: 6,
            padding: [5, 0, 5, 0],
            lineHeight: 22,
            width: metaWidth,
            align: 'right' as const,
          },
          hint: {
            fontSize: 10,
            fontFamily: 'Inter, system-ui, sans-serif',
            fontWeight: 600,
            color: tone.subtext,
            lineHeight: 18,
            width: contentWidth,
            align: 'left' as const,
          },
          addr: {
            fontSize: 11,
            fontFamily: 'JetBrains Mono, SFMono-Regular, Consolas, monospace',
            color: '#334155',
            backgroundColor: tone.chipBg,
            borderColor: tone.chipBorder,
            borderWidth: 1,
            borderRadius: 6,
            padding: [5, 10, 5, 10],
            lineHeight: 26,
            width: contentWidth,
            align: 'left' as const,
          },
          empty: {
            fontSize: 11,
            fontFamily: 'Inter, system-ui, sans-serif',
            color: '#94a3b8',
            backgroundColor: 'rgba(248, 250, 252, 0.88)',
            borderColor: 'rgba(203, 213, 225, 0.72)',
            borderWidth: 1,
            borderRadius: 6,
            padding: [5, 10, 5, 10],
            lineHeight: 26,
            width: contentWidth,
            align: 'left' as const,
          },
        },
      },
    }
  })

  const links = graphData.value.edges.map((edge) => {
    const relatedToSelection =
      !!props.selectedNode && (edge.source === props.selectedNode || edge.target === props.selectedNode)

    return {
      source: edge.source,
      target: edge.target,
      lineStyle: {
        color: relatedToSelection ? 'rgba(37, 99, 235, 0.78)' : 'rgba(59, 130, 246, 0.36)',
        curveness: 0.14,
        width: relatedToSelection ? 3 : 2.2,
        opacity: relatedToSelection ? 1 : 0.92,
      },
      emphasis: {
        lineStyle: {
          width: 3.6,
          color: 'rgba(29, 78, 216, 0.92)',
          opacity: 1,
        },
      },
    }
  })

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
          return `<span style="color:#64748b">${params.data.source}</span> &rarr; <span style="color:#64748b">${params.data.target}</span>`
        }

        const node = graphData.value.nodes.find((item) => item.id === params.data.id)
        if (!node) return params.data.name

        const instances = node.instances.length
          ? node.instances
              .map(
                (instance) =>
                  `<span style="font-family:monospace;font-size:11px;color:#475569">${instance.host}:${instance.port}</span> <span style="color:${instance.status === 'passing' ? '#10b981' : '#ef4444'};font-size:10px">${instance.status === 'passing' ? 'healthy' : 'critical'}</span>`,
              )
              .join('<br/>')
          : '<span style="color:#94a3b8">No live instances</span>'

        return `<div style="font-weight:600;font-size:13px;margin-bottom:6px">${node.name}</div><div style="color:#64748b;font-size:11px;margin-bottom:8px">${node.healthy_count}/${node.instance_count} alive / total</div>${instances}`
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
        edgeSymbol: ['circle', 'arrow'],
        edgeSymbolSize: [4, 12],
        lineStyle: {
          opacity: 0.95,
        },
        emphasis: {
          focus: 'adjacency' as const,
          lineStyle: {
            width: 3.6,
            color: 'rgba(29, 78, 216, 0.92)',
            opacity: 1,
          },
        },
        force: {
          layoutAnimation: false,
          repulsion: 1180,
          edgeLength: [220, 320],
          gravity: 0.03,
          friction: 0.55,
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
    linear-gradient(180deg, #fbfdff 0%, #ffffff 100%);
  background-size: 20px 20px;
  border: 1px solid rgba(148, 163, 184, 0.15);
  border-radius: 14px;
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
  border-radius: 10px;
  overflow: hidden;
  border: 1px solid rgba(148, 163, 184, 0.15);
  box-shadow: 0 10px 24px rgba(15, 23, 42, 0.08);
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
