<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from '../utils/i18n'

export interface GuideStep {
  id: string
  selector?: string
  route?: string
  query?: Record<string, string>
  title: {
    zh: string
    en: string
  }
  description: {
    zh: string
    en: string
  }
  placement?: 'top' | 'right' | 'bottom' | 'left' | 'center'
  padding?: number
}

const props = defineProps<{
  modelValue: boolean
  steps: GuideStep[]
}>()

const emit = defineEmits<{
  (e: 'update:modelValue', value: boolean): void
  (e: 'complete'): void
  (e: 'dismiss'): void
}>()

const CARD_GAP = 20
const VIEWPORT_GAP = 16

const route = useRoute()
const router = useRouter()
const { locale } = useI18n()

const stepIndex = ref(0)
const spotlightRect = ref<{ top: number; left: number; width: number; height: number; radius: number } | null>(null)
const cardStyle = ref<Record<string, string>>({})
const targetReady = ref(false)
const cardEl = ref<HTMLElement | null>(null)

let updateTimer: ReturnType<typeof setTimeout> | null = null
let rafId = 0

const currentStep = computed(() => props.steps[stepIndex.value] || null)
const currentTitle = computed(() => {
  if (!currentStep.value) return ''
  return locale.value === 'zh' ? currentStep.value.title.zh : currentStep.value.title.en
})
const currentDescription = computed(() => {
  if (!currentStep.value) return ''
  return locale.value === 'zh' ? currentStep.value.description.zh : currentStep.value.description.en
})
const backdropStyles = computed<Record<string, string>[]>(() => {
  if (typeof window === 'undefined') return []

  const viewportWidth = window.innerWidth
  const viewportHeight = window.innerHeight

  if (!spotlightRect.value) {
    return [
      {
        top: '0px',
        left: '0px',
        width: `${viewportWidth}px`,
        height: `${viewportHeight}px`,
      },
    ]
  }

  const rect = spotlightRect.value
  const right = rect.left + rect.width
  const bottom = rect.top + rect.height

  return [
    { top: 0, left: 0, width: viewportWidth, height: rect.top },
    { top: rect.top, left: 0, width: rect.left, height: rect.height },
    { top: rect.top, left: right, width: Math.max(0, viewportWidth - right), height: rect.height },
    { top: bottom, left: 0, width: viewportWidth, height: Math.max(0, viewportHeight - bottom) },
  ]
    .filter((segment) => segment.width > 0 && segment.height > 0)
    .map((segment) => ({
      top: `${segment.top}px`,
      left: `${segment.left}px`,
      width: `${segment.width}px`,
      height: `${segment.height}px`,
    }))
})

function sameQuery(expected: Record<string, string> = {}) {
  return Object.entries(expected).every(([key, value]) => `${route.query[key] || ''}` === `${value}`)
}

async function ensureRouteForStep(step: GuideStep) {
  if (!step.route) return

  if (route.path !== step.route || !sameQuery(step.query)) {
    await router.push({
      path: step.route,
      query: step.query,
    })
    await nextTick()
  }
}

function getStepTarget(step: GuideStep) {
  if (!step.selector || typeof window === 'undefined') return null
  return document.querySelector<HTMLElement>(step.selector)
}

async function waitForTarget(step: GuideStep) {
  if (!step.selector) return null

  for (let index = 0; index < 30; index += 1) {
    const element = getStepTarget(step)
    if (element) {
      return element
    }
    await new Promise((resolve) => {
      updateTimer = setTimeout(resolve, 120)
    })
  }

  return null
}

function clamp(value: number, min: number, max: number) {
  return Math.min(max, Math.max(min, value))
}

function updateCardPosition() {
  if (!props.modelValue || !cardEl.value) return

  const viewportWidth = window.innerWidth
  const viewportHeight = window.innerHeight
  const cardWidth = Math.min(cardEl.value.offsetWidth || 320, viewportWidth - VIEWPORT_GAP * 2)
  const cardHeight = cardEl.value.offsetHeight || 220

  if (!spotlightRect.value || currentStep.value?.placement === 'center') {
    cardStyle.value = {
      top: `${Math.max(VIEWPORT_GAP, (viewportHeight - cardHeight) / 2)}px`,
      left: `${Math.max(VIEWPORT_GAP, (viewportWidth - cardWidth) / 2)}px`,
      width: `${cardWidth}px`,
    }
    return
  }

  const rect = spotlightRect.value
  const placement = currentStep.value?.placement || 'bottom'
  let left = rect.left + rect.width / 2 - cardWidth / 2
  let top = rect.top + rect.height + CARD_GAP

  if (placement === 'top') {
    top = rect.top - cardHeight - CARD_GAP
  }
  if (placement === 'left') {
    left = rect.left - cardWidth - CARD_GAP
    top = rect.top + rect.height / 2 - cardHeight / 2
  }
  if (placement === 'right') {
    left = rect.left + rect.width + CARD_GAP
    top = rect.top + rect.height / 2 - cardHeight / 2
  }

  left = clamp(left, VIEWPORT_GAP, viewportWidth - cardWidth - VIEWPORT_GAP)
  top = clamp(top, VIEWPORT_GAP, viewportHeight - cardHeight - VIEWPORT_GAP)

  cardStyle.value = {
    top: `${top}px`,
    left: `${left}px`,
    width: `${cardWidth}px`,
  }
}

function updateSpotlight(element: HTMLElement | null, padding = 14) {
  if (!element) {
    spotlightRect.value = null
    targetReady.value = false
    return
  }

  const rect = element.getBoundingClientRect()
  spotlightRect.value = {
    top: Math.max(VIEWPORT_GAP, rect.top - padding),
    left: Math.max(VIEWPORT_GAP, rect.left - padding),
    width: Math.min(window.innerWidth - VIEWPORT_GAP * 2 - Math.max(VIEWPORT_GAP, rect.left - padding) + VIEWPORT_GAP, rect.width + padding * 2),
    height: Math.min(window.innerHeight - VIEWPORT_GAP * 2 - Math.max(VIEWPORT_GAP, rect.top - padding) + VIEWPORT_GAP, rect.height + padding * 2),
    radius: 18,
  }
  targetReady.value = true
}

async function syncStepPosition() {
  if (!props.modelValue || !currentStep.value) return

  targetReady.value = false
  await ensureRouteForStep(currentStep.value)

  if (!currentStep.value.selector) {
    spotlightRect.value = null
    targetReady.value = true
    await nextTick()
    updateCardPosition()
    return
  }

  const target = await waitForTarget(currentStep.value)
  if (target) {
    target.scrollIntoView({ behavior: 'smooth', block: 'center', inline: 'center' })
    await new Promise((resolve) => {
      updateTimer = setTimeout(resolve, 180)
    })
  }

  updateSpotlight(target, currentStep.value.padding ?? 14)
  await nextTick()
  updateCardPosition()
}

function queuePositionSync() {
  cancelAnimationFrame(rafId)
  rafId = requestAnimationFrame(() => {
    syncStepPosition()
  })
}

function removeListeners() {
  window.removeEventListener('resize', queuePositionSync)
  window.removeEventListener('scroll', queuePositionSync, true)
}

function closeGuide(completed: boolean) {
  emit('update:modelValue', false)
  if (completed) {
    emit('complete')
    return
  }
  emit('dismiss')
}

function nextStep() {
  if (stepIndex.value >= props.steps.length - 1) {
    closeGuide(true)
    return
  }
  stepIndex.value += 1
}

function previousStep() {
  if (stepIndex.value === 0) return
  stepIndex.value -= 1
}

watch(
  () => props.modelValue,
  async (visible) => {
    if (!visible) {
      removeListeners()
      spotlightRect.value = null
      targetReady.value = false
      return
    }

    stepIndex.value = 0
    await nextTick()
    queuePositionSync()
    window.addEventListener('resize', queuePositionSync)
    window.addEventListener('scroll', queuePositionSync, true)
  },
)

watch(
  () => stepIndex.value,
  () => {
    queuePositionSync()
  },
)

watch(
  () => route.fullPath,
  () => {
    if (props.modelValue) {
      queuePositionSync()
    }
  },
)

watch(
  () => locale.value,
  async () => {
    if (!props.modelValue) return
    await nextTick()
    updateCardPosition()
  },
)

onBeforeUnmount(() => {
  if (updateTimer) clearTimeout(updateTimer)
  cancelAnimationFrame(rafId)
  removeListeners()
})
</script>

<template>
  <teleport to="body">
    <transition name="guide-fade">
      <div v-if="modelValue && currentStep" class="guide-overlay" role="dialog" aria-modal="true">
        <div
          v-for="(style, index) in backdropStyles"
          :key="index"
          class="guide-backdrop"
          :style="style"
          @click="closeGuide(false)"
        ></div>
        <div
          v-if="spotlightRect"
          class="guide-spotlight"
          :style="{
            top: `${spotlightRect.top}px`,
            left: `${spotlightRect.left}px`,
            width: `${spotlightRect.width}px`,
            height: `${spotlightRect.height}px`,
            borderRadius: `${spotlightRect.radius}px`,
          }"
        ></div>

        <div ref="cardEl" class="guide-card" :style="cardStyle" :data-target-ready="targetReady ? '1' : '0'">
          <div class="guide-progress">
            <span>{{ stepIndex + 1 }} / {{ steps.length }}</span>
          </div>

          <h3>{{ currentTitle }}</h3>
          <p>{{ currentDescription }}</p>

          <div class="guide-actions">
            <button type="button" class="ghost-btn" @click="closeGuide(false)">
              {{ locale === 'zh' ? '跳过' : 'Skip' }}
            </button>
            <div class="guide-main-actions">
              <button type="button" class="ghost-btn" :disabled="stepIndex === 0" @click="previousStep">
                {{ locale === 'zh' ? '上一步' : 'Back' }}
              </button>
              <button type="button" class="solid-btn" @click="nextStep">
                {{ stepIndex === steps.length - 1 ? (locale === 'zh' ? '完成' : 'Done') : (locale === 'zh' ? '下一步' : 'Next') }}
              </button>
            </div>
          </div>
        </div>
      </div>
    </transition>
  </teleport>
</template>

<style scoped>
.guide-overlay {
  position: fixed;
  inset: 0;
  z-index: 3000;
  pointer-events: none;
}

.guide-backdrop {
  position: absolute;
  background: rgba(15, 23, 42, 0.5);
  backdrop-filter: blur(2px);
  pointer-events: auto;
}

.guide-spotlight {
  position: absolute;
  border: 2px solid rgba(96, 165, 250, 0.92);
  box-shadow:
    0 0 0 1px rgba(191, 219, 254, 0.72),
    0 18px 40px rgba(37, 99, 235, 0.14);
  background: transparent;
  pointer-events: none;
}

.guide-card {
  position: absolute;
  max-width: min(360px, calc(100vw - 32px));
  border-radius: 18px;
  background: rgba(255, 255, 255, 0.96);
  border: 1px solid rgba(148, 163, 184, 0.2);
  box-shadow: 0 20px 48px rgba(15, 23, 42, 0.2);
  padding: 18px 18px 16px;
  color: #0f172a;
  pointer-events: auto;
}

.guide-progress {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  font-size: 12px;
  color: #64748b;
  margin-bottom: 10px;
}

.guide-card h3 {
  margin: 0 0 10px;
  font-size: 18px;
  line-height: 1.3;
}

.guide-card p {
  margin: 0;
  color: #475569;
  line-height: 1.65;
  font-size: 14px;
}

.guide-actions {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-top: 18px;
}

.guide-main-actions {
  display: flex;
  align-items: center;
  gap: 10px;
}

.ghost-btn,
.solid-btn {
  border: 0;
  border-radius: 12px;
  padding: 9px 14px;
  font: inherit;
  cursor: pointer;
  transition: all 0.18s ease;
}

.ghost-btn {
  background: rgba(226, 232, 240, 0.72);
  color: #334155;
}

.ghost-btn:hover:not(:disabled) {
  background: rgba(203, 213, 225, 0.92);
}

.ghost-btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.solid-btn {
  background: linear-gradient(135deg, #2563eb 0%, #1d4ed8 100%);
  color: #ffffff;
  box-shadow: 0 12px 24px rgba(37, 99, 235, 0.22);
}

.solid-btn:hover {
  transform: translateY(-1px);
  box-shadow: 0 16px 28px rgba(37, 99, 235, 0.28);
}

.guide-fade-enter-active,
.guide-fade-leave-active {
  transition: opacity 0.18s ease;
}

.guide-fade-enter-from,
.guide-fade-leave-to {
  opacity: 0;
}

@media (max-width: 768px) {
  .guide-card {
    max-width: calc(100vw - 24px);
    padding: 16px;
  }

  .guide-actions {
    flex-direction: column;
    align-items: stretch;
  }

  .guide-main-actions {
    justify-content: space-between;
  }
}
</style>