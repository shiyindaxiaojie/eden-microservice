<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import * as echarts from 'echarts'
import type { TopologyGraph, TopologyInstance, TopologyNode } from '../api/registry'
import { useI18n } from '../utils/i18n'

type NodeHealthState = 'empty' | 'healthy' | 'partial' | 'critical'

interface ServiceNodeMetrics {
  width: number
  height: number
  titleText: string
  titleLines: number
  iconColumns: number
  iconRows: number
  iconTop: number
}

const SERVICE_CARD_WIDTH = 188
const CARD_PADDING_X = 14
const CARD_PADDING_TOP = 12
const CARD_PADDING_BOTTOM = 12
const STATUS_LINE_HEIGHT = 16
const STATUS_TITLE_GAP = 4
const TITLE_LINE_HEIGHT = 16
const TITLE_MAX_CHARS = 18
const TITLE_MAX_LINES = 3
const INSTANCE_ICON_SIZE = 16
const INSTANCE_ICON_GAP = 8
const INSTANCE_TOP_GAP = 10
const INSTANCE_HEX_SYMBOL = 'path://M0 -1L0.866 -0.5L0.866 0.5L0 1L-0.866 0.5L-0.866 -0.5Z'

const { locale } = useI18n()
const isZh = computed(() => locale.value === 'zh')
const text = (zh: string, en: string) => (isZh.value ? zh : en)

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

function escapeRichText(value: string) {
  return value.replace(/[{}|]/g, ' ')
}

function escapeHtml(value: string) {
  return value
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#39;')
}

function serviceHealthState(node: TopologyNode): NodeHealthState {
  if (node.instance_count === 0) return 'empty'
  if (node.healthy_count === node.instance_count) return 'healthy'
  if (node.healthy_count > 0) return 'partial'
  return 'critical'
}

function nodeStatusLabel(state: NodeHealthState) {
  switch (state) {
    case 'healthy':
      return text('健康', 'Healthy')
    case 'partial':
      return text('部分异常', 'Degraded')
    case 'critical':
      return text('异常', 'Critical')
    default:
      return text('无实例', 'Empty')
  }
}

function nodeTone(node: TopologyNode) {
  const state = serviceHealthState(node)
  if (state === 'empty') {
    return {
      border: '#94a3b8',
      fillTop: '#ffffff',
      fillBottom: '#f8fafc',
      title: '#64748b',
      badgeText: '#64748b',
      badgeBg: 'rgba(148, 163, 184, 0.14)',
      glow: 'rgba(148, 163, 184, 0.14)',
    }
  }

  if (state === 'healthy') {
    return {
      border: '#22c55e',
      fillTop: '#ffffff',
      fillBottom: '#f0fdf4',
      title: '#0f172a',
      badgeText: '#15803d',
      badgeBg: 'rgba(34, 197, 94, 0.14)',
      glow: 'rgba(34, 197, 94, 0.14)',
    }
  }

  if (state === 'partial') {
    return {
      border: '#f59e0b',
      fillTop: '#ffffff',
      fillBottom: '#fffbeb',
      title: '#0f172a',
      badgeText: '#b45309',
      badgeBg: 'rgba(245, 158, 11, 0.16)',
      glow: 'rgba(245, 158, 11, 0.14)',
    }
  }

  return {
    border: '#ef4444',
    fillTop: '#ffffff',
    fillBottom: '#fef2f2',
    title: '#0f172a',
    badgeText: '#dc2626',
    badgeBg: 'rgba(239, 68, 68, 0.14)',
    glow: 'rgba(239, 68, 68, 0.14)',
  }
}

function wrapServiceName(name: string, maxCharsPerLine = TITLE_MAX_CHARS, maxLines = TITLE_MAX_LINES) {
  const segments = name.split('-').flatMap((part, index, array) => (index < array.length - 1 ? [`${part}-`] : [part]))
  const lines: string[] = []
  let current = ''

  const pushChunk = (chunk: string) => {
    if (!chunk) return
    if (lines.length >= maxLines) {
      lines[maxLines - 1] = `${lines[maxLines - 1] || ''}${chunk}`
      return
    }
    lines.push(chunk)
  }

  for (const segment of segments) {
    if (segment.length > maxCharsPerLine) {
      if (current) {
        pushChunk(current)
        current = ''
      }
      for (let offset = 0; offset < segment.length; offset += maxCharsPerLine) {
        pushChunk(segment.slice(offset, offset + maxCharsPerLine))
      }
      continue
    }

    if ((current + segment).length <= maxCharsPerLine) {
      current += segment
      continue
    }

    pushChunk(current)
    current = segment
  }

  if (current) {
    pushChunk(current)
  }

  const compact = lines.filter(Boolean)
  if (!compact.length) {
    compact.push(name)
  }

  if (compact.length > maxLines) {
    compact.splice(maxLines)
  }

  if (compact.length === maxLines && compact[maxLines - 1]!.length > maxCharsPerLine) {
    compact[maxLines - 1] = `${compact[maxLines - 1]!.slice(0, Math.max(1, maxCharsPerLine - 1))}…`
  }

  return {
    text: compact.join('\n'),
    lines: compact.length,
  }
}

function serviceMetrics(node: TopologyNode): ServiceNodeMetrics {
  const wrappedTitle = wrapServiceName(node.name)
  const iconColumns = Math.max(1, Math.floor((SERVICE_CARD_WIDTH - CARD_PADDING_X * 2 + INSTANCE_ICON_GAP) / (INSTANCE_ICON_SIZE + INSTANCE_ICON_GAP)))
  const iconRows = node.instances.length ? Math.ceil(node.instances.length / iconColumns) : 0
  const iconTop =
    CARD_PADDING_TOP + STATUS_LINE_HEIGHT + STATUS_TITLE_GAP + wrappedTitle.lines * TITLE_LINE_HEIGHT + INSTANCE_TOP_GAP
  const iconsHeight = iconRows ? iconRows * INSTANCE_ICON_SIZE + (iconRows - 1) * INSTANCE_ICON_GAP : 0
  const height = Math.max(76, iconTop + iconsHeight + CARD_PADDING_BOTTOM)

  return {
    width: SERVICE_CARD_WIDTH,
    height,
    titleText: wrappedTitle.text,
    titleLines: wrappedTitle.lines,
    iconColumns,
    iconRows,
    iconTop,
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

function computeLayout(width: number, height: number, metricsMap: Map<string, ServiceNodeMetrics>) {
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

  const levelIds = Array.from({ length: maxLevel + 1 }, (_, level) => layerMap.get(level) || [])
  const levelHeights = levelIds.map((ids) =>
    Math.max(
      76,
      ...ids.map((id) => metricsMap.get(id)?.height || 76),
    ),
  )

  const topPadding = 92
  const bottomPadding = 92
  const usableHeight = Math.max(260, height - topPadding - bottomPadding)
  const totalNodeHeight = levelHeights.reduce((sum, value) => sum + value, 0)
  const baseGap = 96
  const rowGap =
    levelHeights.length > 1
      ? Math.max(68, Math.min(baseGap, (usableHeight - totalNodeHeight) / Math.max(1, levelHeights.length - 1)))
      : 0
  const centerX = width / 2
  const sidePadding = Math.min(150, Math.max(SERVICE_CARD_WIDTH / 2 + 24, width * 0.12))
  const minX = sidePadding
  const maxX = Math.max(minX, width - sidePadding)
  const positions = new Map<string, { x: number; y: number }>()

  let currentY = levelHeights.length === 1 ? height / 2 : topPadding + levelHeights[0]! / 2

  for (let level = 0; level <= maxLevel; level += 1) {
    const ids = [...(layerMap.get(level) || [])]
    ids.sort((left, right) => {
      const leftBary = average(
        (incoming.get(left) || []).map((item) => positions.get(item)?.x).filter((item): item is number => typeof item === 'number'),
      )
      const rightBary = average(
        (incoming.get(right) || []).map((item) => positions.get(item)?.x).filter((item): item is number => typeof item === 'number'),
      )
      if (leftBary !== rightBary) return leftBary - rightBary
      return (nodeMap.get(left)?.name || left).localeCompare(nodeMap.get(right)?.name || right)
    })

    const y = levelHeights.length === 1 ? height / 2 : currentY
    const rowSpread = Math.min(176, width * 0.16)
    const rawXs = ids.map((id, index) => {
      const preferredNeighbors = [
        ...(outgoing.get(id) || []).map((item) => positions.get(item)?.x).filter((item): item is number => typeof item === 'number'),
        ...(incoming.get(id) || []).map((item) => positions.get(item)?.x).filter((item): item is number => typeof item === 'number'),
      ]

      const barycenter = preferredNeighbors.length ? average(preferredNeighbors) : centerX
      if (ids.length === 1) {
        const singleOffset = level === 0 ? 0 : level % 2 === 1 ? -rowSpread : rowSpread
        return barycenter * 0.58 + (centerX + singleOffset) * 0.42
      }

      const base = minX + ((maxX - minX) * (index + 1)) / (ids.length + 1)
      return barycenter * 0.64 + base * 0.36
    })

    const minGap = clamp((maxX - minX) / Math.max(ids.length, 1), SERVICE_CARD_WIDTH + 22, SERVICE_CARD_WIDTH + 84)
    const adjusted = spreadPositions(rawXs, minGap, minX, maxX)
    adjusted.forEach((x, index) => {
      positions.set(ids[index]!, { x, y })
    })

    if (level < maxLevel) {
      currentY += levelHeights[level]! / 2 + rowGap + levelHeights[level + 1]! / 2
    }
  }

  return positions
}

function instanceTone(instance: TopologyInstance, selected: boolean) {
  const passing = instance.status === 'passing'
  return {
    fill: passing ? '#22c55e' : '#ef4444',
    border: passing ? (selected ? '#14532d' : '#166534') : selected ? '#7f1d1d' : '#991b1b',
    glow: passing ? 'rgba(34, 197, 94, 0.28)' : 'rgba(239, 68, 68, 0.24)',
  }
}

function buildOption(): echarts.EChartsOption {
  const width = chartEl.value?.clientWidth || 1080
  const height = chartEl.value?.clientHeight || 620
  const metricsMap = new Map(graphData.value.nodes.map((node) => [node.id, serviceMetrics(node)] as const))
  const positions = computeLayout(width, height, metricsMap)

  const serviceNodes: any[] = graphData.value.nodes.map((node) => {
    const metrics = metricsMap.get(node.id) || serviceMetrics(node)
    const tone = nodeTone(node)
    const state = serviceHealthState(node)
    const isSelected = props.selectedNode === node.id
    const contentWidth = metrics.width - CARD_PADDING_X * 2
    const pos = positions.get(node.id) || { x: width / 2, y: height / 2 }

    return {
      id: node.id,
      name: node.name,
      kind: 'service',
      status: state,
      statusLabel: nodeStatusLabel(state),
      healthyCount: node.healthy_count,
      instanceCount: node.instance_count,
      namespace: node.namespace,
      x: pos.x,
      y: pos.y,
      symbol: 'roundRect',
      symbolSize: [metrics.width, metrics.height],
      draggable: false,
      z: 2,
      itemStyle: {
        color: new echarts.graphic.LinearGradient(0, 0, 1, 1, [
          { offset: 0, color: isSelected ? '#f8fbff' : tone.fillTop },
          { offset: 1, color: isSelected ? '#eef5ff' : tone.fillBottom },
        ]),
        borderColor: isSelected ? '#2563eb' : tone.border,
        borderWidth: isSelected ? 2.4 : 1.4,
        borderRadius: 16,
        shadowColor: isSelected ? 'rgba(37, 99, 235, 0.22)' : tone.glow,
        shadowBlur: isSelected ? 22 : 14,
        shadowOffsetY: 8,
      },
      label: {
        show: true,
        position: 'inside',
        align: 'left' as const,
        verticalAlign: 'top' as const,
        padding: [CARD_PADDING_TOP, CARD_PADDING_X, 0, CARD_PADDING_X],
        formatter: `{status|${escapeRichText(nodeStatusLabel(state))}}\n{title|${escapeRichText(metrics.titleText)}}`,
        rich: {
          status: {
            width: contentWidth,
            align: 'right' as const,
            fontSize: 10,
            fontFamily: 'Inter, system-ui, sans-serif',
            fontWeight: 700,
            color: tone.badgeText,

            lineHeight: STATUS_LINE_HEIGHT,
          },
          title: {
            width: contentWidth,
            align: 'left' as const,
            fontSize: 14,
            fontFamily: 'Inter, system-ui, sans-serif',
            fontWeight: 700,
            color: tone.title,
            lineHeight: TITLE_LINE_HEIGHT,
          },
        },
      },
    }
  })

  const instanceNodes: any[] = graphData.value.nodes.flatMap((node) => {
    const metrics = metricsMap.get(node.id) || serviceMetrics(node)
    const pos = positions.get(node.id) || { x: width / 2, y: height / 2 }
    const left = pos.x - metrics.width / 2 + CARD_PADDING_X + INSTANCE_ICON_SIZE / 2
    const top = pos.y - metrics.height / 2 + metrics.iconTop + INSTANCE_ICON_SIZE / 2
    const selectedParent = props.selectedNode === node.id

    return node.instances.map((instance, index) => {
      const column = index % metrics.iconColumns
      const row = Math.floor(index / metrics.iconColumns)
      const tone = instanceTone(instance, selectedParent)

      return {
        id: `${node.id}::${instance.id}`,
        name: `${instance.host}:${instance.port}`,
        kind: 'instance',
        parentId: node.id,
        parentName: node.name,
        instance,
        x: left + column * (INSTANCE_ICON_SIZE + INSTANCE_ICON_GAP),
        y: top + row * (INSTANCE_ICON_SIZE + INSTANCE_ICON_GAP),
        symbol: INSTANCE_HEX_SYMBOL,
        symbolKeepAspect: true,
        symbolSize: INSTANCE_ICON_SIZE,
        draggable: false,
        z: 3,
        itemStyle: {
          color: tone.fill,
          borderColor: tone.border,
          borderWidth: selectedParent ? 1.6 : 1.1,
          shadowColor: tone.glow,
          shadowBlur: selectedParent ? 12 : 8,
        },
        label: { show: false },
      }
    })
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

  const links: any[] = graphData.value.edges.map((edge) => {
    const siblings = outgoingGroups.get(edge.source) || []
    const siblingIndex = siblings.indexOf(edge.target)
    const baseCurve = siblings.length > 1 ? (siblingIndex - (siblings.length - 1) / 2) * 0.16 : 0
    const relatedToSelection =
      !!props.selectedNode && (edge.source === props.selectedNode || edge.target === props.selectedNode)

    return {
      source: edge.source,
      target: edge.target,
      lineStyle: {
        color: relatedToSelection ? 'rgba(37, 99, 235, 0.86)' : 'rgba(59, 130, 246, 0.38)',
        curveness: baseCurve,
        width: relatedToSelection ? 2.9 : 2,
        opacity: relatedToSelection ? 1 : 0.92,
      },
      emphasis: {
        lineStyle: {
          width: 3.4,
          color: 'rgba(29, 78, 216, 0.94)',
          opacity: 1,
        },
      },
    }
  })

  return {
    animationDuration: 450,
    animationDurationUpdate: 280,
    animationEasing: 'cubicOut',
    tooltip: {
      trigger: 'item',
      backgroundColor: '#ffffff',
      borderColor: '#dbe4f0',
      borderWidth: 1,
      padding: [10, 14],
      textStyle: {
        color: '#0f172a',
        fontSize: 12,
        fontFamily: 'Inter, system-ui, sans-serif',
      },
      extraCssText: 'border-radius: 12px; box-shadow: 0 12px 32px rgba(15,23,42,0.12);',
      formatter(params: any) {
        if (params.dataType === 'edge') {
          return `<span style="color:#64748b">${escapeHtml(params.data.source)}</span> &rarr; <span style="color:#64748b">${escapeHtml(params.data.target)}</span>`
        }

        const data = params.data || {}
        if (data.kind === 'instance' && data.instance) {
          const instance = data.instance as TopologyInstance
          const status = instance.status === 'passing' ? text('在线', 'Online') : text('异常', 'Critical')
          const tone = instance.status === 'passing' ? '#16a34a' : '#dc2626'
          return `
            <div style="display:flex;flex-direction:column;gap:4px;min-width:160px">
              <div style="font-size:12px;color:#64748b">${escapeHtml(data.parentName || '')}</div>
              <div style="font-weight:700;font-size:13px;color:#0f172a">${escapeHtml(instance.host)}:${instance.port}</div>
              <div style="font-size:11px;color:${tone};font-weight:600">${status}</div>
            </div>
          `
        }

        if (data.kind === 'service') {
          return `
            <div style="display:flex;flex-direction:column;gap:4px;min-width:168px">
              <div style="font-weight:700;font-size:13px;color:#0f172a">${escapeHtml(data.name || '')}</div>
              <div style="font-size:11px;color:#64748b">${escapeHtml(data.statusLabel || '')}</div>
              <div style="font-size:11px;color:#64748b">${data.healthyCount ?? 0}/${data.instanceCount ?? 0} ${text('健康实例', 'healthy instances')}</div>
            </div>
          `
        }

        return escapeHtml(data.name || '')
      },
    },
    series: [
      {
        type: 'graph',
        layout: 'none',

        data: [...serviceNodes, ...instanceNodes],
        links,
        roam: true,
        zoom: currentZoom.value,
        tooltip: { show: true },
        label: {
          position: 'inside',
        },
        edgeSymbol: ['none', 'arrow'],
        edgeSymbolSize: [0, 12],
        edgeLabel: {
          show: false,
        },
        lineStyle: {
          opacity: 0.95,
        },
        emphasis: {
          focus: 'adjacency' as const,
          lineStyle: {
            width: 3.4,
            color: 'rgba(29, 78, 216, 0.94)',
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
      if (params.dataType !== 'node' || !params.data) return
      if (params.data.kind === 'instance' && params.data.parentId) {
        emit('select', params.data.parentId)
        return
      }
      if (params.data.kind === 'service' && params.data.id) {
        emit('select', params.data.id)
      }
    })
    chart.on('graphRoam', () => {
      const option = chart?.getOption() as any
      if (option?.series?.[0]) {
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
  const current = option.series?.[0]?.zoom || currentZoom.value
  const newZoom = Math.max(0.45, Math.min(2.4, current + delta))
  currentZoom.value = newZoom
  chart.setOption({
    series: [{ zoom: newZoom }],
  })
}

watch(graphData, render, { deep: true })
watch(() => props.selectedNode, render)
watch(locale, render)

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
        <svg width="40" height="40" viewBox="0 0 40 40" fill="none"><rect x="4" y="8" width="14" height="10" rx="4" stroke="#cbd5e1" stroke-width="1.5"/><rect x="22" y="22" width="14" height="10" rx="4" stroke="#cbd5e1" stroke-width="1.5"/><path d="M18 13h4v4" stroke="#cbd5e1" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"/><path d="M22 27h-4v-4" stroke="#cbd5e1" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"/></svg>
        <p>{{ text('暂无拓扑数据', 'No topology data') }}</p>
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
    linear-gradient(90deg, rgba(226, 232, 240, 0.32) 1px, transparent 1px),
    linear-gradient(180deg, rgba(226, 232, 240, 0.28) 1px, transparent 1px),
    linear-gradient(180deg, #fbfdff 0%, #ffffff 100%);
  background-size: 22px 22px, 88px 88px, 88px 88px, auto;
  border: 1px solid rgba(148, 163, 184, 0.14);
  border-radius: 16px;
  overflow: hidden;
}

.board-toolbar {
  position: absolute;
  top: 14px;
  left: 14px;
  z-index: 2;
  display: inline-flex;
  flex-direction: column;
  gap: 1px;
  border-radius: 12px;
  overflow: hidden;
  border: 1px solid rgba(148, 163, 184, 0.16);
  box-shadow: 0 12px 28px rgba(15, 23, 42, 0.08);
  backdrop-filter: blur(10px);
}

.board-toolbar button {
  width: 34px;
  height: 34px;
  border: 0;
  background: rgba(255, 255, 255, 0.94);
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
  width: 34px;
  height: 24px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: rgba(255, 255, 255, 0.94);
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
