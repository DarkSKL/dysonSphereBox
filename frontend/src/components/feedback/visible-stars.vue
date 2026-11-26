<script setup lang="ts">
import {X} from '@lucide/vue'
import {computed, nextTick, ref, watch} from 'vue'

import {starLabel} from '../../composables/feedback/galaxy-art'
import {useGalaxyTheme} from '../../composables/feedback/useGalaxyTheme'
import {resolveFocusFallback, type ProjectedStar} from '../../composables/feedback/galaxy-camera'
import type {DisplayStar, StarPreview} from '../../types/feedback'

const props = defineProps<{
  stars: ProjectedStar[]
  opened: boolean
  quote: StarPreview | null
  /** 列表来源：视野内全部可选星，或点击处附近难以分辨的几颗星。 */
  heading?: string
}>()
const theme = useGalaxyTheme()
const emit = defineEmits<{
  close: []
  open: [starId: string]
  preview: [point: DisplayStar | null]
}>()
const root = ref<HTMLElement>()
const page = ref(1)
const pageSize = 12
const lastPage = computed(() => Math.max(1, Math.ceil(props.stars.length / pageSize)))
const currentPage = computed(() => Math.min(page.value, lastPage.value))
const rows = computed(() => props.stars.slice((currentPage.value - 1) * pageSize, currentPage.value * pageSize))
const rowIds = computed(() => rows.value.map(({point}) => point.starId))

watch(lastPage, (count) => { if (page.value > count) page.value = count })
// 切换列表来源（视野内全部 / 点击处附近）时回到第一页。
watch(() => props.heading, () => { page.value = 1 })
watch(() => props.opened, async (opened) => {
  if (!opened) return
  page.value = 1
  await nextTick()
  root.value?.focus()
})
// 快照更新移除当前聚焦的候选（含页码因总数减少而回退）时，把焦点移到剩余可选候选按钮，避免跌落到 body。
// 边界：只在更新前焦点位于列表候选按钮上时接管；焦点原本在列表外、关闭按钮或分页上时绝不抢焦点。
watch(rowIds, (next, prev) => {
  if (!props.opened || !root.value) return
  const active = document.activeElement
  if (!(active instanceof HTMLElement) || !root.value.contains(active)) return
  const focusedId = active.dataset.starId ?? ''
  const fallback = resolveFocusFallback(prev, next, focusedId)
  if (fallback === null) return
  void nextTick(() => {
    if (!props.opened || !root.value) return
    const current = document.activeElement
    if (current && current !== document.body && !root.value.contains(current)) return
    if (fallback) {
      const buttons = root.value.querySelectorAll<HTMLButtonElement>('button[data-star-id]')
      for (let i = 0; i < buttons.length; i++) {
        if (buttons[i].dataset.starId === fallback) {
          buttons[i].focus({preventScroll: true})
          return
        }
      }
    }
    root.value.focus({preventScroll: true})
  })
})
</script>

<template>
  <!-- ~ 当前视野列表 -->
  <!-- * 说明：用有界列表提供同一批可点击的反馈星。 -->
  <!-- * 交互：Tab 聚焦读取首行，Enter 打开详情，Escape 返回星图。 -->
  <!-- * 依赖：当前镜头内的公开或私有星点。 -->
  <!-- * 边界：列表与画布共用视野内的反馈星。 -->
  <section v-if="opened" ref="root" class="visible-stars" aria-label="可浏览星点" tabindex="-1"
    @keydown.esc.stop.prevent="emit('close')">
    <header>
      <h3>{{ heading || '当前视野' }} · {{ stars.length }} 颗可选</h3>
      <n-button size="tiny" quaternary aria-label="关闭星点列表" title="关闭星点列表" @click="emit('close')">
        <X :size="16" aria-hidden="true" />
      </n-button>
    </header>
    <p v-if="!rows.length" class="empty-note">当前视野没有反馈星。</p>
    <ul v-else>
      <li v-for="{point} in rows" :key="point.starId">
        <button type="button" :data-star-id="point.starId" :aria-describedby="quote?.starId === point.starId ? 'feedback-list-quote' : undefined"
          @focus="emit('preview', point)" @blur="emit('preview', null)" @click="emit('open', point.starId)">
          <span class="list-dot" :class="{private: point.privateLabel}" :style="{color: theme.starColors[point.progress]}" aria-hidden="true" />
          <span>{{ starLabel(point) }} · 星 {{ point.starId }}</span>
        </button>
      </li>
    </ul>
    <p id="feedback-list-quote" class="focus-preview" aria-live="polite">
      <template v-if="quote?.preview">{{ quote.preview }}<span v-if="quote.displayName"> — <bdi>{{ quote.displayName }}</bdi></span></template>
    </p>
    <n-pagination v-if="lastPage > 1" size="small" :page="currentPage" :page-count="lastPage" :page-slot="3" @update:page="page = $event" />
  </section>
</template>

<style scoped lang="scss">
.visible-stars {
  position: absolute; z-index: 2; top: 12px; left: 12px; width: min(300px, calc(100% - 24px));
  max-height: calc(100% - 75px); overflow: auto; padding: 12px;
  border: 1px solid var(--ds-border-subtle); border-radius: 4px;
  color: var(--ds-text-primary); background: var(--ds-bg-panel-elevated);
  &:focus { outline: none; }
  &:focus-visible { outline: 1.5px solid color-mix(in srgb, var(--ds-brand) 78%, transparent); outline-offset: -2px; }
}
header { display: flex; align-items: center; justify-content: space-between; gap: 8px; }
h3 { margin: 0; font-size: 14px; font-weight: 600; }
ul { list-style: none; padding: 0; margin: 7px 0; }
li button {
  display: flex; align-items: center; gap: 8px; width: 100%; color: inherit;
  padding: 5px 6px; border-radius: 4px; border: 0; background: none; text-align: left; cursor: pointer;
  &:hover, &:focus-visible { background: var(--ds-bg-panel); }
  &:focus-visible { outline: 2px solid var(--ds-brand); }
}
@keyframes list-star-twinkle {
  0%, 100% {
    opacity: 0.38;
    transform: scale(0.86);
  }
  50% {
    opacity: 1;
    transform: scale(1.08);
  }
}
.list-dot { flex: 0 0 7px; height: 7px; background: currentColor; border-radius: 50%; box-shadow: inset 0 0 0 1px var(--ds-border-default); }
.list-dot.private { animation: list-star-twinkle 2.4s ease-in-out infinite; }
@media (prefers-reduced-motion: reduce) { .list-dot.private { animation: none; } }
.focus-preview { color: var(--ds-text-secondary); margin: 7px 0; overflow-wrap: anywhere; }
</style>
