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

function clamp(value: number, min: number, max: number) {
  return Math.min(max, Math.max(min, value))
}

function average(values: number[]) {
  if (!values.length) return 0
  return values.reduce((sum, value) => sum + value, 0) / values.length
}

function spreadPositions(values: number[], minGap: number, minX: number, maxX: number) {
  if (values.length <= 1) {
    return values.map((value) => clamp(value, minX, maxX))
  }

  const sorted = values.map((value, index) => ({ value, index })).sort((a, b) => a.value - b.value)
  sorted[0]!.value = clamp(sorted[0]!.value, minX, maxX)

  for (let index = 1; index < sorted.length; index += 1) {
    sorted[index]!.value = Math.max(sorted[index]!.value, sorted[index - 1]!.value + minGap)
  }

  const overflow = sorted[sorted.length - 1]!.value - maxX
  if (overflow > 0) {
    sorted[sorted.length - 1]!.value -= overflow
    for (let index = sorted.length - 2; index >= 0; index -= 1) {
      sorted[index]!.value = Math.min(sorted[index]!.value, sorted[index + 1]!.value - minGap)
    }
  }

  if (sorted[0]!.value < minX) {
    const shift = minX - sorted[0]!.value
    for (const item of sorted) {
      item.value += shift
    }
  }

  const result = new Array<number>(values.length)
  for (const item of sorted) {
    result[item.index] = clamp(item.value, minX, maxX)
  }
  return result
}

function nodeTone(node: TopologyNode) {
  if (node.instance_count === 0) {
    return {
      border: '#94a3b8',
      fillTop: '#ffffff',
      fillBottom: '#f8fafc',
      accent: '#64748b',
      title: '#94a3b8',
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
      title: '#0f172a',
      subtext: '#64748b',
      badgeBg: 'rgba(34, 197, 94, 0.14)',
      chipBg: 'rgba(255, 255, 255, 0.95)',
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
      title: '#0f172a',
      subtext: '#64748b',
      badgeBg: 'rgba(245, 158, 11, 0.15)',
      chipBg: 'rgba(255, 255, 255, 0.95)',
      chipBorder: 'rgba(253, 230, 138, 0.6)',
      glow: 'rgba(245, 158, 11, 0.16)',
    }
  }

  return {
    border: '#ef4444',
    fillTop: '#ffffff',
    fillBottom: '#fef2f2',
    accent: '#dc2626',
    title: '#0f172a',
    subtext: '#64748b',
    badgeBg: 'rgba(239, 68, 68, 0.14)',
    chipBg: 'rgba(255, 255, 255, 0.95)',
    chipBorder: 'rgba(252, 165, 165, 0.58)',
    glow: 'rgba(239, 68, 68, 0.16)',
  }
}

function computeLevels() {
  const nodes = graphData.value.nodes
  const outgoing = new Map<string, string[]>()
  const incoming = new Map<string, string[]>()

  for (const node of nodes) {
    outgoing.set(node.id, [])
    incoming.set(node.id, [])
  }

  for (const edge of graphData.value.edges) {
    if (!outgoing.has(edge.source) || !incoming.has(edge.target)) continue
    outgoing.get(edge.source)!.push(edge.target)
    incoming.get(edge.target)!.push(edge.source)
  }

  const memo = new Map<string, number>()
  const stack = new Set<string>()

  const depthOf = (id: string): number => {
    if (memo.has(id)) return memo.get(id)!
    if (stack.has(id)) return 0

    stack.add(id)
    const next = outgoing.get(id) || []
    const depth = next.length ? 1 + Math.max(...next.map((item) => depthOf(item))) : 0
    stack.delete(id)
    memo.set(id, depth)
    return depth
  }

  for (const node of nodes) {
    depthOf(node.id)
  }

  return { outgoing, incoming, levels: memo }
}

function computeLayout(width: number, height: number) {
  const { outgoing, incoming, levels } = computeLevels()
  const layerMap = new Map<number, string[]>()
  const nodeMap = new Map(graphData.value.nodes.map((node) => [node.id, node] as const))
  const maxLevel = Math.max(0, ...Array.from(levels.values()))

  for (const node of graphData.value.nodes) {
    const level = levels.get(node.id) ?? 0
    if (!layerMap.has(level)) {
      layerMap.set(level, [])
    }
    layerMap.get(level)!.push(node.id)
  }

  const centerX = width / 2
  const topPadding = 110
  const bottomPadding = 110
  const minX = 170
  const maxX = Math.max(minX, width - 170)
  const usableHeight = Math.max(240, height - topPadding - bottomPadding)
  const rowGap = maxLevel > 0 ? usableHeight / maxLevel : 0
  const positions = new Map<string, { x: number; y: number }>()

  for (let level = 0; level <= maxLevel; level += 1) {
    const ids = [...(layerMap.get(level) || [])]
    ids.sort((left, right) => {
      const leftBary = average((incoming.get(left) || []).map((item) => positions.get(item)?.x).filter((item): item is number => typeof item === 'number'))
      const rightBary = average((incoming.get(right) || []).map((item) => positions.get(item)?.x).filter((item): item is number => typeof item === 'number'))
      if (leftBary !== rightBary) return leftBary - rightBary
      return (nodeMap.get(left)?.name || left).localeCompare(nodeMap.get(right)?.name || right)
    })

    const y = maxLevel === 0 ? height / 2 : topPadding + rowGap * level
    const rowSpread = Math.min(220, width * 0.18)
    const rawXs = ids.map((id, index) => {
      const preferredNeighbors = [
        ...(outgoing.get(id) || []).map((item) => positions.get(item)?.x).filter((item): item is number => typeof item === 'number'),
        ...(incoming.get(id) || []).map((item) => positions.get(item)?.x).filter((item): item is number => typeof item === 'number'),
      ]

      const barycenter = preferredNeighbors.length ? average(preferredNeighbors) : centerX
      if (ids.length === 1) {
        const singleOffset = level === 0 ? 0 : level % 2 === 1 ? -rowSpread : rowSpread
        return barycenter * 0.55 + (centerX + singleOffset) * 0.45
      }

      const base = minX + ((maxX - minX) * (index + 1)) / (ids.length + 1)
      return barycenter * 0.65 + base * 0.35
    })

    const minGap = clamp((maxX - minX) / Math.max(ids.length, 1), 250, 340)
    const adjusted = spreadPositions(rawXs, minGap, minX, maxX)
    adjusted.forEach((x, index) => {
      positions.set(ids[index]!, { x, y })
    })
  }

  return positions
}

function buildOption(): echarts.EChartsOption {
  const width = chartEl.value?.clientWidth || 1080
  const height = chartEl.value?.clientHeight || 620
  const positions = computeLayout(width, height)

  const nodes = graphData.value.nodes.map((node) => {
    const tone = nodeTone(node)
    const isSelected = props.selectedNode === node.id
    const addresses = displayAddresses(node)
    const longestAddress = addresses.reduce((max, address) => Math.max(max, address.length), 0)
    const ratioText = `${node.healthy_count}/${node.instance_count}`
    const badgeWidth = 72
    const nodeWidth = Math.min(300, Math.max(244, Math.max(node.name.length * 8.3 + 96, longestAddress * 7 + 66)))
    const nodeHeight = 78 + Math.max(1, addresses.length) * 24
    const titleWidth = Math.max(110, nodeWidth - badgeWidth - 44)
    const metaWidth = badgeWidth
    const contentWidth = nodeWidth - 46
    const title = escapeRichText(node.name)
    const addressRows = addresses.length
      ? addresses.map((address) => `{addr|${escapeRichText(address)}}`).join('\n')
      : '{empty|No instance}'
    const pos = positions.get(node.id) || { x: width / 2, y: height / 2 }

    return {
      id: node.id,
      name: node.name,
      value: node.name,
      x: pos.x,
      y: pos.y,
      symbol: 'rect',
      symbolSize: [nodeWidth, nodeHeight],
      draggable: true,
      itemStyle: {
        color: new echarts.graphic.LinearGradient(0, 0, 1, 1, [
          { offset: 0, color: isSelected ? '#f8fbff' : tone.fillTop },
          { offset: 1, color: isSelected ? '#eef5ff' : tone.fillBottom },
        ]),
        borderColor: isSelected ? '#2563eb' : tone.border,
        borderWidth: isSelected ? 2.6 : 1.6,
        shadowColor: isSelected ? 'rgba(37, 99, 235, 0.20)' : tone.glow,
        shadowBlur: isSelected ? 24 : 16,
        shadowOffsetY: 8,
      },
      label: {
        show: true,
        formatter: `{title|${title}}{meta|${ratioText}}\n{hint|Alive / Registered}\n${addressRows}`,
        rich: {
          title: {
            fontSize: 15,
            fontWeight: 700,
            fontFamily: 'Inter, system-ui, sans-serif',
            color: tone.title,
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
            borderColor: 'rgba(255,255,255,0.55)',
            borderWidth: 1,
            borderRadius: 0,
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
            borderRadius: 0,
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
            borderRadius: 0,
            padding: [5, 10, 5, 10],
            lineHeight: 26,
            width: contentWidth,
            align: 'left' as const,
          },
        },
      },
    }
  })

  const outgoingGroups = new Map<string, string[]>()
  for (const edge of graphData.value.edges) {
    if (!outgoingGroups.has(edge.source)) {
      outgoingGroups.set(edge.source, [])
    }
    outgoingGroups.get(edge.source)!.push(edge.target)
  }
  for (const items of outgoingGroups.values()) {
    items.sort()
  }

  const links = graphData.value.edges.map((edge) => {
    const siblings = outgoingGroups.get(edge.source) || []
    const siblingIndex = siblings.indexOf(edge.target)
    const baseCurve = siblings.length > 1 ? (siblingIndex - (siblings.length - 1) / 2) * 0.18 : 0
    const relatedToSelection =
      !!props.selectedNode && (edge.source === props.selectedNode || edge.target === props.selectedNode)

    return {
      source: edge.source,
      target: edge.target,
      lineStyle: {
        color: relatedToSelection ? 'rgba(37, 99, 235, 0.82)' : 'rgba(59, 130, 246, 0.48)',
        curveness: baseCurve,
        width: relatedToSelection ? 3.2 : 2.4,
        opacity: relatedToSelection ? 1 : 0.95,
      },
      emphasis: {
        lineStyle: {
          width: 3.8,
          color: 'rgba(29, 78, 216, 0.95)',
          opacity: 1,
        },
      },
    }
  })

  return {
    animationDuration: 600,
    animationDurationUpdate: 350,
    animationEasing: 'cubicOut',
    tooltip: {
      trigger: 'item',
      backgroundColor: '#ffffff',
      borderColor: '#dbe4f0',
      borderWidth: 1,
      padding: [12, 16],
      textStyle: {
        color: '#0f172a',
        fontSize: 12,
        fontFamily: 'Inter, system-ui, sans-serif',
      },
      extraCssText: 'border-radius: 0; box-shadow: 0 12px 30px rgba(15,23,42,0.10);',
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

        return `<div style="font-weight:700;font-size:13px;margin-bottom:6px">${node.name}</div><div style="color:#64748b;font-size:11px;margin-bottom:8px">${node.healthy_count}/${node.instance_count} alive / total</div>${instances}`
      },
    },
    series: [
      {
        type: 'graph',
        layout: 'none',
        coordinateSystem: null,
        data: nodes,
        links,
        roam: true,
        zoom: 1,
        tooltip: { show: false },
        label: {
          position: 'inside',
        },
        edgeSymbol: ['none', 'arrow'],
        edgeSymbolSize: [0, 14],
        edgeLabel: {
          show: false,
        },
        lineStyle: {
          opacity: 0.96,
        },
        emphasis: {
          focus: 'adjacency' as const,
          lineStyle: {
            width: 3.8,
            color: 'rgba(29, 78, 216, 0.95)',
          },
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
  chart?.setOption(buildOption(), true)
}

function resize() {
  chart?.resize()
  render()
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
        <svg width="40" height="40" viewBox="0 0 40 40" fill="none"><rect x="4" y="8" width="14" height="10" rx="0" stroke="#cbd5e1" stroke-width="1.5"/><rect x="22" y="22" width="14" height="10" rx="0" stroke="#cbd5e1" stroke-width="1.5"/><path d="M18 13h4v4" stroke="#cbd5e1" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"/><path d="M22 27h-4v-4" stroke="#cbd5e1" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"/></svg>
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
  border-radius: 0;
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
  border-radius: 0;
  overflow: hidden;
  border: 1px solid rgba(148, 163, 184, 0.16);
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
  color: #2563eb;
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
  min-height: 560px;
}

.graph-canvas.hidden {
  display: none;
}

.board-empty {
  display: grid;
  height: 100%;
  min-height: 560px;
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
    min-height: 420px;
  }
}
</style>
