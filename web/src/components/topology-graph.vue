<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import * as echarts from 'echarts'
import type { TopologyGraph, TopologyInstance, TopologyNode } from '../api/registry'
import { useI18n } from '../utils/i18n'

type NodeHealthState = 'empty' | 'healthy' | 'partial' | 'critical'
type Point = [number, number]

interface ServiceNodeMetrics {
  width: number
  height: number
  titleText: string
  titleLines: number
  iconColumns: number
  iconRows: number
  iconTop: number
}

const SERVICE_CARD_WIDTH = 236
const SERVICE_CARD_MIN_HEIGHT = 92
const MIN_RENDER_SIZE = 120
const CARD_PADDING_X = 20
const CARD_PADDING_TOP = 16
const CARD_PADDING_BOTTOM = 18
const STATUS_LINE_HEIGHT = 18
const STATUS_BADGE_WIDTH = 52
const STATUS_TITLE_GAP = 8
const TITLE_LINE_HEIGHT = 18
const TITLE_MAX_CHARS = 20
const TITLE_MAX_LINES = 3
const INSTANCE_ICON_SIZE = 16
const INSTANCE_ICON_GAP = 8
const INSTANCE_ICON_OFFSET_X = INSTANCE_ICON_SIZE + INSTANCE_ICON_GAP
const INSTANCE_TOP_GAP = 14
const EDGE_ARROW_SIZE = 8
const EDGE_LANE_GAP = 18
const EDGE_SIBLING_GAP = 8
const EDGE_PADDING_X = 24
const EDGE_PADDING_Y = 14
const NODE_SCALE_RATIO = 0.6
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
let renderVersion = 0
let resizeObserver: ResizeObserver | null = null
let edgeLayoutFrame = 0

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
    compact[maxLines - 1] = `${compact[maxLines - 1]!.slice(0, Math.max(1, maxCharsPerLine - 3))}...`
  }

  return {
    text: compact.join('\n'),
    lines: compact.length,
  }
}

function serviceMetrics(node: TopologyNode): ServiceNodeMetrics {
  const wrappedTitle = wrapServiceName(node.name)
  const iconColumns = Math.max(
    1,
    Math.floor((SERVICE_CARD_WIDTH - CARD_PADDING_X * 2 - INSTANCE_ICON_OFFSET_X + INSTANCE_ICON_GAP) / (INSTANCE_ICON_SIZE + INSTANCE_ICON_GAP)),
  )
  const iconRows = node.instances.length ? Math.ceil(node.instances.length / iconColumns) : 0
  const headerHeight = Math.max(STATUS_LINE_HEIGHT, TITLE_LINE_HEIGHT)
  const titleExtraHeight = Math.max(0, wrappedTitle.lines - 1) * TITLE_LINE_HEIGHT
  const iconTop = CARD_PADDING_TOP + headerHeight + titleExtraHeight + INSTANCE_TOP_GAP
  const iconsHeight = iconRows ? iconRows * INSTANCE_ICON_SIZE + (iconRows - 1) * INSTANCE_ICON_GAP : 0
  const height = Math.max(SERVICE_CARD_MIN_HEIGHT, iconTop + iconsHeight + CARD_PADDING_BOTTOM)

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

function buildEdgeDescriptor(sourceId: string, targetId: string, index: number) {
  return `${sourceId}->${targetId}#${index}`
}

function getRectAnchor(center: Point, metrics: ServiceNodeMetrics, dx: number, dy: number, laneOffset: number): Point {
  const halfW = metrics.width / 2
  const halfH = metrics.height / 2
  const safeDx = Math.abs(dx) < 1e-6 ? (dx >= 0 ? 1e-6 : -1e-6) : dx
  const safeDy = Math.abs(dy) < 1e-6 ? (dy >= 0 ? 1e-6 : -1e-6) : dy
  const hitX = halfW / Math.abs(safeDx)
  const hitY = halfH / Math.abs(safeDy)

  if (hitX < hitY) {
    const y = clamp(safeDy * hitX + laneOffset, -halfH + EDGE_PADDING_Y, halfH - EDGE_PADDING_Y)
    return [center[0] + Math.sign(safeDx) * halfW, center[1] + y]
  }

  const x = clamp(safeDx * hitY + laneOffset, -halfW + EDGE_PADDING_X, halfW - EDGE_PADDING_X)
  return [center[0] + x, center[1] + Math.sign(safeDy) * halfH]
}

function buildPreciseEdgePoints(
  sourceCenter: Point,
  targetCenter: Point,
  sourceMetrics: ServiceNodeMetrics,
  targetMetrics: ServiceNodeMetrics,
  laneOffset: number,
  bend: number,
): Point[] {
  const dx = targetCenter[0] - sourceCenter[0]
  const dy = targetCenter[1] - sourceCenter[1]
  const start = getRectAnchor(sourceCenter, sourceMetrics, dx, dy, laneOffset)
  const end = getRectAnchor(targetCenter, targetMetrics, -dx, -dy, laneOffset)

  if (Math.abs(bend) < 0.5) {
    return [start, end]
  }

  const vx = end[0] - start[0]
  const vy = end[1] - start[1]
  const length = Math.hypot(vx, vy) || 1
  const mid: Point = [(start[0] + end[0]) / 2, (start[1] + end[1]) / 2]
  const control: Point = [mid[0] - (vy / length) * bend, mid[1] + (vx / length) * bend]
  return [start, end, control]
}

function getGraphRuntime() {
  if (!chart) return null

  const ec = chart as any
  const model = ec.getModel?.()
  const seriesModel = model?.getSeriesByIndex?.(0)
  const graph = seriesModel?.getGraph?.()
  const graphView = seriesModel ? ec.getViewOfSeriesModel?.(seriesModel) : null

  if (!seriesModel || !graph || !graphView) {
    return null
  }

  return { seriesModel, graph, graphView }
}

function applyPreciseEdgeLayouts() {
  const runtime = getGraphRuntime()
  if (!runtime) return

  const metricsMap = new Map(graphData.value.nodes.map((node) => [node.id, serviceMetrics(node)] as const))
  const pairGroups = new Map<string, string[]>()
  const outgoingGroups = new Map<string, string[]>()

  runtime.graph.eachEdge((edge: any, index: number) => {
    const sourceId = String(edge.node1.id)
    const targetId = String(edge.node2.id)
    const descriptor = buildEdgeDescriptor(sourceId, targetId, index)
    const pairKey = [sourceId, targetId].sort().join('::')

    if (!pairGroups.has(pairKey)) {
      pairGroups.set(pairKey, [])
    }
    pairGroups.get(pairKey)!.push(descriptor)

    if (!outgoingGroups.has(sourceId)) {
      outgoingGroups.set(sourceId, [])
    }
    outgoingGroups.get(sourceId)!.push(descriptor)
  })

  runtime.graph.eachEdge((edge: any, index: number) => {
    const sourceId = String(edge.node1.id)
    const targetId = String(edge.node2.id)
    const sourceMetrics = metricsMap.get(sourceId)
    const targetMetrics = metricsMap.get(targetId)
    const sourceLayout = edge.node1.getLayout() as number[] | undefined
    const targetLayout = edge.node2.getLayout() as number[] | undefined

    if (!sourceMetrics || !targetMetrics || !sourceLayout || !targetLayout) {
      return
    }

    const descriptor = buildEdgeDescriptor(sourceId, targetId, index)
    const pairKey = [sourceId, targetId].sort().join('::')
    const pairItems = pairGroups.get(pairKey) || []
    const pairIndex = Math.max(0, pairItems.indexOf(descriptor))
    const pairLane = pairItems.length > 1 ? (pairIndex - (pairItems.length - 1) / 2) * EDGE_LANE_GAP : 0

    const outgoingItems = outgoingGroups.get(sourceId) || []
    const outgoingIndex = Math.max(0, outgoingItems.indexOf(descriptor))
    const outgoingLane = outgoingItems.length > 1 ? (outgoingIndex - (outgoingItems.length - 1) / 2) * EDGE_SIBLING_GAP : 0

    const laneOffset = pairLane + outgoingLane
    const bend = pairItems.length > 2 ? pairLane * 1.15 : 0
    const points = buildPreciseEdgePoints(
      [sourceLayout[0]!, sourceLayout[1]!] as Point,
      [targetLayout[0]!, targetLayout[1]!] as Point,
      sourceMetrics,
      targetMetrics,
      laneOffset,
      bend,
    )

    ;(points as any).__original = points.map((point) => [point[0], point[1]] as Point)
    edge.setLayout(points as any)
  })

  ;(runtime.graphView as any)._lineDraw?.updateLayout()
}

function schedulePreciseEdgeLayouts() {
  if (edgeLayoutFrame) {
    window.cancelAnimationFrame(edgeLayoutFrame)
  }

  edgeLayoutFrame = window.requestAnimationFrame(() => {
    edgeLayoutFrame = 0
    applyPreciseEdgeLayouts()
  })
}
function instanceTone(instance: TopologyInstance, selected: boolean) {
  const passing = instance.status === 'passing'
  return {
    fill: passing ? '#22c55e' : '#ef4444',
    border: passing ? (selected ? '#14532d' : '#166534') : selected ? '#7f1d1d' : '#991b1b',
    glow: passing ? 'rgba(34, 197, 94, 0.28)' : 'rgba(239, 68, 68, 0.24)',
  }
}

function buildOption(width: number, height: number): echarts.EChartsOption {
  const metricsMap = new Map(graphData.value.nodes.map((node) => [node.id, serviceMetrics(node)] as const))
  const positions = computeLayout(width, height, metricsMap)

  const serviceNodes: any[] = graphData.value.nodes.map((node) => {
    const metrics = metricsMap.get(node.id) || serviceMetrics(node)
    const tone = nodeTone(node)
    const state = serviceHealthState(node)
    const isSelected = props.selectedNode === node.id
    const contentWidth = metrics.width - CARD_PADDING_X * 2
    const titleLines = metrics.titleText.split('\n')
    const titleLeadWidth = Math.max(88, contentWidth - STATUS_BADGE_WIDTH - STATUS_TITLE_GAP)
    const titleFormatter = [
      `{titleLead|${escapeRichText(titleLines[0] || node.name)}}{status|${escapeRichText(nodeStatusLabel(state))}}`, 
      ...titleLines.slice(1).map((line) => `{title|${escapeRichText(line)}}`),
    ].join('\n')
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
      symbol: 'rect',
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
        borderRadius: 0,
        shadowColor: isSelected ? 'rgba(37, 99, 235, 0.22)' : tone.glow,
        shadowBlur: isSelected ? 22 : 14,
        shadowOffsetY: 8,
      },
      label: {
        show: true,
        position: 'insideTopLeft',
        align: 'left' as const,
        verticalAlign: 'top' as const,
        padding: [CARD_PADDING_TOP, CARD_PADDING_X, 0, CARD_PADDING_X],
        formatter: titleFormatter,
        rich: {
          status: {
            width: STATUS_BADGE_WIDTH,
            align: 'right' as const,
            fontSize: 10,
            fontFamily: 'Inter, system-ui, sans-serif',
            fontWeight: 700,
            color: tone.badgeText,
            padding: [0, 0, 0, STATUS_TITLE_GAP],
            lineHeight: STATUS_LINE_HEIGHT,
          },
          titleLead: {
            width: titleLeadWidth,
            align: 'left' as const,
            fontSize: 14,
            fontFamily: 'Inter, system-ui, sans-serif',
            fontWeight: 700,
            color: tone.title,
            lineHeight: TITLE_LINE_HEIGHT,
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
      emphasis: {
        disabled: true,
      },
    }
  })

  const instanceNodes: any[] = graphData.value.nodes.flatMap((node) => {
    const metrics = metricsMap.get(node.id) || serviceMetrics(node)
    const pos = positions.get(node.id) || { x: width / 2, y: height / 2 }
    const left = pos.x - metrics.width / 2 + CARD_PADDING_X + INSTANCE_ICON_OFFSET_X + INSTANCE_ICON_SIZE / 2
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
        emphasis: {
          disabled: true,
        },
      }
    })
  })

  const outgoingGroups = new Map<string, string[]>()
  const pairGroups = new Map<string, string[]>()
  for (const edge of graphData.value.edges) {
    if (!outgoingGroups.has(edge.source)) {
      outgoingGroups.set(edge.source, [])
    }
    outgoingGroups.get(edge.source)!.push(edge.target)

    const pairKey = [edge.source, edge.target].sort().join('::')
    if (!pairGroups.has(pairKey)) {
      pairGroups.set(pairKey, [])
    }
    pairGroups.get(pairKey)!.push(`${edge.source}->${edge.target}`)
  }
  for (const items of outgoingGroups.values()) {
    items.sort()
  }

  const links: any[] = graphData.value.edges.map((edge) => {
    const siblings = outgoingGroups.get(edge.source) || []
    const siblingIndex = siblings.indexOf(edge.target)
    const siblingCurve = siblings.length > 1 ? (siblingIndex - (siblings.length - 1) / 2) * 0.12 : 0
    const pairKey = [edge.source, edge.target].sort().join('::')
    const pairItems = pairGroups.get(pairKey) || []
    const pairRoot = pairKey.split('::')[0]
    const pairCurve = pairItems.length > 1 ? (edge.source === pairRoot ? 0.18 : -0.18) : 0
    const curve = clamp(pairCurve + siblingCurve, -0.28, 0.28)
    const relatedToSelection =
      !!props.selectedNode && (edge.source === props.selectedNode || edge.target === props.selectedNode)

    return {
      source: edge.source,
      target: edge.target,
      symbol: ['none', 'arrow'],
      symbolSize: [0, relatedToSelection ? EDGE_ARROW_SIZE + 1 : EDGE_ARROW_SIZE],
      lineStyle: {
        color: relatedToSelection ? 'rgba(37, 99, 235, 0.86)' : 'rgba(59, 130, 246, 0.42)',
        curveness: curve,
        width: relatedToSelection ? 2.8 : 1.9,
        opacity: relatedToSelection ? 1 : 0.94,
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
      position(point: [number, number], _params: any, _el: any, _rect: any, size: any) {
        const [viewWidth, viewHeight] = size.viewSize as [number, number]
        const [contentWidth, contentHeight] = size.contentSize as [number, number]
        const gap = 16
        const margin = 12
        const maxX = Math.max(margin, viewWidth - contentWidth - margin)
        const maxY = Math.max(margin, viewHeight - contentHeight - margin)
        const aboveY = point[1] - contentHeight - gap
        const belowY = point[1] + gap
        let x = point[0] + gap
        let y = aboveY >= margin ? aboveY : belowY

        if (x > maxX) {
          x = point[0] - contentWidth - gap
        }

        return [clamp(x, margin, maxX), clamp(y, margin, maxY)]
      },
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
        nodeScaleRatio: NODE_SCALE_RATIO,

        data: [...serviceNodes, ...instanceNodes],
        links,
        roam: true,
        scaleLimit: {
          min: 0.45,
          max: 2.4,
        },
        zoom: currentZoom.value,
        tooltip: { show: true },
        label: {
          position: 'insideTopLeft',
        },
        edgeSymbol: ['none', 'arrow'],
        edgeSymbolSize: [0, EDGE_ARROW_SIZE],
        edgeLabel: {
          show: false,
        },
        lineStyle: {
          opacity: 0.95,
        },
        emphasis: {
          focus: 'none',
          scale: false,
        },
      },
    ],
  }
}

function getCanvasSize() {
  return {
    width: chartEl.value?.clientWidth || 0,
    height: chartEl.value?.clientHeight || 0,
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
      schedulePreciseEdgeLayouts()
    })
  }
}

async function render() {
  const version = ++renderVersion
  await nextTick()
  await new Promise<void>((resolve) => window.requestAnimationFrame(() => resolve()))
  if (version !== renderVersion) return

  const { width, height } = getCanvasSize()
  if (width < MIN_RENDER_SIZE || height < MIN_RENDER_SIZE) return

  ensureChart()
  chart?.resize({ width, height })
  chart?.setOption(buildOption(width, height), { notMerge: true, lazyUpdate: true })
  schedulePreciseEdgeLayouts()
}

function resize() {
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

function resetView() {
  currentZoom.value = 1
  render()
}

watch(graphData, render, { deep: true, flush: 'post' })
watch(() => props.selectedNode, render, { flush: 'post' })
watch(() => props.loading, render, { flush: 'post' })
watch(locale, render, { flush: 'post' })

onMounted(async () => {
  if (typeof ResizeObserver !== 'undefined' && chartEl.value) {
    resizeObserver = new ResizeObserver(() => {
      render()
    })
    resizeObserver.observe(chartEl.value)
  }

  await render()
  window.addEventListener('resize', resize)
})

onBeforeUnmount(() => {
  renderVersion += 1
  if (edgeLayoutFrame) {
    window.cancelAnimationFrame(edgeLayoutFrame)
    edgeLayoutFrame = 0
  }
  resizeObserver?.disconnect()
  resizeObserver = null
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
      <button type="button" class="zoom-value" @click="resetView" :title="text('重置视图', 'Reset view')">{{ Math.round(currentZoom * 100) }}%</button>
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
  border: 0;
  border-top: 1px solid rgba(148, 163, 184, 0.12);
  border-bottom: 1px solid rgba(148, 163, 184, 0.12);
  cursor: pointer;
  padding: 0;
}

.zoom-value:hover {
  background: #ffffff;
  color: #2563eb;
}

.graph-canvas {
  width: 100%;
  height: 100%;
  min-height: 560px;
  cursor: grab;
}

.graph-canvas:active {
  cursor: grabbing;
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

